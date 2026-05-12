# Changelog — confluence-docs

## v0.8.0 (2026-05-12) — bug fixes

### Bug fixes

#### Fix 1 — `:::properties` now renders as real Page Properties macro (CRITICAL)

**Root cause**: `page create --markdown` and `page upload --markdown` were sending all content as `atlas_doc_format` (ADF). The `:::properties` block was wrapped in a `codeBlock(language="confluence-storage")` ADF node, which Confluence rendered as a syntax-highlighted code block, not as the page-properties macro.

**Fix**: Added `RequiresStorageFormat(markdown string) bool` in `adf/storage_convert.go`. When the markdown source contains a `:::properties` block, both `page create` and `page upload` now:
1. Call `MarkdownToStorage(src)` which uses goldmark's HTML renderer for normal content and substitutes `:::properties` blocks with their storage XML via `PropertiesBlockToStorageXML`.
2. Upload with `representation: "storage"` (new `CreatePageStorage` / `UpdatePageStorage` methods in `adf/confluence.go`) instead of `atlas_doc_format`.

**Tested**: Created live test page on Confluence (`200605698`). Storage format confirmed via `page get --format storage` showing `<ac:structured-macro ac:name="page-properties" ...>`. Page deleted after test.

**Files changed**: `adf/storage_convert.go` (new), `adf/confluence.go` (new methods), `main.go` (branch in `runPageCreate` and `runPageUpload`), `adf/storage_convert_test.go` (new tests).

#### Fix 2 — `check` threshold default lowered to 0.4

**Root cause**: Default threshold of 0.7 was too high — partial title matches like "Magie WhatsApp" vs "Magie — WhatsApp Banking" (similarity 0.48) were returned as `suggestion: create` instead of `update_existing`.

**Fix**: Default threshold in `cmd_check.go` changed from `0.7` to `0.4`. The `--threshold` flag still allows manual override.

#### Fix 3 — Version number fixed (was "dev")

**Root cause**: `var version = "dev"` was the fallback when no ldflags were injected.

**Fix**: Default now `v0.8.0`. Makefile already injects via `-ldflags "-X main.version=$(VERSION)"` so tagged builds override correctly.

#### Fix 4 — `new` templates missing `relacionados:` field

**Root cause**: `generateTemplate` in `cmd_new.go` did not include `relacionados:` in the `:::properties` block for any doc type.

**Fix**: Added `relacionados: ""` line to the properties block in all templates (reference, decision, explanation, how-to, capture). The line is emitted right after `supersedes` (if present) and before `criado`.

### Files changed

| Path | Changes |
|---|---|
| `cli/adf/storage_convert.go` | New: `RequiresStorageFormat`, `MarkdownToStorage`, `extractPropertiesBlocks` |
| `cli/adf/storage_convert_test.go` | New: unit tests for storage conversion |
| `cli/adf/confluence.go` | New: `CreatePageStorage`, `UpdatePageStorage` methods |
| `cli/cmd_check.go` | Default threshold 0.7 → 0.4; doc comment updated |
| `cli/cmd_new.go` | Added `relacionados: ""` line to all template properties blocks |
| `cli/main.go` | `runPageCreate` and `runPageUpload` branch on `RequiresStorageFormat`; version default `dev` → `v0.8.0` |

## v0.4.0 (2026-05-11)

### New features

#### Feature 1 — ADF native builders (`adf/adf_builders.go`)

Added typed builders for modern Confluence Cloud ADF nodes:

- `Status(text, color, localId)` — inline status badge (`green|yellow|red|blue|purple|neutral`)
- `InlineCard(url)` — inline smart link card
- `BlockCard(url)` — block-level smart link card
- `EmbedCard(url, layout)` — embedded preview (YouTube, Figma, etc.)
- `Layout(type, ...columns)` — layoutSection with layoutColumn children; presets: `single`, `two_equal`, `two_left_sidebar`, `two_right_sidebar`, `three_equal`, `three_with_sidebars`
- `LayoutColumn(widthPct, ...content)` — individual column
- `MarshalBodyValue(doc)` — serialize ADF doc as a JSON string (required double-encoding for Confluence API v2 `body.value`)

All builders are pure functions with no I/O.

#### Feature 2 — Full-width pages (`page create`, `page upload`)

`page create` and `page upload` now accept:
- `--full-width` — set page to full-width layout (posts `content-appearance-draft` and `content-appearance-published` page properties after create/update)
- `--fixed-width` — revert to fixed-width layout (default Confluence behavior)

Implemented via `ConfluenceClient.SetPageAppearance(pageID, appearance)` in `adf/confluence.go`, which upserts both page properties atomically (POST → 409 fallback to PUT).

#### Feature 3 — Page Properties macro builder (`adf/properties_parser.go`, `adf/adf_builders.go`)

Markdown extension `:::properties ... :::` generates the Confluence Page Properties macro as storage XML:

```markdown
:::properties
tipo: reference
status: ativo
owner: @diego
tags: psp, cobranca
relacionados: [[Page Title]], [[id:12345]]
criado: 2026-05-12
atualizado: 2026-05-12
:::
```

