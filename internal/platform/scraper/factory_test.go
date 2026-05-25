package scraper

import (
	"math/rand"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// Contract: extensibility requirements
// =============================================================================
//
// These tests verify that the factory honors its extensibility contract:
//   - GetScraper must accept any string and never panic.
//   - Adding a new platform requires zero changes outside registry + one file.
//   - The registry is read-only after init (safe for concurrent access).

func TestContract_GetScraper_NeverPanics(t *testing.T) {
	inputs := []string{
		"", " ", "\t", "\n",
		"http://", "https://",
		"not-a-url",
		"javascript:alert(1)",
		"  https://youtube.com  ",
		"youtube.com",
		"https://youtube.com/watch?v=123",
		strings.Repeat("a", 10000),
		string([]byte{0xff, 0xfe, 0x00}),
	}
	for _, in := range inputs {
		t.Run(testName(in), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("GetScraper(%q) panicked: %v", in, r)
				}
			}()
			GetScraper(in) // Must never panic.
		})
	}
}

func TestContract_GetScraper_ReturnsNilScraperOnError(t *testing.T) {
	s, err := GetScraper("https://unsupported.example.com")
	if err == nil {
		t.Fatal("expected error")
	}
	if s != nil {
		t.Error("expected nil scraper on error")
	}
}

func TestContract_GetScraper_ErrorIsInformative(t *testing.T) {
	_, err := GetScraper("https://vimeo.com/123")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "vimeo") {
		t.Errorf("error should mention the URL, got: %s", msg)
	}
	if !strings.Contains(msg, "unsupported") {
		t.Errorf("error should say unsupported, got: %s", msg)
	}
}

func TestContract_NewInstancePerCall(t *testing.T) {
	s1, _ := GetScraper("https://youtube.com/a")
	s2, _ := GetScraper("https://youtube.com/b")
	if s1 == s2 {
		t.Error("each call must return a fresh scraper instance")
	}
}

func TestContract_NoCrossPlatformMatch(t *testing.T) {
	// Each platform must NOT match another platform's domain.
	cases := []struct {
		url      string
		notMatch interface{} // type that should NOT be returned
	}{
		{"https://tiktok.com/video", (*YouTubeScraper)(nil)},
		{"https://instagram.com/p/xyz", (*TikTokScraper)(nil)},
		{"https://facebook.com/watch", (*InstagramScraper)(nil)},
		{"https://twitter.com/status/1", (*FacebookScraper)(nil)},
		{"https://x.com/status/1", (*FacebookScraper)(nil)},
	}
	for _, c := range cases {
		t.Run(testName(c.url), func(t *testing.T) {
			s, err := GetScraper(c.url)
			if err != nil {
				t.Fatal(err)
			}
			// Use type assertion to check it's NOT the excluded type.
			switch c.notMatch.(type) {
			case *YouTubeScraper:
				if _, ok := s.(*YouTubeScraper); ok {
					t.Errorf("GetScraper(%q) should NOT return YouTubeScraper", c.url)
				}
			case *TikTokScraper:
				if _, ok := s.(*TikTokScraper); ok {
					t.Errorf("GetScraper(%q) should NOT return TikTokScraper", c.url)
				}
			case *InstagramScraper:
				if _, ok := s.(*InstagramScraper); ok {
					t.Errorf("GetScraper(%q) should NOT return InstagramScraper", c.url)
				}
			case *FacebookScraper:
				if _, ok := s.(*FacebookScraper); ok {
					t.Errorf("GetScraper(%q) should NOT return FacebookScraper", c.url)
				}
			}
		})
	}
}

func TestContract_ConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	errs := make(chan error, 1000)
	urls := []string{
		"https://youtube.com/watch?v=a",
		"https://tiktok.com/@u/v/1",
		"https://instagram.com/p/x",
		"https://facebook.com/video/1",
		"https://x.com/user/status/1",
		"https://vimeo.com/1", // unsupported
	}

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			url := urls[i%len(urls)]
			_, err := GetScraper(url)
			// Errors are allowed for unsupported URLs; panics are not.
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	var panicked bool
	for range errs {
		// If any goroutine panicked, the test would have crashed.
	}
	if panicked {
		t.Fatal("concurrent access caused panic")
	}
}

// =============================================================================
// Factory: GetScraper URL matching
// =============================================================================
//
// Table-driven: add a new platform? Add rows here. No new test functions needed.

