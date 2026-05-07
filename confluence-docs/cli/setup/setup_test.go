package setup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ── mock HTTP client ──────────────────────────────────────────────────────────

// mockHTTPClient returns a fixed response for every request.
type mockHTTPClient struct {
	statusCode  int
	body        string
	networkErr  error
}

func (m *mockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	if m.networkErr != nil {
		return nil, m.networkErr
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

func okUserBody(displayName, accountID string) string {
	b, _ := json.Marshal(map[string]string{
		"displayName": displayName,
		"accountId":   accountID,
	})
	return string(b)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// writeTempCreds writes a credentials file inside dir and returns its path.
func writeTempCreds(t *testing.T, dir, email, token string) string {
	t.Helper()
	cfgDir := filepath.Join(dir, "confluence-docs")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(cfgDir, "credentials")
	content := fmt.Sprintf("email=%s\ntoken=%s\n", email, token)
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

// overrideConfigDir temporarily redirects os.UserConfigDir() by setting the
// appropriate env var for the current OS. Returns a cleanup function.
func overrideConfigDir(t *testing.T, dir string) {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", dir)
	case "darwin":
		// UserConfigDir on macOS uses $HOME/Library/Application Support; the
		// simplest override is to set HOME so the fallback path is also inside dir.
		t.Setenv("HOME", dir)
	default: // linux and others
		t.Setenv("XDG_CONFIG_HOME", dir)
		// Also override HOME so the legacy ~/.config/confluence-docs path resolves
		// inside the temp dir — otherwise tests that expect "no creds" can
		// pick up the developer's real legacy credentials file.
		t.Setenv("HOME", dir)
	}
}

func runSetup(t *testing.T, client httpClient, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code, _ = runWithClient(args, strings.NewReader(stdin), &outBuf, &errBuf, client)
	return outBuf.String(), errBuf.String(), code
}

// ── --print-config-path ───────────────────────────────────────────────────────

func TestPrintConfigPath(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	out, _, code := runSetup(t, nil, "", "--print-config-path")
	if code != ExitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	got := strings.TrimSpace(out)
	if got == "" {
		t.Fatal("expected a non-empty path")
	}
	// Must be an absolute path.
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
	// Must end with the canonical suffix.
	if !strings.HasSuffix(got, filepath.Join("confluence-docs", "credentials")) {
		t.Errorf("path %q should end with confluence-docs/credentials", got)
	}
}

// ── --print-config-format ─────────────────────────────────────────────────────

func TestPrintConfigFormat(t *testing.T) {
	out, _, code := runSetup(t, nil, "", "--print-config-format")
	if code != ExitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(out, "email=") {
		t.Errorf("expected 'email=' in output, got %q", out)
	}
	if !strings.Contains(out, "token=") {
		t.Errorf("expected 'token=' in output, got %q", out)
	}
}

// ── --check exit codes ────────────────────────────────────────────────────────

func TestCheck_NoCreds(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)
	// No credentials file created.

	_, errOut, code := runSetup(t, &mockHTTPClient{statusCode: 200}, "", "--check")
	if code != ExitNoCreds {
		t.Fatalf("want exit %d (no creds), got %d", ExitNoCreds, code)
	}
	if !strings.Contains(errOut, "no credentials configured") {
		t.Errorf("expected 'no credentials configured', got %q", errOut)
	}
}

func TestCheck_ValidCreds(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)
	writeTempCreds(t, dir, "user@example.com", "tok123")

	mock := &mockHTTPClient{
		statusCode: 200,
		body:       okUserBody("Alice", "acc-001"),
	}
	out, _, code := runSetup(t, mock, "", "--check")
	if code != ExitOK {
		t.Fatalf("want exit 0 (valid), got %d", code)
	}
	if !strings.Contains(out, "credentials valid") {
		t.Errorf("expected 'credentials valid' in stdout, got %q", out)
	}
}

func TestCheck_InvalidAuth(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)
	writeTempCreds(t, dir, "user@example.com", "bad-token")

	mock := &mockHTTPClient{statusCode: http.StatusUnauthorized}
	_, errOut, code := runSetup(t, mock, "", "--check")
	if code != ExitInvalidAuth {
		t.Fatalf("want exit %d (invalid auth), got %d", ExitInvalidAuth, code)
	}
	if !strings.Contains(errOut, "invalid") {
		t.Errorf("expected 'invalid' in stderr, got %q", errOut)
	}
}

func TestCheck_NetworkError(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)
	writeTempCreds(t, dir, "user@example.com", "tok123")

	mock := &mockHTTPClient{networkErr: fmt.Errorf("dial tcp: connection refused")}
	_, errOut, code := runSetup(t, mock, "", "--check")
	if code != ExitNetworkErr {
		t.Fatalf("want exit %d (network error), got %d", ExitNetworkErr, code)
	}
	if !strings.Contains(errOut, "network error") {
		t.Errorf("expected 'network error' in stderr, got %q", errOut)
	}
}

// ── legacy migration warning ──────────────────────────────────────────────────

