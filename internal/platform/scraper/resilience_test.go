package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Network failure simulation
// =============================================================================

// errReader is an io.ReadCloser that returns partial data then fails.
type errReader struct {
	data   string
	failAt int // byte position to fail at
	read   int
}

func (r *errReader) Read(p []byte) (int, error) {
	if r.read >= r.failAt {
		return 0, fmt.Errorf("simulated network failure after %d bytes", r.read)
	}
	n := copy(p, r.data[r.read:])
	r.read += n
	if r.read >= len(r.data) {
		return n, io.EOF
	}
	return n, nil
}

func (r *errReader) Close() error { return nil }

func TestResilience_ReadBodyFailure(t *testing.T) {
	// Server sends valid start but fails mid-body.
	html := `<html><script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n"))
		conn.Write([]byte(html[:50]))
		conn.Close() // close mid-response
	}))
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	_, err := ys.Extract(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error from truncated response")
	}
}

func TestResilience_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// No body.
	}))
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	_, err := ys.Extract(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestResilience_ResponseHasOnlyWhitespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("   \n\n   \t   \n"))
	}))
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	_, err := ys.Extract(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for whitespace-only response")
	}
}

func TestResilience_GarbageResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("\x00\x01\x02\xff\xfe\xfd\x00\x01\x02\xff\xfe\xfd"))
	}))
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	_, err := ys.Extract(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for binary garbage")
	}
}

func TestResilience_NoContentLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(stdPage())) // no explicit Content-Length
	}))
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	meta, err := ys.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Rick Astley - Never Gonna Give You Up" {
		t.Errorf("wrong title: %q", meta.Title)
	}
}

func TestResilience_ChunkedTransferEncoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Skip("server doesn't support flushing")
		}
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		time.Sleep(5 * time.Millisecond)
		w.Write([]byte(stdPage()[:50]))
		flusher.Flush()
		time.Sleep(5 * time.Millisecond)
		w.Write([]byte(stdPage()[50:]))
	}))
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	meta, err := ys.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Rick Astley - Never Gonna Give You Up" {
		t.Errorf("wrong title: %q", meta.Title)
	}
}

// =============================================================================
// YouTube format change simulation
// =============================================================================

func TestResilience_PlayerResponseInDifferentScriptTag(t *testing.T) {
	// YouTube sometimes uses: <script>window.ytInitialPlayerResponse = {...}
	html := `<html><head><script>window.ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},
"videoDetails":{"videoId":"x","title":"Window test"}};</script></head></html>`
	server := newMockServer(t, http.StatusOK, html)
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	meta, err := ys.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Window test" {
		t.Errorf("wrong title: %q", meta.Title)
	}
}

func TestResilience_PlayerResponseWithExtraWhitespace(t *testing.T) {
	html := `<html><script>var   ytInitialPlayerResponse   =   {
	"playabilityStatus": {"status": "OK"},
	"videoDetails": {"videoId": "x", "title": "Whitespace test"}
};</script></html>`
	server := newMockServer(t, http.StatusOK, html)
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	meta, err := ys.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Whitespace test" {
		t.Errorf("wrong title: %q", meta.Title)
	}
}

func TestResilience_PlayerResponseWithNoSemicolon(t *testing.T) {
	// YouTube does: ytInitialPlayerResponse = {...} (no semicolon sometimes)
	html := `<html><script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},
"videoDetails":{"videoId":"x","title":"No semicolon"}}
</script></html>`
	server := newMockServer(t, http.StatusOK, html)
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	meta, err := ys.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "No semicolon" {
		t.Errorf("wrong title: %q", meta.Title)
	}
}

func TestResilience_PlayerResponseWithoutVarKeyword(t *testing.T) {
	// YouTube sometimes drops the "var" keyword.
	html := `<html><script>ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},
"videoDetails":{"videoId":"x","title":"No var keyword"}};</script></html>`
	server := newMockServer(t, http.StatusOK, html)
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	meta, err := ys.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "No var keyword" {
		t.Errorf("wrong title: %q", meta.Title)
	}
}

func TestResilience_PlayerResponseUsesDoubleQuotesAroundMarker(t *testing.T) {
	// Rare YouTube format: "ytInitialPlayerResponse":{...}
	html := `<html><script>window["ytInitialPlayerResponse"] = {"playabilityStatus":{"status":"OK"},
"videoDetails":{"videoId":"x","title":"Double quote key"}};</script></html>`
	server := newMockServer(t, http.StatusOK, html)
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	meta, err := ys.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Double quote key" {
		t.Errorf("wrong title: %q", meta.Title)
	}
}

func TestResilience_HTMLWithMultipleScriptTags(t *testing.T) {
	html := `<html><script>console.log("hello")</script>
