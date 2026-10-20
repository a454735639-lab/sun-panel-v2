package router

import (
	"os"
	"path/filepath"
	"strings"

	"sun-panel/global"

	"github.com/gin-gonic/gin"
)

// VueWebRoot 是 Vue 前端构建产物（dist）在容器/磁盘上的位置。
//
// 命名沿用原版约定：原版把 vite 产物放在 ./web，由
// router.StaticFile("/", webPath+"/index.html") 直接托管在根路径。
// 现在改为挂在 /v2/ 前缀下，与 SSR 版共存（见 InitVueWeb）。
const VueWebRoot = "./web"

// VueBasePath 是 Vue 前端对外暴露的 URL 前缀。
//
// 为什么用前缀而不是根路径：项目里同时有两套前端 ——
// SSR 版占据 / 、/login 、/settings 等；Vue 版用 hash 路由
// （createWebHashHistory），前端内部跳转靠 #/... 处理，
// 服务端只需托管一个入口页，因此给它一个独立前缀即可零冲突共存。
//
// 若将来决定以 Vue 为主，把 SSR 路由摘掉、此处前缀改为 "/" 即可。
var VueBasePath = "/v2"

// InitVueWeb 注册 Vue 前端的静态托管。
//
// 未构建（./web 不存在）时静默跳过 —— 例如纯 SSR 部署场景，
// 不应因为缺少 Vue 产物而影响服务启动。
func InitVueWeb(r *gin.Engine) {
	base := VueBasePath // 形如 "/v2"，不带结尾斜杠

	indexFile := filepath.Join(VueWebRoot, "index.html")
	if _, err := os.Stat(indexFile); err != nil {
		if global.Logger != nil {
			global.Logger.Infof(
				"[VueWeb] 未找到 %s，跳过 Vue 前端托管（仅提供 SSR 版）", indexFile)
		}
		return
	}

	// 用 RouterGroup 而不是在 Engine 上逐个注册：
	// gin 的 Engine.Use 只对"之后"注册的路由生效，
	// 用 group 才能保证下面所有路由都在同一组中间件之内。
	g := r.Group(base)

	// 缓存策略：build 产物文件名带内容 hash，可长缓存；
	// 而入口页会随构建变化，必须禁缓存，否则资源 hash 与页面不符会白屏。
	// 注意 SSR 版已有的 webrender.StaticCache 只覆盖 /static/ 前缀，
	// 不会作用于 /v2/，所以这里单独处理。
	g.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, base+"/assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else if !strings.Contains(lastSegment(c.Request.URL.Path), ".") {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		c.Next()
	})

	// 入口页 —— /v2 与 /v2/ 都要能打开，否则少打一个斜杠会 404
	sendIndex := func(c *gin.Context) { c.File(indexFile) }
	g.GET("", sendIndex)
	g.GET("/", sendIndex)

	// 构建产物目录
	g.Static("/assets", filepath.Join(VueWebRoot, "assets"))
	g.Static("/static", filepath.Join(VueWebRoot, "static"))

	// 产物根目录下散落的图标等文件。
	// 用静态清单而非通配，避免误吞 /v2 下其它路径。
	for _, f := range []string{
		"favicon.ico", "favicon.svg", "favicon-black.svg",
		"logo.png", "manifest.webmanifest", "sw.js",
	} {
		if _, err := os.Stat(filepath.Join(VueWebRoot, f)); err == nil {
			g.StaticFile("/"+f, filepath.Join(VueWebRoot, f))
		}
	}

	// 兜底：/v2 下的未知非资源路径交给入口页。
	// Vue 用 hash 路由，前端路由不出现在服务端 URL 中，
	// 这里主要覆盖手输路径或旧链接。
	//
	// 注意 NoRoute 只能注册在 Engine 上（RouterGroup 没有该方法），
	// 所以这里用 r.NoRoute 并自行判断路径前缀。
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == base || strings.HasPrefix(p, base+"/") {
			if !strings.Contains(lastSegment(p), ".") {
				c.File(indexFile)
				return
			}
		}
		c.Status(404)
	})

	if global.Logger != nil {
		global.Logger.Infof("[VueWeb] Vue 前端已挂载于 %s/（产物目录 %s）", base, VueWebRoot)
	}
}

// lastSegment 取 URL 路径的最后一段，用于判断是否像静态资源（含点号）。
func lastSegment(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}