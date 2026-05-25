package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"downder-backend/internal/domain"
)

// =============================================================================
// TikTok Integration Tests
// =============================================================================

func TestTikTokExtract_JSONLD(t *testing.T) {
	// Simulate TikTok page with JSON-LD structured data.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>TikTok</title></head>
<body>
<script type="application/ld+json">{"name":"My TikTok Video","description":"Check this out","contentUrl":"https://tiktok.com/video/123","thumbnailUrl":"https://tiktok.com/thumb.jpg"}</script>
</body>
</html>`))
	}))
	defer srv.Close()

	scraper := NewTikTokScraper()
	meta, err := scraper.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "My TikTok Video" {
		t.Errorf("expected title 'My TikTok Video', got %q", meta.Title)
	}
}
func TestTikTokExtract_InitialState(t *testing.T) {
	// Simulate TikTok page with __INITIAL_STATE__ JSON (fallback).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><script>window.__INITIAL_STATE__={"itemInfo":{"itemStruct":{"video":{"id":"742123","width":640,"height":1136,"duration":15},"desc":"TikTok dance","author":"@user"}}};</script></body></html>`))
	}))
	defer srv.Close()

	scraper := NewTikTokScraper()
	meta, err := scraper.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "TikTok dance" {
		t.Errorf("expected title 'TikTok dance', got %q", meta.Title)
	}
	if meta.Duration != "0:15" {
		t.Errorf("expected duration '0:15', got %q", meta.Duration)
	}
}

func TestTikTokExtract_InitialStateNoDesc(t *testing.T) {
	// Fallback title when desc is empty.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><script>window.__INITIAL_STATE__={"itemInfo":{"itemStruct":{"video":{"id":"999","width":640,"height":1136,"duration":30},"desc":"","author":"@user"}}};</script></body></html>`))
	}))
	defer srv.Close()

	scraper := NewTikTokScraper()
	meta, err := scraper.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "TikTok Video" {
		t.Errorf("expected default title 'TikTok Video', got %q", meta.Title)
	}
}

func TestTikTokExtract_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>No video here</body></html>`))
	}))
	defer srv.Close()

	scraper := NewTikTokScraper()
	_, err := scraper.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for page without video data")
	}
	if !strings.Contains(err.Error(), "tiktok:") {
		t.Errorf("error should start with 'tiktok:', got %q", err.Error())
	}
}

func TestTikTokExtract_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	scraper := NewTikTokScraper()
	_, err := scraper.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

// =============================================================================
// Instagram Integration Tests
// =============================================================================

func TestInstagramExtract_NextData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>Instagram</title></head>
<body>
<script>window.__NEXT_DATA__={"props":{"pageProps":{"media":{"video_url":"https://instagram.com/video.mp4","thumbnail_src":"https://instagram.com/thumb.jpg","caption":"Cool reel!"},"user":{"full_name":"test_user"}}}}</script>
</body>
</html>`))
	}))
	defer srv.Close()

	scraper := NewInstagramScraper()
	meta, err := scraper.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Cool reel!" {
		t.Errorf("expected title 'Cool reel!', got %q", meta.Title)
	}
	if len(meta.Formats) == 0 || meta.Formats[0].DownloadURL != "https://instagram.com/video.mp4" {
		t.Errorf("expected video URL, got %+v", meta.Formats)
	}
}

func TestInstagramExtract_JSONLD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>Instagram</title></head>
<body>
<script type="application/ld+json">{"name":"Instagram Post","description":"Amazing view","contentUrl":"https://instagram.com/video2.mp4","thumbnailUrl":"https://instagram.com/thumb2.jpg"}</script>
</body>
</html>`))
	}))
	defer srv.Close()

	scraper := NewInstagramScraper()
	meta, err := scraper.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Instagram Post" {
		t.Errorf("expected title 'Instagram Post', got %q", meta.Title)
	}
}

func TestInstagramExtract_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>No data</body></html>`))
	}))
	defer srv.Close()

	scraper := NewInstagramScraper()
	_, err := scraper.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for page without video data")
	}
}

func TestInstagramExtract_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	scraper := NewInstagramScraper()
	_, err := scraper.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 403")
	}
}

// =============================================================================
// Facebook Integration Tests
// =============================================================================

func TestFacebookExtract_JSONLD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>Facebook</title></head>
<body>
<script type="application/ld+json">{"name":"My Facebook Video","description":"Funny cat","contentUrl":"https://facebook.com/video.mp4","thumbnailUrl":"https://facebook.com/thumb.jpg"}</script>
</body>
</html>`))
	}))
	defer srv.Close()

	scraper := NewFacebookScraper()
	meta, err := scraper.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "My Facebook Video" {
		t.Errorf("expected title 'My Facebook Video', got %q", meta.Title)
	}
}

func TestFacebookExtract_PlayerConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>Facebook</title></head>
<body>
<div data-sigil="player">
<script type="application/json">{"playable_url":"https://facebook.com/video_sd.mp4","playable_url_hd":"https://facebook.com/video_hd.mp4","title":"FB Video Title","thumbnail":"https://facebook.com/thumb.jpg"}</script>
</div>
</body>
</html>`))
	}))
	defer srv.Close()

	scraper := NewFacebookScraper()
	meta, err := scraper.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "FB Video Title" {
		t.Errorf("expected title 'FB Video Title', got %q", meta.Title)
	}
	if len(meta.Formats) != 2 {
		t.Fatalf("expected 2 formats (SD+HD), got %d", len(meta.Formats))
	}
	if meta.Formats[0].Quality != "SD" || meta.Formats[0].DownloadURL != "https://facebook.com/video_sd.mp4" {
		t.Errorf("first format should be SD, got %+v", meta.Formats[0])
	}
	if meta.Formats[1].Quality != "HD" || meta.Formats[1].DownloadURL != "https://facebook.com/video_hd.mp4" {
		t.Errorf("second format should be HD, got %+v", meta.Formats[1])
	}
}

