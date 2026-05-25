package http

import (
	"downder-backend/internal/domain"
	"net/http"

	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	service domain.VideoService
}

func NewVideoHandler(service domain.VideoService) *VideoHandler {
	return &VideoHandler{service: service}
}

// Extract Payload
type ExtractRequest struct {
	URL string `json:"url" binding:"required,url"`
}

// Extract รับลิงก์และคืนค่าข้อมูลวิดีโอกลับไป
func (h *VideoHandler) Extract(c *gin.Context) {
	var req ExtractRequest

	// Bind JSON และตรวจสอบความถูกต้องของ URL เบื้องต้น
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or missing URL format"})
		return
	}

	metadata, err := h.service.FetchMetadata(c.Request.Context(), req.URL)
	if err != nil {
		// ถ้าเป็น Error จากระบบให้ส่ง 500 หรือจะเช็คประเภท Error เพื่อส่ง 400 ก็ได้
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Metadata extracted successfully",
		"data":    metadata,
	})

	// example payload:
	// {
	//   "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	// }
}

// Process Payload
type ProcessRequest struct {
	URL     string             `json:"url" binding:"required,url"`
	Options domain.TrimOptions `json:"options"`
}

// Process รับคำสั่งตัดต่อ/แปลงไฟล์
func (h *VideoHandler) Process(c *gin.Context) {
	var req ProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	downloadURL, err := h.service.ProcessVideo(c.Request.Context(), req.URL, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Video processed successfully",
		"download_url": downloadURL,
	})

	// example payload:
	// {
	//   "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	//   "options": {
	//     "start_time": "00:01:00",
	//     "end_time": "00:02:00",
	//     "format": "mp3"
	//   }
	// }
}
