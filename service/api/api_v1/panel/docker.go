package panel

import (
	"context"
	"strings"
	"time"

	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/api/api_v1/common/base"
	"sun-panel/global"
	"sun-panel/lib/dockerApi"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// DockerApi 容器管理的 JSON API。
//
// 为什么需要这一层：容器列表与启停原先只实现了 SSR 版
// （pages/settings_actions.go 的 DockerAction，走表单 + HX-Refresh），
// Vue 前端无法调用。后果是**卡片编辑器里根本没有"选择容器"的入口** ——
// 用户只能靠"扫描导入"批量生成卡片，无法给已有卡片绑定/更换容器，
// 也就看不到容器状态。
//
// 这里补一套 JSON 接口，让两套前端共用同一份 dockerApi 能力
// （列表在 lib/dockerApi，不重复实现）。
type DockerApi struct{}

// dockerReady 判断 Docker 是否可用。
//
// 单独抽出来是因为每个端点都要用，且"不可用"的返回语义要一致：
// 不是报错，而是**明确告知不可用**，让前端能显示恰当提示而不是空列表。
func dockerReady() bool {
	return global.DockerClient != nil
}

// List 返回全部容器（供"选择容器"下拉与 Docker 管理页使用）。
//
// withStats 为 true 时附带 CPU/内存。默认关闭 ——
// 读 cgroup 较慢，而"选择容器"只需要名称与状态。
func (a *DockerApi) List(c *gin.Context) {
	if _, ok := base.GetCurrentUserInfo(c); !ok {
		apiReturn.ErrorByCode(c, 1000)
		return
	}

	if !dockerReady() {
		// 明确告知不可用，而不是返回空列表。
		// 两者对前端的含义完全不同：空列表 = "确实没有容器"，
		// 不可用 = "读不到 Docker"。混为一谈会让用户以为容器全没了。
		apiReturn.SuccessData(c, gin.H{
			"list":        []any{},
			"dockerReady": false,
		})
		return
	}

	var req struct {
		WithStats bool `json:"withStats"`
	}
	_ = c.ShouldBindBodyWith(&req, binding.JSON)

	// 与 pages 层一致：10 秒超时
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	list, err := global.DockerClient.ListContainers(ctx)
	if err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}

	// 需要指标时再补一次状态查询（ListContainers 不含 CPU/内存）
	if req.WithStats && len(list) > 0 {
		ids := make([]string, 0, len(list))
		for _, it := range list {
			ids = append(ids, it.ID)
		}
		states := global.DockerClient.GetContainerStates(ctx, ids, true)
		byID := make(map[string]dockerApi.ContainerState, len(states))
		for _, s := range states {
			byID[s.ID] = s
		}
		for i := range list {
			if s, ok := byID[list[i].ID]; ok {
				list[i].CPUPercent = s.CPUPercent
				list[i].MemUsage = s.MemUsage
			}
		}
	}

	apiReturn.SuccessData(c, gin.H{
		"list":        list,
		"dockerReady": true,
		"count":       len(list),
	})
}

// Action 启停/重启容器。
//
// 限管理员：启停容器会直接影响 NAS 上运行的服务（可能中断其他用户的访问），
// 不属于"普通用户可自助"的操作。普通用户能看状态，但不能动。
func (a *DockerApi) Action(c *gin.Context) {
	userInfo, ok := base.GetCurrentUserInfo(c)
	if !ok {
		apiReturn.ErrorByCode(c, 1000)
		return
	}
	if userInfo.Role != 1 {
		// 1005 = 当前无权限操作（见 apiReturn/ErrorCode.go）
		apiReturn.ErrorByCode(c, 1005)
		return
	}

	// 先做参数校验，再检查 Docker 可用性。
	//
	// 顺序有讲究：若先判 Docker 不可用就直接返回，
	// 非法 action / 空 id 这类**调用方错误**会被掩盖成"Docker 不可用"，
	// 排查时容易被误导去查 docker.sock。
	var req struct {
		ID     string `json:"id"`
		Action string `json:"action"`
	}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	id := strings.TrimSpace(req.ID)
	action := strings.TrimSpace(req.Action)
	if id == "" {
		apiReturn.ErrorParamFomat(c, "id 不能为空")
		return
	}
	// 白名单：只接受这三种，避免把任意字符串透传给 docker
	if action != "start" && action != "stop" && action != "restart" {
		apiReturn.ErrorParamFomat(c, "未知操作")
		return
	}

	if !dockerReady() {
		apiReturn.SuccessData(c, gin.H{
			"ok":          false,
			"dockerReady": false,
			"msg":         "Docker 不可用",
		})
		return
	}

	// 10 秒超时：容器启停可能比查询慢，但也不该无限等
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var err error
	switch action {
	case "start":
		err = global.DockerClient.StartContainer(ctx, id)
	case "stop":
		err = global.DockerClient.StopContainer(ctx, id)
	case "restart":
		err = global.DockerClient.RestartContainer(ctx, id)
	}
	if err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}

	apiReturn.SuccessData(c, gin.H{"ok": true})
}
