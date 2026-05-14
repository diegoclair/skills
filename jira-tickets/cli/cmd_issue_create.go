package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/diegoclair/skills/pkg/atlassian/adf"
	"github.com/diegoclair/skills/pkg/atlassian/jira"
)

func runIssueCreate(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	var (
		project         string
		issueType       string
		summary         string
		description     string
		descriptionFile string
		labels          string
		assignee        string
		parent          string
		dueDate         string
		priority        string
		dryRun          bool
		asJSON          bool
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
		case "--project":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--project requires a value")
				return exitInputErr, errInvalidUsage
			}
			project = remaining[i+1]
			i++
		case "--type":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--type requires a value")
				return exitInputErr, errInvalidUsage
			}
			issueType = remaining[i+1]
			i++
		case "--summary":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--summary requires a value")
				return exitInputErr, errInvalidUsage
			}
			summary = remaining[i+1]
			i++
		case "--description":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--description requires a value")
				return exitInputErr, errInvalidUsage
			}
			description = remaining[i+1]
			i++
		case "--description-file":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--description-file requires a value")
				return exitInputErr, errInvalidUsage
			}
			descriptionFile = remaining[i+1]
			i++
		case "--labels":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--labels requires a value")
				return exitInputErr, errInvalidUsage
			}
			labels = remaining[i+1]
			i++
		case "--assignee":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--assignee requires a value")
				return exitInputErr, errInvalidUsage
			}
			assignee = remaining[i+1]
			i++
		case "--parent":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--parent requires a value")
				return exitInputErr, errInvalidUsage
			}
			parent = remaining[i+1]
			i++
		case "--due-date":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--due-date requires a value")
				return exitInputErr, errInvalidUsage
			}
			dueDate = remaining[i+1]
			i++
		case "--priority":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--priority requires a value")
				return exitInputErr, errInvalidUsage
			}
			priority = remaining[i+1]
			i++
		case "-h", "--help":
			fmt.Fprintln(stdout, "issue create — create a new Jira issue.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  --project KEY           project key, e.g. PROJ (required)")
			fmt.Fprintln(stdout, "  --type TYPE             issue type, e.g. Task, Bug, Story (required)")
			fmt.Fprintln(stdout, "  --summary \"...\"         one-line title (required)")
			fmt.Fprintln(stdout, "  --description \"text\"    body as markdown (optional)")
			fmt.Fprintln(stdout, "  --description-file path.md  read body from file (optional)")
			fmt.Fprintln(stdout, "  --labels a,b,c          comma-separated labels (optional)")
			fmt.Fprintln(stdout, "  --assignee ACCOUNTID    assignee account ID (optional)")
			fmt.Fprintln(stdout, "  --parent KEY            parent issue or epic key (optional)")
			fmt.Fprintln(stdout, "  --due-date YYYY-MM-DD   due date (optional)")
			fmt.Fprintln(stdout, "  --priority NAME         e.g. High, Medium, Low (optional)")
			fmt.Fprintln(stdout, "  --dry-run               print intended action without creating")
			fmt.Fprintln(stdout, "  --json                  emit JSON of the created issue")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	// --dry-run short-circuits BEFORE validation and buildClient.
	if dryRun {
		fmt.Fprintf(stdout, `{"status":"dry-run","action":"create","project":%q,"type":%q,"summary":%q}`+"\n",
			project, issueType, summary)
		return exitOK, nil
	}

	if project == "" {
		fmt.Fprintln(stderr, "issue create: --project is required")
		return exitInputErr, errInvalidUsage
	}
	if issueType == "" {
		fmt.Fprintln(stderr, "issue create: --type is required")
		return exitInputErr, errInvalidUsage
	}
	if summary == "" {
		fmt.Fprintln(stderr, "issue create: --summary is required")
		return exitInputErr, errInvalidUsage
	}
	if description != "" && descriptionFile != "" {
		fmt.Fprintln(stderr, "issue create: --description and --description-file are mutually exclusive")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	// Build the description ADF if provided.
	var descADF json.RawMessage
	if descriptionFile != "" {
		data, err := os.ReadFile(descriptionFile)
		if err != nil {
			fmt.Fprintln(stderr, "issue create: read description file:", err)
			return exitUnknownErr, err
		}
		node, err := adf.Convert(data)
		if err != nil {
			fmt.Fprintln(stderr, "issue create: convert description to ADF:", err)
			return exitUnknownErr, err
		}
		descADF, err = json.Marshal(node)
		if err != nil {
			fmt.Fprintln(stderr, "issue create: marshal ADF:", err)
			return exitUnknownErr, err
		}
	} else if description != "" {
		node, err := adf.Convert([]byte(description))
		if err != nil {
			fmt.Fprintln(stderr, "issue create: convert description to ADF:", err)
			return exitUnknownErr, err
		}
		descADF, err = json.Marshal(node)
		if err != nil {
			fmt.Fprintln(stderr, "issue create: marshal ADF:", err)
			return exitUnknownErr, err
		}
	}

	input := jira.CreateIssueInput{
		ProjectKey:   project,
		IssueType:    issueType,
		Summary:      summary,
		Description:  descADF,
		Labels:       parseStringList(labels),
		AssigneeID:   assignee,
		ParentKey:    parent,
		DueDate:      dueDate,
		PriorityName: priority,
	}

	issue, err := client.CreateIssue(input)
	if err != nil {
		fmt.Fprintln(stderr, "issue create:", err)
		return exitUnknownErr, err
	}

	if asJSON {
		b, _ := json.MarshalIndent(issue, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return exitOK, nil
	}

	fmt.Fprintf(stdout, "created: %s\nurl:     %s\n", issue.Key, issueWebURL(client, issue.Key))
	return exitOK, nil
}
