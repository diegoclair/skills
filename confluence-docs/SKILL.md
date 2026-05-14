---
name: confluence-docs
version: 0.12.2
description: Navigation assistant for the project's Confluence knowledge base. Searches, creates, lists, and updates pages via natural language using the local Go CLI as the primary tool and the Atlassian MCP as fallback. Use this skill whenever the user mentions documentation, wiki, knowledge base, processes, partners, decisions, roadmap, strategy, ICPs, brand, design system, governance, or asks "where is X", "find the page for Y", "create a page for Z", "list X", "add this", "document this process", "update the page for Q", "what's the status of Y", "is there a doc about Z", "add advisor/partner/investor X" — even if they don't explicitly mention "Confluence". Stores no specific data; fresh state lives in Confluence and is fetched per session via local CLI cache. Replies match the user's language and tone.
allowed-tools: |
  Bash(confluence-docs *)
  mcp__atlassian__getConfluencePage
  mcp__atlassian__searchConfluenceUsingCql
  mcp__atlassian__getPagesInConfluenceSpace
  mcp__atlassian__getConfluencePageDescendants
  mcp__atlassian__createConfluencePage
  mcp__atlassian__updateConfluencePage
  mcp__atlassian__getConfluenceSpaces
  mcp__atlassian__search
  Read
  Write
---

# Confluence Docs — Knowledge Base Assistant

## Overview

Connects Claude to a Confluence space to search, create, list and update documentation in natural language — without manually opening Confluence. The skill is **deliberately timeless**: it stores no names, lists or state (advisors, investors, partners, page IDs). All of that lives in Confluence and is read fresh per session, starting from the Home. The `reference/` files describe procedures, not project data.

## Language rule

This document is in English because Claude reasons more robustly with English instructions. User-facing output, however, must match the user's language and tone (formal/informal). Page titles, category names, and content stay in whatever language they exist in Confluence — only technical terms and proper nouns stay in English when the user writes in another language.

## Two critical rules — internalize once

1. **Single-quote any argument that may contain `$`** (currency like `R$200` / `US$1k`, env-var-like patterns, anything money-adjacent). Bash silently expands `$200` → `00`, `$VAR` → `""`, etc., BEFORE the CLI sees the value. The failure mode is opaque ("section not found") with no obvious cause. Single quotes (`'`) keep `$` literal; double quotes (`"`) don't. Examples in this doc all use `'single quotes'` deliberately. The skill emits a diagnostic hint at error time, but prevention costs nothing.

2. **Never use `contentFormat: "markdown"` via MCP on pages with macros** (TOC, Expand, panel, status, page-properties). It silently flattens the macros and destroys structure. The CLI always handles macros correctly — prefer it.

## Tool priority — CLI first, MCP as fallback

The `confluence-docs` Go CLI talks directly to the Confluence REST API and is **always the preferred tool**. The Atlassian MCP returns the full ADF body of every page (10–40 KB); the CLI returns digests (~500 bytes), section slices (~hundreds of bytes), TSV rows (~150 bytes), or one-line status payloads. Across a multi-edit session the difference is usually 10–50× in token cost.

**Order of preference:**

1. `confluence-docs ...` if the binary exists (verify with `confluence-docs --version`).
2. **MCP Atlassian** only when the CLI can't do the job (rare: complex space exploration, attachments, comments, Jira).
3. `contentFormat: "markdown"` via MCP — last resort, **only** for pages with no macros.

The CLI handles credentials, CQL search, ADF section editing with macro preservation, and 409-conflict retry on writes — all without round-tripping bytes through the conversation.

## Reference files

The body below is the entry point — daily use, mental model, common workflows. Detailed procedures, format specs and gotchas live in `reference/`. Each reference file has a "When to read" callout at the top.

