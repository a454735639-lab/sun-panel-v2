package pages

import (
	"encoding/json"
	"strings"

	"sun-panel/global"
	"sun-panel/models"
)

// PanelConfig 面板外观配置，存于 user_config.panel_json
type PanelConfig struct {
	BackgroundImageSrc         string  `json:"backgroundImageSrc"`
	BackgroundBlur             int     `json:"backgroundBlur"`
	BackgroundMaskNumber       float64 `json:"backgroundMaskNumber"`
	ClockShow                  bool    `json:"clockShow"`
	ClockShowSecond            bool    `json:"clockShowSecond"`
	SearchBoxShow              bool    `json:"searchBoxShow"`
	SearchBoxBackground        string  `json:"searchBoxBackground"`
	SearchBoxTextColor         string  `json:"searchBoxTextColor"`
	SearchBoxBorderColor       string  `json:"searchBoxBorderColor"`
	SearchBoxSearchIcon        string  `json:"searchBoxSearchIcon"`
	MaxWidth                   int     `json:"maxWidth"`
	IconTextColor              string  `json:"iconTextColor"`
	TextColor                  string  `json:"textColor"`
	FooterHtml                 string  `json:"footerHtml"`
	NetModeChangeButtonShow    bool    `json:"netModeChangeButtonShow"`
	NetModeChangeAuto          bool    `json:"netModeChangeAuto"`
	NetModeChangeAutoMaxPingMs int     `json:"netModeChangeAutoMaxPingMs"`
}

func defaultPanelConfig() PanelConfig {
	return PanelConfig{
		BackgroundImageSrc:         "defaultBackground.jpg",
		BackgroundBlur:             0,
		BackgroundMaskNumber:       0,
		ClockShow:                  true,
		ClockShowSecond:            true,
		SearchBoxShow:              true,
		SearchBoxBackground:        "rgba(255,255,255,.14)",
		SearchBoxTextColor:         "#ffffff",
		SearchBoxBorderColor:       "rgba(255,255,255,.28)",
		MaxWidth:                   1200,
		IconTextColor:              "#ffffff",
		TextColor:                  "#ffffff",
		FooterHtml:                 "",
		NetModeChangeButtonShow:    true,
		NetModeChangeAuto:          true,
		NetModeChangeAutoMaxPingMs: 700,
	}
}

// loadPanelConfig 读取用户面板配置；缺失字段用默认值补齐
func loadPanelConfig(userId uint) PanelConfig {
	cfg := defaultPanelConfig()
	if global.Db == nil {
		return cfg
	}
	var uc models.UserConfig
	if err := global.Db.Where("user_id = ?", userId).First(&uc).Error; err != nil {
		return cfg
	}
	raw := strings.TrimSpace(uc.PanelJson)
	if raw == "" || raw == "null" {
		return cfg
	}
	_ = json.Unmarshal([]byte(raw), &cfg)
	// 修正零值
	if cfg.MaxWidth <= 0 {
		cfg.MaxWidth = 1200
	}
	if cfg.NetModeChangeAutoMaxPingMs <= 0 {
		cfg.NetModeChangeAutoMaxPingMs = 700
	}
	if cfg.BackgroundImageSrc == "" {
		cfg.BackgroundImageSrc = "defaultBackground.jpg"
	}
	return cfg
}

// GroupCardStyle 分组卡片样式
type GroupCardStyle struct {
	Style                   int    `json:"style"` // 0 详情横卡 / 1 图标方卡 / 2 大卡
	TextColor               string `json:"textColor"`
	TextInfoHideDescription bool   `json:"textInfoHideDescription"`
	TextIconHideTitle       bool   `json:"textIconHideTitle"`
}

// parseGroupCardStyle 解析分组样式 JSON，失败回落 style=1
func parseGroupCardStyle(raw string) GroupCardStyle {
	out := GroupCardStyle{Style: 1, TextColor: "#ffffff"}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}
