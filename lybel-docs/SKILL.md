---
name: lybel-docs
description: |
  Navigation assistant for Lybel's Confluence knowledge base (space `lybel` at lybel.atlassian.net) — search, create, list, update pages. Aliases: confluence, wiki, kb, base de conhecimento, página/doc da Lybel, anota isso, registra isso, salva no doc, joga no wiki. Use for any Lybel documentation: processes, partners, advisors, investors, accelerators, retailers, fornecedores, roadmap, strategy, marca, design system, governance, or organizational artifact — even when "Confluence" isn't said. Triggers (pt-BR): "onde fica X", "me dá a página de Y", "cria página pra Z", "lista X", "qual o status de Y", "tem doc sobre Z", "adiciona isso", "documenta esse processo", "atualiza a página de Q", "adiciona advisor/parceiro/investidor", "anota no kb". Stores no specific data — fresh state lives in Confluence, fetched at session start via local CLI cache (preferred) or Atlassian MCP (fallback). Always replies in pt-BR with full URLs to lybel.atlassian.net.
allowed-tools: |
  Bash(lybel-docs *)
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

# Lybel Docs — Confluence Knowledge Base Assistant

## Overview

Skill that connects Claude to Lybel's Confluence (`lybel.atlassian.net`, space key `lybel`) to search, create, list and update documentation in natural language — in Portuguese, without manually opening Confluence.

The skill is **deliberately timeless**: it stores no names, lists or state (advisors, investors, partners, accelerators, page IDs). All of that lives in Confluence and is read fresh in every session starting from the Home. The `reference/` files here are generic fallback, not the source of truth.

## Language rule

**This document is in English for Claude's performance (Claude is trained primarily on English data, and English instructions yield more robust reasoning). However, all user-facing output MUST be in Brazilian Portuguese (pt-BR).**

When you respond to the user:
- Use Brazilian Portuguese
- Match the user's tone (formal/informal as they write)
- Keep page titles, category names, and content IN PORTUGUESE (they exist in Portuguese in Confluence)
- Only technical terms or proper nouns stay in English

## Tool priority — CLI first, MCP as fallback

The `lybel-docs` Go CLI talks directly to the Confluence REST API and is **always the preferred tool** for reads, writes and searches. The Atlassian MCP returns the full ADF body of every page (tens of KB), which inflates the conversation context fast. The CLI returns a small digest, a TSV row, or a one-line status — orders of magnitude cheaper.

**Order of preference for any operation:**
1. **`lybel-docs ...`** if the binary exists (check with `lybel-docs --version`).
2. **MCP Atlassian** only when the CLI cannot do the job (rare cases: complex space exploration, attachments, comments, Jira).
3. **`contentFormat: "markdown"` via MCP** — last resort, ONLY for pages with no macros (it flattens TOC/expand/panel and silently destroys structure).

The CLI also handles credentials, CQL search, ADF section editing with macro preservation, and a 409-conflict retry on writes — all without round-tripping bytes through the conversation.

## Mandatory bootstrap

In **EVERY new session**, just use the Home commands directly. The CLI handles freshness — you don't track TTLs, sessions, or cache state.

1. **Read the cache for navigation — auto-refreshes when stale:**
   ```
   lybel-docs home --query "advisor"           # alias/decision lookup
   lybel-docs home --query "Aceleração"        # find category + pageId
   lybel-docs home --digest                    # outline view (~500 bytes)
   lybel-docs home --show                      # full text rendering
   ```
   These commands auto-refresh the cache when it's missing or older than 1h. You don't need to run `home --refresh` first. Most navigation answers are in the cache, returning a few hundred bytes per query.

2. **Use the cached Home content as source of truth** for:
   - Current taxonomy (categories and sub-structures)
   - "Where do I put X?" decision map
   - Aliases (keywords → pages)
   - Page ID Index (if present)
   - Organization rules

3. **Fall back to the generic reference files** (`reference/taxonomy.md`, `reference/aliases.md`, etc.) **only if both the CLI and MCP are inaccessible**.

### Cache lifecycle (the contract)

The cache lives at `~/.cache/lybel-docs/home.json` — **a single file shared across all Claude sessions on the same machine**. So if one session refreshes (manually or via auto-refresh-on-write), every other session reading it next sees the updated state automatically. No per-session bookkeeping.

Three rules govern when the cache is updated:

