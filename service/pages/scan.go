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

	"github.com/gin-gonic/gin"
)

// ScanCandidate 扫描出的候选卡片
type ScanCandidate struct {
	ContainerID string
	Name        string
	Image       string
	State       string
	Port        int
	LanUrl      string
	WanUrl      string
	Group       string
	IconUrl     string
	Exists      bool
	Excluded    bool
}

// serviceClass 按镜像名/容器名推断服务分类
func serviceClass(name, image string) string {
	s := strings.ToLower(name + " " + image)
	switch {
	case containsAny(s, "qbittorrent", "transmission", "aria2", "deluge", "rtorrent", "download"):
		return "下载"
	case containsAny(s, "emby", "jellyfin", "plex", "navidrome", "audiobookshelf", "subsonic"):
		return "媒体"
	case containsAny(s, "moviepilot", "nas-tools", "nastool", "jackett", "prowlarr", "radarr", "sonarr"):
		return "影音管理"
	case containsAny(s, "postgres", "mysql", "mariadb", "redis", "mongo", "adminer", "pgadmin"):
		return "数据库"
	case containsAny(s, "vaultwarden", "bitwarden", "passbolt", "keepass"):
		return "密码管理"
	case containsAny(s, "clash", "mihomo", "zashboard", "openwrt", "nginx", "traefik", "caddy", "adguard", "dns"):
		return "网络运维"
	case containsAny(s, "openai", "ollama", "agent", "gpt", "llm", "ai"):
		return "AI"
	case containsAny(s, "n8n", "node-red", "seltool", "watchtower", "cron", "auto"):
		return "自动化"
	case containsAny(s, "rss", "trendradar", "newsnow", "fresh", "reader"):
		return "资讯"
	case containsAny(s, "sun-panel", "sunpanel", "homepage", "dashboard", "nav"):
		return "导航面板"
	}
	return "未分组"
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

// isExcluded 面板/工具类容器默认不导入
func isExcluded(name, image string) bool {
	s := strings.ToLower(name + " " + image)
	return containsAny(s, "adminer", "pgadmin", "sun-panel", "sunpanel", "helper", "db-init", "init-db")
}

// buildScanCandidates 扫描容器并组装候选
func buildScanCandidates(c *gin.Context, userId uint) []ScanCandidate {
	if global.DockerClient == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	list, err := global.DockerClient.ListContainers(ctx)
	if err != nil {
		return nil
	}

	// 已有容器 ID
	existing := map[string]bool{}
	var icons []models.ItemIcon
	global.Db.Where("user_id = ?", userId).Find(&icons)
	for i := range icons {
		if p := icons[i].GetDockerExpandParam(); p != nil && p.ContainerId != "" {
			existing[p.ContainerId] = true
		}
	}

	host := c.Request.Host
	if h, _, found := strings.Cut(host, ":"); found {
		host = h
	}

	out := make([]ScanCandidate, 0, len(list))
	for _, ct := range list {
		if ct.PrimaryPort == 0 {
			continue
		}
		lan := fmt.Sprintf("http://%s:%d", host, ct.PrimaryPort)
		out = append(out, ScanCandidate{
			ContainerID: ct.ID,
			Name:        containerDisplayName(ct),
			Image:       ct.Image,
			State:       ct.State,
			Port:        ct.PrimaryPort,
			LanUrl:      lan,
			Group:       serviceClass(ct.Name, ct.Image),
			Exists:      existing[ct.ID],
			Excluded:    isExcluded(ct.Name, ct.Image),
		})
	}
	return out
}

// DockerScanImport 导入选中的容器为卡片
func DockerScanImport(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	var req struct {
		Items []struct {
			ContainerID string `json:"containerId"`
			Name        string `json:"name"`
			LanUrl      string `json:"lanUrl"`
			WanUrl      string `json:"wanUrl"`
			Group       string `json:"group"`
			IconUrl     string `json:"iconUrl"`
			Port        int    `json:"port"`
		} `json:"items"`
		SkipExisting bool `json:"skipExisting"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Items) == 0 {
		c.String(http.StatusBadRequest, "没有选中任何容器")
		return
	}

	existing := map[string]bool{}
	var icons []models.ItemIcon
	global.Db.Where("user_id = ?", user.ID).Find(&icons)
	for i := range icons {
		if p := icons[i].GetDockerExpandParam(); p != nil && p.ContainerId != "" {
			existing[p.ContainerId] = true
		}
	}

	groupID := map[string]uint{}
	var groups []models.ItemIconGroup
	global.Db.Where("user_id = ?", user.ID).Find(&groups)
	for _, g := range groups {
		groupID[g.Title] = g.ID
	}
	var maxSort int
	global.Db.Model(&models.ItemIconGroup{}).Where("user_id = ?", user.ID).
		Select("COALESCE(MAX(sort),0)").Scan(&maxSort)

	tx := global.Db.Begin()
	created, skipped := 0, 0
	for _, item := range req.Items {
		if req.SkipExisting && existing[item.ContainerID] {
			skipped++
			continue
		}
		gName := strings.TrimSpace(item.Group)
		if gName == "" {
			gName = "未分组"
		}
		gid, ok := groupID[gName]
		if !ok {
			maxSort++
			ng := models.ItemIconGroup{Title: gName, Sort: maxSort, UserId: user.ID,
				CardStyle: `{"style":1,"textColor":"#ffffff"}`}
			if err := tx.Create(&ng).Error; err != nil {
				tx.Rollback()
				c.String(http.StatusInternalServerError, "创建分组失败："+err.Error())
				return
			}
			gid = ng.ID
			groupID[gName] = gid
		}

		p := map[string]string{"containerId": item.ContainerID, "containerName": item.Name}
		pb, _ := json.Marshal(p)
		icon := fmt.Sprintf(`{"itemType":1,"src":"","text":%q,"backgroundColor":"#2a2a6b6b"}`, firstRune(item.Name))
		card := models.ItemIcon{
			Title:           item.Name,
			Url:             item.WanUrl,
			LanUrl:          item.LanUrl,
			IconJson:        icon,
			OpenMethod:      2,
			CardType:        2,
			ItemIconGroupId: int(gid),
			ExpandParam:     string(pb),
			Sort:            created + 1,
			UserId:          user.ID,
		}
		if err := tx.Create(&card).Error; err != nil {
			tx.Rollback()
			c.String(http.StatusInternalServerError, "创建卡片失败："+err.Error())
			return
		}
		created++
	}
	tx.Commit()
	c.String(http.StatusOK, fmt.Sprintf("导入 %d 张卡片，跳过 %d 张", created, skipped))
}

// listContainersForScan 供模板渲染
func listContainersForScan() []dockerApi.ContainerInfo {
	if global.DockerClient == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	list, _ := global.DockerClient.ListContainers(ctx)
	return list
}

// containerDisplayName 优先用 nav.name label
func containerDisplayName(ct dockerApi.ContainerInfo) string {
	if ct.NavName != "" {
		return ct.NavName
	}
	return ct.Name
}

var _ = strconv.Itoa
