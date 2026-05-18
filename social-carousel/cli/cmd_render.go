package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// runRender implements the `social-carousel render <input.yaml>` command.
//
// It loads the carousel YAML, optionally runs the linter gate, resolves
// the theme, calls Render() for the HTML→PNG→PDF pipeline, and prints
// the generated artifact paths plus caption/hashtag hints to stdout.
func runRender(args []string, stdout, stderr io.Writer) (int, error) {
	// ----------------------------------------------------------------
	// Flag parsing
	// ----------------------------------------------------------------
	var (
		outDir     string
		forcePDF   bool
		force      bool // skip linter gate
		chromePath string
		keepHTML   bool
		inputFile  string
	)

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--pdf":
			forcePDF = true
		case a == "--force":
			force = true
		case a == "--keep-html":
			keepHTML = true
		case a == "-h" || a == "--help":
			fmt.Fprintln(stdout, renderHelp)
			return exitOK, nil
		case a == "--out":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "render: --out requires a directory argument")
				return exitInputErr, errInvalidUsage
			}
			i++
			outDir = args[i]
		case strings.HasPrefix(a, "--out="):
			outDir = strings.TrimPrefix(a, "--out=")
		case a == "--chrome":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "render: --chrome requires a path argument")
				return exitInputErr, errInvalidUsage
			}
			i++
			chromePath = args[i]
		case strings.HasPrefix(a, "--chrome="):
			chromePath = strings.TrimPrefix(a, "--chrome=")
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "render: unknown flag %q\n", a)
			return exitInputErr, errInvalidUsage
		default:
			if inputFile != "" {
				fmt.Fprintln(stderr, "render: unexpected argument:", a)
				return exitInputErr, errInvalidUsage
			}
			inputFile = a
		}
	}

	if inputFile == "" {
		fmt.Fprintln(stderr, "render: input YAML file is required")
		fmt.Fprintln(stderr, renderHelp)
		return exitInputErr, errInvalidUsage
	}

	// ----------------------------------------------------------------
	// Load carousel
	// ----------------------------------------------------------------
	c, err := loadCarousel(inputFile)
	if err != nil {
		fmt.Fprintln(stderr, "render: load carousel:", err)
		return exitInputErr, err
	}

	if len(c.Slides) == 0 {
		fmt.Fprintln(stderr, "render: carousel has no slides")
		return exitInputErr, errInvalidUsage
	}

	// ----------------------------------------------------------------
	// Load theme (needed before linting for contrast rules)
	// ----------------------------------------------------------------
	theme, err := loadTheme(c.Theme)
	if err != nil {
		fmt.Fprintln(stderr, "render: load theme:", err)
		return exitUnknownErr, err
	}

	// ----------------------------------------------------------------
	// Linter gate (unless --force)
	// ----------------------------------------------------------------
	if !force {
		report := LintCarousel(c, theme)
		errs := filterErrors(report)
		if len(errs) > 0 {
			fmt.Fprintln(stderr, "render: linter found errors — fix them or run with --force:")
			for _, iss := range errs {
				fmt.Fprintf(stderr, "  [%s] slide %d: %s\n", iss.Code, iss.SlideIdx+1, iss.Message)
			}
			return exitLintFailed, fmt.Errorf("linter failed with %d error(s)", len(errs))
		}
	}

	// ----------------------------------------------------------------
	// Resolve output directory
	// ----------------------------------------------------------------
	if outDir == "" {
		absInput, _ := filepath.Abs(inputFile)
		base := strings.TrimSuffix(absInput, filepath.Ext(absInput))
		outDir = base + ".out"
	}

	// ----------------------------------------------------------------
	// Render
	// ----------------------------------------------------------------
	opts := RenderOptions{
		ForcePDF:   forcePDF,
		ChromePath: chromePath,
		KeepHTML:   keepHTML,
	}

	fmt.Fprintf(stdout, "Rendering %d slides → %s\n", len(c.Slides), outDir)

	result, err := Render(c, theme, outDir, opts)
	if err != nil {
		fmt.Fprintln(stderr, "render:", err)
		return exitUnknownErr, err
	}

	// ----------------------------------------------------------------
	// Print results
	// ----------------------------------------------------------------
	fmt.Fprintln(stdout, "\nGenerated files:")
	for _, p := range result.PNGFiles {
		fmt.Fprintln(stdout, " ", p)
	}
	if result.PDFFile != "" {
		fmt.Fprintln(stdout, " ", result.PDFFile)
	}

	if c.CaptionSeed != "" {
		fmt.Fprintln(stdout, "\nCaption seed:")
		fmt.Fprintln(stdout, c.CaptionSeed)
	}

	if len(c.Hashtags) > 0 {
		fmt.Fprintln(stdout, "\nHashtags:")
		fmt.Fprintln(stdout, strings.Join(c.Hashtags, " "))
	}

	return exitOK, nil
}

// filterErrors returns only the error-severity issues from a LintReport.
// Issues with SeverityWarn are excluded and do not block rendering.
func filterErrors(report LintReport) []LintIssue {
	var errs []LintIssue
	for _, iss := range report.Issues {
		if iss.Severity == SeverityErr {
			errs = append(errs, iss)
		}
	}
	return errs
}

const renderHelp = `render — generate PNGs (or PDF) from a carousel YAML.

USAGE:
  social-carousel render <input.yaml> [flags]

FLAGS:
  --out DIR         Output directory (default: <input>.out/ alongside the YAML)
  --pdf             Force PDF output even for Instagram (PNG-default) targets
  --force           Skip the linter gate and render unconditionally
  --chrome PATH     Path to Chrome/Chromium binary (overrides auto-detect)
  --keep-html       Keep intermediate HTML files in <out>/_html/ (debug)
  -h, --help        Show this help

OUTPUTS:
  <out>/slide-01.png ... slide-NN.png   Individual slide PNGs
  <out>/carousel.pdf                     Combined PDF (LinkedIn or --pdf)
  stdout: file paths + caption_seed + hashtags`