| Trigger | Behavior |
|---|---|
| Read with stale cache (>1h old) or missing | **Auto-refresh** before serving. Caller doesn't have to think about it. |
| Write to the Home via CLI (`page apply`, `index add/remove/sync` on pageId 164232) | **Auto-refresh after PUT** succeeds. Your session sees the new state immediately. |
| Explicit `home --refresh` | **Always fetches**, ignores TTL. Use only when you know another machine just edited the Home and you don't want to wait for the TTL. |

What this means in practice: in a typical session, you never call `home --refresh` explicitly. You just query/show/digest, and writes refresh themselves.

**WRITE SAFETY (critical):** the cache is **read-only for navigation**. It is **NEVER** the source for an update. Any mutation of the Home (or any page) goes through `page apply`, which always GETs fresh ADF before PUT — ensuring you never overwrite changes someone made on another machine.

This skill is deliberately **timeless**. It stores no specific names (advisors, investors, retailers, accelerators) — everything comes fresh from Confluence whenever you query, with the cache layer making it cheap.

**Token-cost rule of thumb:** prefer cached `home --query/--show` over `page digest` over `page get` over MCP. Only escalate to a full read when the cheaper option doesn't carry the answer.

## Reference files

- `reference/bootstrap.md` — principle + detailed bootstrap procedure
- `reference/taxonomy.md` — generic structure of the space (fallback)
- `reference/aliases.md` — generic alias patterns (fallback)
- `reference/templates.md` — formats by page type (partner sheet, meeting notes, ADR, etc.)
- `reference/workflows.md` — standard steps (search, create, update, status)

## Default workflows

### 1. Search — "onde fica X" / "tem doc sobre Y?"

**Source-of-truth ladder for resolving a term to a pageId.** Try in order; stop at the first one that returns a plausible match. The next `page digest` call functions as the verification step — if the title or content doesn't match expectations, fall back to the next rung.

1. **Memory file or recent conversation context.** If a Claude memory file (e.g. `project_X_relationship.md`) or the current conversation already has a pageId for the term, **use it directly**. Skip steps 2–3. The follow-up `page digest` validates it: a 404, a renamed title, or unrelated content tells you the memory is stale, and you fall back to step 2.

2. **Cached Home (`home --query`).** Local, free, fastest after memory:
   ```
   lybel-docs home --query "<term>"
   ```
   Hits the Page ID Index, aliases, and "Onde coloco X?" decision map. Returns matching lines grouped by section.

   **Single term per call** — no OR / regex / multi-term syntax. To search multiple terms, run the command multiple times (each call is local and cheap, zero API cost). Same applies to `--show` + `grep` if you need a richer pattern.

3. **CLI search (`search`).** When the term isn't in the Home (new pages, niche topics):
   ```
   lybel-docs search "<term>" --limit 5
   ```
   Output is TSV: `pageId<TAB>title<TAB>url<TAB>excerpt`. ~150 bytes per result. The default CQL is `space="lybel" AND type="page" AND (title ~ "term" OR text ~ "term")`. Pass `--cql "raw CQL"` for fine control.

4. **Confirm and return.** Once you have a pageId, call `page digest --page-id <id>` to verify the page exists and matches the user's intent before quoting it back. If returning multiple results from `search`, format as bullets: `- **Title** — excerpt (URL)`.

**Fallback** (CLI unavailable): `mcp__atlassian__searchConfluenceUsingCql` with the same CQL filter.

**Why the memory-first rung exists:** memory files capture stable IDs that survive across sessions. Going straight to `page digest` skips two steps with no downside — the digest is cheap and self-validating. Don't over-think it: if you have an ID and a name that matches what the user asked, just check the digest.

### 2. Read a page — "me dá a página de Y" / "o que tem em Z"

**Three-step cascade**, escalate only when needed:

**Step 1 — `digest` (cheapest, ~500 bytes).** Most questions can be answered from the outline alone:

```
lybel-docs page digest --page-id <id>
```

The digest also carries a `Status` line when the title starts with a Lybel status emoji (🟢 active, 🟡 in-progress, 🟠 evaluating, 🔴 blocked, 🔵 researched, ⚪ idle, ✅ done). For "qual o status de X" questions, this single field is often the entire answer. Add `--json` to get a structured object.

**Step 2 — `get --section "Heading" --format text` (one section, ~hundreds of bytes).** When the digest tells you which section has the answer, fetch just that section as readable plain text:

