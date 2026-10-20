package pages

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ctxWithHeaders 构造一个带指定请求头的 gin.Context，用于测 token 提取。
func ctxWithHeaders(h map[string]string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/docker/container/getContainerStates", nil)
	for k, v := range h {
		req.Header.Set(k, v)
	}
	c.Request = req
	return c
}

// TestResolveRequestToken 鉴权来源的优先级。
//
// 这个测试存在的意义：本文件下的接口原先只认 token 头与 Cookie，
// 是纯 SSR 时期的设计。Vue 前端统一发 Authorization: Bearer，
// 导致 /api/docker/container/getContainerStates 对它返回 401 ——
// 表现为卡片上的容器状态灯始终拿不到数据。
//
// 兼容 Bearer 后要防止两件事：
//   - 后续有人"简化"掉 Bearer 分支，Vue 再次登录不上；
//   - 优先级被改乱（例如 Cookie 抢先于显式头），
//     造成"浏览器里有旧 Cookie 时新 token 不生效"这类诡异问题。
func TestResolveRequestToken(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{
			name:    "Bearer 标准写法",
			headers: map[string]string{"Authorization": "Bearer abc123"},
			want:    "abc123",
		},
		{
			name:    "Bearer 小写 scheme",
			headers: map[string]string{"Authorization": "bearer abc123"},
			want:    "abc123",
		},
		{
			name:    "Bearer 大写 scheme",
			headers: map[string]string{"Authorization": "BEARER abc123"},
			want:    "abc123",
		},
		{
			name:    "Bearer 前后多余空格",
			headers: map[string]string{"Authorization": "Bearer   abc123  "},
			want:    "abc123",
		},
		{
			name:    "token 头（原版前端）",
			headers: map[string]string{"token": "tk-1"},
			want:    "tk-1",
		},
		{
			name:    "Token 头（大小写变体）",
			headers: map[string]string{"Token": "tk-2"},
			want:    "tk-2",
		},
		{
			name:    "无任何凭据",
			headers: map[string]string{},
			want:    "",
		},
		{
			name:    "Authorization 非 Bearer 格式时忽略",
			headers: map[string]string{"Authorization": "Basic dXNlcjpwYXNz"},
			want:    "",
		},
		{
			name:    "Bearer 后为空则视为无凭据",
			headers: map[string]string{"Authorization": "Bearer   "},
			want:    "",
		},
		{
			// 优先级：Bearer 优先于 token 头。
			// 两者同时存在通常意味着前端在过渡期，应以更明确的 Bearer 为准。
			name: "Bearer 优先于 token 头",
			headers: map[string]string{
				"Authorization": "Bearer from-bearer",
				"token":         "from-token-header",
			},
			want: "from-bearer",
		},
		{
			// 优先级：显式头优先于 Cookie。
			// 若 Cookie 抢先，会出现"浏览器残留旧 Cookie 导致新 token 失效"。
			name: "token 头优先于 Cookie",
			headers: map[string]string{
				"token": "from-header",
				"Cookie": SessionCookieName + "=from-cookie",
			},
			want: "from-header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveRequestToken(ctxWithHeaders(tt.headers))
			if got != tt.want {
				t.Errorf("resolveRequestToken() = %q, 期望 %q（headers=%v）",
					got, tt.want, tt.headers)
			}
		})
	}
}

// TestResolveRequestTokenCookie 单独覆盖 Cookie 来源。
// 上面的用例都在验证"优先级"，Cookie 作为最低优先级需要单独的正面用例，
// 否则若 Cookie 分支被误删，上面所有用例仍会通过。
func TestResolveRequestTokenCookie(t *testing.T) {
	c := ctxWithHeaders(map[string]string{
		"Cookie": SessionCookieName + "=cookie-token",
	})
	got := resolveRequestToken(c)
	if got != "cookie-token" {
		t.Errorf("仅带 Cookie 时应取到 cookie-token，实际 %q", got)
	}
}