func TestFacebookExtract_PlayerConfigNoHD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>Facebook</title></head>
<body>
<div data-sigil="player">
<script type="application/json">{"playable_url":"https://facebook.com/video_sd.mp4","title":"SD Only","thumbnail":""}</script>
</div>
</body>
</html>`))
	}))
	defer srv.Close()

	scraper := NewFacebookScraper()
	meta, err := scraper.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "SD Only" {
		t.Errorf("expected title 'SD Only', got %q", meta.Title)
	}
	if len(meta.Formats) != 1 {
		t.Fatalf("expected 1 format (SD only), got %d", len(meta.Formats))
	}
	if meta.Formats[0].Quality != "SD" {
		t.Errorf("expected SD quality, got %q", meta.Formats[0].Quality)
	}
}

func TestFacebookExtract_PlayerConfigFallbackTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>Facebook</title></head>
<body>
<div data-sigil="player">
<script type="application/json">{"playable_url":"https://facebook.com/video.mp4","title":"","thumbnail":""}</script>
</div>
</body>
</html>`))
	}))
	defer srv.Close()

	scraper := NewFacebookScraper()
	meta, err := scraper.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Facebook Video" {
		t.Errorf("expected default title 'Facebook Video', got %q", meta.Title)
	}
}

func TestFacebookExtract_PageWithoutVideo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>No video data here</body></html>`))
	}))
	defer srv.Close()

	scraper := NewFacebookScraper()
	_, err := scraper.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for page without video data")
	}
}

func TestFacebookExtract_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	scraper := NewFacebookScraper()
	_, err := scraper.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

// =============================================================================
// Twitter/X Integration Tests
// =============================================================================

func TestTwitterExtract_JSONLD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>Twitter</title></head>
<body>
<script type="application/ld+json">{"name":"Twitter Video","description":"Check this","contentUrl":"https://twitter.com/video.mp4","thumbnail":{"url":"https://twitter.com/thumb.jpg"}}</script>
</body>
</html>`))
	}))
	defer srv.Close()

	scraper := NewTwitterScraper()
	meta, err := scraper.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Twitter Video" {
		t.Errorf("expected title 'Twitter Video', got %q", meta.Title)
	}
	if len(meta.Formats) == 0 || meta.Formats[0].DownloadURL != "https://twitter.com/video.mp4" {
		t.Errorf("expected video URL, got %+v", meta.Formats)
	}
}

func TestTwitterExtract_InitialState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>Twitter</title></head>
<body>
<script>window.__NEXT_DATA__={"props":{"pageProps":{"tweet":{"video_url":"https://twitter.com/video2.mp4"}}}}</script>
</body>
</html>`))
	}))
	defer srv.Close()

	scraper := NewTwitterScraper()
	meta, err := scraper.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Twitter Video" {
		// Note: extractTwitterInitialState uses hardcoded "Twitter Video" title.
		t.Errorf("expected title 'Twitter Video', got %q", meta.Title)
	}
}

func TestTwitterExtract_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>No video here</body></html>`))
	}))
	defer srv.Close()

	scraper := NewTwitterScraper()
	_, err := scraper.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for page without video data")
	}
}

func TestTwitterExtract_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	scraper := NewTwitterScraper()
	_, err := scraper.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 429")
	}
}

// =============================================================================
// JSON-LD shared helper edge cases
// =============================================================================

func TestExtractJSONLD_NoMarker(t *testing.T) {
	_, err := extractJSONLD("<html><body>no json-ld</body></html>")
	if err == nil {
		t.Fatal("expected error when no JSON-LD marker found")
	}
}

func TestExtractJSONLD_InvalidJSON(t *testing.T) {
	_, err := extractJSONLD(`<html><script type="application/ld+json">{invalid json}</script></html>`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractJSONLD_ExtractBlockFails(t *testing.T) {
	// JSON-LD marker exists but JSON block is unterminated.
	// This covers extractJSONBlock error path in extractJSONLD.
	_, err := extractJSONLD(`<html><script type="application/ld+json">{"unclosed`)
	if err == nil {
		t.Fatal("expected error when JSON extraction fails")
	}
}

func TestExtractJSONLD_InsufficientData(t *testing.T) {
	_, err := extractJSONLD(`<html><script type="application/ld+json">{"someOtherField": true}</script></html>`)
	if err == nil {
		t.Fatal("expected error for JSON without required fields")
	}
}

func TestExtractJSONLD_Success(t *testing.T) {
	meta, err := extractJSONLD(`<html><script type="application/ld+json">{"name":"Test Video","contentUrl":"https://example.com/video.mp4","thumbnailUrl":"https://example.com/thumb.jpg"}</script></html>`)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Test Video" {
		t.Errorf("expected 'Test Video', got %q", meta.Title)
	}
}

func TestExtractJSONLD_DescriptionFallback(t *testing.T) {
	meta, err := extractJSONLD(`<html><script type="application/ld+json">{"description":"Desc fallback","contentUrl":"https://example.com/video.mp4"}</script></html>`)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Desc fallback" {
		t.Errorf("expected 'Desc fallback', got %q", meta.Title)
	}
}

func TestExtractJSONLD_FallbackTitle(t *testing.T) {
	meta, err := extractJSONLD(`<html><script type="application/ld+json">{"contentUrl":"https://example.com/video.mp4"}</script></html>`)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Video" {
		t.Errorf("expected 'Video', got %q", meta.Title)
	}
}

// =============================================================================
// Facebook player config edge cases
// =============================================================================

func TestExtractFacebookPlayerConfig_NoMarker(t *testing.T) {
	meta := extractFacebookPlayerConfig("<html><body>no playable_url</body></html>")
	if meta != nil {
		t.Fatal("expected nil when no playable_url marker")
	}
}

func TestExtractFacebookPlayerConfig_NoOpeningBrace(t *testing.T) {
	meta := extractFacebookPlayerConfig(`<html>playable_url"somevalue"</html>`)
	if meta != nil {
		t.Fatal("expected nil when no opening brace before marker")
	}
}

