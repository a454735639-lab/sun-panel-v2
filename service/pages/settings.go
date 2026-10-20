package pages

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sun-panel/global"
	"sun-panel/lib/dockerApi"
	"sun-panel/models"
	"sun-panel/webrender"

	"github.com/gin-gonic/gin"
)

var settingsTabs = map[string]bool{
	"export-import": true, "style": true, "docker": true, "scan": true,
	"gallery": true, "users": true, "user": true, "about": true,
}

// Settings 系统应用页
func Settings(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	tab := strings.TrimSpace(c.Param("tab"))
	if tab == "" {
		tab = "export-import"
	}
	if !settingsTabs[tab] {
		tab = "export-import"
	}

	data := gin.H{
		"Title":     "系统应用",
		"SiteTitle": siteTitle(),
		"Theme":     themeMode(c),
		"BodyClass": "settings-page",
		"Tab":       tab,
		"LoggedIn":  true,
		"User":      user,
		"Panel":     loadPanelConfig(user.ID),
	}

	switch tab {
	case "style":
		// Panel 已在上面注入

	case "docker":
		data["DockerAvailable"] = global.DockerClient != nil
		if global.DockerClient != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			list, _ := global.DockerClient.ListContainers(ctx)
			running := 0
			for _, ct := range list {
				if ct.State == "running" {
					running++
				}
			}
			ids := make([]string, 0, len(list))
			for _, ct := range list {
				ids = append(ids, ct.ID)
			}
			states := global.DockerClient.GetContainerStates(ctx, ids, true)
			sm := map[string]interface{}{}
			byID := map[string]*dockerApi.ContainerState{}
			for i := range states {
				st := states[i]
				sm[st.ID] = st
				byID[st.ID] = &st
			}
			// 回填到 ContainerInfo，模板可直接取 .CPUPercent / .MemUsage
			for i := range list {
				if st, ok := byID[list[i].ID]; ok {
					list[i].CPUPercent = st.CPUPercent
					list[i].MemUsage = st.MemUsage
				}
			}
			data["Containers"] = list
			data["Running"] = running
			data["States"] = sm
		}

	case "gallery":
		files := []models.File{}
		if global.Db != nil {
			global.Db.Where("user_id = ?", user.ID).Order("id desc").Limit(500).Find(&files)
		}
		data["Files"] = files

	case "users":
		users := []models.User{}
		if global.Db != nil {
			global.Db.Order("id asc").Find(&users)
		}
		data["Users"] = users

	case "user":
		// User 已注入

	case "scan":
		data["DockerAvailable"] = global.DockerClient != nil
		if global.DockerClient != nil {
			cands := buildScanCandidates(c, user.ID)
			grouped := map[string][]ScanCandidate{}
			for _, cd := range cands {
				g := cd.Group
				if g == "" {
					g = "未分组"
				}
				grouped[g] = append(grouped[g], cd)
			}
			names := make([]string, 0, len(grouped))
			for g := range grouped {
				names = append(names, g)
			}
			sortStrings(names)
			data["Candidates"] = cands
			data["Grouped"] = grouped
			data["GroupNames"] = names
			data["GroupOptions"] = groupOptions(user.ID)
		}

	case "export-import", "about":
		var groups int64
		var cards int64
		if global.Db != nil {
			global.Db.Model(&models.ItemIconGroup{}).Where("user_id = ?", user.ID).Count(&groups)
			global.Db.Model(&models.ItemIcon{}).Where("user_id = ?", user.ID).Count(&cards)
		}
		data["Groups"] = groups
		data["Cards"] = cards
		data["Stats"] = gin.H{"Groups": groups, "Cards": cards}
	}

	webrender.Render(c, http.StatusOK, "settings.html", data)
}

// parseUint 解析无符号整数
func parseUint(s string) uint {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	if n < 0 {
		return 0
	}
	return uint(n)
}
