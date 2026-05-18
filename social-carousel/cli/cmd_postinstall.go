package main

import (
	"fmt"
	"io"
	"runtime"
)

// runPostinstall is invoked by pkg/install/install.{sh,ps1} right after the
// binary is unpacked. Its job is to surface environmental requirements that
// the binary cannot fix itself: a working Chrome / Chromium install. Fonts
// are bundled into the release archive (the embedded go:embed FS), so they
// don't need a runtime download.
//
// Contract enforced by the shared installer:
//
//	postinstall --check   exit 0  → "yes, I implement postinstall"
//	postinstall           exit 0  → all checks passed
//	postinstall           exit !0 → user must address hints; install continues
//
// Skills that don't implement it return errInvalidUsage (exit 2) for
// "unknown command"; the installer treats that as "skip silently".
func runPostinstall(args []string, stdout, stderr io.Writer) (int, error) {
	var checkOnly bool
	for _, a := range args {
		switch a {
		case "--check":
			checkOnly = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "postinstall — run post-install environment checks.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  social-carousel postinstall          # run checks, print hints")
			fmt.Fprintln(stdout, "  social-carousel postinstall --check  # exit 0 if implemented (used by the installer)")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}
	if checkOnly {
		// Simply signal "I implement postinstall" — the installer relies
		// on this exit code, not on output.
		return exitOK, nil
	}

	// Run the actual checks.
	var problems int

	// 1. Chrome / Chromium / cached CFT.
	if path, err := findChrome(); err == nil {
		fmt.Fprintf(stdout, "  ✓ Chrome / Chromium found: %s\n", path)
	} else {
		// Try the auto-download path. This is what Puppeteer does:
		// fetch a small headless Chromium build from Google's CFT
		// channel into a user-local cache. No sudo, no apt-get.
		fmt.Fprintln(stdout, "")
		bin, dlErr := ensureCFT(stdout)
		if dlErr == nil {
			fmt.Fprintf(stdout, "  ✓ Ready (using Chrome for Testing at %s)\n", bin)
		} else {
			problems++
			fmt.Fprintln(stdout, "")
			fmt.Fprintf(stdout, "  ✗ Auto-download failed: %v\n", dlErr)
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  Fallback: install Chrome / Chromium manually.")
			printChromeHints(stdout)
		}
	}

	if problems > 0 {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "  ════════════════════════════════════════════════════════════════")
		fmt.Fprintf(stdout, "  ACTION REQUIRED — %d check(s) failed.\n", problems)
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "  Auto-download did not succeed (network / proxy?). Install Chrome")
		fmt.Fprintln(stdout, "  via the OS command shown above, then re-run:")
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "    social-carousel postinstall    # re-verify everything is ready")
		fmt.Fprintln(stdout, "  ════════════════════════════════════════════════════════════════")
		return exitUnknownErr, nil
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "  ✓ All checks passed. Try:")
	fmt.Fprintln(stdout, "    social-carousel new listicle --out /tmp/carousel.yaml")
	fmt.Fprintln(stdout, "    social-carousel check /tmp/carousel.yaml")
	fmt.Fprintln(stdout, "    social-carousel render /tmp/carousel.yaml")
	return exitOK, nil
}

// printChromeHints emits a per-OS hint pointing the user at the right
// install command. We deliberately do NOT auto-invoke apt/brew/winget —
// silently escalating privileges during a `curl | bash` is a trust
// violation. The user runs the suggested command and re-tries.
func printChromeHints(w io.Writer) {
	fmt.Fprintln(w, "")
	switch runtime.GOOS {
	case "linux":
		fmt.Fprintln(w, "    Install on Debian / Ubuntu / WSL:")
		fmt.Fprintln(w, "      sudo apt-get update && sudo apt-get install -y chromium-browser")
		fmt.Fprintln(w, "    Install on Fedora / RHEL:")
		fmt.Fprintln(w, "      sudo dnf install -y chromium")
		fmt.Fprintln(w, "    Or download Google Chrome stable:")
		fmt.Fprintln(w, "      https://www.google.com/chrome/")
	case "darwin":
		fmt.Fprintln(w, "    Install on macOS via Homebrew:")
		fmt.Fprintln(w, "      brew install --cask google-chrome")
		fmt.Fprintln(w, "    Or download from https://www.google.com/chrome/")
	case "windows":
		fmt.Fprintln(w, "    Install on Windows via winget:")
		fmt.Fprintln(w, "      winget install -e --id Google.Chrome")
		fmt.Fprintln(w, "    Or download from https://www.google.com/chrome/")
	default:
		fmt.Fprintln(w, "    Download Google Chrome from https://www.google.com/chrome/")
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "    Alternatively, set SOCIAL_CAROUSEL_CHROME_PATH to an existing Chrome binary.")
}
