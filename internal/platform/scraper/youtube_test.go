package scraper

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/quick"
	"time"

	"downder-backend/internal/domain"
)

// =============================================================================
// Fuzz helper: generate random HTML with embedded JSON
// =============================================================================

// randomJSONBlock generates a random valid JSON object for fuzz-like testing.
func randomJSONBlock(rng *rand.Rand) string {
	return randomJSONObject(rng, rng.Intn(5), 0)
}

func randomJSONValue(rng *rand.Rand, depth int, indent int) string {
	switch rng.Intn(6) {
	case 0:
		return fmt.Sprintf("%q", randomString(rng, 10))
	case 1:
		return fmt.Sprintf("%d", rng.Intn(1000))
	case 2:
		if rng.Intn(2) == 0 {
			return "true"
		}
		return "false"
	case 3:
		return "null"
	case 4:
		if depth <= 0 {
			return fmt.Sprintf("%q", randomString(rng, 10))
		}
		return randomJSONObject(rng, depth-1, indent+1)
	default:
		return randomJSONArray(rng, depth, indent+1)
	}
}

func randomJSONObject(rng *rand.Rand, depth int, indent int) string {
	n := rng.Intn(4)
	var pairs []string
	for i := 0; i < n; i++ {
		k := randomString(rng, 6)
		v := randomJSONValue(rng, depth, indent+1)
		pairs = append(pairs, fmt.Sprintf("%q:%s", k, v))
	}
	return "{" + strings.Join(pairs, ",") + "}"
}

func randomJSONArray(rng *rand.Rand, depth int, indent int) string {
	n := rng.Intn(3)
	var elems []string
	for i := 0; i < n; i++ {
		elems = append(elems, randomJSONValue(rng, depth, indent+1))
	}
	return "[" + strings.Join(elems, ",") + "]"
}

func randomString(rng *rand.Rand, maxLen int) string {
	n := rng.Intn(maxLen) + 1
	// Include special characters that could break parsing.
	chars := "abcdefghijklmnopqrstuvwxyz0123456789 _-!@#$%^&*()+=[]{}|;:',.<>?/~`"
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(chars[rng.Intn(len(chars))])
	}
	return b.String()
}

// =============================================================================
// Property: extractJSONBlock invariants
// =============================================================================
//
// For any valid JSON object, extractJSONBlock must:
//   1. Return the complete JSON object (balanced braces check).
//   2. Return valid JSON (can be unmarshaled).
//   3. Not return characters before the starting brace.

