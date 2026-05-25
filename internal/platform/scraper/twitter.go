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

// TwitterScraper extracts video metadata from Twitter/X.
// Twitter/X recently requires authentication for most content.
type TwitterScraper struct {
	client *http.Client
}

func NewTwitterScraper() *TwitterScraper {
	return &TwitterScraper{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Extract fetches a tweet page and extracts video metadata.
func (s *TwitterScraper) Extract(ctx context.Context, url string) (*domain.VideoMetadata, error) {
	log.Printf("[TwitterScraper] Extracting: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("twitter: create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twitter: fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitter: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("twitter: read body: %w", err)
	}

	html := string(body)

	// Try JSON-LD structured data.
	meta, err := extractTwitterJSONLD(html)
	if err == nil {
		log.Printf("[TwitterScraper] Found video via JSON-LD: %s", meta.Title)
		return meta, nil
	}

	// Try extracting from the initial state JSON.
	meta = extractTwitterInitialState(html)
	if meta != nil {
		log.Printf("[TwitterScraper] Found video via initial state: %s", meta.Title)
		return meta, nil
	}

	return nil, fmt.Errorf("twitter: could not extract video metadata (may require login)")
}

func extractTwitterJSONLD(html string) (*domain.VideoMetadata, error) {
	// Twitter uses multiple JSON-LD blocks. Find the one with video info.
	marker := `<script type="application/ld+json">`
	var lastErr error

	for {
		idx := strings.Index(html, marker)
		if idx == -1 {
			break
		}
		start := idx + len(marker)
		block, err := extractJSONBlock(html, start)
		if err != nil {
			lastErr = err
			html = html[start:]
			continue
		}

		var data struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			ContentURL  string `json:"contentUrl"`
			Thumbnail   struct {
				URL string `json:"url"`
			} `json:"thumbnail"`
		}
		if err := json.Unmarshal([]byte(block), &data); err != nil {
			lastErr = err
			html = html[start:]
			continue
		}

		if data.ContentURL != "" && strings.Contains(data.ContentURL, "video") {
			return &domain.VideoMetadata{
				Title:        data.Name,
				ThumbnailURL: data.Thumbnail.URL,
				Formats: []domain.DownloadFormat{
					{Quality: "default", Extension: "mp4", FileSize: "Unknown", DownloadURL: data.ContentURL},
				},
			}, nil
		}
		html = html[start:]
	}

	return nil, fmt.Errorf("no video JSON-LD found: %v", lastErr)
}

func extractTwitterInitialState(html string) *domain.VideoMetadata {
	// Twitter/X embeds initial state in __NEXT_DATA__ or similar.
	markers := []string{"__NEXT_DATA__", `"entryData"`, `"tweet"`}
	for _, m := range markers {
		idx := strings.Index(html, m)
		if idx == -1 {
			continue
		}
		start := idx + len(m)
		start = skipToBrace(html, start)
		if start == -1 {
			continue
		}
		block, err := extractJSONBlock(html, start)
		if err != nil {
			continue
		}

		// Look for video URL in the block.
		vidMarker := `"video_url"`
		if !strings.Contains(block, vidMarker) {
			continue
		}
		vidIdx := strings.Index(block, vidMarker)
		// Extract the URL value after the key.
		valStart := vidIdx + len(vidMarker) + strings.Index(block[vidIdx+len(vidMarker):], `"`) + 1 // skip `: "` to reach value start
		valEnd := strings.Index(block[valStart:], `"`)
		if valEnd == -1 {
			continue
		}
		videoURL := block[valStart : valStart+valEnd]

		return &domain.VideoMetadata{
			Title: "Twitter Video",
			Formats: []domain.DownloadFormat{
				{Quality: "default", Extension: "mp4", FileSize: "Unknown", DownloadURL: videoURL},
			},
		}
	}
	return nil
}
