// social-carousel is a token-efficient CLI for generating viral Instagram /
// LinkedIn carousels from a small YAML brief. Designed to drive Claude
// (and other LLM agents) without round-trips to paid SaaS APIs.
//
// Inputs: a YAML file describing the slides + a theme reference.
// Outputs: PNGs (Instagram) or a combined PDF (LinkedIn), rendered locally
// via headless Chrome (chromedp). Zero per-image cost, zero account.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// version is injected at build time via -ldflags "-X main.version=..."
var version = "v0.1.0"

const (
	exitOK              = 0
	exitInputErr        = 2
	exitUnknownErr      = 3
	exitLintFailed      = 4  // `check` found errors
	exitUpdateAvailable = 10 // `update --check` and there is one
)

var errInvalidUsage = errors.New("invalid usage")

const helpText = `social-carousel — token-efficient carousel generator for LLM agents.

USAGE:
  social-carousel render   <input.yaml> [--out DIR] [--pdf] [--force]
  social-carousel html     <input.yaml> [--out DIR] [--force]
  social-carousel check    <input.yaml> [--json]
  social-carousel new      <kind> [--out FILE]
  social-carousel preview  <input.yaml> [--port 7777]
  social-carousel theme    VERB [flags]
  social-carousel setup    [--check]
  social-carousel update   [--check]
  social-carousel --version | --help

RENDER:
  Renders the carousel to PNG (Instagram) or PDF (LinkedIn) using a
  headless Chrome instance via chromedp. Output dir defaults to a sibling
  of the input YAML named "<input>.out/". --pdf forces PDF output even
  for Instagram targets. --force skips the linter gate.

HTML:
  Renders the carousel to standalone HTML files (one per slide + an
  index.html that scrolls through all of them). Does NOT invoke Chrome —
  pure Go template execution. Open the generated HTML in any browser to
  preview, or use the browser's "Save as PDF" to export. Use this when
  Chrome is not installed, when you want fast iteration on copy/theme,
  or when you prefer a PDF over PNGs and want to avoid the chromedp dep.

CHECK:
  Runs the linter (32 viral-carousel rules: U1–U12 universal + per-layout).
  Returns exit 0 if clean, 4 if errors. Use --json for machine-readable output.
  Render is blocked when check fails unless --force is set on render.

NEW:
  Scaffold a YAML stub for a given kind. Kinds: "listicle", "case-study",
  "framework", "comparison", "story", "data-drop". Writes to stdout
  unless --out is given.

PREVIEW:
  Serves the rendered HTML on http://localhost:PORT for visual iteration
  without invoking chromedp. Faster feedback loop while drafting copy.

THEME:
  theme list                    List shipped presets + any custom themes.
  theme show <name>             Print theme YAML (preset or custom).
  theme create --from <yaml>    Author a new custom theme by extracting
                                tokens from a rendered carousel you liked.
                                Saved to ~/.config/social-carousel/themes/.

SETUP:
  Detects Google Chrome / Chromium on PATH, validates it can run headless,
  and writes ~/.config/social-carousel/config.yaml. --check prints status
  without modifying anything.

UPDATE:
  Self-update. Resolves the latest release filtered by tag prefix
  "carousel-v*" and shells out to install.sh / install.ps1.

EXIT CODES:
  0   success
  2   bad usage / missing argument
  3   runtime error (chrome not found, IO failure, ...)
  4   linter found errors and render was not --force
  10  update --check: a newer version is available
`

func main() {
	code, err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "social-carousel:", err)
	}
	os.Exit(code)
}

// run is the testable entry point. Dispatches to the matching cmd_*.go.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprint(stderr, helpText)
		return exitInputErr, errInvalidUsage
	}
	switch args[0] {
	case "-h", "--help":
		fmt.Fprint(stdout, helpText)
		return exitOK, nil
	case "-v", "--version":
		fmt.Fprintln(stdout, "social-carousel", version)
		return exitOK, nil
	case "render":
		return runRender(args[1:], stdout, stderr)
	case "html":
		return runHTML(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "new":
		return runNew(args[1:], stdout, stderr)
	case "preview":
		return runPreview(args[1:], stdout, stderr)
	case "theme":
		return runTheme(args[1:], stdout, stderr)
	case "setup":
		return runSetup(args[1:], stdout, stderr)
	case "update":
		return runUpdate(args[1:], stdout, stderr)
	case "postinstall":
		return runPostinstall(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, "unknown command:", args[0])
	fmt.Fprint(stderr, helpText)
	return exitInputErr, errInvalidUsage
}
