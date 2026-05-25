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

const (
	youtubeUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"
	youtubeTimeout   = 30 * time.Second
)

// YouTubeScraper extracts video metadata from YouTube watch pages.
type YouTubeScraper struct {
	client *http.Client
}

// NewYouTubeScraper creates a scraper with a 30-second HTTP client timeout.
func NewYouTubeScraper() *YouTubeScraper {
	return &YouTubeScraper{
		client: &http.Client{Timeout: youtubeTimeout},
	}
}

// Extract implements domain.VideoScraper.
//
// It uses a multi-strategy approach to find video metadata in the page:
//  1. Fast path: search for known JavaScript variable markers
//  2. Fallback: scan ALL script tags for any JSON matching our schema
//
// This makes it resilient to YouTube changing variable names — as long as the
// JSON structure remains similar, we'll find it.
func (s *YouTubeScraper) Extract(ctx context.Context, url string) (*domain.VideoMetadata, error) {
	log.Printf("[YouTubeScraper] Extracting: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("youtube: create request: %w", err)
	}
	req.Header.Set("User-Agent", youtubeUserAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube: fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("youtube: read body: %w", err)
	}

	html := string(body)

	// Strategy 1: Fast path — search for known JavaScript variable markers.
	page, err := findByMarkers(html)
	if err != nil {
		// Strategy 2: Broad search — scan every JSON object in every <script> tag.
		log.Printf("[YouTubeScraper] Marker search failed, scanning all script tags...")
		page, err = findAnyPlayerResponse(html)
		if err != nil {
			return nil, fmt.Errorf("youtube: %w", err)
		}
	}

	// Check if the video is playable.
	if page.PlayabilityStatus != nil && page.PlayabilityStatus.Status != "OK" {
		reason := page.PlayabilityStatus.Reason
		if reason == "" {
			reason = "unknown reason"
		}
		return nil, fmt.Errorf("youtube: video not playable: %s", reason)
	}

	if page.VideoDetails == nil {
		return nil, fmt.Errorf("youtube: response missing videoDetails")
	}

	details := page.VideoDetails
	meta := &domain.VideoMetadata{
		ID:    details.VideoID,
		Title: details.Title,
	}

	// Duration: convert "seconds" string to "MM:SS" or "H:MM:SS".
	if details.LengthSeconds != "" {
		var secs int
		if _, err := fmt.Sscanf(details.LengthSeconds, "%d", &secs); err == nil {
			meta.Duration = fmt.Sprintf("%d:%02d", secs/60, secs%60)
		}
	}

	// Thumbnail: pick the highest resolution available (last entry).
	if details.Thumbnail != nil && len(details.Thumbnail.Thumbnails) > 0 {
		meta.ThumbnailURL = details.Thumbnail.Thumbnails[len(details.Thumbnail.Thumbnails)-1].URL
	}

	// Process formats.
	if page.StreamingData != nil {
		seen := make(map[string]bool, len(page.StreamingData.Formats)+len(page.StreamingData.AdaptiveFormats))
		addUnique := func(f youtubeFormat) {
			if f.URL == "" || seen[f.URL] {
				return
			}
			seen[f.URL] = true

			df := domain.DownloadFormat{
				Quality:     f.label(),
				Extension:   f.extension(),
				FileSize:    f.humanSize(),
				DownloadURL: f.URL,
			}
			meta.Formats = append(meta.Formats, df)
		}

		for _, f := range page.StreamingData.Formats {
			addUnique(f)
		}
		for _, f := range page.StreamingData.AdaptiveFormats {
			addUnique(f)
		}
	}

	if len(meta.Formats) == 0 {
		log.Printf("[YouTubeScraper] No downloadable formats found (encrypted or restricted)")
	}

	log.Printf("[YouTubeScraper] OK: %q (%d formats)", meta.Title, len(meta.Formats))
	return meta, nil
}

// =============================================================================
// Multi-strategy JSON extraction
// =============================================================================

