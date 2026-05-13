# CLI Features — full-width pages, properties, smart links, templates, knowledge map

> **When to read:** when a user request touches one of the features below (full-width dashboards, page properties metadata, knowledge map regeneration, template generation, smart-link embeds, duplicate detection). The skill body keeps only the daily-use commands; details live here.

## Contents

- [Full-width pages](#full-width-pages)
- [Page Properties macro (`:::properties` block)](#page-properties-macro-properties-block)
- [Smart Link embeds](#smart-link-embeds)
- [`check` — duplicate detection](#check--duplicate-detection)
- [`new <type>` — template generator](#new-type--template-generator)
- [`km` — Knowledge Map generator](#km--knowledge-map-generator)

---

## Full-width pages

`page create` and `page upload` accept `--full-width` (and `--fixed-width` to revert):

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

Under the hood the CLI posts two properties (`content-appearance-draft` and `content-appearance-published`) to the page properties API after the create/update.

**Use full-width for:** dashboards, wide tables, anything where horizontal real estate matters. Fixed-width remains the Confluence default for prose pages.

## Page Properties macro (`:::properties` block)

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

**Use properties for:** any page that should be discoverable in a Page Properties Report (KM index, reference catalogs, decision logs). Properties are how the Confluence KM macro aggregates pages.

## Smart Link embeds

In markdown, certain standalone URL patterns are automatically converted to Confluence Smart Link nodes:

| Markdown | ADF node | Renders as |
|---|---|---|
| `![embed](https://youtube.com/...)` | `embedCard` (wide layout) | Embedded player/preview |
| `https://linear.app/...` on its own line | `blockCard` | Preview card |
| `https://github.com/...` on its own line | `blockCard` | Preview card |
| `[text](url)` in the middle of a paragraph | normal link | Inline hyperlink |

Bare URL lines and `![embed](url)` trigger smart link conversion. Named links like `[Click here](url)` in prose always remain regular links.

## `check` — duplicate detection

**Always run `check` before creating a new page or running `new`.** Catches exact duplicates and near-duplicates before they clutter the space.

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

Uses trigram-based fuzzy matching (Jaccard similarity, threshold 0.7 by default). Accepts `--tags` to filter by Confluence labels and `--threshold` to tighten or loosen the match.

## `new <type>` — template generator

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

## `km` — Knowledge Map generator

Generates a KNOWLEDGE_MAP page from triage JSON batches produced by subagents, with optional hand-classified baseline overrides.

### Typical workflow

```bash
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
