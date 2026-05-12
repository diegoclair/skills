---
name: confluence-docs
description: |
  Navigation assistant for your Confluence knowledge base — search, create, list, update pages. Aliases: confluence, wiki, kb, knowledge base, docs. Use for any project documentation: processes, partners, decisions, roadmap, strategy, design system, governance, or any organizational artifact — even when "Confluence" isn't mentioned. Triggers: "where is X", "find the page for Y", "create a page for Z", "list X", "what's the status of Y", "is there a doc about Z", "add this", "document this process", "update the page for Q", "add advisor/partner/investor", "log this in the kb". Stores no specific data — fresh state lives in Confluence, fetched at session start via local CLI cache (preferred) or Atlassian MCP (fallback). Replies using the same language as the user.
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

Skill that connects Claude to your Confluence space to search, create, list and update documentation in natural language — without manually opening Confluence.

The skill is **deliberately timeless**: it stores no names, lists or state (advisors, investors, partners, accelerators, page IDs). All of that lives in Confluence and is read fresh in every session starting from the Home. The `reference/` files here are generic fallback, not the source of truth.

## Language rule

**This document is in English for Claude's performance (Claude is trained primarily on English data, and English instructions yield more robust reasoning). However, all user-facing output MUST match the user's language and match their tone (formal/informal as they write).**

When you respond to the user:
- Use the same language the user writes in
- Keep page titles, category names, and content in whatever language they exist in Confluence
- Only technical terms or proper nouns stay in English when the user writes in another language

## Tool priority — CLI first, MCP as fallback

The `confluence-docs` Go CLI talks directly to the Confluence REST API and is **always the preferred tool** for reads, writes and searches. The Atlassian MCP returns the full ADF body of every page (tens of KB), which inflates the conversation context fast. The CLI returns a small digest, a TSV row, or a one-line status — orders of magnitude cheaper.

**Order of preference for any operation:**
1. **`confluence-docs ...`** if the binary exists (check with `confluence-docs --version`).
2. **MCP Atlassian** only when the CLI cannot do the job (rare cases: complex space exploration, attachments, comments, Jira).
3. **`contentFormat: "markdown"` via MCP** — last resort, ONLY for pages with no macros (it flattens TOC/expand/panel and silently destroys structure).

The CLI also handles credentials, CQL search, ADF section editing with macro preservation, and a 409-conflict retry on writes — all without round-tripping bytes through the conversation.

## Mandatory bootstrap

In **EVERY new session**, just use the Home commands directly. The CLI handles freshness — you don't track TTLs, sessions, or cache state.

1. **Read the cache for navigation — auto-refreshes when stale:**
   ```
   confluence-docs home --query "advisor"           # alias/decision lookup
   confluence-docs home --query "Programs"          # find category + pageId
   confluence-docs home --digest                    # outline view (~500 bytes)
   confluence-docs home --show                      # full text rendering
   ```
   These commands auto-refresh the cache when it's missing or older than 1h. You don't need to run `home --refresh` first. Most navigation answers are in the cache, returning a few hundred bytes per query.

2. **Use the cached Home content as source of truth** for:
   - Current taxonomy (categories and sub-structures)
   - "Where do I put X?" decision map
   - Aliases (keywords → pages)
   - Page ID Index (if present)
   - Organization rules

3. **Fall back to the generic reference files** in `reference/` (`bootstrap.md`, `workflows.md`, `doc-types.md`) **only if both the CLI and MCP are inaccessible** — they describe procedures, not current data.

### Cache lifecycle (the contract)

The cache lives at `~/.cache/confluence-docs/home.json` — **a single file shared across all Claude sessions on the same machine**. So if one session refreshes (manually or via auto-refresh-on-write), every other session reading it next sees the updated state automatically. No per-session bookkeeping.

Three rules govern when the cache is updated:

| Trigger | Behavior |
|---|---|
| Read with stale cache (>1h old) or missing | **Auto-refresh** before serving. Caller doesn't have to think about it. |
| Write to the Home via CLI (`page apply`, `index add/remove/sync` on the Home pageId) | **Auto-refresh after PUT** succeeds. Your session sees the new state immediately. |
| Explicit `home --refresh` | **Always fetches**, ignores TTL. Use only when you know another machine just edited the Home and you don't want to wait for the TTL. |

What this means in practice: in a typical session, you never call `home --refresh` explicitly. You just query/show/digest, and writes refresh themselves.

**WRITE SAFETY (critical):** the cache is **read-only for navigation**. It is **NEVER** the source for an update. Any mutation of the Home (or any page) goes through `page apply`, which always GETs fresh ADF before PUT — ensuring you never overwrite changes someone made on another machine.

