# Bootstrap — How the Skill Orients Itself

## Principle

This skill **does not store project-specific data**. It knows:

- The 5 canonical doc types (see `doc-types.md`)
- Standard workflows for searching, creating, updating pages
- Frontmatter, naming and cross-linking rules
- Macro syntax (`:::properties`, `:::info`, `:::expand`, etc.) and storage XML emission

The **current state of your knowledge base** (your category structure, page IDs, who the advisors are today, what's in the active roadmap, etc.) lives **exclusively in your Confluence space's Home page**. The skill fetches it fresh before acting and caches locally for cheap reads.

---

## Bootstrap procedure

In every new session, Claude queries the cache directly — the CLI handles freshness, you don't track sessions, TTLs, or cache state.

1. **Query the cache for navigation:**
   ```
   confluence-docs home --query "<term>"   # search aliases / decision map / page ID index
   confluence-docs home --digest           # outline view of the Home page
   confluence-docs home --show             # full text rendering of the Home
   ```
   These commands auto-refresh when the cache is missing or older than 1h. No need to call `home --refresh` first.

   If the CLI is unavailable, fall back to `mcp__atlassian__getConfluencePage(pageId=<home>, contentFormat="markdown")` and operate from in-memory data.

2. **Extract from the Home content** whatever your project documents there. Typical patterns:
   - A "where do I put X?" decision table (item → parent page)
   - Aliases (natural-language keywords → category)
   - A Page ID Index for structural parents
   - Category descriptions with current links

3. **Use this data as the source of truth** for the rest of the conversation.

4. **Writes to the Home auto-refresh the cache.** When you `page apply` on the Home, the cache is automatically refreshed after the PUT succeeds. No need to call `home --refresh` after writes.

5. **The cache is shared across all sessions on the same machine.** If one session refreshes, every other session reading next sees the updated state. No per-session bookkeeping.

6. **`home --refresh`** exists for the rare case where you know another machine just updated the Home and you don't want to wait for the 1h TTL.

**Why this design:** the cache costs at most one GET per hour per machine and zero per query within that hour. The MCP route, in contrast, would re-fetch the Home (10–25 KB ADF) for every navigation question. The cache also enforces a clean invariant: it is **read-only for navigation**, and `page apply` always GETs fresh ADF before any PUT, so we never overwrite changes from another machine.

---

## Knowledge Map (cross-cutting index by type)

If your project uses the `confluence-docs km` command, there is a second navigation entry point in addition to the Home: the **KNOWLEDGE_MAP page**, which indexes pages by **doc type** (`reference` / `decision` / `explanation` / `how-to` / `capture`) instead of by topic area.

When the agent needs to "find all decisions" or "find all how-to guides", the KM is the right entry point. When it needs "find pages about partners" or "where do I put a new competitor analysis", the Home is the right entry point. The two indexes are complementary.

See `doc-types.md` for the canonical spec of the 5 types.

---

## Why this design

- **Timeless skill:** never goes stale as your project evolves. Confluence reflects current state; the skill never needs an update for content changes.
- **Project-agnostic:** the skill ships zero project-specific data — usable by any company with a Confluence space.
- **Single source of truth:** Confluence. No divergence between skill and the real KB.

---

## When bootstrap is NOT needed

- Purely conceptual questions about the skill itself ("how do macros work?", "what's the difference between reference and explanation?") — answer from `doc-types.md` or SKILL.md.
- A conversation continuing a previous session that was already bootstrapped in this CLI invocation.

**For any action that creates, edits, moves, or references a specific pageId → bootstrap is mandatory.**
