// cmd_space_test.go — tests for `confluence-docs space` subcommands.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/diegoclair/skills/pkg/atlassian/adf"
	"github.com/diegoclair/skills/pkg/atlassian/setup"
)

// ── test helpers ──────────────────────────────────────────────────────────────

// overrideConfigDirMain redirects os.UserConfigDir() in the main package tests.
func overrideConfigDirMain(t *testing.T, dir string) {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", dir)
	case "darwin":
		t.Setenv("HOME", dir)
	default:
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("HOME", dir)
	}
}

// overrideCacheDirMain redirects os.UserCacheDir() for the cache file.
func overrideCacheDirMain(t *testing.T, dir string) {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("LOCALAPPDATA", dir)
	case "darwin":
		t.Setenv("HOME", dir)
	default:
		t.Setenv("XDG_CACHE_HOME", dir)
	}
}

// writeTestConfig writes a config file with the given values.
func writeTestConfig(t *testing.T, dir string, cfg setup.Config) {
	t.Helper()
	cfgDir := filepath.Join(dir, "confluence-docs")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfgDir, "config")
	var sb strings.Builder
	if cfg.Cloud != "" {
		sb.WriteString("cloud=" + cfg.Cloud + "\n")
	}
	if cfg.SpaceID != "" {
		sb.WriteString("active_space_id=" + cfg.SpaceID + "\n")
	}
	if cfg.SpaceKey != "" {
		sb.WriteString("active_space_key=" + cfg.SpaceKey + "\n")
	}
	if cfg.SpaceName != "" {
		sb.WriteString("active_space_name=" + cfg.SpaceName + "\n")
	}
	if cfg.HomePageID != "" {
		sb.WriteString("active_home_page_id=" + cfg.HomePageID + "\n")
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatal(err)
	}
}

// writeTestCreds writes a minimal credentials file.
func writeTestCreds(t *testing.T, dir, email, token string) {
	t.Helper()
	cfgDir := filepath.Join(dir, "confluence-docs")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfgDir, "credentials")
	content := "email=" + email + "\ntoken=" + token + "\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

// mockRoundTripper implements http.RoundTripper for test HTTP clients.
type mockRoundTripper struct {
	statusCode int
	body       string
	netErr     error
	calls      int
}

