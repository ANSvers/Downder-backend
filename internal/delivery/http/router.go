package http

import (
	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine, videoHandler *VideoHandler) {
	// สร้าง API Group V1
	v1 := r.Group("/api/v1")
	{
		videoGroup := v1.Group("/video")
		{
			videoGroup.POST("/extract", videoHandler.Extract)
			videoGroup.POST("/process", videoHandler.Process)
		}
	}
}
