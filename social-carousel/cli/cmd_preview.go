package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

// runPreview implements `social-carousel preview <input.yaml> [--port N]`.
//
// It renders all slides to HTML in a temp directory, then starts a local
// HTTP server that serves a scrollable index page with all slides stacked
// vertically. Faster feedback loop than invoking chromedp on every save.
//
// Blocks until the user presses Ctrl+C.
func runPreview(args []string, stdout, stderr io.Writer) (int, error) {
	// ----------------------------------------------------------------
	// Flag parsing
	// ----------------------------------------------------------------
	var (
		port      = "7777"
		inputFile string
	)

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprintln(stdout, previewHelp)
			return exitOK, nil
		case a == "--port":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "preview: --port requires a value")
				return exitInputErr, errInvalidUsage
			}
			i++
			port = args[i]
		case strings.HasPrefix(a, "--port="):
			port = strings.TrimPrefix(a, "--port=")
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "preview: unknown flag %q\n", a)
			return exitInputErr, errInvalidUsage
		default:
			if inputFile != "" {
				fmt.Fprintf(stderr, "preview: unexpected argument %q\n", a)
				return exitInputErr, errInvalidUsage
			}
			inputFile = a
		}
	}

	if inputFile == "" {
		fmt.Fprintln(stderr, "preview: input YAML file is required")
		fmt.Fprintln(stderr, previewHelp)
		return exitInputErr, errInvalidUsage
	}

	// ----------------------------------------------------------------
	// Load carousel + theme
	// ----------------------------------------------------------------
	c, err := loadCarousel(inputFile)
	if err != nil {
		fmt.Fprintln(stderr, "preview: load carousel:", err)
		return exitInputErr, err
	}
	theme, err := loadTheme(c.Theme)
	if err != nil {
		fmt.Fprintln(stderr, "preview: load theme:", err)
		return exitUnknownErr, err
	}

	// ----------------------------------------------------------------
	// Generate HTML files into a temp directory
	// ----------------------------------------------------------------
	tmpDir, err := os.MkdirTemp("", "social-carousel-preview-*")
	if err != nil {
		return exitUnknownErr, fmt.Errorf("preview: create tmpdir: %w", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	// Extract embedded assets into tmpDir so fonts and base.css resolve.
	if err := extractEmbeddedAssets(tmpDir); err != nil {
		return exitUnknownErr, fmt.Errorf("preview: extract assets: %w", err)
	}

	spec := resolvePlatform(c)

	slidePaths := make([]string, 0, len(c.Slides))
	for i, slide := range c.Slides {
		htmlPath, err := renderSlideHTML(tmpDir, i, slide, c, theme, spec)
		if err != nil {
			return exitUnknownErr, fmt.Errorf("preview: render slide %d: %w", i+1, err)
		}
		slidePaths = append(slidePaths, filepath.Base(htmlPath))
	}

	// ----------------------------------------------------------------
	// Build the index page
	// ----------------------------------------------------------------
	indexHTML, err := buildPreviewIndex(c, slidePaths, spec)
	if err != nil {
		return exitUnknownErr, fmt.Errorf("preview: build index: %w", err)
	}
	indexPath := filepath.Join(tmpDir, "index.html")
	if err := os.WriteFile(indexPath, indexHTML, 0o644); err != nil {
		return exitUnknownErr, fmt.Errorf("preview: write index: %w", err)
	}

	// ----------------------------------------------------------------
	// HTTP server
	// ----------------------------------------------------------------
	mux := http.NewServeMux()
	// Serve the whole tmpDir as a file tree.
	mux.Handle("/", http.FileServer(http.Dir(tmpDir)))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-stop
		_ = srv.Shutdown(context.Background())
	}()

	fmt.Fprintf(stdout, "Preview at: http://localhost:%s\n", port)
	fmt.Fprintln(stdout, "Press Ctrl+C to stop.")

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return exitUnknownErr, fmt.Errorf("preview: server: %w", err)
	}
	return exitOK, nil
}

// buildPreviewIndex generates an HTML page that embeds all slide iframes in
// a vertically scrollable layout. Each iframe is sized to the platform's
// logical dimensions (CSS px) so the proportions match the final export.
func buildPreviewIndex(c *Carousel, slidePaths []string, spec PlatformSpec) ([]byte, error) {
	const indexTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Preview — {{.Title}}</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    background: #1a1a2e;
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 32px 0;
    gap: 24px;
    font-family: system-ui, sans-serif;
    color: #eee;
  }
  h1 { font-size: 1.1rem; letter-spacing: .15em; color: #11C47E; margin-bottom: 8px; }
  .slide-wrapper {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }
  .slide-label {
    font-size: .75rem;
    color: #888;
    letter-spacing: .1em;
  }
  iframe {
    width: {{.Width}}px;
    height: {{.Height}}px;
    border: 1px solid #333;
    border-radius: 8px;
    background: #000;
  }
  footer {
    color: #555;
    font-size: .7rem;
    padding: 16px;
  }
</style>
</head>
<body>
<h1>{{.Title}} — {{.Platform}} — {{len .Slides}} slides</h1>
{{range $i, $path := .Slides}}
<div class="slide-wrapper">
  <div class="slide-label">Slide {{inc $i}} / {{len $.Slides}}</div>
  <iframe src="{{$path}}" frameborder="0" scrolling="no"></iframe>
</div>
{{end}}
<footer>social-carousel preview · http://localhost:{{.Port}}</footer>
</body>
</html>`

	funcMap := template.FuncMap{
		"inc": func(i int) int { return i + 1 },
		"len": func(s []string) int { return len(s) },
	}

	tmpl, err := template.New("index").Funcs(funcMap).Parse(indexTmpl)
	if err != nil {
		return nil, fmt.Errorf("parse index template: %w", err)
	}

	data := struct {
		Title    string
		Platform string
		Width    int
		Height   int
		Slides   []string
		Port     string
	}{
		Title:    carouselTitle(c),
		Platform: spec.Name,
		Width:    spec.Width,
		Height:   spec.Height,
		Slides:   slidePaths,
		Port:     "7777", // placeholder; actual port printed separately
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute index template: %w", err)
	}
	return buf.Bytes(), nil
}

// carouselTitle returns a human-readable title for the preview page,
// derived from the first slide's hook or label.
func carouselTitle(c *Carousel) string {
	if len(c.Slides) == 0 {
		return "Untitled"
	}
	first := c.Slides[0]
	if first.Hook != "" {
		if len(first.Hook) > 40 {
			return first.Hook[:40] + "…"
		}
		return first.Hook
	}
	if first.Label != "" {
		return first.Label
	}
	return "Carousel"
}

const previewHelp = `preview — serve carousel HTML on a local port for fast visual iteration.

USAGE:
  social-carousel preview <input.yaml> [--port N]

FLAGS:
  --port N      Port to listen on (default: 7777)
  -h, --help    Show this help

NOTES:
  Renders templates to HTML in a temp directory and serves them via HTTP.
  No Chrome / chromedp involved — instant reload on re-run.
  Access slides at http://localhost:PORT (scrollable index) or
  http://localhost:PORT/slide-01.html directly.`
