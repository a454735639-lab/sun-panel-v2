package router

import (
	"net/http"
	"sun-panel/global"

	// "sun-panel/router/admin"
	"sun-panel/router/openness"
	"sun-panel/router/panel"
	"sun-panel/router/system"
	"sun-panel/webrender"

	"github.com/gin-gonic/gin"
)

// 初始化总路由
func InitRouters(addr string) error {
	router := gin.Default()
	router.Use(webrender.Gzip())
	router.Use(webrender.StaticCache(604800))
	rootRouter := router.Group("/")
	routerGroup := rootRouter.Group("api")
	routerGroup.Use(Cors()) // Apply CORS to all /api routes

	// 接口
	system.Init(routerGroup)
	system.InitPingRouter(rootRouter)
	panel.Init(routerGroup)
	openness.Init(routerGroup)

	// 服务端渲染页面 + htmx 端点 + 静态资源
	InitSSR(router)

	global.Logger.Info("Sun-Panel is Started.  Listening and serving HTTP on ", addr)
	return router.Run(addr)
}

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
			c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization, Token, token, x-token")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
			c.Header("Access-Control-Allow-Credentials", "true")
			// Private Network Access support
			c.Header("Access-Control-Allow-Private-Network", "true")
		}

		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
