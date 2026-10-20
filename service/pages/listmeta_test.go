package pages

import "testing"

// TestNewListMeta 列表截断元信息。
//
// 这个结构存在的意义是：原先模板显示 len(结果)，
// 数据超过查询上限时会给用户**错误的数字**（有 600 张却显示"共 500 个"）。
// 因此 Total 必须来自独立计数，Truncated 判定必须准确。
func TestNewListMeta(t *testing.T) {
	tests := []struct {
		name          string
		shown         int
		total         int
		limit         int
		wantTruncated bool
		wantRemaining int
	}{
		{"未达上限-不截断", 12, 12, 500, false, 0},
		{"少于上限-不截断", 3, 3, 500, false, 0},
		{"空数据-不截断", 0, 0, 500, false, 0},
		{"刚好等于上限-不截断", 500, 500, 500, false, 0},
		{"超过上限-截断", 500, 600, 500, true, 100},
		{"远超上限-截断", 500, 5000, 500, true, 4500},
		{"只多一条-截断", 500, 501, 500, true, 1},
		// 边界：total 小于 shown（理论上不应出现，但要保证不 panic、
		// 且 Remaining 不为负数）
		{"异常值-total小于shown", 10, 5, 500, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewListMeta(tt.shown, tt.total, tt.limit)
			if m.Shown != tt.shown {
				t.Errorf("Shown = %d, 期望 %d", m.Shown, tt.shown)
			}
			if m.Total != tt.total {
				t.Errorf("Total = %d, 期望 %d", m.Total, tt.total)
			}
			if m.Limit != tt.limit {
				t.Errorf("Limit = %d, 期望 %d", m.Limit, tt.limit)
			}
			if m.Truncated != tt.wantTruncated {
				t.Errorf("Truncated = %v, 期望 %v（shown=%d total=%d）",
					m.Truncated, tt.wantTruncated, tt.shown, tt.total)
			}
			if got := m.Remaining(); got != tt.wantRemaining {
				t.Errorf("Remaining() = %d, 期望 %d", got, tt.wantRemaining)
			}
			// 剩余数绝不能为负数
			if m.Remaining() < 0 {
				t.Errorf("Remaining() 为负: %d", m.Remaining())
			}
		})
	}
}

// TestListMeta_TruncatedImpliesMore 截断标志与剩余数必须自洽。
// 若显示"已截断"却提示 0 条未显示，是自相矛盾的文案，必须排除。
func TestListMeta_TruncatedImpliesMore(t *testing.T) {
	cases := [][3]int{
		{500, 600, 500},
		{100, 101, 100},
		{1, 9999, 1},
	}
	for _, c := range cases {
		m := NewListMeta(c[0], c[1], c[2])
		if m.Truncated && m.Remaining() <= 0 {
			t.Errorf("shown=%d total=%d 判定为截断，但 Remaining=%d，文案会自相矛盾",
				c[0], c[1], m.Remaining())
		}
	}
}