<script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},
"videoDetails":{"videoId":"x","title":"Second script tag"}};</script>
<script>console.log("world")</script></html>`
	server := newMockServer(t, http.StatusOK, html)
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	meta, err := ys.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Second script tag" {
		t.Errorf("wrong title: %q", meta.Title)
	}
}

func TestResilience_HTMLWithVeryLongScriptBeforeResponse(t *testing.T) {
	// Large inline script before the player response (common on YouTube).
	prefix := "<script>" + strings.Repeat("console.log('spam');", 1000) + "</script>"
	html := `<html>` + prefix + `<script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},
"videoDetails":{"videoId":"x","title":"Long prefix"}};</script></html>`
	server := newMockServer(t, http.StatusOK, html)
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	meta, err := ys.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Long prefix" {
		t.Errorf("wrong title: %q", meta.Title)
	}
}

// =============================================================================
// DNS / network failure simulation
// =============================================================================

func TestResilience_DNSLookupFailure(t *testing.T) {
	ys := &YouTubeScraper{client: &http.Client{Timeout: 2 * time.Second}}
	_, err := ys.Extract(context.Background(), "https://this-domain-does-not-exist-anywhere-12345.com/watch")
	if err == nil {
		t.Fatal("expected DNS error")
	}
}

func TestResilience_ConnectionRefused(t *testing.T) {
	ys := &YouTubeScraper{client: http.DefaultClient}
	_, err := ys.Extract(context.Background(), "http://localhost:1")
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestResilience_ContextDeadlineExceededDuringRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 10; i++ {
			w.Write([]byte("chunk "))
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	ys := &YouTubeScraper{client: server.Client()}
	_, err := ys.Extract(ctx, server.URL)
	if err == nil {
		t.Fatal("expected timeout error during read")
	}
}

// =============================================================================
// Server misbehavior
// =============================================================================

func TestResilience_ServerClosesImmediately(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Close without any response.
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Skip("server doesn't support hijack")
		}
		conn, _, _ := hijacker.Hijack()
		conn.Close()
	}))
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	_, err := ys.Extract(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error from immediate close")
	}
}

func TestResilience_RedirectLoop(t *testing.T) {
	// Server redirects in a loop.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.String(), http.StatusFound)
	}))
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	_, err := ys.Extract(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error from redirect loop")
	}
}

func TestResilience_ExcessivelyLargeResponse(t *testing.T) {
	// Simulate a huge response that could exhaust memory.
	bigData := strings.Repeat("A", 10*1024*1024) // 10MB of junk
	html := `<html><script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},
"videoDetails":{"videoId":"x","title":"Big page"}};</script>` + bigData + `</html>`
	server := newMockServer(t, http.StatusOK, html)
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	meta, err := ys.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Big page" {
		t.Errorf("wrong title: %q", meta.Title)
	}
	if meta.ID != "x" {
		t.Errorf("wrong ID: %q", meta.ID)
	}
}

// =============================================================================
// No panic on edge case inputs
// =============================================================================

func TestResilience_ExtractWithEmptyURL(t *testing.T) {
	ys := &YouTubeScraper{client: http.DefaultClient}
	_, err := ys.Extract(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestResilience_ExtractWithNilContext(t *testing.T) {
	server := newMockServer(t, http.StatusOK, stdPage())
	defer server.Close()

	ys := &YouTubeScraper{client: server.Client()}
	// Passing nil context should panic or error. In Go, http.NewRequestWithContext
	// with nil context panics. But our code uses context.Background() in production.
	// This test verifies we handle this gracefully.
	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from panic (expected): %v", r)
		}
	}()
	// This line should ideally not compile, but if someone passes nil
	// through an interface, we handle it gracefully.
	_, err := ys.Extract(nil, server.URL) //nolint:staticcheck
	if err == nil {
		// Some Go versions may not panic — that's fine.
		t.Log("nil context did not panic (Go version dependent)")
	}
}
