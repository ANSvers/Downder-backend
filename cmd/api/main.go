package main

import (
	"downder-backend/config"
	deliveryHttp "downder-backend/internal/delivery/http"
	"downder-backend/internal/platform/scraper"
	"downder-backend/internal/platform/storage"
	"downder-backend/internal/service"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load Configurations from .env
	cfg := config.LoadConfig()
	log.Printf("⚙️  Config Loaded: Port=%s, Storage=%s\n", cfg.Port, cfg.StorageDir)

	// Start Storage Cleanup Worker (Background Job)
	storage.StartCleanupWorker(cfg.StorageDir, 15*time.Minute, 5*time.Minute)
	log.Println("🧹 Storage cleanup worker started")

	// ==========================================
	// Dependency Injection
	// ==========================================

	// Platform Layer: สร้าง Scraper สำหรับแต่ละแพลตฟอร์ม
	ytScraper := scraper.NewYouTubeScraper()
	tiktokScraper := scraper.NewTikTokScraper()
	igScraper := scraper.NewInstagramScraper()
	fbScraper := scraper.NewFacebookScraper()
	twitterScraper := scraper.NewTwitterScraper()

	// Service Layer: สร้าง Service หลัก แล้วส่ง Scraper เข้าไปให้มันใช้ทำงาน
	videoSvc := service.NewVideoService(ytScraper, tiktokScraper, igScraper, fbScraper, twitterScraper)
	// Delivery Layer: สร้าง Handler สำหรับรับ HTTP Request แล้วส่ง Service ให้มันไปเรียกใช้
	videoHandler := deliveryHttp.NewVideoHandler(videoSvc)

	// Gin Router Setup
	r := gin.Default()

	// CORS Middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"}, // Allow frontend orgins
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "UP",
			"message": "Docker, Go, and Config are working perfectly! 🚀",
			"port":    cfg.Port,
		})
	})

	deliveryHttp.SetupRouter(r, videoHandler)

	// Start Server
	log.Printf("🚀 Server is starting on port %s...\n", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