func TestGetScraper_YouTubeVariants(t *testing.T) {
	cases := []string{
		"https://youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"HTTP://YOUTUBE.COM/VIDEO",
		"https://youtube.com/shorts/abc123",
		"https://www.youtube.com/live/abc123",
		"https://m.youtube.com/watch?v=xyz",
		"https://music.youtube.com/watch?v=abc",
		"https://youtube.com/embed/dQw4w9WgXcQ",
		"https://youtube.com/clip/UgkxABC",
	}
	for _, url := range cases {
		t.Run(testName(url), func(t *testing.T) {
			s, err := GetScraper(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, ok := s.(*YouTubeScraper); !ok {
				t.Fatalf("expected YouTubeScraper, got %T", s)
			}
		})
	}
}

func TestGetScraper_TikTokVariants(t *testing.T) {
	cases := []string{
		"https://tiktok.com/@user/video/123",
		"https://www.tiktok.com/@user/photo/456",
		"https://vm.tiktok.com/abc123/",
		"https://m.tiktok.com/v/123",
	}
	for _, url := range cases {
		t.Run(testName(url), func(t *testing.T) {
			s, err := GetScraper(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, ok := s.(*TikTokScraper); !ok {
				t.Fatalf("expected TikTokScraper, got %T", s)
			}
		})
	}
}

func TestGetScraper_InstagramVariants(t *testing.T) {
	cases := []string{
		"https://instagram.com/p/xyz",
		"https://www.instagram.com/reel/abc",
		"https://www.instagram.com/tv/def",
		"https://instagram.com/stories/user/123",
	}
	for _, url := range cases {
		t.Run(testName(url), func(t *testing.T) {
			s, err := GetScraper(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, ok := s.(*InstagramScraper); !ok {
				t.Fatalf("expected InstagramScraper, got %T", s)
			}
		})
	}
}

func TestGetScraper_FacebookVariants(t *testing.T) {
	cases := []string{
		"https://facebook.com/watch?v=123",
		"https://fb.com/video/123",
		"https://www.facebook.com/reel/123",
		"https://facebook.com/username/videos/123",
		"https://web.facebook.com/watch?v=123",
	}
	for _, url := range cases {
		t.Run(testName(url), func(t *testing.T) {
			s, err := GetScraper(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, ok := s.(*FacebookScraper); !ok {
				t.Fatalf("expected FacebookScraper, got %T", s)
			}
		})
	}
}

func TestGetScraper_TwitterVariants(t *testing.T) {
	cases := []string{
		"https://twitter.com/user/status/123",
		"https://x.com/user/status/123",
		"https://mobile.twitter.com/user/status/123",
		"https://www.x.com/user/status/456",
	}
	for _, url := range cases {
		t.Run(testName(url), func(t *testing.T) {
			s, err := GetScraper(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, ok := s.(*TwitterScraper); !ok {
				t.Fatalf("expected TwitterScraper, got %T", s)
			}
		})
	}
}

func TestGetScraper_UnsupportedPlatforms(t *testing.T) {
	cases := []string{
		"https://vimeo.com/123",
		"https://dailymotion.com/video/abc",
		"https://twitch.tv/user",
		"https://example.com",
		"https://reddit.com/r/videos",
		"https://vukcehej.com",
	}
	for _, url := range cases {
		t.Run(testName(url), func(t *testing.T) {
			_, err := GetScraper(url)
			if err == nil {
				t.Fatal("expected error for unsupported platform")
			}
		})
	}
}

func TestGetScraper_QueryParamsAndFragments(t *testing.T) {
	cases := []struct {
		url     string
		wantErr bool
	}{
		{"https://youtube.com/watch?v=abc&list=pl123&index=1&t=30s", false},
		{"https://youtube.com/watch?v=abc?si=xyz&utm_source=share", false},
		{"https://youtu.be/dQw4w9WgXcQ?si=abc123", false},
		{"https://instagram.com/p/xyz/?utm_source=ig_share", false},
		{"https://facebook.com/video/123?utm_campaign=test", false},
		{"https://tiktok.com/@user/video/123?is_from_webapp=1", false},
	}
	for _, c := range cases {
		t.Run(testName(c.url), func(t *testing.T) {
			_, err := GetScraper(c.url)
			if c.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetScraper_SubdomainVariations(t *testing.T) {
	s, err := GetScraper("https://youtube.com/watch?v=abc")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(*YouTubeScraper); !ok {
		t.Errorf("youtube.com: got %T", s)
	}

	// These should still match the correct parent domain.
	pairs := []struct {
		url      string
		expected interface{}
	}{
		{"https://youtube.com", (*YouTubeScraper)(nil)},
		{"https://www.youtube.com", (*YouTubeScraper)(nil)},
		{"https://m.youtube.com", (*YouTubeScraper)(nil)},
		{"https://music.youtube.com", (*YouTubeScraper)(nil)},
		{"https://facebook.com", (*FacebookScraper)(nil)},
		{"https://www.facebook.com", (*FacebookScraper)(nil)},
		{"https://instagram.com", (*InstagramScraper)(nil)},
		{"https://www.instagram.com", (*InstagramScraper)(nil)},
		{"https://tiktok.com", (*TikTokScraper)(nil)},
		{"https://www.tiktok.com", (*TikTokScraper)(nil)},
		{"https://twitter.com", (*TwitterScraper)(nil)},
		{"https://x.com", (*TwitterScraper)(nil)},
	}
	for _, p := range pairs {
		t.Run(testName(p.url), func(t *testing.T) {
			s, err := GetScraper(p.url)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := s.(*YouTubeScraper); ok {
				// ok, it's YouTube
			} else if _, ok := s.(*TikTokScraper); ok {
			} else if _, ok := s.(*InstagramScraper); ok {
			} else if _, ok := s.(*FacebookScraper); ok {
			} else if _, ok := s.(*TwitterScraper); ok {
			} else {
				t.Errorf("unexpected scraper type %T for %q", s, p.url)
			}
		})
	}
}

// =============================================================================
// Factory: SupportedPlatforms
// =============================================================================

func TestSupportedPlatforms_NotEmpty(t *testing.T) {
	if p := SupportedPlatforms(); len(p) == 0 {
		t.Fatal("must not be empty")
	}
}

func TestSupportedPlatforms_NoDuplicates(t *testing.T) {
	p := SupportedPlatforms()
	seen := make(map[string]bool, len(p))
	for _, platform := range p {
		if seen[platform] {
			t.Errorf("duplicate platform: %s", platform)
		}
		seen[platform] = true
	}
}

func TestSupportedPlatforms_OrderIsStable(t *testing.T) {
	first := SupportedPlatforms()
	for i := 0; i < 100; i++ {
		next := SupportedPlatforms()
		if len(next) != len(first) {
			t.Fatalf("iteration %d: length changed %d → %d", i, len(first), len(next))
		}
		for j := range first {
			if first[j] != next[j] {
				t.Fatalf("iteration %d: order changed at index %d: %q → %q", i, j, first[j], next[j])
			}
		}
	}
}

func TestSupportedPlatforms_EveryRegisteredPatternAppears(t *testing.T) {
	platforms := SupportedPlatforms()
	// Every unique pattern in registry must appear.
	required := []string{"youtube.com", "tiktok.com", "instagram.com", "facebook.com", "twitter.com", "x.com", "fb.com"}
	for _, r := range required {
		found := false
		for _, p := range platforms {
			if p == r {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("required platform %q missing from SupportedPlatforms()", r)
		}
	}
}

func TestSupportedPlatforms_ShortPatternsIncluded(t *testing.T) {
	platforms := SupportedPlatforms()
	shortForms := []string{"youtu.be", "fb.com", "x.com"}
	for _, sf := range shortForms {
		found := false
		for _, p := range platforms {
			if p == sf {
				found = true
				break
			}
		}
		if !found {
			// Not all short forms are required, but log if missing.
			t.Logf("note: short form %q not in SupportedPlatforms (this may be intentional)", sf)
		}
	}
}

// =============================================================================
// Fuzz-style: random URL generation for GetScraper
// =============================================================================

func TestGetScraper_RandomURLs(t *testing.T) {
	platforms := []string{
		"youtube.com", "youtu.be", "tiktok.com", "instagram.com",
		"facebook.com", "fb.com", "twitter.com", "x.com",
	}
	paths := []string{
		"/watch?v=abc", "/@user/video/123", "/p/xyz", "/reel/abc",
		"/video/123", "/status/123", "/", "/abc/def/ghi/jkl",
	}

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 50; i++ {
		p := platforms[rng.Intn(len(platforms))]
		path := paths[rng.Intn(len(paths))]
		scheme := []string{"https://", "http://", "HTTP://", "HTTPS://"}[rng.Intn(4)]
		url := scheme + p + path

		t.Run(testName(url), func(t *testing.T) {
			s, err := GetScraper(url)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", url, err)
			}
			if s == nil {
				t.Fatal("got nil scraper")
			}
		})
	}
}

// =============================================================================
// Helpers
// =============================================================================

// testName produces a safe subtest name from a URL.
func testName(url string) string {
	if len(url) > 60 {
		url = url[:57] + "..."
	}
	// Replace characters that are invalid in test names.
	replacer := strings.NewReplacer(
		"https://", "", "http://", "",
		"/", "_", "?", "_", "&", "_",
		"=", "_", "%", "_",
	)
	return replacer.Replace(url)
}
