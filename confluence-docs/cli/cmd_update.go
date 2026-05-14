package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func runUpdate(args []string, stdout, stderr io.Writer) (int, error) {
	const (
		repoOwnerRepo = "diegoclair/skills"
		installShURL  = "https://raw.githubusercontent.com/diegoclair/skills/main/confluence-docs/install/install.sh"
		installPS1URL = "https://raw.githubusercontent.com/diegoclair/skills/main/confluence-docs/install/install.ps1"
		// exit 10 is reserved for "update available" so scripts/CI can
		// distinguish "all good" (0) from "needs upgrade" without parsing.
		exitUpdateAvailable = 10
	)

	var checkOnly bool
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--check":
			checkOnly = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "update — fetch the latest release of confluence-docs.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  confluence-docs update            # download + install latest release")
			fmt.Fprintln(stdout, "  confluence-docs update --check    # only report whether an update is available")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Behavior: resolves the latest release tag from GitHub, compares with the")
			fmt.Fprintln(stdout, "currently-installed version, and (unless --check) shells out to install.sh")
			fmt.Fprintln(stdout, "(or install.ps1 on Windows) to perform the upgrade. Credentials and the")
			fmt.Fprintln(stdout, "home cache are preserved across the update.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Exit codes:")
			fmt.Fprintln(stdout, "  0   up to date (or upgrade succeeded)")
			fmt.Fprintln(stdout, "  10  --check: an update is available")
			fmt.Fprintln(stdout, "  3   network error / installer failure")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	latest, err := resolveLatestVersion(repoOwnerRepo)
	if err != nil {
		fmt.Fprintln(stderr, "could not resolve latest version:", err)
		return exitUnknownErr, err
	}

	current := version
	if normalizeVersion(current) == normalizeVersion(latest) {
		fmt.Fprintf(stdout, "confluence-docs is up to date (%s).\n", current)
		return exitOK, nil
	}

	if checkOnly {
		fmt.Fprintf(stdout, "current: %s\nlatest:  %s\nrun: confluence-docs update\n", current, latest)
		return exitUpdateAvailable, nil
	}

	fmt.Fprintf(stdout, "Updating confluence-docs: %s → %s ...\n", current, latest)

	// Shell out to the public installer. This works for Linux/macOS via
	// `curl | bash`. On Windows the equivalent is `iwr | iex` in PowerShell.
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// Try to move the running binary out of the way so install.ps1 can
		// overwrite the destination cleanly. Best-effort — on failure the
		// installer will likely fail with a clear error from Windows.
		if exe, eerr := os.Executable(); eerr == nil {
			_ = os.Rename(exe, exe+".old")
		}
		cmd = exec.Command("powershell", "-NoProfile", "-Command",
			fmt.Sprintf("iwr -useb %s | iex", installPS1URL))
	default:
		cmd = exec.Command("sh", "-c",
			fmt.Sprintf("curl -fsSL %s | bash", installShURL))
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(stderr, "installer failed:", err)
		return exitUnknownErr, err
	}
	return exitOK, nil
}

// resolveLatestVersion returns the latest release tag for repo ("owner/repo")
// by following GitHub's /releases/latest redirect chain to its final URL,
// which always ends in /releases/tag/<tag>.
//
// The naive single-redirect approach (CheckRedirect: ErrUseLastResponse +
// read first Location header) breaks when a repo has been transferred:
// GitHub returns the owner-renamed URL as the first Location, then a
// second redirect resolves "latest" → "tag/<tag>". Following the full
// chain is what install.sh has always done (curl -fsSL is default-follow),
// so this just brings the Go binary in line.
//
// We let the Go HTTP client follow redirects (default cap is 10) and
// inspect resp.Request.URL — that's the URL after the last redirect,
// regardless of how many hops it took.
func resolveLatestVersion(repo string) (string, error) {
	url := fmt.Sprintf("https://github.com/%s/releases/latest", repo)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	idx := strings.LastIndex(finalURL, "/tag/")
	if idx < 0 {
		return "", fmt.Errorf("unexpected final URL after redirects (status %d): %s — repo may have no releases yet", resp.StatusCode, finalURL)
	}
	tag := strings.TrimSpace(finalURL[idx+len("/tag/"):])
	if tag == "" {
		return "", fmt.Errorf("empty tag in final URL: %s", finalURL)
	}
	return tag, nil
}

// normalizeVersion strips a leading "v" so "v0.3.3" and "0.3.3" compare equal.
// Also handles the build-time "dev" / "v0.3.0-3-g734f5ea-dirty" variants — for
// those, equality with a clean tag is impossible, which is the intent (a dev
// build should always show as "behind").
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	return v
}

// writeJSON marshals n and writes it to stdout.