func TestCheck_LegacyCredsWarning(t *testing.T) {
	if runtime.GOOS == "linux" {
		// On Linux, XDG_CONFIG_HOME and ~/.config are the same path when
		// XDG_CONFIG_HOME is not set, so the legacy path == new path.
		// Override HOME to a temp dir; legacy path will differ from new XDG path.
	}

	// Create a temporary HOME with legacy credentials.
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// On macOS/Windows, UserConfigDir differs from ~/.config so legacy ≠ new.
	// On Linux, force XDG_CONFIG_HOME to a *different* dir to simulate divergence.
	xdgDir := t.TempDir()
	if runtime.GOOS == "linux" {
		t.Setenv("XDG_CONFIG_HOME", xdgDir)
	}

	// Write legacy creds at ~/.config/confluence-docs/credentials.
	legacyDir := filepath.Join(homeDir, ".config", "confluence-docs")
	if err := os.MkdirAll(legacyDir, 0700); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(legacyDir, "credentials")
	if err := os.WriteFile(legacyPath, []byte("email=legacy@example.com\ntoken=legacytok\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Determine new path; skip test if they're still the same (e.g. macOS with
	// HOME pointing into the same structure).
	newPath, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if newPath == legacyPath {
		t.Skip("new config path == legacy path on this OS; migration warning not applicable")
	}

	mock := &mockHTTPClient{
		statusCode: 200,
		body:       okUserBody("Legacy User", "acc-legacy"),
	}

	_, errOut, _ := runSetup(t, mock, "", "--check")
	if !strings.Contains(errOut, "legacy path") {
		t.Errorf("expected legacy path warning in stderr, got %q", errOut)
	}
}

// ── cross-platform path resolution ───────────────────────────────────────────

func TestConfigPath_IsAbsolute(t *testing.T) {
	p, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error: %v", err)
	}
	if !filepath.IsAbs(p) {
		t.Errorf("ConfigPath() = %q, want absolute path", p)
	}
}

func TestConfigPath_ContainsLybelDocs(t *testing.T) {
	p, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error: %v", err)
	}
	if !strings.Contains(p, "confluence-docs") {
		t.Errorf("ConfigPath() = %q, want 'confluence-docs' in path", p)
	}
	if !strings.HasSuffix(p, "credentials") {
		t.Errorf("ConfigPath() = %q, want to end with 'credentials'", p)
	}
}

func TestConfigPath_OSSpecific(t *testing.T) {
	dir := t.TempDir()

	switch runtime.GOOS {
	case "linux":
		t.Setenv("XDG_CONFIG_HOME", dir)
		p, err := ConfigPath()
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(dir, "confluence-docs", "credentials")
		if p != want {
			t.Errorf("Linux: got %q, want %q", p, want)
		}

	case "darwin":
		// On macOS UserConfigDir() = $HOME/Library/Application Support
		t.Setenv("HOME", dir)
		p, err := ConfigPath()
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(dir, "Library", "Application Support", "confluence-docs", "credentials")
		if p != want {
			t.Errorf("macOS: got %q, want %q", p, want)
		}

	case "windows":
		t.Setenv("APPDATA", dir)
		p, err := ConfigPath()
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(dir, "confluence-docs", "credentials")
		if p != want {
			t.Errorf("Windows: got %q, want %q", p, want)
		}

	default:
		t.Skipf("no OS-specific assertion for GOOS=%s", runtime.GOOS)
	}
}

// ── non-interactive mode ──────────────────────────────────────────────────────

func TestNonInteractive_ValidCreds(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	mock := &mockHTTPClient{
		statusCode: 200,
		body:       okUserBody("Bob", "acc-bob"),
	}
	out, errOut, code := runSetup(t, mock, "",
		"--email", "bob@example.com", "--token", "tok-bob")
	if code != ExitOK {
		t.Fatalf("want exit 0, got %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	if !strings.Contains(out, "credentials saved") {
		t.Errorf("expected 'credentials saved' in stdout, got %q", out)
	}

	// Verify the file was actually written.
	credPath, _ := ConfigPath()
	data, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("credentials file not written: %v", err)
	}
	if !strings.Contains(string(data), "bob@example.com") {
		t.Errorf("credentials file missing email: %s", data)
	}
}

func TestNonInteractive_InvalidToken(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	mock := &mockHTTPClient{statusCode: http.StatusUnauthorized}
	_, errOut, code := runSetup(t, mock, "",
		"--email", "bob@example.com", "--token", "bad")
	if code != ExitInvalidAuth {
		t.Fatalf("want exit %d, got %d\nstderr: %s", ExitInvalidAuth, code, errOut)
	}
	if !strings.Contains(errOut, "invalid credentials") {
		t.Errorf("expected 'invalid credentials' in stderr, got %q", errOut)
	}
}

// ── WriteCreds file permissions ───────────────────────────────────────────────

func TestWriteCreds_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission check not applicable on Windows")
	}
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	if err := WriteCreds("u@example.com", "tok"); err != nil {
		t.Fatalf("WriteCreds: %v", err)
	}
	p, _ := ConfigPath()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("want 0600, got %04o", perm)
	}
}

// ── parseCredsData ────────────────────────────────────────────────────────────

func TestParseCredsData(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		wantEmail  string
		wantToken  string
	}{
		{"basic", "email=a@b.com\ntoken=tok\n", "a@b.com", "tok"},
		{"spaces", "  email = a@b.com \n  token = tok \n", "a@b.com", "tok"},
		{"comment", "# comment\nemail=a@b.com\ntoken=tok\n", "a@b.com", "tok"},
		{"empty lines", "\nemail=a@b.com\n\ntoken=tok\n", "a@b.com", "tok"},
		{"token with equals", "email=a@b.com\ntoken=ATAT=abc==\n", "a@b.com", "ATAT=abc=="},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			email, token, err := parseCredsData([]byte(tc.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if email != tc.wantEmail {
				t.Errorf("email: got %q, want %q", email, tc.wantEmail)
			}
			if token != tc.wantToken {
				t.Errorf("token: got %q, want %q", token, tc.wantToken)
			}
		})
	}
}
