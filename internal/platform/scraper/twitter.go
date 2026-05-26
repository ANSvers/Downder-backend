package scraper

import (
	"context"
	"downder-backend/internal/domain"
	"downder-backend/pkg/ytdlp"
)

type twitterScraper struct{}

func NewTwitterScraper() domain.VideoScraper {
	return &twitterScraper{}
}

func (t *twitterScraper) Extract(ctx context.Context, url string) (*domain.VideoMetadata, error) {
	return ytdlp.Extract(ctx, url)
}
