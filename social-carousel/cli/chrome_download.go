package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// chrome_download.go — auto-download Chrome for Testing (CFT) on first
// use. The Puppeteer crowd has done this for years: a small headless
// Chromium build, signed by Google, served at a public URL, no sudo, no
// package manager, no impact on the user's system Chrome.
//
// We grab the `chrome-headless-shell` variant (~80 MB) rather than the
// full Chrome (~280 MB) because we never need a GUI — just CDP screenshots.
//
// Cache layout (idempotent):
//
//	~/.cache/social-carousel/chrome/
//	  <version>/
//	    chrome-headless-shell-<platform>/
//	      chrome-headless-shell        ← the binary we exec
//	      <support libs>
//
// findChrome() prefers (in order):
//   1. SOCIAL_CAROUSEL_CHROME_PATH env var
//   2. System Chrome / Chromium on PATH
//   3. The CFT binary in the cache dir
//   4. (Caller can then invoke ensureCFT() to download.)

// cftLastKnownGoodURL is the canonical endpoint Google maintains for
// "what is the current stable Chrome for Testing build". Fetched once
// per install; the response is small (~10 KB).
const cftLastKnownGoodURL = "https://googlechromelabs.github.io/chrome-for-testing/last-known-good-versions-with-downloads.json"

// cftDownloadTimeout caps the entire download+extract roundtrip. The
// chrome-headless-shell ZIP is ~80 MB; on a slow connection (1 Mbps)
// that's ~11 minutes, so we give 15 minutes of headroom.
const cftDownloadTimeout = 15 * time.Minute

type cftResponse struct {
	Channels map[string]struct {
		Channel   string `json:"channel"`
		Version   string `json:"version"`
		Downloads struct {
			HeadlessShell []cftAsset `json:"chrome-headless-shell"`
		} `json:"downloads"`
	} `json:"channels"`
}

type cftAsset struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

// cftPlatform maps Go's GOOS+GOARCH to the CFT platform string.
// Returns "" if the current OS/arch isn't covered by CFT (Google ships
// linux64, mac-arm64, mac-x64, win32, win64 — no arm Linux, no FreeBSD).
func cftPlatform() string {
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "amd64" {
			return "linux64"
		}
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return "mac-arm64"
		case "amd64":
			return "mac-x64"
		}
	case "windows":
		if runtime.GOARCH == "amd64" {
			return "win64"
		}
		return "win32"
	}
	return ""
}

// chromeCacheDir returns the per-OS cache root for our Chrome bundle.
// Uses os.UserCacheDir() so each OS lands in its native convention:
//
//	Linux/WSL:   $XDG_CACHE_HOME/social-carousel/chrome
//	             or ~/.cache/social-carousel/chrome
//	macOS:       ~/Library/Caches/social-carousel/chrome
//	Windows:     %LocalAppData%\social-carousel\chrome
//	             (e.g. C:\Users\<u>\AppData\Local\social-carousel\chrome)
//
// All three locations are user-writable without elevation, survive
// reboots, and are correctly excluded from cloud-sync defaults on each
// OS (Dropbox, OneDrive, iCloud all skip their respective cache roots).
func chromeCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "social-carousel", "chrome"), nil
}

