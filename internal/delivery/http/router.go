package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine, videoHandler *VideoHandler) {
	// API Group V1
	v1 := r.Group("/api/v1")
	{
		// Route for downloading processed videos
		v1.StaticFS("/downloads", http.Dir("./tmp/downloads"))

		// Grouping video-related routes
		videoGroup := v1.Group("/video")
		{
			videoGroup.POST("/extract", videoHandler.Extract)
			videoGroup.POST("/process", videoHandler.Process)
		}
	}
}
