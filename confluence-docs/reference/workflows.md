# Workflows — Generic Operations on a Confluence Knowledge Base

This file defines deterministic flows for the universal actions the agent performs against a Confluence space — search, read, create, update, delete, list. Project-specific routing (which parent page does a given new entity belong under?) lives in **your project's Home page**, not here. See `bootstrap.md` for how the agent learns that.

**Conventions:**

- **Tool priority:** prefer the Go CLI (`confluence-docs ...`) over the Atlassian MCP. The CLI returns sub-KB summaries; MCP returns full ADF bodies (10–40 KB). See SKILL.md §"Tool preferences" for the canonical table.
- **`cloudId`, `spaceId`, `parentId` come from the credentials file + the project's Home page** — never hardcoded here.
- **PageIds of parent pages** are resolved from your Home (via `home --query` or `home --show`) before any action that requires one.

## Contents

- [Workflow 0 — Bootstrap (always run at session start)](#workflow-0--bootstrap-always-run-at-session-start)
- [Workflow 1 — Search](#workflow-1--search)
- [Workflow 2 — Read a page](#workflow-2--read-a-page)
- [Workflow 3 — Create a new page](#workflow-3--create-a-new-page)
- [Workflow 4 — Update an existing page](#workflow-4--update-an-existing-page)
- [Workflow 5 — Status](#workflow-5--status)
- [Workflow 6 — Delete (soft) a page](#workflow-6--delete-soft-a-page)
- [Workflow 7 — Regenerate the Knowledge Map](#workflow-7--regenerate-the-knowledge-map)
- [Cross-cutting execution rules](#cross-cutting-execution-rules)

---

## Workflow 0 — Bootstrap (always run at session start)

**Trigger:** first interaction of the session involving the knowledge base.

**Why it exists:** the skill is timeless — it knows generic structure/rules but **not the current state** (categories your project uses today, current pageIds of parents, names of people/companies in each category). That state lives on the Confluence Home and must be read fresh.

**Steps:**

1. **Query the cache directly — it auto-refreshes when stale:**
   ```
   confluence-docs home --query "<term>"   # alias / decision-map lookup
   confluence-docs home --digest           # outline + heading list
   confluence-docs home --show             # full text rendering
   ```
   These commands handle freshness internally: if the cache file is missing or older than 1h, they auto-fetch and serve.

2. **Extract whatever your project documents on its Home.** Typical patterns:
   - A "where do I put X?" decision table mapping content type → parent page
   - Aliases (natural-language keywords → category)
   - A Page ID Index of structural parents
   - Category descriptions

3. **Use this as source of truth** for the rest of the conversation.

4. **Writes auto-refresh the cache.** When you call `page apply` / `index add` etc. on the Home, the cache is automatically re-fetched after the PUT succeeds.

5. **`home --refresh`** exists only for the rare "another machine just updated; I don't want to wait for the 1h TTL" case.

6. **Fallback:** if the CLI is unavailable, use MCP `getConfluencePage(pageId=<home>, contentFormat="markdown")` and operate from in-memory data.

**Cache safety invariant:** the cache is **read-only for navigation**. Writes ALWAYS go through `page apply`, which GETs fresh ADF before any PUT.

---

## Workflow 1 — Search for a page by theme

**Trigger:** user asks "where is X?", "is there a page about Y?", "show me what we have on Z".

**Steps:**

1. **Bootstrap** if not done.
2. **Check the Home's aliases / decision map** (`home --query "TERM"`). If the term is mapped, return the indicated page or parent directly.
3. **If not mapped, run the CLI search:**
   ```
   confluence-docs search "TERM" --limit 10
   ```
   This expands to `space = "<your space>" AND type = "page" AND (title ~ "TERM" OR text ~ "TERM")`. Output is TSV (`pageId<TAB>title<TAB>url<TAB>excerpt`).
4. **Try variants** (with/without accents, singular/plural) before declaring zero results.
5. **Return** for each hit: title, full URL, short excerpt, pageId.
6. **If zero results:** suggest creating a new page, citing the most likely parent based on the Home's decision map.

---

## Workflow 2 — Read a page (3-step cascade)

Most "what's in page X?" questions can be answered with cheap reads. Always escalate only when needed.

1. **`page digest`** — outline of headings + macros + first-paragraph extract. ~500 bytes. Answers most "qual o status / o que tem nessa página" questions.
2. **`page get --section "Y" --format text`** — one section as plain text. Use when the digest pinpointed where the answer lives.
3. **`page get --format text`** — whole page. Use only when you need cross-section context.
4. **`page get --format adf`** / **`page get --format export_view`** — last resort. Used when editing programmatically or when the rendered HTML is needed.

Never fetch the full ADF body just to read a single section.

---

## Workflow 3 — Create a new page

**Trigger:** "create a page for X", "register Y", "document Z".

**Steps:**

1. **Bootstrap.**
2. **Run `confluence-docs check --title "..."` first** to surface near-duplicates (Jaccard trigram similarity ≥ 0.4 default). If a similar page exists, prompt the user: "Update existing or create new?"
3. **Decide the doc type** (`reference` / `decision` / `explanation` / `how-to` / `capture`) — see `doc-types.md`.
4. **Decide the parent page** — resolved from the Home's decision map / aliases.
5. **Generate a template:**
   ```
   confluence-docs new <type> --title "..." [--parent-id <ID>] [--full-width]
   ```
   The output is markdown with the required frontmatter (`:::properties`), TL;DR placeholder, and type-specific structure already in place.
6. **Fill in the content.** Apply the standard rules: TL;DR ≤ 5 bullets if > 300 words, descriptive headers (`## Context: <qualifier>`, not bare `## Context`), parágrafos self-sufficient.
7. **Create the page:**
   ```
   confluence-docs page create --space-id <ID> --parent-id <PID> --title "..." --markdown FILE
   ```
8. **If this is a `:::properties` page**, the storage path is auto-detected; tags from the frontmatter are also applied as real Confluence labels on the created page. Owner / reviewer `@mentions` are resolved to user mention chips.
9. **Register in the KNOWLEDGE_MAP** (if your project uses one) in the same turn, under the section matching the type. Otherwise the page becomes orphaned.
10. **If the parent has a summary table**, update it via `page apply --table-add-row "Heading" --row "col1|col2|..."` so the new entry shows up there too.

---

## Workflow 4 — Update a page section

**Trigger:** "update the X section of page Y", "add a row to the table on Z", "rewrite the recommendations on W".

**Steps:**

1. **Bootstrap.**
2. **Identify the page** (via search, alias, or pageId from context).
3. **Read the digest** to confirm the section name exists.
4. **Apply atomically:**
   ```
   confluence-docs page apply --page-id <ID> --replace-section "Heading" --fragment FILE
   ```
   - For tables: `--table-add-row "Heading" --row "col1|col2|..."` (idempotent with `--if-missing`).
   - For appends: `--append --fragment FILE`.
   - For inserts: `--insert-after "Heading" --fragment FILE` or `--insert-before "Heading" --fragment FILE`.
   - For deletes: `--delete-section "Heading"`.

   The CLI does GET → edit → PUT in one shot, with 409-conflict retry. Macros are preserved everywhere outside the targeted section.

5. **If the operation needs to span multiple sections atomically**, use the two-step fallback:
   ```
   confluence-docs page get --page-id <ID> --format adf --output /tmp/p.json
   confluence-docs edit -i /tmp/p.json [ops...] > /tmp/new.json
   confluence-docs page upload --page-id <ID> --adf /tmp/new.json --message "..."
   ```

6. **Never use `contentFormat="markdown"` on a page with macros** (TOC, expand, panel, status, page-properties). It silently flattens the structure on update. The CLI uses ADF round-trips internally and never has this risk.

---

## Workflow 5 — Move, rename, or reorder a page

**Trigger:** "rename page X", "move page Y under parent Z", "make this a sibling of W".

```
confluence-docs page move --page-id <ID> --title "New Title"           # rename only
confluence-docs page move --page-id <ID> --parent-id <NEW>             # move only
confluence-docs page move --page-id <ID> --parent-id <NEW> --title T   # both
confluence-docs page reorder --page-id <ID> --before <SIBLING>         # reorder
confluence-docs page reorder --page-id <ID> --after  <SIBLING>
confluence-docs page reorder --page-id <ID> --append-to <PARENT>
```

Body is preserved byte-for-byte; macros stay intact.

---

## Workflow 6 — Delete (soft) a page

**Trigger:** "delete page X", "trash this".

```
confluence-docs page delete --page-id <ID> --yes
```

Soft delete — Confluence trash is restorable. **Always confirm with the user** before issuing.

---

## Workflow 7 — Regenerate the Knowledge Map

If your project uses `km` to maintain a KNOWLEDGE_MAP page (recommended), regenerate after major edits:

```
confluence-docs km generate \
    --input /path/to/triage/json/dir \
    [--baseline baseline.json] \
    --target-page-id <KM_PAGE_ID> \
    --full-width \
    --message "..."
```

See `doc-types.md` for the canonical 5 types and `cmd_km.go` source for the JSON formats.

---

## Cross-cutting execution rules

- **Use the cache for navigation, not API calls.** `home --query "X"` is local and free (auto-refreshes when needed); reach for `page digest` only when the term isn't in the Home and you need to inspect a specific page.
- **Prefer `digest` to full reads.** Most "what's the status of X?" questions are answered by the headings outline + first-paragraph extract alone.
- **Prefer `page apply` for updates.** Atomic, macro-preserving, with 409-conflict retry. Only fall back to the two-step `get + edit + upload` flow if you need multiple ops in one PUT.
- **Always include a TL;DR (≤ 5 bullets)** on new pages > 300 words.
- **Descriptive headers**, not bare ones. `## Context: why competitor X matters` beats `## Context`.
- **For tables that summarize sub-pages**, prefer `page apply --table-add-row` for atomic, idempotent updates. Don't `get → modify → upload` a whole page just to add one row.
- **Confirm before deleting.** Even soft deletes (Confluence trash is restorable).
- **When in doubt, ask once and proceed.** Don't stack questions — most teams prefer action with explicit assumptions over a long interrogation.
- **Use placeholders** (e.g. `[fill in]`) when the user doesn't know a field — don't invent data.