// findCachedChrome returns the path to a previously-downloaded CFT binary
// in the cache dir, if any AND if its full lib graph resolves. On Linux
// we use ldd as the gate: `chrome --version` is a misleadingly-cheap
// printf path that does NOT actually load NSS/NSPR (they're lazy-loaded
// at first browser op), so it would pass even when libnss3 is missing.
// `ldd` reports every declared dep, which is what we need.
func findCachedChrome() (string, bool) {
	root, err := chromeCacheDir()
	if err != nil {
		return "", false
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", false
	}
	platform := cftPlatform()
	if platform == "" {
		return "", false
	}
	// Walk newest version directories first. Lexicographic sort is good
	// enough for Chrome's MAJOR.MINOR.BUILD.PATCH numbering when versions
	// have matching length (CFT version strings always do).
	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	// Sort descending.
	for i := 0; i < len(versions); i++ {
		for j := i + 1; j < len(versions); j++ {
			if versions[i] < versions[j] {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}
	for _, v := range versions {
		bin := cftBinaryPath(filepath.Join(root, v), platform)
		if _, err := os.Stat(bin); err != nil {
			continue
		}
		// On Linux, gate on ldd output — see comment on findCachedChrome.
		// Mac/Windows bundles are self-contained, skip the check.
		if runtime.GOOS == "linux" {
			if len(scanMissingLibs(bin)) > 0 {
				continue
			}
		}
		return bin, true
	}
	return "", false
}

// cftBinaryPath returns the expected path to the executable inside an
// extracted CFT bundle, given the extraction root and the CFT platform.
func cftBinaryPath(extractRoot, platform string) string {
	subdir := "chrome-headless-shell-" + platform
	exe := "chrome-headless-shell"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	return filepath.Join(extractRoot, subdir, exe)
}

// ensureCFT guarantees that a Chrome for Testing binary exists in the
// cache and returns its absolute path. If a cached version is already
// present, returns immediately. Otherwise: fetches the last-known-good
// JSON, picks the Stable channel, downloads + extracts the ZIP for the
// current platform, and returns the binary path.
//
// progress is where status lines are written (typically stdout). Pass
// io.Discard to silence them.
func ensureCFT(progress io.Writer) (string, error) {
	// Fast path: already cached.
	if bin, ok := findCachedChrome(); ok {
		return bin, nil
	}

	platform := cftPlatform()
	if platform == "" {
		return "", fmt.Errorf("Chrome for Testing does not publish a build for %s/%s; install Chrome / Chromium manually", runtime.GOOS, runtime.GOARCH)
	}

	fmt.Fprintln(progress, "→ Chrome / Chromium not found on system.")
	fmt.Fprintln(progress, "→ Downloading Chrome for Testing (~80 MB, one-time, no sudo)…")

	// Step 1: fetch the index.
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(cftLastKnownGoodURL)
	if err != nil {
		return "", fmt.Errorf("fetch CFT index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("fetch CFT index: HTTP %d", resp.StatusCode)
	}
	var idx cftResponse
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return "", fmt.Errorf("parse CFT index: %w", err)
	}

	stable, ok := idx.Channels["Stable"]
	if !ok {
		return "", fmt.Errorf("CFT index has no Stable channel")
	}
	var zipURL string
	for _, a := range stable.Downloads.HeadlessShell {
		if a.Platform == platform {
			zipURL = a.URL
			break
		}
	}
	if zipURL == "" {
		return "", fmt.Errorf("CFT index has no chrome-headless-shell for platform %q", platform)
	}

	fmt.Fprintf(progress, "    Version: %s (channel: Stable)\n", stable.Version)
	fmt.Fprintf(progress, "    URL: %s\n", zipURL)

	// Step 2: download the ZIP to a temp file.
	dlClient := &http.Client{Timeout: cftDownloadTimeout}
	dlResp, err := dlClient.Get(zipURL)
	if err != nil {
		return "", fmt.Errorf("download CFT zip: %w", err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != 200 {
		return "", fmt.Errorf("download CFT zip: HTTP %d", dlResp.StatusCode)
	}

	tmpZip, err := os.CreateTemp("", "social-carousel-cft-*.zip")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpZip.Name())
	written, err := io.Copy(tmpZip, dlResp.Body)
	tmpZip.Close()
	if err != nil {
		return "", fmt.Errorf("write CFT zip: %w", err)
	}
	fmt.Fprintf(progress, "    Downloaded %.1f MB\n", float64(written)/1024/1024)

	// Step 3: extract to <cache>/<version>/.
	cacheRoot, err := chromeCacheDir()
	if err != nil {
		return "", err
	}
	extractRoot := filepath.Join(cacheRoot, stable.Version)
	if err := os.MkdirAll(extractRoot, 0o755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	if err := extractZip(tmpZip.Name(), extractRoot); err != nil {
		return "", fmt.Errorf("extract CFT zip: %w", err)
	}

	// Step 4: locate binary, mark executable.
	bin := cftBinaryPath(extractRoot, platform)
	if _, err := os.Stat(bin); err != nil {
		return "", fmt.Errorf("CFT binary not found at expected path %s: %w", bin, err)
	}
	if err := os.Chmod(bin, 0o755); err != nil {
		// Non-fatal: zip extract may already preserve mode on Unix.
		fmt.Fprintf(progress, "    warning: could not chmod %s: %v\n", bin, err)
	}
	fmt.Fprintf(progress, "✓ Chrome for Testing installed: %s\n", bin)

	// Step 5: smoke-test on Linux for the dynamic libs Chrome needs.
	// macOS and Windows ship with everything; Linux distros vary.
	if runtime.GOOS == "linux" {
		if err := validateLinuxChromeLibs(bin, progress); err != nil {
			return "", err
		}
	}
	return bin, nil
}

// validateLinuxChromeLibs uses `ldd` (NOT `chrome --version`) to detect
// missing shared libraries. `chrome --version` is a printf that doesn't
// load the heavy libs (NSS/NSPR) so it can pass even when they're absent
// — only to fail later at first render. ldd reports the full dep graph
// statically. If any dep is missing, returns a friendly error pointing
// at the exact per-distro install command.
func validateLinuxChromeLibs(bin string, progress io.Writer) error {
	missingSOs := scanMissingLibs(bin)
	if len(missingSOs) == 0 {
		return nil
	}

	fmt.Fprintln(progress, "")
	fmt.Fprintln(progress, "  ✗ Chrome downloaded but is missing Linux system libraries.")
	fmt.Fprintln(progress, "")
	fmt.Fprintln(progress, "  ════════════════════════════════════════════════════════════════")
	fmt.Fprintln(progress, "  ACTION REQUIRED — install system libraries for headless Chrome.")
	fmt.Fprintln(progress, "")
	fmt.Fprintln(progress, "  This is a one-time install. Puppeteer / Playwright / any Linux")
	fmt.Fprintln(progress, "  headless Chrome workflow needs the same dependencies.")
	fmt.Fprintln(progress, "")

	if len(missingSOs) > 0 {
		fmt.Fprintf(progress, "  Detected %d missing libraries:\n", len(missingSOs))
		for _, so := range missingSOs {
			fmt.Fprintf(progress, "    - %s\n", so)
		}
		fmt.Fprintln(progress, "")
	}

	printDistroInstallCommand(progress, missingSOs)

	fmt.Fprintln(progress, "")
	fmt.Fprintln(progress, "  After installing, re-run:")
	fmt.Fprintln(progress, "    social-carousel postinstall")
	fmt.Fprintln(progress, "  ════════════════════════════════════════════════════════════════")
	return fmt.Errorf("missing Linux system libraries for headless Chrome (see hints above)")
}

// scanMissingLibs runs `ldd <bin>` and returns the unique list of
// "lib*.so* => not found" entries. Returns nil if ldd isn't available or
// the parse fails — callers fall back to the conservative full list.
func scanMissingLibs(bin string) []string {
	if !hasCmd("ldd") {
		return nil
	}
	out, err := exec.Command("ldd", bin).CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var missing []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// Format: "libfoo.so.1 => not found"
		if !strings.Contains(line, "=> not found") {
			continue
		}
		so := strings.TrimSpace(strings.SplitN(line, "=>", 2)[0])
		if so == "" {
			continue
		}
		if _, ok := seen[so]; ok {
			continue
		}
		seen[so] = struct{}{}
		missing = append(missing, so)
	}
	return missing
}

// soToAptPackage maps a .so name to its Debian/Ubuntu package.
// Each entry lists multiple package-name candidates because Ubuntu has
// been rolling a "*t64" rename (64-bit time_t transition) across LTS
// versions: e.g. libasound2 → libasound2t64 in 24.04+, libatk1.0-0 →
// libatk1.0-0t64, libgtk-3-0 → libgtk-3-0t64. Newer names ship first;
// `apt-get install` accepts a list and installs whichever exists.
// Some libs (libnss3, libnspr4) kept their original name in 26.04 so
// they only appear once.
var soToAptPackage = map[string][]string{
	"libnss3.so":              {"libnss3"},
	"libnssutil3.so":          {"libnss3"},
	"libsmime3.so":            {"libnss3"},
	"libnspr4.so":             {"libnspr4"},
	"libplc4.so":              {"libnspr4"},
	"libplds4.so":             {"libnspr4"},
	"libatk-1.0.so.0":         {"libatk1.0-0t64", "libatk1.0-0"},
	"libatk-bridge-2.0.so.0":  {"libatk-bridge2.0-0t64", "libatk-bridge2.0-0"},
	"libcups.so.2":            {"libcups2t64", "libcups2"},
	"libxkbcommon.so.0":       {"libxkbcommon0"},
	"libxcomposite.so.1":      {"libxcomposite1"},
	"libxdamage.so.1":         {"libxdamage1"},
	"libxfixes.so.3":          {"libxfixes3"},
	"libxrandr.so.2":          {"libxrandr2"},
	"libgbm.so.1":             {"libgbm1"},
	"libxshmfence.so.1":       {"libxshmfence1"},
	"libasound.so.2":          {"libasound2t64", "libasound2"},
	"libpango-1.0.so.0":       {"libpango-1.0-0"},
	"libpangocairo-1.0.so.0":  {"libpangocairo-1.0-0"},
	"libcairo.so.2":           {"libcairo2"},
	"libgtk-3.so.0":           {"libgtk-3-0t64", "libgtk-3-0"},
	"libdrm.so.2":             {"libdrm2"},
	"libdbus-1.so.3":          {"libdbus-1-3"},
	"libgdk-pixbuf-2.0.so.0":  {"libgdk-pixbuf-2.0-0"},
	"libgio-2.0.so.0":         {"libglib2.0-0t64", "libglib2.0-0"},
	"libglib-2.0.so.0":        {"libglib2.0-0t64", "libglib2.0-0"},
	"libgobject-2.0.so.0":     {"libglib2.0-0t64", "libglib2.0-0"},
}

// printDistroInstallCommand emits the install command for the detected
// distro. If we successfully mapped every missing .so to a known package,
// installs only those. Otherwise prints the full known-good list as
// fallback (so the user can't be blocked by an unmapped .so).
func printDistroInstallCommand(progress io.Writer, missingSOs []string) {
	if hasCmd("apt-get") {
		pkgs := mapToAptPackages(missingSOs)
		fmt.Fprintln(progress, "  On Debian / Ubuntu / WSL:")
		if len(pkgs) > 0 {
			fmt.Fprintf(progress, "    sudo apt-get update && sudo apt-get install -y %s\n", strings.Join(pkgs, " "))
		} else {
			// Conservative full list: works for any Chrome version.
			fmt.Fprintln(progress, "    sudo apt-get update && sudo apt-get install -y \\")
			fmt.Fprintln(progress, "      libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 \\")
			fmt.Fprintln(progress, "      libxcomposite1 libxdamage1 libxrandr2 libxkbcommon0 \\")
			fmt.Fprintln(progress, "      libgbm1 libgtk-3-0 libasound2 libpangocairo-1.0-0 \\")
			fmt.Fprintln(progress, "      libpango-1.0-0 libcairo2 libcups2 libdbus-1-3 \\")
			fmt.Fprintln(progress, "      libdrm2 libxshmfence1 fonts-liberation")
		}
	} else if hasCmd("dnf") {
		// Fedora/RHEL: lib name mapping is more involved (libfoo.so.X →
		// "yum provides */libfoo.so.X" → package). Use the conservative
		// list — distro users tend to be more comfortable with it.
		fmt.Fprintln(progress, "  On Fedora / RHEL:")
		fmt.Fprintln(progress, "    sudo dnf install -y nss nspr atk at-spi2-atk \\")
		fmt.Fprintln(progress, "      libXcomposite libXdamage libXrandr libxkbcommon \\")
		fmt.Fprintln(progress, "      mesa-libgbm gtk3 alsa-lib libdrm cairo cups-libs")
	} else if hasCmd("pacman") {
		fmt.Fprintln(progress, "  On Arch Linux:")
		fmt.Fprintln(progress, "    sudo pacman -S --needed nss nspr atk at-spi2-atk \\")
		fmt.Fprintln(progress, "      libxcomposite libxdamage libxrandr libxkbcommon \\")
		fmt.Fprintln(progress, "      mesa gtk3 alsa-lib libdrm cairo libcups")
	} else {
		fmt.Fprintln(progress, "  Install the equivalent of:")
		fmt.Fprintln(progress, "    libnss3 libnspr4 libatk1.0 libatk-bridge2.0 libxcomposite1")
		fmt.Fprintln(progress, "    libxdamage1 libxrandr2 libxkbcommon0 libgbm1 libgtk-3-0")
		fmt.Fprintln(progress, "    libasound2 libpangocairo-1.0 libcairo2 libcups2 libdrm2")
	}
}

// mapToAptPackages translates a list of missing .so files into the
// minimal set of Debian/Ubuntu packages. Returns nil if any .so can't be
// mapped — the caller falls back to the full list.
//
// Each .so may map to multiple candidate package names (libfoo +
// libfoot64) because of the 24.04+ t64 transition. We emit ALL candidates
// — `apt-get install` accepts a list with non-existent packages on most
// versions, but to be safe we prefer using a `|`-or pattern that any
// apt understands.
func mapToAptPackages(missingSOs []string) []string {
	if len(missingSOs) == 0 {
		return nil
	}
	pkgs := map[string]struct{}{}
	for _, so := range missingSOs {
		candidates, ok := soToAptPackage[so]
		if !ok {
			// Conservative: if we don't recognise even one .so, bail
			// to the full list — better than silently missing a dep.
			return nil
		}
		// Add all candidate names so the caller can join them. Newer
		// (t64) names come first in our table; we keep that order in
		// the output so users see modern names first.
		for _, pkg := range candidates {
			pkgs[pkg] = struct{}{}
		}
	}
	out := make([]string, 0, len(pkgs))
	for p := range pkgs {
		out = append(out, p)
	}
	// Sort: t64 variants first, then plain names. This nudges the user
	// toward the modern naming on 24.04+ but the install line still
	// includes both — apt picks whichever exists.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			ti := strings.Contains(out[i], "t64")
			tj := strings.Contains(out[j], "t64")
			if ti == tj {
				if out[i] > out[j] {
					out[i], out[j] = out[j], out[i]
				}
			} else if !ti && tj {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// hasCmd is a tiny PATH-lookup helper used to pick the right per-distro
// install command in error hints.
func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// extractZip is a minimal zip extractor that preserves Unix file modes
// (important for the Chrome binary, which needs the executable bit).
// It also rejects entries that would write outside dst (zip-slip).
func extractZip(zipPath, dst string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Reject ".." segments that would escape dst.
		target := filepath.Join(dst, f.Name)
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) &&
			target != filepath.Clean(dst) {
			return fmt.Errorf("zip entry %q escapes destination", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		mode := f.Mode()
		if mode == 0 {
			mode = 0o644
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			src.Close()
			return err
		}
		if _, err := io.Copy(out, src); err != nil {
			out.Close()
			src.Close()
			return err
		}
		out.Close()
		src.Close()
	}
	return nil
}