- `reference/bootstrap.md` — bootstrap procedure (how the skill orients itself per session)
- `reference/workflows.md` — generic workflows (search, read, create, update, move, delete, KM regeneration)
- `reference/doc-types.md` — canonical spec of the 5 doc types (`reference`, `decision`, `explanation`, `how-to`, `capture`), frontmatter contract, **child page title rules**, anti-patterns
- `reference/operations-matrix.md` — CLI subcommand × constraint × fail mode × workaround. **Read before chaining table/section edits** — captures the gotchas not visible from `--help` (notably: `--match-cell` always matches the first column unless `--match-col` is provided in v0.11+)
- `reference/editorial-patterns.md` — how pages should be written (Pattern 1: header; 2: Context→Problem→Solution; 3: clarity for outside readers; 4: no process meta-noise)
- `reference/features.md` — CLI features: full-width pages, `:::properties` macro, Smart Link embeds, `check`, `new`, `km`
- `reference/configuration.md` — credentials, spaces, cache lifecycle, install check, exit codes

Project-specific routing (your category structure, aliases, templates) lives on the **Confluence Home page**, fetched dynamically — the skill ships zero project-specific data.

## Bootstrap — every session

In every new session, query the Home cache directly. The CLI handles freshness — there's no TTL or session state to track.

```bash
confluence-docs home --query "advisor"     # alias / decision-map lookup
confluence-docs home --query "Programs"    # find category + pageId
confluence-docs home --digest              # outline view (~500 bytes)
confluence-docs home --show                # full text rendering
```

Reads auto-refresh when the cache is missing or older than 1h. Writes to the Home (via `page apply`) refresh the cache after PUT. You almost never need `home --refresh` explicitly.

**Use the cached Home as source of truth** for current taxonomy, "where do I put X?" decisions, aliases, and the Page ID Index. Fall back to the generic `reference/*.md` files only if both the CLI and MCP are inaccessible — those describe procedures, not current data.

**Write safety:** the cache is **read-only for navigation**. Every mutation goes through `page apply`, which GETs fresh ADF before PUT — so you never overwrite changes someone made on another machine. Don't try to PUT the cached ADF directly.

**Cost rule of thumb:** cached `home --query/--show` → `page digest` → `page get` → MCP. Escalate only when the cheaper option doesn't carry the answer.

For the full cache contract and override flags, see `reference/configuration.md` § Home cache lifecycle.

---

## Default workflows

### 1. Search — "where is X" / "is there a doc about Y?"

Resolve a term to a pageId via this ladder; stop at the first plausible match. The next `page digest` validates it (a 404, renamed title, or unrelated content tells you to fall back).

1. **Memory file or current conversation** — if a Claude memory file or the current conversation already has a pageId, **use it directly**. The follow-up `page digest` is self-validating.
2. **Cached Home** — `confluence-docs home --query "<term>"`. Local, free, fastest after memory. Single term per call.
3. **CLI search** — `confluence-docs search "<term>" --limit 5`. Used when the term isn't in the Home. Output is TSV (`pageId\ttitle\turl\texcerpt`, ~150 bytes per result). Default CQL filters by your active space; pass `--cql "..."` for full control.
4. **Confirm and return** — once you have a pageId, `page digest --page-id <id>` to verify before quoting it back.

Fallback (CLI unavailable): `mcp__atlassian__searchConfluenceUsingCql` with the same CQL filter.

### 2. Read a page — "what's in X" / "give me the page for Y"

Three-step cascade — escalate only when needed:

1. **`page digest --page-id <id>`** (~500 bytes) — outline + status. Answers most "what's in there" and "what's the status" questions. Parses leading status emoji into a `Status:` field (🟢 active · 🟡 in-progress · 🟠 evaluating · 🔴 blocked · 🔵 researched · ⚪ idle · ✅ done).
2. **`page get --section 'H' --format text`** — fetch one section as readable plain text. Use `--at-level N` when the heading repeats. Output is markdown-ish (`## headings`, `- bullets`, pipe-tables, `[text](url)`).
3. **`page get --format text`** (or `--format adf` for editing chains) — whole page, last resort. Multi-KB cost.

All `page get` calls accept `--output FILE` and `--quiet`.

### 3. Create — "create a page for Z"

