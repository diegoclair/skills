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

> **v0.1.0 ships the core read + write commands.** Sprint, epic-linking, and self-update remain MCP-fallback for now (see status table below).

## Overview

Drives Claude against Jira Cloud through a local Go binary that returns **digests, JQL TSV slices, and surgical updates** instead of full ADF round-trips. The same Atlassian token used by the `confluence-docs` skill works here — both read credentials from `~/.config/atlassian/credentials` (with fallback to the per-skill files for back-compat).

## Status (v0.1.0)

| Operation | Where today |
|---|---|
| `setup` (credentials) | ✅ `jira-tickets setup` (writes to `~/.config/atlassian/credentials`, shared with `confluence-docs`) |
| Authenticate / sanity probe | ✅ `jira-tickets myself` |
| Search by JQL | ✅ `jira-tickets search "JQL"` (TSV or `--json`) |
| Read issue (slim) | ✅ `jira-tickets issue digest --key K` (~500 bytes) |
| Read issue (full) | ✅ `jira-tickets issue get --key K [--fields a,b]` |
| Create issue | ✅ `jira-tickets issue create --project K --type Task --summary "..."` |
| Update fields | ✅ `jira-tickets issue update --key K --set name=value` (repeatable) |
| List transitions | ✅ `jira-tickets issue transitions --key K` |
| Apply transition | ✅ `jira-tickets issue transition --key K --to "In Progress"` |
| Add comment | ✅ `jira-tickets issue comment --key K --body "..."` (markdown → ADF) |
| Self-update | ⏳ v0.2.0 (re-run install one-liner for now) |
| Epic add/remove child | ⏳ v0.2.0 (MCP fallback) |
| Sprint move | ⏳ v0.3.0 (MCP fallback) |
| Project list | ⏳ v0.2.0 (MCP fallback) |
| Custom-field shapes | ⏳ v0.2.0 (only flat scalars in v0.1.0) |

## Why this skill (vs MCP alone)

- `getJiraIssue` with comments routinely exceeds 25,000 tokens (Atlassian's own MCP returns full ADF body of description + every comment). Sprint-level JQL frequently breaks the limit entirely.
- Write operations against the official MCP are reported as incomplete (transition, sprint move, custom fields).
- The CLI ships **digests** (~500 bytes), **JQL TSV** (`key\tsummary\tstatus\tassignee\turl`), and **surgical updates** that never round-trip the full ADF.

## Tool priority

1. `jira-tickets` if the binary is on PATH (verify with `jira-tickets --version`) **and** the operation is in the supported list above.
2. Atlassian MCP for: project listing, epic linking, sprint membership, complex custom-field shapes.
3. Manual UI for: anything destructive (delete issue, force-transition across screens), or screen-protected transitions that require fill-in fields.

## Common workflows

**Find an issue:**
```bash
jira-tickets search 'project = PROJ AND status = "In Progress"' --limit 10
```
TSV columns: `KEY  SUMMARY  STATUS  ASSIGNEE  UPDATED  URL`.

**Read the slim summary before editing:**
```bash
jira-tickets issue digest --key PROJ-123
```
Returns ~500 bytes — enough to know what the issue is about without paying for the full ADF description.

**Update status:**
```bash
jira-tickets issue transitions --key PROJ-123          # list what's available
jira-tickets issue transition --key PROJ-123 --to "In Progress"
```

**Add a comment from markdown:**
```bash
jira-tickets issue comment --key PROJ-123 --body 'Done — see [the PR](https://github.com/.../pull/42).'
```
Inline links, bold, code, and lists all survive the markdown→ADF conversion.

**Create an issue (dry-run first to confirm):**
```bash
jira-tickets issue create \
  --project PROJ --type Task \
  --summary "Investigate flaky test in payment-svc" \
  --labels "qa,flaky" \
  --assignee 5b10a2... \
  --dry-run

# Reviewed? Drop --dry-run and run again.
```
`--dry-run` works without credentials and emits a JSON line describing exactly what would happen.

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
