package main

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	"github.com/diegoclair/skills/pkg/atlassian/jira"
)

var validProjectKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,9}$`)

func runProjectUpdate(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		projectKey  string
		newName     string
		newKey      string
		newDesc     string
		dryRun      bool
	)

	remaining, cloud, email, token, err := parseCommonFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--name":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--name requires a value")
				return exitInputErr, errInvalidUsage
			}
			newName = remaining[i+1]
			i++
		case "--key":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--key requires a value")
				return exitInputErr, errInvalidUsage
			}
			newKey = remaining[i+1]
			i++
		case "--description":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--description requires a value")
				return exitInputErr, errInvalidUsage
			}
			newDesc = remaining[i+1]
			i++
		case "--dry-run":
			dryRun = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "project update — rename or edit a project's metadata.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  jira-tickets project update LYBEL --name \"Lybel Platform\"")
			fmt.Fprintln(stdout, "  jira-tickets project update LYBEL --key LYBL --dry-run")
			fmt.Fprintln(stdout, "  jira-tickets project update LYBEL --description \"Core platform\" --name \"Lybel\"")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --name \"New Name\"    new display name")
			fmt.Fprintln(stdout, "  --key NEW_KEY        new project key (uppercase only, 2–10 chars,")
			fmt.Fprintln(stdout, "                       must match ^[A-Z][A-Z0-9_]{1,9}$)")
			fmt.Fprintln(stdout, "  --description \"...\" new description")
			fmt.Fprintln(stdout, "  --dry-run            print intended payload without calling the API")
			fmt.Fprintln(stdout, "  --cloud / --email / --token   override credentials")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "WARNING: renaming the key (--key) changes all issue prefixes")
			fmt.Fprintln(stdout, "  (SCRUM-1 → NEWKEY-1) and breaks external references hardcoded")
			fmt.Fprintln(stdout, "  to the old key. Jira keeps a redirect from the old key, but")
			fmt.Fprintln(stdout, "  agents/scripts that fetch by hardcoded key still work.")
			return exitOK, nil
		default:
			if projectKey != "" {
				fmt.Fprintln(stderr, "project update: unexpected argument:", a)
				return exitInputErr, errInvalidUsage
			}
			projectKey = a
		}
	}

	if projectKey == "" {
		fmt.Fprintln(stderr, "project update: project KEY is required (e.g. jira-tickets project update LYBEL --name \"New Name\")")
		return exitInputErr, errInvalidUsage
	}
	if newName == "" && newKey == "" && newDesc == "" {
		fmt.Fprintln(stderr, "project update: at least one of --name, --key, or --description is required")
		return exitInputErr, errInvalidUsage
	}
	if newKey != "" && !validProjectKey.MatchString(newKey) {
		fmt.Fprintln(stderr, "project update: --key must match ^[A-Z][A-Z0-9_]{1,9}$ (uppercase, 2–10 chars)")
		return exitInputErr, errInvalidUsage
	}

	update := jira.ProjectUpdate{}
	if newName != "" {
		update.Name = &newName
	}
	if newKey != "" {
		update.Key = &newKey
	}
	if newDesc != "" {
		update.Description = &newDesc
	}

	if dryRun {
		out, _ := json.MarshalIndent(map[string]any{
			"dry_run":     true,
			"method":      "PUT",
			"path":        "/rest/api/3/project/" + projectKey,
			"body":        update,
		}, "", "  ")
		fmt.Fprintln(stdout, string(out))
		return exitOK, nil
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	p, err := client.UpdateProject(projectKey, update)
	if err != nil {
		if jira.IsNotFound(err) {
			fmt.Fprintf(stderr, "project %s not found\n", projectKey)
		} else {
			fmt.Fprintln(stderr, "project update error:", err)
		}
		return exitUnknownErr, err
	}

	fmt.Fprintf(stdout, "%-16s %s\n", "KEY", p.Key)
	fmt.Fprintf(stdout, "%-16s %s\n", "Name", p.Name)
	fmt.Fprintf(stdout, "%-16s %s\n", "ID", p.ID)

	return exitOK, nil
}