- `ParsePropertiesBlock(body)` — parses key:value lines
- `PagePropertiesToStorage(entries)` — converts to `<ac:structured-macro ac:name="page-properties">` XML
- `[[Title]]` links become `<ac:link><ri:page ri:content-title="Title"/></ac:link>`
- `[[id:N]]` links use the ID as the `ri:content-title` (callers may resolve to title)
- Integrated into `adf/macros.go` preprocessor as a new block kind

#### Feature 4 — Smart Link embed in markdown (`adf/smartlinks.go`)

Markdown preprocessing now detects standalone URL patterns and converts them to ADF Smart Link nodes:

| Input | ADF output |
|---|---|
| `![embed](URL)` (standalone line, alt="embed") | `embedCard` with `layout: wide` |
| `https://...` on its own line | `blockCard` |
| `[text](url)` where text == url (auto-pasted) | `blockCard` |
| Named links in prose `[text](url)` | unchanged (regular link mark) |

Implemented via `preprocessSmartLinks()` which runs before the TOC/panel/expand scan. Backward-compatible: named links `[text](url)` in paragraphs are unaffected.

#### Feature 5 — `confluence-docs check` (`cmd_check.go`)

New subcommand for duplicate detection before page creation:

```bash
confluence-docs check --title "Análise Stripe Brasil" [--type reference] [--tags psp] [--threshold 0.7]
```

- CQL search for similar titles in the space
- Trigram-based Jaccard similarity scoring (fallback to Levenshtein for short strings)
- Returns JSON: `{ exists, similar: [{id, title, url, similarity_score}], suggestion: create|update_existing }`
- `--threshold` (default 0.7): score above which `suggestion` becomes `update_existing`
- `--text` flag for plain-text output

#### Feature 6 — `confluence-docs new <type>` (`cmd_new.go`)

New subcommand to generate markdown templates for the five standard doc types:

```bash
confluence-docs new reference --title "..." [--parent-id ID] [--full-width] [--output FILE]
confluence-docs new decision  --title "..." [--supersedes PAGE_ID]
confluence-docs new explanation --title "..."
confluence-docs new how-to    --title "..."
confluence-docs new capture   --title "..."
```

Each template includes:
- `:::properties` block with `tipo`, `status: rascunho`, `owner` (from `git config user.email`), `criado`/`atualizado` (today)
- `## TL;DR` heading with a type-appropriate prompt
- Type-specific structural headings (e.g. `decision` includes Alternatives Considered table, Consequences, Review date)

#### Feature 7 — SKILL.md updated

SKILL.md now documents:
- The five doc types (reference, decision, explanation, how-to, capture) with purpose and creation triggers
- The mandatory `check` → `new` → `page create` workflow
- How to invoke each new feature with example commands
- Backward-compatibility guarantees

### Files created

| Path | Purpose |
|---|---|
| `cli/adf/adf_builders.go` | Status, InlineCard, BlockCard, EmbedCard, Layout, MarshalBodyValue builders |
| `cli/adf/adf_builders_test.go` | Tests for all new ADF builders |
| `cli/adf/properties_parser.go` | ParsePropertiesBlock, PropertiesBlockToStorageXML |
| `cli/adf/properties_parser_test.go` | Tests for properties parser |
| `cli/adf/smartlinks.go` | Smart link line detection and preprocessSmartLinks() |
| `cli/adf/smartlinks_test.go` | Tests for smart link detection |
| `cli/cmd_check.go` | `confluence-docs check` subcommand |
| `cli/cmd_new.go` | `confluence-docs new <type>` subcommand |
| `CHANGELOG.md` | This file |

### Files modified

| Path | Changes |
|---|---|
| `cli/main.go` | Added `check` and `new` to command router; added `--full-width`/`--fixed-width` to `page create` and `page upload`; updated helpText |
| `cli/adf/macros.go` | Integrated `preprocessSmartLinks` and `:::properties` block handling into the preprocess pipeline; updated openRe to handle hyphenated block kinds |
| `cli/adf/confluence.go` | Added `SetPageAppearance`, `upsertPageProperty`, `getPagePropertyIDAndVersion` methods |
| `SKILL.md` | Added doc types taxonomy, check workflow, new features reference section |

### Known limitations / TODOs

- `:::properties` renders as a `codeBlock(language="confluence-storage")` in ADF. This is the safest approach for storage XML round-trips, but Confluence renders it as a code block visually if the page body format is `atlas_doc_format`. For the macro to render correctly as a table, the page should be created/updated using the `storage` representation. A future improvement could detect `confluence-storage` blocks and switch the body representation accordingly.
- Smart link detection for `[text](url)` on its own line: only converts when `text == url` (auto-pasted URLs). Named links like `[Click here](url)` in prose always stay as regular links. This is intentional to avoid breaking existing content.
- `confluence-docs check` uses CQL `title ~` which does a Confluence-side fuzzy match first, then applies trigram scoring client-side. For spaces with many similarly-named pages, increase `--limit` to cast a wider net.
- `confluence-docs new` reads `git config user.email` for the owner field. If git is not configured, falls back to `$USER`. A future improvement: read from the CLI credentials file.
- The `--supersedes` flag for `new decision` adds the ID to the `:::properties` block but does not automatically update the superseded page. That should be done manually.
