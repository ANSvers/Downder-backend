package scraper

import (
	"context"
	"downder-backend/internal/domain"
	"downder-backend/pkg/ytdlp"
)

type instagramScraper struct{}

func NewInstagramScraper() domain.VideoScraper {
	return &instagramScraper{}
}

func (i *instagramScraper) Extract(ctx context.Context, url string) (*domain.VideoMetadata, error) {
	return ytdlp.Extract(ctx, url)
}
