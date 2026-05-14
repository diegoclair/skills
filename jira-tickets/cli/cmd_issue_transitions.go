package main

import (
	"encoding/json"
	"fmt"
	"io"
)

func runIssueTransitions(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		key    string
		asJSON bool
	)

	remaining, cloud, email, token, err := parseCommonFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--json":
			asJSON = true
		case "--key":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--key requires a value")
				return exitInputErr, errInvalidUsage
			}
			key = remaining[i+1]
			i++
		case "-h", "--help":
			fmt.Fprintln(stdout, "issue transitions — list available workflow transitions for an issue.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  --key K    issue key, e.g. PROJ-123 (required)")
			fmt.Fprintln(stdout, "  --json     emit JSON array instead of TSV")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if key == "" {
		fmt.Fprintln(stderr, "issue transitions: --key is required")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	transitions, err := client.GetTransitions(key)
	if err != nil {
		fmt.Fprintln(stderr, "issue transitions:", err)
		return exitUnknownErr, err
	}

	if asJSON {
		b, _ := json.MarshalIndent(transitions, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return exitOK, nil
	}

	// TSV output.
	fmt.Fprintln(stdout, "ID\tNAME\tTO_STATUS\tAVAILABLE")
	for _, t := range transitions {
		avail := "no"
		if t.IsAvailable {
			avail = "yes"
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", t.ID, t.Name, t.To.Name, avail)
	}
	return exitOK, nil
}
