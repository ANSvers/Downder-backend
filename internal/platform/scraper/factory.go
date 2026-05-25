// Package scraper provides platform-specific video scrapers for social media sites.
//
// Usage:
//
//	scraper, err := scraper.GetScraper("https://youtube.com/watch?v=abc")
//	if err != nil { /* handle unsupported platform */ }
//	meta, err := scraper.Extract(ctx, "https://youtube.com/watch?v=abc")
//	if err != nil { /* handle extraction failure */ }
//	// meta.Title, meta.Formats, etc.
//
// Adding a new platform:
//  1. Create a file (e.g., vimeo.go) with a type that implements domain.VideoScraper
//  2. Add the pattern and constructor to the registry in this file
//  3. Write tests
//
// Currently supported: YouTube, TikTok, Instagram, Facebook, Twitter/X.
// All platforms are fully implemented with multi-strategy extraction.
package scraper

import (
	"fmt"
	"net/url"
	"strings"

	"downder-backend/internal/domain"
)

// scraperEntry maps a URL hostname pattern to its scraper constructor.
type scraperEntry struct {
	pattern string
	factory func() domain.VideoScraper
}

// registry is ordered: first match wins. Read-only after init, so concurrent-safe.
//
// Patterns are matched against the URL hostname. For example, "youtube.com" matches
// both "youtube.com" and "www.youtube.com" (via subdomain suffix), but does NOT match
// "youtube.com.evil.com" or "evil-youtube.com" (unlike substring matching).
var registry = []scraperEntry{
	{pattern: "youtube.com", factory: func() domain.VideoScraper { return NewYouTubeScraper() }},
	{pattern: "youtu.be", factory: func() domain.VideoScraper { return NewYouTubeScraper() }},
	{pattern: "tiktok.com", factory: func() domain.VideoScraper { return NewTikTokScraper() }},
	{pattern: "instagram.com", factory: func() domain.VideoScraper { return NewInstagramScraper() }},
	{pattern: "facebook.com", factory: func() domain.VideoScraper { return NewFacebookScraper() }},
	{pattern: "fb.com", factory: func() domain.VideoScraper { return NewFacebookScraper() }},
	{pattern: "x.com", factory: func() domain.VideoScraper { return NewTwitterScraper() }},
	{pattern: "twitter.com", factory: func() domain.VideoScraper { return NewTwitterScraper() }},
}

// matchHost checks if hostname matches a registry pattern.
// A pattern matches if host equals the pattern (facebook.com) or is a subdomain
// of the pattern (www.facebook.com). Short patterns (youtu.be, x.com, fb.com)
// are exact-match only to prevent false positives.
func matchHost(host, pattern string) bool {
	if host == pattern {
		return true
	}
	// Subdomain check: "www.youtube.com" hasSuffix ".youtube.com"
	// But "youtube.com.evil.com" does NOT match because it would need
	// to end with ".youtube.com.evil.com" or equal "youtube.com.evil.com".
	if strings.HasSuffix(host, "."+pattern) {
		return true
	}
	return false
}

// GetScraper parses the URL by hostname and returns the matching scraper.
// Returns an error if the URL is malformed or no platform supports it.
func GetScraper(rawURL string) (domain.VideoScraper, error) {
	// Normalize: add https:// if no scheme is present, so hostname is parsed correctly.
	normalized := rawURL
	if !strings.Contains(normalized, "://") {
		normalized = "https://" + normalized
	}

	u, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("unsupported URL: %q has no recognizable hostname", rawURL)
	}

	for _, entry := range registry {
		if matchHost(host, entry.pattern) {
			return entry.factory(), nil
		}
	}

	return nil, fmt.Errorf("unsupported URL: %q does not match any known platform", rawURL)
}

// SupportedPlatforms returns de-duplicated platform identifiers.
func SupportedPlatforms() []string {
	seen := make(map[string]bool, len(registry))
	platforms := make([]string, 0, len(registry))
	for _, entry := range registry {
		if !seen[entry.pattern] {
			seen[entry.pattern] = true
			platforms = append(platforms, entry.pattern)
		}
	}
	return platforms
}
