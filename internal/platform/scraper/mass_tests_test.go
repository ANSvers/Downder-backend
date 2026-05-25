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
// Mass tests to reach 500+ total
// =============================================================================

// TestExtractJSONBlock_RandomStrings tests extractJSONBlock with many
// different valid JSON structures to ensure correctness.
func TestExtractJSONBlock_RandomStrings(t *testing.T) {
	cases := []struct {
		name  string
		input string
		start int
		want  string
	}{
		{"simple", `{"a":1}`, 0, `{"a":1}`},
		{"nested", `{"a":{"b":2}}`, 0, `{"a":{"b":2}}`},
		{"array", `{"a":[1,2,3]}`, 0, `{"a":[1,2,3]}`},
		{"string with braces", `{"a":"{hello}"}`, 0, `{"a":"{hello}"}`},
		{"escaped quote", `{"a":"\"quote\"", "b":2}`, 0, `{"a":"\"quote\"", "b":2}`},
		{"unicode", `{"a":"Hi"}`, 0, `{"a":"Hi"}`},
		{"null", `{"a":null}`, 0, `{"a":null}`},
		{"bool", `{"a":true,"b":false}`, 0, `{"a":true,"b":false}`},
		{"number", `{"a":-42.5}`, 0, `{"a":-42.5}`},
		{"deep nest", `{"a":{"b":{"c":{"d":1}}}}`, 0, `{"a":{"b":{"c":{"d":1}}}}`},
		{"multiple keys", `{"a":1,"b":2,"c":3,"d":4,"e":5}`, 0, `{"a":1,"b":2,"c":3,"d":4,"e":5}`},
		{"with prefix", `prefix = {"x":1}`, 9, `{"x":1}`},
		{"whitespace", `  {  "a"  :  1  }  `, 2, `{  "a"  :  1  }`},
		{"newlines", "{\n\"a\":\n1\n}", 0, "{\n\"a\":\n1\n}"},
		{"tabs", "{\t\"a\":\t1\t}", 0, "{\t\"a\":\t1\t}"},
		{"empty object", `{}`, 0, `{}`},
		{"nested arrays", `{"a":[[[]]]}`, 0, `{"a":[[[]]]}`},
		{"escape sequences", `{"a":"\\\n\r\t\b\f"}`, 0, `{"a":"\\\n\r\t\b\f"}`},
		{"multiple strings", `{"k1":"v1","k2":"v2","k3":"v3"}`, 0, `{"k1":"v1","k2":"v2","k3":"v3"}`},
		{"with trailing text after", `{"a":1} extra`, 0, `{"a":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractJSONBlock(tc.input, tc.start)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestExtractJSONBlock_InvalidInputs tests that extractJSONBlock returns
// errors for invalid inputs.
func TestExtractJSONBlock_InvalidInputs(t *testing.T) {
	cases := []struct {
		name  string
		input string
		start int
	}{
		{"unclosed", `{"a":1`, 0},
		{"unclosed nested", `{"a":{"b":2}`, 0},
		{"no brace", `abc`, 0},
		{"wrong start", `abc`, 3},
		{"empty", ``, 0},
		{"just brace", `{`, 0},
		{"unclosed string", `{"a":"unclosed`, 0},
		{"escaped unclosed", `{"a":"\\`, 0},
		{"depth never zero", `{{{{`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := extractJSONBlock(tc.input, tc.start)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

// TestSkipToBrace_EdgeCases tests skipToBrace with many edge cases.
func TestSkipToBrace_EdgeCases(t *testing.T) {
	cases := []struct {
		name  string
		input string
		start int
		want  int
	}{
		{"empty", "", 0, -1},
		{"no brace", "abc", 0, -1},
		{"only brace", "{", 0, 0},
		{"whitespace and brace", "  {", 0, 2},
		{"newline and brace", "\n\n{", 0, 2},
		{"ident and brace", "abc{", 0, 3},
		{"dots and brace", "abc.def{", 0, 7},
		{"underscore and brace", "abc_def{", 0, 7},
		{"mixed and brace", "abc123.def_456{", 0, 14},
		{"operators and brace", "=={", 0, 2},
		{"brackets and brace", "[{", 0, 1},
		{"parens and brace", "({", 0, 1},
		{"colon and brace", ":{", 0, 1},
		{"quote and brace", "\"{", 0, 1},
		{"single quote and brace", "'{", 0, 1},
		{"unknown char", "abc\x00{", 0, -1},
		{"unknown char 2", "abc\x01{", 0, -1},
		{"del", "abc\x7f{", 0, -1},
		{"start out of bounds", "abc", 10, -1},
		{"start at end", "abc", 3, -1},
		{"just after brace", "abc{def", 0, 3},
		{"multiple whitespace", "     {", 0, 5},
		{"tab then brace", "\t{", 0, 1},
		{"cr then brace", "\r{", 0, 1},
		{"numbers only", "123{", 0, 3},
		{"mixed case", "AbCdEf{", 0, 6},
		{"long identifier", "abcdefghijklmnopqrstuvwxyz{", 0, 26},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := skipToBrace(tc.input, tc.start)
			if got != tc.want {
				t.Errorf("skipToBrace(%q, %d) = %d, want %d", tc.input, tc.start, got, tc.want)
			}
		})
	}
}

// TestExtractScriptTags_VariousHTML tests extractScriptTags with many HTML variants.
func TestExtractScriptTags_VariousHTML(t *testing.T) {
	cases := []struct {
		name    string
		html    string
		want    int // number of expected scripts
	}{
		{"no script", "<html></html>", 0},
		{"one script", "<html><script>content</script></html>", 1},
		{"two scripts", "<html><script>a</script><script>b</script></html>", 2},
		{"three scripts", "<html><script>a</script><script>b</script><script>c</script></html>", 3},
		{"with attributes", `<script type="text/javascript">content</script>`, 1},
		{"with async", `<script async src="file.js"></script>`, 1},
		{"with defer", `<script defer src="file.js"></script>`, 1},
		{"with newlines", "<html>\n<script>\ncontent\n</script>\n</html>", 1},
		{"empty script", "<html><script></script></html>", 1},
		{"script in script", `<script>var x = "<script>";</script>`, 1},
		{"multiple attributes", `<script type="text/javascript" async defer>content</script>`, 1},
		{"unclosed tag", `<html><script`, 0},
		{"unclosed after opening", `<html><script data-x="value`, 0},
		{"opening no close", `<html><script>content`, 0},
		{"close before open", `</script>`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractScriptTags(tc.html)
			if len(got) != tc.want {
				t.Errorf("extractScriptTags(%q) = %d scripts, want %d", tc.name, len(got), tc.want)
			}
		})
	}
}

// TestMatchHost_VariousPatterns tests matchHost with many domain variations.
func TestMatchHost_VariousPatterns(t *testing.T) {
	cases := []struct {
		host    string
		pattern string
		match   bool
	}{
		// Exact matches
		{"youtube.com", "youtube.com", true},
		{"tiktok.com", "tiktok.com", true},
		{"instagram.com", "instagram.com", true},
		{"facebook.com", "facebook.com", true},
		{"twitter.com", "twitter.com", true},
		{"x.com", "x.com", true},
		{"fb.com", "fb.com", true},
		{"youtu.be", "youtu.be", true},
		// Subdomain matches
		{"www.youtube.com", "youtube.com", true},
		{"m.youtube.com", "youtube.com", true},
		{"music.youtube.com", "youtube.com", true},
		{"www.tiktok.com", "tiktok.com", true},
		{"vm.tiktok.com", "tiktok.com", true},
		{"m.tiktok.com", "tiktok.com", true},
		{"www.instagram.com", "instagram.com", true},
		{"www.facebook.com", "facebook.com", true},
		{"web.facebook.com", "facebook.com", true},
		{"m.facebook.com", "facebook.com", true},
		{"www.twitter.com", "twitter.com", true},
		{"mobile.twitter.com", "twitter.com", true},
		{"api.twitter.com", "twitter.com", true},
		{"www.x.com", "x.com", true},
		// Non-matches
		{"example.com", "youtube.com", false},
		{"evil-youtube.com", "youtube.com", false},
		{"youtube.com.evil.com", "youtube.com", false},
		{"youtube.co", "youtube.com", false},
		{"notyoutube.com", "youtube.com", false},
		{"tiktok.com.evil.com", "tiktok.com", false},
		{"evil-tiktok.com", "tiktok.com", false},
		{"instagram.com.evil.com", "instagram.com", false},
		{"facebook.com.evil.com", "facebook.com", false},
		{"twitter.com.evil.com", "twitter.com", false},
		{"x.com.evil.com", "x.com", false},
		{"examplex.com", "x.com", false},
		{"examplefb.com", "fb.com", false},
		// Short pattern exact only
		{"abcx.com", "x.com", false},
		{"x.community", "x.com", false},
		{"fbf.com", "fb.com", false},
		{"youtubex.com", "youtu.be", false},
	}
	for _, tc := range cases {
		t.Run(tc.host+"/"+tc.pattern, func(t *testing.T) {
			got := matchHost(tc.host, tc.pattern)
			if got != tc.match {
				t.Errorf("matchHost(%q, %q) = %v, want %v", tc.host, tc.pattern, got, tc.match)
			}
		})
	}
}

// TestGetScraper_VariousURLs tests GetScraper with many URL variants.
func TestGetScraper_VariousURLs(t *testing.T) {
	cases := []struct {
		url      string
		wantType string // type name substring
		wantErr  bool
	}{
		// YouTube
		{"https://youtube.com/watch?v=abc", "*scraper.YouTubeScraper", false},
		{"https://www.youtube.com/watch?v=abc", "*scraper.YouTubeScraper", false},
		{"https://youtu.be/abc", "*scraper.YouTubeScraper", false},
		{"https://m.youtube.com/watch?v=abc", "*scraper.YouTubeScraper", false},
		{"https://music.youtube.com/watch?v=abc", "*scraper.YouTubeScraper", false},
		// TikTok
		{"https://tiktok.com/@user/video/123", "*scraper.TikTokScraper", false},
		{"https://www.tiktok.com/@user/video/123", "*scraper.TikTokScraper", false},
		{"https://vm.tiktok.com/abc123", "*scraper.TikTokScraper", false},
		{"https://m.tiktok.com/v/123", "*scraper.TikTokScraper", false},
		// Instagram
		{"https://instagram.com/p/xyz", "*scraper.InstagramScraper", false},
		{"https://www.instagram.com/reel/abc", "*scraper.InstagramScraper", false},
		{"https://www.instagram.com/tv/def", "*scraper.InstagramScraper", false},
		{"https://instagram.com/stories/user/123", "*scraper.InstagramScraper", false},
		// Facebook
		{"https://facebook.com/watch?v=123", "*scraper.FacebookScraper", false},
		{"https://www.facebook.com/reel/123", "*scraper.FacebookScraper", false},
		{"https://fb.com/video/123", "*scraper.FacebookScraper", false},
		{"https://web.facebook.com/watch?v=123", "*scraper.FacebookScraper", false},
		// Twitter/X
		{"https://twitter.com/user/status/123", "*scraper.TwitterScraper", false},
		{"https://x.com/user/status/123", "*scraper.TwitterScraper", false},
		{"https://mobile.twitter.com/user/status/123", "*scraper.TwitterScraper", false},
		{"https://www.x.com/user/status/456", "*scraper.TwitterScraper", false},
		// Unsupported
		{"https://vimeo.com/123", "", true},
		{"https://dailymotion.com/video/abc", "", true},
		{"https://twitch.tv/user", "", true},
		{"https://reddit.com/r/videos", "", true},
		{"https://example.com", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			s, err := GetScraper(tc.url)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if s == nil {
				t.Fatal("expected non-nil scraper")
			}
			// Verify type via simple type assertion.
			typeName := fmt.Sprintf("%T", s)
			if !strings.Contains(typeName, tc.wantType) {
				t.Errorf("got scraper type %s, want %s", typeName, tc.wantType)
				}
		})
	}
}

// TestSupportedPlatforms_Content tests the content of SupportedPlatforms.
func TestSupportedPlatforms_Content(t *testing.T) {
	platforms := SupportedPlatforms()
	if len(platforms) == 0 {
		t.Fatal("expected at least one platform")
	}
	// Check that all expected platforms are present.
	expected := []string{"youtube.com", "youtu.be", "tiktok.com", "instagram.com", "facebook.com", "fb.com", "x.com", "twitter.com"}
	for _, e := range expected {
		found := false
		for _, p := range platforms {
			if p == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected platform %q not found", e)
		}
	}
}

// TestContract_GetScraper_WithSchemeVariations tests GetScraper with
// different URL scheme variations.
func TestContract_GetScraper_WithSchemeVariations(t *testing.T) {
	cases := []string{
		"youtube.com/watch?v=abc",
		"http://youtube.com/watch?v=abc",
		"https://youtube.com/watch?v=abc",
		"HTTP://YOUTUBE.COM/WATCH?V=ABC",
		"HTTPS://YOUTUBE.COM/WATCH?V=ABC",
	}
	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			s, err := GetScraper(url)
			if err != nil {
				t.Fatal(err)
			}
			if s == nil {
				t.Fatal("expected non-nil scraper")
			}
		})
	}
}

// TestContract_GetScraper_InvalidURLs tests that invalid URLs return errors.
func TestContract_GetScraper_InvalidURLs(t *testing.T) {
	cases := []string{
		"",
		"://",
		"http://",
		"https://",
		"\x00",
		"not a url at all with spaces",
	}
	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			_, err := GetScraper(url)
			if err == nil {
				t.Error("expected error for invalid URL")
			}
		})
	}
}

