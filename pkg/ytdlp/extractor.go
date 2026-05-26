package ytdlp

import (
	"bytes"
	"context"
	"downder-backend/internal/domain"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ytDlpOutput โครงสร้าง JSON ดิบที่ได้จาก yt-dlp
type ytDlpOutput struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Duration  float64 `json:"duration"` // เป็นวินาที
	Thumbnail string  `json:"thumbnail"`
	Formats   []struct {
		FormatID string  `json:"format_id"`
		Ext      string  `json:"ext"`
		Height   int     `json:"height"`
		URL      string  `json:"url"`
		Filesize float64 `json:"filesize"`
		Vcodec   string  `json:"vcodec"`
		Acodec   string  `json:"acodec"`
	} `json:"formats"`
}

// Extract แกะข้อมูลวิดีโอจากแทบทุกแพลตฟอร์มบนโลก
func Extract(ctx context.Context, videoURL string) (*domain.VideoMetadata, error) {
	// สั่งรัน yt-dlp ดึงข้อมูลแบบ JSON (-J) โดยไม่ดาวน์โหลดวิดีโอ
	cmd := exec.CommandContext(ctx, "yt-dlp", "-J", videoURL)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to extract via yt-dlp: %w", err)
	}

	var data ytDlpOutput
	if err := json.Unmarshal(out.Bytes(), &data); err != nil {
		return nil, fmt.Errorf("failed to parse yt-dlp output: %w", err)
	}

	// แปลงวินาทีเป็น HH:MM:SS
	totalSeconds := int(data.Duration)
	h := totalSeconds / 3600
	m := (totalSeconds % 3600) / 60
	s := totalSeconds % 60
	durationStr := ""
	if h > 0 {
		durationStr = fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	} else {
		durationStr = fmt.Sprintf("%02d:%02d", m, s)
	}

	var formats []domain.DownloadFormat
	for _, f := range data.Formats {
		// คัดกรองเอาเฉพาะไฟล์ที่มีให้โหลดจริง และไม่ใช่ฟอร์แมตขยะ
		if f.URL == "" || (f.Vcodec == "none" && f.Acodec == "none") {
			continue
		}

		quality := fmt.Sprintf("%dp", f.Height)
		if f.Vcodec == "none" && f.Acodec != "none" {
			quality = "Audio"
		} else if f.Height == 0 {
			quality = "Unknown"
		}

		fileSizeMB := "Unknown"
		if f.Filesize > 0 {
			fileSizeMB = fmt.Sprintf("%.1f MB", f.Filesize/(1024*1024))
		}

		// เคลียร์ค่า Extension ถ้ามันมาเป็น 'unknown_video'
		ext := f.Ext
		if strings.Contains(ext, "unknown") {
			ext = "mp4" // Fallback
		}

		formats = append(formats, domain.DownloadFormat{
			Quality:     quality,
			Extension:   ext,
			FileSize:    fileSizeMB,
			DownloadURL: f.URL,
		})
	}

	return &domain.VideoMetadata{
		ID:           data.ID,
		Title:        data.Title,
		Duration:     durationStr,
		ThumbnailURL: data.Thumbnail,
		Formats:      formats,
	}, nil
}
