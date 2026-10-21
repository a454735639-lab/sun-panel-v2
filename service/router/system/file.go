package system

import (
	"sun-panel/api/api_v1"
	"sun-panel/api/api_v1/middleware"

	"github.com/gin-gonic/gin"
)

func InitFileRouter(router *gin.RouterGroup) {
	FileApi := api_v1.ApiGroupApp.ApiSystem.FileApi

	// 验证项目的权限(有访问密码的需要验证访问token)
	private := router.Group("", middleware.LoginInterceptor)
	{
		private.POST("/file/uploadImg", FileApi.UploadImg)
		private.POST("/file/uploadFiles", FileApi.UploadFiles)

		private.POST("/file/getList", FileApi.GetList)
		private.POST("/file/deletes", FileApi.Deletes)

		// 公共图库：站内共享的图片。仍要求登录 ——
		// 它是"所有账号都能用"，不是"对互联网公开"。
		private.POST("/file/publicList", FileApi.PublicList)
		private.POST("/file/setPublic", FileApi.SetPublic)
		private.POST("/file/setUsage", FileApi.SetUsage)
	}

}
