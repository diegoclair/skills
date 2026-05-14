package main

import (
	"fmt"
	"io"

	"github.com/lybel-app/skills/confluence-docs/cli/adf"
)

// runPage handles the `page` subcommand with verbs: get, upload, create.
func runPage(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "page: requires a verb: get, upload, create, children, digest, apply, rewrite, move, reorder, delete")
		return exitInputErr, errInvalidUsage
	}
	switch args[0] {
	case "get":
		return runPageGet(args[1:], stdout, stderr)
	case "upload":
		return runPageUpload(args[1:], stdout, stderr)
	case "create":
		return runPageCreate(args[1:], stdout, stderr)
	case "children", "list-children":
		// `children` is the canonical (single-word) verb, parallel to
		// `get`/`digest`/`apply`. `list-children` kept as a back-compat alias
		// from the original kebab-case name.
		return runPageListChildren(args[1:], stdout, stderr)
	case "digest":
		return runPageDigest(args[1:], stdout, stderr)
	case "apply":
		return runPageApply(args[1:], stdout, stderr)
	case "rewrite":
		return runPageRewrite(args[1:], stdout, stderr)
	case "move", "rename":
		return runPageMove(args[1:], stdout, stderr)
	case "reorder":
		return runPageReorder(args[1:], stdout, stderr)
	case "delete", "trash":
		return runPageDelete(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "page: unknown verb:", args[0])
		fmt.Fprintln(stderr, "  valid verbs: get, upload, create, children, digest, apply, rewrite, move, reorder, delete")
		return exitInputErr, errInvalidUsage
	}
}

// parseCommonPageFlags parses --cloud, --email, --token flags shared by page verbs.
// Returns remaining args and the parsed values.
func parseCommonPageFlags(args []string) (remaining []string, cloud, email, token string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--cloud":
			if i+1 >= len(args) {
				return nil, "", "", "", fmt.Errorf("--cloud requires a value")
			}
			cloud = args[i+1]
			i++
		case "--email":
			if i+1 >= len(args) {
				return nil, "", "", "", fmt.Errorf("--email requires a value")
			}
			email = args[i+1]
			i++
		case "--token":
			if i+1 >= len(args) {
				return nil, "", "", "", fmt.Errorf("--token requires a value")
			}
			token = args[i+1]
			i++
		default:
			remaining = append(remaining, a)
		}
	}
	return remaining, cloud, email, token, nil
}

// buildClient resolves cloud+creds and returns a ready-to-use ConfluenceClient.
func buildClient(cloud, email, token string, stderr io.Writer) (*adf.ConfluenceClient, bool) {
	resolvedCloud := adf.ResolveCloud(cloud)
	creds, err := adf.ResolveCreds(email, token)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, false
	}
	return adf.NewClient(resolvedCloud, creds), true
}

// pageWebURL constructs the Confluence web UI URL for a page using the
// configured space key. Falls back to the client base URL + page ID if the
// space key is not configured.
func pageWebURL(client *adf.ConfluenceClient, pageID string) string {
	if key, err := currentSpaceKey(); err == nil && key != "" {
		return fmt.Sprintf("%s/spaces/%s/pages/%s", client.BaseURL(), key, pageID)
	}
	// Fallback: just include the page ID as a path (still a valid redirect for
	// most Confluence Cloud installations).
	return fmt.Sprintf("%s/pages/%s", client.BaseURL(), pageID)
}
