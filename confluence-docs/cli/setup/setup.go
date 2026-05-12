// Package setup implements the `confluence-docs setup` sub-command: an interactive
// wizard that guides the user through obtaining and storing Atlassian API
// credentials, plus --check / --print-config-path / --print-config-format
// informational modes.
package setup

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// NOTE on masked input: golang.org/x/term would give us proper masked input
// (password prompts that hide characters). We deliberately avoid adding new
// dependencies, so we fall back to plain readline. If masked input is needed
// in the future, add golang.org/x/term and replace the readLine call in
// runInteractive with:
//
//	byteToken, err := term.ReadPassword(int(os.Stdin.Fd()))

// Exit codes for `setup --check`.
const (
	ExitOK          = 0
	ExitNoCreds     = 1
	ExitInvalidAuth = 2
	ExitNetworkErr  = 3
)

// httpClient is the interface used for HTTP calls so tests can inject a mock.
type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// defaultHTTPClient is the real HTTP client used in production.
var defaultHTTPClient httpClient = http.DefaultClient

// Config holds the non-sensitive workspace configuration.
type Config struct {
	Cloud         string
	SpaceID       string
	SpaceKey      string
	SpaceName     string
	HomePageID    string
}

// SpaceInfo is a single Confluence space from the list-spaces API.
type SpaceInfo struct {
	ID         string
	Key        string
	Name       string
	HomepageID string
}

// ConfigPath returns the platform-appropriate path to the credentials file.
//
//   - Linux:   $XDG_CONFIG_HOME/confluence-docs/credentials  (falls back to ~/.config/…)
//   - macOS:   ~/Library/Application Support/confluence-docs/credentials
//   - Windows: %AppData%\confluence-docs\credentials
func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(dir, "confluence-docs", "credentials"), nil
}

// ConfigFilePath returns the platform-appropriate path to the non-sensitive
// config file (cloud, active_space_*, active_home_page_id).
func ConfigFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(dir, "confluence-docs", "config"), nil
}

// legacyConfigPath returns the old hardcoded path used before the
// cross-platform migration: ~/.config/confluence-docs/credentials.
func legacyConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "confluence-docs", "credentials"), nil
}

// ReadCredsFile reads the credentials file at the canonical config path.
// If the new path does not exist but the legacy path does, it reads the legacy
// file and prints a warning to stderr suggesting migration.
//
// Returns (email, token, error). An os.IsNotExist error means no file found.
// Cloud is no longer returned here — it's read from ReadConfigFile.
func ReadCredsFile(stderr io.Writer) (email, token string, err error) {
	newPath, pathErr := ConfigPath()
	if pathErr != nil {
		return "", "", pathErr
	}

	data, readErr := os.ReadFile(newPath)
	if readErr != nil && os.IsNotExist(readErr) {
		// Try legacy path (~/.config/confluence-docs/credentials).
		// On Linux the legacy and new paths are identical, so skip the
		// legacy check in that case to avoid double-reading.
		legacyPath, legacyErr := legacyConfigPath()
		if legacyErr == nil && legacyPath != newPath {
			if legacyData, legacyReadErr := os.ReadFile(legacyPath); legacyReadErr == nil {
				fmt.Fprintf(stderr,
					"warning: credentials found at legacy path %s — run `confluence-docs setup` to migrate to %s\n",
					legacyPath, newPath)
				data = legacyData
				readErr = nil
			}
		}
	}
	if readErr != nil {
		return "", "", readErr
	}

	return parseCredsData(data)
}

// parseCredsData parses a key=value credential file, returning only email and
// token (cloud is no longer part of the credentials file in v0.10.0+).
func parseCredsData(data []byte) (email, token string, err error) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "email":
			email = val
		case "token":
			token = val
		}
	}
	return email, token, nil
}

