// jira-tickets is a token-efficient CLI for Atlassian Jira Cloud,
// designed to drive Claude (and other LLM agents) without the per-call
// ADF round-trip cost of the Atlassian MCP server.
//
// Returns slim digests, JQL searches as TSV, and surgical updates.
// Status of v0.1.0: scaffold only — no real Jira commands wired yet.
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

const exitOK = 0
const exitInputErr = 2

var errInvalidUsage = errors.New("invalid usage")

const helpText = `jira-tickets — token-efficient Jira Cloud CLI for LLM agents.

USAGE:
  jira-tickets setup          [--email X --token Y | --check | --print-config-path]
  jira-tickets update         [--check]
  jira-tickets --version
  jira-tickets --help

This is a scaffold release (v0.1.0). Full commands ship in later versions:
  search "JQL" --limit N           JQL search as compact TSV
  issue digest --key PROJ-123      Slim issue summary (~500 bytes)
  issue get --key PROJ-123         Full issue or one section
  issue create / update / transition / comment / assign / link
  epic add-child / remove-child
  sprint move
  project list
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
	}
	fmt.Fprintln(stderr, "unknown command:", args[0])
	fmt.Fprint(stderr, helpText)
	return exitInputErr, errInvalidUsage
}
