package system

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	"io"
	"os"
	"path"
	"strings"
	"sun-panel/api/api_v1/common/apiData/commonApiStructs"
	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/api/api_v1/common/base"
	"sun-panel/global"
	"sun-panel/lib/cmn"
	"sun-panel/models"
	"time"
)

type FileApi struct{}

func (a *FileApi) UploadImg(c *gin.Context) {
	// 获取上传的文件
	f, err := c.FormFile("imgfile")
	if err != nil {
		apiReturn.ErrorByCode(c, 1300)
		return
	}

	// 检查文件类型
	fileExt := strings.ToLower(path.Ext(f.Filename))
	agreeExts := []string{
		".png",
		".jpg",
		".gif",
		".jpeg",
		".webp",
		".svg",
		".ico",
	}

	if !strings.Contains(strings.Join(agreeExts, "|"), fileExt) {
		apiReturn.ErrorByCode(c, 1301)
		return
	}

	// 打开上传的文件
	file, err := f.Open()
	if err != nil {
		apiReturn.ErrorByCode(c, 1300)
		return
	}
	defer file.Close()

	// 读取文件内容
	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, file); err != nil {
		apiReturn.ErrorByCode(c, 1300)
		return
	}

	// 获取文件的MIME类型
	contentType := "image/png" // 默认使用PNG
	switch fileExt {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	case ".svg":
		contentType = "image/svg+xml"
	case ".ico":
		contentType = "image/x-icon"
	}

	// 转换为base64
	base64Str := base64.StdEncoding.EncodeToString(buffer.Bytes())

	// 返回data URL格式
	dataURL := fmt.Sprintf("data:%s;base64,%s", contentType, base64Str)

	// 直接返回base64数据
	apiReturn.SuccessData(c, gin.H{
		"imageUrl": dataURL,
	})
}

func (a *FileApi) UploadFiles(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)
	configUpload := global.Config.GetValueString("base", "source_path")

	form, err := c.MultipartForm()
	if err != nil {
		apiReturn.ErrorByCode(c, 1300)
		return
	}
	files := form.File["files[]"]
	errFiles := []string{}
	succMap := map[string]string{}
	for _, f := range files {
		fileExt := strings.ToLower(path.Ext(f.Filename))
		fileName := cmn.Md5(fmt.Sprintf("%s%s", f.Filename, time.Now().String()))
		fildDir := fmt.Sprintf("%s/%d/%d/%d/", configUpload, time.Now().Year(), time.Now().Month(), time.Now().Day())
		isExist, _ := cmn.PathExists(fildDir)
		if !isExist {
			os.MkdirAll(fildDir, os.ModePerm)
		}
		filepath := fmt.Sprintf("%s%s%s", fildDir, fileName, fileExt)
		if c.SaveUploadedFile(f, filepath) != nil {
			errFiles = append(errFiles, f.Filename)
		} else {
			// 成功
			// 像数据库添加记录
			mFile := models.File{}
			mFile.AddFile(userInfo.ID, f.Filename, fileExt, filepath)
			succMap[f.Filename] = filepath[1:]
		}
	}

	apiReturn.SuccessData(c, gin.H{
		"succMap":  succMap,
		"errFiles": errFiles,
	})
}

func (a *FileApi) GetList(c *gin.Context) {
	list := []models.File{}
	userInfo, _ := base.GetCurrentUserInfo(c)
	var count int64
	if err := global.Db.Order("created_at desc").Find(&list, "user_id=?", userInfo.ID).Count(&count).Error; err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}

	data := []map[string]interface{}{}
	for _, v := range list {
		data = append(data, map[string]interface{}{
			"src":        v.Src[1:],
			"fileName":   v.FileName,
			"id":         v.ID,
			"createTime": v.CreatedAt,
			"updateTime": v.UpdatedAt,
			"path":       v.Src,
			// 扩展名。模型与数据库一直有该字段，但这里手工拼返回结构时
			// 漏掉了 —— 前端想按类型筛选文件（如图库只显示图片）时
			// 只能退化到解析文件名后缀。补上更可靠。
			"ext": v.Ext,
			// 是否已共享到公共图库。列表里要显示"已共享"标记，
			// 否则用户不知道自己哪些图是公开的。
			"isPublic": v.IsPublic,
		})
	}
	apiReturn.SuccessListData(c, data, count)
}

func (a *FileApi) Deletes(c *gin.Context) {
	req := commonApiStructs.RequestDeleteIds[uint]{}
	userInfo, _ := base.GetCurrentUserInfo(c)
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	global.Db.Transaction(func(tx *gorm.DB) error {
		files := []models.File{}

		if err := tx.Order("created_at desc").Find(&files, "user_id=? AND id in ?", userInfo.ID, req.Ids).Error; err != nil {
			return err
		}

		for _, v := range files {
			os.Remove(v.Src)
		}

		if err := tx.Order("created_at desc").Delete(&files, "user_id=? AND id in ?", userInfo.ID, req.Ids).Error; err != nil {
			return err
		}

		return nil
	})

	apiReturn.Success(c)

}

// SetPublic 把已有文件设为共享 / 取消共享到公共图库。
//
// 只允许操作自己的文件（带 user_id 条件）—— 否则任何用户都能
// 把别人的图"共享"出去，等于变相公开他人的私人图片。
func (a *FileApi) SetPublic(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)

	var req struct {
		IDs      []uint `json:"ids"`
		IsPublic int    `json:"isPublic"`
	}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}
	if len(req.IDs) == 0 {
		apiReturn.ErrorParamFomat(c, "ids 不能为空")
		return
	}

	v := 0
	if req.IsPublic == 1 {
		v = 1
	}

	res := global.Db.Model(&models.File{}).
		Where("user_id = ? AND id in ?", userInfo.ID, req.IDs).
		Update("is_public", v)
	if res.Error != nil {
		apiReturn.ErrorDatabase(c, res.Error.Error())
		return
	}

	apiReturn.SuccessData(c, gin.H{"updated": res.RowsAffected})
}

// PublicList 公共图库：所有用户共享的图片。
//
// 只返回 is_public=1 的文件，且**不按当前用户过滤** —— 这正是"公共"的含义。
// 但仍要求登录：公共图库是站内共享，不是对互联网公开。
func (a *FileApi) PublicList(c *gin.Context) {
	list := []models.File{}
	if err := global.Db.
		Where("is_public = ?", 1).
		Order("created_at desc").
		Find(&list).Error; err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}

	// 与 GetList 保持同一返回结构（src 去掉开头的 "."，
	// 使前端拿到的都是可直接用于 <img src> 的路径）。
	data := []map[string]interface{}{}
	for _, v := range list {
		src := v.Src
		if len(src) > 0 && src[0] == '.' {
			src = src[1:]
		}
		data = append(data, map[string]interface{}{
			"src":        src,
			"fileName":   v.FileName,
			"id":         v.ID,
			"createTime": v.CreatedAt,
			"updateTime": v.UpdatedAt,
			"path":       v.Src,
			"ext":        v.Ext,
			"isPublic":   v.IsPublic,
			// 上传者：公共图库需要知道图是谁分享的，
			// 便于使用者辨认来源、以及后续追溯。
			"userId": v.UserId,
		})
	}

	apiReturn.SuccessListData(c, data, int64(len(data)))
}
