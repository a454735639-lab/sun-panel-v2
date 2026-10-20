package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// extractToken 从请求中取出客户端 token。
//
// 支持三种来源，按优先级：
//  1. Authorization: Bearer <token>   —— Vue 前端（axios 拦截器）用的方式
//  2. token 头                        —— 原版前端与 SSR 版 htmx 用的方式
//  3. sunpanel_token Cookie           —— SSR 版的服务端会话方式
//
// 为什么需要这个统一函数：
// 仓库里同时存在两套前端 —— 原有的 Vue SPA（发 Bearer）与后加的 SSR 版
// （发 token 头 / 用 Cookie）。三个拦截器原先各自只读 token 头，
// 导致 Vue 前端调 API 一律 401。这里集中解析，两套前端都能用。
//
// 注意：Bearer 前缀大小写不敏感（RFC 7235 规定 scheme 不区分大小写），
// 实际见过 `Bearer` / `bearer` 两种写法，都要能识别。
func extractToken(c *gin.Context) string {
	// 1) Authorization: Bearer xxx
	if h := c.GetHeader("Authorization"); h != "" {
		if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
			if t := strings.TrimSpace(h[7:]); t != "" {
				return t
			}
		}
	}

	// 2) token 头（原版方式）
	if t := strings.TrimSpace(c.GetHeader("token")); t != "" {
		return t
	}
	if t := strings.TrimSpace(c.GetHeader("Token")); t != "" {
		return t
	}

	// 3) Cookie（SSR 版会话）
	if v, err := c.Cookie("sunpanel_token"); err == nil {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}

	return ""
}