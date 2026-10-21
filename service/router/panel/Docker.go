package panel

import (
	"sun-panel/api/api_v1"
	"sun-panel/api/api_v1/middleware"

	"github.com/gin-gonic/gin"
)

// InitDocker 容器管理的路由。
//
// 与 SSR 层的 pages/settings_actions.go:DockerAction 并存：
//   - SSR 走表单 + HX-Refresh（htmx 场景）
//   - 这里是 JSON（Vue 场景）
// 两者共用 lib/dockerApi，能力不重复实现。
//
// 鉴权层级刻意不同：
//   - list 只要求登录 —— 卡片编辑器要"选择容器"，普通用户也得能看列表
//   - action 在处理器内部再校验 role==1 —— 启停容器会中断服务，仅管理员
func InitDocker(router *gin.RouterGroup) {
	api := api_v1.ApiGroupApp.ApiPanel.Docker
	r := router.Group("", middleware.LoginInterceptor)
	{
		r.POST("/panel/docker/list", api.List)
		r.POST("/panel/docker/action", api.Action)
	}
}
