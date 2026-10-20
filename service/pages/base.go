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

// resolveRequestToken 按优先级从请求中提取 token。
//
// 抽成独立函数是为了可测试：鉴权来源的优先级容易在后续改动中被破坏，
// 而这类错误只在"某种前端登录不上"时才暴露，很难在生产中发现。
//
// 优先级：Authorization: Bearer → token 头 → Token 头 → Cookie。
// Bearer 排最前是因为它最明确（显式声明了 scheme），
// 其次是自定义头，最后才是 Cookie（浏览器会隐式携带，最不确定）。
func resolveRequestToken(c *gin.Context) string {
	// 1) Authorization: Bearer xxx（scheme 大小写不敏感，RFC 7235）
	if h := c.GetHeader("Authorization"); h != "" {
		if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
			if t := strings.TrimSpace(h[7:]); t != "" {
				return t
			}
		}
	}
	// 2) token 头（原版前端 / SSR htmx）
	if t := strings.TrimSpace(c.GetHeader("token")); t != "" {
		return t
	}
	// 3) Token 头（历史大小写变体）
	if t := strings.TrimSpace(c.GetHeader("Token")); t != "" {
		return t
	}
	// 4) Cookie（SSR 服务端会话；浏览器隐式携带）
	if v, err := c.Cookie(SessionCookieName); err == nil {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

// currentUser 从请求中解析当前用户。
//
// 支持三种 token 来源，见 resolveRequestToken。
// 之所以要兼容 Bearer：本文件里的接口（如 /api/docker/container/getContainerStates）
// 原先只认 token 头与 Cookie —— 那是 SSR 版的设计。Vue 前端统一发
// Authorization: Bearer，若不兼容则一律 401。
// 实测该端点因此对 Vue 不可用（卡片上的容器状态灯一直拿不到数据）。
func currentUser(c *gin.Context) (models.User, bool) {
	token := resolveRequestToken(c)
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

// loadUrlRules 读取用户的地址规则（按 sort 升序）。
// 出错或未配置时返回 nil，调用方回落内置 IP 判断。
func loadUrlRules(userId uint) []models.UrlRule {
	if global.Db == nil || userId == 0 {
		return nil
	}
	var rules []models.UrlRule
	if err := global.Db.Where("user_id = ? AND enabled = ?", userId, models.INT_TURE).
		Order("sort asc, id asc").Find(&rules).Error; err != nil {
		return nil
	}
	return rules
}

// pickerCtx 地址选择所需的上下文，**一次性算好**后在卡片循环里复用。
//
// 为什么需要它：pickUrl 原先在每张卡片的循环里被调用，
// 而它内部会 currentUser（可能回落到数据库查 token）+ 查一次规则表，
// 于是渲染 N 张卡片就产生 N 次多余的 SQL（N+1 查询），卡片多时面板明显变慢。
// 把这两个查询提到循环外只做一次即可。
type pickerCtx struct {
	mode   string           // 显式网络模式 lan/wan/auto
	rules  []models.UrlRule // 已按 sort 排好，可为空
	useLan bool             // 内置 IP 判断的结果
}

// newPickerCtx 在进入卡片循环前调用一次。
func newPickerCtx(c *gin.Context) pickerCtx {
	p := pickerCtx{mode: netMode(c)}

	// 内置 IP 判断（纯本地计算，无 IO）
	clientIP := getClientIP(c)
	global.ServerPublicIPMutex.RLock()
	pubIP := global.ServerPublicIP
	global.ServerPublicIPMutex.RUnlock()
	p.useLan = isLanIP(clientIP) || isNatLoopback(clientIP, pubIP)

	// 规则只查一次
	if u, ok := currentUser(c); ok {
		p.rules = loadUrlRules(u.ID)
	}
	return p
}

// pick 用预取好的上下文为单个站点选地址。
//
// 优先级：
//  1. 显式网络模式（lan/wan）—— 用户手动切换，最高优先
//  2. 地址规则匹配 —— 逐条匹配，命中即用
//  3. 内置 IP 判断 —— 无规则命中时的兜底，保持与改造前一致
func (p pickerCtx) pick(wan, lan string) string {
	wan = strings.TrimSpace(wan)
	lan = strings.TrimSpace(lan)
	if lan == "" {
		return wan
	}
	if wan == "" {
		return lan
	}

	switch p.mode {
	case "lan":
		return lan
	case "wan":
		return wan
	}

	// 地址规则：对内外网两个地址都尝试匹配
	// （用户写的 pattern 可能针对内网域名，也可能针对外网域名）
	if len(p.rules) > 0 {
		if t, hit := models.PickUrlByRules(p.rules, lan); hit {
			return t
		}
		if t, hit := models.PickUrlByRules(p.rules, wan); hit {
			return t
		}
	}

	if p.useLan {
		return lan
	}
	return wan
}

// pickUrl 单次选择地址。
//
// 仅供不在循环中的调用点使用（如搜索结果只处理少量条目）。
// 渲染卡片列表请改用 newPickerCtx + pickerCtx.pick，避免 N+1 查询。
func pickUrl(c *gin.Context, wan, lan string) string {
	return newPickerCtx(c).pick(wan, lan)
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
	// 在循环外一次性算好地址选择上下文，避免每张卡片重复查库（N+1）
	pk := newPickerCtx(c)

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
			OpenUrl:     pk.pick(it.Url, it.LanUrl),
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
