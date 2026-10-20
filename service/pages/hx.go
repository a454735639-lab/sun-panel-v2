package pages

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sun-panel/global"
	"sun-panel/lib/dockerApi"
	"sun-panel/models"
	"sun-panel/models/datatype"
	"sun-panel/webrender"

	"github.com/gin-gonic/gin"
)

// ---- 卡片编辑 ----

// loadContainerList 供编辑器下拉
func loadContainerList() []dockerApi.ContainerInfo {
	if global.DockerClient == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	list, err := global.DockerClient.ListContainers(ctx)
	if err != nil {
		return nil
	}
	return list
}

func cardEditorData(user models.User, item models.ItemIcon, isNew bool) gin.H {
	unmarshalIcon(item.IconJson, &item)
	// 小组件配置拆出来给模板用
	wc := map[string]string{}
	if strings.TrimSpace(item.WidgetConfig) != "" {
		_ = json.Unmarshal([]byte(item.WidgetConfig), &wc)
	}
	dockerID := ""
	if p := item.GetDockerExpandParam(); p != nil {
		dockerID = p.ContainerId
	}
	return gin.H{
		"Card":              item,
		"IsNew":             isNew,
		"Groups":            groupList(user.ID),
		"ContainerList":     loadContainerList(),
		"DockerAvailable":   global.DockerClient != nil,
		"DockerContainerID": dockerID,
		"WidgetType":        item.WidgetType,
		"WidgetUrl":         wc["url"],
		"WidgetUsername":    wc["username"],
		"WidgetPassword":    wc["password"],
		"WidgetApikey":      wc["apikey"],
	}
}

// CardNew 新建卡片弹窗
func CardNew(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	gid, _ := strconv.Atoi(c.Query("groupId"))
	item := models.ItemIcon{
		ItemIconGroupId: gid,
		OpenMethod:      2,
		Icon:            datatype.ItemIconIconInfo{ItemType: 1, Text: "", BackgroundColor: "#2a2a6b6b"},
	}
	webrender.RenderPartial(c, "card_editor.html", cardEditorData(user, item, true))
}

// CardEdit 编辑卡片弹窗
func CardEdit(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var item models.ItemIcon
	if err := global.Db.Where("id = ? AND user_id = ?", id, user.ID).First(&item).Error; err != nil {
		c.String(http.StatusNotFound, "卡片不存在")
		return
	}
	webrender.RenderPartial(c, "card_editor.html", cardEditorData(user, item, false))
}