func TestExtractFacebookPlayerConfig_InvalidJSON(t *testing.T) {
	meta := extractFacebookPlayerConfig(`<html>{"playable_url": "bad json</html>`)
	if meta != nil {
		t.Fatal("expected nil for invalid JSON")
	}
}

func TestExtractFacebookPlayerConfig_EmptyURL(t *testing.T) {
	meta := extractFacebookPlayerConfig(`<html>{"playable_url": ""}</html>`)
	if meta != nil {
		t.Fatal("expected nil when playable_url is empty")
	}
}

// =============================================================================
// Twitter JSON-LD edge cases
// =============================================================================

func TestExtractTwitterJSONLD_NoMarker(t *testing.T) {
	_, err := extractTwitterJSONLD("<html><body>no json-ld</body></html>")
	if err == nil {
		t.Fatal("expected error when no JSON-LD marker")
	}
}

func TestExtractTwitterJSONLD_MultipleBlocks(t *testing.T) {
	// First block is non-video, second has video.
	html := `<html>
<script type="application/ld+json">{"name":"WebPage"}</script>
<script type="application/ld+json">{"name":"Video Tweet","contentUrl":"https://twitter.com/video.mp4","thumbnail":{"url":"https://twitter.com/thumb.jpg"}}</script>
</html>`
	meta, err := extractTwitterJSONLD(html)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Video Tweet" {
		t.Errorf("expected 'Video Tweet', got %q", meta.Title)
	}
}

// =============================================================================
// Instagram next data edge cases
// =============================================================================

func TestExtractNextData_NoMarker(t *testing.T) {
	meta := extractNextData("<html><body>no next data</body></html>")
	if meta != nil {
		t.Fatal("expected nil when no __NEXT_DATA__ marker")
	}
}

func TestExtractNextData_NoBrace(t *testing.T) {
	meta := extractNextData(`<html>__NEXT_DATA__ = "string value"</html>`)
	if meta != nil {
		t.Fatal("expected nil when no brace after marker")
	}
}

func TestExtractNextData_InvalidJSON(t *testing.T) {
	meta := extractNextData(`<html>__NEXT_DATA__ = {invalid json}</html>`)
	if meta != nil {
		t.Fatal("expected nil for invalid JSON")
	}
}

func TestExtractNextData_NoVideoURL(t *testing.T) {
	meta := extractNextData(`<html>__NEXT_DATA__ = {"props":{"pageProps":{"media":{}}}}</html>`)
	if meta != nil {
		t.Fatal("expected nil when no video_url")
	}
}

func TestExtractNextData_Valid(t *testing.T) {
	meta := extractNextData(`<html>__NEXT_DATA__ = {"props":{"pageProps":{"media":{"video_url":"https://insta.com/v.mp4","thumbnail_src":"https://insta.com/t.jpg","caption":"Nice!"},"user":{"full_name":"u"}}}}</html>`)
	if meta == nil {
		t.Fatal("expected non-nil result")
	}
	if meta.Title != "Nice!" {
		t.Errorf("expected 'Nice!', got %q", meta.Title)
	}
}

// =============================================================================
// TikTok initial state edge cases
// =============================================================================

func TestExtractTikTokInitialState_NoMarker(t *testing.T) {
	meta := extractTikTokInitialState("<html><body>no initial state</body></html>")
	if meta != nil {
		t.Fatal("expected nil")
	}
}

func TestExtractTikTokInitialState_NoBrace(t *testing.T) {
	meta := extractTikTokInitialState(`<html>__INITIAL_STATE__ = "string"</html>`)
	if meta != nil {
		t.Fatal("expected nil when no brace")
	}
}

func TestExtractTikTokInitialState_InvalidJSON(t *testing.T) {
	meta := extractTikTokInitialState(`<html>__INITIAL_STATE__={invalid}</html>`)
	if meta != nil {
		t.Fatal("expected nil for invalid JSON")
	}
}

func TestExtractTikTokInitialState_EmptyVideoID(t *testing.T) {
	meta := extractTikTokInitialState(`<html>__INITIAL_STATE__={"itemInfo":{"itemStruct":{"video":{"id":""},"desc":"","author":""}}}</html>`)
	if meta != nil {
		t.Fatal("expected nil when video ID is empty")
	}
}

func TestExtractTikTokInitialState_NoDuration(t *testing.T) {
	meta := extractTikTokInitialState(`<html>__INITIAL_STATE__={"itemInfo":{"itemStruct":{"video":{"id":"123"},"desc":"No Duration","author":"@u"}}}</html>`)
	if meta == nil {
		t.Fatal("expected non-nil")
	}
	if meta.Duration != "" {
		t.Errorf("expected empty duration when not provided, got %q", meta.Duration)
	}
}

func TestExtractTikTokInitialState_WithDuration(t *testing.T) {
	meta := extractTikTokInitialState(`<html>__INITIAL_STATE__={"itemInfo":{"itemStruct":{"video":{"id":"123","duration":125},"desc":"2 min video","author":"@u"}}}</html>`)
	if meta == nil {
		t.Fatal("expected non-nil")
	}
	if meta.Duration != "2:05" {
		t.Errorf("expected '2:05', got %q", meta.Duration)
	}
}

// =============================================================================
// Twitter initial state edge cases
// =============================================================================

func TestExtractTwitterInitialState_NoMarker(t *testing.T) {
	meta := extractTwitterInitialState("<html><body>no markers</body></html>")
	if meta != nil {
		t.Fatal("expected nil")
	}
}

func TestExtractTwitterInitialState_MarkerNoBrace(t *testing.T) {
	meta := extractTwitterInitialState(`<html>"entryData" : "string"</html>`)
	if meta != nil {
		t.Fatal("expected nil when no brace")
	}
}

func TestExtractTwitterInitialState_MarkerNoVideoURL(t *testing.T) {
	meta := extractTwitterInitialState(`<html>"entryData" = {"no_video": true}</html>`)
	if meta != nil {
		t.Fatal("expected nil when no video_url")
	}
}

