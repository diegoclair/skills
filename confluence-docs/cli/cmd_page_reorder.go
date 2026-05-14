package main

import (
	"fmt"
	"io"
)

func runPageReorder(args []string, stdout, stderr io.Writer) (int, error) {
	var pageID string
	var before, after, appendTo string
	var dryRun bool

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--dry-run":
			dryRun = true
		case "--page-id":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--page-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			pageID = remaining[i+1]
			i++
		case "--before":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--before requires a TARGET_PAGE_ID")
				return exitInputErr, errInvalidUsage
			}
			before = remaining[i+1]
			i++
		case "--after":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--after requires a TARGET_PAGE_ID")
				return exitInputErr, errInvalidUsage
			}
			after = remaining[i+1]
			i++
		case "--append-to":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--append-to requires a NEW_PARENT_PAGE_ID")
				return exitInputErr, errInvalidUsage
			}
			appendTo = remaining[i+1]
			i++
		case "-h", "--help":
			fmt.Fprintln(stdout, "page reorder — reposition a page among its siblings.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  --page-id ID           page to reposition (required)")
			fmt.Fprintln(stdout, "  --before TARGET_ID     place page right before TARGET (same parent)")
			fmt.Fprintln(stdout, "  --after  TARGET_ID     place page right after  TARGET (same parent)")
			fmt.Fprintln(stdout, "  --append-to PARENT_ID  append as last child of PARENT (re-parents)")
			fmt.Fprintln(stdout, "  --dry-run              print intended action as JSON; no HTTP call")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Exactly one of --before / --after / --append-to is required. Body and")
			fmt.Fprintln(stdout, "title are NOT modified — use 'page move' for rename or reparent-to-")
			fmt.Fprintln(stdout, "first-position. This calls the v1 endpoint")
			fmt.Fprintln(stdout, "PUT /wiki/rest/api/content/{pageId}/move/{position}/{targetId}.")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page reorder: --page-id is required")
		return exitInputErr, errInvalidUsage
	}

	var position, targetID string
	picked := 0
	if before != "" {
		position, targetID, picked = "before", before, picked+1
	}
	if after != "" {
		position, targetID, picked = "after", after, picked+1
	}
	if appendTo != "" {
		position, targetID, picked = "append", appendTo, picked+1
	}
	if picked == 0 {
		fmt.Fprintln(stderr, "page reorder: one of --before / --after / --append-to is required")
		return exitInputErr, errInvalidUsage
	}
	if picked > 1 {
		fmt.Fprintln(stderr, "page reorder: --before, --after, --append-to are mutually exclusive")
		return exitInputErr, errInvalidUsage
	}

	// --dry-run skips both buildClient and the HTTP call. The point of dry-run
	// is to let a caller (typically the agent) confirm intent before applying;
	// requiring credentials for that defeats the purpose, and the output is
	// always derivable from the flags alone (the API doesn't add information).
	if dryRun {
		fmt.Fprintf(stdout, `{"status":"dry-run","pageId":%q,"position":%q,"targetId":%q}`+"\n", pageID, position, targetID)
		return exitOK, nil
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	if err := client.ReorderPage(pageID, position, targetID); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownErr, err
	}

	refreshHomeCacheAfterWrite(pageID, client, stderr)
	fmt.Fprintf(stdout, `{"status":"ok","pageId":%q,"position":%q,"targetId":%q}`+"\n", pageID, position, targetID)
	return exitOK, nil
}

// runPageDigest fetches a page, parses its ADF, and emits a slim summary
// (title, version, headings + word counts, macros). Designed to answer
// "what's in this page?" without round-tripping the full ADF.
