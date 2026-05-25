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

// FacebookScraper extracts video metadata from Facebook.
// Facebook heavily requires authentication — this attempts public video extraction.
type FacebookScraper struct {
	client *http.Client
}

func NewFacebookScraper() *FacebookScraper {
	return &FacebookScraper{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Extract fetches a Facebook video page and extracts video metadata.
func (s *FacebookScraper) Extract(ctx context.Context, url string) (*domain.VideoMetadata, error) {
	log.Printf("[FacebookScraper] Extracting: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("facebook: create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook: fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facebook: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("facebook: read body: %w", err)
	}

	html := string(body)

	// Try JSON-LD structured data.
	meta, err := extractJSONLD(html)
	if err == nil {
		log.Printf("[FacebookScraper] Found video via JSON-LD: %s", meta.Title)
		return meta, nil
	}

	// Try extracting from embedded player config.
	meta = extractFacebookPlayerConfig(html)
	if meta != nil {
		log.Printf("[FacebookScraper] Found video via player config: %s", meta.Title)
		return meta, nil
	}

	return nil, fmt.Errorf("facebook: could not extract video metadata (may require login)")
}

func extractFacebookPlayerConfig(html string) *domain.VideoMetadata {
	// Facebook embeds video data in a script with type "application/json"
	// inside a div with data-sigil="player".
	marker := `"playable_url"`
	idx := strings.Index(html, marker)
	if idx == -1 {
		return nil
	}

	// Find the enclosing JSON object.
	start := idx
	for start > 0 && html[start] != '{' {
		start--
	}
	if html[start] != '{' {
		return nil
	}

	block, err := extractJSONBlock(html, start)
	if err != nil {
		return nil
	}

	var data struct {
		PlayableURL  string `json:"playable_url"`
		PlayableURLHD string `json:"playable_url_hd"`
		Title        string `json:"title"`
		Thumbnail    string `json:"thumbnail"`
	}
	if err := json.Unmarshal([]byte(block), &data); err != nil {
		return nil
	}
	if data.PlayableURL == "" {
		return nil
	}

	formats := []domain.DownloadFormat{
		{Quality: "SD", Extension: "mp4", FileSize: "Unknown", DownloadURL: data.PlayableURL},
	}
	if data.PlayableURLHD != "" {
		formats = append(formats, domain.DownloadFormat{
			Quality: "HD", Extension: "mp4", FileSize: "Unknown", DownloadURL: data.PlayableURLHD,
		})
	}

	meta := &domain.VideoMetadata{
		Title:        data.Title,
		ThumbnailURL: data.Thumbnail,
		Formats:      formats,
	}
	if meta.Title == "" {
		meta.Title = "Facebook Video"
	}
	return meta
}

// extractJSONLD is a shared helper used by TikTok and Instagram too.
func extractJSONLD(html string) (*domain.VideoMetadata, error) {
	marker := `<script type="application/ld+json">`
	idx := strings.Index(html, marker)
	if idx == -1 {
		return nil, fmt.Errorf("JSON-LD not found")
	}

	start := idx + len(marker)
	block, err := extractJSONBlock(html, start)
	if err != nil {
		return nil, err
	}

	var data struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ContentURL  string `json:"contentUrl"`
		Thumbnail   string `json:"thumbnailUrl"`
	}
	if err := json.Unmarshal([]byte(block), &data); err != nil {
		return nil, err
	}
	if data.Name == "" && data.ContentURL == "" {
		return nil, fmt.Errorf("insufficient data in JSON-LD")
	}

	title := data.Name
	if title == "" {
		title = data.Description
	}
	if title == "" {
		title = "Video"
	}

	return &domain.VideoMetadata{
		Title:        title,
		ThumbnailURL: data.Thumbnail,
		Formats: []domain.DownloadFormat{
			{Quality: "default", Extension: "mp4", FileSize: "Unknown", DownloadURL: data.ContentURL},
		},
	}, nil
}