1. `confluence-docs check --title "..."` — catch duplicates before creating. Suggestion is `update_existing` or `create`. See `reference/features.md` for details.
2. Use the Home's "where do I put X?" map to discover the correct parent.
3. Decide the doc type (`reference` / `decision` / `explanation` / `how-to` / `capture`) — see `reference/doc-types.md`.
4. `confluence-docs new <type> --title "..." [--parent-id ID] [--output /tmp/page.md]` — generate the template (frontmatter + headings).
5. **Confirm with the user** the final title, parent and type before creating.
6. **Title rule for child pages: don't duplicate parent context.** If the parent is `ICPs + Validation Plan`, the child title is `Personal trainer autonomous solo`, NOT `ICP — Personal trainer autonomous solo`. The Confluence sidebar already shows the breadcrumb; the prefix wastes space. See `reference/doc-types.md` § "Child page titles" for the full pattern (and the slug-vs-title distinction).
7. Fill the markdown template, then create:
   ```bash
   confluence-docs page create \
     --parent-id <parentId> --title "Final Title" \
     --markdown /tmp/page.md
   ```
   (Active space is read from config; pass `--space-id <ID>` to override.)
8. The CLI prints `{"pageId": "...", "title": "...", "url": "..."}`. Return the URL to the user.

Fallback (CLI unavailable): `mcp__atlassian__createConfluencePage` with `contentFormat: "adf"` after running `confluence-docs adf` to convert the markdown.

### 4. Update — "update the page for X" / "add section Y"

**Preferred path:** `confluence-docs page apply` does GET → section-edit → PUT in one shot, with automatic refetch-and-retry on 409 conflict. The full ADF never enters the conversation context.

```bash
# Replace a single section (fragment must include the heading line)
confluence-docs page apply --page-id <id> \
  --replace-section 'Roadmap' --fragment /tmp/new.md \
  --message "rewrite roadmap"

# Insert / append / delete
confluence-docs page apply --page-id <id> --insert-after  'Research' --fragment /tmp/new.md
confluence-docs page apply --page-id <id> --insert-before 'FAQ'      --fragment /tmp/new.md
confluence-docs page apply --page-id <id> --append                   --fragment /tmp/new.md
confluence-docs page apply --page-id <id> --delete-section 'Old TODO'

# Table operations — see reference/operations-matrix.md for constraints and decision matrix
confluence-docs page apply --page-id <id> \
  --table-add-row "Current Status" --row "Acme Corp|🟡 In progress|Source X|note" \
  --if-missing
confluence-docs page apply --page-id <id> \
  --table-update-cell "Current Status" --match-cell "Acme Corp" \
  --col-name "Status" --value "✅ Done"

# v0.11+: match a row by any column (not just column 1)
confluence-docs page apply --page-id <id> \
  --table-update-cell "Reavaliação dos ICPs" \
  --match-col "ICP" --match-value "Lash designer" \
  --col-name "Score" --value "2.4"

# v0.11+: move a row to a new absolute position (1-indexed across data rows; header stays at row 0)
confluence-docs page apply --page-id <id> \
  --table-move-row "Reavaliação dos ICPs" \
  --match-col "ICP" --match-value "Lash designer" \
  --position 10

# Preview without writing
confluence-docs page apply --page-id <id> --replace-section 'Roadmap' --fragment frag.md --dry-run
```

**Section bounds** = heading + descendants until the next heading of equal-or-higher level. Use `--at-level N` to disambiguate repeated headings.

**Table operations gotcha (critical):** `--match-cell` always matches the **first column** of the row. If the first column is not unique (numeric rank, repeating ID), the operation fails with "no row with first cell containing X". In v0.11+, add `--match-col COL --match-value VAL` to match by any column. See `reference/operations-matrix.md` for the full matrix (when to use `--table-update-cell` vs `--table-update-row` vs `--replace-section`).

**If `apply` reports "section not found"**, it lists the page's current top-level headings — correct the spelling or pick a different section. **Never blindly retry** — confirm with the user when the page structure differs from what was expected.

For two-step fallback (`get + edit + upload`) and MCP fallback, see `reference/workflows.md` § Workflow 4.