This skill is deliberately **timeless**. It stores no specific names (advisors, investors, partners, accelerators) — everything comes fresh from Confluence whenever you query, with the cache layer making it cheap.

**Token-cost rule of thumb:** prefer cached `home --query/--show` over `page digest` over `page get` over MCP. Only escalate to a full read when the cheaper option doesn't carry the answer.

## Reference files

- `reference/bootstrap.md` — principle + bootstrap procedure (how the skill orients itself per session)
- `reference/doc-types.md` — canonical spec of the 5 doc types (`reference`, `decision`, `explanation`, `how-to`, `capture`), frontmatter, anti-patterns
- `reference/workflows.md` — generic workflows (search, read, create, update, move, delete, regenerate KM)

Project-specific routing (your category structure, aliases, templates) lives on **your Confluence Home page**, fetched dynamically — the skill ships zero project-specific data.

## Default workflows

### 1. Search — "where is X" / "is there a doc about Y?"

**Source-of-truth ladder for resolving a term to a pageId.** Try in order; stop at the first one that returns a plausible match. The next `page digest` call functions as the verification step — if the title or content doesn't match expectations, fall back to the next rung.

1. **Memory file or recent conversation context.** If a Claude memory file (e.g. `project_X_relationship.md`) or the current conversation already has a pageId for the term, **use it directly**. Skip steps 2–3. The follow-up `page digest` validates it: a 404, a renamed title, or unrelated content tells you the memory is stale, and you fall back to step 2.

2. **Cached Home (`home --query`).** Local, free, fastest after memory:
   ```
   confluence-docs home --query "<term>"
   ```
   Hits the Page ID Index, aliases, and "Where do I put X?" decision map. Returns matching lines grouped by section.

   **Single term per call** — no OR / regex / multi-term syntax. To search multiple terms, run the command multiple times (each call is local and cheap, zero API cost). Same applies to `--show` + `grep` if you need a richer pattern.

3. **CLI search (`search`).** When the term isn't in the Home (new pages, niche topics):
   ```
   confluence-docs search "<term>" --limit 5
   ```
   Output is TSV: `pageId<TAB>title<TAB>url<TAB>excerpt`. ~150 bytes per result. The default CQL is `space="<YOUR_SPACE>" AND type="page" AND (title ~ "term" OR text ~ "term")`. Pass `--cql "raw CQL"` for fine control.

4. **Confirm and return.** Once you have a pageId, call `page digest --page-id <id>` to verify the page exists and matches the user's intent before quoting it back. If returning multiple results from `search`, format as bullets: `- **Title** — excerpt (URL)`.

**Fallback** (CLI unavailable): `mcp__atlassian__searchConfluenceUsingCql` with the same CQL filter.

**Why the memory-first rung exists:** memory files capture stable IDs that survive across sessions. Going straight to `page digest` skips two steps with no downside — the digest is cheap and self-validating. Don't over-think it: if you have an ID and a name that matches what the user asked, just check the digest.

### 2. Read a page — "get the page for Y" / "what's in Z"

**Three-step cascade**, escalate only when needed:

**Step 1 — `digest` (cheapest, ~500 bytes).** Most questions can be answered from the outline alone:

```
confluence-docs page digest --page-id <id>
```

The digest also carries a `Status` line when the title starts with a status emoji (see the **Optional status-emoji pattern** in the Tool preferences section below). For "what's the status of X" questions, this single field is often the entire answer. Add `--json` to get a structured object.

**Step 2 — `get --section "Heading" --format text` (one section, ~hundreds of bytes).** When the digest tells you which section has the answer, fetch just that section as readable plain text:

```
confluence-docs page get --page-id <id> --section "Status" --format text
```

Section bounds = heading + all following nodes until the next heading of equal-or-higher level (so an h2 includes its h3 children). Use `--at-level N` to disambiguate when the same heading text appears at multiple levels. Output is markdown-ish (`## headings`, `- bullets`, pipe-tables, `[text](url)` links) — readable by a human and trivially parseable by an agent.

