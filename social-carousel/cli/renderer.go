package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// highlightRe matches **word** pairs (markdown-style emphasis). Used by the
// "highlight" template func to wrap matched runs in <span class="keyword-accent">.
var highlightRe = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)

// highlightHTML converts **text** runs to <span class="keyword-accent">text</span>
// so authors can stress key words in any user-facing prose field. The result
// is marked as template.HTML — the input is first HTML-escaped to keep the
// renderer safe against ill-formed YAML content.
func highlightHTML(s string) template.HTML {
	escaped := template.HTMLEscapeString(s)
	out := highlightRe.ReplaceAllString(escaped, `<span class="keyword-accent">$1</span>`)
	return template.HTML(out)
}

// plainText converts **text** runs to plain text (stripping the markers
// without adding any styling). Used on slides where the layout primitive
// already carries the accent role (big-number's number, cta-box bg,
// quote's decorative mark) — Research D §3: "the accent appears only on
// keywords, numbers and CTAs — never as a recurring background." Two
// accent roles per slide saturate; one role + plain emphasis reads
// cleanly. This is the "accent budget" enforcer.
func plainText(s string) template.HTML {
	escaped := template.HTMLEscapeString(s)
	out := highlightRe.ReplaceAllString(escaped, "$1")
	return template.HTML(out)
}

// wordCount returns the number of whitespace-separated tokens in s, ignoring
// **emphasis** markers. Used by templates that pick a font-size class based
// on copy length.
func wordCount(s string) int {
	stripped := highlightRe.ReplaceAllString(s, "$1")
	return len(strings.Fields(stripped))
}

// RenderOptions controls optional behaviour of the render pipeline.
type RenderOptions struct {
	// ForcePDF forces PDF output even for Instagram (PNG-default) targets.
	ForcePDF bool
	// ChromePath is the absolute path to the Chrome/Chromium binary.
	// If empty, findChrome() is called to locate it automatically.
	ChromePath string
	// KeepHTML prevents deletion of the intermediate _html/ directory.
	// Useful for debugging template issues or running preview alongside.
	KeepHTML bool
}

// RenderResult contains the paths of all artifacts produced by Render.
// All paths are absolute.
type RenderResult struct {
	PNGFiles []string // one per slide, in slide order
	PDFFile  string   // non-empty only when PDF was generated
	HTMLDir  string   // path to _html/ directory (may have been removed if !KeepHTML)
}

// slideHTMLCtx is the per-slide row inside RenderData.Slides. It matches
// the schema expected by templates/carousel.html.tmpl and the footer
// partial — pre-resolved values, no pointer-bool gymnastics in the template.
type slideHTMLCtx struct {
	Index           int     // 1-based
	Total           int     // total slides in the carousel (always 1 in single-slide render)
	Layout          string  // for data-layout attribute and dispatch
	Slide           Slide   // raw slide payload
	Carousel        *Carousel
	Theme           *Theme
	ShowSlideNumber bool
	// ToneClass is "tone-" + Slide.Tone when Tone is non-empty, else "".
	// Used by templates as a CSS class hook for tone-specific overrides
	// that complement the data-tone attribute on the .slide root.
	ToneClass string
}

// renderData mirrors the doc-comment header at the top of carousel.html.tmpl:
//
//	.Carousel  *Carousel
//	.Theme     *Theme
//	.Spec      PlatformSpec
//	.Slides    []slideHTMLCtx
//	.BaseCSS   template.CSS
type renderData struct {
	Carousel *Carousel
	Theme    *Theme
	Spec     PlatformSpec
	Slides   []slideHTMLCtx
	BaseCSS  template.CSS
}

