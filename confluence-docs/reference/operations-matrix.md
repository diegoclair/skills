# Operations Matrix — CLI subcommands, constraints, fail modes, workarounds

> **Why this file exists:** the CLI exposes many fine-grained operations (`page apply` with `--table-add-row`, `--table-update-cell`, `--table-update-row`, `--replace-section`, etc.). Each carries subtle constraints that aren't visible from `--help`. This matrix prevents trial-and-error and consolidates gotchas. Read it once per session; keep it open during long edit chains.

## Contents

- [Page reads](#page-reads)
- [Page edits — section level](#page-edits--section-level)
- [Page edits — table level](#page-edits--table-level)
- [Decision: table-* vs replace-section](#decision-table--vs-replace-section)
- [Structural operations (move, reorder, delete)](#structural-operations-move-reorder-delete)
- [Common failure modes](#common-failure-modes)
- [Title patterns for child pages](#title-patterns-for-child-pages)

---

## Page reads

| Operation | Use case | Constraint / cost |
|---|---|---|
| `page digest --page-id ID` | First read of any page | Cheap (~500 bytes). Parses leading status emoji into a `Status:` field. Always start here. |
| `page get --section 'H'` | Read one section | Heading must match exactly. Wrap in single quotes if `$` is in the heading (bash expands `$` silently otherwise). |
| `page get --format text` | Whole page as readable text | Token cost scales with page size — avoid unless `digest` outline is insufficient. |
| `page get --format adf` | Whole page as ADF JSON | For downstream `edit` chains; do not read into conversation context. |

---

## Page edits — section level

All section operations route through `page apply`. The fragment file must include the **heading line** itself (the operation finds and replaces by heading match).

| Operation | Use case | Notes |
|---|---|---|
| `--replace-section 'H' --fragment FILE` | Rewrite one section verbatim | Section bounds = heading + descendants until next equal-or-higher heading |
| `--insert-before 'H' --fragment FILE` | Add new section above an existing one | Anchor heading must exist |
| `--insert-after 'H' --fragment FILE` | Add new section below an existing one | Anchor heading must exist |
| `--append --fragment FILE` | Add new section at end of page | No anchor needed |
| `--delete-section 'H'` | Remove a section and its descendants | Same matching rules as replace |
| `--at-level N` | Disambiguate when heading text repeats at multiple depths | Combine with `--replace-section`, `--insert-*`, `--delete-section` |

---

## Page edits — table level

Use these for granular updates inside an existing table. The operations target the table inside a **named section** (passed as the first positional arg, e.g. `"Current Status"`). Row matching uses one of two modes — see the section "Row matching: `--match-cell` vs `--match-col`" right after the table.

| Operation | Use case | Constraint |
|---|---|---|
| `--table-add-row "Section H" --row "c1\|c2\|c3"` | Append a row | First row of the table determines schema (columns) |
| `--table-add-row ... --if-missing --match-cell "X"` | Idempotent insert (legacy: matches column 1) | Skips if a row with column-1 containing "X" already exists |
| `--table-add-row ... --if-missing --match-col "ICP" --match-value "X"` (v0.11+) | Idempotent insert by named column | Skips if the named column already contains "X" in any data row |
| `--table-remove-row "Section H" --match-cell "X"` | Remove row where column 1 contains X | Legacy mode (first-column match) |
| `--table-remove-row "Section H" --match-col "ICP" --match-value "X"` (v0.11+) | Remove row by named column | Header-based match |
| `--table-update-cell "Section H" --match-cell "X" --col-name "Score" --value "8.4"` | Change one cell, finding the row by column 1 | Surgical; rest of the table preserved |
| `--table-update-cell "Section H" --match-col "ICP" --match-value "X" --col-name "Score" --value "8.4"` (v0.11+) | Change one cell, finding the row by named column | Surgical; works when column 1 isn't unique |
| `--table-update-row "Section H" --match-cell "X" --row "c1\|c2\|c3"` | Replace whole row, finding it by column 1 | All cells overwritten in one shot |
| `--table-update-row "Section H" --match-col "ICP" --match-value "X" --row "c1\|c2\|c3"` (v0.11+) | Replace whole row, finding it by named column | Works for non-unique column 1 |
| `--table-move-row "Section H" --match-cell "X" --position N` (v0.11+) | Move a row to a different position | Position is 1-indexed across data rows (header at row 0 is never moved); out-of-range positions clamp to the boundary |
| `--table-move-row "Section H" --match-col "Score" --match-value "X" --position N` (v0.11+) | Same, matching by named column | Combine with `--match-col` when column 1 isn't unique |

### Row matching: `--match-cell` vs `--match-col`

Two modes, mutually exclusive:

- **`--match-cell VALUE`** (legacy, backward compatible) — matches `VALUE` against the **first column** of each row. Substring (`Contains`) match. The header row is included in the search for `--table-remove-row` and `--table-update-row` (long-standing quirk, preserved for compatibility); excluded for `--table-update-cell`.
- **`--match-col COL_NAME --match-value VALUE`** (v0.11+) — matches `VALUE` against the column whose **header** contains `COL_NAME`. Both flags must be passed together. Header row is always skipped. Mutually exclusive with `--match-cell`.

The legacy "first column only" gotcha is still relevant when you read older docs or run an older CLI — keep the next section in mind.

### Legacy gotcha: pre-v0.11 `--match-cell` ALWAYS matches the first column

When using `--match-cell` (or running a CLI older than v0.11), the match logic ignores anything past column 1. This is the gotcha most likely to bite during long edit sessions.

**Failure mode:**
```
operation failed: no row with first cell containing "X" found in table
```

The error appears even when "X" exists in some other column of that table. Match logic ignores anything past column 1.

### Solution (v0.11+): `--match-col` / `--match-value`

Since v0.11.0 the four table operations accept `--match-col COL_NAME --match-value VAL` as an alternative to `--match-cell`. The match runs against the named **column** (resolved by header text in the first table row), regardless of position:

```bash
# Table where column 1 is rank (1, 2, 3 — non-unique across tables)
# and column 2 is the ICP name.
confluence-docs page apply --page-id <id> \
  --table-update-cell "Reavaliação dos ICPs" \
  --match-col "ICP" --match-value "Lash designer" \
  --col-name "Score" --value "2.4"
```

Rules:

- `--match-cell` and `--match-col` are **mutually exclusive** — passing both is an error.
- `--match-col` and `--match-value` **must be used together** — passing only one is an error.
- Column matching uses **substring `Contains`** (consistent with the legacy `--match-cell` behaviour). So `--match-value "Lash"` will match `"Lash designer"`. Be specific enough to avoid false positives.
- When `--match-col` references a column that doesn't exist in the table, the error lists **available headers** so you can correct the name.
- The header row (row 0) is always skipped in `--match-col` mode. Legacy `--match-cell` keeps its quirk of searching all rows (header included) for backward compatibility.

When to still reach for `--replace-section`:

1. **Many rows change at once** — chaining N `--table-update-*` calls is more expensive than rewriting the section fragment.
2. **Schema changes** — adding/removing/reordering columns. `--table-*` operations don't change schema.
3. **No column is unique** — even with `--match-col`, you need a unique value per row in the target column. If no column has unique values, rewrite the section.

### Pre-v0.11 workarounds (if you're stuck on an older CLI)

If you must support an older CLI version:

1. **Surgical: `--table-update-cell` with `--match-cell`** — still matches column 1, but at least the change is column-scoped via `--col-name`.
2. **Bulk: `--replace-section`** — rewrite the whole section with a fragment file.
3. **Refactor at design time** — when authoring a new page, put a unique key as **column 1** so future updates compose.

---

## Decision: table-* vs replace-section

Pick the cheapest operation that does the job. Assumes v0.11+ CLI.

| Situation | Recommended op | Why |
|---|---|---|
| Add 1 new row | `--table-add-row` | Atomic; idempotent via `--if-missing` |
| Remove 1 row (column 1 unique) | `--table-remove-row --match-cell` | Direct |
| Remove 1 row (column 1 NOT unique) | `--table-remove-row --match-col --match-value` | Header-based match |
| Change 1 cell (column 1 unique) | `--table-update-cell --match-cell` | Smallest diff |
| Change 1 cell (column 1 NOT unique) | `--table-update-cell --match-col --match-value` | Header-based match |
| Change multiple cells on 1 row | `--table-update-row` (`--match-cell` or `--match-col`) | Whole row swap, atomic |
| Change multiple rows | `--replace-section` rewriting the section | Cheaper than chaining N `--table-update-row` calls |
| Move a row to a different position | `--table-move-row --position N` | Surgical reorder; preserves the rest of the table byte-for-byte |
| Sort all rows by a column | Chain `--table-move-row` per row, or `--replace-section` if many rows change | No single-shot sort op yet — could be a future `--table-sort` |
| No column has unique values | `--replace-section` | Match flags can't disambiguate without uniqueness |
| Add / remove / reorder columns | `--replace-section` | `--table-*` ops cannot change the schema |
| Add a brand-new table | `--replace-section` with new markdown | New tables come from a full section rewrite |

---

## Structural operations (move, reorder, delete)

These mutate page hierarchy or rename — body is untouched.

| Operation | Use case | Constraint |
|---|---|---|
| `page move --title 'T'` | Rename only | Body preserved byte-for-byte (macros intact) |
| `page move --parent-id NEW` | Reparent only | Same |
| `page move --title 'T' --parent-id NEW` | Rename + reparent in one PUT | Same |
| `page reorder --before SIBLING_ID` | Sort among siblings | Sibling must share the same parent |
| `page reorder --after SIBLING_ID` | Sort among siblings | Same |
| `page reorder --append-to PARENT_ID` | Append as last child (may reparent) | Lands at last position |
| `page delete --yes` | Soft delete (moves to Confluence trash; restorable) | **ALWAYS confirm with user before issuing** — even reversible deletes deserve authorization |

---

## Common failure modes

| Symptom | Root cause | Fix |
|---|---|---|
| `section not found: "..."` | Bash expanded `$` in section name | Wrap section name in single quotes: `--replace-section '$200 calc'` |
| `section not found` despite correct title | Heading text has a trailing space or non-breaking space | `page digest` to copy the exact heading; check whitespace |
| `no row with first cell containing "X"` | Column 1 doesn't match X (case-sensitive) OR X lives in a different column | Verify column 1 via `page get --section 'H'`; if X is elsewhere, use `--replace-section` |
| `409 conflict` | Page was edited between GET and PUT | `page apply` auto-retries once; if persistent, refetch and reapply |
| `200 OK with no version bump` | Fragment is byte-identical to existing content | Likely a logic error in the caller (re-applying the same content) |
| Created child page shows duplicated title prefix in sidebar tree (e.g. parent "ICPs", child "ICP — ...") | Title duplicates parent context | See "Title patterns for child pages" below |

---

## Title patterns for child pages

A child page lives **under** a parent in the Confluence tree. The sidebar shows both: `Parent ▸ Child`. Repeating the parent's prefix in the child title wastes the limited visual space of the sidebar tree and creates redundancy when the user navigates.

### Rule

**Don't duplicate parent context in the child title.**

| Parent | ❌ Bad child title | ✅ Good child title |
|---|---|---|
| `ICPs + Validation Plan` | `ICP — Personal trainer autonomous solo` | `Personal trainer autonomous solo` |
| `ICPs + Validation Plan` | `ICP — Nutritionist autonomous` | `Nutritionist autonomous` |
| `Decisions` | `Decision: Payment Provider Selection` | `Payment Provider Selection` |
| `Partners` | `Partner: Acme Corp` | `Acme Corp` |
| `Competitors` | `Competitor — Stripe Brazil` | `Stripe Brazil` |

### Why this matters

- The Confluence sidebar truncates long titles. Every redundant character costs visibility.
- A reader scanning the tree reads `Parent > Child` together — the type label is already implicit from the parent.
- Search results show the title alone, but the parent path is also surfaced in most clients; redundancy persists.

### Exception: slug vs visible title

The **slug** (frontmatter `slug`, Git filename, value used in `related` cross-links) **keeps** the type prefix because slugs need to be globally unique and unambiguous:

- Slug: `decision-payment-provider-selection`
- Visible page title: `Payment Provider Selection`

Slugs and titles do different jobs. Drop the prefix only from the **visible title**, not from the slug.

### When to break the rule

Rare cases where the prefix carries **information not derivable from the parent**:

- A page that lives in a generic parent (e.g. `Research`) but is a specific type (e.g. `Competitor: Stripe`) — the prefix helps scan.
- A page that may move parents over time and needs to keep its identity in the title (e.g. an ADR that survives reorgs).

When in doubt, drop the prefix. You can always add it later if discovery suffers.
