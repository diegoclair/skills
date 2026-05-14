package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/lybel-app/skills/confluence-docs/cli/adf"
)

func runPageCreate(args []string, stdout, stderr io.Writer) (int, error) {
	var spaceID, parentID, title, markdownFile, adfFile string
	var fullWidth, fixedWidth bool

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--space-id":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--space-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			spaceID = remaining[i+1]
			i++
		case "--parent-id":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--parent-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			parentID = remaining[i+1]
			i++
		case "--title":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--title requires a value")
				return exitInputErr, errInvalidUsage
			}
			title = remaining[i+1]
			i++
		case "--markdown":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--markdown requires a file path")
				return exitInputErr, errInvalidUsage
			}
			markdownFile = remaining[i+1]
			i++
		case "--adf":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--adf requires a file path")
				return exitInputErr, errInvalidUsage
			}
			adfFile = remaining[i+1]
			i++
		case "--full-width":
			fullWidth = true
		case "--fixed-width":
			fixedWidth = true
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if spaceID == "" {
		fmt.Fprintln(stderr, "page create: --space-id is required")
		return exitInputErr, errInvalidUsage
	}
	if parentID == "" {
		fmt.Fprintln(stderr, "page create: --parent-id is required")
		return exitInputErr, errInvalidUsage
	}
	if title == "" {
		fmt.Fprintln(stderr, "page create: --title is required")
		return exitInputErr, errInvalidUsage
	}
	if markdownFile != "" && adfFile != "" {
		fmt.Fprintln(stderr, "page create: specify either --markdown or --adf, not both")
		return exitInputErr, errInvalidUsage
	}
	if fullWidth && fixedWidth {
		fmt.Fprintln(stderr, "page create: --full-width and --fixed-width are mutually exclusive")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	var result *adf.PageCreateResult

	if markdownFile != "" {
		src, err := os.ReadFile(markdownFile)
		if err != nil {
			fmt.Fprintln(stderr, "reading markdown:", err)
			return exitInputErr, err
		}
		if adf.RequiresStorageFormat(string(src)) {
			// Markdown contains :::properties or other storage-only macros.
			// Convert to Confluence storage XML and upload with representation=storage.
			// Use client-aware conversion so @handle mentions in :::properties
			// are resolved to real Confluence user mention links.
			storageBody, sErr := adf.MarkdownToStorageWithClient(src, client)
			if sErr != nil {
				fmt.Fprintln(stderr, "convert markdown to storage:", sErr)
				return exitParseErr, sErr
			}
			result, err = client.CreatePageStorage(spaceID, parentID, title, storageBody)
		} else {
			doc, cErr := adf.Convert(src)
			if cErr != nil {
				fmt.Fprintln(stderr, "parse markdown:", cErr)
				return exitParseErr, cErr
			}
			result, err = client.CreatePage(spaceID, parentID, title, &doc)
		}
	} else if adfFile != "" {
		adfBytes, err := os.ReadFile(adfFile)
		if err != nil {
			fmt.Fprintln(stderr, "reading ADF:", err)
			return exitInputErr, err
		}
		doc, uErr := adf.UnmarshalDoc(adfBytes)
		if uErr != nil {
			fmt.Fprintln(stderr, "invalid ADF:", uErr)
			return exitParseErr, uErr
		}
		result, err = client.CreatePage(spaceID, parentID, title, &doc)
	} else {
		result, err = client.CreatePage(spaceID, parentID, title, nil)
	}

	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownErr, err
	}

	// Apply page appearance (full-width / fixed-width) if requested.
	if fullWidth || fixedWidth {
		appearance := adf.PageAppearanceFullWidth
		if fixedWidth {
			appearance = adf.PageAppearanceFixedWidth
		}
		if appErr := client.SetPageAppearance(result.ID, appearance); appErr != nil {
			fmt.Fprintf(stderr, "warning: page created but appearance could not be set: %v\n", appErr)
		}
	}

	out := map[string]string{
		"pageId": result.ID,
		"title":  result.Title,
		"url":    client.PageURL(result.Links.WebUI),
	}
	outBytes, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintln(stdout, string(outBytes))
	return exitOK, nil
}
