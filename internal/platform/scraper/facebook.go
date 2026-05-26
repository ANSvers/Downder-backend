package scraper

import (
	"context"
	"downder-backend/internal/domain"
	"downder-backend/pkg/ytdlp"
)

type facebookScraper struct{}

func NewFacebookScraper() domain.VideoScraper {
	return &facebookScraper{}
}

func (f *facebookScraper) Extract(ctx context.Context, url string) (*domain.VideoMetadata, error) {
	return ytdlp.Extract(ctx, url)
}
