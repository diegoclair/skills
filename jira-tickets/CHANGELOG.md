# Changelog — jira-tickets

## v0.3.0 — project list/get/update

Adds the `project` command group with three subcommands:

- `jira-tickets project list [--limit N] [--start-at N] [--json]` — lists projects as TSV (`KEY\tNAME\tTYPE\tID`); paginates via `--start-at`; defaults to 50 results per page (max 100).
- `jira-tickets project get KEY [--json]` — shows key, name, id, project type, lead, default assignee, simplified (yes/no), and avatar URL for a single project.
- `jira-tickets project update KEY [--name X] [--key Y] [--description Z] [--dry-run]` — renames/edits a project. New key is validated against `^[A-Z][A-Z0-9_]{1,9}$`. `--dry-run` prints the intended PUT payload and skips credential resolution. The `--help` text includes a prominent warning that renaming the key changes all issue prefixes (e.g. `SCRUM-1 → NEWKEY-1`) and breaks external references hardcoded to the old key (Jira maintains an internal redirect, but external scripts fetching by hardcoded key still work after the change).

### Implementation

Three new client methods in `pkg/atlassian/jira/client.go`: `ListProjects`, `GetProject`, `UpdateProject`. Three new types in `types.go`: `ProjectFull`, `ProjectSearchResult`, `ProjectUpdate` (pointer fields on `ProjectUpdate` so only set fields are marshaled). Four new CLI files: `cmd_project.go` (dispatcher), `cmd_project_list.go`, `cmd_project_get.go`, `cmd_project_update.go`, each with a companion `_test.go`.

---

## v0.2.0 (2026-05-14) — self-update

Adds `jira-tickets update [--check]` — the self-update subcommand parked in v0.1.0.

### How it works

Resolution uses `release.FindLatestByPrefix("diegoclair/skills", "jira-v", nil)` from the shared `pkg/atlassian/release` package — the same code path that powers `confluence-docs update` as of v0.14.0. The function queries the GitHub Releases API (`/repos/diegoclair/skills/releases?per_page=30`), filters on the client by the `jira-v` tag prefix (newest-first response order), and returns the first non-draft, non-prerelease match.

### Why a custom resolver was needed

GitHub's `/releases/latest` redirect is a singleton per repository — it points at whichever release was published with `make_latest: true`. In a monorepo that ships multiple CLIs from distinct tag prefixes (`confluence-v*` and `jira-v*`), only one product can claim that "latest" pointer. Querying the Releases list API and filtering by prefix on the client is the correct pattern for this shape (same approach used by Projektor, Streamdal, and release-please consumers).

### Rate limit

The GitHub unauthenticated REST API allows 60 requests/hour per IP — plenty for interactive `update` or `update --check` calls. CI matrices that call this frequently should pin the version explicitly via the `JIRA_TICKETS_VERSION` env var instead of resolving, to avoid hitting the ceiling.

### Version comparison

`release.NormalizeVersion` strips the tag prefix (`jira-v0.2.0` → `0.2.0`) before comparing against the ldflags-stamped binary version (which carries no prefix). The installed binary reports `v0.2.0`; the GitHub tag is `jira-v0.2.0`; after normalization both become `0.2.0` — equal.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Up to date, or upgrade completed successfully |
| 10 | `--check` only: an update is available |
| 3 | Network error or installer failure |

---

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
