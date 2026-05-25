package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"downder-backend/internal/domain"
)

// InstagramScraper extracts video metadata from Instagram.
// Instagram requires authentication for most content — this scraper
// attempts public-only extraction via embedded JSON data.
type InstagramScraper struct {
	client *http.Client
}

func NewInstagramScraper() *InstagramScraper {
	return &InstagramScraper{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Extract fetches an Instagram post/reel page and extracts video metadata.
func (s *InstagramScraper) Extract(ctx context.Context, url string) (*domain.VideoMetadata, error) {
	log.Printf("[InstagramScraper] Extracting: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("instagram: create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram: fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instagram: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("instagram: read body: %w", err)
	}

	html := string(body)

	// Try __NEXT_DATA__ first (public pages).
	meta := extractNextData(html)
	if meta != nil {
		log.Printf("[InstagramScraper] Found video via next data: %s", meta.Title)
		return meta, nil
	}

	// Try JSON-LD.
	meta = extractInstagramJSONLD(html)
	if meta != nil {
		log.Printf("[InstagramScraper] Found video via JSON-LD: %s", meta.Title)
		return meta, nil
	}

	return nil, fmt.Errorf("instagram: could not extract metadata (may require login)")
}

func extractNextData(html string) *domain.VideoMetadata {
	marker := `__NEXT_DATA__`
	idx := strings.Index(html, marker)
	if idx == -1 {
		return nil
	}

	start := idx + len(marker)
	start = skipToBrace(html, start)
	if start == -1 {
		return nil
	}

	block, err := extractJSONBlock(html, start)
	if err != nil {
		return nil
	}

	var data struct {
		Props struct {
			PageProps struct {
				Media struct {
					VideoURL  string `json:"video_url"`
					Thumbnail string `json:"thumbnail_src"`
					Caption   string `json:"caption"`
				} `json:"media"`
				User struct {
					FullName string `json:"full_name"`
				} `json:"user"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal([]byte(block), &data); err != nil {
		return nil
	}
	if data.Props.PageProps.Media.VideoURL == "" {
		return nil
	}

	return &domain.VideoMetadata{
		Title:        data.Props.PageProps.Media.Caption,
		ThumbnailURL: data.Props.PageProps.Media.Thumbnail,
		Formats: []domain.DownloadFormat{
			{Quality: "default", Extension: "mp4", FileSize: "Unknown", DownloadURL: data.Props.PageProps.Media.VideoURL},
		},
	}
}

func extractInstagramJSONLD(html string) *domain.VideoMetadata {
	meta, err := extractJSONLD(html)
	if err != nil {
		return nil
	}
	return meta
}
