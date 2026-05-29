package http

import (
	"downder-backend/internal/delivery/http/middleware"
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
			// 20 times per minute
			videoGroup.POST("/extract", middleware.LimitMiddleware(20, 20), videoHandler.Extract)

			// 4 times per minute
			videoGroup.POST("/process", middleware.LimitMiddleware(4, 4), videoHandler.Process)
		}
	}
}
