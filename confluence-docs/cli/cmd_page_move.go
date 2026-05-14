package main

import (
	"fmt"
	"io"
)

func runPageMove(args []string, stdout, stderr io.Writer) (int, error) {
	var pageID, parentID, title, message string
	var dryRun bool

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
		case "--message":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--message requires a value")
				return exitInputErr, errInvalidUsage
			}
			message = remaining[i+1]
			i++
		case "--dry-run":
			dryRun = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "page move — rename and/or reparent a Confluence page (alias: page rename).")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  --page-id ID           page to move (required)")
			fmt.Fprintln(stdout, "  --parent-id NEW_PARENT new parent page id")
			fmt.Fprintln(stdout, "  --title NEW_TITLE      new title")
			fmt.Fprintln(stdout, "  --message MSG          version comment")
			fmt.Fprintln(stdout, "  --dry-run              show what would change without writing")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "At least one of --parent-id or --title is required. Body is preserved.")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page move: --page-id is required")
		return exitInputErr, errInvalidUsage
	}
	if parentID == "" && title == "" {
		fmt.Fprintln(stderr, "page move: at least one of --parent-id or --title is required")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	if err := client.MovePage(pageID, parentID, title, message, dryRun, stderr); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownErr, err
	}

	if !dryRun {
		refreshHomeCacheAfterWrite(pageID, client, stderr)
		fmt.Fprintf(stdout, `{"status":"ok","pageId":%q}`+"\n", pageID)
	}
	return exitOK, nil
}
