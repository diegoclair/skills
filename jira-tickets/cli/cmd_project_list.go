package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

func runProjectList(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		limit   int
		startAt int
		asJSON  bool
	)

	remaining, cloud, email, token, err := parseCommonFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--limit":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--limit requires a value")
				return exitInputErr, errInvalidUsage
			}
			n, sErr := strconv.Atoi(remaining[i+1])
			if sErr != nil || n < 1 || n > 100 {
				fmt.Fprintln(stderr, "--limit must be an integer between 1 and 100")
				return exitInputErr, errInvalidUsage
			}
			limit = n
			i++
		case "--start-at":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--start-at requires a value")
				return exitInputErr, errInvalidUsage
			}
			n, sErr := strconv.Atoi(remaining[i+1])
			if sErr != nil || n < 0 {
				fmt.Fprintln(stderr, "--start-at must be a non-negative integer")
				return exitInputErr, errInvalidUsage
			}
			startAt = n
			i++
		case "--json":
			asJSON = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "project list — list projects on the Jira instance. TSV output by default.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  jira-tickets project list")
			fmt.Fprintln(stdout, "  jira-tickets project list --limit 20")
			fmt.Fprintln(stdout, "  jira-tickets project list --json")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --limit N     max results per page (default 50, max 100)")
			fmt.Fprintln(stdout, "  --start-at N  zero-based offset for pagination (default 0)")
			fmt.Fprintln(stdout, "  --json        emit JSON instead of TSV")
			fmt.Fprintln(stdout, "  --cloud / --email / --token   override credentials")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if limit == 0 {
		limit = 50
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	result, err := client.ListProjects(limit, startAt)
	if err != nil {
		fmt.Fprintln(stderr, "project list error:", err)
		return exitUnknownErr, err
	}

	if asJSON {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(out))
		return exitOK, nil
	}

	for _, p := range result.Projects {
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", p.Key, p.Name, p.ProjectTypeKey, p.ID)
	}

	if !result.IsLast {
		nextStart := result.StartAt + len(result.Projects)
		fmt.Fprintf(stderr, "# more results available — re-run with --start-at %d\n", nextStart)
	}
	return exitOK, nil
}
