# confluence-docs

> A Claude skill + Go CLI that edits Confluence Cloud pages **without burning the conversation context.** Surgical section, table, and macro edits with byte-level precision — drop-in replacement for round-tripping ADF JSON through the Atlassian MCP.

Built for any team running a Confluence Cloud space alongside Claude (or any other LLM agent) that maintains docs as a daily artifact, not a quarterly cleanup. Companion to Atlassian's MCP — uses it as fallback, but does the heavy lifting itself.

```bash
# One-shot install (Linux/macOS)
curl -fsSL https://raw.githubusercontent.com/diegoclair/skills/main/confluence-docs/install/install.sh | bash
```

---

## Why this exists — the token-cost story

Editing Confluence pages via the Atlassian MCP looks elegant: one `getConfluencePage` + one `updateConfluencePage`. But every read returns the **full ADF body** of the page, and every write requires sending the **full ADF body** back. ADF (Atlassian Document Format) is JSON with deeply nested attrs/marks/content arrays. For a typical 1,500-word page that means ~40 KB of ruidoso JSON entering and leaving the conversation — every single edit.

If you're doing a long doc session (refactor, sprint review, knowledge-base cleanup), the cost compounds fast. We hit context-window pressure inside one session of ~40 doc operations.

**This CLI fixes that by doing three things differently:**

1. **Section-level and table-level operations** — `--replace-section`, `--table-update-cell`, `--table-move-row`, etc. The CLI fetches the ADF, mutates only the targeted node, and re-PUTs the page. The full ADF **never enters the conversation context.** Only the operation status (`{"status":"ok","fromVersion":7,"toVersion":8}`) does.
2. **Markdown-shaped reads** — `page get --section --format text` returns the section as readable markdown (`## headings`, `- bullets`, pipe tables, `[text](url)` links) instead of ADF JSON. Roughly 5× more compact for the same content.
3. **A `digest` operation cheaper than any read** — `page digest` returns title + outline + status emoji + macros-present, around 500–1,500 bytes. Most "what's the status of X?" / "what's on this page?" questions need nothing more.

### Measured bytes — same page, three ways

The numbers below come from a real page in a real workspace (a 11-row ICP scoring matrix with `:::properties` macros, smart links, and embedded blockCards — roughly 1,400 words). Measured with `wc -c` on the actual outputs.

| Read operation | What you get | Bytes | Tokens (approx) |
|---|---|---|---|
| `page digest --page-id X` | Title + status + outline + macro list | **1,344** | ~336 |
| `page get --section 'X' --format text` | One section as readable markdown | **2,264** | ~566 |
| `page get --format text` | Whole page as readable markdown | **15,017** | ~3,754 |
| `page get --format adf` (= what MCP returns) | Full ADF JSON of the page | **45,880** | ~11,470 |

Same page, same content, four different reads. A targeted read via the CLI costs **20× to 34× fewer bytes** than the equivalent MCP `getConfluencePage`.

### Measured bytes — writes

Imagine the typical edit during a long doc session: update one row inside a 12-row table.

| Write path | Conversation bytes (in + out) |
|---|---|
| CLI `--table-update-cell --match-col X --match-value Y` | **~400 bytes** (command + status JSON) |
| CLI `--replace-section` rewriting the whole section | **~3 KB** (markdown of section + status JSON) |
| MCP `getConfluencePage(adf)` + edit + `updateConfluencePage(adf)` | **~90 KB** (45 KB read + 45 KB write) |

For surgical edits, the CLI is **~225× lighter** than going through the MCP directly. Even the heaviest CLI path (rewriting an entire section) is **~30× lighter** than the MCP round-trip.

### Why this matters in practice

A real session with this skill (40 doc operations: page creates, table updates, section rewrites, reorder, soft delete) accumulates roughly **50–80 KB of conversation bytes**. The same session via MCP direct would accumulate roughly **1.5–2 MB.**

That's the difference between a session that fits in the context window comfortably and one that paginates or fails halfway through. It's also the difference in per-session token bill — typically 20–30× less spend on doc ops, at no quality cost.

### Bonus: macro preservation comes free

Markdown-flavoured updates via MCP (`contentFormat: "markdown"`) **silently flatten Confluence macros** — TOCs, expand blocks, panels, page-properties, smart-link cards. The CLI parses to ADF, mutates only the targeted node, and re-PUTs the page byte-for-byte for everything outside the edit boundary. Macros stay intact.

---

## Quick start

After the one-shot install above:

```bash
confluence-docs setup            # interactive: paste Atlassian email + token, pick a space
confluence-docs --version        # confirm install (expect v0.11.0+)
confluence-docs home --digest    # bootstrap: read the project's Home page outline
```

Then in any Claude conversation:

> _"Find the page for X"_ — Claude calls `home --query "X"` (cached, ~50 bytes).
> _"What's in page Y?"_ — `page digest --page-id Y` (~1 KB).
> _"Update the score row for Z to 8.4"_ — `page apply --table-update-cell ... --col-name Score --value 8.4` (~300 bytes).
> _"Move the Lash row to position 10"_ — `page apply --table-move-row ... --position 10` (~250 bytes).

