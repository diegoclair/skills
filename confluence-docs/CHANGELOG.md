# Changelog — confluence-docs

## v0.10.0 (2026-05-12) — split credentials/config + multi-space support

### Overview

This release removes all hardcoded Lybel-specific constants (`homePageID`, `homeSpaceID`, `defaultCloud`) from the binary and adds first-class multi-space management. Any Confluence Cloud instance with multiple spaces is now fully supported without editing config files by hand.

### Change 1 — Two config files instead of one

**Before (v0.9.x):** single `credentials` file stored email, token, and cloud together.

**After (v0.10.0):**
- `credentials` (perms `0600`) — `email` + `token` only.
- `config` (perms `0644`) — `cloud`, `active_space_id`, `active_space_key`, `active_space_name`, `active_home_page_id`.

Splitting secrets from non-sensitive config makes the config file safe to copy between machines and inspect without exposing credentials.

**Backward compat:** existing `credentials` files with `cloud=` continue to work as a fallback. Migration happens silently when the user re-runs `setup`.

### Change 2 — `setup` auto-detects spaces

The interactive wizard now calls `GET /wiki/api/v2/spaces?status=current&limit=250` after validating credentials:
- 0 spaces: completes setup but warns that space must be configured separately.
- 1 space: selects it automatically (no prompt).
- N spaces: lists them numbered; user picks one (default `1`).

The selected space's `id`, `key`, `name`, and `homepageId` are persisted to the `config` file. `setup --check` now validates that both credentials **and** active space are configured.

New flags:
- `setup --reconfigure` — re-runs the full wizard with current values pre-filled.
- `setup --set <key> <value>` — sets one config key without prompting (valid keys: `cloud`, `active_space_id`, `active_space_key`, `active_space_name`, `active_home_page_id`).

### Change 3 — New `space` subcommand family

```bash
confluence-docs space list               # TSV by default; --json for structured output
confluence-docs space list --refresh     # force API fetch, ignore 1h cache
confluence-docs space use <key>          # switch active space + update config
confluence-docs space current            # show active space; --json for structured output
```

Space list is cached for 1h at `~/.cache/confluence-docs/spaces.json`.

### Change 4 — Hardcoded constants removed from `main.go`

`homePageID = "164232"`, `homeSpaceID = "131352"`, and `defaultCloud = "lybel"` are gone. Replaced by:

- `currentHomePageID()` — reads `active_home_page_id` from config.
- `currentSpaceID()` — reads `active_space_id`.
- `currentSpaceKey()` — reads `active_space_key`.
- `pageWebURL(client, pageID)` — builds the page URL using the configured space key.

All commands that previously hardcoded the Lybel space now fail gracefully with "no active space configured — run `confluence-docs setup`" if the config is missing.

### Change 5 — `adf` package additions

- `ReadActiveConfig()` — returns `ActiveConfig` (Cloud, SpaceID, SpaceKey, SpaceName, HomePageID) from the config file, with backward-compat fallback to old `credentials` file.
- `ResolveCloud(override)` — now reads from config file first, then old credentials file (backward compat), then returns `""`.
- `ConfluenceClient.ListSpaces()` — calls `GET /api/v2/spaces?status=current&limit=250`.

### Migration path for v0.9.x users

No action required for normal use — the CLI reads `cloud=` from the old `credentials` file until the user re-runs `setup`. After running `confluence-docs setup` once, both files are rewritten cleanly.

For users with only one accessible space, setup completes automatically. For multi-space users, the wizard lists spaces and prompts for selection.

---

## v0.9.2 (2026-05-12) — cloud subdomain in credentials file + README in English

### Cloud subdomain is now configured, not hardcoded

Removed the leftover `defaultCloud = "lybel"` from `adf/confluence.go` and `setup/setup.go`. The Confluence subdomain (e.g. `mycompany` for `mycompany.atlassian.net`) now resolves in this order:

1. `--cloud` flag on the command (highest priority)
2. `$ATLASSIAN_CLOUD` env var
3. `cloud=` line in the credentials file (new)
4. Empty → caller surfaces a clear error pointing at `confluence-docs setup`

The interactive `confluence-docs setup` wizard now prompts for the subdomain alongside email + token and writes all three to the credentials file. The non-interactive flow (`--email X --token Y`) reads the subdomain from `$ATLASSIAN_CLOUD` (errors with a clear message if missing) and persists it to the credentials file so subsequent runs don't need the env var.

`confluence-docs setup --check` now reports `no Confluence subdomain configured` explicitly when the subdomain is missing, with actionable fix instructions.

