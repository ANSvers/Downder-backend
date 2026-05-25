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

// TikTokScraper extracts video metadata from TikTok.
type TikTokScraper struct {
	client *http.Client
}

func NewTikTokScraper() *TikTokScraper {
	return &TikTokScraper{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *TikTokScraper) Extract(ctx context.Context, url string) (*domain.VideoMetadata, error) {
	log.Printf("[TikTokScraper] Extracting: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("tiktok: create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tiktok: fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tiktok: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tiktok: read body: %w", err)
	}

	html := string(body)

	// Try JSON-LD structured data first.
	meta, err := extractJSONLD(html)
	if err == nil && meta != nil {
		log.Printf("[TikTokScraper] Found video via JSON-LD: %s", meta.Title)
		return meta, nil
	}

	// Fallback: try to find __INITIAL_STATE__ JSON.
	meta = extractTikTokInitialState(html)
	if meta != nil {
		log.Printf("[TikTokScraper] Found video via initial state: %s", meta.Title)
		return meta, nil
	}

	return nil, fmt.Errorf("tiktok: could not extract video metadata from page")
}

func extractTikTokInitialState(html string) *domain.VideoMetadata {
	marker := `__INITIAL_STATE__`
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

	var state struct {
		ItemInfo struct {
			ItemStruct struct {
				Video struct {
					ID       string `json:"id"`
					Width    int    `json:"width"`
					Height   int    `json:"height"`
					Duration int    `json:"duration"`
				} `json:"video"`
				Desc   string `json:"desc"`
				Author string `json:"author"`
			} `json:"itemStruct"`
		} `json:"itemInfo"`
	}
	if err := json.Unmarshal([]byte(block), &state); err != nil {
		return nil
	}
	if state.ItemInfo.ItemStruct.Video.ID == "" {
		return nil
	}

	meta := &domain.VideoMetadata{
		ID:    state.ItemInfo.ItemStruct.Video.ID,
		Title: state.ItemInfo.ItemStruct.Desc,
	}
	if meta.Title == "" {
		meta.Title = "TikTok Video"
	}
	if state.ItemInfo.ItemStruct.Video.Duration > 0 {
		d := state.ItemInfo.ItemStruct.Video.Duration
		meta.Duration = fmt.Sprintf("%d:%02d", d/60, d%60)
	}
	meta.Formats = []domain.DownloadFormat{
		{Quality: "default", Extension: "mp4", FileSize: "Unknown", DownloadURL: ""},
	}
	return meta
}
