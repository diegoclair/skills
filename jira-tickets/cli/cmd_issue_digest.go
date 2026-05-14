package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/diegoclair/skills/pkg/atlassian/jira"
)

// digestOut is a slim representation of an issue for the `issue digest` command.
// Defined here — not exported to types.go — since it's only a CLI output shape.
type digestOut struct {
	Key      string   `json:"key"`
	Summary  string   `json:"summary"`
	Status   string   `json:"status"`
	Category string   `json:"statusCategory"`
	Type     string   `json:"type"`
	Subtask  bool     `json:"subtask"`
	Priority string   `json:"priority,omitempty"`
	Assignee string   `json:"assignee,omitempty"`
	Reporter string   `json:"reporter,omitempty"`
	Parent   string   `json:"parent,omitempty"`
	Labels   []string `json:"labels,omitempty"`
	Updated  string   `json:"updated"`
	DueDate  string   `json:"dueDate,omitempty"`
	URL      string   `json:"url"`
}

// digestFields is the minimal field list fetched by `issue digest`.
// Intentionally excludes description and comment to keep response under ~500 bytes.
var digestFields = []string{
	"summary", "status", "issuetype", "priority",
	"assignee", "reporter", "parent", "labels",
	"updated", "duedate",
}

func runIssueDigest(args []string, stdout, stderr io.Writer) (int, error) {
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
		case "--key":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--key requires a value")
				return exitInputErr, errInvalidUsage
			}
			key = remaining[i+1]
			i++
		case "--json":
			asJSON = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "issue digest — slim issue summary (~500 bytes, no ADF body).")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  jira-tickets issue digest --key PROJ-123")
			fmt.Fprintln(stdout, "  jira-tickets issue digest --key PROJ-123 --json")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Fetches only the minimal field set: summary, status, type, priority,")
			fmt.Fprintln(stdout, "assignee, reporter, parent, labels, updated, duedate.")
			fmt.Fprintln(stdout, "No description or comment bodies are fetched.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --key KEY              issue key, e.g. PROJ-123 (required)")
			fmt.Fprintln(stdout, "  --json                 emit JSON instead of human-readable text")
			fmt.Fprintln(stdout, "  --cloud / --email / --token   override credentials")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if key == "" {
		fmt.Fprintln(stderr, "issue digest: --key is required (e.g. --key PROJ-123)")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	issue, err := client.GetIssue(key, digestFields)
	if err != nil {
		if jira.IsNotFound(err) {
			fmt.Fprintf(stderr, "issue %s not found\n", key)
		} else {
			fmt.Fprintln(stderr, "issue digest error:", err)
		}
		return exitUnknownErr, err
	}

	d := buildDigest(client, issue)

	if asJSON {
		out, _ := json.MarshalIndent(d, "", "  ")
		fmt.Fprintln(stdout, string(out))
		return exitOK, nil
	}

	printDigestText(stdout, d)
	return exitOK, nil
}

// buildDigest converts a *jira.Issue into a digestOut struct.
func buildDigest(client *jira.Client, issue *jira.Issue) digestOut {
	d := digestOut{
		Key:      issue.Key,
		Status:   issue.Fields.Status.Name,
		Category: issue.Fields.Status.StatusCategory.Key,
		Type:     issue.Fields.Issuetype.Name,
		Subtask:  issue.Fields.Issuetype.Subtask,
		Updated:  "",
		URL:      issueWebURL(client, issue.Key),
	}

	// Summary — truncate to 120 chars.
	d.Summary = issue.Fields.Summary
	if len(d.Summary) > 120 {
		d.Summary = d.Summary[:120] + "..."
	}

	// Updated — date only (first 10 chars of ISO 8601).
	if len(issue.Fields.Updated) >= 10 {
		d.Updated = issue.Fields.Updated[:10]
	} else {
		d.Updated = issue.Fields.Updated
	}

	if issue.Fields.Priority != nil {
		d.Priority = issue.Fields.Priority.Name
	}

	if issue.Fields.Assignee != nil {
		d.Assignee = issue.Fields.Assignee.DisplayName
	}

	if issue.Fields.Reporter != nil {
		d.Reporter = issue.Fields.Reporter.DisplayName
	}

	if issue.Fields.Parent != nil {
		d.Parent = issue.Fields.Parent.Key
	}

	if len(issue.Fields.Labels) > 0 {
		d.Labels = issue.Fields.Labels
	}

	if issue.Fields.DueDate != "" {
		d.DueDate = issue.Fields.DueDate
	}

	return d
}

// printDigestText renders a digestOut in human-readable tab-aligned format.
func printDigestText(w io.Writer, d digestOut) {
	printRow := func(label, value string) {
		if value != "" {
			fmt.Fprintf(w, "%-10s %s\n", label, value)
		}
	}

	printRow("KEY", d.Key)
	printRow("Summary", d.Summary)

	if d.Status != "" {
		statusLine := d.Status
		if d.Category != "" {
			statusLine += "  (" + d.Category + ")"
		}
		printRow("Status", statusLine)
	}

	typeLine := d.Type
	if d.Subtask {
		typeLine += "   (Subtask: yes)"
	}
	printRow("Type", typeLine)

	printRow("Priority", d.Priority)

	assignee := d.Assignee
	if assignee == "" {
		assignee = "Unassigned"
	}
	printRow("Assignee", assignee)

	printRow("Reporter", d.Reporter)
	printRow("Parent", d.Parent)

	if len(d.Labels) > 0 {
		printRow("Labels", strings.Join(d.Labels, ", "))
	}

	printRow("Updated", d.Updated)
	printRow("DueDate", d.DueDate)
	printRow("URL", d.URL)
}
