package main

import (
	"fmt"
	"io"
)

func runPageDelete(args []string, stdout, stderr io.Writer) (int, error) {
	var pageID string
	var confirmed bool

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
		case "--yes":
			confirmed = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "page delete — soft-delete a Confluence page (alias: page trash).")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  --page-id ID    page to delete (required)")
			fmt.Fprintln(stdout, "  --yes           required to confirm (otherwise rejected)")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Page goes to Confluence trash and is restorable from there.")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page delete: --page-id is required")
		return exitInputErr, errInvalidUsage
	}
	if !confirmed {
		fmt.Fprintln(stderr, "page delete: pass --yes to confirm (page goes to Confluence trash, restorable)")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	if err := client.DeletePage(pageID); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownErr, err
	}

	refreshHomeCacheAfterWrite(pageID, client, stderr)
	fmt.Fprintf(stdout, `{"status":"trashed","pageId":%q}`+"\n", pageID)
	return exitOK, nil
}

// runPageReorder repositions a page among its siblings, or appends it as the
// last child of a different parent. Wraps the v1 move endpoint
// (PUT /rest/api/content/{id}/move/{position}/{targetId}) — body and title
// are not touched.
