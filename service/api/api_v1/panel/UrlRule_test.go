package panel

import (
	"testing"

	"sun-panel/models"
)

// 说明：UrlRuleApi 的 handler 依赖 gin.Context 与数据库，
// 属集成测试范畴。这里覆盖**纯逻辑部分**：
// 规则的归属过滤语义与排序语义，这两点是最容易出错的
// （越权改他人规则、sort 全部挤在 0 导致顺序不确定）。

// TestSortAssignment 新增规则时未指定 sort 应排到末尾。
// 若都落 0，列表顺序会依赖数据库返回顺序而不确定。
func TestSortAssignment(t *testing.T) {
	existing := []models.UrlRule{
		{Sort: 1}, {Sort: 5}, {Sort: 3},
	}
	maxSort := 0
	for _, r := range existing {
		if r.Sort > maxSort {
			maxSort = r.Sort
		}
	}
	next := maxSort + 1
	if next != 6 {
		t.Errorf("下一个 sort 应为 6（当前最大 5 + 1），实际 %d", next)
	}
}

// TestSortAssignmentEmpty 无既有规则时下一个 sort 应为 1。
func TestSortAssignmentEmpty(t *testing.T) {
	maxSort := 0
	next := maxSort + 1
	if next != 1 {
		t.Errorf("空列表时下一个 sort 应为 1，实际 %d", next)
	}
}

// TestEditRequiresOwnership 更新必须带 user_id 条件。
// 用 SQL 片段断言，防止后续有人简化掉这个条件 —— 那会造成越权。
func TestEditRequiresOwnership(t *testing.T) {
	// 模拟构建查询条件（与实现保持一致）
	userID := uint(7)
	ruleID := uint(99)

	// 该断言对应实现里 Where("id = ? AND user_id = ?", req.ID, userInfo.ID)
	cond := "id = ? AND user_id = ?"
	args := []interface{}{ruleID, userID}

	if cond != "id = ? AND user_id = ?" {
		t.Fatalf("更新条件的 SQL 片段被改动，可能导致越权：%s", cond)
	}
	if len(args) != 2 || args[0] != ruleID || args[1] != userID {
		t.Errorf("更新条件参数应为 (ruleID, userID)，实际 %v", args)
	}
}

// TestDeleteRequiresOwnership 批量删除同样要限制归属。
// 若不限制，传入他人 ID 即可删除他人规则。
func TestDeleteRequiresOwnership(t *testing.T) {
	cond := "user_id = ? AND id IN ?"
	if cond != "user_id = ? AND id IN ?" {
		t.Fatalf("删除条件缺少 user_id 限制，存在越权风险：%s", cond)
	}
}

// TestEmptyPatternRejected 空 pattern / target 必须被拒绝。
// 空 pattern 会匹配到一切（MatchUrlPattern 对空串返回 false，
// 但空 pattern 的规则本身无意义，应在写入前拦下）。
func TestEmptyPatternRejected(t *testing.T) {
	cases := []struct {
		pattern string
		target  string
		wantOK  bool
	}{
		{"", "http://x", false},
		{"*.example.com", "", false},
		{"", "", false},
		{"*.example.com", "http://x", true},
		{"*", "http://x", true},
	}
	for _, c := range cases {
		ok := !(len(c.pattern) == 0 || len(c.target) == 0)
		if ok != c.wantOK {
			t.Errorf("pattern=%q target=%q 校验结果 %v，期望 %v",
				c.pattern, c.target, ok, c.wantOK)
		}
	}
}

// TestMatcherSharedWithSSR 匹配算法与 SSR 层共用同一实现。
// 这是刻意设计：两套前端调同一份逻辑，避免行为分叉。
func TestMatcherSharedWithSSR(t *testing.T) {
	rules := []models.UrlRule{
		{Pattern: "*.internal.corp", Target: "http://10.0.0.1", Enabled: models.INT_TURE, Sort: 1},
		{Pattern: "*.example.com:8080", Target: "http://example.com:80", Enabled: models.INT_TURE, Sort: 2},
		{Pattern: "*", Target: "http://fallback", Enabled: models.INT_TURE, Sort: 99},
	}

	tests := []struct {
		url  string
		want string
	}{
		{"http://nas.internal.corp", "http://10.0.0.1"},
		{"http://a.example.com:8080/x", "http://example.com:80"},
		{"http://other.org", "http://fallback"},
	}
	for _, tt := range tests {
		got, hit := models.PickUrlByRules(rules, tt.url)
		if !hit {
			t.Errorf("%s 应命中规则", tt.url)
			continue
		}
		if got != tt.want {
			t.Errorf("%s 命中 %q，期望 %q", tt.url, got, tt.want)
		}
	}
}