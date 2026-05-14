package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/diegoclair/skills/pkg/atlassian/jira"
)

func runIssueGet(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		key       string
		fieldsStr string
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
		case "--key":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--key requires a value")
				return exitInputErr, errInvalidUsage
			}
			key = remaining[i+1]
			i++
		case "--fields":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--fields requires a value")
				return exitInputErr, errInvalidUsage
			}
			fieldsStr = remaining[i+1]
			i++
		case "--json":
			asJSON = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "issue get — fetch a single Jira issue by key.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  jira-tickets issue get --key PROJ-123")
			fmt.Fprintln(stdout, "  jira-tickets issue get --key PROJ-123 --json")
			fmt.Fprintln(stdout, "  jira-tickets issue get --key PROJ-123 --fields \"*all\"")
			fmt.Fprintln(stdout, "  jira-tickets issue get --key PROJ-123 --fields summary,status,comment")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --key KEY              issue key, e.g. PROJ-123 (required)")
			fmt.Fprintln(stdout, "  --fields A,B,C         comma-separated field list (default: sane set)")
			fmt.Fprintln(stdout, "                         pass --fields \"*all\" to get all fields")
			fmt.Fprintln(stdout, "  --json                 emit pretty JSON of the full Issue struct")
			fmt.Fprintln(stdout, "  --cloud / --email / --token   override credentials")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if key == "" {
		fmt.Fprintln(stderr, "issue get: --key is required (e.g. --key PROJ-123)")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	var fields []string
	if fieldsStr != "" {
		fields = parseStringList(fieldsStr)
	}

	issue, err := client.GetIssue(key, fields)
	if err != nil {
		if jira.IsNotFound(err) {
			fmt.Fprintf(stderr, "issue %s not found\n", key)
		} else {
			fmt.Fprintln(stderr, "issue get error:", err)
		}
		return exitUnknownErr, err
	}

	if asJSON {
		out, _ := json.MarshalIndent(issue, "", "  ")
		fmt.Fprintln(stdout, string(out))
		return exitOK, nil
	}

	// Human-readable text rendering — omit nil/empty fields.
	printRow := func(label, value string) {
		if value != "" {
			fmt.Fprintf(stdout, "%-10s %s\n", label, value)
		}
	}

	printRow("KEY", issue.Key)
	printRow("Summary", issue.Fields.Summary)

	if issue.Fields.Status.Name != "" {
		statusLine := issue.Fields.Status.Name
		if issue.Fields.Status.StatusCategory.Name != "" {
			statusLine += "  (category: " + issue.Fields.Status.StatusCategory.Name + ")"
		}
		printRow("Status", statusLine)
	}

	printRow("Type", issue.Fields.Issuetype.Name)

	if issue.Fields.Priority != nil {
		printRow("Priority", issue.Fields.Priority.Name)
	}

	if issue.Fields.Assignee != nil {
		assigneeLine := issue.Fields.Assignee.DisplayName
		if issue.Fields.Assignee.AccountID != "" {
			assigneeLine += "  (accountId: " + issue.Fields.Assignee.AccountID + ")"
		}
		printRow("Assignee", assigneeLine)
	}

	if issue.Fields.Reporter != nil {
		printRow("Reporter", issue.Fields.Reporter.DisplayName)
	}

	if issue.Fields.Project.Key != "" {
		projectLine := issue.Fields.Project.Key
		if issue.Fields.Project.Name != "" {
			projectLine += " — " + issue.Fields.Project.Name
		}
		printRow("Project", projectLine)
	}

	if issue.Fields.Parent != nil {
		printRow("Parent", issue.Fields.Parent.Key)
	}

	if issue.Fields.Sprint != nil {
		sprintLine := issue.Fields.Sprint.Name
		if issue.Fields.Sprint.State != "" {
			sprintLine += "  (state: " + issue.Fields.Sprint.State + ")"
		}
		printRow("Sprint", sprintLine)
	}

	if len(issue.Fields.Labels) > 0 {
		printRow("Labels", strings.Join(issue.Fields.Labels, ", "))
	}

	printRow("Created", issue.Fields.Created)
	printRow("Updated", issue.Fields.Updated)
	printRow("URL", issueWebURL(client, issue.Key))

	if len(issue.Fields.Description) > 0 && string(issue.Fields.Description) != "null" {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "Description:")
		fmt.Fprintf(stdout, "<%d chars of ADF JSON; pass --json to see raw>\n", len(issue.Fields.Description))
	}

	return exitOK, nil
}
