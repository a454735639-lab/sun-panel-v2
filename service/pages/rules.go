package pages

import (
	"net/http"
	"strconv"
	"strings"

	"sun-panel/global"
	"sun-panel/models"
	"sun-panel/webrender"

	"github.com/gin-gonic/gin"
)

// loadRules 读取某用户的全部规则（含停用），供管理界面展示。
func loadRules(userId uint) []models.UrlRule {
	out := []models.UrlRule{}
	if global.Db == nil {
		return out
	}
	global.Db.Where("user_id = ?", userId).Order("sort asc, id asc").Find(&out)
	return out
}

// RuleSave 新增或更新地址规则。
// 表单无 id → 新增；有 id → 更新（且校验归属，禁止改他人规则）。
func RuleSave(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}

	idStr := strings.TrimSpace(c.PostForm("id"))
	pattern := strings.TrimSpace(c.PostForm("pattern"))
	target := strings.TrimSpace(c.PostForm("target"))
	desc := strings.TrimSpace(c.PostForm("description"))
	sortN := atoiDefault(c.PostForm("sort"), 0)
	enabled := models.INT_FALSE
	if c.PostForm("enabled") == "1" || c.PostForm("enabled") == "on" {
		enabled = models.INT_TURE
	}

	if pattern == "" || target == "" {
		c.String(http.StatusBadRequest, "匹配模式与打开地址都不能为空")
		return
	}

	if idStr == "" {
		r := models.UrlRule{
			Pattern:     pattern,
			Target:      target,
			Description: desc,
			Enabled:     enabled,
			Sort:        sortN,
			UserId:      user.ID,
		}
		if err := global.Db.Create(&r).Error; err != nil {
			c.String(http.StatusInternalServerError, "保存失败："+err.Error())
			return
		}
	} else {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "规则 ID 非法")
			return
		}
		// 带 user_id 条件更新：防止越权改他人规则
		res := global.Db.Model(&models.UrlRule{}).
			Where("id = ? AND user_id = ?", id, user.ID).
			Updates(map[string]interface{}{
				"pattern":     pattern,
				"target":      target,
				"description": desc,
				"sort":        sortN,
				"enabled":     enabled,
			})
		if res.Error != nil {
			c.String(http.StatusInternalServerError, "保存失败："+res.Error.Error())
			return
		}
		if res.RowsAffected == 0 {
			c.String(http.StatusNotFound, "规则不存在")
			return
		}
	}

	// 新增走表单内联结果；编辑走模态框关闭
	if idStr != "" {
		c.String(http.StatusOK, "")
		return
	}
	webrender.RenderPartial(c, "rules_saved.html", gin.H{})
}

// RuleEdit 打开编辑对话框（htmx 局部渲染）
func RuleEdit(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "规则 ID 非法")
		return
	}
	var r models.UrlRule
	if err := global.Db.Where("id = ? AND user_id = ?", id, user.ID).First(&r).Error; err != nil {
		c.String(http.StatusNotFound, "规则不存在")
		return
	}
	webrender.RenderPartial(c, "rule_editor", gin.H{"Rule": r})
}

// RuleDelete 删除规则（同样校验归属）
func RuleDelete(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "规则 ID 非法")
		return
	}
	res := global.Db.Where("id = ? AND user_id = ?", id, user.ID).Delete(&models.UrlRule{})
	if res.Error != nil {
		c.String(http.StatusInternalServerError, "删除失败："+res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		c.String(http.StatusNotFound, "规则不存在")
		return
	}
	// 删掉表格行
	c.String(http.StatusOK, "")
}
