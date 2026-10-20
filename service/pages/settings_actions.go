package pages

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sun-panel/global"
	"sun-panel/lib/cmn"
	"sun-panel/models"
	"sun-panel/webrender"

	"github.com/gin-gonic/gin"
)

// ---- 个性化设置 ----

// StyleSave 保存面板外观
func StyleSave(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	cfg := loadPanelConfig(user.ID)

	cfg.BackgroundImageSrc = strings.TrimSpace(c.PostForm("backgroundImageSrc"))
	if cfg.BackgroundImageSrc == "" {
		cfg.BackgroundImageSrc = "defaultBackground.jpg"
	}
	cfg.BackgroundBlur = atoiDefault(c.PostForm("backgroundBlur"), cfg.BackgroundBlur)
	cfg.BackgroundMaskNumber = atofDefault(c.PostForm("backgroundMaskNumber"), cfg.BackgroundMaskNumber)
	cfg.ClockShow = c.PostForm("clockShow") == "1" || c.PostForm("clockShow") == "on"
	cfg.ClockShowSecond = c.PostForm("clockShowSecond") == "1" || c.PostForm("clockShowSecond") == "on"
	cfg.SearchBoxShow = c.PostForm("searchBoxShow") == "1" || c.PostForm("searchBoxShow") == "on"
	cfg.SearchBoxBackground = strings.TrimSpace(c.PostForm("searchBoxBackground"))
	cfg.SearchBoxTextColor = strings.TrimSpace(c.PostForm("searchBoxTextColor"))
	cfg.MaxWidth = atoiDefault(c.PostForm("maxWidth"), cfg.MaxWidth)
	if cfg.MaxWidth < 600 {
		cfg.MaxWidth = 600
	}
	if cfg.MaxWidth > 2400 {
		cfg.MaxWidth = 2400
	}
	cfg.IconTextColor = strings.TrimSpace(c.PostForm("iconTextColor"))
	cfg.FooterHtml = c.PostForm("footerHtml")
	cfg.NetModeChangeButtonShow = c.PostForm("netModeChangeButtonShow") == "1" || c.PostForm("netModeChangeButtonShow") == "on"

	savePanelConfig(user.ID, cfg)
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

func savePanelConfig(userId uint, cfg PanelConfig) {
	if global.Db == nil {
		return
	}
	b, _ := json.Marshal(cfg)
	var uc models.UserConfig
	err := global.Db.Where("user_id = ?", userId).First(&uc).Error
	if err != nil {
		global.Db.Create(&models.UserConfig{UserId: userId, PanelJson: string(b)})
		return
	}
	global.Db.Model(&models.UserConfig{}).Where("user_id = ?", userId).
		Update("panel_json", string(b))
}

// ---- 导出导入 ----

// Export 导出配置为 JSON
func Export(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	groups, _ := loadGroups(c, user.ID)
	// 用原始模型导出，保留全部字段
	var rawGroups []models.ItemIconGroup
	var rawCards []models.ItemIcon
	global.Db.Where("user_id = ?", user.ID).Order("sort asc").Find(&rawGroups)
	global.Db.Where("user_id = ?", user.ID).Order("sort asc").Find(&rawCards)

	payload := gin.H{
		"version":    CurrentBackupVersion,
		"app":        "sun-panel-v2",
		"exportedAt": time.Now().Format(time.RFC3339),
		"groups":     rawGroups,
		"cards":      rawCards,
		"groupCount": len(groups),
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	fn := "sunpanel-backup-" + time.Now().Format("20060102-150405") + ".json"
	c.Header("Content-Disposition", "attachment; filename=\""+fn+"\"")
	c.Data(http.StatusOK, "application/json", b)
}

// backupPayload 备份文件结构。
//
// Version 用 interface{} 接收，再用 normalizeBackupVersion 归一化：
// 备份文件可被手工编辑，version 可能是 1 / "1" / 1.0。
type backupPayload struct {
	Version int                     `json:"-"`
	Groups  []models.ItemIconGroup  `json:"groups"`
	Cards   []models.ItemIcon       `json:"cards"`
}

// Import 导入配置（整体替换，事务）
func Import(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "未选择文件")
		return
	}
	f, err := fh.Open()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()

	// 限制备份文件大小，避免超大文件打满内存
	const maxBackupBytes = 64 << 20 // 64 MB
	raw, err := io.ReadAll(io.LimitReader(f, maxBackupBytes+1))
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	if len(raw) > maxBackupBytes {
		c.String(http.StatusBadRequest, "备份文件过大（超过 64MB）")
		return
	}

	// 先取 version，再决定是否按当前结构解析。
	// 用 map 读一次原始值，避免结构体字段类型限制导致解析失败。
	var head struct {
		Version interface{} `json:"version"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		c.String(http.StatusBadRequest, "文件格式错误："+err.Error())
		return
	}

	ver := normalizeBackupVersion(head.Version)
	chk := checkBackupVersion(ver)
	if !chk.OK {
		// 版本过高：拒绝，避免静默丢数据
		c.String(http.StatusBadRequest, chk.Reason)
		return
	}

	var payload backupPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		c.String(http.StatusBadRequest, "文件格式错误："+err.Error())
		return
	}
	payload.Version = ver

	if chk.Migrate {
		if err := migrateBackup(ver, &payload); err != nil {
			c.String(http.StatusBadRequest, "备份迁移失败："+err.Error())
			return
		}
	}

	tx := global.Db.Begin()
	if err := tx.Where("user_id = ?", user.ID).Delete(&models.ItemIcon{}).Error; err != nil {
		tx.Rollback()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Where("user_id = ?", user.ID).Delete(&models.ItemIconGroup{}).Error; err != nil {
		tx.Rollback()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	for i := range payload.Groups {
		g := payload.Groups[i]
		g.ID = 0
		g.UserId = user.ID
		if err := tx.Create(&g).Error; err != nil {
			tx.Rollback()
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	for i := range payload.Cards {
		cd := payload.Cards[i]
		cd.ID = 0
		cd.UserId = user.ID
		if err := tx.Create(&cd).Error; err != nil {
			tx.Rollback()
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	tx.Commit()

	webrender.RenderPartial(c, "import_ok.html", gin.H{
		"Groups": len(payload.Groups),
		"Cards":  len(payload.Cards),
	})
}

// ---- 图库 ----

// GalleryUpload 上传图片
func GalleryUpload(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		c.String(http.StatusBadRequest, "未选择文件")
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		files = form.File["file"]
	}
	okCount := 0
	now := time.Now()
	rel := filepath.Join("uploads", now.Format("2006"), now.Format("01"), now.Format("02"))
	for _, fh := range files {
		if fh.Size > 10<<20 {
			continue
		}
		dir := filepath.Join("/app", rel)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			dir = rel
			_ = os.MkdirAll(dir, 0o755)
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		name := fmt.Sprintf("%d%s", now.UnixNano()+int64(okCount), ext)
		dst := filepath.Join(dir, name)
		src, err := fh.Open()
		if err != nil {
			continue
		}
		out, err := os.Create(dst)
		if err != nil {
			src.Close()
			continue
		}
		_, _ = io.Copy(out, src)
		out.Close()
		src.Close()

		url := "/" + filepath.ToSlash(filepath.Join(rel, name))
		global.Db.Create(&models.File{Src: url, UserId: user.ID, FileName: fh.Filename, Ext: ext})
		okCount++
	}
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, fmt.Sprintf("已上传 %d 个文件", okCount))
}

// GalleryDelete 删除图库文件
func GalleryDelete(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id := parseUint(c.Param("id"))
	var f models.File
	if err := global.Db.Where("id = ? AND user_id = ?", id, user.ID).First(&f).Error; err != nil {
		c.String(http.StatusNotFound, "文件不存在")
		return
	}
	// 先删磁盘文件
	p := filepath.Join("/app", strings.TrimPrefix(f.Src, "/"))
	_ = os.Remove(p)
	if err := global.Db.Where("id = ? AND user_id = ?", id, user.ID).Delete(&models.File{}).Error; err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

// ---- 账号管理 ----

// UsersCreate 新建账号
func UsersCreate(c *gin.Context) {
	cur, ok := currentUser(c)
	if !ok || cur.Role != 1 {
		c.String(http.StatusForbidden, "需要管理员权限")
		return
	}
	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	name := strings.TrimSpace(c.PostForm("name"))
	mail := strings.TrimSpace(c.PostForm("mail"))
	role := atoiDefault(c.PostForm("role"), 2)
	if username == "" || len(password) < 6 {
		c.String(http.StatusBadRequest, "用户名必填，密码至少 6 位")
		return
	}
	// CheckUsernameExist 在用户名已存在时返回 error
	mUser := models.User{}
	if _, err := mUser.CheckUsernameExist(username); err == nil {
		c.String(http.StatusBadRequest, "用户名已存在")
		return
	}
	u := models.User{
		Username: username,
		Password: cmn.PasswordEncryption(password),
		Name:     name,
		Mail:     mail,
		Role:     role,
		Status:   1,
		Token:    randomToken(32),
	}
	if err := global.Db.Create(&u).Error; err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

// UsersUpdate 修改账号
func UsersUpdate(c *gin.Context) {
	cur, ok := currentUser(c)
	if !ok || cur.Role != 1 {
		c.String(http.StatusForbidden, "需要管理员权限")
		return
	}
	id := parseUint(c.PostForm("id"))
	if id == 0 {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if v := strings.TrimSpace(c.PostForm("name")); v != "" {
		updates["name"] = v
	}
	if v := strings.TrimSpace(c.PostForm("mail")); v != "" {
		updates["mail"] = v
	}
	if v := strings.TrimSpace(c.PostForm("role")); v != "" {
		r := atoiDefault(v, 2)
		// 不能把自己降级
		if id == cur.ID && r != 1 {
			c.String(http.StatusBadRequest, "不能取消自己的管理员权限")
			return
		}
		updates["role"] = r
	}
	if v := strings.TrimSpace(c.PostForm("status")); v != "" {
		st := atoiDefault(v, 1)
		if id == cur.ID && st != 1 {
			c.String(http.StatusBadRequest, "不能停用自己的账号")
			return
		}
		updates["status"] = st
	}
	if pw := c.PostForm("password"); pw != "" {
		if len(pw) < 6 {
			c.String(http.StatusBadRequest, "密码至少 6 位")
			return
		}
		updates["password"] = cmn.PasswordEncryption(pw)
	}
	if len(updates) > 0 {
		if err := global.Db.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

// UsersDelete 删除账号
func UsersDelete(c *gin.Context) {
	cur, ok := currentUser(c)
	if !ok || cur.Role != 1 {
		c.String(http.StatusForbidden, "需要管理员权限")
		return
	}
	id := parseUint(c.Param("id"))
	if id == cur.ID {
		c.String(http.StatusBadRequest, "不能删除自己")
		return
	}
	// 至少保留一个管理员
	var adminCount int64
	global.Db.Model(&models.User{}).Where("role = 1 AND status = 1").Count(&adminCount)
	var target models.User
	if err := global.Db.Where("id = ?", id).First(&target).Error; err != nil {
		c.String(http.StatusNotFound, "账号不存在")
		return
	}
	if target.Role == 1 && adminCount <= 1 {
		c.String(http.StatusBadRequest, "至少保留一个管理员")
		return
	}

	tx := global.Db.Begin()
	for _, m := range []interface{}{&models.ItemIcon{}, &models.ItemIconGroup{}, &models.Bookmark{},
		&models.Notepad{}, &models.UserConfig{}, &models.File{}} {
		if err := tx.Where("user_id = ?", id).Delete(m).Error; err != nil {
			tx.Rollback()
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := tx.Where("id = ?", id).Delete(&models.User{}).Error; err != nil {
		tx.Rollback()
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	tx.Commit()
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

// ---- Docker 操作 ----

// DockerAction 容器启停重启
func DockerAction(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok || user.Role != 1 {
		c.String(http.StatusForbidden, "需要管理员权限")
		return
	}
	if global.DockerClient == nil {
		c.String(http.StatusServiceUnavailable, "Docker 不可用")
		return
	}
	id := strings.TrimSpace(c.PostForm("id"))
	action := strings.TrimSpace(c.PostForm("action"))
	if id == "" || action == "" {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}
	ctx, cancel := contextWithTimeout(10 * time.Second)
	defer cancel()
	var err error
	switch action {
	case "start":
		err = global.DockerClient.StartContainer(ctx, id)
	case "stop":
		err = global.DockerClient.StopContainer(ctx, id)
	case "restart":
		err = global.DockerClient.RestartContainer(ctx, id)
	default:
		c.String(http.StatusBadRequest, "未知操作")
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

// ---- 小工具 ----

func atoiDefault(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func atofDefault(s string, def float64) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}
