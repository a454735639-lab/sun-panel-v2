package panel

import (
	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/api/api_v1/common/base"
	"sun-panel/global"
	"sun-panel/models"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// UrlRuleApi 地址匹配规则的 JSON API。
//
// 为什么需要这一层：规则功能原先只在 SSR 层实现（pages/rules.go 走表单 + htmx），
// Vue 前端无法调用。这里补一套 JSON 接口，让两套前端共用同一份数据与匹配逻辑
// （匹配算法在 models/UrlRule.match.go，两边共用，不重复实现）。
type UrlRuleApi struct {
}

// listReq 列表查询参数
type urlRuleListReq struct {
	// Enabled 为 nil 时返回全部；指定 1/0 时只返回启用/停用项
	Enabled *int `json:"enabled"`
}

// List 返回当前用户的全部规则（按 sort 升序）
func (a *UrlRuleApi) List(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)

	var req urlRuleListReq
	// 允许空 body：前端不带参数时也应正常工作，故忽略绑定错误
	_ = c.ShouldBindBodyWith(&req, binding.JSON)

	q := global.Db.Where("user_id = ?", userInfo.ID)
	if req.Enabled != nil {
		q = q.Where("enabled = ?", *req.Enabled)
	}

	list := []models.UrlRule{}
	if err := q.Order("sort asc, id asc").Find(&list).Error; err != nil {
		apiReturn.ErrorByCode(c, 1200) // 数据库出错
		return
	}
	apiReturn.SuccessData(c, gin.H{"list": list, "count": len(list)})
}

// Edit 新增或更新规则。
//
// 带 ID 视为更新，否则新增。更新时一律附带 user_id 条件，
// 避免越权修改他人规则（与 SSR 层 pages/rules.go 的语义保持一致）。
func (a *UrlRuleApi) Edit(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)

	req := models.UrlRule{}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	// 两端空白交由业务校验，见下
	if len(req.Pattern) == 0 || len(req.Target) == 0 {
		apiReturn.ErrorCode(c, 1400, "匹配模式与打开地址都不能为空", nil)
		return
	}

	// 强制归属当前用户，忽略请求里可能带的 userId
	req.UserId = userInfo.ID

	if req.ID != 0 {
		// 更新：只允许改这几个字段，且必须属于当前用户
		res := global.Db.Model(&models.UrlRule{}).
			Where("id = ? AND user_id = ?", req.ID, userInfo.ID).
			Updates(map[string]interface{}{
				"pattern":     req.Pattern,
				"target":      req.Target,
				"description": req.Description,
				"enabled":     req.Enabled,
				"sort":        req.Sort,
			})
		if res.Error != nil {
			apiReturn.ErrorByCode(c, 1200)
			return
		}
		if res.RowsAffected == 0 {
			apiReturn.ErrorByCode(c, 1202) // 未找到数据记录
			return
		}
		apiReturn.SuccessData(c, gin.H{"id": req.ID})
		return
	}

	// 新增：未指定 sort 时排到末尾，避免全部挤在 0 导致顺序不确定
	if req.Sort == 0 {
		var maxSort int
		global.Db.Model(&models.UrlRule{}).
			Where("user_id = ?", userInfo.ID).
			Select("COALESCE(MAX(sort), 0)").Scan(&maxSort)
		req.Sort = maxSort + 1
	}
	if err := global.Db.Create(&req).Error; err != nil {
		apiReturn.ErrorByCode(c, 1200)
		return
	}
	apiReturn.SuccessData(c, gin.H{"id": req.ID})
}

// Deletes 批量删除。请求体形如 {"ids":[1,2,3]}。
//
// 用批量而非单条：管理界面支持多选删除，逐条请求会产生 N 次往返。
func (a *UrlRuleApi) Deletes(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)

	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}
	if len(req.IDs) == 0 {
		apiReturn.ErrorCode(c, 1201, "请至少保留一个", nil)
		return
	}

	// 附带 user_id 条件：即便传入他人 ID 也不会被删除
	res := global.Db.Where("user_id = ? AND id IN ?", userInfo.ID, req.IDs).
		Delete(&models.UrlRule{})
	if res.Error != nil {
		apiReturn.ErrorByCode(c, 1200)
		return
	}
	apiReturn.SuccessData(c, gin.H{"deleted": res.RowsAffected})
}

// Test 用给定 URL 试跑规则，返回命中的规则与最终地址。
//
// 存在的意义：规则支持通配符与端口，用户很难凭肉眼确认写对没有。
// 提供一个"拿真实地址试一下"的入口，能显著降低配错率。
func (a *UrlRuleApi) Test(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)

	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	list := []models.UrlRule{}
	global.Db.Where("user_id = ? AND enabled = ?", userInfo.ID, models.INT_TURE).
		Order("sort asc, id asc").Find(&list)

	target, hit := models.PickUrlByRules(list, req.URL)
	if !hit {
		apiReturn.SuccessData(c, gin.H{
			"hit":    false,
			"target": "",
		})
		return
	}

	// 一并返回是哪条规则命中，便于用户理解
	var hitRule *models.UrlRule
	for i := range list {
		if models.MatchUrlPattern(list[i].Pattern, req.URL) {
			hitRule = &list[i]
			break
		}
	}
	apiReturn.SuccessData(c, gin.H{
		"hit":    true,
		"target": target,
		"rule":   hitRule,
	})
}