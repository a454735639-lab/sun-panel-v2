package pages

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"sun-panel/global"
	"sun-panel/lib/dockerApi"
	"sun-panel/models"
	"sun-panel/models/datatype"

	"github.com/gin-gonic/gin"
)

// SessionCookieName 会话 cookie 名
const SessionCookieName = "sunpanel_token"

// ThemeCookie 主题偏好 cookie
const ThemeCookie = "sunpanel_theme"

// NetModeCookie 内外网模式 cookie：auto / lan / wan
const NetModeCookie = "sunpanel_netmode"

// CardVM 卡片视图模型
type CardVM struct {
	ID                uint
	Title             string
	Description       string
	OpenUrl           string
	WanUrl            string
	LanUrl            string
	OpenMethod        int
	CardType          int
	WidgetType        string
	Icon              datatype.ItemIconIconInfo
	Initial           string
	DockerContainerID string
	ContainerState    *dockerApi.ContainerState
}

// GroupVM 分组视图模型
type GroupVM struct {
	ID        uint
	Title     string
	Icon      string
	CardStyle GroupCardStyle
	Items     []CardVM
}

// ---- 认证 ----

// currentUser 从请求中解析当前用户（先看 token 头，再回落 cookie）
func currentUser(c *gin.Context) (models.User, bool) {
	token := strings.TrimSpace(c.GetHeader("token"))
	if token == "" {
		if v, err := c.Cookie(SessionCookieName); err == nil {
			token = strings.TrimSpace(v)
		}
	}
	if token == "" {
		return models.User{}, false
	}
	if global.UserToken != nil {
		if u, ok := global.UserToken.Get(token); ok && u.ID != 0 {
			return u, true
		}
	}
	if global.CUserToken != nil {
		if bt, ok := global.CUserToken.Get(token); ok && bt != "" {
			if u, ok2 := global.UserToken.Get(bt); ok2 && u.ID != 0 {
				return u, true
			}
		}
	}
	// 回落到数据库查 token
	if global.Db != nil {
		var u models.User
		if err := global.Db.Where("token = ?", token).First(&u).Error; err == nil && u.ID != 0 {
			if global.UserToken != nil {
				global.UserToken.Set(u.Token, u, 24*time.Hour)
			}
			return u, true
		}
	}
	return models.User{}, false
}

// ---- 主题 / 内外网 ----

func themeMode(c *gin.Context) string {
	if v, err := c.Cookie(ThemeCookie); err == nil {
		switch v {
		case "light", "dark":
			return v
		}
	}
	return "auto"
}

func netMode(c *gin.Context) string {
	if v, err := c.Cookie(NetModeCookie); err == nil {
		switch v {
		case "lan", "wan", "auto":
			return v
		}
	}
	return "auto"
}

// siteTitle 站点标题
func siteTitle() string {
	if global.SystemSetting != nil {
		var v string
		if err := global.SystemSetting.GetValueByInterface("system_setting_site_title", &v); err == nil {
			if strings.TrimSpace(v) != "" {
				return v
			}
		}
	}
	return "Sun-Panel"
}

// ---- 网络判定 ----

// getClientIP 获取客户端真实 IP（优先代理头）
func getClientIP(c *gin.Context) string {
	if v := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); v != "" {
		if i := strings.Index(v, ","); i > 0 {
			v = v[:i]
		}
		if ip := net.ParseIP(strings.TrimSpace(v)); ip != nil {
			return ip.String()
		}
	}
	if v := strings.TrimSpace(c.GetHeader("X-Real-IP")); v != "" {
		if ip := net.ParseIP(v); ip != nil {
			return ip.String()
		}
	}
	if v := c.ClientIP(); v != "" {
		if ip := net.ParseIP(v); ip != nil {
			return ip.String()
		}
		return v
	}
	return ""
}

// isLanIP 判断是否属于局域网地址。
//
// 注意：不包含 100.64.0.0/10（CGNAT / 组网工具网段）——
// 虚拟组网成员物理上可能在任何地方，把 100.64/10 判为内网会导致
// 外网用户拿到内网地址而打不开。
func isLanIP(s string) bool {
	ip := net.ParseIP(strings.TrimSpace(s))
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		switch {
		case v4[0] == 10:
			return true
		case v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31:
			return true
		case v4[0] == 192 && v4[1] == 168:
			return true
		}
		return false
	}
	// IPv6 ULA fc00::/7
	return ip[0]&0xfe == 0xfc
}

// isNatLoopback 判断是否 NAT 回环：客户端 IP 与本机公网 IP 相同
func isNatLoopback(clientIP, serverPublicIP string) bool {
	if serverPublicIP == "" || clientIP == "" {
		return false
	}
	return clientIP == serverPublicIP
}

