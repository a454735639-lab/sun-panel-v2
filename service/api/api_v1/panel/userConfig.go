package panel

import (
	"encoding/json"
	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/api/api_v1/common/base"
	"sun-panel/global"
	"sun-panel/models"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
)

type UserConfig struct {
}

func (a *UserConfig) Get(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)
	cfg := models.UserConfig{}
	if err := global.Db.First(&cfg, "user_id=?", userInfo.ID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 首次访问时用户还没有保存过任何配置 —— 这是**正常状态**，
			// 不是错误。返回一份空配置，让前端用默认值渲染面板。
			//
			// 原先返回 ErrorDataNotFound（code 1202），前端会当成失败：
			// 轻则控制台报错，重则面板拿不到配置而显示异常。
			// 实测场景：新建用户或公开模式下访问一个从未保存过配置的账号。
			apiReturn.SuccessData(c, gin.H{
				"panel":        gin.H{},
				"searchEngine": gin.H{},
			})
			return
		}
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}

	// 处理字段
	if err := json.Unmarshal([]byte(cfg.PanelJson), &cfg.Panel); err != nil {
		cfg.Panel = nil
	}
	if err := json.Unmarshal([]byte(cfg.SearchEngineJson), &cfg.SearchEngine); err != nil {
		cfg.SearchEngine = nil
	}
	apiReturn.SuccessData(c, cfg)

}

func (a *UserConfig) Set(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)
	req := models.UserConfig{}

	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	// 处理字段
	if jb, err := json.Marshal(req.Panel); err != nil {
		req.PanelJson = "{}"
	} else {
		req.PanelJson = string(jb)
	}

	if jb, err := json.Marshal(req.SearchEngine); err != nil {
		req.SearchEngineJson = "{}"
	} else {
		req.SearchEngineJson = string(jb)
	}

	// 保存操作
	if err := global.Db.First(&models.UserConfig{}, "user_id=?", userInfo.ID).Error; err != nil {
		req.UserId = userInfo.ID
		if err == gorm.ErrRecordNotFound {
			// 新增
			if err := global.Db.Create(&req).Error; err != nil {
				apiReturn.ErrorDatabase(c, err.Error())
				return
			}
		} else {
			// 报错
			apiReturn.ErrorDatabase(c, err.Error())
			return
		}
	} else {
		// 修改
		if err := global.Db.Where("user_id=?", userInfo.ID).Updates(&req).Error; err != nil {
			apiReturn.ErrorDatabase(c, err.Error())
			return
		}
	}

	apiReturn.Success(c)
}
