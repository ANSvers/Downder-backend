package scraper

import (
	"strings"
	"testing"
)

// =============================================================================
// Targeted coverage for skipToBrace
// =============================================================================

func TestSkipToBrace_UnknownChar(t *testing.T) {
	// Characters that are not whitespace, operators, or alphanumeric should
	// cause skipToBrace to return -1.
	unknown := []byte{0x00, 0x01, 0x7f, 0xff}
	for _, b := range unknown {
		input := "prefix" + string(b) + "suffix"
		got := skipToBrace(input, 6)
		if got != -1 {
			t.Errorf("skipToBrace with 0x%x = %d, want -1", b, got)
		}
	}
}

func TestSkipToBrace_OutOfBounds(t *testing.T) {
	got := skipToBrace("abc", 10)
	if got != -1 {
		t.Errorf("expected -1 for out-of-bounds start, got %d", got)
	}
}

func TestSkipToBrace_NoBraceFound(t *testing.T) {
	got := skipToBrace("no brace here", 0)
	if got != -1 {
		t.Errorf("expected -1 when no brace, got %d", got)
	}
}

func TestSkipToBrace_Empty(t *testing.T) {
	got := skipToBrace("", 0)
	if got != -1 {
		t.Errorf("expected -1 for empty string, got %d", got)
	}
}

func TestSkipToBrace_SkipOperatorsAndIdentifiers(t *testing.T) {
	// Should skip = : " [ ] ( ) and alphanumeric.
	cases := []struct {
		input string
		start int
		want  int
	}{
		{` = {`, 0, 3},
		{`=   {`, 0, 4},
		{`:"{`, 0, 2},
		{`abc123 = {`, 0, 9},
		{`window.ytPlayer = {`, 0, 18},
		{`["key"] = {`, 0, 10},
		{`({`, 0, 1},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got := skipToBrace(c.input, c.start)
			if got != c.want {
				t.Errorf("skipToBrace(%q, %d) = %d, want %d", c.input, c.start, got, c.want)
			}
		})
	}
}

// =============================================================================
// Targeted coverage for extractScriptTags
// =============================================================================

func TestExtractScriptTags_NoScriptTag(t *testing.T) {
	scripts := extractScriptTags("<html><body>no script here</body></html>")
	if len(scripts) != 0 {
		t.Errorf("expected 0 scripts, got %d", len(scripts))
	}
}

func TestExtractScriptTags_UnclosedOpeningTag(t *testing.T) {
	// <script without a closing > should break.
	scripts := extractScriptTags("<html><script data-src=\"foo")
	if len(scripts) != 0 {
		t.Errorf("expected 0 scripts for unclosed tag, got %d", len(scripts))
	}
}

func TestExtractScriptTags_UnclosedScriptBlock(t *testing.T) {
	// <script> without </script> should break.
	scripts := extractScriptTags("<html><script>var x = 1;")
	if len(scripts) != 0 {
		t.Errorf("expected 0 scripts for unclosed block, got %d", len(scripts))
	}
}

func TestExtractScriptTags_MultipleScripts(t *testing.T) {
	html := `<html>
<script>first</script>
<script>second</script>
<script>third</script>
</html>`
	scripts := extractScriptTags(html)
	if len(scripts) != 3 {
		t.Fatalf("expected 3 scripts, got %d", len(scripts))
	}
	if strings.TrimSpace(scripts[0]) != "first" {
		t.Errorf("scripts[0] = %q", scripts[0])
	}
	if strings.TrimSpace(scripts[1]) != "second" {
		t.Errorf("scripts[1] = %q", scripts[1])
	}
}

func TestExtractScriptTags_WithAttributes(t *testing.T) {
	html := `<script type="text/javascript" async>var x = 1;</script>`
	scripts := extractScriptTags(html)
	if len(scripts) != 1 {
		t.Fatalf("expected 1 script, got %d", len(scripts))
	}
	if strings.TrimSpace(scripts[0]) != "var x = 1;" {
		t.Errorf("script content = %q", scripts[0])
	}
}

