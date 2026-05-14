package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/lybel-app/skills/pkg/atlassian/adf"
)

func runPageGet(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		pageID, format, outputFile string
		section                    string
		atLevel                    int
		quiet                      bool
	)

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--page-id":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--page-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			pageID = remaining[i+1]
			i++
		case "--format":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--format requires a value (adf|text|markdown|export_view|html|view|storage)")
				return exitInputErr, errInvalidUsage
			}
			format = remaining[i+1]
			i++
		case "--output":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--output requires a file path")
				return exitInputErr, errInvalidUsage
			}
			outputFile = remaining[i+1]
			i++
		case "--section":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--section requires a heading")
				return exitInputErr, errInvalidUsage
			}
			section = remaining[i+1]
			i++
		case "--at-level":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--at-level requires a value (1-6)")
				return exitInputErr, errInvalidUsage
			}
			n, lvErr := strconv.Atoi(remaining[i+1])
			if lvErr != nil || n < 1 || n > 6 {
				fmt.Fprintln(stderr, "--at-level must be an integer between 1 and 6")
				return exitInputErr, errInvalidUsage
			}
			atLevel = n
			i++
		case "--quiet":
			quiet = true
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page get: --page-id is required")
		return exitInputErr, errInvalidUsage
	}
	if format == "" {
		// Default: when slicing a section, the caller almost always wants
		// readable prose (text). Otherwise, keep the historical ADF default.
		if section != "" {
			format = "text"
		} else {
			format = "adf"
		}
	}

	// "text" and "markdown" are local renderings of ADF — they require
	// fetching atlas_doc_format and running adf.RenderText. They are NOT
	// API-side body formats.
	wantLocalRender := format == "text" || format == "markdown"

	// --section requires local-side slicing on the parsed ADF. The only
	// formats supported with --section are adf, text, and markdown — the
	// server-rendered formats (view/html/storage/export_view) describe the
	// whole page and can't be sliced reliably by heading.
	if section != "" {
		switch format {
		case "adf", "text", "markdown":
			// ok
		default:
			fmt.Fprintf(stderr, "--section is only supported with --format adf|text|markdown (got %q)\n", format)
			return exitInputErr, errInvalidUsage
		}
	}

	// Validate --format and resolve it to a Confluence API body-format.
	// This MUST happen before buildClient so a bad --format fails fast with
	// a clear message even when credentials are missing (matters for tests
	// and CI where no creds are configured).
	var bodyFormat, fieldName string
	switch format {
	case "adf", "text", "markdown":
		// All three need ADF from the API; "text"/"markdown" are rendered
		// locally after fetch.
		bodyFormat, fieldName = "atlas_doc_format", "atlas_doc_format"
	case "storage":
		bodyFormat, fieldName = "storage", "storage"
	case "view", "html":
		bodyFormat, fieldName = "view", "view"
	case "export_view":
		bodyFormat, fieldName = "export_view", "export_view"
	default:
		fmt.Fprintf(stderr, "unknown format %q — use adf, text, markdown, storage, view, html, or export_view\n", format)
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	meta, err := client.GetPage(pageID, bodyFormat)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownErr, err
	}

	var raw []byte
	switch fieldName {
	case "atlas_doc_format":
		raw = []byte(meta.Body.AtlasDocFormat.Value)
	case "storage":
		raw = []byte(meta.Body.Storage.Value)
	case "view":
		raw = []byte(meta.Body.View.Value)
	case "export_view":
		raw = []byte(meta.Body.ExportView.Value)
	}
	if len(raw) == 0 {
		fmt.Fprintf(stderr, "page has no %s body — try a different --format\n", fieldName)
		return exitUnknownErr, fmt.Errorf("empty %s body", fieldName)
	}

	// Compute the final output bytes based on (--section, --format).
	var out []byte
	if section != "" || wantLocalRender {
		// Both branches need the parsed ADF doc.
		doc, dErr := adf.UnmarshalDoc(raw)
		if dErr != nil {
			fmt.Fprintln(stderr, "parse ADF:", dErr)
			return exitParseErr, dErr
		}

		// Slice to a section if requested.
		target := doc
		if section != "" {
			sub, sErr := adf.SectionContent(doc, section, atLevel)
			if sErr != nil {
				// Error message embeds the heading list + shell-expansion hint
				// (from adf.sectionNotFoundError).
				fmt.Fprintln(stderr, "operation failed:", sErr)
				return exitInputErr, errInvalidUsage
			}
			target = sub
		}

		// Render into the requested format.
		switch format {
		case "adf":
			marshalled, mErr := adf.Marshal(target, false)
			if mErr != nil {
				fmt.Fprintln(stderr, "marshal:", mErr)
				return exitUnknownErr, mErr
			}
			out = marshalled
		case "text", "markdown":
			out = []byte(adf.RenderText(target))
		}
	} else {
		// No slicing, no local rendering — emit the API body as-is.
		out = raw
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, out, 0644); err != nil {
			fmt.Fprintln(stderr, "writing output:", err)
			return exitUnknownErr, err
		}
		if !quiet {
			fmt.Fprintf(stderr, "wrote %d bytes to %s\n", len(out), outputFile)
		}
	} else {
		if _, err := stdout.Write(out); err != nil {
			return exitUnknownErr, err
		}
	}
	return exitOK, nil
}