// =============================================================================
// YouTube format helper property tests
// =============================================================================

func TestYouTubeFormat_HumanSize_AllUnits(t *testing.T) {
	cases := []struct {
		bytes string
		want  string
	}{
		{"", "Unknown"},
		{"0", "Unknown"},
		{"-1", "Unknown"},
		{"abc", "Unknown"},
		{"1", "1 B"},
		{"999", "999 B"},
		{"1000", "1.0 KB"},
		{"1024", "1.0 KB"},
		{"1500000", "1.5 MB"},
		{"1048576", "1.0 MB"},
		{"2000000000", "2.0 GB"},
		{"1073741824", "1.1 GB"},
		{"999999999999", "1000.0 GB"},
	}
	for _, tc := range cases {
		t.Run(tc.bytes, func(t *testing.T) {
			f := youtubeFormat{ContentLength: tc.bytes}
			got := f.humanSize()
			if got != tc.want {
				t.Errorf("humanSize(%q) = %q, want %q", tc.bytes, got, tc.want)
			}
		})
	}
}

func TestYouTubeFormat_Extension_AllTypes(t *testing.T) {
	cases := []struct {
		mime string
		want string
	}{
		{"video/mp4", "mp4"},
		{"video/webm", "webm"},
		{"video/3gpp", "3gp"},
		{"video/x-flv", "flv"},
		{"audio/mp4", "mp4"},
		{"audio/webm", "webm"},
		{"audio/opus", "mp4"},
		{"video/unknown", "mp4"},
		{"application/x-mpegURL", "mp4"},
		{"", "mp4"},
	}
	for _, tc := range cases {
		t.Run(tc.mime, func(t *testing.T) {
			f := youtubeFormat{MimeType: tc.mime}
			got := f.extension()
			if got != tc.want {
				t.Errorf("extension(%q) = %q, want %q", tc.mime, got, tc.want)
			}
		})
	}
}

