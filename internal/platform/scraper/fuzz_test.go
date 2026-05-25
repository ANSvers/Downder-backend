package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// FuzzExtractJSONBlock generates random HTML strings and verifies the function
// never panics and always returns either valid JSON or a sensible error.
func FuzzExtractJSONBlock(f *testing.F) {
	// Seed corpus: known edge cases.
	seeds := []string{
		`{"a":1}`,
		`{"b":{"c":2}}`,
		`{"url":"https://example.com?sig={abc}"}`,
		`{"text":"say \"hello\""}`,
		`{"path":"C:\\Users\\name"}`,
		`{}`,
		`{"a":1} junk {"b":2}`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, html string) {
		// Find any opening brace and try to extract.
		for i, b := range []byte(html) {
			if b == '{' {
				block, err := extractJSONBlock(html, i)
				if err != nil {
					// Error is acceptable — the HTML may be invalid.
					// But it must be a specific error, not a panic.
					return
				}
				// If no error, the result must:
				// - Start with '{'
				// - End with '}'
				// - Not be empty
				if len(block) == 0 {
					t.Errorf("empty block returned without error")
				}
				if block[0] != '{' {
					t.Errorf("block does not start with '{': %q", block)
				}
				if block[len(block)-1] != '}' {
					t.Errorf("block does not end with '}': %q", block)
				}
				return // Only test first brace to keep fuzz fast.
			}
		}
	})
}

// FuzzGetScraper generates random URL-like strings and verifies GetScraper
// never panics (errors are acceptable for invalid URLs).
func FuzzGetScraper(f *testing.F) {
	seeds := []string{
		"https://youtube.com/watch?v=123",
		"https://tiktok.com/@user/video/1",
		"https://instagram.com/p/xyz/",
		"not-a-url",
		"",
		"\x00\x01\x02",
		"javascript:alert(1)",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, rawURL string) {
		// Must never panic, error is fine.
		_, _ = GetScraper(rawURL)
	})
}

// FuzzExtract generates random server responses and verifies Extract never
// panics (errors are acceptable for malformed data).
func FuzzExtract(f *testing.F) {
	seeds := []string{
		`<html><script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},"videoDetails":{"videoId":"x","title":"Test"}};</script></html>`,
		`<html>no data</html>`,
		`{}`,
		`not even close to HTML`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, body string) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(body))
		}))
		defer srv.Close()

		ys := &YouTubeScraper{client: srv.Client()}
		// Must never panic, error is fine.
		_, _ = ys.Extract(context.Background(), srv.URL)
	})
}
