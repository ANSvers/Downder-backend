package scraper

import (
	"context"
	"downder-backend/internal/domain"
	"downder-backend/pkg/ytdlp"
)

type tiktokScraper struct{}

func NewTikTokScraper() domain.VideoScraper {
	return &tiktokScraper{}
}

func (t *tiktokScraper) Extract(ctx context.Context, url string) (*domain.VideoMetadata, error) {
	return ytdlp.Extract(ctx, url)
}