```
lybel-docs page get --page-id <id> --section "📌 Veredito atual" --format text
```

Section bounds = heading + all following nodes until the next heading of equal-or-higher level (so an h2 includes its h3 children). Use `--at-level N` to disambiguate when the same heading text appears at multiple levels. Output is markdown-ish (`## headings`, `- bullets`, pipe-tables, `[text](url)` links) — readable by a human and trivially parseable by an agent.

**Step 3 — `get` whole page (last resort, multi-KB).** Only when you genuinely need the full content (e.g. you'll be editing several sections):

```
lybel-docs page get --page-id <id> --format text         # whole page as text
lybel-docs page get --page-id <id> --format adf          # raw ADF (only if editing)
lybel-docs page get --page-id <id> --format export_view  # rendered HTML
```

**`--output FILE` and `--quiet`** are available on all `page get` invocations: `--output` writes to disk instead of stdout; `--quiet` suppresses the "wrote N bytes" stderr message. Use `--quiet` when the caller captures both streams.

### 3. Create — "cria página pra Z"

1. Use the Home's "Where do I put X?" map to discover the correct category/parent.
2. Choose the template in `reference/templates.md`.
3. **Confirm with the user** the final title, parent and template before creating.
4. Write the content as markdown to a temp file (e.g. `/tmp/lybel-edit/page.md`). The CLI's `adf` converter supports Confluence macros (`[TOC]`, `:::expand`, `:::warning`, etc.) via extended markdown.
5. Create directly via the CLI (single command, no MCP round-trip):
   ```
   lybel-docs page create \
     --space-id 131352 --parent-id <parentId> --title "Final Title" \
     --markdown /tmp/lybel-edit/page.md
   ```
6. The CLI prints `{"pageId": "...", "title": "...", "url": "..."}`. Return the final URL to the user.

**Fallback** (CLI unavailable): `mcp__atlassian__createConfluencePage` with `contentFormat: "adf"` after running `lybel-docs adf` to convert the markdown. Last resort: `contentFormat: "markdown"`.

### 4. Update — "atualiza a página de X" / "adiciona seção Y"

**Never build ADF by hand, and never use `contentFormat: "markdown"` to update a page with macros** (TOC, Expand, panel). Markdown update flattens macros and silently destroys structure.

**Preferred path (single atomic command):** `lybel-docs page apply` does GET → section-edit → PUT in one shot, with automatic refetch-and-retry on 409 conflict (someone else updated mid-flight). The full ADF never enters the conversation context.

```
# Replace a single section, preserving every macro outside it
lybel-docs page apply --page-id <id> \
  --replace-section "Roadmap" --fragment /tmp/lybel-edit/new.md \
  --message "rewrite roadmap"

# Append a new section at the end
lybel-docs page apply --page-id <id> \
  --append --fragment /tmp/lybel-edit/new.md \
  --message "add Q3 retrospective"

# Insert relative to an existing heading
lybel-docs page apply --page-id <id> \
  --insert-after "Research" --fragment /tmp/lybel-edit/new.md

lybel-docs page apply --page-id <id> \
  --insert-before "FAQ" --fragment /tmp/lybel-edit/new.md

# Delete a stale section
lybel-docs page apply --page-id <id> --delete-section "TODO antigo"

# Disambiguate when the same heading text appears at multiple levels
lybel-docs page apply --page-id <id> \
  --replace-section "Ops" --at-level 3 --fragment /tmp/lybel-edit/new.md

# Add a row to a table inside a section (idempotent with --if-missing)
lybel-docs page apply --page-id <id> \
  --table-add-row "Status atual" --row "Acme Corp|🟡 Em avaliação|Origem X|nota" \
  --if-missing --message "add Acme to status table"

# Cells with literal pipes: escape with backslash
lybel-docs page apply --page-id <id> \
  --table-add-row "Endpoints" --row "GET /api/v1\|v2|public|200ms"
# → cells: ["GET /api/v1|v2", "public", "200ms"]

# Remove a row by matching cell text
lybel-docs page apply --page-id <id> \
  --table-remove-row "Status atual" --match-cell "Acme Corp"

# Preview without writing
lybel-docs page apply --page-id <id> \
  --replace-section "Roadmap" --fragment frag.md --dry-run
```

For section replacement, include the heading line in the fragment markdown. Section bounds = heading + all following top-level nodes until the next heading of equal-or-higher level (h2 closes at h2 or h1; h3 closes at h3, h2 or h1).

**If `apply` reports "section not found"**, the command also lists the page's current top-level headings so you can correct the spelling or pick a different section. **Never blindly retry** — confirm with the user when the page structure differs from what was expected.

**Two-step fallback** (when `apply` is unavailable but `edit` is):
1. `lybel-docs page get --page-id <id> --format adf --output /tmp/current.json`
2. `lybel-docs edit -i /tmp/current.json --replace-section "..." /tmp/frag.md > /tmp/new.json`
3. `lybel-docs page upload --page-id <id> --adf /tmp/new.json --message "..."`

**MCP fallback** (when no CLI at all):
1. `getConfluencePage(contentFormat="adf")` → save body to disk
2. Convert + edit ADF manually (don't do this — install the CLI instead)
3. `updateConfluencePage(contentFormat="adf", body=...)`

If the page has macros and no CLI is installed, warn the user that updating via `markdown` may destroy structure and ask them to install the CLI first.

### 5. List — "quais aceleradoras temos" / "lista parceiros"

1. Identify the category via the Home.
2. Use the CLI to list direct children:
   ```
   lybel-docs page children --page-id <categoryParentId>
   ```
   Output: `pageId<TAB>title` per line.
3. Return as bullets ordered by title or status.

**Fallback**: `mcp__atlassian__getPagesInConfluenceSpace` or `getConfluencePageDescendants`.

### 6. Status — "qual o status de X"

For most "qual o status" questions, the digest is enough — it shows the page's headings, last version number, and which macros are present (status macros show up). For richer status (labels, properties), see Workflow 5 in `reference/workflows.md`. Always cite the date of the last update.

### 7. Add relationship — "adiciona advisor/parceiro/investidor X"

1. Verify in the Home which department/category is correct (advisor ≠ investor ≠ commercial partner).
2. Confirm template (Advisor Sheet, Investor Sheet, Partner Sheet).
3. Create under the correct parent (Workflow 3). Always confirm location before.

### 8. Reorganize — "renomeia X" / "move X pra dentro de Y" / "deleta X"

The `page move` and `page delete` verbs handle structural changes without touching the page body. Use them when the user asks to rename, reparent, or trash a page. **Always confirm the target with the user before deleting** — even soft delete (Confluence trash is restorable, but the user should still authorize).

```
# Rename only (parent unchanged)
lybel-docs page move --page-id <id> --title "New Title" --message "rename: reason"

# Reparent only (title unchanged)
lybel-docs page move --page-id <id> --parent-id <newParentId> --message "move under X"

# Both at once (single PUT)
lybel-docs page move --page-id <id> --parent-id <newParentId> --title "New Title"

# Preview without writing
lybel-docs page move --page-id <id> --title "..." --dry-run

# Soft-delete (restorable from Confluence trash)
lybel-docs page delete --page-id <id> --yes
```

**How move works under the hood:** v2 PUT requires the body, so `page move` GETs the current ADF and re-PUTs it with the new `parentId` / `title`. Body is preserved byte-for-byte; macros stay intact. The full ADF never enters the conversation context.

**When NOT to use:**
- To change the **content** of a page → use `page apply` (section/table edit) or `page rewrite` / `page upload` (full body replacement).
- To change the body **and** rename in one shot → use `page upload --title "..."` (you already have a new ADF) instead of two separate calls.

**Bulk reorganizations:** when reparenting multiple pages or doing a multi-step restructure (rename + move + delete cascade), prefer running each `page move` / `page delete` as its own atomic command. The CLI returns a one-line JSON status per call (~50 bytes), so even 10–20 calls stay cheap. Confirm the new tree with `page children` afterwards.

## Editorial patterns for Lybel pages

Every **decision**, **proposal**, **strategy**, or **spec** page must follow two conventions. Index pages, reference pages and glossaries can skip them.

### Pattern 1 — Page header

Start every page with a small header block (quote/callout) containing:

> **Contexto:** (one-line summary of what the page is about)
> **Criado em:** YYYY-MM-DD | **Atualizado em:** YYYY-MM-DD | **Criado por:** [nome]

When updating a page, bump the "Atualizado em" date. On first creation, author comes from the current user (confirm if unknown).

### Pattern 2 — Contexto → Problema → Solução structure

For decision/proposal/strategy pages, organize the body around three sections:

1. **Contexto** — where the page comes from, which project/moment it serves, what bigger scope it sits in
2. **Problema** — what is being solved, what hurts, what constraints exist
3. **Solução (possível)** — proposed approach, options, tradeoffs, concrete scope

See `reference/templates.md` → "Decision / Proposal / Strategy" template for the canonical form.

**Why this matters**: the knowledge base becomes predictable — any reader (human or AI) knows where to find the motivation, the pain, and the plan without re-reading everything. Reduces onboarding cost and review time.

**When creating a page**: apply these patterns by default; deviate only if the page type is clearly an index/reference/glossary.

**When updating an existing page** that violates the patterns: offer the user a refactor alongside the content change — don't silently rewrite structure.

### Pattern 3 — Clarity for outside readers

Pages are read by people who weren't in the conversation that generated them. Before saving:

- **Technical jargon** (Merchant of Record, Variante A, MCP, UCP, commission fee, postback, etc.) must be explained briefly on first use OR linked to a page that explains. Don't assume context.
- **Internal labels** (Variante A, Onda 2, v0.1) should come with a short gloss when first introduced on a page.
- **References to other work** (research, analysis, previous decisions) must be linked to the actual Confluence page — never just mention "analisamos X" without linking X.
- **Balance**: don't over-explain the obvious (readers are smart, just not context-equipped). One short clause or parenthetical is usually enough. Prefer linking over inlining when the full explanation exists elsewhere.

**Smell test**: if a new hire at the Lybel opens this page cold, will they understand it? If not, add one link or one half-sentence.

## Tool preferences

Ordered by preference. Always try the cheapest tool that can answer the question — escalate only when necessary.

| Goal | First choice | Second choice | Last resort |
|---|---|---|---|
| Bootstrap (start of session) | `home --refresh` (one GET) | `page digest --page-id 164232` | MCP `getConfluencePage` |
| "Where do I put X?" / aliases | `home --query "X"` (cached, free) | `home --show` + read | MCP `getConfluencePage` |
| "What's in page X?" / outline | `page digest` (~500 bytes) | `page get --section "Y" --format text` | `page get --format text` |
| "Qual o status de X?" | `page digest` (Status field, 0 extra calls) | `page get --section "Status" --format text` | — |
| "O que diz a seção Y de X?" | `page get --page-id X --section "Y" --format text` | `page get --format text` (whole page) | MCP `getConfluencePage(markdown)` |
| Find a page | `search "term"` | MCP `searchConfluenceUsingCql` | — |
| List children of a category | `page children` | MCP `getPagesInConfluenceSpace` | — |
| Update a page (single section) | `page apply` | `page get` + `edit` + `page upload` | MCP `getConfluencePage(adf)` + manual + `updateConfluencePage(adf)` |
| Update a page (table row) | `page apply --table-add-row` / `--table-remove-row` | `page get` + `edit --table-*` + `page upload` | — |
| Create a new page | `page create --markdown` | `adf` to convert + MCP `createConfluencePage(adf)` | MCP `createConfluencePage(markdown)` |
| Rename a page | `page move --page-id ID --title "New Title"` | `page get` + `page upload --title "..."` | MCP `updateConfluencePage` |
| Move a page to a new parent | `page move --page-id ID --parent-id NEW_PARENT` | — | MCP `updateConfluencePage` |
| Soft-delete a page | `page delete --page-id ID --yes` | — | MCP `deleteConfluencePage` |
| Build rich ADF from markdown | `adf` (with `[TOC]`, `:::expand`, `:::warning` extensions) | — | — |

**Why CLI first:** the MCP returns the full ADF body of every page (10–40 KB). The CLI returns digests (~500 bytes), single-section slices (~hundreds of bytes), TSV rows (~150 bytes per result), or one-line status payloads. Across a multi-edit session the difference is usually 10–50× in token cost.

**Status convention:** Lybel page titles often start with a status emoji — 🟢 (active), 🟡 (in-progress), 🟠 (evaluating), 🔴 (blocked), 🔵 (researched), ⚪ (idle), ✅ (done). The CLI's `digest` parses these and exposes `Status: <emoji> <label>` in the text output (and `"status": "<label>"` + `"statusEmoji": "<emoji>"` in `--json`). Use this to answer "qual o status de X?" without any further reads.

**Other notes:**
- **CQL**: prefer `title ~` before `text ~`. **Always** filter by `space = "lybel"`.
- **Batch:** multiple reads in parallel within the same tool-call block.
- **Macro preservation**: `page apply` and `edit` only touch the targeted section; every macro elsewhere on the page is preserved byte-for-byte. Never use `contentFormat: "markdown"` on a page with macros.

## Report style

- Reply in **Brazilian Portuguese (pt-BR)**.
- Full URLs always: `https://lybel.atlassian.net/wiki/spaces/lybel/pages/<id>`.
- Concise — the team includes non-technical people.
- **Confirm exact title and location** (parent + category) before creating any page.
- Listings as bullets: `- **Title** — summary (URL)`.
- If the search is empty, suggest 2-3 variations before giving up.

## Language

**Always respond to the user in Brazilian Portuguese (pt-BR)**, regardless of this document being in English. The user (Lybel team, non-technical) expects Portuguese responses.

## Locked configuration

- **cloudId:** `ab1dada3-b25e-40ad-9dbc-682caeea8d00`
- **Space key:** `lybel`
- **Space ID:** `131352` (used by `page create --space-id`)
- **Home page ID:** `164232`
- **Base URL:** `https://lybel.atlassian.net/wiki`
- **Cloud subdomain:** `lybel` (CLI default — no flag needed)

Don't ask the user — pass these values directly to the MCP tools or CLI flags.

## Home cache lifecycle

The local cache at `~/.cache/lybel-docs/home.json` is shared across all Claude sessions on the same machine. The CLI maintains it for you — see the "Cache lifecycle" table in the bootstrap section above for the full contract.

Quick reference:

- **Reads** (`home --query/--show/--digest`): auto-refresh when stale (>1h) or missing.
- **Writes** to the Home (`page apply` / `index *` on pageId 164232): auto-refresh the cache after the PUT.
- **Explicit `home --refresh`**: always fetches, regardless of cache age. Use it when you know another machine just updated and you don't want to wait for the TTL.
- **Override TTL**: `--max-age 30m` (more aggressive) or `--max-age 6h` (more relaxed) on any read command.

This invariant — **read-only cache, fresh fetch before every write** — is the core safety property. Don't bypass it (e.g. don't try to PUT the cached ADF directly).

## CLI installation check

Before running any `lybel-docs` command, verify the binary exists and credentials are valid. The bootstrap flow is:

```
lybel-docs --version          # binary present?
lybel-docs setup --check      # exit 0 = creds valid
```

Exit codes for `setup --check`:
- `0` — credentials valid → proceed
- `1` — no credentials file → run `lybel-docs setup` interactively (or guide the user through Step 5 of `lybel-docs/cli/README.md`)
- `2` — credentials invalid (token revoked or mistyped) → ask the user to regenerate the token at `https://id.atlassian.com/manage-profile/security/api-tokens` and re-run setup
- `3` — network error → retry once; if it persists, surface the error to the user and fall back to MCP

If the binary is absent entirely, fall back to MCP for the current request and tell the user how to install: `lybel-docs/cli/README.md` has the one-shot install URL.

## Updating the skill — "atualiza a skill" / "tem versão nova?"

When the user asks to update, check, or upgrade the skill (any of: "atualiza a skill", "atualiza o lybel-docs", "tem versão nova?", "verifica se tem update", "tá na última versão?"), run:

```
lybel-docs update            # download + install latest release
lybel-docs update --check    # only report whether an update is available
```

**Behavior:**
- Resolves the latest release tag from GitHub.
- Compares with the currently-installed binary.
- If `--check`: reports `current → latest` and exits (0 = up to date, 10 = update available).
- Without `--check`: shells out to the public installer (install.sh on Linux/macOS, install.ps1 on Windows). The installer overwrites the binary, SKILL.md, and reference files atomically. **Credentials and the home cache are preserved across the update** — no re-setup needed.

**For non-technical users:** they don't need to remember any URL. Just running `lybel-docs update` does everything. Reply in pt-BR with the result, e.g.:
- "Já está na última versão (v0.3.3)."
- "Atualizei de v0.3.0 → v0.3.3."

**When to suggest an update proactively:** if the user reports a CLI behavior that you know was changed in a more recent release (e.g. they say "esse comando não existe" for a flag you know exists), check `lybel-docs --version` and `lybel-docs update --check` before assuming a real bug.