func (m *mockRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	m.calls++
	if m.netErr != nil {
		return nil, m.netErr
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

// spaceListJSON builds the JSON body for a /api/v2/spaces response.
// Confluence v2 returns the human key in `currentActiveAlias`, not in `key`
// (which holds an internal hash for non-personal spaces). We mirror that
// shape here so the parser exercises the real-world path.
func spaceListJSON(spaces []adf.SpaceResult) string {
	results := make([]map[string]any, 0, len(spaces))
	for _, s := range spaces {
		results = append(results, map[string]any{
			"id":                 s.ID,
			"key":                s.Key, // internal hash in real API; here we use the human key — harmless because we read alias first
			"currentActiveAlias": s.Key, // the human key from tests
			"name":               s.Name,
			"homepageId":         s.HomepageID,
		})
	}
	b, _ := json.Marshal(map[string]any{"results": results})
	return string(b)
}

// runSpaceCmd runs `confluence-docs space <args...>` with env pointing to dir.
func runSpaceCmd(t *testing.T, dir string, rt *mockRoundTripper, args ...string) (stdout, stderr string, code int) {
	t.Helper()

	// Patch http.DefaultTransport so NewClient uses our mock.
	orig := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() { http.DefaultTransport = orig })

	t.Setenv("ATLASSIAN_API_TOKEN", "testtoken")
	t.Setenv("ATLASSIAN_EMAIL", "test@example.com")
	t.Setenv("ATLASSIAN_CLOUD", "testcloud")

	var outBuf, errBuf bytes.Buffer
	code, _ = run(append([]string{"space"}, args...), strings.NewReader(""), &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

// ── space list ────────────────────────────────────────────────────────────────

func TestSpaceList_TextOutput(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)
	writeTestCreds(t, dir, "test@example.com", "testtoken")
	writeTestConfig(t, dir, setup.Config{
		Cloud:      "testcloud",
		SpaceKey:   "eng",
		SpaceID:    "99001",
		SpaceName:  "Engineering",
		HomePageID: "10001",
	})

	spaces := []adf.SpaceResult{
		{ID: "99001", Key: "eng", Name: "Engineering", HomepageID: "10001"},
		{ID: "99002", Key: "mkt", Name: "Marketing", HomepageID: "20001"},
	}
	rt := &mockRoundTripper{statusCode: 200, body: spaceListJSON(spaces)}

	out, _, code := runSpaceCmd(t, dir, rt, "list")
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nout: %s", code, out)
	}
	// Active space should be marked with ✓.
	if !strings.Contains(out, "✓") {
		t.Errorf("expected ✓ marker for active space, got:\n%s", out)
	}
	if !strings.Contains(out, "eng") {
		t.Errorf("expected 'eng' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "mkt") {
		t.Errorf("expected 'mkt' in output, got:\n%s", out)
	}
}

func TestSpaceList_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)
	writeTestCreds(t, dir, "test@example.com", "testtoken")
	writeTestConfig(t, dir, setup.Config{
		Cloud:    "testcloud",
		SpaceKey: "eng",
	})

	spaces := []adf.SpaceResult{
		{ID: "99001", Key: "eng", Name: "Engineering", HomepageID: "10001"},
	}
	rt := &mockRoundTripper{statusCode: 200, body: spaceListJSON(spaces)}

	out, _, code := runSpaceCmd(t, dir, rt, "list", "--json")
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nout: %s", code, out)
	}

	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("invalid JSON output: %v\nout: %s", err, out)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0]["key"] != "eng" {
		t.Errorf("expected key 'eng', got %v", results[0]["key"])
	}
	if results[0]["active"] != true {
		t.Errorf("expected active=true for matching space, got %v", results[0]["active"])
	}
}

func TestSpaceList_UsesCache(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)
	writeTestCreds(t, dir, "test@example.com", "testtoken")
	writeTestConfig(t, dir, setup.Config{Cloud: "testcloud"})

	spaces := []adf.SpaceResult{
		{ID: "99001", Key: "eng", Name: "Engineering", HomepageID: "10001"},
	}
	rt := &mockRoundTripper{statusCode: 200, body: spaceListJSON(spaces)}

	// First call — hits API, populates cache.
	runSpaceCmd(t, dir, rt, "list")
	firstCalls := rt.calls

	// Second call — should use cache (no additional API calls).
	runSpaceCmd(t, dir, rt, "list")
	if rt.calls != firstCalls {
		t.Errorf("second call made %d API calls, expected 0 (should use cache)", rt.calls-firstCalls)
	}
}

func TestSpaceList_ForceRefresh(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)
	writeTestCreds(t, dir, "test@example.com", "testtoken")
	writeTestConfig(t, dir, setup.Config{Cloud: "testcloud"})

	spaces := []adf.SpaceResult{{ID: "99001", Key: "eng", Name: "Engineering"}}
	rt := &mockRoundTripper{statusCode: 200, body: spaceListJSON(spaces)}

	// Populate cache.
	runSpaceCmd(t, dir, rt, "list")
	callsAfterFirst := rt.calls

	// Force refresh — must call API again.
	runSpaceCmd(t, dir, rt, "list", "--refresh")
	if rt.calls == callsAfterFirst {
		t.Error("--refresh should have made an API call, but didn't")
	}
}

// ── space use ─────────────────────────────────────────────────────────────────