Full operation catalogue lives in [`SKILL.md`](SKILL.md) and [`reference/operations-matrix.md`](reference/operations-matrix.md).

---

## Key features

### Surgical section editing

```bash
confluence-docs page apply --page-id <id> \
  --replace-section 'Roadmap' --fragment new.md
```

Replaces one section (heading + descendants until the next equal-or-higher heading), preserves every macro outside the edit boundary, automatic 409-retry on concurrent edits.

### Table operations with column-name matching (v0.11+)

```bash
# Match a row by any column — not just the first cell
confluence-docs page apply --page-id <id> \
  --table-update-cell 'ICP Matrix' \
  --match-col 'ICP' --match-value 'Lash designer' \
  --col-name 'Score' --value '2.4'

# Move a row to a new position without rewriting the table
confluence-docs page apply --page-id <id> \
  --table-move-row 'ICP Matrix' \
  --match-col 'ICP' --match-value 'Lash designer' \
  --position 10
```

Eliminates the long-standing failure mode where a non-unique first column (numeric rank, repeating ID) made surgical edits impossible without rewriting the whole section.

### Confluence-flavoured markdown

The `:::properties`, `:::expand`, `:::warning`, `:::info` blocks, smart-link auto-detection (YouTube, Loom, GitHub, Linear become rich cards), `[TOC]` macro, page labels, and `@mentions` are all native first-class citizens of the markdown input — no hand-crafted XML storage format.

```markdown
:::properties
type: reference
status: active
tags: psp, billing, recurring
related: [[Page Title]], [[id:12345]]
:::

# Title

[TOC]

## Context

See the [Linear ticket](https://linear.app/team/issue/PSP-12) for full thread.

![embed](https://www.youtube.com/watch?v=dQw4w9WgXcQ)
```

### Knowledge Map regeneration

`confluence-docs km generate` consumes triage JSON batches (typically produced by subagents) and renders a classified Knowledge Map page using your project's baseline overrides. Idempotent, dry-run capable, supports tag normalization (no more pejorative "obsolete" tags polluting the map).

### Duplicate detection before page creation

`confluence-docs check --title "Stripe Brazil Analysis"` runs trigram fuzzy matching against existing pages and returns either `"create"` or `"update_existing"` with the offending neighbours. Should be mandatory before any `page create`.

### Template generator

`confluence-docs new <type>` produces a starter markdown for each of the five canonical doc types (`reference`, `decision`, `explanation`, `how-to`, `capture`) with frontmatter, required headings in the right order, and descriptive placeholders. See [`reference/doc-types.md`](reference/doc-types.md) for the full editorial contract the skill enforces.

---

## Documentation map

| Reader | Start here |
|---|---|
| **Claude (skill activation)** | [`SKILL.md`](SKILL.md) — the agent contract |
| **Installing the skill** | [`cli/README.md`](cli/README.md) — install + setup wizard |
| **Operation gotchas + decision matrix** | [`reference/operations-matrix.md`](reference/operations-matrix.md) |
| **Editorial patterns for pages** | [`reference/editorial-patterns.md`](reference/editorial-patterns.md) |
| **CLI features** (full-width, properties, smart links, check, new, km) | [`reference/features.md`](reference/features.md) |
| **Configuration / spaces / cache** | [`reference/configuration.md`](reference/configuration.md) |
| **Canonical doc-type spec** | [`reference/doc-types.md`](reference/doc-types.md) |
| **Generic workflows (search, read, create, update, delete)** | [`reference/workflows.md`](reference/workflows.md) |
| **Bootstrap procedure per session** | [`reference/bootstrap.md`](reference/bootstrap.md) |
| **Release history** | [`CHANGELOG.md`](CHANGELOG.md) |

---

## Compatibility

- Confluence **Cloud** — yes, fully tested. Uses the public v2 REST API plus a single v1 endpoint for sibling reorder (no v2 equivalent exists).
- Confluence **Server / Data Center** — not currently supported (different API surface; PRs welcome).
- Tested against Atlassian Cloud API as of 2026-05.

---

## Project agnosticism

This skill is deliberately **timeless and project-agnostic**. It ships:

- Zero hardcoded company names, page IDs, taxonomies, or workflows.
- Zero data files. All project-specific routing (categories, aliases, KM index) lives on **your** Confluence Home page and is read fresh per session via a 1-hour-TTL local cache.
- A single canonical doc-type contract in [`reference/doc-types.md`](reference/doc-types.md) — same five types (`reference`, `decision`, `explanation`, `how-to`, `capture`) for every project that adopts the skill.

Each project picks its own Confluence space, its own taxonomy on the Home page, its own KM page ID at install time. The skill never assumes anything about your space.

---

## License & attribution

Built and maintained by [Lybel](https://lybel.com.br) — released as a free Claude skill for any team that wants to ship faster on Confluence. Contributions welcome at the project repo. Acknowledgements to the Anthropic skill-creator guidance the structure follows.

If you ship this in a team, drop a line — it's nice to know the skill is paying back somewhere.
