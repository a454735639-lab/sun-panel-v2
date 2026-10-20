package pages

import (
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

// RenderGroupCards htmx 局部刷新某个分组的卡片
func RenderGroupCards(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	gid, _ := strconv.Atoi(c.Param("id"))
	groups, err := loadGroups(c, user.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	attachContainerStates(groups)
	for _, g := range groups {
		if int(g.ID) == gid {
			webrender.RenderPartial(c, "group_cards.html", g)
			return
		}
	}
	webrender.RenderPartial(c, "group_cards.html", GroupVM{ID: uint(gid)})
}

// SetNetMode 切换内外网模式
func SetNetMode(c *gin.Context) {
	mode := strings.TrimSpace(c.Param("mode"))
	switch mode {
	case "auto", "lan", "wan":
	default:
		c.String(http.StatusBadRequest, "无效模式")
		return
	}
	c.SetCookie(NetModeCookie, mode, 86400*365, "/", "", false, false)
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

// SetTheme 切换主题
func SetTheme(c *gin.Context) {
	mode := strings.TrimSpace(c.Param("mode"))
	switch mode {
	case "auto", "light", "dark":
	default:
		c.String(http.StatusBadRequest, "无效模式")
		return
	}
	c.SetCookie(ThemeCookie, mode, 86400*365, "/", "", false, false)
	c.String(http.StatusOK, mode)
}

// IconUpload 上传图标
func IconUpload(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "未选择文件"})
		return
	}
	if fh.Size > 2<<20 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "文件超过 2MB"})
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "不支持的格式"})
		return
	}

	now := time.Now()
	rel := filepath.Join("uploads", now.Format("2006"), now.Format("01"), now.Format("02"))
	dir := filepath.Join("/app", rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		// 非容器环境回落当前目录
		dir = rel
		_ = os.MkdirAll(dir, 0o755)
	}
	name := fmt.Sprintf("%d%s", now.UnixNano(), ext)
	dst := filepath.Join(dir, name)

	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}

	url := "/" + filepath.ToSlash(filepath.Join(rel, name))
	if global.Db != nil {
		global.Db.Create(&models.File{Src: url, UserId: user.ID, FileName: fh.Filename, Ext: ext})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"url": url}})
}

// IconPicker 图标选择器弹窗
func IconPicker(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	files := []models.File{}
	if global.Db != nil {
		global.Db.Where("user_id = ?", user.ID).Order("id desc").Limit(120).Find(&files)
	}
	webrender.RenderPartial(c, "icon_picker.html", gin.H{"Files": files})
}

// UserSave 保存个人信息
func UserSave(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	mail := strings.TrimSpace(c.PostForm("mail"))
	updates := map[string]interface{}{}
	if name != "" {
		updates["name"] = name
	}
	if mail != "" {
		updates["mail"] = mail
	}
	if pw := c.PostForm("password"); pw != "" {
		if len(pw) < 6 {
			c.String(http.StatusBadRequest, "密码至少 6 位")
			return
		}
		updates["password"] = cmnPassword(pw)
	}
	if len(updates) > 0 && global.Db != nil {
		if err := global.Db.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

func cmnPassword(p string) string {
	return cmn.PasswordEncryption(p)
}
