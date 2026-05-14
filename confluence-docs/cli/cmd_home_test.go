// cmd_home_test.go — unit tests for `confluence-docs home` subcommand.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lybel-app/skills/pkg/atlassian/adf"
	"github.com/lybel-app/skills/confluence-docs/cli/setup"
)

// ── pure helpers ──────────────────────────────────────────────────────────────

func TestIsMarkdownHeading(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"# Title", true},
		{"## Section", true},
		{"### Sub", true},
		{"###### H6", true},
		{"####### Too many", false}, // 7 hashes — not a heading
		{"#NoSpace", false},         // no space after hash
		{"plain line", false},
		{"", false},
		{"#", false},   // single hash, no space after
		{" # not", false}, // leading space — not a heading
	}
	for _, tc := range cases {
		got := isMarkdownHeading(tc.line)
		if got != tc.want {
			t.Errorf("isMarkdownHeading(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func TestTrimHashPrefix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"# Hello", "Hello"},
		{"## World", "World"},
		{"### Nested Heading", "Nested Heading"},
		{"#NoSpace", "NoSpace"},
		{"plain", "plain"},
		{"", ""},
	}
	for _, tc := range cases {
		got := trimHashPrefix(tc.input)
		if got != tc.want {
			t.Errorf("trimHashPrefix(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFormatDurationCompact(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0 * time.Second, "0s"},
		{1 * time.Second, "1s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{90 * time.Second, "1m"},
		{2 * time.Minute, "2m"},
		{59 * time.Minute, "59m"},
		{60 * time.Minute, "1h"},
		{90 * time.Minute, "1h"},
		{23 * time.Hour, "23h"},
		{24 * time.Hour, "1d"},
		{48 * time.Hour, "2d"},
		{72 * time.Hour, "3d"},
	}
	for _, tc := range cases {
		got := formatDurationCompact(tc.d)
		if got != tc.want {
			t.Errorf("formatDurationCompact(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// ── grepHome ─────────────────────────────────────────────────────────────────

func TestGrepHome_EmptyContent(t *testing.T) {
	got := grepHome("", "anything")
	if len(got) != 0 {
		t.Errorf("expected no matches on empty content, got %v", got)
	}
}

func TestGrepHome_EmptyQuery(t *testing.T) {
	got := grepHome("some content here", "")
	if got != nil {
		t.Errorf("expected nil for empty query, got %v", got)
	}
}

func TestGrepHome_NoMatches(t *testing.T) {
	content := "# Section\nThis is a line\nAnother line"
	got := grepHome(content, "neverfinds")
	if len(got) != 0 {
		t.Errorf("expected 0 matches, got %v", got)
	}
}

func TestGrepHome_SingleMatchWithHeading(t *testing.T) {
	content := "# Features\nSupports recurring billing\nAnother line"
	got := grepHome(content, "recurring")
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d: %v", len(got), got)
	}
	if got[0].heading != "Features" {
		t.Errorf("heading: got %q, want %q", got[0].heading, "Features")
	}
	if !strings.Contains(got[0].line, "recurring billing") {
		t.Errorf("line: got %q, want it to contain 'recurring billing'", got[0].line)
	}
}

func TestGrepHome_CaseInsensitive(t *testing.T) {
	content := "# Section\nThis Has Mixed CASE content"
	got := grepHome(content, "mixed case")
	if len(got) != 1 {
		t.Fatalf("expected 1 case-insensitive match, got %d", len(got))
	}
}

func TestGrepHome_MatchBeforeAnyHeading(t *testing.T) {
	content := "Intro paragraph with target\n# Section"
	got := grepHome(content, "target")
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].heading != "" {
		t.Errorf("expected empty heading for pre-heading match, got %q", got[0].heading)
	}
}

func TestGrepHome_MultipleMatchesSameHeading_Deduped(t *testing.T) {
	content := "# MySection\nfoo first line\nfoo second line\nfoo third line"
	got := grepHome(content, "foo")
	// First match carries the heading; subsequent ones under the same heading get empty heading.
	if len(got) != 3 {
		t.Fatalf("expected 3 matches, got %d: %v", len(got), got)
	}
	if got[0].heading != "MySection" {
		t.Errorf("first match heading: got %q, want %q", got[0].heading, "MySection")
	}
	if got[1].heading != "" {
		t.Errorf("second match heading should be empty (deduped), got %q", got[1].heading)
	}
	if got[2].heading != "" {
		t.Errorf("third match heading should be empty (deduped), got %q", got[2].heading)
	}
}

func TestGrepHome_MultipleHeadingsDifferentMatches(t *testing.T) {
	content := "# Alpha\nfoo in alpha\n# Beta\nfoo in beta"
	got := grepHome(content, "foo")
	if len(got) != 2 {
		t.Fatalf("expected 2 matches (one per section), got %d: %v", len(got), got)
	}
	if got[0].heading != "Alpha" {
		t.Errorf("first match heading: got %q, want %q", got[0].heading, "Alpha")
	}
	if got[1].heading != "Beta" {
		t.Errorf("second match heading: got %q, want %q", got[1].heading, "Beta")
	}
}

func TestGrepHome_BlankLinesIgnored(t *testing.T) {
	content := "# Section\n\n\nfoo here\n\n"
	got := grepHome(content, "foo")
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
}

// ── runHome flag parsing ──────────────────────────────────────────────────────

// runHomeCmd is a convenience wrapper that redirects config/cache dirs.
func runHomeCmd(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)
	var outBuf, errBuf bytes.Buffer
	code, _ = run(append([]string{"home"}, args...), strings.NewReader(""), &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

func TestRunHome_Help(t *testing.T) {
	dir := t.TempDir()
	out, _, code := runHomeCmd(t, dir, "--help")
	if code != exitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(out, "--refresh") {
		t.Errorf("help should mention --refresh, got:\n%s", out)
	}
	if !strings.Contains(out, "--query") {
		t.Errorf("help should mention --query, got:\n%s", out)
	}
}

func TestRunHome_UnknownFlag(t *testing.T) {
	dir := t.TempDir()
	_, errOut, code := runHomeCmd(t, dir, "--not-a-flag")
	if code == exitOK {
		t.Fatal("expected non-zero exit for unknown flag")
	}
	if !strings.Contains(errOut, "unknown flag") {
		t.Errorf("expected 'unknown flag' in stderr, got %q", errOut)
	}
}

func TestRunHome_QueryMissingValue(t *testing.T) {
	dir := t.TempDir()
	_, errOut, code := runHomeCmd(t, dir, "--query")
	if code == exitOK {
		t.Fatal("expected error when --query has no value")
	}
	if !strings.Contains(errOut, "--query requires a value") {
		t.Errorf("expected '--query requires a value' in stderr, got %q", errOut)
	}
}

func TestRunHome_MaxAgeMissingValue(t *testing.T) {
	dir := t.TempDir()
	_, errOut, code := runHomeCmd(t, dir, "--max-age")
	if code == exitOK {
		t.Fatal("expected error when --max-age has no value")
	}
	if !strings.Contains(errOut, "--max-age requires a duration") {
		t.Errorf("expected '--max-age requires a duration' in stderr, got %q", errOut)
	}
}

func TestRunHome_MaxAgeInvalid(t *testing.T) {
	dir := t.TempDir()
	_, errOut, code := runHomeCmd(t, dir, "--max-age", "notaduration")
	if code == exitOK {
		t.Fatal("expected error for invalid --max-age value")
	}
	if !strings.Contains(errOut, "--max-age") {
		t.Errorf("expected '--max-age' in stderr error, got %q", errOut)
	}
}

func TestRunHome_PageIDMissingValue(t *testing.T) {
	dir := t.TempDir()
	_, errOut, code := runHomeCmd(t, dir, "--page-id")
	if code == exitOK {
		t.Fatal("expected error when --page-id has no value")
	}
	if !strings.Contains(errOut, "--page-id requires a value") {
		t.Errorf("expected '--page-id requires a value' in stderr, got %q", errOut)
	}
}

func TestRunHome_RefreshWithoutCredentials(t *testing.T) {
	dir := t.TempDir()
	// Write config with a home page ID so that path reaches buildClient.
	writeTestConfig(t, dir, setup.Config{
		Cloud:      "testcloud",
		HomePageID: "12345",
	})
	// No credentials file → buildClient should fail.
	_, errOut, code := runHomeCmd(t, dir, "--refresh")
	// Must not exit 0.
	if code == exitOK {
		t.Fatal("expected failure without credentials, got exit 0")
	}
	_ = errOut // error message varies by auth resolution path
}

func TestRunHome_RefreshWithoutPageID(t *testing.T) {
	dir := t.TempDir()
	// No config at all → no home page ID.
	_, errOut, code := runHomeCmd(t, dir, "--refresh")
	if code == exitOK {
		t.Fatal("expected failure without page ID")
	}
	if !strings.Contains(errOut, "no home page configured") {
		t.Errorf("expected 'no home page configured' in stderr, got %q", errOut)
	}
}

func TestRunHome_StatusMissingCache(t *testing.T) {
	dir := t.TempDir()
	// No cache file present.
	_, errOut, code := runHomeCmd(t, dir, "--status")
	if code == exitOK {
		t.Fatal("expected failure when cache is missing")
	}
	if !strings.Contains(errOut, "no home cache") {
		t.Errorf("expected 'no home cache' message, got %q", errOut)
	}
}

func TestRunHome_QueryMissingCache_NoPageID(t *testing.T) {
	dir := t.TempDir()
	// No cache, no page ID configured → should report no home page configured.
	_, errOut, code := runHomeCmd(t, dir, "--query", "anything")
	if code == exitOK {
		t.Fatal("expected failure when cache missing and no page ID")
	}
	if !strings.Contains(errOut, "no home page configured") {
		t.Errorf("expected 'no home page configured', got %q", errOut)
	}
}

// writeFixtureHomeCache writes a HomeCache JSON fixture to the cache dir.
func writeFixtureHomeCache(t *testing.T, dir string, c *adf.HomeCache) {
	t.Helper()
	cacheDir := filepath.Join(dir, "confluence-docs")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cacheDir, "home.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// freshCache returns a HomeCache with FetchedAt = now so it is never stale.
func freshCache(textContent string) *adf.HomeCache {
	return &adf.HomeCache{
		FetchedAt:   time.Now().UTC(),
		PageID:      "99999",
		Title:       "Home Page",
		Version:     7,
		URL:         "https://example.atlassian.net/wiki/spaces/ENG/pages/99999",
		TextContent: textContent,
		Digest:      adf.Digest{},
	}
}

func TestRunHome_StatusWithCache(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)

	writeFixtureHomeCache(t, dir, freshCache("Some content here."))

	out, _, code := runHomeCmd(t, dir, "--status")
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nout: %s", code, out)
	}
	if !strings.Contains(out, "home v7") {
		t.Errorf("expected 'home v7' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Home Page") {
		t.Errorf("expected title in output, got:\n%s", out)
	}
	if !strings.Contains(out, "path:") {
		t.Errorf("expected 'path:' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "url:") {
		t.Errorf("expected 'url:' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "size:") {
		t.Errorf("expected 'size:' in output, got:\n%s", out)
	}
}

func TestRunHome_DefaultActionIsStatus(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)

	writeFixtureHomeCache(t, dir, freshCache("Content."))

	// No flags: default action should be --status.
	out, _, code := runHomeCmd(t, dir)
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nout: %s", code, out)
	}
	// Status output always contains "home v".
	if !strings.Contains(out, "home v") {
		t.Errorf("expected status output with 'home v', got:\n%s", out)
	}
}

func TestRunHome_ShowWithCache(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)

	content := "# Features\nRecurring billing\nClient management"
	writeFixtureHomeCache(t, dir, freshCache(content))

	out, _, code := runHomeCmd(t, dir, "--show")
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nout: %s", code, out)
	}
	if !strings.Contains(out, "Recurring billing") {
		t.Errorf("expected cached text in output, got:\n%s", out)
	}
}

func TestRunHome_QueryWithCache_Matches(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)

	content := "# Integrations\nAbacatePay for payments\n# Team\nDiego is founder"
	writeFixtureHomeCache(t, dir, freshCache(content))

	out, errOut, code := runHomeCmd(t, dir, "--query", "abacatepay")
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nerr: %s", code, errOut)
	}
	if !strings.Contains(out, "AbacatePay") {
		t.Errorf("expected match line in output, got:\n%s", out)
	}
	if !strings.Contains(out, "## Integrations") {
		t.Errorf("expected heading in output, got:\n%s", out)
	}
}

func TestRunHome_QueryWithCache_NoMatches(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)

	writeFixtureHomeCache(t, dir, freshCache("# Section\nSome content"))

	_, errOut, code := runHomeCmd(t, dir, "--query", "neverfindsthis")
	if code != exitOK {
		t.Fatalf("want exit 0 (no matches is not an error), got %d", code)
	}
	if !strings.Contains(errOut, "no matches") {
		t.Errorf("expected 'no matches' message in stderr, got %q", errOut)
	}
}

func TestRunHome_QueryCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)

	content := "# Tech\nMonolith Architecture in Go"
	writeFixtureHomeCache(t, dir, freshCache(content))

	out, _, code := runHomeCmd(t, dir, "--query", "MONOLITH")
	if code != exitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(out, "Monolith") {
		t.Errorf("expected case-insensitive match, got:\n%s", out)
	}
}

func TestRunHome_StaleCache_NoPageID(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)

	// Write a cache that is already old.
	stale := &adf.HomeCache{
		FetchedAt:   time.Now().UTC().Add(-2 * time.Hour),
		PageID:      "99999",
		Title:       "Home",
		Version:     1,
		URL:         "https://example.atlassian.net/wiki/pages/99999",
		TextContent: "# Section\nsome content",
	}
	writeFixtureHomeCache(t, dir, stale)

	// No page ID configured → auto-refresh will fail with "no home page configured".
	_, errOut, code := runHomeCmd(t, dir, "--query", "content")
	if code == exitOK {
		t.Fatal("expected failure: stale cache + no page ID should not exit 0")
	}
	if !strings.Contains(errOut, "no home page configured") {
		t.Errorf("expected 'no home page configured', got %q", errOut)
	}
}
