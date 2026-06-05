package importn

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册导入路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	// 笔记本下的导入操作
	notebooks := r.Group("/notebooks/:nbId/import")
	{
		notebooks.POST("/file", ctrl.ImportFile)
		notebooks.POST("/audio/preview", ctrl.PreviewAudio)
	}

	// 全局导入操作
	imp := r.Group("/import")
	{
		imp.POST("/audio/confirm", ctrl.ConfirmAudio)
		imp.GET("/tasks/:taskId", ctrl.GetTask)
	}
}