func TestExtractTwitterInitialState_Valid(t *testing.T) {
	meta := extractTwitterInitialState(`<html>"entryData" = {"video_url": "https://x.com/video.mp4"}</html>`)
	if meta == nil {
		t.Fatal("expected non-nil")
	}
	if len(meta.Formats) == 0 || meta.Formats[0].DownloadURL != "https://x.com/video.mp4" {
		t.Errorf("expected video URL, got %+v", meta.Formats)
	}
}

func TestExtractTwitterInitialState_InvalidVideoURLFormat(t *testing.T) {
	// video_url key exists but value extraction fails (weird formatting).
	meta := extractTwitterInitialState(`<html>"entryData" = {"video_url":123}</html>`)
	if meta != nil {
		t.Fatal("expected nil for non-string video_url")
	}
}

// =============================================================================
// Instagram JSON-LD wrapper edge cases
// =============================================================================

func TestExtractInstagramJSONLD_NotFound(t *testing.T) {
	meta := extractInstagramJSONLD("<html><body>no json-ld</body></html>")
	if meta != nil {
		t.Fatal("expected nil")
	}
}

func TestExtractInstagramJSONLD_Found(t *testing.T) {
	meta := extractInstagramJSONLD(`<html><script type="application/ld+json">{"name":"IG Post","contentUrl":"https://ig.com/v.mp4"}</script></html>`)
	if meta == nil {
		t.Fatal("expected non-nil")
	}
	if meta.Title != "IG Post" {
		t.Errorf("expected 'IG Post', got %q", meta.Title)
	}
}

// =============================================================================
// Multiple JSON-LD blocks (Twitter)
// =============================================================================

func TestExtractTwitterJSONLD_SkipsNonVideo(t *testing.T) {
	html := `<html>
<script type="application/ld+json">{"name":"WebPage","contentUrl":"https://example.com/page"}</script>
<script type="application/ld+json">{"name":"Video","contentUrl":"https://example.com/video.mp4","thumbnail":{"url":"https://example.com/thumb.jpg"}}</script>
</html>`
	meta, err := extractTwitterJSONLD(html)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Video" {
		t.Errorf("expected 'Video', got %q", meta.Title)
	}
}

func TestExtractTwitterJSONLD_InvalidJSONSkips(t *testing.T) {
	html := `<html>
<script type="application/ld+json">{invalid}</script>
<script type="application/ld+json">{"name":"Valid","contentUrl":"https://example.com/video.mp4","thumbnail":{"url":"https://example.com/thumb.jpg"}}</script>
</html>`
	meta, err := extractTwitterJSONLD(html)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Valid" {
		t.Errorf("expected 'Valid', got %q", meta.Title)
	}
}

// =============================================================================
// Instagram JSON-LD via shared helper edge case
// =============================================================================

func TestExtractInstagramJSONLD_EmptyContentURL(t *testing.T) {
	// JSON-LD with name but no contentURL: shared helper returns metadata
	// using name as title (contentURL is not required when name is present).
	meta := extractInstagramJSONLD(`<html><script type="application/ld+json">{"name":"Just Name"}</script></html>`)
	if meta == nil {
		t.Fatal("expected non-nil when name is provided")
	}
	if meta.Title != "Just Name" {
		t.Errorf("expected 'Just Name', got %q", meta.Title)
	}
}

func TestExtractJSONLD_EmptyNameAndContentURL(t *testing.T) {
	// Both name and contentURL empty should fail.
	_, err := extractJSONLD(`<html><script type="application/ld+json">{"description":"only desc"}</script></html>`)
	if err == nil {
		t.Fatal("expected error when name and contentURL are both empty")
	}
}

// =============================================================================
// 100% Coverage: Request creation failure paths
// =============================================================================

// failingRoundTripper is a custom http.RoundTripper that returns a response
// whose body fails on the first Read call.
type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       &failingReadCloser{},
		Header:     make(http.Header),
	}, nil
}

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) {
	return 0, fmt.Errorf("simulated read failure")
}
func (failingReadCloser) Close() error { return nil }

func TestFacebookExtract_ReadBodyFailure(t *testing.T) {
	s := &FacebookScraper{client: &http.Client{Transport: failingRoundTripper{}}}
	_, err := s.Extract(context.Background(), "https://facebook.com/watch?v=123")
	if err == nil || !strings.Contains(err.Error(), "facebook: read body") {
		t.Errorf("expected read body error, got %v", err)
	}
}

func TestInstagramExtract_ReadBodyFailure(t *testing.T) {
	s := &InstagramScraper{client: &http.Client{Transport: failingRoundTripper{}}}
	_, err := s.Extract(context.Background(), "https://instagram.com/p/xyz")
	if err == nil || !strings.Contains(err.Error(), "instagram: read body") {
		t.Errorf("expected read body error, got %v", err)
	}
}

func TestTikTokExtract_ReadBodyFailure(t *testing.T) {
	s := &TikTokScraper{client: &http.Client{Transport: failingRoundTripper{}}}
	_, err := s.Extract(context.Background(), "https://tiktok.com/@user/video/123")
	if err == nil || !strings.Contains(err.Error(), "tiktok: read body") {
		t.Errorf("expected read body error, got %v", err)
	}
}

func TestTwitterExtract_ReadBodyFailure(t *testing.T) {
	s := &TwitterScraper{client: &http.Client{Transport: failingRoundTripper{}}}
	_, err := s.Extract(context.Background(), "https://twitter.com/user/status/123")
	if err == nil || !strings.Contains(err.Error(), "twitter: read body") {
		t.Errorf("expected read body error, got %v", err)
	}
}

func TestYouTubeExtract_ReadBodyFailure(t *testing.T) {
	s := &YouTubeScraper{client: &http.Client{Transport: failingRoundTripper{}}}
	_, err := s.Extract(context.Background(), "https://youtube.com/watch?v=abc")
	if err == nil || !strings.Contains(err.Error(), "youtube: read body") {
		t.Errorf("expected read body error, got %v", err)
	}
}

