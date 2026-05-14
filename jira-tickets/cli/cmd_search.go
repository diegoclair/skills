package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/diegoclair/skills/pkg/atlassian/jira"
)

func runSearch(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		jqlQuery      string
		limit         int
		fieldsStr     string
		nextPageToken string
		asJSON        bool
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
		case "--fields":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--fields requires a value")
				return exitInputErr, errInvalidUsage
			}
			fieldsStr = remaining[i+1]
			i++
		case "--next-page-token":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--next-page-token requires a value")
				return exitInputErr, errInvalidUsage
			}
			nextPageToken = remaining[i+1]
			i++
		case "--json":
			asJSON = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "search — JQL search via the Jira REST v3 search API. TSV output by default.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  jira-tickets search \"JQL\"")
			fmt.Fprintln(stdout, "  jira-tickets search 'project = PROJ AND status = \"In Progress\"' --limit 20")
			fmt.Fprintln(stdout, "  jira-tickets search 'assignee = currentUser()' --json")
			fmt.Fprintln(stdout, "  jira-tickets search 'project = PROJ' --fields summary,status,assignee")
			fmt.Fprintln(stdout, "  jira-tickets search 'project = PROJ' --next-page-token <token>")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --limit N              max results (default 50, max 100)")
			fmt.Fprintln(stdout, "  --fields A,B,C         comma-separated field list (default: sane set)")
			fmt.Fprintln(stdout, "  --next-page-token TOK  cursor for next page")
			fmt.Fprintln(stdout, "  --json                 emit JSON instead of TSV")
			fmt.Fprintln(stdout, "  --cloud / --email / --token   override credentials")
			return exitOK, nil
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintln(stderr, "unknown flag:", a)
				return exitInputErr, errInvalidUsage
			}
			// positional: JQL query
			if jqlQuery != "" {
				fmt.Fprintln(stderr, "search: unexpected argument:", a)
				return exitInputErr, errInvalidUsage
			}
			jqlQuery = a
		}
	}

	if jqlQuery == "" {
		fmt.Fprintln(stderr, "search: JQL query is required (positional argument)")
		fmt.Fprintln(stderr, "  example: jira-tickets search 'project = PROJ'")
		return exitInputErr, errInvalidUsage
	}
	if limit == 0 {
		limit = 50
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	opts := jira.SearchOpts{
		JQL:           jqlQuery,
		MaxResults:    limit,
		NextPageToken: nextPageToken,
	}
	if fieldsStr != "" {
		opts.Fields = parseStringList(fieldsStr)
	}

	result, err := client.SearchJQL(opts)
	if err != nil {
		fmt.Fprintln(stderr, "search error:", err)
		return exitUnknownErr, err
	}

	if asJSON {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(out))
		// pagination hint even in JSON mode
		if result.NextPageToken != "" {
			fmt.Fprintf(stderr, "# next-page-token: %s  (re-run with --next-page-token %s for more)\n",
				result.NextPageToken, result.NextPageToken)
		}
		return exitOK, nil
	}

	// TSV output: KEY\tSUMMARY\tSTATUS\tASSIGNEE\tUPDATED\tURL
	for _, issue := range result.Issues {
		summary := issue.Fields.Summary
		if len(summary) > 80 {
			summary = summary[:80] + "..."
		}
		status := issue.Fields.Status.Name
		assignee := "-"
		if issue.Fields.Assignee != nil {
			assignee = issue.Fields.Assignee.DisplayName
		}
		updated := ""
		if len(issue.Fields.Updated) >= 10 {
			updated = issue.Fields.Updated[:10]
		}
		url := issueWebURL(client, issue.Key)
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n",
			issue.Key, summary, status, assignee, updated, url)
	}

	if result.NextPageToken != "" {
		fmt.Fprintf(stderr, "# next-page-token: %s  (re-run with --next-page-token %s for more)\n",
			result.NextPageToken, result.NextPageToken)
	}
	return exitOK, nil
}
