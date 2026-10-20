package panel

import (
	"errors"
	"fmt"
	"strings"
	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/api/api_v1/common/base"
	"sun-panel/global"
	"sun-panel/lib/cmn"
	"sun-panel/lib/cmn/systemSetting"
	"sun-panel/models"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
)

// 此API 临时使用，后期带有管理功能，将废除！！！
type UsersApi struct {
}

var (
	ErrUsersApiAtLeastKeepOne = errors.New("at least keep one")
)

func (a UsersApi) Create(c *gin.Context) {
	param := models.User{}
	if err := c.ShouldBindBodyWith(&param, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	if errMsg, err := base.ValidateInputStruct(param); err != nil {
		apiReturn.ErrorParamFomat(c, errMsg)
		return
	}

	param.Username = strings.TrimSpace(param.Username)
	if len(param.Username) < 5 {
		apiReturn.ErrorParamFomat(c, "The account must be no less than 5 characters long")
		return
	}

	mUser := models.User{
		Username:  strings.TrimSpace(param.Username),
		Password:  cmn.PasswordEncryption(param.Password),
		Name:      param.Name,
		HeadImage: param.HeadImage,
		Status:    1,
		Role:      param.Role,
		// Mail:      param.Username, 不再保存邮箱账号字段
	}

	// 验证账号是否存在
	if _, err := mUser.CheckUsernameExist(param.Username); err != nil {
		apiReturn.ErrorByCode(c, 1006)
		// apiReturn.Error(c, global.Lang.Get("register.mail_exist"))
		return
	}

	userInfo, err := mUser.CreateOne()

	if err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}

	apiReturn.SuccessData(c, gin.H{"userId": userInfo.ID})
}

func (a UsersApi) Deletes(c *gin.Context) {
	type UserIds struct {
		UserIds []uint
	}
	param := UserIds{}
	if err := c.ShouldBindBodyWith(&param, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		c.Abort()
		return
	}

	txErr := global.Db.Transaction(func(tx *gorm.DB) error {
		mitemIconGroup := models.ItemIconGroup{}

		for _, v := range param.UserIds {
			// 删除图标
			if err := tx.Delete(&models.ItemIcon{}, "user_id=?", v).Error; err != nil {
				return err
			}
			// 删除分组
			if err := mitemIconGroup.DeleteByUserId(tx, v); err != nil {
				return err
			}
			// 删除模块配置
			if err := tx.Delete(&models.ModuleConfig{}, "user_id=?", v).Error; err != nil {
				return err
			}
			// 删除用户配置
			if err := tx.Delete(&models.ModuleConfig{}, "user_id=?", v).Error; err != nil {
				return err
			}
			// // 删除文件记录（不删除资源文件）
			// if err := tx.Delete(&models.File{}, "user_id=?", v).Error; err != nil {
			// 	return err
			// }
		}

		if err := tx.Delete(&models.User{}, &param.UserIds).Error; err != nil {
			apiReturn.ErrorDatabase(c, err.Error())
			return err
		}

		// 验证是否还存在管理员
		var count int64
		if err := tx.Model(&models.User{}).Where("role=?", 1).Count(&count).Error; err != nil {
			return err
		} else if count == 0 {
			return ErrUsersApiAtLeastKeepOne
		}

		return nil
	})
	if txErr == ErrUsersApiAtLeastKeepOne {
		apiReturn.ErrorByCode(c, 1201)
		return
	} else if txErr != nil {
		apiReturn.ErrorDatabase(c, txErr.Error())
		return
	}

	apiReturn.Success(c)
}

func (a UsersApi) Update(c *gin.Context) {
	param := models.User{}
	if err := c.ShouldBindBodyWith(&param, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		c.Abort()
		return
	}

	if param.Password == "" {
		param.Password = "-" // 修改不允许修改密码，为了验证通过
	}

	// param.Mail = param.Username // 密码邮箱同时修改
	if errMsg, err := base.ValidateInputStruct(param); err != nil {
		apiReturn.ErrorParamFomat(c, errMsg)
		return
	}

	param.Username = strings.Trim(param.Username, " ")
	if len(param.Username) < 3 {
		// 账号不得少于3个字符
		apiReturn.ErrorParamFomat(c, "The account must be no less than 3 characters long")
		return
	}

	allowField := []string{"Username", "Name", "Mail", "Token", "Role"}

	// 密码不为默认“-”空，修改密码
	if param.Password != "-" {
		param.Password = cmn.PasswordEncryption(param.Password)
		allowField = append(allowField, "Password")
	}

	mUser := models.User{}

	userInfo := models.User{}
	// 验证账号是否存在
	if user, err := mUser.CheckUsernameExist(param.Username); err != nil {
		userInfo = user
		if user.ID != param.ID {
			apiReturn.ErrorByCode(c, 1006)
			// apiReturn.Error(c, global.Lang.Get("register.mail_exist"))
			return
		}
	} else {
		userInfo = user
	}

	param.Token = "" // 修改资料就重置token
	if err := global.Db.Select(allowField).Where("id=?", param.ID).Updates(&param).Error; err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}
	// global.Logger.Debug("修改资料清空token", userInfo.Token)
	global.UserToken.Delete(userInfo.Token) // 更新用户信息
	// 返回token等基本信息
	apiReturn.SuccessData(c, param)
}