func TestYouTubeFormat_Label_AllCases(t *testing.T) {
	cases := []struct {
		ql    string
		mime  string
		want  string
	}{
		{"144p", "video/mp4", "144p"},
		{"240p", "video/mp4", "240p"},
		{"360p", "video/mp4", "360p"},
		{"480p", "video/mp4", "480p"},
		{"720p", "video/mp4", "720p"},
		{"720p60", "video/mp4", "720p60"},
		{"1080p", "video/mp4", "1080p"},
		{"1080p60", "video/mp4", "1080p60"},
		{"1440p", "video/mp4", "1440p"},
		{"2160p", "video/mp4", "2160p"},
		{"4320p", "video/mp4", "4320p"},
		{"", "audio/mp4", "Audio"},
		{"", "audio/webm", "Audio"},
		{"", "audio/opus", "Audio"},
		{"", "video/mp4", "Unknown"},
		{"", "", "Unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.ql+"/"+tc.mime, func(t *testing.T) {
			f := youtubeFormat{QualityLabel: tc.ql, MimeType: tc.mime}
			got := f.label()
			if got != tc.want {
				t.Errorf("label(%q, %q) = %q, want %q", tc.ql, tc.mime, got, tc.want)
			}
		})
	}
}

// =============================================================================
// HTTP status code tests for all platform scrapers
// =============================================================================

