package wgt

import "strings"

// TypeInfo 小组件类型元信息
type TypeInfo struct {
	Value  string   `json:"value"`
	Label  string   `json:"label"`
	Fields []string `json:"fields"`
	Hint   string   `json:"hint"`
}

// Types 全部可用类型
func Types() []TypeInfo {
	return []TypeInfo{
		{Value: "", Label: "无"},
		{Value: "qbittorrent", Label: "qBittorrent", Fields: []string{"url", "apikey", "username", "password"},
			Hint: "地址填 qB Web 界面根地址；推荐填 API Key（qB 4.6+ 的 WebUI\\APIKey）"},
		{Value: "transmission", Label: "Transmission", Fields: []string{"url", "username", "password"},
			Hint: "地址填 Transmission 根地址，如 http://192.168.3.20:9091"},
		{Value: "emby", Label: "Emby", Fields: []string{"url", "apikey"},
			Hint: "在 Emby 后台「API 密钥」页生成"},
		{Value: "jellyfin", Label: "Jellyfin", Fields: []string{"url", "apikey"},
			Hint: "在 Jellyfin 控制台「API 密钥」页生成"},
	}
}

// Run 按类型执行
func Run(kind string, cfg map[string]string) *Result {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "qbittorrent":
		return QBittorrent(cfg)
	case "transmission":
		return Transmission(cfg)
	case "emby":
		return Emby(cfg)
	case "jellyfin":
		return Jellyfin(cfg)
	default:
		return errResult("未知的小组件类型：%s", kind)
	}
}
