# Roadmap — jira-tickets

Parked features, in rough priority order. Open a PR (or an issue) if you want to take one — they're independent.

## Shipped

- **v0.1.0** — `myself`, `search "JQL"`, `issue digest`, `issue get`, `issue create`, `issue update --set`, `issue transitions`, `issue transition`, `issue comment`.
- **v0.2.0** — `update` (self-update via GitHub API + tag prefix filter).
- **v0.3.0** — `project list`, `project get`, `project update` (rename name/key/description). ADF spec fix that unblocks `issue create --description` and `issue comment --body` against Jira Cloud v3.

## Backlog

### Epics (linking, not creating — that already works via `issue create --type Epic`)

- `jira-tickets epic add-child <EPIC-KEY> <ISSUE-KEY>` — link an existing issue to an epic. Underlying API: `PUT /rest/api/3/issue/{key}` with `fields.parent = {"key": "EPIC-KEY"}` for team-managed (next-gen) projects, or the `agile/1.0/epic/{epicKey}/issue` POST for company-managed (classic) projects. Detection via `project.simplified`.
- `jira-tickets epic list-children <EPIC-KEY>` — JQL shortcut for `parent = EPIC-KEY` plus a compact TSV.
- `jira-tickets epic remove-child <ISSUE-KEY>` — clear the parent link.

### Sprints (Agile API — `/rest/agile/1.0/...`)

- `jira-tickets sprint list <BOARD-ID>` — active / future / closed sprints with id, name, state, dates.
- `jira-tickets sprint move <ISSUE-KEY> <SPRINT-ID>` — `POST /rest/agile/1.0/sprint/{id}/issue` with `issues: [KEY]`.
- `jira-tickets sprint create <BOARD-ID> --name "..." --start ... --end ...` — start a new sprint.
- `jira-tickets sprint close <SPRINT-ID>` — transition state to `closed`.

### Backlog

- `jira-tickets backlog <BOARD-ID>` — list issues in the backlog (Agile API: `GET /rest/agile/1.0/board/{id}/backlog`), TSV like `search`.
- `jira-tickets backlog move <ISSUE-KEY> <BOARD-ID>` — move issue from sprint back to backlog.

### Boards / columns

- `jira-tickets board list` — boards visible to the user. `GET /rest/agile/1.0/board`.
- `jira-tickets board get <BOARD-ID>` — board details: type (scrum/kanban), columns, statuses per column.
- Column add/remove/rename — possible via `PUT /rest/agile/1.0/board/{id}/configuration`, but admin-only. Defer until there's real demand.

### Attachments

- `jira-tickets issue attach <ISSUE-KEY> <file>` — `POST /rest/api/3/issue/{key}/attachments` with multipart.
- `jira-tickets issue attachments <ISSUE-KEY>` — list attachments with size, mime, download URL.

### Worklogs (time tracking)

- `jira-tickets issue log <ISSUE-KEY> --time 1h30m [--comment "..."]` — `POST /rest/api/3/issue/{key}/worklog`.
- `jira-tickets issue worklogs <ISSUE-KEY>` — list worklogs.

### Users / Assignees

- `jira-tickets user search <query>` — assignee picker; `GET /rest/api/3/user/assignable/search?project=KEY&query=...`. Today the user has to pass an `accountId` to `--assignee` — finding it requires a separate trip to the UI.

### Custom fields

- Higher-level shapes for complex custom fields: cascading selects, multi-version pickers, Tempo Account, etc. Today `--set` accepts only flat string or comma-list values.

## Non-goals

- **`project create` / `project delete`** — admin-only operations done once per company lifetime. Not worth a CLI command; the Jira UI is fine for the rare case.
- **A general `jql` REPL or fuzzy search** — `search` already covers it, and Jira's UI has better autocomplete.
- **Webhook subscription management** — outside the "interactive agent" scope of this skill.