func TestSpaceUse_SwitchesActiveSpace(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)
	writeTestCreds(t, dir, "test@example.com", "testtoken")
	writeTestConfig(t, dir, setup.Config{
		Cloud:      "testcloud",
		SpaceKey:   "eng",
		SpaceID:    "99001",
		SpaceName:  "Engineering",
		HomePageID: "10001",
	})

	spaces := []adf.SpaceResult{
		{ID: "99001", Key: "eng", Name: "Engineering", HomepageID: "10001"},
		{ID: "99002", Key: "mkt", Name: "Marketing", HomepageID: "20001"},
	}
	// Two calls: list spaces + get homepage title for confirmation.
	pageMetaBody, _ := json.Marshal(map[string]any{
		"id":      "20001",
		"title":   "Marketing Home",
		"version": map[string]any{"number": 1},
	})
	responses := []struct {
		code int
		body string
	}{
		{200, spaceListJSON(spaces)}, // list spaces
		{200, string(pageMetaBody)},  // get home page title
	}
	callIdx := 0
	rt := &mockRoundTripper{}
	http.DefaultTransport = &multiRoundTripper{responses: responses, idx: &callIdx}
	t.Cleanup(func() { http.DefaultTransport = http.DefaultTransport })

	// Override transport using env-based approach.
	orig := http.DefaultTransport
	callIdx = 0
	http.DefaultTransport = &multiRoundTripper{responses: responses, idx: &callIdx}
	defer func() { http.DefaultTransport = orig }()

	t.Setenv("ATLASSIAN_API_TOKEN", "testtoken")
	t.Setenv("ATLASSIAN_EMAIL", "test@example.com")
	t.Setenv("ATLASSIAN_CLOUD", "testcloud")

	var outBuf, errBuf bytes.Buffer
	code, _ := run([]string{"space", "use", "mkt"}, strings.NewReader(""), &outBuf, &errBuf)
	out := outBuf.String()
	_ = rt

	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nout: %s\nerr: %s", code, out, errBuf.String())
	}
	if !strings.Contains(out, "Marketing") {
		t.Errorf("expected space name in output, got %q", out)
	}

	// Verify config was updated.
	cfg := adf.ReadActiveConfig()
	if cfg.SpaceKey != "mkt" {
		t.Errorf("SpaceKey: want %q, got %q", "mkt", cfg.SpaceKey)
	}
	if cfg.SpaceID != "99002" {
		t.Errorf("SpaceID: want %q, got %q", "99002", cfg.SpaceID)
	}
	if cfg.HomePageID != "20001" {
		t.Errorf("HomePageID: want %q, got %q", "20001", cfg.HomePageID)
	}
}

func TestSpaceUse_Idempotent(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)
	writeTestCreds(t, dir, "test@example.com", "testtoken")
	writeTestConfig(t, dir, setup.Config{
		Cloud:    "testcloud",
		SpaceKey: "eng",
	})

	spaces := []adf.SpaceResult{
		{ID: "99001", Key: "eng", Name: "Engineering", HomepageID: "10001"},
	}
	// Two calls expected per run: spaces list + page title fetch.
	spaceBody := spaceListJSON(spaces)
	pageBody, _ := json.Marshal(map[string]any{"id": "10001", "title": "Eng Home", "version": map[string]any{"number": 1}})

	rt1 := &mockRoundTripper{statusCode: 200}

	t.Setenv("ATLASSIAN_API_TOKEN", "testtoken")
	t.Setenv("ATLASSIAN_EMAIL", "test@example.com")
	t.Setenv("ATLASSIAN_CLOUD", "testcloud")

	// Run space use twice with the same key.
	for i := 0; i < 2; i++ {
		responses := []struct {
			code int
			body string
		}{
			{200, spaceBody},
			{200, string(pageBody)},
		}
		idx := 0
		orig := http.DefaultTransport
		http.DefaultTransport = &multiRoundTripper{responses: responses, idx: &idx}

		var outBuf, errBuf bytes.Buffer
		code, _ := run([]string{"space", "use", "eng"}, strings.NewReader(""), &outBuf, &errBuf)
		http.DefaultTransport = orig
		_ = rt1

		if code != exitOK {
			t.Fatalf("run %d: want exit 0, got %d\nout: %s\nerr: %s", i+1, code, outBuf.String(), errBuf.String())
		}
	}

	// After two runs, config should still be eng.
	cfg := adf.ReadActiveConfig()
	if cfg.SpaceKey != "eng" {
		t.Errorf("after idempotent use, SpaceKey = %q, want %q", cfg.SpaceKey, "eng")
	}
}

