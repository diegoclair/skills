package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/diegoclair/skills/pkg/atlassian/adf"
)

func runPageDigest(args []string, stdout, stderr io.Writer) (int, error) {
	var pageID string
	var asJSON bool

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
		case "--json":
			asJSON = true
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page digest: --page-id is required")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	meta, err := client.GetPage(pageID, "atlas_doc_format")
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownErr, err
	}
	if meta.Body.AtlasDocFormat.Value == "" {
		fmt.Fprintln(stderr, "page has no ADF body")
		return exitUnknownErr, fmt.Errorf("empty ADF body")
	}

	doc, err := adf.UnmarshalDoc([]byte(meta.Body.AtlasDocFormat.Value))
	if err != nil {
		fmt.Fprintln(stderr, "parse ADF:", err)
		return exitParseErr, err
	}

	digest := adf.BuildDigest(doc, meta.ID, meta.Title, client.PageURL(meta.Links.WebUI), meta.Version.Number)

	if asJSON {
		out, _ := json.MarshalIndent(digest, "", "  ")
		fmt.Fprintln(stdout, string(out))
	} else {
		fmt.Fprint(stdout, digest.FormatText())
	}
	return exitOK, nil
}

// multiOp is a single batch operation read from --multi JSON or built
// internally by `page rewrite`. The shape mirrors the single-op CLI flags so
// users who already know `page apply` can compose multi files easily.