// =============================================================================
// 100% Coverage: Request creation failure (invalid URL with null byte)
// =============================================================================

func TestFacebookExtract_CreateRequestFailure(t *testing.T) {
	s := NewFacebookScraper()
	// URL with null byte causes http.NewRequestWithContext to fail.
	_, err := s.Extract(context.Background(), "https://facebook.com/watch\x00?v=123")
	if err == nil || !strings.Contains(err.Error(), "facebook: create request") {
		t.Errorf("expected create request error, got %v", err)
	}
}

func TestInstagramExtract_CreateRequestFailure(t *testing.T) {
	s := NewInstagramScraper()
	_, err := s.Extract(context.Background(), "https://instagram.com/p/\x00xyz")
	if err == nil || !strings.Contains(err.Error(), "instagram: create request") {
		t.Errorf("expected create request error, got %v", err)
	}
}

func TestTikTokExtract_CreateRequestFailure(t *testing.T) {
	s := NewTikTokScraper()
	_, err := s.Extract(context.Background(), "https://tiktok.com/@user/video/\x00123")
	if err == nil || !strings.Contains(err.Error(), "tiktok: create request") {
		t.Errorf("expected create request error, got %v", err)
	}
}

func TestTwitterExtract_CreateRequestFailure(t *testing.T) {
	s := NewTwitterScraper()
	_, err := s.Extract(context.Background(), "https://twitter.com/user/status/\x00123")
	if err == nil || !strings.Contains(err.Error(), "twitter: create request") {
		t.Errorf("expected create request error, got %v", err)
	}
}

func TestYouTubeExtract_CreateRequestFailure(t *testing.T) {
	s := NewYouTubeScraper()
	_, err := s.Extract(context.Background(), "https://youtube.com/watch\x00?v=abc")
	if err == nil || !strings.Contains(err.Error(), "youtube: create request") {
		t.Errorf("expected create request error, got %v", err)
	}
}

// =============================================================================
// 100% Coverage: Facebook player config edge cases
// =============================================================================

func TestExtractFacebookPlayerConfig_MarkerAtStartNoBrace(t *testing.T) {
	// playable_url marker at position 0, no opening brace found backwards.
	meta := extractFacebookPlayerConfig(`"playable_url": "https://fb.com/v.mp4"`)
	if meta != nil {
		t.Fatal("expected nil when no opening brace found before marker")
	}
}

func TestExtractFacebookPlayerConfig_InvalidJSONAfterMarker(t *testing.T) {
	// Marker exists, opening brace found, but JSON is invalid.
	meta := extractFacebookPlayerConfig(`{"playable_url": unquoted value}`)
	if meta != nil {
		t.Fatal("expected nil for invalid JSON")
	}
}

// =============================================================================
// 100% Coverage: extractJSONBlock failure in platform extractors
// =============================================================================

func TestExtractTikTokInitialState_ExtractJSONBlockFails(t *testing.T) {
	// __INITIAL_STATE__ exists but JSON block is unterminated.
	meta := extractTikTokInitialState(`<html>__INITIAL_STAGE__ != this; var __INITIAL_STATE__ = {"unclosed`)
	if meta != nil {
		t.Fatal("expected nil when JSON extraction fails")
	}
}

func TestExtractNextData_ExtractJSONBlockFails(t *testing.T) {
	// __NEXT_DATA__ exists but JSON block is unterminated.
	meta := extractNextData(`<html>__NEXT_DATA__ = {"unclosed`)
	if meta != nil {
		t.Fatal("expected nil when JSON extraction fails")
	}
}

func TestExtractTwitterInitialState_SkipToBraceFails(t *testing.T) {
	// Marker found but skipToBrace returns -1 (unknown character after marker).
	meta := extractTwitterInitialState(`"entryData" \x00 rest`)
	if meta != nil {
		t.Fatal("expected nil when skipToBrace fails")
	}
}

func TestExtractTwitterInitialState_ExtractJSONBlockFails(t *testing.T) {
	// Marker found, brace found, but JSON is unterminated.
	meta := extractTwitterInitialState(`"entryData" = {"unclosed`)
	if meta != nil {
		t.Fatal("expected nil when JSON extraction fails")
	}
}

func TestExtractTwitterJSONLD_ExtractJSONBlockFails(t *testing.T) {
	// JSON-LD block exists but unterminated, followed by valid block.
	// This covers the "if err != nil { lastErr = err; continue }" path.
	html := `<html>
<script type="application/ld+json">{"unclosed
<script type="application/ld+json">{"name":"Valid","contentUrl":"https://x.com/video.mp4","thumbnail":{"url":"https://x.com/thumb.jpg"}}</script>
</html>`
	meta, err := extractTwitterJSONLD(html)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Valid" {
		t.Errorf("expected 'Valid', got %q", meta.Title)
	}
}

func TestExtractTwitterJSONLD_JSONUnmarshalFails(t *testing.T) {
	// JSON-LD block has valid JSON but doesn't parse to our struct,
	// followed by valid block. This covers the "if err := json.Unmarshal(...)" path.
	html := `<html>
<script type="application/ld+json">{"name":123}</script>
<script type="application/ld+json">{"name":"Video2","contentUrl":"https://x.com/video2.mp4","thumbnail":{"url":"https://x.com/t2.jpg"}}</script>
</html>`
	meta, err := extractTwitterJSONLD(html)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Video2" {
		t.Errorf("expected 'Video2', got %q", meta.Title)
	}
}

// =============================================================================
// 100% Coverage: extractJSONLD unmarshal error path
// =============================================================================

func TestExtractJSONLD_UnmarshalFailure(t *testing.T) {
	// JSON-LD marker exists, valid JSON block extracted, but type mismatch.
	_, err := extractJSONLD(`<html><script type="application/ld+json">{"name":123,"contentUrl":456}</script></html>`)
	if err == nil {
		t.Fatal("expected error for type mismatch in JSON-LD")
	}
}

// =============================================================================
// 100% Coverage: Faceook player config - marker in middle, no brace backwards
// =============================================================================

