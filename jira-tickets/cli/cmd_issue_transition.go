package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func runIssueTransition(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		key    string
		to     string
		dryRun bool
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
		case "--key":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--key requires a value")
				return exitInputErr, errInvalidUsage
			}
			key = remaining[i+1]
			i++
		case "--to":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--to requires a value")
				return exitInputErr, errInvalidUsage
			}
			to = remaining[i+1]
			i++
		case "-h", "--help":
			fmt.Fprintln(stdout, "issue transition — apply a workflow transition to an issue.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  --key K          issue key, e.g. PROJ-123 (required)")
			fmt.Fprintln(stdout, "  --to \"Name\"      transition name or target status name (required)")
			fmt.Fprintln(stdout, "  --dry-run        print intended action without applying")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Use `jira-tickets issue transitions --key K` to list available transitions.")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	// --dry-run short-circuits before any API call.
	if dryRun {
		fmt.Fprintf(stdout, `{"status":"dry-run","action":"transition","key":%q,"to":%q}`+"\n", key, to)
		return exitOK, nil
	}

	if key == "" {
		fmt.Fprintln(stderr, "issue transition: --key is required")
		return exitInputErr, errInvalidUsage
	}
	if to == "" {
		fmt.Fprintln(stderr, "issue transition: --to is required")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	transitions, err := client.GetTransitions(key)
	if err != nil {
		fmt.Fprintln(stderr, "issue transition:", err)
		return exitUnknownErr, err
	}

	toLower := strings.ToLower(to)
	var matchedID, matchedName string
	for _, t := range transitions {
		if strings.ToLower(t.Name) == toLower || strings.ToLower(t.To.Name) == toLower {
			matchedID = t.ID
			matchedName = t.Name
			break
		}
	}

	if matchedID == "" {
		fmt.Fprintf(stderr, "issue transition: no transition matching %q found for %s\n", to, key)
		fmt.Fprintln(stderr, "available transitions:")
		for _, t := range transitions {
			fmt.Fprintf(stderr, "  %s → %s\n", t.Name, t.To.Name)
		}
		return exitInputErr, errInvalidUsage
	}

	if err := client.TransitionIssue(key, matchedID); err != nil {
		fmt.Fprintln(stderr, "issue transition:", err)
		return exitUnknownErr, err
	}

	out := map[string]string{
		"status":       "ok",
		"key":          key,
		"transition":   matchedName,
		"transitionId": matchedID,
	}
	b, _ := json.Marshal(out)
	fmt.Fprintln(stdout, string(b))
	return exitOK, nil
}