### 5. List — "what programs do we have" / "list partners"

1. Identify the category via the Home.
2. `confluence-docs page children --page-id <categoryParentId>` — TSV of `pageId\ttitle`.
3. Return as bullets ordered by title or status.

Fallback: `mcp__atlassian__getPagesInConfluenceSpace`.

### 6. Status — "what's the status of X"

`page digest` exposes a `Status:` field from the leading title emoji. For most "what's the status" questions, that's the whole answer. For richer status (labels, properties), see `reference/workflows.md` § Workflow 5. Always cite the date of the last update.

### 7. Add relationship — "add advisor/partner/investor X"

1. Verify in the Home which department/category is correct (advisor ≠ investor ≠ commercial partner — the Home's "where do I put X?" map decides).
2. Confirm template (Advisor Sheet, Investor Sheet, Partner Sheet) and run `confluence-docs new <type>`.
3. Create under the correct parent (Workflow 3). Always confirm location before.

### 8. Reorganize — rename / move / reorder / delete

Structural changes that don't touch the page body. **Always confirm deletes with the user** — even soft-delete (Confluence trash is restorable) deserves authorization.

```bash
# Rename only (parent unchanged)
confluence-docs page move --page-id <id> --title "New Title" --message "rename: reason"

# Reparent only (title unchanged) — or both at once
confluence-docs page move --page-id <id> --parent-id <newParentId>
confluence-docs page move --page-id <id> --parent-id <newParentId> --title "New Title"

# Reorder among siblings (must share parent)
confluence-docs page reorder --page-id <id> --before <siblingId>
confluence-docs page reorder --page-id <id> --after  <siblingId>

# Append as last child of a (possibly different) parent — re-parents
confluence-docs page reorder --page-id <id> --append-to <parentId>

# Soft-delete (restorable from Confluence trash)
confluence-docs page delete --page-id <id> --yes
```

**`page move` preserves the body byte-for-byte** (macros stay intact); the full ADF never enters conversation context. **`page reorder` uses the v1 endpoint** (positions: `before` / `after` / `append`); body and title aren't touched.

When you need to change both **body and title**, use `page upload --title "..."` (one PUT). When you need to change both **order and title**, run `page reorder` and `page move` as separate calls (different APIs).

For bulk reorganizations, prefer separate atomic commands — each returns a ~50-byte status payload, so even 10–20 calls stay cheap. Confirm the resulting tree with `page children` afterwards.

---

## Tool preferences

Ordered by preference. Try the cheapest tool that can answer; escalate only when necessary.

| Goal | First choice | Second choice | Last resort |
|---|---|---|---|
| Bootstrap (start of session) | `home --refresh` (one GET) | `page digest --page-id <HOME_ID>` | MCP `getConfluencePage` |
| "Where do I put X?" / aliases | `home --query "X"` (cached, free) | `home --show` + read | MCP `getConfluencePage` |
| "What's in page X?" / outline | `page digest` (~500 bytes) | `page get --section "Y" --format text` | `page get --format text` |
| "What's the status of X?" | `page digest` (Status field, 0 extra calls) | `page get --section "Status" --format text` | — |
| "What does section Y of X say?" | `page get --page-id X --section "Y" --format text` | `page get --format text` (whole page) | MCP `getConfluencePage(markdown)` |
| Find a page | `search "term"` | MCP `searchConfluenceUsingCql` | — |
| List children of a category | `page children` | MCP `getPagesInConfluenceSpace` | — |
| Update a page (single section) | `page apply` | `page get` + `edit` + `page upload` | MCP `getConfluencePage(adf)` + manual + `updateConfluencePage(adf)` |
| Update a page (table row) | `page apply --table-*` (see operations-matrix.md) | `page get` + `edit --table-*` + `page upload` | — |
| Update a single table cell | `page apply --table-update-cell` | replace whole table section | — |
| Replace whole page body | `page upload --markdown FILE` | `page upload --adf FILE` | delete+recreate (loses pageId) |
| Create a new page | `page create --markdown` | `adf` → MCP `createConfluencePage(adf)` | MCP `createConfluencePage(markdown)` |
| Create a full-width page | `page create --markdown FILE --full-width` | — | — |
| Rename a page | `page move --page-id ID --title "..."` | `page get` + `page upload --title "..."` | MCP `updateConfluencePage` |
| Move a page to a new parent | `page move --page-id ID --parent-id NEW_PARENT` | — | MCP `updateConfluencePage` |
| Reorder siblings | `page reorder --before / --after SIBLING_ID` | — | manual drag in UI |
| Append as last child of a parent | `page reorder --append-to PARENT_ID` | `page move --parent-id` (lands at first position) | manual drag in UI |
| Soft-delete a page | `page delete --page-id ID --yes` | — | MCP `deleteConfluencePage` |
| Build rich ADF from markdown | `adf` (with `[TOC]`, `:::expand`, `:::warning`, `:::properties`, smart-link extensions) | — | — |
| Check for duplicate before create | `check --title "..." --type reference` | manual `search "..."` | — |
| Generate a template for a new page | `new <type> --title "..." --output /tmp/page.md` | — | — |

**Other notes:**

- **CQL**: prefer `title ~` before `text ~`. Filter by the active space automatically (or pass `space = "<KEY>"` explicitly).
- **Macro preservation**: `page apply` and `edit` only touch the targeted section — every macro elsewhere is preserved byte-for-byte. Never use `contentFormat: "markdown"` on a page with macros.
- **Batch**: multiple reads in parallel within the same tool-call block.

## Doc types — knowledge base taxonomy

Every page belongs to one of five standard doc types. These drive both the `check` filter and the `new` template generator. Full structural contract: `reference/doc-types.md`.

| Type | Purpose | When to create |
|---|---|---|
| `reference` | Static facts about something external (PSP, competitor, partner, technology, API). The canonical "what is X" answer. | New integration, new competitor, new tool being evaluated. |
| `decision` | A choice made and why — Architecture / Product Decision Record (ADR). Immutable once accepted (supersede rather than edit). | After any significant architectural, product, or partnership decision. |
| `explanation` | Conceptual "why" — not how to do something, but what it is and why it exists. | Onboarding gaps, recurring questions, concepts used across multiple pages. |
| `how-to` | Step-by-step operational guide. Action-oriented; assumes the reader knows the concept. | Any repeatable operational task (deploy, configure, run a meeting, etc.). |
| `capture` | Quick capture: spike result, meeting note, idea, research dump. Low-polish, high-freshness. | After a spike, meeting, or discovery that needs to be logged before it disappears. |

## Updating the skill

When the user asks to update, check, or upgrade the skill ("update the skill", "is there a new version?", "check for updates"), run:

```bash
confluence-docs update            # download + install latest release
confluence-docs update --check    # only report whether an update is available
```

**Behavior:**

- Resolves the latest release tag from GitHub.
- Compares with the currently-installed binary.
- `--check`: reports `current → latest` and exits (`0` = up to date, `10` = update available).
- Without `--check`: shells out to the public installer (install.sh on Linux/macOS, install.ps1 on Windows). The installer overwrites the binary, SKILL.md, and reference files atomically. **Credentials and the home cache are preserved** — no re-setup needed.

For non-technical users, `confluence-docs update` is enough — no URL to remember. Reply in the user's language with the result, e.g. "Already on the latest version (v0.12.2)" or "Updated from v0.12.1 → v0.12.2".

**When to suggest an update proactively:** if the user reports a CLI behavior that you know was changed in a later release (e.g. they say "this flag doesn't exist" for a flag you know exists), check `confluence-docs --version` and `confluence-docs update --check` before assuming a real bug.

## Report style

- Reply in the user's language.
- Full URLs always (full URL of the Confluence instance, not just pageId).
- Concise — the team may include non-technical people.
- **Confirm exact title and location** (parent + category) before creating any page.
- Listings as bullets: `- **Title** — summary (URL)`.
- If a search is empty, suggest 2-3 variations before giving up.
