package scraper

import (
	"context"
	"strings"
	"testing"

	"downder-backend/internal/domain"
)

// TestPlatformScrapers_ReturnErrorOnInvalidURL verifies that all fully
// implemented platform scrapers return clear, platform-specific error
// messages when given an invalid/non-video URL, rather than panicking
// or returning nil data.
func TestPlatformScrapers_ReturnErrorOnInvalidURL(t *testing.T) {
	cases := []struct {
		name     string
		scraper  domain.VideoScraper
		wantHint string // substring expected in error
	}{
		{"TikTok", NewTikTokScraper(), "tiktok:"},
		{"Instagram", NewInstagramScraper(), "instagram:"},
		{"Facebook", NewFacebookScraper(), "facebook:"},
		{"Twitter", NewTwitterScraper(), "twitter:"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			meta, err := c.scraper.Extract(context.Background(), "https://example.com/test")
			if err == nil {
				t.Fatal("expected error for unimplemented scraper, got nil")
			}
			if meta != nil {
				t.Errorf("expected nil metadata, got %+v", meta)
			}
			if !strings.Contains(err.Error(), c.wantHint) {
				t.Errorf("error %q should contain %q", err.Error(), c.wantHint)
			}
		})
	}
}

// TestStubs_ConstructorDoesNotPanic verifies that constructing each scraper
// is safe and returns a non-nil instance.
func TestStubs_ConstructorDoesNotPanic(t *testing.T) {
	cases := []struct {
		name string
		fn   func() domain.VideoScraper
	}{
		{"TikTok", func() domain.VideoScraper { return NewTikTokScraper() }},
		{"Instagram", func() domain.VideoScraper { return NewInstagramScraper() }},
		{"Facebook", func() domain.VideoScraper { return NewFacebookScraper() }},
		{"Twitter", func() domain.VideoScraper { return NewTwitterScraper() }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("constructor panicked: %v", r)
				}
			}()
			s := c.fn()
			if s == nil {
				t.Fatal("constructor returned nil")
			}
		})
	}
}

// TestStubs_ContextCancellation verifies that unimplemented scrapers still
// respect context cancellation (they should return immediately since they
// don't do any work).
func TestStubs_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	scrapers := []domain.VideoScraper{
		NewTikTokScraper(),
		NewInstagramScraper(),
		NewFacebookScraper(),
		NewTwitterScraper(),
	}

	for i, s := range scrapers {
		_, err := s.Extract(ctx, "https://example.com/test")
		if err == nil {
			t.Errorf("scraper[%d] expected error with cancelled context", i)
		}
	}
}

// TestPlatformScrapers_ReturnExtractionErrors verifies that all fully
// implemented platform scrapers return descriptive errors when given a
// URL that does not contain video metadata (e.g. https://example.com).
// These are no longer stubs — they attempt real extraction and fail
// gracefully with platform-specific error messages.
func TestPlatformScrapers_ReturnExtractionErrors(t *testing.T) {
	scrapers := map[string]domain.VideoScraper{
		"tiktok":    NewTikTokScraper(),
		"instagram": NewInstagramScraper(),
		"facebook":  NewFacebookScraper(),
		"twitter":   NewTwitterScraper(),
	}

	for name, s := range scrapers {
		t.Run(name, func(t *testing.T) {
			_, err := s.Extract(context.Background(), "https://example.com")
			if err == nil {
				t.Fatal("expected error")
			}
			// Must start with platform prefix.
			if !strings.HasPrefix(err.Error(), name+":") {
				t.Errorf("error %q should start with %q", err.Error(), name+":")
			}
			// Must not be generic "not implemented".
			if strings.Contains(err.Error(), "not implemented") {
				t.Errorf("scraper is fully implemented, should not say 'not implemented': %q", err.Error())
			}
		})
	}
}