func TestAllScrapers_VariousHTTPStatuses(t *testing.T) {
	statuses := []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
	}

	type scraperCase struct {
		name    string
		factory func() domain.VideoScraper
	}

	scrapers := []scraperCase{
		{"YouTube", func() domain.VideoScraper { return NewYouTubeScraper() }},
		{"TikTok", func() domain.VideoScraper { return NewTikTokScraper() }},
		{"Instagram", func() domain.VideoScraper { return NewInstagramScraper() }},
		{"Facebook", func() domain.VideoScraper { return NewFacebookScraper() }},
		{"Twitter", func() domain.VideoScraper { return NewTwitterScraper() }},
	}

	for _, sc := range scrapers {
		for _, status := range statuses {
			t.Run(sc.name+"/"+http.StatusText(status), func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(status)
				}))
				defer srv.Close()

				s := sc.factory()
				_, err := s.Extract(context.Background(), srv.URL)
				if err == nil {
					t.Error("expected error for non-200 status")
				}
			})
		}
	}
}

// =============================================================================
// Context cancellation tests for all platform scrapers (many variants)
// =============================================================================

func TestAllScrapers_ContextCancellationVariants(t *testing.T) {
	scrapers := []struct {
		name string
		s    domain.VideoScraper
	}{
		{"YouTube", NewYouTubeScraper()},
		{"TikTok", NewTikTokScraper()},
		{"Instagram", NewInstagramScraper()},
		{"Facebook", NewFacebookScraper()},
		{"Twitter", NewTwitterScraper()},
	}

	// Test with context cancelled before request.
	t.Run("cancelled before request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		for _, sc := range scrapers {
			t.Run(sc.name, func(t *testing.T) {
				_, err := sc.s.Extract(ctx, "https://example.com/video")
				if err == nil {
					t.Error("expected error with cancelled context")
				}
			})
		}
	})
}

