package service

import (
	"context"
	"downder-backend/internal/domain"
	"downder-backend/pkg/ffmpeg"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ProcessVideo รับคำสั่ง ตัดสินใจตั้งชื่อไฟล์ แล้วโยนให้ FFmpeg ไปทำงาน
func (s *videoService) ProcessVideo(ctx context.Context, inputURL string, opts domain.TrimOptions) (string, error) {
	if inputURL == "" {
		return "", fmt.Errorf("input URL is required")
	}

	// 1. กำหนดนามสกุลไฟล์
	extension := "mp4"
	format := strings.ToLower(opts.Format)
	if format == "mp3" {
		extension = "mp3"
	} else if format == "webm" {
		extension = "webm"
	}

	// 2. สร้างชื่อไฟล์และโฟลเดอร์ปลายทาง
	timestamp := time.Now().UnixNano()
	fileName := fmt.Sprintf("media_%d.%s", timestamp, extension)

	outputDir := "./tmp/downloads"
	outputPath := filepath.Join(outputDir, fileName)

	// 3. สั่ง FFmpeg ทำงาน
	err := ffmpeg.ProcessMedia(ctx, inputURL, outputPath, opts)
	if err != nil {
		return "", fmt.Errorf("failed to process media: %w", err)
	}

	// 4. ส่งกลับ Path สำหรับดาวน์โหลดไฟล์ที่สำเร็จแล้ว
	// ตรงนี้จะไปเชื่อมกับ Route ที่เราจะเขียนใน Phase 5 (เช่น /api/v1/downloads/media_xxx.mp4)
	downloadPath := fmt.Sprintf("/api/v1/downloads/%s", fileName)
	return downloadPath, nil
}