// CardSave 保存卡片
func CardSave(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id, _ := strconv.Atoi(c.PostForm("id"))
	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		c.String(http.StatusBadRequest, "卡片名称不能为空")
		return
	}
	url := strings.TrimSpace(c.PostForm("url"))
	lanUrl := strings.TrimSpace(c.PostForm("lanUrl"))
	desc := strings.TrimSpace(c.PostForm("description"))
	iconSrc := strings.TrimSpace(c.PostForm("iconSrc"))
	openMethod, _ := strconv.Atoi(c.PostForm("openMethod"))
	if openMethod == 0 {
		openMethod = 2
	}
	cardType, _ := strconv.Atoi(c.PostForm("cardType"))
	gid, _ := strconv.Atoi(c.PostForm("itemIconGroupId"))
	containerId := strings.TrimSpace(c.PostForm("containerId"))

	// 图标
	icon := datatype.ItemIconIconInfo{ItemType: 1, Text: firstRune(title), BackgroundColor: "#2a2a6b6b"}
	if iconSrc != "" {
		if strings.HasPrefix(iconSrc, "data:image") {
			icon = datatype.ItemIconIconInfo{ItemType: 2, Src: iconSrc, BackgroundColor: "#2a2a6b6b"}
		} else if strings.Contains(iconSrc, ":") && !strings.Contains(iconSrc, "://") && !strings.HasPrefix(iconSrc, "/") {
			// iconify 名，如 material-symbols:home
			icon = datatype.ItemIconIconInfo{ItemType: 3, Text: iconSrc, BackgroundColor: "#2a2a6b6b"}
		} else {
			icon = datatype.ItemIconIconInfo{ItemType: 2, Src: iconSrc, BackgroundColor: "#2a2a6b6b"}
		}
	}
	iconJson, _ := json.Marshal(icon)

	// Docker 扩展参数
	expandParam := ""
	if cardType == 2 && containerId != "" {
		p := map[string]string{"containerId": containerId}
		for _, ct := range loadContainerList() {
			if ct.ID == containerId {
				p["containerName"] = ct.Name
				break
			}
		}
		b, _ := json.Marshal(p)
		expandParam = string(b)
	}

	// 小组件
	widgetType := strings.TrimSpace(c.PostForm("widgetType"))
	widgetConfig := ""
	if widgetType != "" {
		wc := map[string]string{
			"url":      strings.TrimSpace(c.PostForm("widgetUrl")),
			"username": strings.TrimSpace(c.PostForm("widgetUsername")),
			"password": strings.TrimSpace(c.PostForm("widgetPassword")),
			"apikey":   strings.TrimSpace(c.PostForm("widgetApikey")),
		}
		b, _ := json.Marshal(wc)
		widgetConfig = string(b)
	}

	if id > 0 {
		err := global.Db.Model(&models.ItemIcon{}).
			Where("id = ? AND user_id = ?", id, user.ID).
			Updates(map[string]interface{}{
				"title":              title,
				"url":                url,
				"lan_url":            lanUrl,
				"description":        desc,
				"icon_json":          string(iconJson),
				"open_method":        openMethod,
				"card_type":          cardType,
				"item_icon_group_id": gid,
				"expand_param":       expandParam,
				"widget_type":        widgetType,
				"widget_config":      widgetConfig,
			}).Error
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		var maxSort int
		global.Db.Model(&models.ItemIcon{}).Where("user_id = ? AND item_icon_group_id = ?", user.ID, gid).
			Select("COALESCE(MAX(sort),0)").Scan(&maxSort)
		item := models.ItemIcon{
			Title:           title,
			Url:             url,
			LanUrl:          lanUrl,
			Description:     desc,
			IconJson:        string(iconJson),
			OpenMethod:      openMethod,
			CardType:        cardType,
			ItemIconGroupId: gid,
			ExpandParam:     expandParam,
			WidgetType:      widgetType,
			WidgetConfig:    widgetConfig,
			Sort:            maxSort + 1,
			UserId:          user.ID,
		}
		if err := global.Db.Create(&item).Error; err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

// CardDelete 删除卡片
func CardDelete(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	if err := global.Db.Where("id = ? AND user_id = ?", id, user.ID).
		Delete(&models.ItemIcon{}).Error; err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

// CardSort 卡片拖拽排序
func CardSort(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	var req struct {
		Ids []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Ids) == 0 {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}
	tx := global.Db.Begin()
	for i, id := range req.Ids {
		if err := tx.Model(&models.ItemIcon{}).
			Where("id = ? AND user_id = ?", id, user.ID).
			Update("sort", i+1).Error; err != nil {
			tx.Rollback()
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	tx.Commit()
	c.String(http.StatusOK, "")
}

// ---- 分组 ----

// GroupNew 新建分组弹窗
func GroupNew(c *gin.Context) {
	if _, ok := currentUser(c); !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	webrender.RenderPartial(c, "group_editor.html", gin.H{
		"IsNew":     true,
		"Group":     models.ItemIconGroup{},
		"CardStyle": GroupCardStyle{Style: 1, TextColor: "#ffffff"},
	})
}

// GroupEdit 编辑分组弹窗
func GroupEdit(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var g models.ItemIconGroup
	if err := global.Db.Where("id = ? AND user_id = ?", id, user.ID).First(&g).Error; err != nil {
		c.String(http.StatusNotFound, "分组不存在")
		return
	}
	webrender.RenderPartial(c, "group_editor.html", gin.H{
		"IsNew":     false,
		"Group":     g,
		"CardStyle": parseGroupCardStyle(g.CardStyle),
	})
}

// GroupSave 保存分组
func GroupSave(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id, _ := strconv.Atoi(c.PostForm("id"))
	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		c.String(http.StatusBadRequest, "分组名称不能为空")
		return
	}
	icon := strings.TrimSpace(c.PostForm("icon"))

	cardStyle := `{"style":1,"textColor":"#ffffff","textInfoHideDescription":false,"textIconHideTitle":false}`
	if v := strings.TrimSpace(c.PostForm("cardStyle")); v != "" {
		st, _ := strconv.Atoi(v)
		if st < 0 || st > 2 {
			st = 1
		}
		cardStyle = fmt.Sprintf(`{"style":%d,"textColor":"#ffffff","textInfoHideDescription":false,"textIconHideTitle":false}`, st)
	}

	if id > 0 {
		if err := global.Db.Model(&models.ItemIconGroup{}).
			Where("id = ? AND user_id = ?", id, user.ID).
			Updates(map[string]interface{}{
				"title":      title,
				"icon":       icon,
				"card_style": cardStyle,
			}).Error; err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		var maxSort int
		global.Db.Model(&models.ItemIconGroup{}).Where("user_id = ?", user.ID).
			Select("COALESCE(MAX(sort),0)").Scan(&maxSort)
		g := models.ItemIconGroup{
			Title:     title,
			Icon:      icon,
			CardStyle: cardStyle,
			Sort:      maxSort + 1,
			UserId:    user.ID,
		}
		if err := global.Db.Create(&g).Error; err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

// GroupDelete 删除分组（有卡片时拒绝）
func GroupDelete(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var cnt int64
	global.Db.Model(&models.ItemIcon{}).Where("item_icon_group_id = ? AND user_id = ?", id, user.ID).Count(&cnt)
	if cnt > 0 {
		c.String(http.StatusBadRequest, fmt.Sprintf("分组内还有 %d 张卡片，请先移走或删除", cnt))
		return
	}
	if err := global.Db.Where("id = ? AND user_id = ?", id, user.ID).
		Delete(&models.ItemIconGroup{}).Error; err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

// GroupSort 分组拖拽排序
func GroupSort(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	var req struct {
		Ids []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Ids) == 0 {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}
	tx := global.Db.Begin()
	for i, id := range req.Ids {
		if err := tx.Model(&models.ItemIconGroup{}).
			Where("id = ? AND user_id = ?", id, user.ID).
			Update("sort", i+1).Error; err != nil {
			tx.Rollback()
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}
	tx.Commit()
	c.String(http.StatusOK, "")
}
