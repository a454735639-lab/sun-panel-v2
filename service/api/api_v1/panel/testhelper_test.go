package panel

import (
	"os"
	"path/filepath"
	"testing"
)

// readSource 读取同目录下的 Go 源码，供"结构断言"类测试使用。
//
// 为什么读源码而不是跑集成测试：
// 这些断言守的是"某个字段有没有被写进更新白名单"这类结构性约束。
// 跑集成测试需要数据库与完整请求链路，成本高；
// 而这类约束一旦被破坏，后果是**静默的**（接口返回成功但值没变），
// 值得用最轻量的方式在 CI 里常驻检查。
//
// 局限：源码被重命名或大幅重构时测试会失败 —— 这是可接受的，
// 因为那时本来就需要人重新审视这些约束。
func readSource(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(".", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v（该测试需在 service/api/api_v1/panel 目录下运行）", p, err)
	}
	return string(b)
}
