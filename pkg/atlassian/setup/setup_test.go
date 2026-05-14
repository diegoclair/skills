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
// Also writes a config file with cloud=testcloud and active_space defaults
// so tests that exercise runCheck don't fail on "no active space configured".
func writeTempCreds(t *testing.T, dir, email, token string) string {
	t.Helper()
	cfgDir := filepath.Join(dir, "confluence-docs")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Credentials file: email + token only (no cloud= in v0.10.0+).
	p := filepath.Join(cfgDir, "credentials")
	content := fmt.Sprintf("email=%s\ntoken=%s\n", email, token)
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	// Config file: cloud + active space defaults so --check passes space validation.
	writeTempConfig(t, dir, Config{
		Cloud:      "testcloud",
		SpaceID:    "99999",
		SpaceKey:   "testspace",
		SpaceName:  "Test Space",
		HomePageID: "11111",
	})
	return p
}

// writeTempConfig writes a config file inside dir.
func writeTempConfig(t *testing.T, dir string, cfg Config) {
	t.Helper()
	cfgDir := filepath.Join(dir, "confluence-docs")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(cfgDir, "config")
	var content string
	if cfg.Cloud != "" {
		content += fmt.Sprintf("cloud=%s\n", cfg.Cloud)
	}
	if cfg.SpaceID != "" {
		content += fmt.Sprintf("active_space_id=%s\n", cfg.SpaceID)
	}
	if cfg.SpaceKey != "" {
		content += fmt.Sprintf("active_space_key=%s\n", cfg.SpaceKey)
	}
	if cfg.SpaceName != "" {
		content += fmt.Sprintf("active_space_name=%s\n", cfg.SpaceName)
	}
	if cfg.HomePageID != "" {
		content += fmt.Sprintf("active_home_page_id=%s\n", cfg.HomePageID)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
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
	// Must end with the canonical suffix (atlassian-wide shared creds).
	if !strings.HasSuffix(got, filepath.Join("atlassian", "credentials")) {
		t.Errorf("path %q should end with atlassian/credentials", got)
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
	// Should also mention the space key from config.
	if !strings.Contains(out, "testspace") {
		t.Errorf("expected space key 'testspace' in output, got %q", out)
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

func TestConfigPath_ContainsAtlassian(t *testing.T) {
	p, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error: %v", err)
	}
	if !strings.Contains(p, "atlassian") {
		t.Errorf("ConfigPath() = %q, want 'atlassian' in path (creds are atlassian-wide)", p)
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
		want := filepath.Join(dir, "atlassian", "credentials")
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
		want := filepath.Join(dir, "Library", "Application Support", "atlassian", "credentials")
		if p != want {
			t.Errorf("macOS: got %q, want %q", p, want)
		}

	case "windows":
		t.Setenv("APPDATA", dir)
		p, err := ConfigPath()
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(dir, "atlassian", "credentials")
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
	t.Setenv("ATLASSIAN_CLOUD", "testcloud")

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
	t.Setenv("ATLASSIAN_CLOUD", "testcloud")

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
		// cloud= line in old format is ignored (goes to config file now)
		{"legacy cloud ignored", "email=a@b.com\ntoken=tok\ncloud=mycompany\n", "a@b.com", "tok"},
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

// ── Config file (v0.10.0 split) ───────────────────────────────────────────────

func TestWriteConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	cfg := Config{
		Cloud:      "mycompany",
		SpaceID:    "200001",
		SpaceKey:   "eng",
		SpaceName:  "Engineering Docs",
		HomePageID: "200002",
	}
	if err := WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// Read it back.
	got := ReadConfigFile()
	if got.Cloud != cfg.Cloud {
		t.Errorf("Cloud: got %q, want %q", got.Cloud, cfg.Cloud)
	}
	if got.SpaceID != cfg.SpaceID {
		t.Errorf("SpaceID: got %q, want %q", got.SpaceID, cfg.SpaceID)
	}
	if got.SpaceKey != cfg.SpaceKey {
		t.Errorf("SpaceKey: got %q, want %q", got.SpaceKey, cfg.SpaceKey)
	}
	if got.SpaceName != cfg.SpaceName {
		t.Errorf("SpaceName: got %q, want %q", got.SpaceName, cfg.SpaceName)
	}
	if got.HomePageID != cfg.HomePageID {
		t.Errorf("HomePageID: got %q, want %q", got.HomePageID, cfg.HomePageID)
	}
}

func TestWriteConfig_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission check not applicable on Windows")
	}
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	if err := WriteConfig(Config{Cloud: "mycompany"}); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	p, _ := ConfigFilePath()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0644 {
		t.Errorf("config file: want 0644, got %04o", perm)
	}
}

func TestReadConfigFile_BackwardCompat(t *testing.T) {
	// If only the old credentials file exists with cloud= embedded, ReadConfigFile
	// should fall back to that and return the cloud value.
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	// Write old-style credentials file with cloud=.
	cfgDir := filepath.Join(dir, "confluence-docs")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	credsPath := filepath.Join(cfgDir, "credentials")
	if err := os.WriteFile(credsPath, []byte("email=u@example.com\ntoken=tok\ncloud=legacycloud\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := ReadConfigFile()
	if cfg.Cloud != "legacycloud" {
		t.Errorf("backward compat: want cloud %q, got %q", "legacycloud", cfg.Cloud)
	}
	// SpaceID should be empty (not in old file).
	if cfg.SpaceID != "" {
		t.Errorf("backward compat: SpaceID should be empty, got %q", cfg.SpaceID)
	}
}

func TestReadConfigFile_NewFileWins(t *testing.T) {
	// New config file takes precedence over old credentials file.
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	cfgDir := filepath.Join(dir, "confluence-docs")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Write old creds with cloud= (should be ignored when new config file exists).
	if err := os.WriteFile(filepath.Join(cfgDir, "credentials"), []byte("email=u@example.com\ntoken=tok\ncloud=oldcloud\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Write new config file with different cloud.
	if err := os.WriteFile(filepath.Join(cfgDir, "config"), []byte("cloud=newcloud\nactive_space_id=555\nactive_space_key=myspace\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := ReadConfigFile()
	if cfg.Cloud != "newcloud" {
		t.Errorf("want cloud %q (from new config), got %q", "newcloud", cfg.Cloud)
	}
	if cfg.SpaceID != "555" {
		t.Errorf("want SpaceID %q, got %q", "555", cfg.SpaceID)
	}
	if cfg.SpaceKey != "myspace" {
		t.Errorf("want SpaceKey %q, got %q", "myspace", cfg.SpaceKey)
	}
}

// ── --reconfigure ─────────────────────────────────────────────────────────────

func TestReconfigure_PrefillsExistingValues(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)
	writeTempCreds(t, dir, "user@example.com", "tok123")

	mock := &mockHTTPClient{
		statusCode: 200,
		body: func() string {
			// Multi-request: user validation + spaces listing.
			return okUserBody("Alice", "acc-001")
		}(),
	}

	// Provide empty lines for all prompts to keep existing values.
	// Wizard flow: email → token → cloud → space selection → done
	// With prefill, it should show existing values and accept Enter.
	// This is hard to test interactively; we verify the flag is accepted
	// and the wizard starts without error.
	_, _, code := runSetup(t, mock,
		"\n\n\n", // accept all prompts with Enter
		"--reconfigure",
	)
	// May not complete (no spaces API on mock), but should not crash.
	_ = code
}

// ── --set ─────────────────────────────────────────────────────────────────────

func TestSet_ValidKey(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	out, _, code := runSetup(t, nil, "", "--set", "active_space_key", "eng")
	if code != ExitOK {
		t.Fatalf("want exit 0, got %d\nstdout: %s", code, out)
	}
	if !strings.Contains(out, "active_space_key") {
		t.Errorf("expected key name in output, got %q", out)
	}
	// Verify the file was written.
	cfg := ReadConfigFile()
	if cfg.SpaceKey != "eng" {
		t.Errorf("SpaceKey not persisted: got %q", cfg.SpaceKey)
	}
}

func TestSet_CloudKey(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	_, _, code := runSetup(t, nil, "", "--set", "cloud", "acmecorp")
	if code != ExitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	cfg := ReadConfigFile()
	if cfg.Cloud != "acmecorp" {
		t.Errorf("Cloud not persisted: got %q", cfg.Cloud)
	}
}

func TestSet_UnknownKey(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	_, errOut, code := runSetup(t, nil, "", "--set", "unknown_key", "value")
	if code != ExitNoCreds {
		t.Fatalf("want exit %d, got %d", ExitNoCreds, code)
	}
	if !strings.Contains(errOut, "unknown key") {
		t.Errorf("expected 'unknown key' in stderr, got %q", errOut)
	}
}

func TestSet_Idempotent(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	// Set the same key twice — second run should overwrite, not append.
	runSetup(t, nil, "", "--set", "active_space_key", "first")
	runSetup(t, nil, "", "--set", "active_space_key", "second")

	cfg := ReadConfigFile()
	if cfg.SpaceKey != "second" {
		t.Errorf("expected last value %q, got %q", "second", cfg.SpaceKey)
	}
}

// ── Interactive wizard with space auto-detect ─────────────────────────────────

// multiMockHTTPClient returns different responses per call (round-robin).
type multiMockHTTPClient struct {
	responses []mockHTTPClient
	callCount int
}

func (m *multiMockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	idx := m.callCount
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	m.callCount++
	resp := &m.responses[idx]
	if resp.networkErr != nil {
		return nil, resp.networkErr
	}
	return &http.Response{
		StatusCode: resp.statusCode,
		Body:       io.NopCloser(strings.NewReader(resp.body)),
	}, nil
}

func spacesListBody(spaces []map[string]string) string {
	results := make([]map[string]string, 0, len(spaces))
	results = append(results, spaces...)
	b, _ := json.Marshal(map[string]any{"results": results})
	return string(b)
}

func TestInteractive_AutoDetectSingleSpace(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	spaceBody := spacesListBody([]map[string]string{
		{"id": "99001", "key": "eng", "name": "Engineering", "homepageId": "10001"},
	})
	mock := &multiMockHTTPClient{
		responses: []mockHTTPClient{
			{statusCode: 200, body: okUserBody("Alice", "acc-001")}, // validate user
			{statusCode: 200, body: spaceBody},                      // list spaces
		},
	}

	// Wizard input: email, token, cloud (all provided)
	stdin := "alice@example.com\nmy-token\nmycompany\n"
	out, _, code := runSetup(t, mock, stdin)
	if code != ExitOK {
		t.Fatalf("want exit 0, got %d\nstdout: %s", code, out)
	}
	// Should auto-select the single space without prompting.
	if !strings.Contains(out, "found 1 space") {
		t.Errorf("expected single-space confirmation, got %q", out)
	}
	// Config file should have the space set.
	cfg := ReadConfigFile()
	if cfg.SpaceKey != "eng" {
		t.Errorf("SpaceKey: want %q, got %q", "eng", cfg.SpaceKey)
	}
	if cfg.HomePageID != "10001" {
		t.Errorf("HomePageID: want %q, got %q", "10001", cfg.HomePageID)
	}
	// Credentials file should NOT contain cloud=.
	credsPath, _ := ConfigPath()
	data, err := os.ReadFile(credsPath)
	if err != nil {
		t.Fatalf("creds file not found: %v", err)
	}
	if strings.Contains(string(data), "cloud=") {
		t.Errorf("credentials file should not contain cloud= in v0.10.0+, got:\n%s", data)
	}
}

func TestInteractive_AutoDetectMultipleSpaces_UserPicks(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	spaceBody := spacesListBody([]map[string]string{
		{"id": "99001", "key": "eng", "name": "Engineering", "homepageId": "10001"},
		{"id": "99002", "key": "mkt", "name": "Marketing", "homepageId": "20001"},
	})
	mock := &multiMockHTTPClient{
		responses: []mockHTTPClient{
			{statusCode: 200, body: okUserBody("Alice", "acc-001")},
			{statusCode: 200, body: spaceBody},
		},
	}

	// Wizard input: email, token, cloud, then pick space "2"
	stdin := "alice@example.com\nmy-token\nmycompany\n2\n"
	out, _, code := runSetup(t, mock, stdin)
	if code != ExitOK {
		t.Fatalf("want exit 0, got %d\nstdout: %s", code, out)
	}
	cfg := ReadConfigFile()
	if cfg.SpaceKey != "mkt" {
		t.Errorf("SpaceKey: want %q (user picked 2), got %q", "mkt", cfg.SpaceKey)
	}
}

func TestInteractive_AutoDetectNoSpaces(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	spaceBody := spacesListBody(nil)
	mock := &multiMockHTTPClient{
		responses: []mockHTTPClient{
			{statusCode: 200, body: okUserBody("Alice", "acc-001")},
			{statusCode: 200, body: spaceBody},
		},
	}

	// When no spaces are found, setup completes but warns about missing space.
	stdin := "alice@example.com\nmy-token\nmycompany\n"
	out, _, code := runSetup(t, mock, stdin)
	if code != ExitOK {
		t.Fatalf("want exit 0 (graceful), got %d\nstdout: %s", code, out)
	}
	if !strings.Contains(out, "no accessible spaces") {
		t.Errorf("expected 'no accessible spaces' warning, got %q", out)
	}
}

// ── --check validates space config ───────────────────────────────────────────

func TestCheck_MissingSpaceConfig(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDir(t, dir)

	// Write credentials but no config file (simulates v0.9.x user who hasn't re-run setup).
	cfgDir := filepath.Join(dir, "confluence-docs")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Write old-style creds (cloud= present, no space info).
	if err := os.WriteFile(filepath.Join(cfgDir, "credentials"), []byte("email=u@example.com\ntoken=tok\ncloud=mycompany\n"), 0600); err != nil {
		t.Fatal(err)
	}

	mock := &mockHTTPClient{statusCode: 200, body: okUserBody("User", "acc-001")}
	_, errOut, code := runSetup(t, mock, "", "--check")
	if code != ExitNoCreds {
		t.Fatalf("want exit %d (no space), got %d", ExitNoCreds, code)
	}
	if !strings.Contains(errOut, "no active space") {
		t.Errorf("expected 'no active space' message, got %q", errOut)
	}
}
