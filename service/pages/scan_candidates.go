package pages

import (
	"net/http"

	"sun-panel/global"

	"github.com/gin-gonic/gin"
)

// DockerScanCandidates 扫描容器并返回候选卡片的 JSON 版。
//
// 为什么需要它：扫描导入的逻辑原先只在 SSR 层（pages/scan.go 的
// buildScanCandidates + DockerScanImport，走表单 + htmx），Vue 前端
// 无法调用。这里补一个 JSON 端点，但**复用同一份 buildScanCandidates**，
// 不重复实现 —— 分类规则与排除规则只有一处维护。
//
// 与 SSR 版的差异：只做"扫描预览"，不在这里写库。
// 让前端拿到候选列表、由用户勾选后再逐条调用既有的
// /panel/itemIcon/edit 创建卡片 —— 这样导入逻辑只有一条路径，
// 也不会因扫描时误点而在服务端产生副作用。
func DockerScanCandidates(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		// 与 DockerStates 一致：返回 JSON 错误码而非裸 401，
		// 让 Vue 的 axios 能按统一约定处理。
		c.JSON(http.StatusOK, gin.H{
			"code": 1000,
			"msg":  "Not logged in yet",
		})
		return
	}

	// 未挂载 docker.sock 时不算错误，返回空列表 + 说明，
	// 让前端能给出"未检测到 Docker"的提示而不是报错。
	if global.DockerClient == nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"list":        []ScanCandidate{},
				"dockerReady": false,
			},
		})
		return
	}

	list := buildScanCandidates(c, user.ID)
	if list == nil {
		list = []ScanCandidate{}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"list":        list,
			"dockerReady": true,
		},
	})
}
