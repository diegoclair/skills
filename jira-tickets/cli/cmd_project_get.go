package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/diegoclair/skills/pkg/atlassian/jira"
)

func runProjectGet(args []string, stdout, stderr io.Writer) (int, error) {
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
		case "-h", "--help":
			fmt.Fprintln(stdout, "project get — show details for one project.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  jira-tickets project get LYBEL")
			fmt.Fprintln(stdout, "  jira-tickets project get LYBEL --json")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Prints: key, name, id, project type, lead, default assignee,")
			fmt.Fprintln(stdout, "  simplified (yes/no), and avatar URL.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json        emit JSON instead of human-readable text")
			fmt.Fprintln(stdout, "  --cloud / --email / --token   override credentials")
			return exitOK, nil
		default:
			if key != "" {
				fmt.Fprintln(stderr, "project get: unexpected argument:", a)
				return exitInputErr, errInvalidUsage
			}
			key = a
		}
	}

	if key == "" {
		fmt.Fprintln(stderr, "project get: project KEY is required (e.g. jira-tickets project get LYBEL)")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	p, err := client.GetProject(key)
	if err != nil {
		if jira.IsNotFound(err) {
			fmt.Fprintf(stderr, "project %s not found\n", key)
		} else {
			fmt.Fprintln(stderr, "project get error:", err)
		}
		return exitUnknownErr, err
	}

	if asJSON {
		out, _ := json.MarshalIndent(p, "", "  ")
		fmt.Fprintln(stdout, string(out))
		return exitOK, nil
	}

	printRow := func(label, value string) {
		if value != "" {
			fmt.Fprintf(stdout, "%-16s %s\n", label, value)
		}
	}

	printRow("KEY", p.Key)
	printRow("Name", p.Name)
	printRow("ID", p.ID)
	printRow("Type", p.ProjectTypeKey)

	simplified := "no"
	if p.Simplified {
		simplified = "yes"
	}
	printRow("Simplified", simplified)

	if p.Lead != nil {
		printRow("Lead", p.Lead.DisplayName)
	}
	printRow("DefaultAssignee", p.DefaultAssignee)
	printRow("AvatarURL", p.AvatarURL)

	return exitOK, nil
}