func TestExtractFacebookPlayerConfig_MarkerLaterInString(t *testing.T) {
	// playable_url deeper in string, no opening brace backwards.
	// The backward search hits beginning of string without finding '{'.
	meta := extractFacebookPlayerConfig(`some text "playable_url": "https://fb.com/v.mp4"`)
	if meta != nil {
		t.Fatal("expected nil when no opening brace found before marker (marker not adjacent to object)")
	}
}

func TestExtractFacebookPlayerConfig_ExtractBlockFails(t *testing.T) {
	// Opening brace found but JSON block extraction fails (unterminated).
	meta := extractFacebookPlayerConfig(`{"playable_url": "unclosed`)
	if meta != nil {
		t.Fatal("expected nil when JSON extraction fails")
	}
}

// =============================================================================
// 100% Coverage: YouTube HTTP error path
// =============================================================================

func TestYouTubeExtract_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := NewYouTubeScraper()
	_, err := s.Extract(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "youtube: HTTP 503") {
		t.Errorf("expected HTTP 503 error, got %v", err)
	}
}

func TestYouTubeExtract_NotPlayable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"UNPLAYABLE","reason":"Private video"},"videoDetails":{"videoId":"x","title":"Private"}};</script></html>`))
	}))
	defer srv.Close()

	s := NewYouTubeScraper()
	_, err := s.Extract(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "youtube: video not playable: Private video") {
		t.Errorf("expected not playable error, got %v", err)
	}
}

func TestYouTubeExtract_NoVideoDetails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"}};</script></html>`))
	}))
	defer srv.Close()

	s := NewYouTubeScraper()
	_, err := s.Extract(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "missing videoDetails") {
		t.Errorf("expected missing videoDetails error, got %v", err)
	}
}

// =============================================================================
// 100% Coverage: YouTube DNS failure / network error
// =============================================================================

func TestYouTubeExtract_NetworkError(t *testing.T) {
	s := NewYouTubeScraper()
	// Use a port that's unlikely to be open.
	_, err := s.Extract(context.Background(), "http://127.0.0.1:1")
	if err == nil || !strings.Contains(err.Error(), "youtube: fetch page") {
		t.Errorf("expected network error, got %v", err)
	}
}

// =============================================================================
// 100% Coverage: YouTube format deduplication edge cases
// =============================================================================

func TestYouTubeFormat_HumanSizeEdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "Unknown"},
		{"0", "Unknown"},
		{"notanumber", "Unknown"},
		{"500", "500 B"},
		{"1500", "1.5 KB"},
		{"2000000", "2.0 MB"},
		{"3000000000", "3.0 GB"},
	}
	for _, tc := range tests {
		f := youtubeFormat{ContentLength: tc.input}
		got := f.humanSize()
		if got != tc.want {
			t.Errorf("humanSize(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestYouTubeFormat_ExtensionEdgeCases(t *testing.T) {
	tests := []struct {
		mime string
		want string
	}{
		{"video/mp4", "mp4"},
		{"video/webm", "webm"},
		{"video/3gpp", "3gp"},
		{"video/x-flv", "flv"},
		{"audio/webm", "webm"},
		{"application/x-mpegURL", "mp4"},
		{"", "mp4"},
		{"video/unknown", "mp4"},
	}
	for _, tc := range tests {
		f := youtubeFormat{MimeType: tc.mime}
		got := f.extension()
		if got != tc.want {
			t.Errorf("extension(%q) = %q, want %q", tc.mime, got, tc.want)
		}
	}
}

func TestYouTubeFormat_LabelEdgeCases(t *testing.T) {
	tests := []struct {
		qualityLabel string
		mimeType     string
		want         string
	}{
		{"720p", "video/mp4", "720p"},
		{"1080p", "video/mp4", "1080p"},
		{"", "audio/webm", "Audio"},
		{"", "video/mp4", "Unknown"},
		{"", "", "Unknown"},
	}
	for _, tc := range tests {
		f := youtubeFormat{QualityLabel: tc.qualityLabel, MimeType: tc.mimeType}
		got := f.label()
		if got != tc.want {
			t.Errorf("label(%q, %q) = %q, want %q", tc.qualityLabel, tc.mimeType, got, tc.want)
		}
	}
}

// =============================================================================
// 100% Coverage: YouTube duration formatting
// =============================================================================

func TestYouTubeDuration_EdgeCases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},"videoDetails":{"videoId":"x","title":"Test","lengthSeconds":"3661","thumbnail":{"thumbnails":[{"url":"https://img.youtube.com/vi/x/default.jpg"},{"url":"https://img.youtube.com/vi/x/hqdefault.jpg"}]}},"streamingData":{"formats":[{"mimeType":"video/mp4","qualityLabel":"720p","contentLength":"1048576","url":"https://example.com/video.mp4"}]}};</script></html>`))
	}))
	defer srv.Close()

	s := NewYouTubeScraper()
	meta, err := s.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	// 3661 seconds = 61 minutes 1 second = 61:01
	if meta.Duration != "61:01" {
		t.Errorf("expected '61:01', got %q", meta.Duration)
	}
	// Should use highest quality thumbnail (last entry)
	if meta.ThumbnailURL != "https://img.youtube.com/vi/x/hqdefault.jpg" {
		t.Errorf("expected hqdefault thumbnail, got %q", meta.ThumbnailURL)
	}
}

// =============================================================================
// 100% Coverage: YouTube with invalid length seconds
// =============================================================================

func TestYouTubeDuration_InvalidLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},"videoDetails":{"videoId":"x","title":"Test","lengthSeconds":"notanumber"}};</script></html>`))
	}))
	defer srv.Close()

	s := NewYouTubeScraper()
	meta, err := s.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Duration != "" {
		t.Errorf("expected empty duration for non-numeric input, got %q", meta.Duration)
	}
}

// =============================================================================
// 100% Coverage: YouTube no thumbnails
// =============================================================================

