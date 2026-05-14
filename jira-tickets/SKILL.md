---
name: jira-tickets
version: 0.1.0
description: Token-efficient Jira Cloud assistant for LLM agents. Reads, creates, updates, transitions and links Jira issues via a local Go CLI (10–50× cheaper than the Atlassian MCP for the same operations) and falls back to MCP only for the rare case the CLI can't cover. Use this skill whenever the user mentions Jira, tickets, issues, sprints, epics, story points, transitions, status changes, "move to In Progress", "create a task", "what's the status of PROJ-123", "assign to X", "what's in this sprint" — even when they don't explicitly say "Jira". Stores no project-specific data; everything is fetched fresh from Jira per session. Replies match the user's language.
allowed-tools: |
  Bash(jira-tickets *)
  mcp__atlassian__getJiraIssue
  mcp__atlassian__searchJiraIssuesUsingJql
  mcp__atlassian__createJiraIssue
  mcp__atlassian__editJiraIssue
  mcp__atlassian__transitionJiraIssue
  mcp__atlassian__addJiraComment
  Read
  Write
---

# jira-tickets — Jira Cloud assistant

> **v0.1.0 is a scaffold release.** The CLI ships only `setup`, `update`, `--version`, `--help`. Full read + write commands land in subsequent releases. Until then, the agent falls back to the Atlassian MCP for any actual Jira operation — at the usual token cost.

## Overview

Drives Claude against Jira Cloud through a local Go binary that returns **digests, JQL TSV slices, and surgical updates** instead of full ADF round-trips. The same Atlassian token used by the `confluence-docs` skill works here — both read credentials from `~/.config/atlassian/credentials` (with fallback to the per-skill files for back-compat).

## Status

| Operation | Where today |
|---|---|
| `setup` (credentials) | ✅ `jira-tickets setup` |
| `update` (self-upgrade) | ✅ `jira-tickets update` |
| Search by JQL | ⏳ MCP fallback (`searchJiraIssuesUsingJql`) |
| Read issue | ⏳ MCP fallback (`getJiraIssue`) |
| Create issue | ⏳ MCP fallback (`createJiraIssue`) |
| Update fields | ⏳ MCP fallback (`editJiraIssue`) |
| Transition status | ⏳ MCP fallback (`transitionJiraIssue`) |
| Add comment | ⏳ MCP fallback (`addJiraComment`) |
| Epic / sprint ops | ⏳ MCP fallback or manual UI |

Each release fills more rows of the table. The current skill body intentionally stays slim so it can grow.

## Why this skill (vs MCP alone)

- `getJiraIssue` with comments routinely exceeds 25,000 tokens (Atlassian's own MCP returns full ADF body of description + every comment). Sprint-level JQL frequently breaks the limit entirely.
- Write operations against the official MCP are reported as incomplete (transition, sprint move, custom fields).
- The CLI ships **digests** (~500 bytes), **JQL TSV** (`key\tsummary\tstatus\tassignee\turl`), and **surgical updates** that never round-trip the full ADF.

## Tool priority

1. `jira-tickets` if the binary is on PATH (verify with `jira-tickets --version`) **and** the operation is in the supported list above.
2. Atlassian MCP otherwise.

## Setup

```bash
jira-tickets setup                                    # interactive wizard
jira-tickets setup --email X --token Y                # non-interactive
jira-tickets setup --check                            # exit 0 = valid creds
```

If `confluence-docs` is already configured on this machine, `jira-tickets setup` reuses the same Atlassian credentials at `~/.config/atlassian/credentials` (or the legacy `~/.config/confluence-docs/credentials` as fallback). No need to paste the API token twice.

## Update

```bash
jira-tickets update                                   # download + install latest
jira-tickets update --check                           # report current vs latest
```

The on-disk binary and skill files refresh in place; credentials are preserved.

## Reference

Detailed workflows and command reference will land here as features ship:

- `reference/workflows.md` (TBD) — read/create/update/transition flows
- `reference/operations-matrix.md` (TBD) — CLI command × constraint × fail mode
- `reference/install-for-ai.md` (TBD) — AI-agent install runbook
