package panel

import (
	"sun-panel/api/api_v1"
	"sun-panel/api/api_v1/middleware"

	"github.com/gin-gonic/gin"
)

// InitUrlRule 地址匹配规则的路由。
//
// 这些规则只影响"打开卡片时用哪个地址"，属个人配置，
// 一律要求登录（与 SSR 层 pages/rules.go 的鉴权一致）。
func InitUrlRule(router *gin.RouterGroup) {
	api := api_v1.ApiGroupApp.ApiPanel.UrlRule
	r := router.Group("", middleware.LoginInterceptor)
	{
		r.POST("/panel/urlRule/list", api.List)
		r.POST("/panel/urlRule/edit", api.Edit)
		r.POST("/panel/urlRule/deletes", api.Deletes)
		r.POST("/panel/urlRule/test", api.Test)
	}
}