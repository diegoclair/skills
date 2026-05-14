package main

import (
	"fmt"
	"io"
	"os"

	"github.com/lybel-app/skills/confluence-docs/cli/adf"
)

func runPageUpload(args []string, stdout, stderr io.Writer) (int, error) {
	var pageID, adfFile, markdownFile, title, message string
	var dryRun, fullWidth, fixedWidth bool

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
		case "--adf":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--adf requires a file path")
				return exitInputErr, errInvalidUsage
			}
			adfFile = remaining[i+1]
			i++
		case "--markdown":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--markdown requires a file path")
				return exitInputErr, errInvalidUsage
			}
			markdownFile = remaining[i+1]
			i++
		case "--title":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--title requires a value")
				return exitInputErr, errInvalidUsage
			}
			title = remaining[i+1]
			i++
		case "--message":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--message requires a value")
				return exitInputErr, errInvalidUsage
			}
			message = remaining[i+1]
			i++
		case "--dry-run":
			dryRun = true
		case "--full-width":
			fullWidth = true
		case "--fixed-width":
			fixedWidth = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "page upload — replace the entire body of an existing page.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  --page-id ID         target page (required)")
			fmt.Fprintln(stdout, "  --adf FILE           new body as ADF JSON")
			fmt.Fprintln(stdout, "  --markdown FILE      new body as markdown (converted to ADF)")
			fmt.Fprintln(stdout, "                       — exactly one of --adf or --markdown is required")
			fmt.Fprintln(stdout, "  --title TITLE        new title (omit to preserve current title)")
			fmt.Fprintln(stdout, "  --message MSG        version comment")
			fmt.Fprintln(stdout, "  --dry-run            print preview without writing")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Use this verb for full-body replacement (no section-level diffing).")
			fmt.Fprintln(stdout, "For surgical edits, prefer `page apply --replace-section` etc.")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page upload: --page-id is required")
		return exitInputErr, errInvalidUsage
	}
	if adfFile == "" && markdownFile == "" {
		fmt.Fprintln(stderr, "page upload: one of --adf or --markdown is required")
		return exitInputErr, errInvalidUsage
	}
	if adfFile != "" && markdownFile != "" {
		fmt.Fprintln(stderr, "page upload: specify either --adf or --markdown, not both")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	var updateErr error
	if adfFile != "" {
		adfBytes, rdErr := os.ReadFile(adfFile)
		if rdErr != nil {
			fmt.Fprintln(stderr, "reading ADF file:", rdErr)
			return exitInputErr, rdErr
		}
		parsed, pErr := adf.UnmarshalDoc(adfBytes)
		if pErr != nil {
			fmt.Fprintln(stderr, "invalid ADF:", pErr)
			return exitParseErr, pErr
		}
		updateErr = client.UpdatePage(pageID, title, 0, parsed, message, dryRun, stderr)
	} else {
		mdBytes, rdErr := os.ReadFile(markdownFile)
		if rdErr != nil {
			fmt.Fprintln(stderr, "reading markdown:", rdErr)
			return exitInputErr, rdErr
		}
		if adf.RequiresStorageFormat(string(mdBytes)) {
			// Markdown contains :::properties or other storage-only macros.
			// Use client-aware conversion so @handle mentions in :::properties
			// are resolved to real Confluence user mention links.
			storageBody, sErr := adf.MarkdownToStorageWithClient(mdBytes, client)
			if sErr != nil {
				fmt.Fprintln(stderr, "convert markdown to storage:", sErr)
				return exitParseErr, sErr
			}
			updateErr = client.UpdatePageStorage(pageID, title, 0, storageBody, message, dryRun, stderr)
		} else {
			converted, cErr := adf.Convert(mdBytes)
			if cErr != nil {
				fmt.Fprintln(stderr, "parse markdown:", cErr)
				return exitParseErr, cErr
			}
			updateErr = client.UpdatePage(pageID, title, 0, converted, message, dryRun, stderr)
		}
	}

	if updateErr != nil {
		fmt.Fprintln(stderr, "error:", updateErr)
		return exitUnknownErr, updateErr
	}

	if !dryRun {
		// Apply page appearance (full-width / fixed-width) if requested.
		if fullWidth || fixedWidth {
			appearance := adf.PageAppearanceFullWidth
			if fixedWidth {
				appearance = adf.PageAppearanceFixedWidth
			}
			if appErr := client.SetPageAppearance(pageID, appearance); appErr != nil {
				fmt.Fprintf(stderr, "warning: page updated but appearance could not be set: %v\n", appErr)
			}
		}
		// Auto-refresh the home cache if this write touched the Home page.
		refreshHomeCacheAfterWrite(pageID, client, stderr)
		fmt.Fprintf(stdout, `{"status":"ok","pageId":%q}`+"\n", pageID)
	}
	return exitOK, nil
}
