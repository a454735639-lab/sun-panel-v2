package pages

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"sun-panel/global"
	"sun-panel/models"
	"sun-panel/models/datatype"
	"sun-panel/webrender"

	"github.com/gin-gonic/gin"
)

// SearchHit 搜索结果项（字段与 search.html 对齐）
type SearchHit struct {
	Kind        string // card / bookmark / note
	ID          uint
	Title       string
	Description string
	GroupTitle  string
	Url         string
	Icon        struct {
		Icon datatype.ItemIconIconInfo
	}
}

// Search 搜索页
func Search(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	q := strings.TrimSpace(c.Query("q"))
	hits := []SearchHit{}
	engines := []models.SearchEngine{}

	if global.Db != nil {
		global.Db.Where("user_id = ?", user.ID).Order("sort asc").Find(&engines)
	}

	if q != "" {
		like := "%" + q + "%"
		// 卡片
		var cards []models.ItemIcon
		global.Db.Where("user_id = ? AND (title LIKE ? OR description LIKE ? OR url LIKE ?)", user.ID, like, like, like).
			Limit(60).Find(&cards)
		for i := range cards {
			it := cards[i]
			unmarshalIcon(it.IconJson, &it)
			h := SearchHit{
				Kind: "card", ID: it.ID, Title: it.Title, Description: it.Description,
				Url: pickUrl(c, it.Url, it.LanUrl),
			}
			h.Icon.Icon = it.Icon
			hits = append(hits, h)
		}
		// 书签
		var bms []models.Bookmark
		global.Db.Where("user_id = ? AND (title LIKE ? OR url LIKE ?)", user.ID, like, like).
			Limit(40).Find(&bms)
		for i := range bms {
			b := bms[i]
			h := SearchHit{Kind: "bookmark", ID: b.ID, Title: b.Title, Url: b.Url}
			if b.IconJson != "" {
				var ic datatype.ItemIconIconInfo
				if json.Unmarshal([]byte(b.IconJson), &ic) == nil {
					h.Icon.Icon = ic
				}
			}
			hits = append(hits, h)
		}
		// 便签
		var notes []models.Notepad
		global.Db.Where("user_id = ? AND (title LIKE ? OR content LIKE ?)", user.ID, like, like).
			Limit(40).Find(&notes)
		for _, n := range notes {
			desc := n.Content
			if len([]rune(desc)) > 80 {
				desc = string([]rune(desc)[:80]) + "…"
			}
			hits = append(hits, SearchHit{
				Kind: "note", ID: n.ID, Title: n.Title, Description: desc,
			})
		}
	}

	// 外部搜索引擎链接
	type EngineLink struct {
		Title string
		IconSrc string
		Href  string
	}
	links := make([]EngineLink, 0, len(engines))
	for _, e := range engines {
		u := e.Url
		if strings.Contains(u, "%s") {
			u = strings.ReplaceAll(u, "%s", url.QueryEscape(q))
		} else {
			u = strings.TrimRight(u, "/") + "/search?q=" + url.QueryEscape(q)
		}
		links = append(links, EngineLink{Title: e.Title, IconSrc: e.IconSrc, Href: u})
	}

	webrender.Render(c, http.StatusOK, "search.html", gin.H{
		"Title":     "搜索 " + q,
		"SiteTitle": siteTitle(),
		"Theme":     themeMode(c),
		"BodyClass": "search-page",
		"LoggedIn":  true,
		"Query":     q,
		"Results":   hits,
		"Engines":   links,
		"Panel":     loadPanelConfig(user.ID),
	})
}

var _ = json.Marshal