**Step 3 — `get` whole page (last resort, multi-KB).** Only when you genuinely need the full content (e.g. you'll be editing several sections):

```
confluence-docs page get --page-id <id> --format text         # whole page as text
confluence-docs page get --page-id <id> --format adf          # raw ADF (only if editing)
confluence-docs page get --page-id <id> --format export_view  # rendered HTML
```

**`--output FILE` and `--quiet`** are available on all `page get` invocations: `--output` writes to disk instead of stdout; `--quiet` suppresses the "wrote N bytes" stderr message. Use `--quiet` when the caller captures both streams.

### 3. Create — "create a page for Z"

1. Run `confluence-docs check --title "..."` first to surface near-duplicates.
2. Use the Home's "where do I put X?" map (or equivalent on your project) to discover the correct parent page.
3. Decide the doc type (`reference` / `decision` / `explanation` / `how-to` / `capture`) — see `reference/doc-types.md`.
4. Generate a template: `confluence-docs new <type> --title "..." [--parent-id ID]`.
5. **Confirm with the user** the final title, parent and type before creating.
6. Write the content as markdown to a temp file. The CLI's `adf` converter supports Confluence macros (`[TOC]`, `:::expand`, `:::warning`, `:::properties`, etc.) via extended markdown.
5. Create directly via the CLI (single command, no MCP round-trip):
   ```
   confluence-docs page create \
     --space-id <SPACE_ID> --parent-id <parentId> --title "Final Title" \
     --markdown /tmp/page.md
   ```
6. The CLI prints `{"pageId": "...", "title": "...", "url": "..."}`. Return the final URL to the user.

**Fallback** (CLI unavailable): `mcp__atlassian__createConfluencePage` with `contentFormat: "adf"` after running `confluence-docs adf` to convert the markdown. Last resort: `contentFormat: "markdown"`.

### 4. Update — "update the page for X" / "add section Y"

**Never build ADF by hand, and never use `contentFormat: "markdown"` to update a page with macros** (TOC, Expand, panel). Markdown update flattens macros and silently destroys structure.

**Preferred path (single atomic command):** `confluence-docs page apply` does GET → section-edit → PUT in one shot, with automatic refetch-and-retry on 409 conflict (someone else updated mid-flight). The full ADF never enters the conversation context.

```
# Replace a single section, preserving every macro outside it
confluence-docs page apply --page-id <id> \
  --replace-section "Roadmap" --fragment /tmp/new.md \
  --message "rewrite roadmap"

# Append a new section at the end
confluence-docs page apply --page-id <id> \
  --append --fragment /tmp/new.md \
  --message "add Q3 retrospective"

# Insert relative to an existing heading
confluence-docs page apply --page-id <id> \
  --insert-after "Research" --fragment /tmp/new.md

confluence-docs page apply --page-id <id> \
  --insert-before "FAQ" --fragment /tmp/new.md

# Delete a stale section
confluence-docs page apply --page-id <id> --delete-section "Old TODO"

# Disambiguate when the same heading text appears at multiple levels
confluence-docs page apply --page-id <id> \
  --replace-section "Ops" --at-level 3 --fragment /tmp/new.md

# Add a row to a table inside a section (idempotent with --if-missing)
confluence-docs page apply --page-id <id> \
  --table-add-row "Current Status" --row "Acme Corp|🟡 In progress|Source X|note" \
  --if-missing --message "add Acme to status table"

# Cells with literal pipes: escape with backslash
confluence-docs page apply --page-id <id> \
  --table-add-row "Endpoints" --row "GET /api/v1\|v2|public|200ms"
# → cells: ["GET /api/v1|v2", "public", "200ms"]

# Remove a row by matching cell text
confluence-docs page apply --page-id <id> \
  --table-remove-row "Current Status" --match-cell "Acme Corp"

# Update a single cell (surgical — preserves the rest of the table)
confluence-docs page apply --page-id <id> \
  --table-update-cell "Current Status" --match-cell "Acme Corp" \
  --col-name "Status" --value "✅ Done"

# Replace an entire row (for multi-cell updates)
confluence-docs page apply --page-id <id> \
  --table-update-row "Current Status" --match-cell "Acme Corp" \
  --row "Acme Corp|✅ Done|Source X|closed"

# Preview without writing
confluence-docs page apply --page-id <id> \
  --replace-section "Roadmap" --fragment frag.md --dry-run
```

For section replacement, include the heading line in the fragment markdown. Section bounds = heading + all following top-level nodes until the next heading of equal-or-higher level (h2 closes at h2 or h1; h3 closes at h3, h2 or h1).

**If `apply` reports "section not found"**, the command also lists the page's current top-level headings so you can correct the spelling or pick a different section. **Never blindly retry** — confirm with the user when the page structure differs from what was expected.

**Two-step fallback** (when `apply` is unavailable but `edit` is):
1. `confluence-docs page get --page-id <id> --format adf --output /tmp/current.json`
2. `confluence-docs edit -i /tmp/current.json --replace-section "..." /tmp/frag.md > /tmp/new.json`
3. `confluence-docs page upload --page-id <id> --adf /tmp/new.json --message "..."`

**MCP fallback** (when no CLI at all):
1. `getConfluencePage(contentFormat="adf")` → save body to disk
2. Convert + edit ADF manually (don't do this — install the CLI instead)
3. `updateConfluencePage(contentFormat="adf", body=...)`

If the page has macros and no CLI is installed, warn the user that updating via `markdown` may destroy structure and ask them to install the CLI first.

### 5. List — "what programs do we have" / "list partners"

1. Identify the category via the Home.
2. Use the CLI to list direct children:
   ```
   confluence-docs page children --page-id <categoryParentId>
   ```
   Output: `pageId<TAB>title` per line.
3. Return as bullets ordered by title or status.

**Fallback**: `mcp__atlassian__getPagesInConfluenceSpace` or `getConfluencePageDescendants`.

### 6. Status — "what's the status of X"

For most "what's the status" questions, the digest is enough — it shows the page's headings, last version number, and which macros are present (status macros show up). For richer status (labels, properties), see Workflow 5 in `reference/workflows.md`. Always cite the date of the last update.

### 7. Add relationship — "add advisor/partner/investor X"

1. Verify in the Home which department/category is correct (advisor ≠ investor ≠ commercial partner).
2. Confirm template (Advisor Sheet, Investor Sheet, Partner Sheet).
3. Create under the correct parent (Workflow 3). Always confirm location before.

### 8. Reorganize — "rename X" / "move X under Y" / "reorder X" / "delete X"

The `page move`, `page reorder`, and `page delete` verbs handle structural changes without touching the page body. Use them when the user asks to rename, reparent, reorder, or trash a page. **Always confirm the target with the user before deleting** — even soft delete (Confluence trash is restorable, but the user should still authorize).

```
# Rename only (parent unchanged)
confluence-docs page move --page-id <id> --title "New Title" --message "rename: reason"

# Reparent only (title unchanged)
confluence-docs page move --page-id <id> --parent-id <newParentId> --message "move under X"

# Both at once (single PUT)
confluence-docs page move --page-id <id> --parent-id <newParentId> --title "New Title"

# Preview without writing
confluence-docs page move --page-id <id> --title "..." --dry-run

# Reorder among siblings (same parent, just change position)
confluence-docs page reorder --page-id <id> --before <siblingId>
confluence-docs page reorder --page-id <id> --after  <siblingId>

# Append as last child of a (possibly different) parent — re-parents
confluence-docs page reorder --page-id <id> --append-to <parentId>

# Soft-delete (restorable from Confluence trash)
confluence-docs page delete --page-id <id> --yes
```

**How move works under the hood:** v2 PUT requires the body, so `page move` GETs the current ADF and re-PUTs it with the new `parentId` / `title`. Body is preserved byte-for-byte; macros stay intact. The full ADF never enters the conversation context.

**How reorder works under the hood:** v2 doesn't expose sibling-order control, so `page reorder` calls the v1 endpoint `PUT /wiki/rest/api/content/{id}/move/{position}/{targetId}` (positions: `before` | `after` | `append`). The body and title aren't touched — single round-trip, no GET needed.

**When NOT to use:**
- To change the **content** of a page → use `page apply` (section/table edit) or `page rewrite` / `page upload` (full body replacement).
- To change the body **and** rename in one shot → use `page upload --title "..."` (you already have a new ADF) instead of two separate calls.
- To change order **and** rename in one shot → run `page reorder` and `page move` as two separate calls (different APIs).

**Bulk reorganizations:** when reparenting multiple pages or doing a multi-step restructure (rename + move + reorder + delete cascade), prefer running each `page move` / `page reorder` / `page delete` as its own atomic command. The CLI returns a one-line JSON status per call (~50 bytes), so even 10–20 calls stay cheap. Confirm the new tree with `page children` afterwards.

## Editorial patterns

Every **decision**, **proposal**, **strategy**, or **spec** page must follow two conventions. Index pages, reference pages and glossaries can skip them.

### Pattern 1 — Page header

Start every page with a small header block (quote/callout) containing:

> **Context:** (one-line summary of what the page is about)
> **Created:** YYYY-MM-DD | **Updated:** YYYY-MM-DD | **Author:** [name]

When updating a page, bump the "Updated" date. On first creation, author comes from the current user (confirm if unknown).

### Pattern 2 — Context → Problem → Solution structure

For decision/proposal/strategy pages, organize the body around three sections:

1. **Context** — where the page comes from, which project/moment it serves, what bigger scope it sits in
2. **Problem** — what is being solved, what hurts, what constraints exist
3. **Solution (proposed)** — proposed approach, options, tradeoffs, concrete scope

See `reference/doc-types.md` § Section 2 (Decision type) for the canonical structure.

**Why this matters**: the knowledge base becomes predictable — any reader (human or AI) knows where to find the motivation, the pain, and the plan without re-reading everything. Reduces onboarding cost and review time.

**When creating a page**: apply these patterns by default; deviate only if the page type is clearly an index/reference/glossary.

**When updating an existing page** that violates the patterns: offer the user a refactor alongside the content change — don't silently rewrite structure.

### Pattern 3 — Clarity for outside readers

Pages are read by people who weren't in the conversation that generated them. Before saving:

- **Technical jargon** (Merchant of Record, Variant A, MCP, commission fee, postback, etc.) must be explained briefly on first use OR linked to a page that explains. Don't assume context.
- **Internal labels** (Variant A, Wave 2, v0.1) should come with a short gloss when first introduced on a page.
- **References to other work** (research, analysis, previous decisions) must be linked to the actual Confluence page — never just mention "we analyzed X" without linking X.
- **Balance**: don't over-explain the obvious (readers are smart, just not context-equipped). One short clause or parenthetical is usually enough. Prefer linking over inlining when the full explanation exists elsewhere.

**Smell test**: if a new team member opens this page cold, will they understand it? If not, add one link or one half-sentence.

### Pattern 4 — No process meta-noise. Pretend prior versions never existed.

The page is the current state of thinking — not a record of how it got there. Avoid documenting your own editing process inside the page body. Specifically, **never write**:

- ❌ "Replaced the previous version that had [problem]" / "This version supersedes the previous" / "Rewritten after re-research"
- ❌ "Was X, now is Y" / "Changed from A to B" (in the body of the page, as if the reader cared about the diff)
- ❌ "v1 → v2" comparisons or any version numbering inside content
- ❌ Apologetic notes about previous errors: "the CEO name was previously wrong", "was written in future tense before"
- ❌ Refactoring announcements: "this page was refactored today", "was a sub-section, now its own page"

**Why this rule:** today's "v1 vs v2" becomes tomorrow's confusion when v3 arrives. A reader 6 months from now doesn't care that the wrong name was once written here — they care if the right one is here now. **When fixing a factual error, just fix it. Pretend it never existed.** When restructuring, just restructure. Git history (or the page's own version history in Confluence) is the audit trail; the page body is the source of truth.

**History section — what to put and what NOT to put.** When a page has a "History" / "Decision History" section, it records **substantive movements of the decision/state being documented** — not edits to the page itself. Use it for:

- ✅ A real strategic pivot ("Decision changed from X to Y after data Z came in") — but write it from the perspective of the *decision*, not the *page*.
- ✅ Regulatory or external events that shifted the page's conclusions ("Authority X published Resolution Y; we updated eligibility accordingly")
- ✅ A consolidated handoff moment ("Phase 0 closed on date X; this page archived its conclusions")

Do NOT use it for:

- ❌ "Refactored today, structure simplified" — that's an edit, not a movement
- ❌ "Morning version replaced by afternoon version" — same intra-day; the reader doesn't experience time as you do
- ❌ "Added section Y / removed section Z" — that's git history, not decision history
- ❌ Author or model self-references — "Sonnet researched", "AI agent updated"

**Smell test for History entries:** if you remove the entry, does the reader lose business context they'd otherwise have to ask about? If no, delete it. Many pages don't need a History section at all.

**Smell test for the rest of the body:** read the page as if you'd never seen it before. Anything that talks *about the page itself* (its versions, its corrections, its refactorings) is noise — strip it. The page is what it says, not how it got there.

## Tool preferences

Ordered by preference. Always try the cheapest tool that can answer the question — escalate only when necessary.

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
| Update a page (table row) | `page apply --table-add-row` / `--table-remove-row` / `--table-update-row` | `page get` + `edit --table-*` + `page upload` | — |
| Update a single table cell | `page apply --table-update-cell --match-cell ROW --col-name COL --value V` | replace whole table section | — |
| Replace whole page body (full-body rewrite) | `page upload --markdown FILE` | `page upload --adf FILE` | delete+recreate (loses pageId) |
| Create a new page | `page create --markdown` | `adf` to convert + MCP `createConfluencePage(adf)` | MCP `createConfluencePage(markdown)` |
| Create a new page (full-width) | `page create --markdown FILE --full-width` | — | — |
| Rename a page | `page move --page-id ID --title "New Title"` | `page get` + `page upload --title "..."` | MCP `updateConfluencePage` |
| Move a page to a new parent | `page move --page-id ID --parent-id NEW_PARENT` | — | MCP `updateConfluencePage` |
| Reorder siblings | `page reorder --page-id ID --before/--after SIBLING_ID` | — | manual drag in UI |
| Append as last child of a parent | `page reorder --page-id ID --append-to PARENT_ID` | `page move --parent-id` (lands at first position) | manual drag in UI |
| Soft-delete a page | `page delete --page-id ID --yes` | — | MCP `deleteConfluencePage` |
| Build rich ADF from markdown | `adf` (with `[TOC]`, `:::expand`, `:::warning`, `:::properties`, smart-link extensions) | — | — |
| Check for duplicate before create | `check --title "..." --type reference` | manual `search "..."` | — |
| Generate a template for a new page | `new reference --title "..." --output /tmp/page.md` | — | — |

**Why CLI first:** the MCP returns the full ADF body of every page (10–40 KB). The CLI returns digests (~500 bytes), single-section slices (~hundreds of bytes), TSV rows (~150 bytes per result), or one-line status payloads. Across a multi-edit session the difference is usually 10–50× in token cost.

**Optional status-emoji pattern:** prefix page titles with a status emoji — 🟢 (active), 🟡 (in-progress), 🟠 (evaluating), 🔴 (blocked), 🔵 (researched), ⚪ (idle), ✅ (done) — and the CLI's `digest` parses it into a `Status: <emoji> <label>` line in text output (and `"status"` + `"statusEmoji"` fields in `--json`). Use this convention in your project if you find it useful for cheap status queries — it's not required.

**Other notes:**
- **CQL**: prefer `title ~` before `text ~`. **Always** filter by your space key: `space = "<YOUR_SPACE>"`.
- **Batch:** multiple reads in parallel within the same tool-call block.
- **Macro preservation**: `page apply` and `edit` only touch the targeted section; every macro elsewhere on the page is preserved byte-for-byte. Never use `contentFormat: "markdown"` on a page with macros.

## Doc types — knowledge base taxonomy

Every Confluence page belongs to one of five standard doc types. These types drive both the `check` filter and the `new` template generator.

| Type | Purpose | When to create |
|---|---|---|
| `reference` | Static facts about something external (PSP, competitor, partner, technology, API). The canonical "what is X" answer. | New integration, new competitor, new tool being evaluated. |
| `decision` | A choice made and why — Architecture / Product Decision Record (ADR). Immutable once published (supersede rather than edit). | After any significant architectural, product, or partnership decision. |
| `explanation` | Conceptual "why" — not how to do something, but what it is and why it exists. | Onboarding gaps, recurring questions, concepts used across multiple pages. |
| `how-to` | Step-by-step operational guide. Action-oriented; assumes the reader knows the concept. | Any repeatable operational task (deploy, configure, run a meeting, etc.). |
| `capture` | Quick capture: spike result, meeting note, idea, research dump. Low-polish, high-freshness. | After a spike, meeting, or discovery that needs to be logged before it disappears. |

## Mandatory workflow: check before create

**ALWAYS run `confluence-docs check` before creating a new page or running `new`.** This catches exact duplicates and near-duplicates before they clutter the space.

```
# 1. Check first
confluence-docs check --title "Stripe Brazil Analysis" --type reference

# Output example:
# { "exists": false, "similar": [...], "suggestion": "create" }

# 2. If suggestion == "update_existing" → update the existing page instead.
# If suggestion == "create" → proceed to generate the template.

# 3. Generate template
confluence-docs new reference \
  --title "Stripe Brazil Analysis" \
  --output /tmp/page.md

# 4. Edit the template (fill in the blanks), then create:
confluence-docs page create \
  --space-id <SPACE_ID> --parent-id <PARENT_ID> \
  --title "Stripe Brazil Analysis" \
  --markdown /tmp/page.md
```

The `check` command uses trigram-based fuzzy matching (Jaccard similarity, threshold 0.7 by default). It also accepts `--tags` to filter by Confluence labels and `--threshold` to tighten or loosen the match.

## `confluence-docs km` — Knowledge Map generator

Generates a KNOWLEDGE_MAP page from triage JSON batches produced by subagents, with optional hand-classified baseline overrides.

### Typical agent workflow

```
# 1. Subagent triage writes batch-*.json into /tmp/km-triage/
# 2. Consolidate and upload:
confluence-docs km generate \
    --input /tmp/km-triage \
    --baseline /tmp/baseline.json \
    --target-page-id <KM_PAGE_ID> \
    --message "regenerate KM after triage" \
    --full-width
```

### Dry-run (review before upload)

```bash
# Render to stdout only:
confluence-docs km generate --input /tmp/km-triage

# Render to file:
confluence-docs km generate --input /tmp/km-triage --output /tmp/km.md

# Dry-run with target page: shows size but skips upload:
confluence-docs km generate \
    --input /tmp/km-triage \
    --target-page-id <KM_PAGE_ID> \
    --dry-run
```

### Baseline format (`--baseline FILE`)

```json
{
  "pages": [
    {"pageId": "185303042", "title": "About the Project", "tipo": "reference", "tags": []},
    {"pageId": "187695141", "title": "Current Fit Proposal", "tipo": "decision", "tags": ["phase-mvp"]}
  ]
}
```

Baseline entries take precedence over triage (type and title are never overridden). Triage can still **augment** baseline entries — e.g. adding a tag or a real anomaly.

### Triage batch format (`batch-*.json` files)

```json
[
  {
    "pageId": "131676",
    "title": "Business Model Canvas",
    "tipo_proposto": "reference",
    "confidence": "high",
    "tags_sugeridas": ["bmc", "strategy"],
    "rationale": "...",
    "anomalia": null
  }
]
```

### Tag rules applied automatically

- Tags with pejorative substrings (`legacy`, `obsolete`, `outdated`, `pre-pivot`, `post-pivot`, `old`) are removed and replaced by the canonical `phase-archived`.
- Anomaly strings containing `"pre-pivot"`, `"b2b2c"`, `"post-pivot"`, `"stale-content"`, etc. mark the entry with `phase-archived` but do NOT become real anomalies.
- Real anomalies (shown in the review section) require: `"borderline"`, `"duplicate"`, or `"outdated-name"` substrings.

### `km classify` (stub)

`confluence-docs km classify --page-id ID` is registered but returns `"not implemented"`. Reserved for future auto-classification via the Confluence REST API.

## New features (v0.4+)

### Full-width pages

`page create` and `page upload` now accept `--full-width` (and `--fixed-width` to revert):

```bash
# Create a full-width page
confluence-docs page create \
  --space-id <SPACE_ID> --parent-id <HOME_ID> \
  --title "Financial Dashboard" \
  --markdown content.md \
  --full-width

# Revert an existing page to fixed-width
confluence-docs page upload --page-id <id> --markdown content.md --fixed-width
```

Under the hood this posts two properties (`content-appearance-draft` and `content-appearance-published`) to the page properties API after the create/update.

### Page Properties macro (:::properties block)

In any markdown file, use the `:::properties` fenced block to generate the Confluence Page Properties macro:

```markdown
:::properties
type: reference
status: active
owner: user@example.com
tags: psp, billing, recurring
related: [[Stripe Brazil Analysis]], [[id:12345]]
created: 2026-01-01
updated: 2026-01-01
:::
```

Links in values use `[[Page Title]]` or `[[id:N]]` syntax, which the converter turns into `<ac:link><ri:page ri:content-title="..."/></ac:link>` storage XML.

The block is rendered as a `codeBlock` with language `confluence-storage` in ADF (so the storage XML passes through the pipeline). When creating pages via `page create --markdown`, Confluence accepts the storage XML inside the ADF body.

### Smart Link embeds

In markdown, certain standalone URL patterns are automatically converted to Confluence Smart Link nodes:

| Markdown | ADF node | Renders as |
|---|---|---|
| `![embed](https://youtube.com/...)` | `embedCard` (wide layout) | Embedded player/preview |
| `https://linear.app/...` on its own line | `blockCard` | Preview card |
| `https://github.com/...` on its own line | `blockCard` | Preview card |
| `[text](url)` in the middle of a paragraph | normal link | Inline hyperlink |

Bare URL lines and `![embed](url)` trigger smart link conversion. Named links like `[Click here](url)` in prose always remain regular links.

### `confluence-docs check` — duplicate detection

```bash
confluence-docs check --title "Stripe Brazil Analysis"
confluence-docs check --title "Stripe Brazil Analysis" --type reference --tags psp,competitor
confluence-docs check --title "..." --threshold 0.8   # stricter matching
confluence-docs check --title "..." --text            # plain-text output
```

JSON output:
```json
{
  "exists": false,
  "similar": [
    {"id": "456", "title": "Stripe Analysis (EN)", "url": "https://...", "similarity_score": 0.78}
  ],
  "suggestion": "create"
}
```

- `suggestion: "update_existing"` — a near-duplicate was found; consider updating it instead.
- `suggestion: "create"` — no close match; safe to create.

### `confluence-docs new <type>` — template generator

Generates a markdown template with a `:::properties` block and type-specific headings. Output goes to stdout or `--output FILE`.

```bash
# Reference doc
confluence-docs new reference \
  --title "PaymentCo — PSP reference" \
  --output /tmp/page.md

# Decision record (supersedes another page)
confluence-docs new decision \
  --title "PSP Wave 1: PaymentCo" \
  --supersedes 98765

# How-to guide
confluence-docs new how-to --title "How to deploy to production"

# Quick capture
confluence-docs new capture --title "Spike PaymentCo webhooks 2026-05-12"
```

Owner is read from `git config user.email`. Template includes `status: draft`, today's date for `created`/`updated`, and structured headings appropriate to the doc type.

For `decision` type, the template also includes: Alternatives Considered (table), Consequences, and Review date.

### Backward compatibility

- Storage XML (`:::warning`, `:::expand`, `[TOC]`) continues to work exactly as before.
- ADF output via `adf` command is unchanged — smart links and properties are new addition.
- `--full-width` and `--fixed-width` are additive flags; omitting them preserves the default Confluence behavior (fixed-width).
- Full URLs always point to your configured Confluence instance.
- Concise — the team may include non-technical people.
- **Confirm exact title and location** (parent + category) before creating any page.
- Listings as bullets: `- **Title** — summary (URL)`.
- If the search is empty, suggest 2-3 variations before giving up.

## Language

**Respond to the user in the same language they write in.** Match their tone (formal/informal). If the user writes in English, respond in English; if they write in another language, respond in that language.

## Configuration

Since v0.10.0 the CLI uses two separate config files:

- **Credentials** (`~/.config/confluence-docs/credentials`, perms `0600`): `email` + `token` only. Never read raw — use `setup --check` to validate.
- **Config** (`~/.config/confluence-docs/config`, perms `0644`): `cloud`, `active_space_id`, `active_space_key`, `active_space_name`, `active_home_page_id`.

All values are set automatically during `confluence-docs setup` (which auto-detects accessible spaces and asks the user to pick one). **Agents must not read or write these files directly.** Use the CLI commands:

```
confluence-docs setup --check          # validate everything is configured
confluence-docs space current          # show active space
confluence-docs space list             # list all accessible spaces
confluence-docs space use <key>        # switch active space
confluence-docs setup --set cloud acmecorp   # change a single config key
```

The active space provides defaults for all commands that need `--space-id` or space key (CQL search, `index`, `home`, `page create`, `check`). Commands that previously required explicit `--space-id 131352` flags now use the configured space automatically.

## Space management

```bash
# List all spaces (cached 1h; shows active space with ✓)
confluence-docs space list

# Switch active space
confluence-docs space use eng

# Force cache refresh
confluence-docs space list --refresh

# JSON output
confluence-docs space list --json
confluence-docs space current --json
```

After `space use <key>`, all subsequent commands use the new space. The switch is persistent (written to `~/.config/confluence-docs/config`).

## Home cache lifecycle

The local cache at `~/.cache/confluence-docs/home.json` is shared across all Claude sessions on the same machine. The CLI maintains it for you — see the "Cache lifecycle" table in the bootstrap section above for the full contract.

Quick reference:

- **Reads** (`home --query/--show/--digest`): auto-refresh when stale (>1h) or missing.
- **Writes** to the Home (`page apply` / `index *` on the Home pageId): auto-refresh the cache after the PUT.
- **Explicit `home --refresh`**: always fetches, regardless of cache age. Use it when you know another machine just updated and you don't want to wait for the TTL.
- **Override TTL**: `--max-age 30m` (more aggressive) or `--max-age 6h` (more relaxed) on any read command.

This invariant — **read-only cache, fresh fetch before every write** — is the core safety property. Don't bypass it (e.g. don't try to PUT the cached ADF directly).