// Render executes the full pipeline:
//
//  1. Resolve PlatformSpec from c.Platform.
//  2. Render each slide to an HTML file under outDir/_html/.
//  3. Launch headless Chrome (one cold start) and capture a PNG per slide.
//  4. If PDF output is requested, combine PNGs with pdfcpu.
//  5. Clean up _html/ unless opts.KeepHTML is true.
//
// All returned paths in RenderResult are absolute.
func Render(c *Carousel, theme *Theme, outDir string, opts RenderOptions) (*RenderResult, error) {
	spec := resolvePlatform(c)

	// Resolve Chrome binary.
	//
	// Order: explicit --chrome flag → findChrome (env / system / CFT
	// cache) → auto-download CFT. This mirrors what Puppeteer does so
	// the first render after install is a self-contained "fetch + render"
	// instead of failing with "install Chrome first".
	chromePath := opts.ChromePath
	if chromePath == "" {
		if p, err := findChrome(); err == nil {
			chromePath = p
		} else {
			bin, dlErr := ensureCFT(os.Stderr)
			if dlErr != nil {
				return nil, fmt.Errorf("render: chrome not found and auto-download failed: %w", dlErr)
			}
			chromePath = bin
		}
	}

	// Ensure output directory exists.
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return nil, fmt.Errorf("render: resolve outDir: %w", err)
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil {
		return nil, fmt.Errorf("render: create outDir: %w", err)
	}

	// Intermediate HTML directory.
	htmlDir := filepath.Join(absOut, "_html")
	if err := os.MkdirAll(htmlDir, 0o755); err != nil {
		return nil, fmt.Errorf("render: create _html dir: %w", err)
	}

	// Extract embedded assets (fonts, base.css) into _html/ so that
	// file:// URIs resolve correctly when Chrome loads the slides.
	if err := extractEmbeddedAssets(htmlDir); err != nil {
		return nil, fmt.Errorf("render: extract assets: %w", err)
	}

	// ----------------------------------------------------------------
	// Step 1 — render HTML files for every slide
	// ----------------------------------------------------------------
	htmlPaths := make([]string, len(c.Slides))
	for i, slide := range c.Slides {
		htmlPath, err := renderSlideHTML(htmlDir, i, slide, c, theme, spec)
		if err != nil {
			return nil, fmt.Errorf("render: slide %d html: %w", i+1, err)
		}
		htmlPaths[i] = htmlPath
	}

	// ----------------------------------------------------------------
	// Step 2 — launch Chrome allocator (one cold start)
	// ----------------------------------------------------------------
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.Flag("allow-file-access-from-files", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.ExecPath(chromePath),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	// Warm the browser with an empty navigate so the first slide doesn't
	// pay the full cold-start penalty inside the per-slide timeout.
	if err := chromedp.Run(browserCtx); err != nil {
		return nil, fmt.Errorf("render: browser warm-up: %w", err)
	}

	// ----------------------------------------------------------------
	// Step 3 — capture PNG per slide, reusing the browser instance
	// ----------------------------------------------------------------
	pngPaths := make([]string, len(c.Slides))
	for i, htmlPath := range htmlPaths {
		absHTML, err := filepath.Abs(htmlPath)
		if err != nil {
			return nil, fmt.Errorf("render: slide %d abs path: %w", i+1, err)
		}
		fileURI := "file://" + absHTML

		pngPath := filepath.Join(absOut, fmt.Sprintf("slide-%02d.png", i+1))

		if err := captureSlide(browserCtx, fileURI, pngPath, spec, i+1); err != nil {
			return nil, fmt.Errorf("render: capture slide %d: %w", i+1, err)
		}
		pngPaths[i] = pngPath
	}

	// ----------------------------------------------------------------
	// Step 4 — optional PDF
	// ----------------------------------------------------------------
	var pdfPath string
	if spec.OutputFormat == "pdf" || opts.ForcePDF {
		pdfPath = filepath.Join(absOut, "carousel.pdf")
		if err := combinePDF(pngPaths, pdfPath, spec); err != nil {
			// PDF failure is non-fatal: PNGs are already written.
			// Log the warning but do not return an error.
			fmt.Fprintf(os.Stderr, "social-carousel: warning: PDF generation failed: %v\n", err)
			pdfPath = ""
		}
	}

	// ----------------------------------------------------------------
	// Step 5 — cleanup
	// ----------------------------------------------------------------
	if !opts.KeepHTML {
		if err := os.RemoveAll(htmlDir); err != nil {
			// Non-fatal: log and continue.
			fmt.Fprintf(os.Stderr, "social-carousel: warning: cleanup _html: %v\n", err)
		}
		htmlDir = "" // signal that it was removed
	}

	return &RenderResult{
		PNGFiles: pngPaths,
		PDFFile:  pdfPath,
		HTMLDir:  htmlDir,
	}, nil
}

// captureSlide navigates to a slide's file:// URI and captures a retina
// screenshot using the platform's DeviceScale factor.
//
// Each call runs inside its own child context with a 60-second timeout.
// On failure it retries once before returning an error.
func captureSlide(browserCtx context.Context, fileURI, pngPath string, spec PlatformSpec, slideNum int) error {
	capture := func() error {
		ctx, cancel := context.WithTimeout(browserCtx, 60*time.Second)
		defer cancel()

		slideCtx, slideCancel := chromedp.NewContext(ctx)
		defer slideCancel()

		var buf []byte
		err := chromedp.Run(slideCtx,
			chromedp.EmulateViewport(int64(spec.Width), int64(spec.Height),
				chromedp.EmulateScale(spec.DeviceScale)),
			chromedp.Navigate(fileURI),
			chromedp.WaitVisible("body", chromedp.ByQuery),
			// Wait for fonts to finish loading before screenshotting.
			chromedp.Evaluate(`document.fonts.ready`, nil),
			chromedp.CaptureScreenshot(&buf),
		)
		if err != nil {
			return err
		}
		if len(buf) == 0 {
			return fmt.Errorf("empty screenshot for slide %d", slideNum)
		}
		return os.WriteFile(pngPath, buf, 0o644)
	}

	// First attempt.
	if err := capture(); err != nil {
		// Retry once.
		if retryErr := capture(); retryErr != nil {
			return fmt.Errorf("slide %d (after retry): %w", slideNum, retryErr)
		}
	}
	return nil
}

// renderSlideHTML generates a self-contained HTML file for one slide by
// executing the carousel base template with the slide-specific data.
// Returns the absolute path to the written file.
func renderSlideHTML(
	htmlDir string,
	idx int,
	slide Slide,
	carousel *Carousel,
	theme *Theme,
	spec PlatformSpec,
) (string, error) {
	// Parse all templates into one tree: base + footer + every layout
	// partial. carousel.html.tmpl dispatches at render time via
	// {{template "layout-<name>"}}, so all layouts must be available.
	tmpl, err := parseCarouselTemplate()
	if err != nil {
		return "", err
	}

	// Load base.css from embedded FS (inlined into <style>).
	cssBytes, err := templatesFS.ReadFile("templates/base.css")
	if err != nil {
		return "", fmt.Errorf("read base.css: %w", err)
	}

	showNum := true
	if carousel.ShowSlideNumber != nil {
		showNum = *carousel.ShowSlideNumber
	}
	total := len(carousel.Slides)
	layoutName := slide.Layout
	if layoutName == "" {
		layoutName = "text"
	}

	toneClass := ""
	if slide.Tone != "" {
		toneClass = "tone-" + slide.Tone
	}

	data := renderData{
		Carousel: carousel,
		Theme:    theme,
		Spec:     spec,
		BaseCSS:  template.CSS(cssBytes),
		Slides: []slideHTMLCtx{{
			Index:           idx + 1,
			Total:           total,
			Layout:          layoutName,
			Slide:           slide,
			Carousel:        carousel,
			Theme:           theme,
			ShowSlideNumber: showNum,
			ToneClass:       toneClass,
		}},
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template for slide %d: %w", idx+1, err)
	}

	outPath := filepath.Join(htmlDir, fmt.Sprintf("slide-%02d.html", idx+1))
	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("write slide HTML %s: %w", outPath, err)
	}
	return outPath, nil
}