func TestSpaceUse_KeyNotFound(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)
	writeTestCreds(t, dir, "test@example.com", "testtoken")
	writeTestConfig(t, dir, setup.Config{Cloud: "testcloud"})

	spaces := []adf.SpaceResult{
		{ID: "99001", Key: "eng", Name: "Engineering", HomepageID: "10001"},
	}
	// space use will: check cache (miss) → fetch spaces → retry.
	// Two fetch calls since cache is empty and key not found triggers refresh.
	rt := &mockRoundTripper{statusCode: 200, body: spaceListJSON(spaces)}
	_, errOut, code := runSpaceCmd(t, dir, rt, "use", "nonexistent")
	if code == exitOK {
		t.Fatal("expected error for unknown key, got exit 0")
	}
	if !strings.Contains(errOut, "nonexistent") {
		t.Errorf("expected key name in error message, got %q", errOut)
	}
}

// ── space current ─────────────────────────────────────────────────────────────

func TestSpaceCurrent_TextOutput(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)
	writeTestCreds(t, dir, "test@example.com", "testtoken")
	writeTestConfig(t, dir, setup.Config{
		Cloud:      "testcloud",
		SpaceKey:   "eng",
		SpaceID:    "99001",
		SpaceName:  "Engineering Docs",
		HomePageID: "10001",
	})

	// Mock returns home page title.
	pageBody, _ := json.Marshal(map[string]any{
		"id": "10001", "title": "Eng Home",
		"version": map[string]any{"number": 1},
	})
	rt := &mockRoundTripper{statusCode: 200, body: string(pageBody)}

	out, _, code := runSpaceCmd(t, dir, rt, "current")
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nout: %s", code, out)
	}
	if !strings.Contains(out, "Engineering Docs") {
		t.Errorf("expected space name, got %q", out)
	}
	if !strings.Contains(out, "eng") {
		t.Errorf("expected space key, got %q", out)
	}
	if !strings.Contains(out, "10001") {
		t.Errorf("expected home page ID, got %q", out)
	}
}

func TestSpaceCurrent_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)
	writeTestCreds(t, dir, "test@example.com", "testtoken")
	writeTestConfig(t, dir, setup.Config{
		Cloud:      "testcloud",
		SpaceKey:   "eng",
		SpaceID:    "99001",
		SpaceName:  "Engineering",
		HomePageID: "10001",
	})

	pageBody, _ := json.Marshal(map[string]any{
		"id": "10001", "title": "Eng Home",
		"version": map[string]any{"number": 1},
	})
	rt := &mockRoundTripper{statusCode: 200, body: string(pageBody)}

	out, _, code := runSpaceCmd(t, dir, rt, "current", "--json")
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nout: %s", code, out)
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if result["key"] != "eng" {
		t.Errorf("key: want %q, got %q", "eng", result["key"])
	}
	if result["id"] != "99001" {
		t.Errorf("id: want %q, got %q", "99001", result["id"])
	}
}

func TestSpaceCurrent_NoConfig(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	overrideCacheDirMain(t, dir)
	// No config file written.
	writeTestCreds(t, dir, "test@example.com", "testtoken")

	rt := &mockRoundTripper{statusCode: 200}
	_, errOut, code := runSpaceCmd(t, dir, rt, "current")
	if code == exitOK {
		t.Fatal("expected error when no space configured, got exit 0")
	}
	if !strings.Contains(errOut, "no active space") {
		t.Errorf("expected 'no active space' error, got %q", errOut)
	}
}

// ── multiRoundTripper: returns different responses per call ───────────────────

type multiRoundTripper struct {
	responses []struct {
		code int
		body string
	}
	idx *int
}

func (m *multiRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	i := *m.idx
	if i >= len(m.responses) {
		i = len(m.responses) - 1
	}
	*m.idx++
	r := m.responses[i]
	return &http.Response{
		StatusCode: r.code,
		Body:       io.NopCloser(strings.NewReader(r.body)),
	}, nil
}
