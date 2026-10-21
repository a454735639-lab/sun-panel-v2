package pages

import (
	"context"
	"net/http"
	"time"

	"sun-panel/global"
	"sun-panel/lib/dockerApi"

	"github.com/gin-gonic/gin"
)

// DockerStates 卡片容器的实时状态查询端点。
//
// 背景：webrender/static/app.js 的 pollDocker() 一直轮询
// POST /api/docker/container/getContainerStates，但该路由从未在后端注册过，
// 导致容器状态灯（绿/黄/红）、CPU/RAM 数字与「N / M 个容器运行中」始终拿不到数据，
// 且前端 catch 静默，用户无感知。此处理器补齐该端点。
//
// 约定：用 POST + JSON，与 /api/widget/batch 保持一致。
// 返回结构保持 app.js 现有的 res.data.list 约定，避免改动前端解析逻辑。
func DockerStates(c *gin.Context) {
	if _, ok := currentUser(c); !ok {
		// 这里刻意返回 JSON 错误码（HTTP 200 + code 1000），
		// 而不是本文件同层其它端点用的裸 HTTP 401。
		//
		// 原因：本端点**同时被两套前端调用** ——
		//   - SSR 版 app.js 用原生 fetch，非 2xx 时静默跳过，两种写法都行；
		//   - Vue 版走 axios，非 2xx 会进入错误拦截器 → failHandler。
		//     虽然前端已给轮询加了 silent，但未登录时返回 401 仍属"异常状态码"，
		//     与 API 层（apiReturn.ErrorByCode(c, 1000)）的约定不一致，
		//     会让调用方难以统一处理。
		// 统一成 JSON 错误码后，前端可据 code=1000 走"跳登录"分支，
		// 不产生任何错误提示 —— 与其它面板接口行为一致。
		c.JSON(http.StatusOK, gin.H{
			"code": 1000,
			"msg":  "Not logged in yet",
		})
		return
	}

	// 未挂载 docker.sock 时视为「无容器」，返回空列表而不是报错：
	// 面板本身仍应正常可用。
	//
	// 同时显式带上 dockerReady=false —— 前端据此区分
	// 「真的没有容器」与「根本读不到 Docker」。
	// 只返回空列表会让前端无法分辨：横幅会永远显示「状态获取中」，
	// 用户以为再等等就有，实际上永远不会。
	if global.DockerClient == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
			"list":        []any{},
			"dockerReady": false,
		}})
		return
	}

	var req struct {
		ContainerIds []string `json:"containerIds"`
		WithStats    bool     `json:"withStats"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "参数错误"})
		return
	}
	if len(req.ContainerIds) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
			"list":        []any{},
			"dockerReady": true,
		}})
		return
	}
	// 上限保护：单次请求最多查 60 个容器，避免被构造超长列表拖慢面板。
	if len(req.ContainerIds) > 60 {
		req.ContainerIds = req.ContainerIds[:60]
	}

	// 与 pages/base.go 中既有的调用方式保持一致：8 秒超时。
	// GetContainerStates 内部已用 8 并发信号量限制，这里只负责整体超时。
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	states := global.DockerClient.GetContainerStates(ctx, req.ContainerIds, req.WithStats)
	if states == nil {
		states = []dockerApi.ContainerState{}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"list":        states,
		"dockerReady": true,
	}})
}