// parseCarouselTemplate loads carousel.html.tmpl + the footer partial +
// every layout partial into a single template tree, registering the
// helper funcs (add1) the layouts depend on.
func parseCarouselTemplate() (*template.Template, error) {
	funcs := template.FuncMap{
		"add1":      func(i int) int { return i + 1 },
		"highlight": highlightHTML,
		"plain":     plainText,
		"wordCount": wordCount,
	}

	baseTmplData, err := templatesFS.ReadFile("templates/carousel.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("read carousel.html.tmpl: %w", err)
	}
	tmpl, err := template.New("carousel").Funcs(funcs).Parse(string(baseTmplData))
	if err != nil {
		return nil, fmt.Errorf("parse carousel.html.tmpl: %w", err)
	}

	// Layout partials and the footer partial all live in templates/layouts/.
	entries, err := templatesFS.ReadDir("templates/layouts")
	if err != nil {
		return nil, fmt.Errorf("read layouts dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".html.tmpl") {
			continue
		}
		data, err := templatesFS.ReadFile("templates/layouts/" + name)
		if err != nil {
			return nil, fmt.Errorf("read layout %s: %w", name, err)
		}
		if _, err := tmpl.Parse(string(data)); err != nil {
			return nil, fmt.Errorf("parse layout %s: %w", name, err)
		}
	}
	return tmpl, nil
}

// extractEmbeddedAssets copies the templates/fonts/ subtree and base.css
// from the embedded FS into htmlDir so that file:// URIs work correctly
// when Chrome loads the slide HTML files.
//
// The fonts are written to htmlDir/assets/fonts/, matching the @font-face
// src paths expected in base.css (url('./assets/fonts/…')).
func extractEmbeddedAssets(htmlDir string) error {
	// Copy base.css
	if err := extractEmbedFile("templates/base.css", filepath.Join(htmlDir, "base.css")); err != nil {
		return fmt.Errorf("extract base.css: %w", err)
	}

	// Copy fonts subtree
	fontsDir := filepath.Join(htmlDir, "assets", "fonts")
	if err := os.MkdirAll(fontsDir, 0o755); err != nil {
		return fmt.Errorf("create fonts dir: %w", err)
	}

	entries, err := templatesFS.ReadDir("templates/fonts")
	if err != nil {
		// Fonts directory missing — not a hard error during development
		// (subagent T may not have added fonts yet).
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := "templates/fonts/" + e.Name()
		dst := filepath.Join(fontsDir, e.Name())
		if err := extractEmbedFile(src, dst); err != nil {
			return fmt.Errorf("extract font %s: %w", e.Name(), err)
		}
	}
	return nil
}

// extractEmbedFile reads one file from templatesFS and writes it to disk.
// It is a no-op if the destination already exists and is non-empty.
func extractEmbedFile(embPath, dstPath string) error {
	// Skip if already extracted (e.g. repeated preview calls).
	if info, err := os.Stat(dstPath); err == nil && info.Size() > 0 {
		return nil
	}
	data, err := templatesFS.ReadFile(embPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, 0o644)
}
