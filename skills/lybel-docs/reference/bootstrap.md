# Bootstrap — How the Skill Orients Itself

## Language reminder

**This document is in English for Claude's performance. All user-facing output MUST be in Brazilian Portuguese (pt-BR).** When you reply to the user, always use pt-BR. Page titles, category names and content remain in Portuguese (they exist in Portuguese in Confluence).

## Principle

This skill **does not store Lybel-specific data**. It knows:

- Generic taxonomic structure (6 categories)
- Templates per content type
- Standard workflows
- Organization and tie-breaking rules

The **current data** (who the advisors are today, which accelerators are in progress, which investor is in conversation, pageIds of each categorical parent, target retailers in the active roadmap) lives **exclusively in Lybel's Confluence**. The skill always fetches current state before acting.

---

## Bootstrap procedure

In every new session involving the Lybel KB, Claude just queries the cache directly. The CLI handles freshness — you don't track sessions, TTLs, or cache state.

1. **Query the cache for navigation:**
   ```
   lybel-docs home --query "advisor"      # alias / decision lookup
   lybel-docs home --digest               # outline view
   lybel-docs home --show                 # full text rendering
   ```
   These commands auto-refresh the cache when it's missing or older than 1h. No need to run `home --refresh` first.

   If the CLI is unavailable, fall back to `mcp__atlassian__getConfluencePage(pageId="164232", contentFormat="markdown")` and operate from in-memory data.

2. **Extract from the Home content:**
   - **"Onde coloco X?" table** — current decision map for routing
   - **"Aliases" section** — keywords → pages (including current proper names: people, companies, vendors)
   - **"Page ID Index" section** (if present) — IDs of structural parents (categories, sub-categories, departments)
   - **Categories with current links** — current state of the 6 categories

3. **Use this data as source of truth** for the rest of the conversation.

4. **Writes to the Home auto-refresh the cache.** When you `page apply` / `index add` / `index remove` / `index sync` on the Home page (pageId 164232), the cache is automatically refreshed after the PUT succeeds. You don't need to call `home --refresh` after writes.

5. **The cache is shared across all Claude sessions on the same machine.** If one session refreshes (manually or via a write), every other session reading next sees the updated state automatically. No per-session bookkeeping needed.

6. **`home --refresh`** exists for the rare case where you know another machine just updated the Home and you don't want to wait for the 1h TTL. It always fetches, ignoring the cache.

**Why this design:** the cache costs at most one GET per hour per machine and zero per query within that hour. The MCP route, in contrast, would re-fetch the Home (10-25 KB ADF) for every navigation question — death by a thousand reads. The cache also enforces a clean invariant: it is **read-only for navigation**, and `page apply` always GETs fresh ADF before any PUT, so we never overwrite changes someone made on another machine.

---

## Fallback

If the Home is inaccessible (auth error, both CLI and MCP offline, page deleted), use the static files:

- `taxonomy.md` — generic structure of the 6 categories
- `aliases.md` — generic keyword patterns
- `templates.md` — page formats per type
- `workflows.md` — steps per action

This always works, but **with less precision** (no current state: pageIds may be wrong, proper names unknown, statuses outdated). Warn the user when operating in degraded mode.

---

## Why this design

- **Timeless skill:** never goes stale. If an investor leaves the pipeline, an advisor changes area, a new retailer enters the roadmap — Confluence reflects, the skill needs no update.
- **Public-safe skill:** no specific names in the repo, can be open-sourced.
- **Fork-friendly:** any company can use it by swapping just the Home page (and the cloudId).
- **Single source of truth:** Confluence. No divergence between skill and real KB.

---

## When bootstrap is NOT needed

- Purely conceptual question ("quais são as 6 categorias?", "como funciona o template de advisor?") — can be answered directly from the static files.
- Conversation continuing a previous session that was already bootstrapped.

**For any action that creates, edits, moves or references a specific pageId → bootstrap is mandatory.**
