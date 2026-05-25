package scraper

import (
	"context"
	"downder-backend/internal/domain"
	"fmt"
	"strings"

	"github.com/kkdai/youtube/v2"
)

type youtubeScraper struct {
	client youtube.Client
}

// NewYouTubeScraper สร้าง Instance ใหม่เพื่อนำไปใช้ใน Service Layer
func NewYouTubeScraper() domain.VideoScraper {
	return &youtubeScraper{
		client: youtube.Client{},
	}
}

func (y *youtubeScraper) Extract(ctx context.Context, videoURL string) (*domain.VideoMetadata, error) {
	// ดึงข้อมูลวิดีโอ
	video, err := y.client.GetVideoContext(ctx, videoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch youtube video: %w", err)
	}

	// จัดรูปแบบความยาววิดีโอ (HH:MM:SS)
	durationStr := ""
	h := int(video.Duration.Hours())
	m := int(video.Duration.Minutes()) % 60
	s := int(video.Duration.Seconds()) % 60
	if h > 0 {
		durationStr = fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	} else {
		durationStr = fmt.Sprintf("%02d:%02d", m, s)
	}

	// ดึงรูปภาพปก (เอาความละเอียดสูงสุดที่ท้ายสุดของ Array)
	thumbnailURL := ""
	if len(video.Thumbnails) > 0 {
		thumbnailURL = video.Thumbnails[len(video.Thumbnails)-1].URL
	}

	// คัดกรองตัวเลือกความละเอียดในการดาวน์โหลด
	var formats []domain.DownloadFormat
	for _, format := range video.Formats {
		isAudioOnly := format.AudioChannels > 0 && format.Width == 0
		hasVideoAndAudio := format.AudioChannels > 0 && format.Width > 0

		// ข้ามไฟล์ที่มีแต่ภาพไม่มีเสียง (Adaptive)
		if !isAudioOnly && !hasVideoAndAudio {
			continue
		}

		streamURL, err := y.client.GetStreamURL(video, &format)
		if err != nil {
			continue
		}

		quality := format.QualityLabel
		ext := "mp4"
		if strings.Contains(format.MimeType, "webm") {
			ext = "webm"
		}

		if isAudioOnly {
			quality = "Audio"
			if strings.Contains(format.MimeType, "mp4") {
				ext = "m4a"
			}
		}

		fileSizeMB := "Unknown"
		if format.ContentLength > 0 {
			fileSizeMB = fmt.Sprintf("%.1f MB", float64(format.ContentLength)/(1024*1024))
		}

		formats = append(formats, domain.DownloadFormat{
			Quality:     quality,
			Extension:   ext,
			FileSize:    fileSizeMB,
			DownloadURL: streamURL,
		})
	}

	return &domain.VideoMetadata{
		ID:           video.ID,
		Title:        video.Title,
		Duration:     durationStr,
		ThumbnailURL: thumbnailURL,
		Formats:      formats,
	}, nil
}
