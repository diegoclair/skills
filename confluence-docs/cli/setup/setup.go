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
// The cloud field is read separately via ReadCloudFromCreds — kept distinct
// for backward compatibility with older credential files that only had
// email/token.
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

// parseCredsData parses a key=value credential file.
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

// ReadCloudFromCreds returns the cloud subdomain stored in the credentials
// file, or "" if not present. Errors reading the file return "".
func ReadCloudFromCreds() string {
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

// WriteCreds writes email, token and cloud subdomain to the config file with
// secure permissions. Creates parent directories if needed. If cloud is "",
// the cloud line is omitted (preserves backward compat).
func WriteCreds(email, token string, cloud ...string) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	content := fmt.Sprintf("email=%s\ntoken=%s\n", email, token)
	if len(cloud) > 0 && cloud[0] != "" {
		content += fmt.Sprintf("cloud=%s\n", cloud[0])
	}
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

// resolveCloud returns the effective Confluence cloud subdomain.
// Priority: $ATLASSIAN_CLOUD env var → cloud= line in credentials file → "".
// Callers must handle the empty case (e.g. error with a clear message).
func resolveCloud() string {
	if env := os.Getenv("ATLASSIAN_CLOUD"); env != "" {
		return env
	}
	if fromCreds := ReadCloudFromCreds(); fromCreds != "" {
		return fromCreds
	}
	return ""
}

// readLine reads a single trimmed line from r.
func readLine(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

// Run is the main entry point for the `setup` sub-command.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error) {
	return runWithClient(args, stdin, stdout, stderr, defaultHTTPClient)
}

// runWithClient is the testable core of Run with an injectable HTTP client.
func runWithClient(args []string, stdin io.Reader, stdout, stderr io.Writer, client httpClient) (int, error) {
	var (
		email       string
		token       string
		doCheck     bool
		printPath   bool
		printFormat bool
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
		case a == "--print-config-path":
			printPath = true
		case a == "--print-config-format":
			printFormat = true
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

	if doCheck {
		return runCheck(stdout, stderr, client)
	}

	// Non-interactive mode: both flags provided.
	if email != "" && token != "" {
		return runNonInteractive(email, token, stdout, stderr, client)
	}

	// Interactive wizard.
	return runInteractive(email, token, stdin, stdout, stderr, client)
}

// runCheck validates existing credentials and returns the appropriate exit code.
func runCheck(stdout, stderr io.Writer, client httpClient) (int, error) {
	email, token, err := ReadCredsFile(stderr)
	if err != nil {
		fmt.Fprintln(stderr, "no credentials configured")
		return ExitNoCreds, nil
	}
	if email == "" || token == "" {
		fmt.Fprintln(stderr, "no credentials configured")
		return ExitNoCreds, nil
	}

	cloud := resolveCloud()
	if cloud == "" {
		fmt.Fprintln(stderr, "no Confluence subdomain configured")
		fmt.Fprintln(stderr, "  fix: run `confluence-docs setup` and provide your subdomain")
		fmt.Fprintln(stderr, "       (e.g. 'mycompany' for mycompany.atlassian.net),")
		fmt.Fprintln(stderr, "       or export ATLASSIAN_CLOUD=mycompany,")
		fmt.Fprintln(stderr, "       or add `cloud=mycompany` to the credentials file.")
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
	fmt.Fprintf(stdout, "credentials valid (%s)\n", name)
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

	// Persist the cloud subdomain too so subsequent runs don't need the env
	// var. If it came from env we still write it — explicit storage avoids
	// surprise when the user later launches a shell without the env set.
	if err := WriteCreds(email, token, cloud); err != nil {
		fmt.Fprintln(stderr, "error saving credentials:", err)
		return ExitNetworkErr, err
	}

	path, _ := ConfigPath()
	fmt.Fprintf(stderr, "connected as %s (%s at %s)\n", res.DisplayName, res.AccountID, cloud)
	fmt.Fprintf(stdout, "credentials saved to %s\n", path)
	return ExitOK, nil
}

// runInteractive runs the interactive setup wizard.
func runInteractive(prefillEmail, prefillToken string, stdin io.Reader, stdout, stderr io.Writer, client httpClient) (int, error) {
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
	// resolveCloud() returns (env var or existing creds file value).
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

	if err := WriteCreds(email, token, cloud); err != nil {
		fmt.Fprintln(stdout, "\nerror saving credentials:", err)
		return ExitNetworkErr, err
	}

	name := res.DisplayName
	if name == "" {
		name = email
	}
	fmt.Fprintf(stdout, "connected as %s (%s at %s)\n", name, res.AccountID, cloud)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Done. You can now use:")
	fmt.Fprintln(stdout, "  confluence-docs page get/upload/create")
	fmt.Fprintln(stdout, "  confluence-docs index add/remove/sync")
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