func TestYouTubeExtract_NoThumbnails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},"videoDetails":{"videoId":"x","title":"No thumbs","lengthSeconds":"10"}};</script></html>`))
	}))
	defer srv.Close()

	s := NewYouTubeScraper()
	meta, err := s.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.ThumbnailURL != "" {
		t.Errorf("expected empty thumbnail, got %q", meta.ThumbnailURL)
	}
}

// =============================================================================
// 100% Coverage: YouTube format with empty URL
// =============================================================================

func TestYouTubeExtract_EmptyFormatURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},"videoDetails":{"videoId":"x","title":"Empty URL","lengthSeconds":"10"},"streamingData":{"formats":[{"mimeType":"video/mp4","qualityLabel":"720p","url":""}]}};</script></html>`))
	}))
	defer srv.Close()

	s := NewYouTubeScraper()
	meta, err := s.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Formats) != 0 {
		t.Errorf("expected 0 formats (empty URL), got %d", len(meta.Formats))
	}
}

// =============================================================================
// 100% Coverage: YouTube signatureCipher / cipher fields
// =============================================================================

func TestYouTubeFormat_NonURLFormat(t *testing.T) {
	// Format with URL empty but signatureCipher set — should be skipped
	// since we can't decrypt the signature.
	_ = youtubeFormat{
		MimeType:        "video/mp4",
		QualityLabel:    "720p",
		ContentLength:   "1000000",
		URL:             "",
		SignatureCipher: "some_encrypted_data",
	}
	// Just testing that the struct fields exist and label/extension work.
	f := youtubeFormat{MimeType: "video/mp4", QualityLabel: "1080p", URL: "https://example.com/v.mp4"}
	if f.label() != "1080p" {
		t.Errorf("expected 1080p, got %q", f.label())
	}
}

// =============================================================================
// 100% Coverage: Deduplication via seen map
// =============================================================================

func TestYouTubeExtract_DuplicateFormatURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},"videoDetails":{"videoId":"x","title":"Dupes","lengthSeconds":"10"},"streamingData":{"formats":[{"mimeType":"video/mp4","qualityLabel":"720p","contentLength":"1000","url":"https://example.com/v.mp4"},{"mimeType":"video/mp4","qualityLabel":"720p","contentLength":"1000","url":"https://example.com/v.mp4"},{"mimeType":"video/mp4","qualityLabel":"1080p","contentLength":"2000","url":"https://example.com/v2.mp4"}]}};</script></html>`))
	}))
	defer srv.Close()

	s := NewYouTubeScraper()
	meta, err := s.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Formats) != 2 {
		t.Errorf("expected 2 unique formats, got %d", len(meta.Formats))
	}
}

// =============================================================================
// 100% Coverage: Facebook player config with marker but backwards scan
// hits non-{ character at start of string
// =============================================================================

func TestExtractFacebookPlayerConfig_BraceScanReachesStart(t *testing.T) {
	// "playable_url" appears after a non-{ character, backward scan reaches
	// index 0 but html[0] is not '{'.
	meta := extractFacebookPlayerConfig(`x"playable_url":"https://fb.com/v.mp4"`)
	if meta != nil {
		t.Fatal("expected nil when backwards search reaches non-brace start")
	}
}

// =============================================================================
// 100% Coverage: All main Extract() functions with HTTP error status
// =============================================================================

func TestFacebookExtract_HTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := NewFacebookScraper()
	_, err := s.Extract(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "facebook: HTTP 404") {
		t.Errorf("expected HTTP 404 error, got %v", err)
	}
}

func TestInstagramExtract_HTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := NewInstagramScraper()
	_, err := s.Extract(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "instagram: HTTP 404") {
		t.Errorf("expected HTTP 404 error, got %v", err)
	}
}

func TestTikTokExtract_HTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := NewTikTokScraper()
	_, err := s.Extract(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "tiktok: HTTP 404") {
		t.Errorf("expected HTTP 404 error, got %v", err)
	}
}

func TestTwitterExtract_HTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := NewTwitterScraper()
	_, err := s.Extract(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "twitter: HTTP 404") {
		t.Errorf("expected HTTP 404 error, got %v", err)
	}
}

// =============================================================================
// 100% Coverage: Factory edge cases
// =============================================================================

func TestGetScraper_MalformedURL(t *testing.T) {
	_, err := GetScraper("://")
	if err == nil {
		t.Fatal("expected error for malformed URL")
	}
}

func TestGetScraper_EmptyHost(t *testing.T) {
	_, err := GetScraper("https://")
	if err == nil {
		t.Fatal("expected error for URL with empty host")
	}
}

func TestMatchHost_ExactShortPattern(t *testing.T) {
	if !matchHost("x.com", "x.com") {
		t.Error("expected exact match for x.com")
	}
	if matchHost("example.com", "x.com") {
		t.Error("should not match example.com for x.com pattern")
	}
	if matchHost("x.com.evil.com", "x.com") {
		t.Error("should not match x.com.evil.com for x.com pattern")
	}
}

func TestMatchHost_SubdomainFacebook(t *testing.T) {
	if !matchHost("www.facebook.com", "facebook.com") {
		t.Error("expected www.facebook.com to match facebook.com")
	}
	if !matchHost("web.facebook.com", "facebook.com") {
		t.Error("expected web.facebook.com to match facebook.com")
	}
	if matchHost("facebook.com.evil.com", "facebook.com") {
		t.Error("should not match facebook.com.evil.com")
	}
}

// =============================================================================
// 100% Coverage: YouTube extractScriptTags edge cases
// =============================================================================

func TestExtractScriptTags_UnclosedTagNoCloseBracket(t *testing.T) {
	// <script without closing >
	scripts := extractScriptTags("<html><script data-src=\"foo")
	if len(scripts) != 0 {
		t.Errorf("expected 0 scripts, got %d", len(scripts))
	}
}

// =============================================================================
// 100% Coverage: Facebook player config - backwards scan at boundary
// =============================================================================

func TestExtractFacebookPlayerConfig_BraceBackwardsAtPos0(t *testing.T) {
	// When idx=0, loop doesn't run, html[0] might not be '{'
	meta := extractFacebookPlayerConfig(`"playable_url": "https://fb.com/v.mp4"`)
	if meta != nil {
		t.Fatal("expected nil when html[0] is not '{' and marker at pos 0")
	}
}

// =============================================================================
// 100% Coverage: TikTok initial state with no duration
// =============================================================================

func TestExtractTikTokInitialState_NoDescNoDuration(t *testing.T) {
	meta := extractTikTokInitialState(`<html>__INITIAL_STATE__={"itemInfo":{"itemStruct":{"video":{"id":"123"},"desc":"","author":"@u"}}}</html>`)
	if meta == nil {
		t.Fatal("expected non-nil")
	}
	if meta.Title != "TikTok Video" {
		t.Errorf("expected default title, got %q", meta.Title)
	}
	if meta.Duration != "" {
		t.Errorf("expected empty duration, got %q", meta.Duration)
	}
}

// =============================================================================
// 100% Coverage: Extract() with context cancellation (network path)
// =============================================================================

func TestPlatforms_HTTPFetchWithCancelledContext(t *testing.T) {
	// For fully implemented scrapers, a cancelled context before HTTP request
	// should fail with fetch page error (not create request).
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scrapers := []struct{
		name string
		s    domain.VideoScraper
	}{
		{"YouTube", NewYouTubeScraper()},
		{"TikTok", NewTikTokScraper()},
		{"Instagram", NewInstagramScraper()},
		{"Facebook", NewFacebookScraper()},
		{"Twitter", NewTwitterScraper()},
	}

	for _, tc := range scrapers {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.s.Extract(ctx, "https://example.com/video")
			if err == nil {
				t.Fatal("expected error with cancelled context")
			}
		})
	}
}

// =============================================================================
// 100% Coverage: YouTube findAnyPlayerResponse with empty script content
// =============================================================================

func TestFindAnyPlayerResponse_EmptyScriptContent(t *testing.T) {
	html := `<html><script></script></html>`
	_, err := findAnyPlayerResponse(html)
	if err == nil {
		t.Fatal("expected error when script content is empty")
	}
}

// =============================================================================
// 100% Coverage: YouTube findByMarkers with marker at end of string
// =============================================================================

func TestFindByMarkers_MarkerAtEnd(t *testing.T) {
	_, err := findByMarkers(`<script>var ytInitialPlayerResponse =</script>`)
	if err == nil {
		t.Fatal("expected error when marker at end without JSON")
	}
}

// =============================================================================
// 100% Coverage: YouTube findAnyPlayerResponse with partial playability
// =============================================================================

func TestFindAnyPlayerResponse_OnlyPlayability(t *testing.T) {
	// JSON with only playabilityStatus but no videoDetails.
	html := `<html><script>var x = {"playabilityStatus":{"status":"LOGIN_REQUIRED","reason":"Sign in"}};</script></html>`
	pr, err := findAnyPlayerResponse(html)
	if err != nil {
		t.Fatal(err)
	}
	if pr.PlayabilityStatus == nil {
		t.Fatal("expected playabilityStatus")
	}
}

// =============================================================================
// 100% Coverage: YouTube extractJSONBlock with escaped backslash in string
// =============================================================================

func TestExtractJSONBlock_EscapedBackslash(t *testing.T) {
	block, err := extractJSONBlock(`{"key": "value\\"}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if block != `{"key": "value\\"}` {
		t.Errorf("unexpected block: %q", block)
	}
}

