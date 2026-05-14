package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func runIssueUpdate(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		key    string
		sets   []string
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
		case "--set":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--set requires a name=value argument")
				return exitInputErr, errInvalidUsage
			}
			sets = append(sets, remaining[i+1])
			i++
		case "-h", "--help":
			fmt.Fprintln(stdout, "issue update — update fields on an existing Jira issue.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  --key K              issue key, e.g. PROJ-123 (required)")
			fmt.Fprintln(stdout, "  --set name=value     set a field; repeatable")
			fmt.Fprintln(stdout, "  --dry-run            preview field map without sending")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Known list fields (split by comma): labels, components")
			fmt.Fprintln(stdout, "Example: --set summary=\"New title\" --set priority=High --set labels=bug,ux")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if !dryRun && key == "" {
		fmt.Fprintln(stderr, "issue update: --key is required")
		return exitInputErr, errInvalidUsage
	}

	if len(sets) == 0 {
		fmt.Fprintln(stderr, "issue update: at least one --set name=value is required")
		return exitInputErr, errInvalidUsage
	}

	// Known list fields — values are split by comma into []string.
	listFields := map[string]bool{
		"labels":     true,
		"components": true,
	}

	fields := make(map[string]any, len(sets))
	updatedKeys := make([]string, 0, len(sets))
	for _, kv := range sets {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			fmt.Fprintf(stderr, "issue update: invalid --set value %q (expected name=value)\n", kv)
			return exitInputErr, errInvalidUsage
		}
		name := kv[:idx]
		value := kv[idx+1:]

		if listFields[name] {
			fields[name] = parseStringList(value)
		} else {
			fields[name] = value
		}
		updatedKeys = append(updatedKeys, name)
	}

	// --dry-run: short-circuit before buildClient.
	if dryRun {
		out := map[string]any{
			"status":  "dry-run",
			"action":  "update",
			"key":     key,
			"fields":  fields,
			"updated": updatedKeys,
		}
		b, _ := json.Marshal(out)
		fmt.Fprintln(stdout, string(b))
		return exitOK, nil
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	if err := client.EditIssue(key, fields); err != nil {
		fmt.Fprintln(stderr, "issue update:", err)
		return exitUnknownErr, err
	}

	out := map[string]any{
		"status":  "ok",
		"key":     key,
		"updated": updatedKeys,
	}
	b, _ := json.Marshal(out)
	fmt.Fprintln(stdout, string(b))
	return exitOK, nil
}
