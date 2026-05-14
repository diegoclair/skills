// cmd_page_reorder_test.go — tests for `confluence-docs page reorder`.
package main

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/lybel-app/skills/confluence-docs/cli/setup"
)

// runReorderCmd is a thin helper that sets up env/config and calls
// run([]string{"page","reorder",...}) with the given extra args.
func runReorderCmd(t *testing.T, dir string, rt http.RoundTripper, args ...string) (stdout, stderr string, code int) {
	t.Helper()

	orig := http.DefaultTransport
	if rt != nil {
		http.DefaultTransport = rt
	}
	t.Cleanup(func() { http.DefaultTransport = orig })

	t.Setenv("ATLASSIAN_API_TOKEN", "testtoken")
	t.Setenv("ATLASSIAN_EMAIL", "test@example.com")
	t.Setenv("ATLASSIAN_CLOUD", "testcloud")

	var outBuf, errBuf bytes.Buffer
	code, _ = run(append([]string{"page", "reorder"}, args...), strings.NewReader(""), &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

// setupReorderEnv creates a temp dir with creds + config and wires up
// overrideConfigDirMain / overrideCacheDirMain.
func setupReorderEnv(t *testing.T) string {
	t.Helper()
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
	return dir
}

// ── flag-validation tests ─────────────────────────────────────────────────────

func TestPageReorder_MissingPageID(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir
	_, errOut, code := runReorderCmd(t, dir, nil, "--before", "222")
	if code == exitOK {
		t.Fatal("expected non-zero exit for missing --page-id, got 0")
	}
	if !strings.Contains(errOut, "--page-id") {
		t.Errorf("expected '--page-id' in error output, got: %q", errOut)
	}
}

func TestPageReorder_NoPositionFlag(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir
	_, errOut, code := runReorderCmd(t, dir, nil, "--page-id", "111")
	if code == exitOK {
		t.Fatal("expected non-zero exit for missing position flag, got 0")
	}
	if !strings.Contains(errOut, "--before") || !strings.Contains(errOut, "--after") {
		t.Errorf("expected position flag names in error output, got: %q", errOut)
	}
}

func TestPageReorder_MultiplePositionFlags(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir
	_, errOut, code := runReorderCmd(t, dir, nil, "--page-id", "111", "--before", "222", "--after", "333")
	if code == exitOK {
		t.Fatal("expected non-zero exit for multiple position flags, got 0")
	}
	if !strings.Contains(errOut, "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error output, got: %q", errOut)
	}
}

func TestPageReorder_PageIDWithoutValue(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir
	// --page-id is the last arg with no following value
	_, errOut, code := runReorderCmd(t, dir, nil, "--page-id")
	if code == exitOK {
		t.Fatal("expected non-zero exit for --page-id without value, got 0")
	}
	if !strings.Contains(errOut, "--page-id") {
		t.Errorf("expected '--page-id' in error output, got: %q", errOut)
	}
}

func TestPageReorder_BeforeWithoutValue(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir
	_, errOut, code := runReorderCmd(t, dir, nil, "--page-id", "111", "--before")
	if code == exitOK {
		t.Fatal("expected non-zero exit for --before without value, got 0")
	}
	if !strings.Contains(errOut, "--before") {
		t.Errorf("expected '--before' in error output, got: %q", errOut)
	}
}

func TestPageReorder_AllThreePositionFlagsExclusive(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir
	_, errOut, code := runReorderCmd(t, dir, nil,
		"--page-id", "111",
		"--before", "222",
		"--after", "333",
		"--append-to", "444",
	)
	if code == exitOK {
		t.Fatal("expected non-zero exit for all three position flags, got 0")
	}
	if !strings.Contains(errOut, "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error output, got: %q", errOut)
	}
}

// ── help flag ─────────────────────────────────────────────────────────────────

func TestPageReorder_Help(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir
	out, _, code := runReorderCmd(t, dir, nil, "--help")
	if code != exitOK {
		t.Fatalf("expected exit 0 for --help, got %d", code)
	}
	if !strings.Contains(out, "--page-id") {
		t.Errorf("expected '--page-id' in help output, got: %q", out)
	}
	if !strings.Contains(out, "--before") {
		t.Errorf("expected '--before' in help output, got: %q", out)
	}
	if !strings.Contains(out, "--after") {
		t.Errorf("expected '--after' in help output, got: %q", out)
	}
	if !strings.Contains(out, "--append-to") {
		t.Errorf("expected '--append-to' in help output, got: %q", out)
	}
}

func TestPageReorder_ShortHelp(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir
	out, _, code := runReorderCmd(t, dir, nil, "-h")
	if code != exitOK {
		t.Fatalf("expected exit 0 for -h, got %d", code)
	}
	if !strings.Contains(out, "reorder") {
		t.Errorf("expected 'reorder' in help output, got: %q", out)
	}
}

// ── happy path: --before ──────────────────────────────────────────────────────