// pickUrl 按客户端网络环境选择打开地址
func pickUrl(c *gin.Context, wan, lan string) string {
	wan = strings.TrimSpace(wan)
	lan = strings.TrimSpace(lan)
	if lan == "" {
		return wan
	}
	if wan == "" {
		return lan
	}

	switch netMode(c) {
	case "lan":
		return lan
	case "wan":
		return wan
	}

	clientIP := getClientIP(c)
	global.ServerPublicIPMutex.RLock()
	pubIP := global.ServerPublicIP
	global.ServerPublicIPMutex.RUnlock()
	if isLanIP(clientIP) || isNatLoopback(clientIP, pubIP) {
		return lan
	}
	return wan
}

// ---- 数据加载 ----

func firstRune(s string) string {
	for _, r := range s {
		return string(r)
	}
	return "?"
}

func unmarshalIcon(iconJson string, item *models.ItemIcon) {
	if strings.TrimSpace(iconJson) == "" {
		return
	}
	_ = json.Unmarshal([]byte(iconJson), &item.Icon)
}

// loadGroups 读取用户的全部分组与卡片
func loadGroups(c *gin.Context, userId uint) ([]GroupVM, error) {
	if global.Db == nil {
		return nil, nil
	}
	var groups []models.ItemIconGroup
	if err := global.Db.Where("user_id = ?", userId).
		Order("sort asc, id asc").Find(&groups).Error; err != nil {
		return nil, err
	}
	var icons []models.ItemIcon
	if err := global.Db.Where("user_id = ?", userId).
		Order("sort asc, id asc").Find(&icons).Error; err != nil {
		return nil, err
	}

	byGroup := map[uint][]CardVM{}
	for _, it := range icons {
		unmarshalIcon(it.IconJson, &it)
		vm := CardVM{
			ID:          it.ID,
			Title:       it.Title,
			Description: it.Description,
			WanUrl:      it.Url,
			LanUrl:      it.LanUrl,
			OpenUrl:     pickUrl(c, it.Url, it.LanUrl),
			OpenMethod:  it.OpenMethod,
			CardType:    it.CardType,
			WidgetType:  it.WidgetType,
			Icon:        it.Icon,
			Initial:     firstRune(it.Title),
		}
		if p := it.GetDockerExpandParam(); p != nil {
			vm.DockerContainerID = p.ContainerId
		}
		gid := uint(it.ItemIconGroupId)
		byGroup[gid] = append(byGroup[gid], vm)
	}

	out := make([]GroupVM, 0, len(groups))
	for _, g := range groups {
		out = append(out, GroupVM{
			ID:        g.ID,
			Title:     g.Title,
			Icon:      g.Icon,
			CardStyle: parseGroupCardStyle(g.CardStyle),
			Items:     byGroup[g.ID],
		})
	}
	return out, nil
}

// attachContainerStates 给 Docker 卡片附加实时状态（并发）
func attachContainerStates(groups []GroupVM) {
	if global.DockerClient == nil {
		return
	}
	ids := []string{}
	for _, g := range groups {
		for _, it := range g.Items {
			if it.DockerContainerID != "" {
				ids = append(ids, it.DockerContainerID)
			}
		}
	}
	if len(ids) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	states := global.DockerClient.GetContainerStates(ctx, ids, true)
	if len(states) == 0 {
		return
	}
	m := map[string]*dockerApi.ContainerState{}
	for i := range states {
		s := states[i]
		m[s.ID] = &s
	}
	for gi := range groups {
		for ci := range groups[gi].Items {
			if st, ok := m[groups[gi].Items[ci].DockerContainerID]; ok {
				groups[gi].Items[ci].ContainerState = st
			}
		}
	}
}

// groupList 已有分组（供下拉，含 ID）
func groupList(userId uint) []models.ItemIconGroup {
	out := []models.ItemIconGroup{}
	if global.Db == nil {
		return out
	}
	global.Db.Where("user_id = ?", userId).Order("sort asc, id asc").Find(&out)
	return out
}

// groupOptions 已有分组名（供下拉）
func groupOptions(userId uint) []string {
	out := []string{}
	if global.Db == nil {
		return out
	}
	var gs []models.ItemIconGroup
	global.Db.Where("user_id = ?", userId).Order("sort asc").Find(&gs)
	for _, g := range gs {
		out = append(out, g.Title)
	}
	return out
}

// sortStrings 排序辅助
func sortStrings(s []string) []string {
	sort.Strings(s)
	return s
}

// writeNoContent 返回 htmx 常用空响应
func writeNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// ensureDir 创建目录
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// cctx 后台 context
func cctx() context.Context { return context.Background() }

// timeNow 当前时间
func timeNow() time.Time { return time.Now() }

var _ = sync.Mutex{}