## CLI installation check

Before running any `confluence-docs` command, verify the binary exists and credentials are valid. The bootstrap flow is:

```
confluence-docs --version          # binary present?
confluence-docs setup --check      # exit 0 = creds valid
```

Exit codes for `setup --check`:
- `0` — credentials valid AND active space configured → proceed
- `1` — no credentials OR no active space configured → run `confluence-docs setup` interactively (or guide the user through Step 5 of `confluence-docs/cli/README.md`)
- `2` — credentials invalid (token revoked or mistyped) → ask the user to regenerate the token at `https://id.atlassian.com/manage-profile/security/api-tokens` and re-run setup
- `3` — network error → retry once; if it persists, surface the error to the user and fall back to MCP

If the binary is absent entirely, fall back to MCP for the current request and tell the user how to install: `confluence-docs/cli/README.md` has the one-shot install URL.

## Updating the skill

When the user asks to update, check, or upgrade the skill (any of: "update the skill", "update confluence-docs", "is there a new version?", "check for updates", "am I on the latest version?"), run:

```
confluence-docs update            # download + install latest release
confluence-docs update --check    # only report whether an update is available
```

**Behavior:**
- Resolves the latest release tag from GitHub.
- Compares with the currently-installed binary.
- If `--check`: reports `current → latest` and exits (0 = up to date, 10 = update available).
- Without `--check`: shells out to the public installer (install.sh on Linux/macOS, install.ps1 on Windows). The installer overwrites the binary, SKILL.md, and reference files atomically. **Credentials and the home cache are preserved across the update** — no re-setup needed.

**When to suggest an update proactively:** if the user reports a CLI behavior that you know was changed in a more recent release (e.g. they say "that command doesn't exist" for a flag you know exists), check `confluence-docs --version` and `confluence-docs update --check` before assuming a real bug.