func TestExtractScriptTags_EmptyScript(t *testing.T) {
	html := `<script></script>`
	scripts := extractScriptTags(html)
	if len(scripts) != 1 {
		t.Fatalf("expected 1 script, got %d", len(scripts))
	}
	if scripts[0] != "" {
		t.Errorf("expected empty script content, got %q", scripts[0])
	}
}

// =============================================================================

func TestFindByMarkers_AllMarkersFail(t *testing.T) {
	_, err := findByMarkers("<html>no markers here</html>")
	if err == nil {
		t.Fatal("expected error when no markers found")
	}
}

func TestFindByMarkers_MarkerFoundButNoBrace(t *testing.T) {
	// Marker exists but has no JSON object after it.
	_, err := findByMarkers(`<script>var ytInitialPlayerResponse = "string value";</script>`)
	if err == nil {
		t.Fatal("expected error when marker has no JSON object")
	}
}

func TestFindByMarkers_MarkerWithBadJSON(t *testing.T) {
	// Marker followed by invalid JSON.
	_, err := findByMarkers(`<script>var ytInitialPlayerResponse = {invalid json};</script>`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFindByMarkers_MarkerWithEmptyJSON(t *testing.T) {
	// Valid JSON but missing both videoDetails and playabilityStatus.
	_, err := findByMarkers(`<script>var ytInitialPlayerResponse = {"someOtherField": true};</script>`)
	if err == nil {
		t.Fatal("expected error for JSON without required fields")
	}
}

func TestFindByMarkers_UsesSecondMarker(t *testing.T) {
	// First marker not found, second marker should work.
	pr, err := findByMarkers(`<script>var player_response = {"playabilityStatus":{"status":"OK"},"videoDetails":{"videoId":"x","title":"Fallback"}};</script>`)
	if err != nil {
		t.Fatal(err)
	}
	if pr.VideoDetails.Title != "Fallback" {
		t.Errorf("expected title 'Fallback', got %q", pr.VideoDetails.Title)
	}
}

// =============================================================================
// Targeted coverage for findAnyPlayerResponse
// =============================================================================

func TestFindAnyPlayerResponse_NoScripts(t *testing.T) {
	_, err := findAnyPlayerResponse("<html><body>no scripts</body></html>")
	if err == nil {
		t.Fatal("expected error when no script tags")
	}
}

func TestFindAnyPlayerResponse_NoJSONInScripts(t *testing.T) {
	html := `<html><script>console.log("hello");</script></html>`
	_, err := findAnyPlayerResponse(html)
	if err == nil {
		t.Fatal("expected error when no JSON in scripts")
	}
}

func TestFindAnyPlayerResponse_JSONWithoutRequiredFields(t *testing.T) {
	html := `<html><script>var data = {"someKey": [1,2,3]};</script></html>`
	_, err := findAnyPlayerResponse(html)
	if err == nil {
		t.Fatal("expected error when JSON lacks required fields")
	}
}

func TestFindAnyPlayerResponse_FindsVideoDetails(t *testing.T) {
	html := `<html><script>var ytInitialPlayerResponse = {"playabilityStatus":{"status":"OK"},"videoDetails":{"videoId":"abc","title":"Found via script scan"}};</script></html>`
	pr, err := findAnyPlayerResponse(html)
	if err != nil {
		t.Fatal(err)
	}
	if pr.VideoDetails.Title != "Found via script scan" {
		t.Errorf("expected title, got %q", pr.VideoDetails.Title)
	}
}

func TestFindAnyPlayerResponse_FindsPlayabilityStatusOnly(t *testing.T) {
	// JSON with only playabilityStatus (no videoDetails) should still match.
	html := `<html><script>var data = {"playabilityStatus":{"status":"UNPLAYABLE","reason":"Private video"}};</script></html>`
	pr, err := findAnyPlayerResponse(html)
	if err != nil {
		t.Fatal(err)
	}
	if pr.PlayabilityStatus == nil {
		t.Fatal("expected playabilityStatus")
	}
	if pr.PlayabilityStatus.Status != "UNPLAYABLE" {
		t.Errorf("expected UNPLAYABLE, got %q", pr.PlayabilityStatus.Status)
	}
}

func TestFindAnyPlayerResponse_SkipsThenFinds(t *testing.T) {
	// First JSON block doesn't have required fields, second one does.
	html := `<html><script>var a = {"irrelevant": true}; var b = {"playabilityStatus":{"status":"OK"},"videoDetails":{"videoId":"x","title":"Second match"}};</script></html>`
	pr, err := findAnyPlayerResponse(html)
	if err != nil {
		t.Fatal(err)
	}
	if pr.VideoDetails.Title != "Second match" {
		t.Errorf("expected 'Second match', got %q", pr.VideoDetails.Title)
	}
}

// =============================================================================
// Targeted coverage: factory matchHost edge cases
// =============================================================================

func TestMatchHost_Exact(t *testing.T) {
	if !matchHost("youtube.com", "youtube.com") {
		t.Error("expected exact match")
	}
}

func TestMatchHost_Subdomain(t *testing.T) {
	if !matchHost("www.youtube.com", "youtube.com") {
		t.Error("expected subdomain match")
	}
	if !matchHost("m.youtube.com", "youtube.com") {
		t.Error("expected subdomain match")
	}
}

func TestMatchHost_NoMatch(t *testing.T) {
	if matchHost("evil-youtube.com", "youtube.com") {
		t.Error("should not match different domain")
	}
	if matchHost("youtube.com.evil.com", "youtube.com") {
		t.Error("should not match suffixed domain")
	}
}

func TestMatchHost_ShortPattern(t *testing.T) {
	if !matchHost("x.com", "x.com") {
		t.Error("expected exact match for short pattern")
	}
	if matchHost("example.com", "x.com") {
		t.Error("should not match other domain with short pattern")
	}
}


// =============================================================================
// 100% coverage edge cases for findByMarkers and findAnyPlayerResponse
// =============================================================================

func TestFindByMarkers_UnclosedJSON(t *testing.T) {
	// Marker with a `{` found by skipToBrace but JSON is unterminated.
	// This hits the "if err != nil { continue }" path (line 194).
	_, err := findByMarkers(`<script>var ytInitialPlayerResponse = {"unclosed`)
	if err == nil {
		t.Fatal("expected error for unclosed JSON")
	}
}

func TestFindAnyPlayerResponse_UnclosedBlock(t *testing.T) {
	// Script with '{' but unterminated JSON block.
	// This hits line 230: "if err != nil { continue }" after extractJSONBlock.
	html := `<html><script>var x = {"unclosed</script></html>`
	_, err := findAnyPlayerResponse(html)
	if err == nil {
		t.Fatal("expected error when JSON extraction fails")
	}
}

func TestFindByMarkers_ValidJSONWithoutRequiredFields(t *testing.T) {
	// Marker with valid JSON but missing videoDetails and playabilityStatus.
	// This hits the final validation check and continues to next marker.
	// We already have this test, this ensures it runs the full path.
	_, err := findByMarkers(`<script>var ytInitialPlayerResponse = {"someField": true};</script>`)
	if err == nil {
		t.Fatal("expected error for JSON without required fields")
	}
}

func TestFindAnyPlayerResponse_JSONBlockExtractFails(t *testing.T) {
	// Script with a '{' that starts an unterminated JSON block.
	// This hits "if err != nil { continue }" after extractJSONBlock.
	html := `<html><script>var x = {"unclosed</script></html>`
	_, err := findAnyPlayerResponse(html)
	if err == nil {
		t.Fatal("expected error when JSON extraction fails")
	}
}

func TestFindAnyPlayerResponse_JSONParseFails(t *testing.T) {
	// Script with JSON that passes the quick check (has "videoDetails")
	// but fails json.Unmarshal because of a trailing comma.
	// This hits "if err := json.Unmarshal(...); err != nil { continue }".
	html := `<html><script>var x = {"videoDetails": {"videoId": "x"},}</script></html>`
	_, err := findAnyPlayerResponse(html)
	if err == nil {
		t.Fatal("expected error for malformed JSON (trailing comma)")
	}
}