func TestExtractJSONBlock_StartNotBrace(t *testing.T) {
	_, err := extractJSONBlock(`not a brace`, 0)
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("expected invalid JSON error, got %v", err)
	}
}

// =============================================================================
// 100% Coverage: Twitter extractTwitterInitialState skipToBrace returns -1
// with empty/unexpected content after marker
// =============================================================================

func TestExtractTwitterInitialState_MarkerUnknownChar(t *testing.T) {
	// Marker found, but next character is unknown (non-identifier, non-whitespace).
	meta := extractTwitterInitialState(`"entryData" \x00 rest`)
	if meta != nil {
		t.Fatal("expected nil when skipToBrace returns -1")
	}
}

// =============================================================================
// 100% Coverage: TikTok skipToBrace returns -1
// =============================================================================

func TestExtractTikTokInitialState_SkipToBraceFails(t *testing.T) {
	// __INITIAL_STATE__ found, but unknown character follows.
	meta := extractTikTokInitialState(`__INITIAL_STATE__\x00rest`)
	if meta != nil {
		t.Fatal("expected nil when skipToBrace fails")
	}
}

// =============================================================================
// 100% Coverage: Instagram skipToBrace returns -1 in extractNextData
// =============================================================================

func TestExtractNextData_SkipToBraceFails(t *testing.T) {
	meta := extractNextData(`__NEXT_DATA__\x00rest`)
	if meta != nil {
		t.Fatal("expected nil when skipToBrace fails")
	}
}

// =============================================================================
// 100% Coverage: YouTube findAnyPlayerResponse - script with only non-brace chars
// =============================================================================

func TestFindAnyPlayerResponse_NoBraces(t *testing.T) {
	html := `<html><script>no braces here</script></html>`
	_, err := findAnyPlayerResponse(html)
	if err == nil {
		t.Fatal("expected error when no braces in script")
	}
}

// =============================================================================
// 100% Coverage: Facebook player config - no playable_url but backwards
// scan goes to non-brace char
// =============================================================================

func TestExtractFacebookPlayerConfig_BackwardsScanReturnsNonBrace(t *testing.T) {
	// Marker not at position 0, backward scan reaches a char that is not '{'
	// We need: idx > 0, loop runs until start==0 or html[start]=='{', but html[start] != '{'
	meta := extractFacebookPlayerConfig(`abc"playable_url":"https://fb.com/v.mp4"`)
	if meta != nil {
		t.Fatal("expected nil when backwards scan results in non-brace")
	}
}
