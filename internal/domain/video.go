package domain

import "context"

// VideoMetadata : หน้าตาของข้อมูลที่จะส่งกลับไปให้ Frontend
type VideoMetadata struct {
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	Duration     string           `json:"duration"` // ex. "03:45"
	ThumbnailURL string           `json:"thumbnail_url"`
	Formats      []DownloadFormat `json:"formats"`
}

// DownloadFormat : ตัวเลือกความละเอียดที่ user สามารถกดโหลดได้
type DownloadFormat struct {
	Quality     string `json:"quality"`      // ex. "1080p", "720p", "Audio"
	Extension   string `json:"extension"`    // ex. "mp4", "mp3"
	FileSize    string `json:"file_size"`    // ex. "15.5 MB"
	DownloadURL string `json:"download_url"` // link สตรีมจริงที่ดูดมาได้
}

// TrimOptions :  เก็บคำสั่งที่ผู้ใช้ส่งมาจากหน้าเว็บว่าอยากให้ตัดต่อหรือแปลงไฟล์ยังไง
type TrimOptions struct {
	StartTime string `json:"start_time"` // เวลาเริ่ม เช่น "00:01:30"
	EndTime   string `json:"end_time"`   // เวลาจบ เช่น "00:02:00"
	Format    string `json:"format"`     // ประเภทไฟล์ เช่น "mp4", "mp3"
	Bitrate   string `json:"bitrate"`    // สำหรับ audio เช่น "320k"
	Quality   string `json:"quality"`
}

// ========================================================
// Interfaces (สัญญาที่ Layer อื่นๆ ต้องทำตาม)
// ========================================================

// VideoScraper : กติกาสำหรับบอทแกะรอย (YouTube, TikTok ต้องมีฟังก์ชันนี้)
type VideoScraper interface {
	Extract(ctx context.Context, url string) (*VideoMetadata, error)
}

// VideoService : กติกาสำหรับสมองของระบบ
type VideoService interface {
	FetchMetadata(ctx context.Context, url string) (*VideoMetadata, error)
	ProcessVideo(ctx context.Context, inputURL string, opts TrimOptions) (string, error)
}
