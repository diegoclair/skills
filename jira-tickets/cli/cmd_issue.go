package main

import (
	"fmt"
	"io"
)

// runIssue dispatches `jira-tickets issue <verb>` to the verb-specific handler.
// Verbs land in separate files: cmd_issue_digest.go, cmd_issue_get.go,
// cmd_issue_create.go, cmd_issue_update.go, cmd_issue_transitions.go,
// cmd_issue_transition.go, cmd_issue_comment.go.
func runIssue(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "issue: requires a verb: digest, get, create, update, transitions, transition, comment")
		return exitInputErr, errInvalidUsage
	}
	switch args[0] {
	case "digest":
		return runIssueDigest(args[1:], stdout, stderr)
	case "get":
		return runIssueGet(args[1:], stdout, stderr)
	case "create":
		return runIssueCreate(args[1:], stdin, stdout, stderr)
	case "update":
		return runIssueUpdate(args[1:], stdout, stderr)
	case "transitions":
		return runIssueTransitions(args[1:], stdout, stderr)
	case "transition":
		return runIssueTransition(args[1:], stdout, stderr)
	case "comment":
		return runIssueComment(args[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintln(stderr, "issue: unknown verb:", args[0])
		fmt.Fprintln(stderr, "  valid verbs: digest, get, create, update, transitions, transition, comment")
		return exitInputErr, errInvalidUsage
	}
}
