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

// format time string "hh:mm:ss" or "mm:ss" to total seconds
func parseDurationToSeconds(durationStr string) int {
	parts := strings.Split(durationStr, ":")
	var h, m, s int

	if len(parts) == 3 {
		fmt.Sscanf(parts[0], "%d", &h)
		fmt.Sscanf(parts[1], "%d", &m)
		fmt.Sscanf(parts[2], "%d", &s)
		return (h * 3600) + (m * 60) + s
	} else if len(parts) == 2 {
		fmt.Sscanf(parts[0], "%d", &m)
		fmt.Sscanf(parts[1], "%d", &s)
		return (m * 60) + s
	}
	return 0
}

// ProcessVideo : receive URL, extract stream URL, call FFmpeg to process, return download path
func (s *videoService) ProcessVideo(ctx context.Context, inputURL string, opts domain.TrimOptions) (string, error) {
	if inputURL == "" {
		return "", fmt.Errorf("input URL is required")
	}

	// 1. fetch Metadata to get stream URLs
	metadata, err := s.FetchMetadata(ctx, inputURL)
	if err != nil {
		return "", fmt.Errorf("failed to extract stream url: %w", err)
	}

	// video should not be longer than 2 hours
	maxAllowedSeconds := 2 * 3600
	videoSeconds := parseDurationToSeconds(metadata.Duration)
	if videoSeconds > maxAllowedSeconds {
		return "", fmt.Errorf("video duration (%s) exceeds the maximum limit of 2 hours", metadata.Duration)
	}

	if len(metadata.Formats) == 0 {
		return "", fmt.Errorf("no downloadable formats found")
	}

	// 2. select the best stream URL
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
			// เช็คก่อนว่า User ระบุ Quality มาไหม ถ้าระบุ แล้วไม่ตรง ให้ข้ามไปหาตัวถัดไป
			if opts.Quality != "" && f.Quality != opts.Quality {
				continue
			}
			streamURL = f.DownloadURL
			break
		}
	}

	// Fallback : ถ้าหาความละเอียดที่เจาะจงไม่เจอ ให้เลือกประเภทที่ตรงกันแทน
	if streamURL == "" {
		for _, f := range metadata.Formats {
			if formatReq == "mp3" && f.Quality == "Audio" {
				streamURL = f.DownloadURL
				break
			}
			// ถ้าจะเอาวิดีโอ ให้คว้าเอาวิดีโอตัวแรกที่เจอในระบบ (ดีกว่าหลุดไปได้ไฟล์เสียง)
			if formatReq != "mp3" && f.Quality != "Audio" {
				streamURL = f.DownloadURL
				break
			}
		}
	}

	// Last Resort: ถ้าในระบบไม่มีอะไรตรงเลยจริงๆ ค่อยเอาตัวแรกสุดกันเหนียว
	if streamURL == "" {
		streamURL = metadata.Formats[0].DownloadURL
	}

	// 3. file format
	extension := "mp4"
	if formatReq == "mp3" {
		extension = "mp3"
	} else if formatReq == "webm" {
		extension = "webm"
	}

	// 4. create unique file name
	timestamp := time.Now().UnixNano()
	fileName := fmt.Sprintf("media_%d.%s", timestamp, extension)

	outputDir := "./tmp/downloads"
	outputPath := filepath.Join(outputDir, fileName)

	// 5. สั่ง FFmpeg ทำงาน (เปลี่ยนจาก inputURL ที่เป็นหน้าเว็บ มาเป็น streamURL ของแท้)
	err = ffmpeg.ProcessMedia(ctx, streamURL, outputPath, opts)
	if err != nil {
		return "", fmt.Errorf("failed to process media: %w", err)
	}

	// 6. return download path (relative path for frontend to access)
	downloadPath := fmt.Sprintf("/api/v1/downloads/%s", fileName)
	return downloadPath, nil
}
