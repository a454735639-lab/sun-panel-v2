package pages

import (
	"sun-panel/global"
)

// galleryLimit 图库单次查询上限。
// 500 是原先写死的值，抽成常量便于与提示文案保持一致。
const galleryLimit = 500

// ListMeta 列表截断的元信息，供模板提示用户"还有更多"。
//
// 背景：多处查询用 Limit(N) 硬截断（图库 500、便签 120、搜索 60/40），
// 但页面显示的是 len(结果) —— 一旦真实数据超过上限，
// 用户看到的数字就是**错的**（例如有 600 张图却显示"共 500 个"），
// 比"没有提示"更糟：它是误导。
//
// 用法：查询前先算 Meta，把 Meta 一并传给模板；
// 模板在 Truncated 为 true 时显示"仅显示前 N 条，共 M 条"。
type ListMeta struct {
	Shown     int  // 实际返回条数
	Total     int  // 数据库中的真实总数
	Limit     int  // 本次查询的上限
	Truncated bool // 是否因上限被截断
}

// NewListMeta 组装元信息。
func NewListMeta(shown, total, limit int) ListMeta {
	return ListMeta{
		Shown:     shown,
		Total:     total,
		Limit:     limit,
		Truncated: total > shown,
	}
}

// Remaining 还有多少条未显示（未截断时为 0）。
func (m ListMeta) Remaining() int {
	if m.Total <= m.Shown {
		return 0
	}
	return m.Total - m.Shown
}

// countRows 统计表里属于该用户的记录数。
// 出错时返回 0 —— 统计失败不应影响主查询结果，只是不显示总数提示。
func countRows(model interface{}, userId uint) int {
	if global.Db == nil || userId == 0 {
		return 0
	}
	var n int64
	if err := global.Db.Model(model).Where("user_id = ?", userId).Count(&n).Error; err != nil {
		return 0
	}
	return int(n)
}