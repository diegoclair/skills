package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// runMyself reports the authenticated user. Cheapest API call available —
// used by `jira-tickets setup --check`-style flows and as a sanity probe.
func runMyself(args []string, stdout, stderr io.Writer) (int, error) {
	var asJSON bool
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--json":
			asJSON = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "myself — print the authenticated Atlassian user.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  --json      emit JSON instead of human-readable text")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Reads credentials from ~/.config/atlassian/credentials, the legacy")
			fmt.Fprintln(stdout, "per-skill path, or the --email/--token flags / env vars.")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	client, ok := buildClient("", "", "", stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	user, err := client.Myself()
	if err != nil {
		fmt.Fprintln(stderr, "myself:", err)
		return exitUnknownErr, err
	}

	if asJSON {
		b, _ := json.MarshalIndent(user, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return exitOK, nil
	}

	fmt.Fprintf(stdout, "accountId:   %s\ndisplayName: %s\nemail:       %s\n",
		user.AccountID, user.DisplayName, user.EmailAddress)
	return exitOK, nil
}
