package router

import (
	"net/http"

	"sun-panel/pages"
	"sun-panel/webrender"

	"github.com/gin-gonic/gin"
)

// InitSSR 注册服务端渲染页面与 htmx 端点
func InitSSR(rootRouter *gin.Engine) {
	// 静态资源
	_ = webrender.RegisterStatic(rootRouter, "/static")
	rootRouter.GET("/uploads/*filepath", gin.WrapH(http.StripPrefix("/uploads/", http.FileServer(http.Dir("/app/uploads")))))

	// 页面
	rootRouter.GET("/", pages.Panel)
	rootRouter.GET("/healthz", pages.Healthz)
	rootRouter.GET("/login", pages.LoginPage)
	rootRouter.POST("/login", pages.LoginSubmit)
	rootRouter.POST("/logout", pages.Logout)
	rootRouter.GET("/logout", pages.Logout)
	rootRouter.GET("/search", pages.Search)
	rootRouter.GET("/bookmarks", pages.Bookmarks)
	rootRouter.GET("/notepad", pages.Notepad)
	rootRouter.GET("/settings", pages.Settings)
	rootRouter.GET("/settings/:tab", pages.Settings)

	hx := rootRouter.Group("/hx")
	{
		// 卡片
		hx.GET("/card/new", pages.CardNew)
		hx.GET("/card/:id/edit", pages.CardEdit)
		hx.POST("/card/save", pages.CardSave)
		hx.POST("/card/:id/delete", pages.CardDelete)
		hx.POST("/card/sort", pages.CardSort)

		// 分组
		hx.GET("/group/new", pages.GroupNew)
		hx.GET("/group/:id/edit", pages.GroupEdit)
		hx.POST("/group/save", pages.GroupSave)
		hx.POST("/group/:id/delete", pages.GroupDelete)
		hx.POST("/group/sort", pages.GroupSort)

		// 图标
		hx.GET("/icon/picker", pages.IconPicker)
		hx.POST("/icon/upload", pages.IconUpload)

		// 内外网 / 主题
		hx.POST("/netmode/:mode", pages.SetNetMode)
		hx.POST("/theme/:mode", pages.SetTheme)

		// 设置
		hx.POST("/style/save", pages.StyleSave)
		hx.GET("/export", pages.Export)
		hx.POST("/import", pages.Import)
		hx.POST("/user/save", pages.UserSave)

		// 地址规则
		hx.POST("/rule/save", pages.RuleSave)
		hx.GET("/rule/:id/edit", pages.RuleEdit)
		hx.POST("/rule/:id/delete", pages.RuleDelete)

		// 图库
		hx.POST("/gallery/upload", pages.GalleryUpload)
		hx.POST("/gallery/:id/delete", pages.GalleryDelete)

		// 账号
		hx.POST("/users/create", pages.UsersCreate)
		hx.POST("/users/update", pages.UsersUpdate)
		hx.POST("/users/:id/delete", pages.UsersDelete)

		// Docker
		hx.POST("/docker/action", pages.DockerAction)
		hx.POST("/docker/scan-import", pages.DockerScanImport)

		// 书签 / 便签
		hx.GET("/bookmark/form", pages.BookmarkForm)
		hx.POST("/bookmark/save", pages.BookmarkSave)
		hx.POST("/bookmark/:id/delete", pages.BookmarkDelete)
		hx.GET("/note/form", pages.NoteForm)
		hx.POST("/note/save", pages.NoteSave)
		hx.POST("/note/:id/delete", pages.NoteDelete)

		// 分组卡片局部刷新
		hx.GET("/group/:id/cards", pages.RenderGroupCards)
	}

	// 小组件 API
	api := rootRouter.Group("/api")
	{
		api.POST("/widget/batch", pages.WidgetBatch)
		api.GET("/widget/types", pages.WidgetTypes)

		// 容器实时状态：app.js 的 pollDocker() 依赖此端点。
		// 路径与前端现有调用保持一致（/api/docker/container/getContainerStates），
		// 不改前端即可修复状态灯与 CPU/RAM 显示。
		api.POST("/docker/container/getContainerStates", pages.DockerStates)
	}
}
