package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// runSetup detects Chrome / Chromium and writes ~/.config/social-carousel/config.yaml.
//
// The skill has no SaaS credentials — the only "setup" step is verifying
// that headless Chrome is reachable. The config file caches the resolved
// path so subsequent renders skip the search.
func runSetup(args []string, stdout, stderr io.Writer) (int, error) {
	var checkOnly bool
	for _, a := range args {
		switch a {
		case "--check":
			checkOnly = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "setup — detect Chrome / Chromium and persist its path.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  social-carousel setup            # detect and write config")
			fmt.Fprintln(stdout, "  social-carousel setup --check    # report status without writing")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	chromePath, err := findChrome()
	if err != nil {
		fmt.Fprintln(stderr, "Chrome / Chromium not found on PATH.")
		fmt.Fprintln(stderr, "Install Google Chrome (https://google.com/chrome) or Chromium,")
		fmt.Fprintln(stderr, "or set SOCIAL_CAROUSEL_CHROME_PATH to its absolute path.")
		return exitUnknownErr, err
	}

	fmt.Fprintf(stdout, "Chrome detected: %s\n", chromePath)

	if checkOnly {
		return exitOK, nil
	}

	cfgPath, err := configPath()
	if err != nil {
		return exitUnknownErr, err
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return exitUnknownErr, err
	}
	content := fmt.Sprintf("chrome_path: %s\n", chromePath)
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		return exitUnknownErr, err
	}
	fmt.Fprintf(stdout, "Config written: %s\n", cfgPath)
	return exitOK, nil
}

// findChrome returns the absolute path to a working Chrome/Chromium binary.
// Resolution order:
//
//  1. SOCIAL_CAROUSEL_CHROME_PATH env var (explicit override)
//  2. System-installed Chrome / Chromium / google-chrome / chromium-browser
//  3. Previously-downloaded Chrome for Testing in ~/.cache/social-carousel/chrome/
//
// findChrome does NOT auto-download. Callers that want auto-download
// should call ensureCFT() after a failed findChrome().
func findChrome() (string, error) {
	if env := os.Getenv("SOCIAL_CAROUSEL_CHROME_PATH"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env, nil
		}
	}
	candidates := chromeCandidates()
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	// Fall back to the CFT cache so manual auto-download doesn't have
	// to re-fetch every run.
	if bin, ok := findCachedChrome(); ok {
		return bin, nil
	}
	return "", fmt.Errorf("no chrome binary found in %v", candidates)
}

// chromeCandidates lists plausible names / paths per OS. Order matters:
// stable Chrome is preferred over Chromium when both exist.
func chromeCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"google-chrome", "chromium",
		}
	case "windows":
		return []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			"chrome.exe",
		}
	default: // linux et al.
		return []string{
			"google-chrome", "google-chrome-stable", "chromium",
			"chromium-browser", "/usr/bin/google-chrome",
			"/usr/bin/chromium", "/usr/bin/chromium-browser",
		}
	}
}

// configPath returns the canonical config file location.
// Honors XDG_CONFIG_HOME if set.
func configPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "social-carousel", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "social-carousel", "config.yaml"), nil
}