// playerResponse is the minimal struct we need from YouTube's massive JSON.
type playerResponse struct {
	PlayabilityStatus *struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"playabilityStatus"`
	VideoDetails *struct {
		VideoID       string `json:"videoId"`
		Title         string `json:"title"`
		LengthSeconds string `json:"lengthSeconds"`
		Thumbnail     *struct {
			Thumbnails []struct {
				URL string `json:"url"`
			} `json:"thumbnails"`
		} `json:"thumbnail"`
	} `json:"videoDetails"`
	StreamingData *struct {
		Formats         []youtubeFormat `json:"formats"`
		AdaptiveFormats []youtubeFormat `json:"adaptiveFormats"`
	} `json:"streamingData"`
}

// findByMarkers tries to find the player response near known variable names.
// This is the fast path covering ~99% of YouTube page formats.
func findByMarkers(html string) (*playerResponse, error) {
	markers := []string{
		"ytInitialPlayerResponse", // Primary: stable since ~2015
		"player_response",         // Used in embed pages
		"ytInitialData",           // Contains similar data for some pages
	}

	for _, marker := range markers {
		idx := strings.Index(html, marker)
		if idx == -1 {
			continue
		}

		// Find the opening brace after the marker.
		start := idx + len(marker)
		start = skipToBrace(html, start)
		if start == -1 {
			continue
		}

		// Extract and parse the JSON block.
		block, err := extractJSONBlock(html, start)
		if err != nil {
			continue
		}

		var pr playerResponse
		if err := json.Unmarshal([]byte(block), &pr); err != nil {
			continue
		}

		// Validate: must have videoDetails or playabilityStatus to be useful.
		if pr.VideoDetails != nil || pr.PlayabilityStatus != nil {
			log.Printf("[YouTubeScraper] Found player response via marker %q", marker)
			return &pr, nil
		}
	}

	return nil, fmt.Errorf("no player response found via markers")
}

// findAnyPlayerResponse scans every <script> tag in the HTML, extracts every
// JSON object, and tries to parse each one as a playerResponse.
//
// This is the broad fallback that catches any JSON structure, regardless of
// the JavaScript variable name YouTube uses to embed it.
func findAnyPlayerResponse(html string) (*playerResponse, error) {
	// Extract all <script>...</script> contents.
	scriptContents := extractScriptTags(html)

	for _, script := range scriptContents {
		// Find every JSON object in this script.
		for i := 0; i < len(script); i++ {
			if script[i] != '{' {
				continue
			}

			block, err := extractJSONBlock(script, i)
			if err != nil {
				continue
			}

			// Quick check: must contain key fields we need.
			if !strings.Contains(block, `"videoDetails"`) &&
				!strings.Contains(block, `"playabilityStatus"`) {
				continue
			}

			// Try to parse as playerResponse.
			var pr playerResponse
			if err := json.Unmarshal([]byte(block), &pr); err != nil {
				continue
			}

			if pr.VideoDetails != nil && pr.VideoDetails.VideoID != "" {
				log.Printf("[YouTubeScraper] Found player response via script scan (len=%d)", len(block))
				return &pr, nil
			}
			if pr.PlayabilityStatus != nil {
				log.Printf("[YouTubeScraper] Found unplayable response via script scan")
				return &pr, nil
			}
		}
	}

	return nil, fmt.Errorf("no player response found in any script tag")
}

// extractScriptTags returns the content of all <script>...</script> blocks.
func extractScriptTags(html string) []string {
	var scripts []string
	for {
		start := strings.Index(html, "<script")
		if start == -1 {
			break
		}
		// Find the end of the opening tag.
		closeTag := strings.Index(html[start:], ">")
		if closeTag == -1 {
			break
		}
		start += closeTag + 1

		// Find </script>.
		end := strings.Index(html[start:], "</script>")
		if end == -1 {
			break
		}

		scripts = append(scripts, html[start:start+end])
		html = html[start+end:]
	}
	return scripts
}

// skipToBrace advances past any non-brace characters (whitespace, assignment
// operators, identifiers) until it finds '{' or runs out of characters.
func skipToBrace(s string, start int) int {
	for i := start; i < len(s); i++ {
		b := s[i]
		if b == '{' {
			return i
		}
		// Skip whitespace, operators, quotes, brackets.
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' ||
			b == '=' || b == ':' || b == '"' || b == '\'' ||
			b == '[' || b == ']' || b == '(' || b == ')' {
			continue
		}
		// Skip alphanumeric / identifier characters.
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
			(b >= '0' && b <= '9') || b == '_' || b == '.' {
			continue
		}
		// Unknown character — stop.
		return -1
	}
	return -1
}

// =============================================================================
// YouTube format helpers
// =============================================================================

// youtubeFormat mirrors the format objects in YouTube's streamingData.
type youtubeFormat struct {
	MimeType      string `json:"mimeType"`
	ContentLength string `json:"contentLength"`
	QualityLabel  string `json:"qualityLabel"`
	URL           string `json:"url"`
	SignatureCipher string `json:"signatureCipher"`
	Cipher          string `json:"cipher"`
}

func (f youtubeFormat) label() string {
	if f.QualityLabel != "" {
		return f.QualityLabel
	}
	if strings.Contains(f.MimeType, "audio") {
		return "Audio"
	}
	return "Unknown"
}

func (f youtubeFormat) extension() string {
	mime := strings.Split(f.MimeType, ";")[0]
	parts := strings.Split(mime, "/")
	if len(parts) == 2 {
		switch parts[1] {
		case "mp4":
			return "mp4"
		case "webm":
			return "webm"
		case "3gpp":
			return "3gp"
		case "x-flv":
			return "flv"
		}
	}
	return "mp4"
}

func (f youtubeFormat) humanSize() string {
	if f.ContentLength == "" {
		return "Unknown"
	}
	var b uint64
	if _, err := fmt.Sscanf(f.ContentLength, "%d", &b); err != nil || b == 0 {
		return "Unknown"
	}
	switch {
	case b >= 1_000_000_000:
		return fmt.Sprintf("%.1f GB", float64(b)/1_000_000_000)
	case b >= 1_000_000:
		return fmt.Sprintf("%.1f MB", float64(b)/1_000_000)
	case b >= 1_000:
		return fmt.Sprintf("%.1f KB", float64(b)/1_000)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// extractJSONBlock extracts a balanced JSON object starting at startIdx.
// It correctly handles strings, escaped characters, and nested braces.
func extractJSONBlock(html string, startIdx int) (string, error) {
	if startIdx >= len(html) || html[startIdx] != '{' {
		return "", fmt.Errorf("extractJSONBlock: invalid JSON in page response (startIdx=%d)", startIdx)
	}

	depth := 0
	inStr := false
	esc := false

	for i := startIdx; i < len(html); i++ {
		b := html[i]

		if esc {
			esc = false
			continue
		}
		if b == '\\' && inStr {
			esc = true
			continue
		}
		if b == '"' {
			inStr = !inStr
			continue
		}
		if !inStr {
			switch b {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return html[startIdx : i+1], nil
				}
			}
		}
	}

	return "", fmt.Errorf("unbalanced JSON object (depth=%d)", depth)
}