// ReadConfigFile reads the non-sensitive config file and returns a Config
// struct. Falls back to reading cloud= from the old credentials file for
// backward compatibility with v0.9.x installs.
// Returns a zero-value Config (not an error) when the file doesn't exist yet.
func ReadConfigFile() Config {
	var cfg Config

	// Try the new config file first.
	path, err := ConfigFilePath()
	if err == nil {
		if data, rerr := os.ReadFile(path); rerr == nil {
			cfg = parseConfigData(data)
			if cfg.Cloud != "" {
				return cfg
			}
		}
	}

	// Backward compat: read cloud= from the old credentials file.
	cfg.Cloud = readCloudFromOldCreds()
	return cfg
}

// parseConfigData parses a key=value config file into a Config struct.
func parseConfigData(data []byte) Config {
	var cfg Config
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "cloud":
			cfg.Cloud = val
		case "active_space_id":
			cfg.SpaceID = val
		case "active_space_key":
			cfg.SpaceKey = val
		case "active_space_name":
			cfg.SpaceName = val
		case "active_home_page_id":
			cfg.HomePageID = val
		}
	}
	return cfg
}

// WriteConfig writes the config file with the given Config. Creates parent
// directories if needed. Permissions: 0644 (non-sensitive data).
func WriteConfig(cfg Config) error {
	path, err := ConfigFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	var sb strings.Builder
	if cfg.Cloud != "" {
		fmt.Fprintf(&sb, "cloud=%s\n", cfg.Cloud)
	}
	if cfg.SpaceID != "" {
		fmt.Fprintf(&sb, "active_space_id=%s\n", cfg.SpaceID)
	}
	if cfg.SpaceKey != "" {
		fmt.Fprintf(&sb, "active_space_key=%s\n", cfg.SpaceKey)
	}
	if cfg.SpaceName != "" {
		fmt.Fprintf(&sb, "active_space_name=%s\n", cfg.SpaceName)
	}
	if cfg.HomePageID != "" {
		fmt.Fprintf(&sb, "active_home_page_id=%s\n", cfg.HomePageID)
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// readCloudFromOldCreds reads the cloud= line from the old credentials file
// (v0.9.x single-file format). Used as backward-compat fallback only.
func readCloudFromOldCreds() string {
	path, err := ConfigPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// Try legacy path.
		legacy, lerr := legacyConfigPath()
		if lerr != nil || legacy == path {
			return ""
		}
		data, err = os.ReadFile(legacy)
		if err != nil {
			return ""
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		if strings.TrimSpace(kv[0]) == "cloud" {
			return strings.TrimSpace(kv[1])
		}
	}
	return ""
}

// ReadCloudFromCreds returns the cloud subdomain from either the config file
// (v0.10.0+) or the old single credentials file (v0.9.x backward compat).
// Kept for compatibility; new code should use ReadConfigFile().Cloud.
func ReadCloudFromCreds() string {
	cfg := ReadConfigFile()
	return cfg.Cloud
}

// WriteCreds writes email and token to the credentials file with secure
// permissions (0600). Creates parent directories if needed.
// The variadic cloud arg is accepted but ignored (cloud now goes to WriteConfig).
// For compatibility with callers that still pass cloud, use WriteConfig separately.
func WriteCreds(email, token string, cloud ...string) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	content := fmt.Sprintf("email=%s\ntoken=%s\n", email, token)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	// On Windows, os.WriteFile with 0600 is not sufficient to restrict access
	// to the current user only. The proper approach requires
	// golang.org/x/sys/windows to set an explicit ACL. This is a known
	// limitation; the file has no permissive sharing but Windows does not
	// enforce POSIX-style mode bits. A future setup_windows.go can add this.
	if runtime.GOOS == "windows" {
		// TODO: implement via golang.org/x/sys/windows if Windows support is
		// prioritized.
		_ = path
	}
	return nil
}

// userInfoResult holds the parsed response from the Confluence current-user API.
type userInfoResult struct {
	DisplayName string
	AccountID   string
	StatusCode  int
	// Err is non-nil only for network/transport errors.
	// Auth failures (401/403) are indicated by StatusCode, not Err.
	Err error
}

// fetchCurrentUser calls the Confluence API to validate credentials.
func fetchCurrentUser(client httpClient, email, token, cloud string) userInfoResult {
	if cloud == "" {
		cloud = resolveCloud()
	}
	baseURL := fmt.Sprintf("https://%s.atlassian.net/wiki", cloud)
	// Use the classic v1 REST endpoint which always exposes displayName.
	url := baseURL + "/rest/api/user/current"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return userInfoResult{Err: fmt.Errorf("build request: %w", err)}
	}

	cred := email + ":" + token
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(cred)))
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return userInfoResult{Err: fmt.Errorf("network error: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return userInfoResult{StatusCode: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return userInfoResult{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("unexpected status %d", resp.StatusCode),
		}
	}

	var body struct {
		DisplayName string `json:"displayName"`
		AccountID   string `json:"accountId"`
		PublicName  string `json:"publicName"` // v2 API alternative
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return userInfoResult{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("parse response: %w", err),
		}
	}
	name := body.DisplayName
	if name == "" {
		name = body.PublicName
	}
	return userInfoResult{
		DisplayName: name,
		AccountID:   body.AccountID,
		StatusCode:  resp.StatusCode,
	}
}

