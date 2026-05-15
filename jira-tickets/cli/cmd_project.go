package main

import (
	"fmt"
	"io"
)

// runProject dispatches `jira-tickets project <verb>` to the verb-specific handler.
// Verbs land in separate files: cmd_project_list.go, cmd_project_get.go,
// cmd_project_update.go.
func runProject(args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "project: requires a verb: list, get, update")
		return exitInputErr, errInvalidUsage
	}
	switch args[0] {
	case "list":
		return runProjectList(args[1:], stdout, stderr)
	case "get":
		return runProjectGet(args[1:], stdout, stderr)
	case "update":
		return runProjectUpdate(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "project: unknown verb:", args[0])
		fmt.Fprintln(stderr, "  valid verbs: list, get, update")
		return exitInputErr, errInvalidUsage
	}
}
