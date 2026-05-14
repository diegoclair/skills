package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/diegoclair/skills/pkg/atlassian/adf"
)

func runIssueComment(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	var (
		key       string
		body      string
		bodyFile  string
		bodyStdin bool
		dryRun    bool
		asJSON    bool
	)

	remaining, cloud, email, token, err := parseCommonFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--dry-run":
			dryRun = true
		case "--json":
			asJSON = true
		case "--key":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--key requires a value")
				return exitInputErr, errInvalidUsage
			}
			key = remaining[i+1]
			i++
		case "--body":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--body requires a value")
				return exitInputErr, errInvalidUsage
			}
			body = remaining[i+1]
			i++
		case "--body-file":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--body-file requires a value")
				return exitInputErr, errInvalidUsage
			}
			bodyFile = remaining[i+1]
			i++
		case "--body-stdin":
			bodyStdin = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "issue comment — add a comment to a Jira issue.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  --key K              issue key, e.g. PROJ-123 (required)")
			fmt.Fprintln(stdout, "  --body \"text\"        markdown comment body")
			fmt.Fprintln(stdout, "  --body-file path.md  read body from file")
			fmt.Fprintln(stdout, "  --body-stdin         read body from stdin")
			fmt.Fprintln(stdout, "  --dry-run            preview ADF size without posting")
			fmt.Fprintln(stdout, "  --json               emit JSON of the created comment")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Exactly one of --body, --body-file, --body-stdin is required.")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	// Validate body source mutually exclusive and at-least-one.
	bodySourceCount := 0
	if body != "" {
		bodySourceCount++
	}
	if bodyFile != "" {
		bodySourceCount++
	}
	if bodyStdin {
		bodySourceCount++
	}
	if bodySourceCount > 1 {
		fmt.Fprintln(stderr, "issue comment: --body, --body-file, and --body-stdin are mutually exclusive")
		return exitInputErr, errInvalidUsage
	}
	if bodySourceCount == 0 {
		fmt.Fprintln(stderr, "issue comment: one of --body, --body-file, or --body-stdin is required")
		return exitInputErr, errInvalidUsage
	}

	// Resolve markdown bytes.
	var mdBytes []byte
	switch {
	case bodyFile != "":
		mdBytes, err = os.ReadFile(bodyFile)
		if err != nil {
			fmt.Fprintln(stderr, "issue comment: read body file:", err)
			return exitUnknownErr, err
		}
	case bodyStdin:
		mdBytes, err = io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintln(stderr, "issue comment: read stdin:", err)
			return exitUnknownErr, err
		}
	default:
		mdBytes = []byte(body)
	}

	// Convert markdown to ADF.
	node, err := adf.Convert(mdBytes)
	if err != nil {
		fmt.Fprintln(stderr, "issue comment: convert markdown to ADF:", err)
		return exitUnknownErr, err
	}
	adfBody, err := json.Marshal(node)
	if err != nil {
		fmt.Fprintln(stderr, "issue comment: marshal ADF:", err)
		return exitUnknownErr, err
	}

	// --dry-run short-circuits before buildClient.
	if dryRun {
		fmt.Fprintf(stdout, `{"status":"dry-run","action":"comment","key":%q,"adfBytes":%d}`+"\n", key, len(adfBody))
		return exitOK, nil
	}

	if key == "" {
		fmt.Fprintln(stderr, "issue comment: --key is required")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	comment, err := client.AddComment(key, json.RawMessage(adfBody))
	if err != nil {
		fmt.Fprintln(stderr, "issue comment:", err)
		return exitUnknownErr, err
	}

	if asJSON {
		b, _ := json.MarshalIndent(comment, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return exitOK, nil
	}

	fmt.Fprintf(stdout, "commented: %s (comment ID %s)\nurl:       %s?focusedCommentId=%s\n",
		key, comment.ID, issueWebURL(client, key), comment.ID)
	return exitOK, nil
}
