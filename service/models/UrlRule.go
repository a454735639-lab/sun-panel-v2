package models

import (
	"gorm.io/gorm"
)

// UrlRule 地址匹配规则。
//
// 用途：替代"lan/wan 二选一"的粗糙切换。
// 场景举例："这个站点内网走 A、外网走 B、另一域名走 C"。
//
// 匹配语义（与原版 v2 对齐）：
//   - 每行一条规则，按 Sort 升序**逐条**尝试
//   - Pattern 支持通配符 *（匹配任意字符）与端口（:8080）
//   - 命中第一条即返回其 Target；全部不中则回落到内置的 IP 判断
type UrlRule struct {
	BaseModel
	Pattern     string         `gorm:"type:varchar(500)" json:"pattern"`     // 匹配模式，如 *.example.com:8080
	Target      string         `gorm:"type:varchar(1000)" json:"target"`     // 命中后使用的地址模板
	Description string         `gorm:"type:varchar(200)" json:"description"` // 备注
	Enabled     int            `gorm:"type:tinyint(1);default:1" json:"enabled"`
	Sort        int            `gorm:"type:int(11)" json:"sort"`
	UserId      uint           `json:"userId"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 与现有模型保持单数表名约定
func (UrlRule) TableName() string { return "url_rule" }