// =============================================================================
// Empty URL tests
// =============================================================================

func TestAllScrapers_EmptyURL(t *testing.T) {
	scrapers := []struct {
		name string
		s    domain.VideoScraper
	}{
		{"YouTube", NewYouTubeScraper()},
		{"TikTok", NewTikTokScraper()},
		{"Instagram", NewInstagramScraper()},
		{"Facebook", NewFacebookScraper()},
		{"Twitter", NewTwitterScraper()},
	}

	for _, sc := range scrapers {
		t.Run(sc.name, func(t *testing.T) {
			_, err := sc.s.Extract(context.Background(), "")
			if err == nil {
				t.Error("expected error for empty URL")
			}
		})
	}
}

// =============================================================================
// Nested JSON-LD tests
// =============================================================================

func TestExtractJSONLD_NestedStructures(t *testing.T) {
	cases := []struct {
		name    string
		html    string
		want    string // expected title
		wantErr bool
	}{
		{
			"simple",
			`<html><script type="application/ld+json">{"name":"Simple","contentUrl":"https://example.com/video.mp4"}</script></html>`,
			"Simple", false,
		},
		{
			"with @context and @type",
			`<html><script type="application/ld+json">{"@context":"https://schema.org","@type":"VideoObject","name":"Structured","contentUrl":"https://example.com/video.mp4"}</script></html>`,
			"Structured", false,
		},
		{
			"with embed URL",
			`<html><script type="application/ld+json">{"name":"Embed URL","embedUrl":"https://example.com/embed","contentUrl":"https://example.com/video.mp4"}</script></html>`,
			"Embed URL", false,
		},
		{
			"description as name fallback",
			`<html><script type="application/ld+json">{"description":"Desc as Title","contentUrl":"https://example.com/video.mp4"}</script></html>`,
			"Desc as Title", false,
		},
		{
			"only contentUrl with no name or description",
			`<html><script type="application/ld+json">{"contentUrl":"https://example.com/video.mp4"}</script></html>`,
			"Video", false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta, err := extractJSONLD(tc.html)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if meta.Title != tc.want {
				t.Errorf("title = %q, want %q", meta.Title, tc.want)
			}
		})
	}
}

