# Workflows — jira-tickets

> **When to read:** during normal use of the skill. This file documents the
> day-to-day operations the agent performs against Jira; the SKILL.md body
> stays slim and points here for detail.

## Table of contents

- [1. Find an issue](#1-find-an-issue)
- [2. Read an issue without burning tokens](#2-read-an-issue-without-burning-tokens)
- [3. Change status (transition)](#3-change-status-transition)
- [4. Add a comment](#4-add-a-comment)
- [5. Create an issue](#5-create-an-issue)
- [6. Update fields](#6-update-fields)
- [7. Bypass with `--dry-run`](#7-bypass-with---dry-run)

## 1. Find an issue

Use `search` with a JQL query. The TSV output is sub-200 bytes per row
(`KEY  SUMMARY  STATUS  ASSIGNEE  UPDATED  URL`):

```bash
jira-tickets search 'project = PROJ AND status = "In Progress" AND assignee = currentUser()' --limit 10
```

For multi-page result sets, the command prints `# next-page-token: <token>`
to stderr when there are more issues. Re-run with `--next-page-token <token>`.

When the agent needs structured fields back (e.g. to feed into another tool),
add `--json` and parse the array.

## 2. Read an issue without burning tokens

Two levels of detail:

| Need | Use | Approximate size |
|---|---|---|
| "What is PROJ-123 about?" | `jira-tickets issue digest --key PROJ-123` | ~500 bytes |
| "Show me the full ticket" | `jira-tickets issue get --key PROJ-123` | ~1–3 KB (no comments, no description body) |
| "Show me the description ADF" | `jira-tickets issue get --key PROJ-123 --json` | varies |
| "Read comments" | `jira-tickets issue get --key PROJ-123 --fields "*all,comment"` | grows with comment count |

`digest` deliberately drops `description` and `comment` from the field list.
It's the cheapest way to answer "what's this issue?" without paying for
ADF round-trip costs.

## 3. Change status (transition)

Always run `transitions` first to see what's available from the current
status — Jira transition IDs differ per project and workflow:

```bash
jira-tickets issue transitions --key PROJ-123
# ID  NAME            TO_STATUS    AVAILABLE
# 21  Start Progress  In Progress  yes
# 31  Done            Done         yes
```

Then apply by name (case-insensitive match against transition name first,
then against target-status name):

```bash
jira-tickets issue transition --key PROJ-123 --to "In Progress"
# {"status":"ok","key":"PROJ-123","transition":"Start Progress","transitionId":"21"}
```

If the name doesn't resolve, the command prints the available transitions
to stderr and exits non-zero. Use `--dry-run` to preview which transition
would be selected without applying.

## 4. Add a comment

Markdown is converted to ADF before posting — bold, links, lists, and
inline code all survive:

```bash
jira-tickets issue comment --key PROJ-123 --body 'Done — see [the PR](https://github.com/.../pull/42). Note: the test in `payment-svc` is still flaky.'
```

For longer comments, prefer `--body-file path.md` (keeps shell quoting
out of the picture). For comments built up programmatically, pipe via
`--body-stdin`:

```bash
echo "## Status update\n\nMerged at 14:23." | jira-tickets issue comment --key PROJ-123 --body-stdin
```

## 5. Create an issue

Always **dry-run first** — Jira issues are cheap to create but expensive
to clean up:

```bash
jira-tickets issue create \
  --project PROJ --type Task \
  --summary "Investigate flaky payment-svc test" \
  --labels "qa,flaky" \
  --assignee 5b10a2... \
  --priority Medium \
  --dry-run
# {"status":"dry-run","action":"create","project":"PROJ","type":"Task","summary":"..."}
```

Once the agent has confirmed with the user, drop `--dry-run` and rerun.

`--assignee` takes the Atlassian **accountId**, not email or display name.
Get yours with `jira-tickets myself`. Get someone else's via the Jira UI
(profile URL) or `search` with `assignee=currentUser()`.

## 6. Update fields

Use `--set name=value` (repeatable). List fields (`labels`, `components`)
split on comma. Other fields are sent as-is:

```bash
jira-tickets issue update --key PROJ-123 \
  --set summary="New title" \
  --set priority=High \
  --set labels=qa,flaky,p1
```

Custom fields (`customfield_*`) work for simple scalar values. Complex
shapes (multi-select, cascading, user picker with array body) are not
supported in v0.1.0 — fall back to the Atlassian MCP for those.

## 7. Bypass with `--dry-run`

Every write command (`create`, `update`, `transition`, `comment`) supports
`--dry-run`. The contract:

- Skips credential resolution entirely (no creds needed)
- Skips the HTTP call
- Prints a JSON line describing the intended action to stdout
- Exits 0

This mirrors `confluence-docs page reorder --dry-run` (introduced in
v0.11.3). Useful for: previewing an agent action with the user before
applying, scripting CI checks ("would this rule fire?"), and writing
deterministic tests without mocking HTTP.

## What's NOT in v0.1.0

These workflows fall back to the Atlassian MCP (with the usual token cost)
until later releases:

- **Epic management**: linking issues as children of an epic, removing
  children, listing epic children. Land in v0.2.0.
- **Sprint membership**: moving issues between sprints, listing sprint
  contents. Land in v0.3.0 (Agile v1 endpoints).
- **Project listing**: enumerating accessible projects + their issue
  types. Land in v0.2.0.
- **Complex custom fields**: cascading selects, multi-user pickers, etc.
  Use the MCP for these.
- **Self-update** (`jira-tickets update`): use the install one-liner to
  upgrade for now. Land in v0.2.0.
