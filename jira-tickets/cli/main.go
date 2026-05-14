// jira-tickets is a token-efficient CLI for Atlassian Jira Cloud,
// designed to drive Claude (and other LLM agents) without the per-call
// ADF round-trip cost of the Atlassian MCP server.
//
// Returns slim digests, JQL searches as TSV, and surgical updates.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/diegoclair/skills/pkg/atlassian/setup"
)

func init() {
	// Scope the shared setup package's per-skill config (active workspace,
	// etc.) to this skill. Credentials themselves live atlassian-wide at
	// ~/.config/atlassian/credentials — see pkg/atlassian/setup.ConfigPath.
	setup.SetSkillName("jira-tickets")
}

// version is injected at build time via -ldflags "-X main.version=..."
// Falls back to the source-tree version when not set via ldflags (dev builds).
var version = "v0.1.0"

const (
	exitOK         = 0
	exitInputErr   = 2
	exitUnknownErr = 3
)

var errInvalidUsage = errors.New("invalid usage")

const helpText = `jira-tickets — token-efficient Jira Cloud CLI for LLM agents.

USAGE:
  jira-tickets setup        [--email X --token Y | --check | --print-config-path]
  jira-tickets myself       [--json]
  jira-tickets search       "JQL" [--limit N] [--fields a,b] [--next-page-token T] [--json]
  jira-tickets issue        VERB [flags]
  jira-tickets --version
  jira-tickets --help

ISSUE VERBS:
  issue digest      --key K [--json]
                    Slim summary (~500 bytes). No ADF body. Cheapest read.
  issue get         --key K [--fields a,b] [--json]
                    Full issue. Pass --fields to slice. ADF description shown
                    as a char count unless --json is also set.
  issue create      --project KEY --type Task --summary "..." [--description X
                    | --description-file PATH] [--labels a,b] [--assignee ID]
                    [--parent KEY] [--due-date YYYY-MM-DD] [--priority NAME]
                    [--dry-run] [--json]
  issue update      --key K --set name=value [--set ...] [--dry-run]
                    Repeatable --set; list fields (labels, components) split
                    by comma.
  issue transitions --key K [--json]
                    List transitions available from the current status.
  issue transition  --key K --to "Status Name" [--dry-run]
                    Apply a transition (matches case-insensitive against
                    transition name and target status name).
  issue comment     --key K (--body "..." | --body-file PATH | --body-stdin)
                    [--dry-run] [--json]
                    Markdown body converted to ADF on the way to the API.

COMMON FLAGS (any subcommand):
  --cloud X         Override the configured Atlassian cloud subdomain.
  --email X         Override the configured Atlassian email.
  --token X         Override the configured Atlassian API token.
  --dry-run         Where supported, print intended JSON action without
                    making the API call. Skips credential resolution.

UPDATE:
  jira-tickets does not yet self-update (the shared /releases/latest path
  is already used by the sibling confluence-docs skill). To upgrade:

    curl -fsSL https://raw.githubusercontent.com/diegoclair/skills/main/jira-tickets/install/install.sh | bash

  Self-update lands in v0.2.0 (GitHub API + tag-prefix filter).

CREDENTIALS:
  Read from ~/.config/atlassian/credentials (shared with confluence-docs)
  with fallback to per-skill legacy paths. Run 'jira-tickets setup' to
  configure. Same Atlassian API token works for both skills.
`

func main() {
	code, err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jira-tickets:", err)
	}
	os.Exit(code)
}

// run is the testable entry point.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprint(stderr, helpText)
		return exitInputErr, errInvalidUsage
	}
	switch args[0] {
	case "-h", "--help":
		fmt.Fprint(stdout, helpText)
		return exitOK, nil
	case "-v", "--version":
		fmt.Fprintln(stdout, "jira-tickets", version)
		return exitOK, nil
	case "setup":
		return setup.Run(args[1:], stdin, stdout, stderr)
	case "myself":
		return runMyself(args[1:], stdout, stderr)
	case "search":
		return runSearch(args[1:], stdout, stderr)
	case "issue":
		return runIssue(args[1:], stdin, stdout, stderr)
	}
	fmt.Fprintln(stderr, "unknown command:", args[0])
	fmt.Fprint(stderr, helpText)
	return exitInputErr, errInvalidUsage
}