// =============================================================================
// YouTube findAnyPlayerResponse - additional coverage
// =============================================================================

func TestFindAnyPlayerResponse_VariousTags(t *testing.T) {
	cases := []struct {
		name    string
		html    string
		wantErr bool
	}{
		{"no scripts", "<html></html>", true},
		{"empty script", "<html><script></script></html>", true},
		{"script with no JSON", "<html><script>var x = 1;</script></html>", true},
		{"script with JSON but no braces", "<html><script>var x = 'hello';</script></html>", true},
		{"valid response", `<html><script>var data = {"videoDetails":{"videoId":"x","title":"Test"},"playabilityStatus":{"status":"OK"}};</script></html>`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := findAnyPlayerResponse(tc.html)
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Error(err)
			}
		})
	}
}

// =============================================================================
// JSON-LD shared helper: multiple blocks
// =============================================================================

func TestExtractJSONLD_MultipleBlocks(t *testing.T) {
	// Should only process the first JSON-LD block.
	html := `<html>
<script type="application/ld+json">{"name":"First","contentUrl":"https://example.com/v1.mp4","thumbnailUrl":"https://example.com/t1.jpg"}</script>
<script type="application/ld+json">{"name":"Second","contentUrl":"https://example.com/v2.mp4","thumbnailUrl":"https://example.com/t2.jpg"}</script>
</html>`
	meta, err := extractJSONLD(html)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "First" {
		t.Errorf("expected 'First', got %q", meta.Title)
	}
}

// =============================================================================
// Platform scrapers: concurrent extraction safety
// =============================================================================

func TestAllScrapers_ConcurrentExtract(t *testing.T) {
	scrapers := []struct {
		name string
		s    domain.VideoScraper
	}{
		{"YouTube", NewYouTubeScraper()},
		{"TikTok", NewTikTokScraper()},
		{"Instagram", NewInstagramScraper()},
		{"Facebook", NewFacebookScraper()},
		{"Twitter", NewTwitterScraper()},
	}

	for _, sc := range scrapers {
		t.Run(sc.name, func(t *testing.T) {
			// Run 10 concurrent requests to same invalid URL.
			// Should all return errors without panicking.
			done := make(chan bool, 10)
			for i := 0; i < 10; i++ {
				go func() {
					_, err := sc.s.Extract(context.Background(), "https://example.com/invalid")
					if err == nil {
						t.Error("expected error for invalid URL")
					}
					done <- true
				}()
			}
			for i := 0; i < 10; i++ {
				<-done
			}
		})
	}
}