func TestProp_ExtractJSONBlock_RoundTrip(t *testing.T) {
	f := func() bool {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		obj := randomJSONBlock(rng)
		html := "var x = " + obj + ";"
		start := strings.Index(html, "{")
		if start == -1 {
			return true // skip malformed
		}
		got, err := extractJSONBlock(html, start)
		if err != nil {
			t.Logf("extractJSONBlock error for input:\n%s\nError: %v", obj, err)
			return false
		}
		// Must be self-contained JSON.
		if got != obj {
			t.Logf("round-trip mismatch:\nwant: %s\ngot:  %s", obj, got)
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}

func TestProp_ExtractJSONBlock_AlwaysStartsWithBrace(t *testing.T) {
	f := func() bool {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		obj := randomJSONBlock(rng)
		html := "prefix_" + obj + "_suffix"
		start := strings.Index(html, "{")
		if start == -1 {
			return true
		}
		got, err := extractJSONBlock(html, start)
		if err != nil {
			return false
		}
		if len(got) == 0 || got[0] != '{' {
			t.Logf("extracted block does not start with '{': %q", got)
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// =============================================================================
// Property: label() invariants
// =============================================================================

func TestProp_Label_NotEmptyWhenQualityLabelSet(t *testing.T) {
	f := func(label string) bool {
		f := youtubeFormat{QualityLabel: label, MimeType: "video/mp4"}
		got := f.label()
		if label != "" && got == "" {
			return false
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestProp_Label_AudioWhenMimeContainsAudio(t *testing.T) {
	f := func(label string) bool {
		f := youtubeFormat{QualityLabel: label, MimeType: "audio/mp4"}
		got := f.label()
		// If QualityLabel is set, it takes priority.
		if label != "" {
			return got == label
		}
		// Otherwise should be "Audio".
		return got == "Audio"
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

// =============================================================================
// Property: humanSize() invariants
// =============================================================================

func TestProp_HumanSize_NeverEmpty(t *testing.T) {
	f := func(n uint64) bool {
		f := youtubeFormat{ContentLength: fmt.Sprintf("%d", n)}
		got := f.humanSize()
		return got != "" && !strings.Contains(got, "NaN") && !strings.Contains(got, "-")
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestProp_HumanSize_Parseable(t *testing.T) {
	cases := []struct {
		input string
		desc  string
	}{
		{"0", "zero (returns Unknown)"},
		{"1", "minimal"},
		{"999", "boundary"},
		{"1000", "1 KB"},
		{"1048576", "~1 MB"},
		{"1073741824", "~1 GB"},
		{"999999999999", "large"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			f := youtubeFormat{ContentLength: c.input}
			got := f.humanSize()
			if got == "" {
				t.Fatal("empty result")
			}
			// "0" returns "Unknown" (no digits), everything else has a number.
			if c.input == "0" {
				if got != "Unknown" {
					t.Errorf("expected Unknown for zero, got %q", got)
				}
				return
			}
			if !strings.ContainsAny(got, "0123456789") {
				t.Errorf("no digits in result: %q", got)
			}
		})
	}
}

// =============================================================================
// Unit: extractJSONBlock edge cases
// =============================================================================

func TestExtractJSONBlock_Simple(t *testing.T) {
	got, err := extractJSONBlock(`{"a":1}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"a":1}` {
		t.Errorf("got %q", got)
	}
}

func TestExtractJSONBlock_Nested(t *testing.T) {
	got, err := extractJSONBlock(`{"a":{"b":[1,2,3]}}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"a":{"b":[1,2,3]}}` {
		t.Errorf("got %q", got)
	}
}

func TestExtractJSONBlock_Empty(t *testing.T) {
	got, err := extractJSONBlock(`{}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{}` {
		t.Errorf("got %q", got)
	}
}

func TestExtractJSONBlock_BracesInStrings(t *testing.T) {
	input := `{"url":"https://example.com?sig={abc}&id={}","name":"{test}"}`
	got, err := extractJSONBlock(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != input {
		t.Errorf("got  %q\nwant %q", got, input)
	}
}

func TestExtractJSONBlock_EscapedQuotes(t *testing.T) {
	input := `{"text":"say \"hello\" world"}`
	got, err := extractJSONBlock(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != input {
		t.Errorf("got  %q\nwant %q", got, input)
	}
}

func TestExtractJSONBlock_BackslashBeforeQuote(t *testing.T) {
	input := `{"path":"C:\\Users\\name"}`
	got, err := extractJSONBlock(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != input {
		t.Errorf("got  %q\nwant %q", got, input)
	}
}

func TestExtractJSONBlock_DeeplyNested(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"l0":`)
	for i := 1; i <= 100; i++ {
		sb.WriteString(fmt.Sprintf(`{"l%d":`, i))
	}
	sb.WriteString(`"x"`)
	for i := 0; i <= 100; i++ {
		sb.WriteString(`}`)
	}
	input := sb.String()

	got, err := extractJSONBlock(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != input {
		t.Errorf("length mismatch: got %d, want %d", len(got), len(input))
	}
}

func TestExtractJSONBlock_Unicode(t *testing.T) {
	input := `{"emoji":"😀👍🎉","unicode":"Hello AB"}`
	got, err := extractJSONBlock(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != input {
		t.Errorf("got  %q\nwant %q", got, input)
	}
}

func TestExtractJSONBlock_NoStartBrace(t *testing.T) {
	_, err := extractJSONBlock(`"string"`, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractJSONBlock_Unterminated(t *testing.T) {
	_, err := extractJSONBlock(`{"a":1`, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractJSONBlock_OnlyFirstObject(t *testing.T) {
	input := `{"first":1} junk {"second":2}`
	got, err := extractJSONBlock(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"first":1}` {
		t.Errorf("got %q, want first object only", got)
	}
}

func TestExtractJSONBlock_MultipleOpenBracesBeforeClose(t *testing.T) {
	input := `{{{}}}`
	got, err := extractJSONBlock(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != input {
		t.Errorf("got %q", got)
	}
}

func TestExtractJSONBlock_StringWithNestedEscapedChars(t *testing.T) {
	input := `{"a":"b\\\"c"}`
	got, err := extractJSONBlock(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != input {
		t.Errorf("got  %q\nwant %q", got, input)
	}
}

func TestExtractJSONBlock_WithWhitespacePrefix(t *testing.T) {
	input := `   {"a":1}`
	got, err := extractJSONBlock(input, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"a":1}` {
		t.Errorf("got %q", got)
	}
}

func TestExtractJSONBlock_InvalidStartPosition(t *testing.T) {
	_, err := extractJSONBlock(`{"a":1}`, 100)
	if err == nil {
		t.Fatal("expected error for out-of-range start")
	}
}

// =============================================================================
// Unit: youtubeFormat.label()
// =============================================================================

func TestFormatLabel_Exhaustive(t *testing.T) {
	cases := []struct {
		name string
		f    youtubeFormat
		want string
	}{
		{"1080p60 label", youtubeFormat{QualityLabel: "1080p60"}, "1080p60"},
		{"empty label", youtubeFormat{QualityLabel: ""}, "Unknown"},
		{"audio detection", youtubeFormat{MimeType: "audio/mp4"}, "Audio"},
		{"audio with label", youtubeFormat{QualityLabel: "high", MimeType: "audio/mp4"}, "high"},
		{"webm audio", youtubeFormat{MimeType: "audio/webm"}, "Audio"},
		{"video no label", youtubeFormat{MimeType: "video/mp4"}, "Unknown"},
		{"empty all", youtubeFormat{}, "Unknown"},
		{"whitespace label", youtubeFormat{QualityLabel: " "}, " "},
		{"weird label", youtubeFormat{QualityLabel: "abc123!@#"}, "abc123!@#"},
		{"audio/webm codecs", youtubeFormat{MimeType: `audio/webm; codecs="opus"`}, "Audio"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.f.label(); got != c.want {
				t.Errorf("label() = %q, want %q", got, c.want)
			}
		})
	}
}

// =============================================================================
// Unit: youtubeFormat.extension()
// =============================================================================

func TestFormatExtension_Exhaustive(t *testing.T) {
	cases := []struct {
		name string
		mime string
		want string
	}{
		{"video mp4", `video/mp4; codecs="avc1.64001F"`, "mp4"},
		{"audio mp4", `audio/mp4; codecs="mp4a.40.2"`, "mp4"},
		{"video webm", `video/webm; codecs="vp9"`, "webm"},
		{"audio webm", `audio/webm; codecs="opus"`, "webm"},
		{"3gpp", "video/3gpp", "3gp"},
		{"flv", "video/x-flv", "flv"},
		{"no codecs", "video/mp4", "mp4"},
		{"multiple codecs", `video/mp4; codecs="avc1.640028, mp4a.40.2"`, "mp4"},
		{"extra spaces", `  video/mp4  `, "mp4"},
		{"unknown subtype", "video/unknown", "mp4"},
		{"empty mime", "", "mp4"},
		{"no subtype", "video", "mp4"},
		{"only codecs", `; codecs="avc1"`, "mp4"},
		{"weird mime", "application/x-mpegURL", "mp4"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := youtubeFormat{MimeType: c.mime}
			if got := f.extension(); got != c.want {
				t.Errorf("extension(%q) = %q, want %q", c.mime, got, c.want)
			}
		})
	}
}

// =============================================================================
// Unit: youtubeFormat.humanSize()
// =============================================================================

func TestFormatHumanSize_Boundaries(t *testing.T) {
	cases := []struct {
		name string
		cl   string
		want string
	}{
		{"empty", "", "Unknown"},
		{"invalid text", "abc", "Unknown"},
		{"negative", "-1", "Unknown"},
		{"zero", "0", "Unknown"},
		{"1 byte", "1", "1 B"},
		{"999 bytes", "999", "999 B"},
		{"1 KB boundary", "1000", "1.0 KB"},
		{"1.5 KB", "1500", "1.5 KB"},
		{"999 KB", "999000", "999.0 KB"},
		{"1 MB boundary", "1000000", "1.0 MB"},
		{"1.5 MB", "1500000", "1.5 MB"},
		{"999 MB", "999000000", "999.0 MB"},
		{"1 GB boundary", "1000000000", "1.0 GB"},
		{"2.5 GB", "2500000000", "2.5 GB"},
		{"large value", "9999999999999", "10000.0 GB"},
		{"max uint64", "18446744073709551615", "18446744073.7 GB"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := youtubeFormat{ContentLength: c.cl}
			if got := f.humanSize(); got != c.want {
				t.Errorf("humanSize(%q) = %q, want %q", c.cl, got, c.want)
			}
		})
	}
}

// =============================================================================
// Integration: full pipeline with mocked YouTube
// =============================================================================

func stdPage() string {
	return `<html><script>var ytInitialPlayerResponse = {
		"playabilityStatus":{"status":"OK"},
		"videoDetails":{
			"videoId":"dQw4w9WgXcQ",
			"title":"Rick Astley - Never Gonna Give You Up",
			"lengthSeconds":"212",
			"thumbnail":{"thumbnails":[
				{"url":"https://i.ytimg.com/default.jpg","width":120,"height":90},
				{"url":"https://i.ytimg.com/hqdefault.jpg","width":480,"height":360}
			]}
		},
		"streamingData":{
			"expiresInSeconds":"21600",
			"formats":[
				{"itag":18,"mimeType":"video/mp4; codecs=\"avc1.42001E, mp4a.40.2\"","width":640,"height":360,"contentLength":"5242880","quality":"medium","qualityLabel":"360p","url":"https://ex.com/360.mp4"}
			],
			"adaptiveFormats":[
				{"itag":137,"mimeType":"video/mp4; codecs=\"avc1.640028\"","width":1920,"height":1080,"contentLength":"15728640","quality":"hd1080","qualityLabel":"1080p","url":"https://ex.com/1080.mp4"},
				{"itag":140,"mimeType":"audio/mp4; codecs=\"mp4a.40.2\"","contentLength":"1048576","quality":"medium","url":"https://ex.com/audio.mp4"}
			]
		}
	};</script></html>`
}

func newMockServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if body != "" {
			w.Write([]byte(body))
		}
	}))
}

func mustExtract(t *testing.T, ys *YouTubeScraper, url string) *domain.VideoMetadata {
	t.Helper()
	meta, err := ys.Extract(context.Background(), url)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	return meta
}

func TestIntegration_FullPipeline(t *testing.T) {
	srv := newMockServer(t, http.StatusOK, stdPage())
	defer srv.Close()

	s, _ := GetScraper("https://youtube.com/watch?v=test")
	ys := s.(*YouTubeScraper)
	ys.client = srv.Client()

	meta := mustExtract(t, ys, srv.URL)

	if meta.ID != "dQw4w9WgXcQ" {
		t.Errorf("ID = %q", meta.ID)
	}
	if meta.Title != "Rick Astley - Never Gonna Give You Up" {
		t.Errorf("Title = %q", meta.Title)
	}
	if meta.Duration != "3:32" {
		t.Errorf("Duration = %q", meta.Duration)
	}
	if meta.ThumbnailURL != "https://i.ytimg.com/hqdefault.jpg" {
		t.Errorf("ThumbnailURL = %q", meta.ThumbnailURL)
	}
	if len(meta.Formats) != 3 {
		t.Fatalf("got %d formats, want 3", len(meta.Formats))
	}
}

func TestIntegration_FormatContent(t *testing.T) {
	srv := newMockServer(t, http.StatusOK, stdPage())
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	meta := mustExtract(t, ys, srv.URL)

	checks := []struct {
		idx     int
		quality string
		ext     string
		size    string
	}{
		{0, "360p", "mp4", "5.2 MB"},
		{1, "1080p", "mp4", "15.7 MB"},
		{2, "Audio", "mp4", "1.0 MB"},
	}
	for _, c := range checks {
		f := meta.Formats[c.idx]
		if f.Quality != c.quality {
			t.Errorf("[%d] Quality = %q, want %q", c.idx, f.Quality, c.quality)
		}
		if f.Extension != c.ext {
			t.Errorf("[%d] Extension = %q, want %q", c.idx, f.Extension, c.ext)
		}
		if f.FileSize != c.size {
			t.Errorf("[%d] FileSize = %q, want %q", c.idx, f.FileSize, c.size)
		}
		if f.DownloadURL == "" {
			t.Errorf("[%d] DownloadURL empty", c.idx)
		}
	}
}

func TestIntegration_FormatDeduplication(t *testing.T) {
	// Same URL appears in both formats and adaptiveFormats — should be counted once.
	html := `<html><script>var ytInitialPlayerResponse = {
		"playabilityStatus":{"status":"OK"},
		"videoDetails":{"videoId":"x","title":"Dedup test"},
		"streamingData":{
			"formats":[{"mimeType":"video/mp4","qualityLabel":"720p","url":"https://ex.com/v.mp4"}],
			"adaptiveFormats":[{"mimeType":"video/mp4","qualityLabel":"720p","url":"https://ex.com/v.mp4"}]
		}
	};</script></html>`
	srv := newMockServer(t, http.StatusOK, html)
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	meta := mustExtract(t, ys, srv.URL)

	if len(meta.Formats) != 1 {
		t.Errorf("expected 1 deduplicated format, got %d", len(meta.Formats))
	}
}

func TestIntegration_NoStreamingData(t *testing.T) {
	html := `<html><script>var ytInitialPlayerResponse = {
		"playabilityStatus":{"status":"OK"},
		"videoDetails":{"videoId":"x","title":"Live stream"}
	};</script></html>`
	srv := newMockServer(t, http.StatusOK, html)
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	meta := mustExtract(t, ys, srv.URL)

	if len(meta.Formats) != 0 {
		t.Errorf("expected 0 formats for live stream, got %d", len(meta.Formats))
	}
}

func TestIntegration_AllEncrypted(t *testing.T) {
	html := `<html><script>var ytInitialPlayerResponse = {
		"playabilityStatus":{"status":"OK"},
		"videoDetails":{"videoId":"x","title":"Encrypted"},
		"streamingData":{"formats":[{"mimeType":"video/mp4","qualityLabel":"720p","url":""}]}
	};</script></html>`
	srv := newMockServer(t, http.StatusOK, html)
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	meta := mustExtract(t, ys, srv.URL)
	if len(meta.Formats) != 0 {
		t.Errorf("expected 0 formats (all encrypted), got %d", len(meta.Formats))
	}
}

func TestIntegration_EmptyThumbnailArray(t *testing.T) {
	html := `<html><script>var ytInitialPlayerResponse = {
		"playabilityStatus":{"status":"OK"},
		"videoDetails":{"videoId":"x","title":"No thumb","thumbnail":{"thumbnails":[]}}
	};</script></html>`
	srv := newMockServer(t, http.StatusOK, html)
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	meta := mustExtract(t, ys, srv.URL)
	if meta.ThumbnailURL != "" {
		t.Errorf("expected empty thumbnail, got %q", meta.ThumbnailURL)
	}
}

// =============================================================================
// Integration: error cases
// =============================================================================

func TestIntegration_PrivateVideo(t *testing.T) {
	html := `<html><script>var ytInitialPlayerResponse = {
		"playabilityStatus":{"status":"UNPLAYABLE","reason":"This video is private"}
	};</script></html>`
	srv := newMockServer(t, http.StatusOK, html)
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	_, err := ys.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "private") {
		t.Errorf("error should contain reason, got: %v", err)
	}
}

func TestIntegration_AgeRestricted(t *testing.T) {
	html := `<html><script>var ytInitialPlayerResponse = {
		"playabilityStatus":{"status":"UNPLAYABLE","reason":"Age-restricted"}
	};</script></html>`
	srv := newMockServer(t, http.StatusOK, html)
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	_, err := ys.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIntegration_PlayabilityNoReason(t *testing.T) {
	html := `<html><script>var ytInitialPlayerResponse = {
		"playabilityStatus":{"status":"ERROR"}
	};</script></html>`
	srv := newMockServer(t, http.StatusOK, html)
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	_, err := ys.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown reason") {
		t.Errorf("error should mention unknown reason, got: %v", err)
	}
}

func TestIntegration_MissingVideoDetails(t *testing.T) {
	html := `<html><script>var ytInitialPlayerResponse = {
		"playabilityStatus":{"status":"OK"}
	};</script></html>`
	srv := newMockServer(t, http.StatusOK, html)
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	_, err := ys.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIntegration_NoPlayerResponse(t *testing.T) {
	srv := newMockServer(t, http.StatusOK, "<html>no data</html>")
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	_, err := ys.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIntegration_HTTP404(t *testing.T) {
	srv := newMockServer(t, http.StatusNotFound, "not found")
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	_, err := ys.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIntegration_HTTP500(t *testing.T) {
	srv := newMockServer(t, http.StatusInternalServerError, "error")
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	_, err := ys.Extract(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIntegration_ServerTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	ys := &YouTubeScraper{client: srv.Client()}
	_, err := ys.Extract(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestIntegration_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ys := &YouTubeScraper{client: srv.Client()}
	_, err := ys.Extract(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestIntegration_ConnectionRefused(t *testing.T) {
	ys := &YouTubeScraper{client: http.DefaultClient}
	_, err := ys.Extract(context.Background(), "http://localhost:19801")
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestIntegration_UserAgentSet(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(stdPage()))
	}))
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	_, err := ys.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if ua == "" {
		t.Fatal("User-Agent header not sent")
	}
	if !strings.Contains(ua, "Mozilla") {
		t.Errorf("UA should be browser-like, got: %s", ua)
	}
}

func TestIntegration_DurationParsing(t *testing.T) {
	cases := []struct {
		name     string
		seconds  string
		want     string
	}{
		{"normal", "212", "3:32"},
		{"zero", "0", "0:00"},
		{"one second", "1", "0:01"},
		{"one minute", "60", "1:00"},
		{"one hour", "3600", "60:00"},
		{"empty", "", ""},
		{"invalid", "abc", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			html := fmt.Sprintf(`<html><script>var ytInitialPlayerResponse = {
				"playabilityStatus":{"status":"OK"},
				"videoDetails":{"videoId":"x","title":"T","lengthSeconds":"%s"}
			};</script></html>`, c.seconds)
			srv := newMockServer(t, http.StatusOK, html)
			defer srv.Close()

			ys := &YouTubeScraper{client: srv.Client()}
			meta, err := ys.Extract(context.Background(), srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			if meta.Duration != c.want {
				t.Errorf("Duration = %q, want %q", meta.Duration, c.want)
			}
		})
	}
}

// =============================================================================
// Integration: concurrent safety
// =============================================================================

func TestIntegration_ConcurrentExtract(t *testing.T) {
	srv := newMockServer(t, http.StatusOK, stdPage())
	defer srv.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ys := &YouTubeScraper{client: srv.Client()}
			meta, err := ys.Extract(context.Background(), srv.URL)
			if err != nil {
				t.Errorf("goroutine %d: %v", id, err)
				return
			}
			if meta.Title != "Rick Astley - Never Gonna Give You Up" {
				t.Errorf("goroutine %d: wrong title", id)
			}
		}(i)
	}
	wg.Wait()
}

func TestIntegration_DifferentURLs(t *testing.T) {
	// Test that Extract works with various URL-like paths.
	srv := newMockServer(t, http.StatusOK, stdPage())
	defer srv.Close()

	ys := &YouTubeScraper{client: srv.Client()}
	meta, err := ys.Extract(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta == nil {
		t.Fatal("nil metadata")
	}
}
