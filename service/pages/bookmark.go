package pages

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"sun-panel/global"
	"sun-panel/models"
	"sun-panel/models/datatype"
	"sun-panel/webrender"

	"github.com/gin-gonic/gin"
)

type BookmarkNode struct {
	models.Bookmark
	Icon   datatype.ItemIconIconInfo
	Childs []BookmarkNode
}

// buildBookmarkTree 组装树形书签（限深 3 层，防脏数据死循环）
func buildBookmarkTree(list []models.Bookmark, parentId uint, depth int) []BookmarkNode {
	out := []BookmarkNode{}
	if depth > 3 {
		return out
	}
	for _, b := range list {
		if uint(b.ParentId) != parentId {
			continue
		}
		n := BookmarkNode{Bookmark: b}
		if b.IconJson != "" {
			_ = json.Unmarshal([]byte(b.IconJson), &n.Icon)
		}
		if b.IsFolder == 1 {
			n.Childs = buildBookmarkTree(list, b.ID, depth+1)
		}
		out = append(out, n)
	}
	return out
}

// Bookmarks 书签页
func Bookmarks(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	var list []models.Bookmark
	if global.Db != nil {
		global.Db.Where("user_id = ?", user.ID).Order("sort asc, id asc").Find(&list)
	}
	tree := buildBookmarkTree(list, 0, 0)
	webrender.Render(c, http.StatusOK, "bookmarks.html", gin.H{
		"Title":     "书签",
		"SiteTitle": siteTitle(),
		"Theme":     themeMode(c),
		"BodyClass": "bookmarks-page",
		"LoggedIn":  true,
		"Nodes":     tree,
		"Total":     len(list),
		"Panel":     loadPanelConfig(user.ID),
	})
}

// BookmarkForm 书签/文件夹编辑弹窗
func BookmarkForm(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id, _ := strconv.Atoi(c.Query("id"))
	isFolder := c.Query("isFolder") == "1"
	parentId, _ := strconv.Atoi(c.Query("parentId"))

	b := models.Bookmark{IsFolder: 0, ParentId: uint(parentId), UserId: user.ID}
	if isFolder {
		b.IsFolder = 1
	}
	if id > 0 && global.Db != nil {
		global.Db.Where("id = ? AND user_id = ?", id, user.ID).First(&b)
	}
	webrender.RenderPartial(c, "bookmark_form.html", gin.H{
		"Bookmark": b,
		"IsNew":    id == 0,
		"IsFolder": b.IsFolder == 1,
	})
}

// BookmarkSave 保存书签/文件夹
func BookmarkSave(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id, _ := strconv.Atoi(c.PostForm("id"))
	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		c.String(http.StatusBadRequest, "名称不能为空")
		return
	}
	url := strings.TrimSpace(c.PostForm("url"))
	parentId, _ := strconv.Atoi(c.PostForm("parentId"))
	isFolder := 0
	if c.PostForm("isFolder") == "1" {
		isFolder = 1
	}

	if id > 0 {
		global.Db.Model(&models.Bookmark{}).Where("id = ? AND user_id = ?", id, user.ID).
			Updates(map[string]interface{}{"title": title, "url": url})
	} else {
		var maxSort int
		global.Db.Model(&models.Bookmark{}).Where("user_id = ?", user.ID).
			Select("COALESCE(MAX(sort),0)").Scan(&maxSort)
		b := models.Bookmark{
			Title: title, Url: url, IsFolder: isFolder,
			ParentId: uint(parentId), Sort: maxSort + 1, UserId: user.ID,
		}
		global.Db.Create(&b)
	}
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

// BookmarkDelete 删除（文件夹递归）
func BookmarkDelete(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id := parseUint(c.Param("id"))
	delRecursive(user.ID, id, 0)
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

func delRecursive(userId, id uint, depth int) {
	if depth > 3 || global.Db == nil {
		return
	}
	var children []models.Bookmark
	global.Db.Where("user_id = ? AND parent_id = ?", userId, id).Find(&children)
	for _, ch := range children {
		delRecursive(userId, ch.ID, depth+1)
	}
	global.Db.Where("id = ? AND user_id = ?", id, userId).Delete(&models.Bookmark{})
}

// ---- 便签 ----

// Notepad 便签页
func Notepad(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	notes := []models.Notepad{}
	if global.Db != nil {
		global.Db.Where("user_id = ?", user.ID).Order("updated_at desc").Find(&notes)
	}
	webrender.Render(c, http.StatusOK, "notepad.html", gin.H{
		"Title":     "便签",
		"SiteTitle": siteTitle(),
		"Theme":     themeMode(c),
		"BodyClass": "notepad-page",
		"LoggedIn":  true,
		"Notes":     notes,
		"Panel":     loadPanelConfig(user.ID),
	})
}

// NoteForm 便签编辑弹窗
func NoteForm(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id, _ := strconv.Atoi(c.Query("id"))
	n := models.Notepad{UserID: user.ID}
	if id > 0 && global.Db != nil {
		global.Db.Where("id = ? AND user_id = ?", id, user.ID).First(&n)
	}
	webrender.RenderPartial(c, "bookmark_form.html", gin.H{
		"Note":   n,
		"IsNew":  id == 0,
		"IsNote": true,
	})
}

// NoteSave 保存便签
func NoteSave(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id, _ := strconv.Atoi(c.PostForm("id"))
	title := strings.TrimSpace(c.PostForm("title"))
	content := c.PostForm("content")
	if title == "" {
		title = "无标题"
	}
	if id > 0 {
		global.Db.Model(&models.Notepad{}).Where("id = ? AND user_id = ?", id, user.ID).
			Updates(map[string]interface{}{"title": title, "content": content})
	} else {
		global.Db.Create(&models.Notepad{Title: title, Content: content, UserID: user.ID})
	}
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}

// NoteDelete 删除便签
func NoteDelete(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id := parseUint(c.Param("id"))
	global.Db.Where("id = ? AND user_id = ?", id, user.ID).Delete(&models.Notepad{})
	c.Header("HX-Refresh", "true")
	c.String(http.StatusOK, "")
}
