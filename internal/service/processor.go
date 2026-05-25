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

// ProcessVideo รับคำสั่ง สกัดลิงก์สตรีม และโยนให้ FFmpeg ไปทำงาน
func (s *videoService) ProcessVideo(ctx context.Context, inputURL string, opts domain.TrimOptions) (string, error) {
	if inputURL == "" {
		return "", fmt.Errorf("input URL is required")
	}

	// 1. ดึง Metadata เพื่อหาลิงก์สตรีมวิดีโอของจริง (Raw Stream URL)
	metadata, err := s.FetchMetadata(ctx, inputURL)
	if err != nil {
		return "", fmt.Errorf("failed to extract stream url: %w", err)
	}

	if len(metadata.Formats) == 0 {
		return "", fmt.Errorf("no downloadable formats found")
	}

	// 2. เลือก Stream URL ที่เหมาะสมกับสิ่งที่ผู้ใช้ต้องการ
	var streamURL string
	formatReq := strings.ToLower(opts.Format)

	for _, f := range metadata.Formats {
		// ถ้า User ขอ MP3 เราจะฉลาดเลือกดึงแค่ไฟล์เสียงไปให้ FFmpeg
		if formatReq == "mp3" && f.Quality == "Audio" {
			streamURL = f.DownloadURL
			break
		}
		// ถ้าเป็นวิดีโอ ก็หาฟอร์แมตที่มีภาพและเสียง
		if formatReq != "mp3" && f.Quality != "Audio" {
			streamURL = f.DownloadURL
			break
		}
	}

	// ถ้าไม่ตรงเงื่อนไขเลย ให้เอาลิงก์แรกที่ใช้ได้ในระบบ
	if streamURL == "" {
		streamURL = metadata.Formats[0].DownloadURL
	}

	// 3. กำหนดนามสกุลไฟล์
	extension := "mp4"
	if formatReq == "mp3" {
		extension = "mp3"
	} else if formatReq == "webm" {
		extension = "webm"
	}

	// 4. สร้างชื่อไฟล์และโฟลเดอร์ปลายทาง
	timestamp := time.Now().UnixNano()
	fileName := fmt.Sprintf("media_%d.%s", timestamp, extension)

	outputDir := "./tmp/downloads"
	outputPath := filepath.Join(outputDir, fileName)

	// 5. สั่ง FFmpeg ทำงาน (เปลี่ยนจาก inputURL ที่เป็นหน้าเว็บ มาเป็น streamURL ของแท้)
	err = ffmpeg.ProcessMedia(ctx, streamURL, outputPath, opts)
	if err != nil {
		return "", fmt.Errorf("failed to process media: %w", err)
	}

	// 6. ส่งกลับ Path สำหรับดาวน์โหลดไฟล์
	downloadPath := fmt.Sprintf("/api/v1/downloads/%s", fileName)
	return downloadPath, nil
}