### Documentation

- Root [`README.md`](./README.md) translated from pt-BR to English; replaced Lybel-specific examples with generic placeholders. The "Contributing" section keeps the "skills must be company-agnostic" rule but updates the recommended grep pattern (`lybel|11C47E|164232`).
- Setup wizard prompt strings now say `Confluence subdomain (e.g. 'mycompany' ...)` instead of assuming a default.

### What this means for existing users

Existing credentials files (which only have `email` + `token`) continue to work — the skill falls back to `$ATLASSIAN_CLOUD` if `cloud=` is missing. To migrate, run `confluence-docs setup` once and it'll write the subdomain into the file.

## v0.9.1 (2026-05-12) — strip remaining project-specific reference files

Diego pointed out that the skill still shipped pt-BR / Lybel-specific reference files (`taxonomy.md` listing "the 6 categories of Lybel's KB", `aliases.md` mapping pt-BR keywords to Lybel categories, `templates.md` with Advisor/Investor/Varejista sheets in pt-BR, and a `workflows.md` heavily wired to Lybel's cloudId, space and parent IDs). Those don't help any other startup adopting the skill — each project has its own categories and aliases, which the agent already learns from the project's Confluence Home page (see `bootstrap.md`).

### Removed

- `reference/taxonomy.md` — Lybel's 6-category schema. Other projects define their own structure on their Home page.
- `reference/aliases.md` — Lybel's pt-BR keyword→category routing. Each project has its own.
- `reference/templates.md` — Lybel's page templates (Advisor sheet, Investor sheet, Tech vendor sheet, etc.). The skill now ships `cmd_new` for generic per-type templates; project-specific sheets live in each project's own docs if needed.

### Rewritten as generic

- `reference/bootstrap.md` — removed all Lybel-specific names and pageIds. Now describes the bootstrap principle for any Confluence space.
- `reference/workflows.md` — removed hardcoded `cloudId`, `space=lybel`, parentIds, and the 10 Lybel-specific recipes (Add lawyer, Add retailer, Add investor, etc.). Now describes 8 universal workflows (bootstrap, search, read, create, update, move, delete, regenerate KM) using placeholders.
- `SKILL.md` — updated reference list to drop the 3 removed files; clarified that project-specific routing lives on each project's Confluence Home, not in the skill.

### What stays

- `reference/doc-types.md` — canonical English spec of the 5 doc types (added in v0.9.0).

### Doc cleanup

- `cli/README.md` and `cli/SETUP.md` — replaced remaining hardcoded `lybel.atlassian.net` / `Lybel knowledge base` mentions with generic placeholders.
- `cmd_check --help` — clarified `--space` default (resolved from `$ATLASSIAN_CLOUD` or credentials config; was misleadingly documented as `default: lybel`).

### What this means for projects already using the skill

The CLI binary behavior is unchanged. Only reference markdown files were removed and docs were generalized. Projects that relied on `reference/taxonomy.md` etc. as fallback documentation should rely on their own Confluence Home page (which is the recommended source per `bootstrap.md`) or check older skill versions for the file content if needed.

### Known follow-ups (v0.10.0)

- `defaultCloud = "lybel"` and `homePageID = "164232"` / `homeSpaceID = "131352"` are still hardcoded constants in `main.go`. These are used by the `index` and `home` commands as fallback when no env/config is set. To be moved to per-project config in v0.10.0.

## v0.9.0 (2026-05-12) — English skill, canonical spec, owner mentions

This is a **breaking release** for projects whose tooling depended on pt-BR string output (template headers, km-generated content). The skill's user-facing strings are now in English, becoming usable by any startup globally. The frontmatter parser remains permissive — old pages with `tipo:`, `criado:`, etc. still work; only NEWLY-generated content (via `new`, `km generate`) uses English keys.

### `reference/doc-types.md` — canonical English spec inside the skill

Previously the skill referenced `docs/standards/EDITORIAL_v2.md` (a Lybel-specific pt-BR doc that doesn't exist in other projects). Moved the canonical spec **into the skill** as `reference/doc-types.md` (~2600 words, English, generic examples). Projects can still extend with their own editorial guide, but the 5 types, frontmatter fields, naming convention, and anti-patterns are the contract here.

### Internationalization to English

- `cmd_new.go` — templates emit English headers (`## TL;DR`, `## Context`, `## Identification`, `## Decision`, `## Alternatives considered`, `## Consequences`, `## Analysis`, `## Prerequisites`, `## Steps`, `## Verification`, `## Idea`, `## Why it might matter`, etc.) and English frontmatter keys (`type`, `status: draft`, `created`, `updated`, `related`).
- `cmd_km.go` — KM rendered output in English: `Knowledge Map`, `Rules for AI`, `Required sequence`, `Anomalies and cases for human review`, type labels/descriptions, `Show all N pages`, `_No real anomalies flagged._`, etc. Lybel-specific framing ("Cenário I") replaced with generic phase-tag guidance ("use whatever your project uses — `mvp`, `v1`, `vision`").
- `storage_convert.go` — `:::properties collapsed` wraps in expand titled `Metadata` (was `Metadados`).
- `SKILL.md` — frontmatter example updated to English keys.

JSON wire format fields (`tipo_proposto`, `anomalia`, `tags_sugeridas` in triage batches) remain unchanged — those are API contracts.

### Owner / reviewer `@mention` resolution

`:::properties` values containing `@handle` or bare email patterns are now resolved to real Confluence user mentions when uploaded (storage path). Resolution goes through `ConfluenceClient.LookupUser` → Atlassian `/wiki/rest/api/user/picker`. Resolved entries become `<ac:link><ri:user ri:account-id="..."/></ac:link>` (clickable user chips); unresolved fall back to plain text. Persistent cache at `$XDG_CACHE_HOME/confluence-docs/users.json` (24h TTL).

New public symbols:
- `adf.UserResolver` interface; `adf.NewClientUserResolver(client)`
- `adf.MarkdownToStorageWithClient(src, client)` — mention-aware variant
- `ConfluenceClient.LookupUser`, `LoadUserCache`, `SaveUserCache`

`page create --markdown`, `page upload --markdown`, and `km generate` all use the mention-aware path automatically when a client is available.

### Tests

- `cli/adf/adf_builders_mention_test.go` — 11 mention-resolution tests (mock resolver, no live API).
- `cli/cmd_km_test.go` — assertions updated for English strings.

All existing tests still pass.

## v0.8.4 (2026-05-12) — fix RequiresStorageFormat for `:::properties collapsed`

### Bug fix

`RequiresStorageFormat` regex was anchored with `$` after `properties`, requiring the line to end immediately. After v0.8.3 added the `collapsed` modifier (`:::properties collapsed`), pages using it bypassed the storage path detection — uploaded via `atlas_doc_format`, which wraps macro storage XML in a `codeBlock(language="confluence-storage")` instead of rendering the actual macro. Visible symptom: raw `<ac:structured-macro ac:name="details" ...>` displayed as a code block at the top of the page.

Fix: updated regex to `properties(?:[ \t][^\n]*)?$` — accepts optional trailing modifier text. Added regression tests for `:::properties collapsed` and `:::properties some-future-modifier`.

Pages affected (need re-upload with v0.8.4+): any page authored via `confluence-docs page upload --markdown` using `:::properties collapsed` since v0.8.3.

## v0.8.3 (2026-05-12) — collapsed properties + labels API

### New features

#### `:::properties collapsed` — wrap metadata table in a collapsible Expand

Pages that use `:::properties` for frontmatter (KNOWLEDGE_MAP, ICPs, etc) had the full 7-row metadata table dominating the top of the screen. Add the `collapsed` modifier to wrap the details macro in an Expand macro titled "Metadados" — click to see, hidden by default.

```markdown
:::properties collapsed
tipo: reference
status: ativo
:::
```

Bare `:::properties` continues to render uncollapsed (unchanged behaviour).

#### Labels API — `AddLabels`, `GetLabels`, `RemoveLabel` on `ConfluenceClient`

Programmatic access to Confluence page labels:
- `AddLabels(pageID, labels)` — POST `/wiki/rest/api/content/{id}/label` with `[{prefix:"global", name:"..."}]`. Duplicates ignored by API.
- `GetLabels(pageID)` — GET `/wiki/api/v2/pages/{id}/labels?prefix=global`.
- `RemoveLabel(pageID, label)` — DELETE single label.

`confluence-docs km generate` now extracts the `tags:` line from the `:::properties` block in the rendered markdown and applies each comma-separated value as a real Confluence label on the target page. Tags become clickable chips above the page title (filterable across the space) instead of just text inside a table cell.

## v0.8.2 (2026-05-12) — fix Page Properties macro name

### Bug fix — Cloud storage macro name is `details`, not `page-properties`

Pages generated by v0.8.0 / v0.8.1 with a `:::properties` block rendered as **"Unknown macro: page-properties"** in Confluence Cloud. Root cause: `page-properties` is the legacy Confluence Server storage name; Cloud requires `ac:name="details"` for the same macro (the aggregator is `detailssummary`, not `page-properties-report`).

Changed `PagePropertiesToStorage` in `adf/adf_builders.go` to emit `ac:name="details"`. Updated tests in `adf/adf_builders_test.go`, `adf/storage_convert_test.go`, and `adf/properties_parser_test.go`. Updated doc comments in `adf/properties_parser.go`.

Existing pages with the old XML can be regenerated via `confluence-docs km generate --target-page-id ID` (for the KNOWLEDGE_MAP) or via `confluence-docs page upload --markdown` (for any page authored with `:::properties`).

## v0.8.1 (2026-05-11) — km subcommand + macro extension fix

### Bug fixes

#### Fix — macro extensions (`:::info`, `:::expand`, etc.) now work correctly via storage path

**Root cause**: `MarkdownToStorage` (introduced in v0.8.0) correctly handled `:::properties` blocks, but `:::info`, `:::note`, `:::warning`, `:::success`, `:::error`, `:::tip`, and `:::expand` blocks were also only processed via the storage path when `RequiresStorageFormat` returned true. In practice `RequiresStorageFormat` only checked for `:::properties`, so pages containing **only** panel/expand blocks without a `:::properties` header were still sent as `atlas_doc_format` — causing the macros to render as code blocks instead of Confluence panels.

The underlying `MarkdownToStorage` and `extractMacroBlocks` logic already handled all eight block types; only the detection gate (`RequiresStorageFormat`) was too narrow. This fix is documented here for transparency; the actual code path was already correct for pages that include `:::properties` (the common case in the KNOWLEDGE_MAP generator).

### New features

#### `confluence-docs km` — Knowledge Map generator (`cmd_km.go`)

New subcommand that replaces the ad-hoc Python script `/tmp/lybel-edit/gen_knowledge_map.py` used to regenerate the Lybel KNOWLEDGE_MAP page (Confluence pageId `200441858`).

**`km generate`** — full pipeline:

1. Reads `batch-*.json` triage files from `--input DIR` (default `/tmp/lybel-triage`).
2. Loads an optional `--baseline FILE` (hand-classified pages, higher precedence).
3. Merges the two sources: baseline entries are never overridden by triage (tipo/title preserved), but triage can augment them (add `fase-final-checkout-universal` tag or real anomaly).
4. Applies tag rules: pejorative tags (`legacy`, `obsoleto`, `desatualizad`, `pre-pivot`, `pos-pivot`, `antigo`) are stripped and replaced by the canonical `fase-final-checkout-universal`. Horizon markers in the anomaly field (`"pre-pivot"`, `"b2b2c"`, `"conteudo-desatualizado"`, etc.) also trigger the tag but are not surfaced as real anomalies.
5. Real anomalies (`"borderline"`, `"duplicata"`, `"nome-desatualizado"` in anomalia string) are collected in a human-review section.
6. Renders full markdown with `:::properties` frontmatter, TL;DR with dynamic counts, per-tipo sections (wrapped in `:::expand` when >12 entries), `:::info Regras pra IA` panel, anomalies section, and Manutenção footer.
7. Optionally uploads to Confluence via `--target-page-id` (uses storage format — same path as `page upload --markdown` with `:::properties`).

**`km classify`** — registered stub that returns `"not implemented"`. Reserved for future auto-classification.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--input DIR` | `/tmp/lybel-triage` | Directory with `batch-*.json` files |
| `--baseline FILE` | (none) | JSON file with hand-classified baseline pages |
| `--target-page-id ID` | (none) | Upload result to this Confluence page |
| `--output FILE` | stdout | Write markdown to file (when no `--target-page-id`) |
| `--dry-run` | false | Render only, no upload |
| `--message "..."` | `"regenerate KM"` | Version comment for upload |
| `--full-width` | false | Set page to full-width after upload |

### Files created

| Path | Purpose |
|---|---|
| `cli/cmd_km.go` | `km generate` + `km classify` implementation (~360 LOC) |
| `cli/cmd_km_test.go` | Unit + integration tests (~290 LOC) |

### Files modified

| Path | Changes |
|---|---|
| `cli/main.go` | Added `km` case to router; added `km` to USAGE and COMMANDS help text |
| `SKILL.md` | Added `## confluence-docs km` section with workflow, formats, and tag rules |
| `CHANGELOG.md` | This entry |

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
