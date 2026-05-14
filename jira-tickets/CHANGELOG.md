# Changelog — jira-tickets

## v0.1.0 (2026-05-14) — initial release with core read + write commands

First public release of `jira-tickets`. Reuses the shared `pkg/atlassian` module (HTTP client, ADF parsing, credentials setup) from `confluence-docs` in the same monorepo.

### What ships in v0.1.0

**Read operations** — all returning slim outputs designed for LLM context budgets:

- `jira-tickets myself` — authenticated user info; cheapest sanity probe
- `jira-tickets search "JQL" [--limit N] [--fields a,b,c] [--next-page-token T] [--json]` — JQL search returning TSV (`KEY  SUMMARY  STATUS  ASSIGNEE  UPDATED  URL`) or JSON; cursor pagination via `--next-page-token`
- `jira-tickets issue digest --key K [--json]` — ~500-byte summary (no ADF body, no comments); the canonical "what is this ticket about?" call
- `jira-tickets issue get --key K [--fields a,b] [--json]` — full issue with optional field-list slicing; ADF description rendered as a char-count placeholder unless `--json`
- `jira-tickets issue transitions --key K [--json]` — TSV of transitions available from the current status (`ID  NAME  TO_STATUS  AVAILABLE`)

**Write operations** — every mutating command supports `--dry-run` (mirrors the contract `confluence-docs page reorder --dry-run` established in v0.11.3: prints intended JSON action, skips credential resolution, exits 0):

- `jira-tickets issue create --project K --type T --summary "..." [--description X | --description-file PATH] [--labels a,b] [--assignee ID] [--parent K] [--due-date YYYY-MM-DD] [--priority NAME] [--dry-run] [--json]`
- `jira-tickets issue update --key K --set name=value [--set ...] [--dry-run]` — repeatable `--set`; list fields (labels, components) split on comma
- `jira-tickets issue transition --key K --to "Status Name" [--dry-run]` — case-insensitive match against transition name then target status name; on miss, prints available transitions to stderr
- `jira-tickets issue comment --key K (--body "..." | --body-file PATH | --body-stdin) [--dry-run] [--json]` — markdown body converted to ADF via `pkg/atlassian/adf.Convert` before posting

**Plumbing**:

- `jira-tickets setup [--email X --token Y | --check | --print-config-path]` — delegates to `pkg/atlassian/setup.Run`. Credentials at `~/.config/atlassian/credentials`, shared atlassian-wide; same token works for `confluence-docs`. Legacy fallback chain for old per-skill paths.

### Token-cost rationale

The Atlassian MCP server returns the full ADF body of every `getJiraIssue` call. Issues with ~10+ comments routinely exceed 25,000 tokens — past the Cursor / Claude Code per-call limit. The `digest` command was sized specifically for this: by passing `fields="summary,status,issuetype,priority,assignee,reporter,parent,labels,updated,duedate"` (no `description`, no `comment`) the response is sub-500 bytes. Same logic for `search` — the TSV columns deliberately omit description so a JQL with 50 hits returns ~7 KB instead of ~1 MB.

### Architecture

- `pkg/atlassian/jira/` (632 LoC + 379 LoC types + 1265 LoC tests) — the REST v3 client. Idempotent GETs retry up to 3× on 5xx with exponential back-off. Write methods execute exactly once. Errors unwrap from Atlassian's `{"errorMessages":[…], "errors":{…}}` envelope into `*APIError` with `IsNotFound` / `IsUnauthorized` / `IsConflict` helpers.
- `jira-tickets/cli/` — thin CLI layer. Each command is its own file (`cmd_search.go`, `cmd_issue_digest.go`, …). `cmd_issue.go` dispatches issue verbs.
- `cli/helpers.go` — `buildClient` resolves cloud + creds from flag/env/file chain; mirrors the Confluence helper.

### What does NOT ship in v0.1.0 (parked for v0.2+)

- **Self-update** (`jira-tickets update`): the `/releases/latest` redirect path is already used by `confluence-docs` in the same repo. Adding it correctly needs a GitHub API + tag-prefix filter (`jira-v*`). For now, re-run the install one-liner to upgrade: `curl -fsSL https://raw.githubusercontent.com/diegoclair/skills/main/jira-tickets/install/install.sh | bash`.
- **Project listing** (`jira-tickets project list`).
- **Epic add-child / remove-child** (link issues under an epic).
- **Sprint move** (move issue between sprints — Agile v1 endpoint).
- **Complex custom-field shapes** (only flat scalar `customfield_*` values are supported in `--set`).

These are documented in `SKILL.md` and fall back to the Atlassian MCP for now.

### Distribution

- Released via tag `jira-v0.1.0` (sibling to `confluence-v*` in the same monorepo).
- CI workflow `.github/workflows/release-jira.yml` cross-compiles 5 platform zips (darwin amd64/arm64, linux amd64/arm64, windows amd64) on tag push.
- Install one-liner: `curl -fsSL https://raw.githubusercontent.com/diegoclair/skills/main/jira-tickets/install/install.sh | bash`
- Marks `make_latest: false` so `/releases/latest` keeps pointing at the more-mature `confluence-docs` for now.

### Tests

- `pkg/atlassian/jira`: 45 tests covering every client method, retry behavior, error envelope parsing.
- `jira-tickets/cli`: ~60 tests covering help, missing-args, dry-run JSON shape, empty-creds path across all 8 commands.

### Why no separate "scaffold" release?

Originally batch 1 of this session landed a "scaffold v0.1.0" tag that would only respond to `--version` / `--help` / `setup`. As batches 2–5 of the same session filled in actual commands, it made more sense to publish one real v0.1.0 than to ship a scaffold tag the world would have to skip past.
