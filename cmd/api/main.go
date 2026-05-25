package main

import (
	"downder-backend/config"
	"downder-backend/internal/platform/storage"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load Configurations from .env
	cfg := config.LoadConfig()
	log.Printf("⚙️  Config Loaded: Port=%s, Storage=%s\n", cfg.Port, cfg.StorageDir)

	// Start Storage Cleanup Worker (Background Job)
	storage.StartCleanupWorker(cfg.StorageDir, 15*time.Minute, 5*time.Minute)
	log.Println("🧹 Storage cleanup worker started")

	// Gin Router พื้นฐาน
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "UP",
			"message": "Docker, Go, and Config are working perfectly! 🚀",
			"port":    cfg.Port,
		})
	})

	// Start Server
	log.Printf("🚀 Server is starting on port %s...\n", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