func (a UsersApi) GetList(c *gin.Context) {

	type ParamsStruct struct {
		models.User
		Limit   int
		Page    int
		Keyword string `json:"keyword"`
	}

	param := ParamsStruct{}
	if err := c.ShouldBindBodyWith(&param, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		c.Abort()
		return
	}

	var (
		list  []models.User
		count int64
	)
	db := global.Db

	// 查询条件
	if param.Keyword != "" {
		db = db.Where("name LIKE ? OR username LIKE ?", "%"+param.Keyword+"%", "%"+param.Keyword+"%")
	}

	if err := db.Limit(param.Limit).Offset((param.Page - 1) * param.Limit).Find(&list).Limit(-1).Offset(-1).Count(&count).Error; err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}

	// 白名单返回字段。
	//
	// 原实现用 db.Omit("Password") 只挡掉了密码，但 models.User 还有
	// Token 与 ReferralCode 两个敏感字段 —— 实测响应里确实带着
	// "token":"63ad808c..."。这意味着任何能调本接口的管理员，
	// 都能拿到其它账号（含其他管理员）的 token，直接冒充登录。
	//
	// 用显式白名单而不是继续往 Omit 里加字段：以后 User 再加敏感字段时，
	// 默认不会泄漏出去。忘记加 Omit 的代价是凭据泄露，忘记加白名单的
	// 代价只是前端少显示一个字段 —— 出错方向相反，应当选更安全的那个。
	type publicUser struct {
		ID        uint   `json:"id"`
		Username  string `json:"username"`
		Name      string `json:"name"`
		HeadImage string `json:"headImage"`
		Status    int    `json:"status"`
		Role      int    `json:"role"`
		Mail      string `json:"mail"`
		CreatedAt string `json:"createTime"`
	}
	resList := make([]publicUser, 0, len(list))
	for _, v := range list {
		resList = append(resList, publicUser{
			ID:        v.ID,
			Username:  v.Username,
			Name:      v.Name,
			HeadImage: v.HeadImage,
			Status:    v.Status,
			Role:      v.Role,
			Mail:      v.Mail,
			CreatedAt: v.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	apiReturn.SuccessListData(c, resList, count)
}

func (a UsersApi) SetPublicVisitUser(c *gin.Context) {
	type Req struct {
		UserId *uint `json:"userId"`
	}

	req := Req{}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	if req.UserId != nil {
		userInfo := models.User{}
		if err := global.Db.First(&userInfo, "id=?", req.UserId).Error; err != nil {
			fmt.Println(err, userInfo)
			apiReturn.ErrorDataNotFound(c)
			return
		}
	}

	if err := global.SystemSetting.Set(systemSetting.PANEL_PUBLIC_USER_ID, req.UserId); err != nil {
		apiReturn.Error(c, "set fail")
		return
	}
	apiReturn.Success(c)
}

func (a UsersApi) GetPublicVisitUser(c *gin.Context) {
	var userId *uint
	if err := global.SystemSetting.GetValueByInterface(systemSetting.PANEL_PUBLIC_USER_ID, &userId); err == nil && userId != nil {
		userInfo := models.User{}
		// 注意：这里必须解引用 userId。传 *uint 给占位符时，
		// GORM 会按指针处理而非取值，查询条件不成立 → 永远查不到，
		// 表现为"设了访客账号却读不出来"。
		if err := global.Db.First(&userInfo, "id=?", *userId).Error; err == nil {
			// 只返回前端需要的公开字段。
			//
			// 原先直接返回整个 models.User，而它带有
			// password / token / referralCode —— 接口会把凭据一并吐给前端，
			// 属凭据泄露（任何能调此接口的人都能拿到访客账号的 token，
			// 进而以该身份操作）。必须显式白名单。
			apiReturn.SuccessData(c, gin.H{
				"id":        userInfo.ID,
				"username":  userInfo.Username,
				"name":      userInfo.Name,
				"headImage": userInfo.HeadImage,
				"role":      userInfo.Role,
				"status":    userInfo.Status,
			})
			return
		}
	}

	// 没有此配置
	apiReturn.ErrorDataNotFound(c)
}