func TestPageReorder_Before_HappyPath(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir

	// refreshHomeCacheAfterWrite will also call the transport; we need at least
	// 2 responses: reorder (204) + home cache refresh (200 page meta).
	responses := []struct {
		code int
		body string
	}{
		{204, ""},
		{200, `{"id":"10001","title":"Eng Home","version":{"number":1}}`},
	}
	idx := 0
	var capturedURL string
	mrt := &multiRoundTripper{responses: responses, idx: &idx}
	captureRT := &urlCapturingMultiRT{inner: mrt, capturedURL: &capturedURL, captureAt: 0}

	out, errOut, code := runReorderCmd(t, dir, captureRT, "--page-id", "111", "--before", "222")
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	// URL must contain /move/before/222
	if !strings.Contains(capturedURL, "/move/before/222") {
		t.Errorf("expected URL to contain '/move/before/222', got: %q", capturedURL)
	}
	if !strings.Contains(out, `"position":"before"`) {
		t.Errorf("expected position in JSON output, got: %q", out)
	}
	if !strings.Contains(out, `"targetId":"222"`) {
		t.Errorf("expected targetId in JSON output, got: %q", out)
	}
}

// ── happy path: --after ───────────────────────────────────────────────────────

func TestPageReorder_After_HappyPath(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir

	responses := []struct {
		code int
		body string
	}{
		{204, ""},
		{200, `{"id":"10001","title":"Eng Home","version":{"number":1}}`},
	}
	idx := 0
	var capturedURL string
	mrt := &multiRoundTripper{responses: responses, idx: &idx}
	captureRT := &urlCapturingMultiRT{inner: mrt, capturedURL: &capturedURL, captureAt: 0}

	out, errOut, code := runReorderCmd(t, dir, captureRT, "--page-id", "111", "--after", "333")
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	if !strings.Contains(capturedURL, "/move/after/333") {
		t.Errorf("expected URL to contain '/move/after/333', got: %q", capturedURL)
	}
	if !strings.Contains(out, `"position":"after"`) {
		t.Errorf("expected position in JSON output, got: %q", out)
	}
}

// ── happy path: --append-to ───────────────────────────────────────────────────

func TestPageReorder_AppendTo_HappyPath(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir

	responses := []struct {
		code int
		body string
	}{
		{204, ""},
		{200, `{"id":"10001","title":"Eng Home","version":{"number":1}}`},
	}
	idx := 0
	var capturedURL string
	mrt := &multiRoundTripper{responses: responses, idx: &idx}
	captureRT := &urlCapturingMultiRT{inner: mrt, capturedURL: &capturedURL, captureAt: 0}

	out, errOut, code := runReorderCmd(t, dir, captureRT, "--page-id", "111", "--append-to", "444")
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	// --append-to maps to position "append" in the v1 endpoint
	if !strings.Contains(capturedURL, "/move/append/444") {
		t.Errorf("expected URL to contain '/move/append/444', got: %q", capturedURL)
	}
	if !strings.Contains(out, `"position":"append"`) {
		t.Errorf("expected position in JSON output, got: %q", out)
	}
}

// ── dry-run: no HTTP call made ────────────────────────────────────────────────

func TestPageReorder_DryRun_NoHTTPCall(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir

	// --dry-run prints the intended action as JSON and exits 0 without
	// making any HTTP call. This is what an agent runs to preview the
	// reorder for the user to confirm before the real call.
	rt := &mockRoundTripper{statusCode: 200}
	out, _, code := runReorderCmd(t, dir, rt, "--page-id", "111", "--before", "222", "--dry-run")
	if code != exitOK {
		t.Fatalf("expected exit 0 for --dry-run, got %d", code)
	}
	if rt.calls != 0 {
		t.Errorf("expected zero HTTP calls in dry-run, got %d", rt.calls)
	}
	if !strings.Contains(out, `"status":"dry-run"`) {
		t.Errorf("expected dry-run status marker in output, got: %q", out)
	}
	if !strings.Contains(out, `"pageId":"111"`) || !strings.Contains(out, `"targetId":"222"`) || !strings.Contains(out, `"position":"before"`) {
		t.Errorf("expected pageId/targetId/position in dry-run output, got: %q", out)
	}
}

// ── HTTP 5xx error → CLI exits non-zero ──────────────────────────────────────

func TestPageReorder_HTTP5xx_ExitsNonZero(t *testing.T) {
	dir := setupReorderEnv(t)
	_ = dir

	rt := &mockRoundTripper{statusCode: 500, body: `{"message":"internal server error"}`}
	_, errOut, code := runReorderCmd(t, dir, rt, "--page-id", "111", "--before", "222")
	if code == exitOK {
		t.Fatal("expected non-zero exit for 5xx HTTP response, got 0")
	}
	if !strings.Contains(errOut, "error") {
		t.Errorf("expected 'error' in stderr, got: %q", errOut)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// urlCapturingMultiRT wraps a multiRoundTripper and captures the request URL
// at the given call index (0 = first call).
type urlCapturingMultiRT struct {
	inner       *multiRoundTripper
	capturedURL *string
	captureAt   int
	calls       int
}

func (u *urlCapturingMultiRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if u.calls == u.captureAt {
		*u.capturedURL = req.URL.String()
	}
	u.calls++
	return u.inner.RoundTrip(req)
}
