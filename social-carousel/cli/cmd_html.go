package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// runHTML renders the carousel to standalone HTML files (one per slide)
// WITHOUT invoking chromedp / headless Chrome. Useful when:
//
//   - Chrome / Chromium is not installed (e.g. WSL without the package)
//   - The user wants to preview slides in their own browser before
//     committing to a PNG render
//   - The user wants to "print to PDF" from their browser (legacy
//     workflow that worked at LeaderPro: open HTML → File → Save as PDF)
//
// Output layout (mirrors what `render` produces internally under _html/):
//
//	<outDir>/
//	  base.css
//	  assets/fonts/*.woff2  *.ttf
//	  slide-01.html ... slide-NN.html
//	  index.html              ← all slides in one scrollable page
//
// Each slide HTML is self-contained: opens directly with file:// or via
// `\\wsl.localhost\Ubuntu\<path>` from Windows.
func runHTML(args []string, stdout, stderr io.Writer) (int, error) {
	var inputPath, outDir string
	force := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--out":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--out requires a directory")
				return exitInputErr, errInvalidUsage
			}
			outDir = args[i+1]
			i++
		case "--force":
			force = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "html — render the carousel to standalone HTML (no Chrome required).")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  social-carousel html <input.yaml> [--out DIR] [--force]")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Produces one HTML file per slide + an index.html that scrolls through")
			fmt.Fprintln(stdout, "all of them. Open in any browser; print → \"Save as PDF\" for a paper-")
			fmt.Fprintln(stdout, "carousel deliverable without invoking headless Chrome.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "For automated PNG/PDF generation, use `render` (requires Chrome).")
			return exitOK, nil
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintln(stderr, "unknown flag:", a)
				return exitInputErr, errInvalidUsage
			}
			if inputPath != "" {
				fmt.Fprintln(stderr, "only one input YAML at a time")
				return exitInputErr, errInvalidUsage
			}
			inputPath = a
		}
	}
	if inputPath == "" {
		fmt.Fprintln(stderr, "html: missing input YAML")
		return exitInputErr, errInvalidUsage
	}

	c, err := loadCarousel(inputPath)
	if err != nil {
		fmt.Fprintln(stderr, "html:", err)
		return exitUnknownErr, err
	}
	theme, err := loadTheme(c.Theme)
	if err != nil {
		fmt.Fprintln(stderr, "html: load theme:", err)
		return exitUnknownErr, err
	}

	// Lint unless --force; same gate as `render`. The HTML still gets
	// produced even on warn-only, but errors block by default.
	if !force {
		report := LintCarousel(c, theme)
		if report.ErrCount > 0 {
			_, _ = printCheckText(report, false, stderr)
			fmt.Fprintf(stderr, "\nhtml blocked: %d error(s). Use --force to ignore.\n", report.ErrCount)
			return exitLintFailed, nil
		}
	}

	// Resolve outDir. Default: <input>.html/
	if outDir == "" {
		base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		outDir = filepath.Join(filepath.Dir(inputPath), base+".html")
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return exitUnknownErr, err
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil {
		return exitUnknownErr, err
	}

	// Copy base.css + fonts into outDir, then render each slide HTML
	// using the same helpers the chromedp pipeline uses internally.
	if err := extractEmbeddedAssets(absOut); err != nil {
		fmt.Fprintln(stderr, "html: extract assets:", err)
		return exitUnknownErr, err
	}

	spec := resolvePlatform(c)

	var paths []string
	for i, slide := range c.Slides {
		p, err := renderSlideHTML(absOut, i, slide, c, theme, spec)
		if err != nil {
			fmt.Fprintln(stderr, "html: render slide:", err)
			return exitUnknownErr, err
		}
		paths = append(paths, p)
	}

	// Generate index.html that stacks every slide vertically with a
	// small navigation strip — opens directly in any browser, even from
	// Windows via \\wsl.localhost\Ubuntu\<path>.
	if err := writeHTMLIndex(absOut, paths, c, spec); err != nil {
		fmt.Fprintln(stderr, "html: write index:", err)
		return exitUnknownErr, err
	}

	fmt.Fprintf(stdout, "Generated %d slides in %s\n", len(paths), absOut)
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Open in your browser:")
	fmt.Fprintf(stdout, "  file://%s/index.html\n", absOut)
	if isWSL() {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "From Windows, paste in the address bar:")
		winPath := strings.ReplaceAll(absOut, "/", `\`)
		fmt.Fprintf(stdout, "  \\\\wsl.localhost\\%s%s\\index.html\n", wslDistro(), winPath)
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "To save as PDF: open in browser → Print → Destination: \"Save as PDF\"")
	fmt.Fprintln(stdout, "  (set margins to None and paper size to 1080×1350 px for fidelity).")

	if c.CaptionSeed != "" {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "Caption seed:")
		fmt.Fprintln(stdout, c.CaptionSeed)
	}
	if len(c.Hashtags) > 0 {
		fmt.Fprintln(stdout, "Hashtags:", strings.Join(c.Hashtags, " "))
	}
	return exitOK, nil
}

// writeHTMLIndex writes an index.html that embeds every slide as an
// <iframe> stacked vertically. Iframes preserve the slide's own CSS
// scope so the index page doesn't inherit slide layouts.
func writeHTMLIndex(outDir string, slidePaths []string, c *Carousel, spec PlatformSpec) error {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="UTF-8">
<title>`)
	b.WriteString(htmlEscape(c.Handle))
	b.WriteString(` — Carousel Preview</title>
<style>
  body { margin: 0; background: #1a1a2e; color: #94a3b8; font-family: system-ui, sans-serif; }
  header { padding: 24px 32px; display: flex; justify-content: space-between; align-items: center;
           position: sticky; top: 0; background: rgba(26,26,46,.95); backdrop-filter: blur(8px); z-index: 10; }
  header h1 { font-size: 18px; font-weight: 600; color: #e2e8f0; }
  header .meta { font-size: 14px; opacity: .7; }
  main { padding: 32px; display: flex; flex-direction: column; gap: 32px; align-items: center; }
  .slide-wrap { background: white; border-radius: 12px; overflow: hidden;
                box-shadow: 0 12px 40px rgba(0,0,0,.4); position: relative; }
  .slide-wrap iframe { display: block; border: 0; }
  .slide-wrap .num { position: absolute; top: 16px; left: 16px; background: rgba(0,0,0,.6);
                     color: white; padding: 4px 12px; border-radius: 999px; font-size: 13px; font-weight: 600;
                     z-index: 1; pointer-events: none; }
</style>
</head>
<body>
<header>
  <h1>`)
	b.WriteString(htmlEscape(c.Handle))
	b.WriteString(` — Carousel Preview</h1>
  <div class="meta">`)
	fmt.Fprintf(&b, "%d slides · %s · %dx%d", len(slidePaths), spec.Name, spec.Width, spec.Height)
	b.WriteString(`</div>
</header>
<main>
`)
	// Each iframe sized to half-scale of the canvas so the preview fits
	// on a typical desktop screen. The actual slide HTML still defines
	// 1080×1350 internally; iframe just clips the viewport.
	halfW := spec.Width / 2
	halfH := spec.Height / 2
	for i, p := range slidePaths {
		rel := filepath.Base(p)
		fmt.Fprintf(&b, `  <div class="slide-wrap">
    <span class="num">%d / %d</span>
    <iframe src="%s" width="%d" height="%d" style="transform: scale(.5); transform-origin: 0 0; width: %dpx; height: %dpx;"></iframe>
  </div>
`, i+1, len(slidePaths), rel, spec.Width, spec.Height, halfW*2, halfH*2)
		_ = rel
	}
	b.WriteString(`</main>
</body>
</html>
`)
	return os.WriteFile(filepath.Join(outDir, "index.html"), []byte(b.String()), 0o644)
}

// htmlEscape is a tiny escape for the few attributes the index.html
// injects from carousel config. (Slide bodies go through Go html/template
// which does proper escaping; this function only covers handle/title.)
func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}

// isWSL returns true when running under WSL. Used to emit the Windows-side
// path hint in the `html` command output.
func isWSL() bool {
	if v := os.Getenv("WSL_DISTRO_NAME"); v != "" {
		return true
	}
	b, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(b)), "microsoft")
}

// wslDistro returns the active WSL distro name, fallback "Ubuntu".
func wslDistro() string {
	if v := os.Getenv("WSL_DISTRO_NAME"); v != "" {
		return v
	}
	return "Ubuntu"
}