// fetchSpaces fetches accessible spaces from the Confluence API.
// Returns the list of SpaceInfo and any error.
func fetchSpaces(client httpClient, email, token, cloud string) ([]SpaceInfo, error) {
	baseURL := fmt.Sprintf("https://%s.atlassian.net/wiki", cloud)
	url := baseURL + "/api/v2/spaces?status=current&limit=250"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	cred := email + ":" + token
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(cred)))
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d fetching spaces", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			ID          string `json:"id"`
			Key         string `json:"key"`
			Name        string `json:"name"`
			HomepageID  string `json:"homepageId"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse spaces response: %w", err)
	}

	spaces := make([]SpaceInfo, 0, len(result.Results))
	for _, r := range result.Results {
		spaces = append(spaces, SpaceInfo{
			ID:         r.ID,
			Key:        r.Key,
			Name:       r.Name,
			HomepageID: r.HomepageID,
		})
	}
	return spaces, nil
}

// resolveCloud returns the effective Confluence cloud subdomain.
// Priority: $ATLASSIAN_CLOUD env var → config file → old credentials file → "".
// Callers must handle the empty case (e.g. error with a clear message).
func resolveCloud() string {
	if env := os.Getenv("ATLASSIAN_CLOUD"); env != "" {
		return env
	}
	cfg := ReadConfigFile()
	return cfg.Cloud
}

// readLine reads a single trimmed line from r.
// Uses bufio.Reader.ReadString to avoid reading ahead past the first newline,
// so successive calls on the same underlying reader each get one line.
func readLine(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")
	return strings.TrimSpace(line), nil
}

// Run is the main entry point for the `setup` sub-command.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error) {
	return runWithClient(args, stdin, stdout, stderr, defaultHTTPClient)
}

// runWithClient is the testable core of Run with an injectable HTTP client.
func runWithClient(args []string, stdin io.Reader, stdout, stderr io.Writer, client httpClient) (int, error) {
	var (
		email        string
		token        string
		doCheck      bool
		doReconfigure bool
		printPath    bool
		printFormat  bool
		setKey       string
		setValue     string
		doSet        bool
	)

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--email":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag --email requires a value")
				return ExitNoCreds, fmt.Errorf("missing value")
			}
			email = args[i+1]
			i++
		case strings.HasPrefix(a, "--email="):
			email = a[8:]
		case a == "--token":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag --token requires a value")
				return ExitNoCreds, fmt.Errorf("missing value")
			}
			token = args[i+1]
			i++
		case strings.HasPrefix(a, "--token="):
			token = a[8:]
		case a == "--check":
			doCheck = true
		case a == "--reconfigure":
			doReconfigure = true
		case a == "--print-config-path":
			printPath = true
		case a == "--print-config-format":
			printFormat = true
		case a == "--set":
			if i+2 >= len(args) {
				fmt.Fprintln(stderr, "flag --set requires two values: --set <key> <value>")
				return ExitNoCreds, fmt.Errorf("missing value")
			}
			setKey = args[i+1]
			setValue = args[i+2]
			doSet = true
			i += 2
		default:
			fmt.Fprintln(stderr, "setup: unknown flag:", a)
			return ExitNoCreds, fmt.Errorf("unknown flag: %s", a)
		}
	}

	if printPath {
		p, err := ConfigPath()
		if err != nil {
			fmt.Fprintln(stderr, "setup:", err)
			return ExitNetworkErr, err
		}
		fmt.Fprintln(stdout, p)
		return ExitOK, nil
	}

	if printFormat {
		fmt.Fprintln(stdout, "email=user@example.com")
		fmt.Fprintln(stdout, "token=ATATT3xFfGF0...")
		return ExitOK, nil
	}

	if doSet {
		return runSet(setKey, setValue, stdout, stderr)
	}

	if doCheck {
		return runCheck(stdout, stderr, client)
	}

	// Non-interactive mode: both flags provided.
	if email != "" && token != "" && !doReconfigure {
		return runNonInteractive(email, token, stdout, stderr, client)
	}

	// Interactive wizard. Always prefill from existing credentials when
	// present — the wizard shows them masked and lets the user press Enter
	// to keep, so first-time `setup` after a v0.9.x install doesn't force
	// the user to re-paste the token they already have.
	prefillEmail := email
	prefillToken := token
	if prefillEmail == "" || prefillToken == "" {
		existEmail, existToken, _ := ReadCredsFile(stderr)
		if prefillEmail == "" {
			prefillEmail = existEmail
		}
		if prefillToken == "" {
			prefillToken = existToken
		}
	}
	return runInteractive(prefillEmail, prefillToken, stdin, stdout, stderr, client)
}

// knownConfigKeys is the set of keys accepted by --set.
var knownConfigKeys = map[string]bool{
	"cloud":              true,
	"active_space_id":    true,
	"active_space_key":   true,
	"active_space_name":  true,
	"active_home_page_id": true,
}

// runSet implements `setup --set <key> <value>`.
func runSet(key, value string, stdout, stderr io.Writer) (int, error) {
	if !knownConfigKeys[key] {
		fmt.Fprintf(stderr, "setup --set: unknown key %q\n", key)
		fmt.Fprintln(stderr, "  valid keys: cloud, active_space_id, active_space_key, active_space_name, active_home_page_id")
		fmt.Fprintln(stderr, "  tip: to switch spaces use `confluence-docs space use <key>` instead")
		return ExitNoCreds, fmt.Errorf("unknown key: %s", key)
	}
	// Read existing config.
	cfg := ReadConfigFile()
	// Apply the new value.
	switch key {
	case "cloud":
		cfg.Cloud = value
	case "active_space_id":
		cfg.SpaceID = value
	case "active_space_key":
		cfg.SpaceKey = value
	case "active_space_name":
		cfg.SpaceName = value
	case "active_home_page_id":
		cfg.HomePageID = value
	}
	if err := WriteConfig(cfg); err != nil {
		fmt.Fprintln(stderr, "setup --set: writing config:", err)
		return ExitNetworkErr, err
	}
	path, _ := ConfigFilePath()
	fmt.Fprintf(stdout, "config updated: %s = %q (saved to %s)\n", key, value, path)
	return ExitOK, nil
}

// runCheck validates existing credentials and returns the appropriate exit code.
func runCheck(stdout, stderr io.Writer, client httpClient) (int, error) {
	email, token, err := ReadCredsFile(stderr)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(stderr, "no credentials configured — run `confluence-docs setup`")
		} else {
			fmt.Fprintln(stderr, "no credentials configured")
		}
		return ExitNoCreds, nil
	}
	if email == "" || token == "" {
		fmt.Fprintln(stderr, "no credentials configured — run `confluence-docs setup`")
		return ExitNoCreds, nil
	}

	cloud := resolveCloud()
	if cloud == "" {
		fmt.Fprintln(stderr, "no Confluence subdomain configured — run `confluence-docs setup`")
		fmt.Fprintln(stderr, "  fix: run `confluence-docs setup` and provide your subdomain")
		fmt.Fprintln(stderr, "       (e.g. 'mycompany' for mycompany.atlassian.net),")
		fmt.Fprintln(stderr, "       or export ATLASSIAN_CLOUD=mycompany")
		return ExitNoCreds, nil
	}

	// Also validate space is configured.
	cfg := ReadConfigFile()
	if cfg.SpaceID == "" || cfg.HomePageID == "" {
		fmt.Fprintln(stderr, "no active space configured — run `confluence-docs setup` or `confluence-docs space use <key>`")
		return ExitNoCreds, nil
	}

	res := fetchCurrentUser(client, email, token, cloud)
	if res.Err != nil {
		fmt.Fprintf(stderr, "could not validate (network error): %v\n", res.Err)
		return ExitNetworkErr, nil
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		fmt.Fprintln(stderr, "credentials present but invalid")
		return ExitInvalidAuth, nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		fmt.Fprintf(stderr, "could not validate (network error): unexpected status %d\n", res.StatusCode)
		return ExitNetworkErr, nil
	}

	name := res.DisplayName
	if name == "" {
		name = email
	}
	fmt.Fprintf(stdout, "credentials valid (%s, space: %s)\n", name, cfg.SpaceKey)
	return ExitOK, nil
}

// runNonInteractive saves credentials provided via flags without prompting.
func runNonInteractive(email, token string, stdout, stderr io.Writer, client httpClient) (int, error) {
	cloud := resolveCloud()
	if cloud == "" {
		fmt.Fprintln(stderr, "error: no Confluence subdomain configured")
		fmt.Fprintln(stderr, "  fix: export ATLASSIAN_CLOUD=mycompany before running setup,")
		fmt.Fprintln(stderr, "       or run setup without flags for the interactive wizard.")
		return ExitNoCreds, fmt.Errorf("no cloud")
	}
	fmt.Fprint(stderr, "Validating connection... ")
	res := fetchCurrentUser(client, email, token, cloud)
	if res.Err != nil {
		fmt.Fprintf(stderr, "\nerror: could not validate (network error): %v\n", res.Err)
		return ExitNetworkErr, res.Err
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		fmt.Fprintf(stderr, "\nerror: invalid credentials — token may be wrong or revoked\n")
		return ExitInvalidAuth, nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		fmt.Fprintf(stderr, "\nerror: unexpected status %d\n", res.StatusCode)
		return ExitNetworkErr, nil
	}

	// Write secrets only to credentials file.
	if err := WriteCreds(email, token); err != nil {
		fmt.Fprintln(stderr, "error saving credentials:", err)
		return ExitNetworkErr, err
	}
	// Persist cloud to config file (migrate old single-file format).
	cfg := ReadConfigFile()
	cfg.Cloud = cloud
	if err := WriteConfig(cfg); err != nil {
		fmt.Fprintln(stderr, "error saving config:", err)
		return ExitNetworkErr, err
	}

	path, _ := ConfigPath()
	fmt.Fprintf(stderr, "connected as %s (%s at %s)\n", res.DisplayName, res.AccountID, cloud)
	fmt.Fprintf(stdout, "credentials saved to %s\n", path)
	return ExitOK, nil
}

// runInteractive runs the interactive setup wizard.
func runInteractive(prefillEmail, prefillToken string, stdinRaw io.Reader, stdout, stderr io.Writer, client httpClient) (int, error) {
	// Wrap stdin in a buffered reader once. readLine calls bufio.NewReader(stdin)
	// internally — when stdin is already a *bufio.Reader, bufio.NewReader returns
	// it unchanged, so successive readLine calls each consume one line correctly.
	stdin := bufio.NewReader(stdinRaw)
	// Detect if we're likely in a headless environment (no interactive terminal).
	// We still proceed identically — just print a note to stderr.
	if isHeadless() {
		fmt.Fprintln(stderr, "(note: headless environment detected — no interactive terminal detected)")
		fmt.Fprintln(stderr, "      You can also use: confluence-docs setup --email X --token Y")
	}

	fmt.Fprintln(stdout, "confluence-docs setup")
	fmt.Fprintln(stdout, "─────────────────")
	fmt.Fprintln(stdout, "To use confluence-docs we need an Atlassian API token.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "  1. Open this URL in your browser:")
	fmt.Fprintln(stdout, "     https://id.atlassian.com/manage-profile/security/api-tokens")
	fmt.Fprintln(stdout, `  2. Click "Create API token"`)
	fmt.Fprintln(stdout, "  3. Label it: confluence-docs")
	fmt.Fprintln(stdout, "  4. Copy the token and paste below")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "(Press Ctrl+C any time to cancel)")
	fmt.Fprintln(stdout)

	email := prefillEmail
	if email == "" {
		fmt.Fprint(stdout, "Atlassian email: ")
		var err error
		email, err = readLine(stdin)
		if err != nil || email == "" {
			fmt.Fprintln(stderr, "setup cancelled")
			return ExitNoCreds, nil
		}
	} else {
		fmt.Fprintf(stdout, "Atlassian email: %s\n", email)
	}

	token := prefillToken
	if token == "" {
		fmt.Fprint(stdout, "API token: ")
		// NOTE: golang.org/x/term would allow masked input (characters hidden).
		// We use plain readline to avoid adding a dependency. To add masking:
		//   import "golang.org/x/term"
		//   byteToken, _ := term.ReadPassword(int(os.Stdin.Fd()))
		//   token = string(byteToken)
		var err error
		token, err = readLine(stdin)
		if err != nil || token == "" {
			fmt.Fprintln(stderr, "setup cancelled")
			return ExitNoCreds, nil
		}
	} else {
		fmt.Fprintf(stdout, "API token: %s\n", maskToken(token))
	}

	// Ask for the Confluence cloud subdomain. The default is whatever
	// resolveCloud() returns (env var or existing config file value).
	currentCloud := resolveCloud()
	prompt := "Confluence subdomain (e.g. 'mycompany' for mycompany.atlassian.net)"
	if currentCloud != "" {
		prompt = fmt.Sprintf("Confluence subdomain (e.g. 'mycompany' — press Enter to keep '%s')", currentCloud)
	}
	fmt.Fprintf(stdout, "%s: ", prompt)
	cloud, err := readLine(stdin)
	if err != nil {
		fmt.Fprintln(stderr, "setup cancelled")
		return ExitNoCreds, nil
	}
	if cloud == "" {
		cloud = currentCloud
	}
	if cloud == "" {
		fmt.Fprintln(stderr, "setup cancelled — a Confluence subdomain is required")
		return ExitNoCreds, nil
	}

	path, err := ConfigPath()
	if err != nil {
		fmt.Fprintln(stderr, "setup:", err)
		return ExitNetworkErr, err
	}
	fmt.Fprintf(stdout, "\nSaving credentials to: %s\n", path)

	fmt.Fprint(stdout, "Validating connection... ")
	res := fetchCurrentUser(client, email, token, cloud)
	if res.Err != nil {
		fmt.Fprintf(stdout, "\nerror: could not validate (network error): %v\n", res.Err)
		return ExitNetworkErr, res.Err
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		fmt.Fprintf(stdout, "\nerror: invalid credentials — token may be wrong or revoked\n")
		return ExitInvalidAuth, nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		fmt.Fprintf(stdout, "\nerror: unexpected status %d\n", res.StatusCode)
		return ExitNetworkErr, nil
	}
	fmt.Fprintln(stdout, "ok")

	// Step 6: auto-detect spaces via API.
	fmt.Fprint(stdout, "Fetching accessible spaces... ")
	spaces, spaceErr := fetchSpaces(client, email, token, cloud)
	if spaceErr != nil {
		fmt.Fprintf(stdout, "\nwarning: could not fetch spaces (%v)\n", spaceErr)
		fmt.Fprintln(stdout, "Space configuration skipped. Run `confluence-docs space use <key>` later.")
		spaces = nil
	}

	var chosenSpace *SpaceInfo
	switch {
	case len(spaces) == 0 && spaceErr == nil:
		fmt.Fprintln(stdout, "\nno accessible spaces found")
		fmt.Fprintln(stdout, "Space configuration skipped. Run `confluence-docs space use <key>` after gaining access.")
	case len(spaces) == 1:
		chosenSpace = &spaces[0]
		fmt.Fprintf(stdout, "found 1 space: %s (%s)\n", chosenSpace.Name, chosenSpace.Key)
	case len(spaces) > 1:
		fmt.Fprintf(stdout, "found %d spaces:\n", len(spaces))
		for i, s := range spaces {
			fmt.Fprintf(stdout, "  %d. %s (%s, id %s)\n", i+1, s.Name, s.Key, s.ID)
		}

		// Check if there's a current active space to use as default.
		currentCfg := ReadConfigFile()
		defaultIdx := 1
		if currentCfg.SpaceKey != "" {
			for i, s := range spaces {
				if s.Key == currentCfg.SpaceKey {
					defaultIdx = i + 1
					break
				}
			}
		}

		fmt.Fprintf(stdout, "Select space [%d]: ", defaultIdx)
		choiceStr, choiceErr := readLine(stdin)
		if choiceErr != nil {
			fmt.Fprintln(stderr, "setup cancelled")
			return ExitNoCreds, nil
		}
		if choiceStr == "" {
			choiceStr = fmt.Sprintf("%d", defaultIdx)
		}
		var choiceNum int
		if _, scanErr := fmt.Sscanf(choiceStr, "%d", &choiceNum); scanErr != nil || choiceNum < 1 || choiceNum > len(spaces) {
			fmt.Fprintf(stderr, "invalid selection %q — setup cancelled\n", choiceStr)
			return ExitNoCreds, nil
		}
		s := spaces[choiceNum-1]
		chosenSpace = &s
	}

	// Write secrets to credentials file.
	if err := WriteCreds(email, token); err != nil {
		fmt.Fprintln(stdout, "\nerror saving credentials:", err)
		return ExitNetworkErr, err
	}

	// Build and write config.
	cfg := Config{Cloud: cloud}
	if chosenSpace != nil {
		cfg.SpaceID = chosenSpace.ID
		cfg.SpaceKey = chosenSpace.Key
		cfg.SpaceName = chosenSpace.Name
		cfg.HomePageID = chosenSpace.HomepageID
	}
	if err := WriteConfig(cfg); err != nil {
		fmt.Fprintln(stdout, "\nerror saving config:", err)
		return ExitNetworkErr, err
	}

	name := res.DisplayName
	if name == "" {
		name = email
	}
	fmt.Fprintf(stdout, "connected as %s (%s at %s)\n", name, res.AccountID, cloud)
	if chosenSpace != nil {
		fmt.Fprintf(stdout, "active space: %s (key: %s, id: %s)\n", chosenSpace.Name, chosenSpace.Key, chosenSpace.ID)
		if chosenSpace.HomepageID != "" {
			fmt.Fprintf(stdout, "home page ID: %s\n", chosenSpace.HomepageID)
		}
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Done. You can now use:")
	fmt.Fprintln(stdout, "  confluence-docs page get/upload/create")
	fmt.Fprintln(stdout, "  confluence-docs index add/remove/sync")
	fmt.Fprintln(stdout, "  confluence-docs space list/use/current")
	return ExitOK, nil
}

// maskToken returns the first 4 characters followed by asterisks, to allow
// confirming the token prefix without showing the full secret.
func maskToken(token string) string {
	if len(token) <= 4 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-4)
}

// isHeadless returns true if the process appears to be running without an
// interactive terminal.
func isHeadless() bool {
	if runtime.GOOS == "linux" {
		// Headless if neither a display server nor a terminal type is set.
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("TERM") == "" {
			return true
		}
	}
	return false
}
