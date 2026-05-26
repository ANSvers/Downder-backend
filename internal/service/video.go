package service

import (
	"context"
	"downder-backend/internal/domain"
	"errors"
	"strings"
)

type videoService struct {
	scrapers map[string]domain.VideoScraper
}

// NewVideoService ผูก Scraper ของแต่ละเว็บเข้ากับ Service
func NewVideoService(
	youtube domain.VideoScraper,
	tiktok domain.VideoScraper,
	ig domain.VideoScraper,
	fb domain.VideoScraper,
	twitter domain.VideoScraper,
) domain.VideoService {
	return &videoService{
		scrapers: map[string]domain.VideoScraper{
			// YouTube
			"youtube.com": youtube,
			"youtu.be":    youtube,
			// TikTok
			"tiktok.com": tiktok,
			"vt.tiktok":  tiktok,
			// Instagram
			"instagram.com": ig,
			// Facebook
			"facebook.com": fb,
			"fb.watch":     fb,
			// Twitter / X
			"twitter.com": twitter,
			"x.com":       twitter,
		},
	}
}

// FetchMetadata หา Scraper ที่ตรงกับ URL แล้วสั่งทำงาน
func (s *videoService) FetchMetadata(ctx context.Context, url string) (*domain.VideoMetadata, error) {
	if url == "" {
		return nil, errors.New("url is required")
	}

	var selectedScraper domain.VideoScraper
	for key, scraper := range s.scrapers {
		if strings.Contains(url, key) {
			selectedScraper = scraper
			break
		}
	}

	if selectedScraper == nil {
		return nil, errors.New("unsupported platform")
	}

	return selectedScraper.Extract(ctx, url)
}
