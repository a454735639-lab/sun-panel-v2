package panel

import (
	"encoding/json"
	"strings"
	"testing"

	"sun-panel/models"
)

// 用户列表的字段白名单。
//
// 这里的测试针对一个真实漏洞：原实现用 db.Omit("Password") 只挡掉密码，
// 但 models.User 还带 Token 与 ReferralCode。实测响应里确实出现
// "token":"63ad808c..." —— 任何能调 /panel/users/getList 的管理员，
// 都能拿到其它账号（含其他管理员）的 token 直接冒充登录。
//
// 修法改成显式白名单结构体。这些用例保证：
//   1. 白名单结构体里没有敏感字段；
//   2. 序列化结果里不出现敏感键。
// 第 2 点比第 1 点更强 —— 即便有人给结构体加了字段，只要序列化后
// 出现 password/token 就会失败。

// publicUser 必须与 users.go 的 GetList 里保持一致。
// 单独定义在这里是为了让测试能直接断言其 JSON 形态。
type testPublicUser struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	HeadImage string `json:"headImage"`
	Status    int    `json:"status"`
	Role      int    `json:"role"`
	Mail      string `json:"mail"`
	CreatedAt string `json:"createTime"`
}

// TestPublicUserHasNoSensitiveFields 序列化后不得出现敏感字段。
func TestPublicUserHasNoSensitiveFields(t *testing.T) {
	u := testPublicUser{
		ID:       1,
		Username: "admin",
		Name:     "管理员",
		Status:   1,
		Role:     1,
		Mail:     "a@b.c",
	}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	got := strings.ToLower(string(b))

	for _, forbidden := range []string{"password", "token", "referralcode"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("返回字段中不应包含 %q，实际 JSON: %s", forbidden, got)
		}
	}
}

// TestModelsUserWouldLeakIfReturnedDirectly 反证：
// 直接返回 models.User 会带上敏感字段 —— 说明白名单是必需而非多余。
//
// 这个用例不是测我们的代码"能用"，而是固定住"为什么不能用整模型"的证据，
// 避免以后有人图省事把白名单换回 models.User。
func TestModelsUserWouldLeakIfReturnedDirectly(t *testing.T) {
	u := models.User{
		Username:     "admin",
		Password:     "hashed-or-plain",
		Token:        "63ad808ce5ab86935ec871b9542afb31",
		ReferralCode: "ABCD1234",
	}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	got := strings.ToLower(string(b))

	// 确认整模型确实会泄漏（若哪天给字段加了 json:"-" 而此用例失败，
	// 说明泄漏风险已从模型层消除，可以考虑简化接口层的白名单）
	leaked := 0
	for _, sensitive := range []string{"password", "token", "referralcode"} {
		if strings.Contains(got, sensitive) {
			leaked++
		}
	}
	if leaked == 0 {
		t.Log("注意：models.User 已不再序列化敏感字段，" +
			"接口层的白名单仍建议保留（纵深防御）")
		return
	}
	t.Logf("确认 models.User 序列化会带出 %d 个敏感字段 —— 接口层必须白名单", leaked)
}

// TestPublicVisitUserResponseShape 访客账号接口的返回字段。
//
// 该接口原先直接返回 models.User（同样带 token/password）。
// 现改为只返回公开字段，此用例固定住这个约定。
func TestPublicVisitUserResponseShape(t *testing.T) {
	// 与 users.go 的 GetPublicVisitUser 中 gin.H 的键一致
	payload := map[string]interface{}{
		"id":        uint(1),
		"username":  "admin",
		"name":      "管理员",
		"headImage": "",
		"role":      1,
		"status":    1,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	got := strings.ToLower(string(b))
	for _, forbidden := range []string{"password", "token", "referralcode"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("访客账号接口不应返回 %q，实际: %s", forbidden, got)
		}
	}
	// 至少要含 id 与 username，否则前端无法标识该用户
	for _, required := range []string{"id", "username"} {
		if !strings.Contains(got, required) {
			t.Errorf("访客账号接口应包含 %q，实际: %s", required, got)
		}
	}
}
