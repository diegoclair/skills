# Changelog — confluence-docs

## v0.14.0 (2026-05-14) — update uses prefix-filter API (future-proof)

Minor bump — no end-user CLI flag changes. `confluence-docs update` and `update --check` now resolve the latest release via the GitHub Releases API filtered by tag prefix, replacing the previous redirect-follow approach.

### Why the migration

`/releases/latest` is a singleton per repo — GitHub's redirect points at whichever release has `make_latest: true`. In a monorepo with multiple products (`confluence-v*`, `jira-v*`, and potentially more), only one product can claim that pointer at a time. A third skill claiming `make_latest: true` would silently break `confluence-docs update`, redirecting it to the wrong product's tag.

The fix is what every monorepo with this shape does: query `GET /repos/<owner>/<repo>/releases`, filter client-side by tag prefix, take the first match (GitHub returns newest-first by default). This is now the shared `pkg/atlassian/release` package, also used by the sibling `jira-tickets` skill — one implementation to maintain, two skills covered.

### What changed mechanically

- `cmd_update.go`: removed local `resolveLatestVersion` and `normalizeVersion` functions entirely. The resolution call is now `release.FindLatestByPrefix("diegoclair/skills", "confluence-v", nil)` and normalization uses `release.NormalizeVersion`.
- `main_test.go`: deleted `TestNormalizeVersion` — equivalent tests already exist in `pkg/atlassian/release/release_test.go`.
- Import `"github.com/diegoclair/skills/pkg/atlassian/release"` added; unused `net/http`, `strings`, and `time` imports removed from `cmd_update.go`.

### What was preserved

Same exit codes (0 / 10 / 3), same flags (`--check`, `-h`, `--help`), same human-readable output strings, same installer shell-out logic on Linux/macOS/Windows. Fully backward-compatible — the change is entirely under the hood.

### Migration notes

`confluence-docs update` brings v0.14.0 in. Nothing else to do.

---

## v0.13.0 (2026-05-14) — tag prefix `confluence-v*` + shared atlassian credentials

Minor bump — no end-user CLI flag changes, but the release pipeline and credentials store both move. Triggered by the work that prepares the repo for the sibling `jira-tickets` skill.

### Tag prefix migration: `v*` → `confluence-v*`

Until v0.12.2 the release workflow fired on any `v*` tag, which was fine while `confluence-docs` was the only skill in the repo. Now `jira-tickets` ships from the same monorepo and needs its own release pipeline triggered by `jira-v*`. To keep them disjoint:

- `.github/workflows/release.yml` → renamed to `release-confluence.yml`, trigger changed to `confluence-v*`
- `.github/workflows/release-jira.yml` → new, trigger `jira-v*`

The release-confluence workflow now strips the `confluence-` prefix before passing `VERSION` to `make build-all`, so the binary still reports `v0.13.0` (not `confluence-v0.13.0`). The GitHub release page itself shows the full prefixed tag.

### `normalizeVersion` handles the new prefix

`confluence-docs update --check` compares the installed binary version against the tag the GitHub redirect points at (`/releases/tag/<tag>`). After this release that tag is `confluence-v0.13.0`. Updated `normalizeVersion` in `cmd_update.go` to strip any leading `<prefix>-v` so equality works:

```
v0.13.0            → 0.13.0
confluence-v0.13.0 → 0.13.0   (was: "confluence-0.13.0" — false mismatch)
jira-v0.1.0        → 0.1.0    (futureproofing)
```

### Shared atlassian credentials at `~/.config/atlassian/credentials`

The Atlassian API token authenticates the user, not the product. Installing the upcoming `jira-tickets` skill on a machine that already has `confluence-docs` configured should NOT require pasting the same token a second time. Implemented in `pkg/atlassian/setup`:

```
Before:  <UserConfigDir>/confluence-docs/credentials  (per-skill)
After:   <UserConfigDir>/atlassian/credentials        (shared)
```

Reads fall back through the legacy per-skill paths automatically with a one-line warning suggesting `confluence-docs setup` to migrate. Writes always go to the new path. Per-skill non-secret config (active space, home_page_id) stays scoped to `<UserConfigDir>/<skill>/config` — those values genuinely differ between skills.

`pkg/atlassian/setup.SetSkillName("confluence-docs")` is now called explicitly from `main.go` in both skills' `init()`, so neither relies on a hidden default.

### `pkg/atlassian/jira/` skeleton

A new package — `github.com/diegoclair/skills/pkg/atlassian/jira` — ships with the `Client` type, `NewClient` constructor, and base-URL helpers, but no actual REST calls wired yet. Lets the upcoming `jira-tickets` command files (search, issue digest, transition, comment) be written against a stable contract while the implementation lands across v0.2–v0.5 of jira-tickets.

The package is in `pkg/atlassian/` because `confluence-docs` will share the same auth/retry primitives once a deeper refactor extracts the Confluence-specific bits out of `pkg/atlassian/adf/` into a `pkg/atlassian/confluence/` subpackage. That refactor was attempted in batch 2 of this session but deferred — `storage_convert.go` holds a `*ConfluenceClient` parameter that would create a circular import without an interface extraction step. Will land in a separate release.

### Migration notes

`confluence-docs update` brings v0.13.0 in. Credentials at the legacy `<UserConfigDir>/confluence-docs/credentials` path keep working (fallback chain handles them). Re-running `confluence-docs setup` once writes them to the new shared path and silences the legacy-path warning.

---

## v0.12.2 (2026-05-14) — `update` follows multi-hop redirect chains

Single-fix release. `confluence-docs update` and `confluence-docs update --check` now correctly resolve the latest release tag even when GitHub returns multiple redirects (e.g. right after a repo transfer, when `lybel-app/skills/releases/latest` first redirects to `diegoclair/skills/releases/latest`, then resolves to `diegoclair/skills/releases/tag/v0.12.1`).

### The bug

`resolveLatestVersion` in `cmd_update.go` set `CheckRedirect: http.ErrUseLastResponse` and read only the **first** `Location` header. Before the transfer this worked because GitHub answered with a single redirect straight to `/releases/tag/<tag>`. After the transfer there are two hops, and only the first one (the owner rename) ever made it into the parser — which then errored out with:

```
could not resolve latest version: unexpected Location format: https://github.com/diegoclair/skills/releases/latest
```

The v0.11.x and v0.12.0/v0.12.1 binaries that users already have installed pointing at `lybel-app/skills` are stuck on this error until they get the new binary by some other path. `install.sh` was always immune (it shells `curl -fsSL` which follows the full chain by default), so the rescue path is to re-run the install one-liner.

### The fix

Let the Go HTTP client follow redirects (default cap is 10) and inspect `resp.Request.URL` — the URL after the last hop, regardless of how many it took. If that URL contains `/tag/<tag>` we're done; otherwise emit a clear error.

`install.sh` and `install.ps1` already worked this way, so the binary is now consistent with them.

### Rescue path for installs stuck on v0.11.x/v0.12.0/v0.12.1

Run the install one-liner once (this bypasses `update` entirely):

```bash
curl -fsSL https://raw.githubusercontent.com/diegoclair/skills/main/confluence-docs/install/install.sh | bash
```

After that, `confluence-docs update` resumes working because the new binary points at `diegoclair/skills` natively (no transfer redirect needed) **and** carries this fix.

### Migration notes

None for fresh installs. For existing installs, see "Rescue path" above.

---

## v0.12.1 (2026-05-14) — monorepo refactor + repo moved to `diegoclair/skills`

Combines two structural changes; **zero behavior change for end users**. The `confluence-docs` binary, CLI flags, skill contract, and reference files are byte-for-byte identical to v0.11.3.

(v0.12.0 was prepared internally with the monorepo refactor but never published — the repo move happened in the same session and both ship together as v0.12.1 to keep the public history clean.)

### Repo moved: `lybel-app/skills` → `diegoclair/skills`

GitHub transferred ownership of the repo. All URLs, Go module paths (`github.com/diegoclair/skills/...`), install-script defaults (`CONFLUENCE_DOCS_REPO=diegoclair/skills`), README badges, install-for-ai runbook, and CI workflow now point at the new location. GitHub redirects keep the old `lybel-app/skills` URLs alive, so existing `confluence-docs update` installs continue to find releases — but new clones and Go imports should resolve directly to the new path.

### Monorepo refactor: shared `pkg/atlassian` module

Repository-level refactor preparing the codebase for a sibling **`jira-tickets`** skill (separate binary, same Atlassian token, same ADF format, same packaging convention).

### What changed (under the hood)

The 28 ADF parsing/rendering files and the 2 credentials-setup files were the obvious shared layer: Jira issues and Confluence pages both encode bodies/descriptions as ADF, and both authenticate against the same Atlassian token. Extracting them now avoids duplicating ~3,000 lines of `adf/` + ~1,700 lines of `setup/` once the Jira skill lands.

New layout:

```
skills/                              (repo root)
├── go.work                          NEW — committed; ties the modules together
├── pkg/atlassian/                   NEW shared module
│   ├── go.mod
│   ├── adf/                         moved from confluence-docs/cli/adf/
│   └── setup/                       moved from confluence-docs/cli/setup/
└── confluence-docs/
    └── cli/
        ├── go.mod                   adds replace + require for the local lib
        └── *.go                     21 imports updated to the new path
```

### Resolution mechanism — `replace` directive + `go.work`

`confluence-docs/cli/go.mod` ends with:

```
require github.com/diegoclair/skills/pkg/atlassian v0.0.0-...
replace github.com/diegoclair/skills/pkg/atlassian => ../../pkg/atlassian
```

`go.work` at the repo root lists `pkg/atlassian` and `confluence-docs/cli`. The replace lets `go build` and `make` work inside `confluence-docs/cli/` on its own (CI keeps working unchanged); the workspace gives IDEs/LSPs the same view contributors get. When `pkg/atlassian` eventually publishes its own semver-tagged releases, the replace goes away and the require pins a real version.

`go.work` and `go.work.sum` are committed — the default Go `.gitignore` template ignores them because that's the right call for single-module repos; for this multi-module monorepo, committing is the convention so contributors don't have to run `go work init` after cloning.

### Why a minor bump (0.12.0) and not a patch

End users see nothing. Contributors do — the import paths moved, there are now three `go.mod` files (root workspace + 2 module roots), and the directory layout signals "this is a monorepo holding multiple skills." That's the kind of contract change semver minor bumps exist for.

### What this unlocks

- **`jira-tickets` skill** can now drop into `jira-tickets/cli/` and reuse `pkg/atlassian/adf/` + `pkg/atlassian/setup/` without duplication.
- **Per-skill release pipelines**: `release.yml` continues to tag and ship `confluence-docs` on `v*` tags; when `jira-tickets` arrives, a sibling workflow will fire on a different tag prefix. (Tag convention migration to `confluence-v*` / `jira-v*` will happen alongside the first Jira release.)
- **Shared credential store**: a future patch can migrate `~/.config/confluence-docs/credentials` to `~/.config/atlassian/credentials` so installing the Jira skill doesn't require re-pasting the same API token.

### Verification

- `pkg/atlassian/adf`: `go build` + `go test -count=1 ./...` ✅
- `pkg/atlassian/setup`: `go build` + `go test -count=1 ./...` ✅
- `confluence-docs/cli`: `go build` + `go test -count=1 ./...` ✅
- `make build` inside `confluence-docs/cli/` produces a working binary (replace directive resolves locally even outside the workspace)
- `release.yml` CI path unchanged — `make build-all` from inside `confluence-docs/cli/` still works because of the same replace directive

### Migration notes

`confluence-docs update` brings the v0.12.0 binary; SKILL.md and reference files are refreshed in place. Nothing to do. Contributors who already cloned the repo: `git pull && go work sync && (cd pkg/atlassian && go mod tidy) && (cd confluence-docs/cli && go mod tidy)`.

---

## v0.11.3 (2026-05-14) — `page reorder --dry-run`, `--table-move-row` in help, CI bump

### `page reorder --dry-run`

`page reorder` is the verb agents use to reposition a page among its siblings (`--before`/`--after`/`--append-to`). Without a dry-run, an agent that misreads the intended position can move the page on the first try and surprise the user. Real cases: user reorders a tree manually, asks the agent to add a new sibling near the top, the agent ends up appending it to the bottom; or the user asks "second-most-important" and the agent reaches for `--append-to`, landing it at position six.

This release adds `--dry-run` to `page reorder`. It prints the intended JSON action and exits 0 without making the API call:

```bash
$ confluence-docs page reorder --page-id 100 --after 200 --dry-run
{"status":"dry-run","pageId":"100","position":"after","targetId":"200"}
```

Implementation notes:

- `--dry-run` also **skips credential resolution** (`buildClient` is bypassed when `dryRun == true`). The whole point of dry-run is intent preview, and the output is derivable from the flags alone — the API doesn't add information. This mirrors the `index --input` policy from v0.11.1.
- The existing test `TestPageReorder_DryRun_NoHTTPCall` was updated: it previously documented the "rejected as unknown flag" behaviour and now asserts the implemented behaviour (exit 0, JSON output with `status:"dry-run"`, zero HTTP calls).
- Help text in both `cmd_page_reorder.go --help` and the top-level `--help` was updated.

### `--table-move-row` added to top-level `--help`

The flag was implemented in v0.11.0 but never made it into the `helpText` constant — agents reading `confluence-docs --help` had no way to learn about it. Now appears in three places: EDIT OPERATIONS, EDIT FLAGS (`--position N`), and the `page apply` operations list.

### CI: GitHub Actions versions bumped to Node-24-compatible

`actions/checkout@v4 → v6`, `actions/setup-go@v5 → v6`, `softprops/action-gh-release@v2 → v3`. The previous versions were running on Node.js 20, which GitHub announced would be force-migrated to Node 24 in June 2026 and removed in September 2026. Moving now avoids the warning and the future breakage.

### Decisions parked

- **`lint` warning for unknown ADF node types**: discussed and skipped. Would require an allow-list of the ~35 canonical ADF node types from Atlassian's spec, which goes out of date as Atlassian adds new types (decisionItem, taskItem, mediaSingle, layoutSection variants, etc.). The current lenient behaviour — only validating `heading`, `table`, `bulletList`, `orderedList`, `text`, and link marks — is intentional and stays. If a real false-negative shows up in practice, revisit then.
- **Suporte a outros caminhos de instalação** (Cursor, Continue) além de `~/.claude/skills/`: parking até ter demanda concreta de usuário fora da audiência Claude Code / Desktop atual.

### Migration notes

`confluence-docs update` brings the new binary; SKILL.md and reference files are refreshed in place. Nothing else needs attention.

---

## v0.11.2 (2026-05-14) — install scripts drop stale `reference/` files

A single targeted fix to the install/update flow. No CLI or skill-content changes.

### Why

After upgrading from a release that shipped reference files like `aliases.md` / `taxonomy.md` / `templates.md` to a release that no longer ships them, the obsolete files would linger forever under `~/.claude/skills/confluence-docs/reference/`. The previous `install.sh` and `install.ps1` only **overwrote** files present in the new archive — they never **removed** files that had been retired between releases. The skill itself ignored them (the canonical reference list lives in `SKILL.md`), but the directory listing grew noisier with every cleanup release and could confuse anyone manually browsing the folder.

### Fix

Both installers now perform a clean-slate replace of the `reference/` directory:

1. Download + extract the release archive into a temp dir (unchanged).
2. Verify `reference/` exists in the extracted archive (guards against a broken release — if the archive lacks it, the old `reference/` is preserved).
3. **`rm -rf "$SKILL_DIR/reference"` / `Remove-Item -Recurse -Force $RefDir`** (new step).
4. Recreate the directory and copy the fresh `*.md` files from the extracted archive.

The binary install (atomic temp-rename to handle ETXTBSY on Linux) and the `SKILL.md` overwrite are unchanged. Credentials at `~/.config/confluence-docs/credentials` and the home cache at `~/.cache/confluence-docs/home.json` are untouched — both live outside `$SKILL_DIR`.

### Migration notes

After running `confluence-docs update` to v0.11.2, the install script will (one-time) clean any stale reference files left from older releases. Subsequent updates stay idempotent. No action needed from the user.

---

## v0.11.1 (2026-05-14) — repository refactor + test coverage

A housekeeping release with no behavioral changes. The CLI flags, skill contract, on-disk credentials, home cache, and reference file paths are all identical to v0.11.0. The goal was to make the codebase friendlier to outside contributors before opening for community feedback — and to lift test coverage on five files that previously had no dedicated unit tests.

### CLI source — split monolithic `main.go`

- **`main.go` reduced from 4,359 → 335 lines.** Keeps `main()`, the `run()` dispatcher, and the `helpText` constant; everything else moves out.
- **22 new files**, one per top-level subcommand or related helper set: `cmd_adf.go`, `cmd_edit.go`, `cmd_page.go` (dispatcher + shared helpers), `cmd_page_get.go`, `cmd_page_upload.go`, `cmd_page_create.go`, `cmd_page_apply.go`, `cmd_page_move.go`, `cmd_page_reorder.go`, `cmd_page_delete.go`, `cmd_page_digest.go`, `cmd_page_rewrite.go`, `cmd_page_children.go`, `cmd_search.go`, `cmd_home.go`, `cmd_lint.go`, `cmd_extract.go`, `cmd_index.go`, `cmd_update.go`. Two cross-cutting helper files: `markdown_helpers.go` (ATX heading parsing + `mdSection` type + ADF heading-text extraction) and `io_helpers.go` (`readInput`, `readADFInput`, `writeJSON`).
- All files stay in `package main` — no new sub-package, no export ceremony, no API breakage. The Go convention `cmd/` directory is reserved for multi-binary projects; for sub-commands of a single binary, flat-with-prefix is idiomatic.

### `cli/README.md` rewritten — single install flow

Previously the file maintained three audience-specific paths ("For Claude", "For humans", "For developers") that duplicated content with `reference/install-for-ai.md` and the root README. The new file (282 → 165 lines) leads with the install one-liner, then `confluence-docs setup` for credentials, then `update` for upgrades, followed by the command catalogue and the build-from-source instructions for contributors.

### Test coverage — five command files lifted from zero

Before this release the following files had no dedicated unit tests; only `main_test.go` exercised them indirectly:

- `cmd_home.go` (Confluence Home cache: `home --refresh/--status/--show/--query/--digest`)
- `cmd_index.go` (Page ID Index table manipulation: `index add/remove/sync`)
- `cmd_page_reorder.go` (sibling reordering via v1 endpoint)
- `cmd_page_rewrite.go` (markdown → multi-section diff against an existing page)
- `markdown_helpers.go` (ATX heading parsing + ADF heading-text traversal)

Five new test files now cover these, contributing **107 new `Test*` functions** and many more `t.Run` sub-cases. The IO-bound paths use the `http.DefaultTransport` RoundTripper mocking pattern shared with `cmd_space_test.go`; the pure-function paths use the table-driven pattern shared with `cmd_km_test.go`. Totals after this release:

- **207** `Test*` functions
- **461** `t.Run` sub-cases
- **381** individual `PASS` lines under `go test -v`
- `go build ./...`, `go test -count=1 ./...`: all green
- `go vet ./...` reports one pre-existing warning (`cmd_space_test.go:298` self-assignment), not regressed

### Skill repository — packaging cleanup

- **`LICENSE` duplicated into the skill directory** (`confluence-docs/LICENSE`). When the install script or future plugin host copies the skill into a user's `~/.claude/skills/confluence-docs/`, the licence file follows. The repository root keeps its own `LICENSE`.
- **`INSTALL_FOR_AI.md` moved** to `confluence-docs/reference/install-for-ai.md`. It's a runbook for AI agents performing the install — conceptually a reference doc. References in the root `README.md` updated.
- **Empty `.gitkeep` files removed** (`bin/`, `cli/`, `reference/`). The install script creates these directories via `mkdir -p`, so the placeholders served no purpose.
- **`version: 0.11.1`** added to the `SKILL.md` frontmatter (was absent before).

### Findings parked for follow-up (no fix in this release)

- **`page reorder --dry-run`** is documented in `--help` but not implemented — falls into the unknown-flag branch. A test now documents the current behaviour.
- **`lint` silently ignores ADF nodes of unknown type.** The implementation only validates `heading`, `table`, `bulletList`, `orderedList`, `text`, and link marks. By design today; worth deciding whether to add a `default:` warning later.

### Migration notes

No action needed. The on-disk binary, CLI flags, credentials, home cache, frontmatter contract, and reference file paths are all unchanged. `confluence-docs update` downloads the v0.11.1 release archive and overwrites the binary atomically; existing credentials and home cache are preserved.

---

## v0.11.0 (2026-05-13) — major skill refactor + CLI table-match by column

A focused pass to align the skill with Anthropic's official skill-creator best practices and to remove two real-world friction points observed in long edit sessions: (1) `--match-cell` silently failing when the first column wasn't unique, and (2) the lack of guidance on child-page titles that duplicate parent context. Both surfaced during a multi-hour session refactoring an "ICPs + Validation Plan" page tree (~10 child pages, multiple tables to edit).

### Skill — SKILL.md slimmed and progressive disclosure deepened

- **SKILL.md cut from 739 → 283 lines** to honor the official <500-line guideline. Daily-use procedures stay in the body; deep dives move to `reference/`.
- **`description` frontmatter rewritten** in third person and made deliberately pushy (per skill-creator best practices: Claude tends to under-trigger skills). New phrasing includes "use this skill whenever... even if they don't explicitly mention 'Confluence'". Also covers the project documentation aliases (wiki, kb, knowledge base, docs).
- **Three new reference files** with a "When to read" callout each:
  - `reference/editorial-patterns.md` — patterns 1–4 (header, Context→Problem→Solution, clarity for outside readers, no process meta-noise). Moved from SKILL.md body.
  - `reference/features.md` — full-width pages, `:::properties` macro, Smart Link embeds, `check`, `new`, `km` (Knowledge Map). Moved from SKILL.md body.
  - `reference/configuration.md` — credentials, space management, home cache lifecycle contract, CLI installation check + exit codes. Moved from SKILL.md body.
- **`reference/doc-types.md` gained** a Table of Contents (large reference file >300 lines, official skill-creator recommendation) and a new sub-section **"Child page titles — don't duplicate parent context"** with ❌/✅ examples for ICP / Decision / Partner / Competitor parents. Real-world driver: a session created child pages like "ICP — Personal trainer autonomous solo" under a parent already named "ICPs + Validation Plan"; the duplication wasted sidebar space and felt redundant.
- **`reference/workflows.md` gained** a TOC for the same reason.
- **`reference/operations-matrix.md`** (NEW) — CLI subcommand × constraint × fail mode × workaround. Centralizes the gotchas not visible from `--help`: `--match-cell` always matching the first column (mitigated this release by the new `--match-col` flag), shell `$` expansion in section names, when to use `--table-update-cell` vs `--table-update-row` vs `--replace-section`, and the new "Title patterns for child pages" rule from a CLI-operator angle.
- **SKILL.md body cross-references** the new reference files explicitly with "When to read" context (matching the skill-creator pattern of "tell Claude what's there and when to load it", not just `see X`).

### CLI — match by any column + row reorder

- **New flags `--match-col COL_NAME` and `--match-value VALUE`** on all four mutating table operations (`--table-update-cell`, `--table-update-row`, `--table-remove-row`, `--table-add-row` with `--if-missing`). When set, the row match runs against the named column (header text from the first table row) instead of column 1. Eliminates the long-standing limitation where tables with numeric rank or repeating IDs in column 1 couldn't be edited surgically.
- **New operation `--table-move-row "Heading" [match flags] --position N`** reorders a row inside its table without rewriting the section. Position is 1-indexed across data rows (the header at row 0 is never moved); out-of-range positions clamp to the boundary rather than erroring. Pairs with either `--match-cell` or `--match-col`/`--match-value`. Real-world use: after updating a row's score, move it to the position the new score deserves — surgical reorder instead of replacing the whole section.
- **`--match-cell` continues to work unchanged** (backward compatible — defaults to column-1 match). Mutually exclusive with `--match-col` / `--match-value` (validation rejects both at once).
- **Descriptive error** when `--match-col` references a column not found in the table: lists available headers in the error message, mirroring the `sectionNotFoundError` pattern.
- **Documentation updated** in `reference/operations-matrix.md` with examples and decision-matrix entries for all new flags.
- **Token-cost win in practice:** a real edit chain that previously required replacing an entire section (≈ 3 KB upload) now requires a single `--table-move-row` or `--table-update-cell` call (≈ 400 B upload + ≈ 150 B status response). Compared to going through the Atlassian MCP directly (which would fetch and re-upload the full ADF of the page — typically 25–40 KB), the savings are roughly 60–100× on bytes-in-conversation for fine-grained edits.

### Why this version is 0.11 and not 0.10.4

The SKILL.md refactor, the description rewrite (which changes the triggering surface), the new reference files, and the new CLI flags together amount to a structural release rather than a patch. The contracts (CLI flags, frontmatter description, reference file paths) change such that consumers of the skill should re-read it. No installed credentials or cached data are affected — install/upgrade is non-destructive.

### Migration notes

- **Agents that already loaded `SKILL.md`** in a long-lived session pick up the slimmer version on next session start; no action needed during the current session.
- **Custom scripts that called `--match-cell`** keep working unchanged. To opt into column-name matching, add `--match-col NAME --match-value VAL` instead of `--match-cell VAL`.
- **Sub-skill paths**: agents referencing files inside `reference/` should expect `editorial-patterns.md`, `features.md`, `configuration.md`, and `operations-matrix.md` to exist; the previous monolithic SKILL.md sections by the same names no longer carry the full content.

---

## v0.10.3 (2026-05-12) — shell `$` expansion hint applied uniformly

The friction note #11 ("Shell `$` in section names breaks silently") had been partially fixed in an earlier release: `sectionNotFoundError` in `adf/table_edit.go` embeds a helpful hint in the error message when a heading contains `$` but the user's input doesn't. That helper was only used by table operations and `DeleteSection`. The other section operations — `ReplaceSection`, `InsertAfter`, `InsertBefore`, `SectionContent` (used by `page get --section`) — still emitted a bare `section not found: %q` error, no heading list, no hint.

Consolidated: all five section operations in `adf/edit.go` now route through `sectionNotFoundError`, so users get the same error format and the same shell-expansion hint everywhere. Real-world case: `R$200 × 5 clientes = R$100k GMV` (where bash silently turns `$200` into `00` and `$100k` into `00k`) now produces:

```
section not found: "Cálculo de margem (R00 × 5 clientes = R00k GMV)"
Headings found in document:
  (h2) Cálculo de margem (R$200 × 5 clientes = R$100k GMV)

Hint: shell may have expanded variables in your section name.
  Received: "..."
  Heading:  "..."
  Wrap the section name in single quotes to prevent expansion, e.g. --replace-section '...'.
```

regardless of which operation triggered it. Removed the now-redundant inline `current top-level sections` blocks in `main.go` — the error itself carries everything needed.

## v0.10.2 (2026-05-12) — uniform setup wizard UX + space alias in setup flow

Two small UX fixes for the setup wizard after v0.10.1.

### Bug 1 — inconsistent prefill UX across fields

In v0.10.x the email and token prompts showed the current value inline (`Atlassian email: diego@example.com`) and skipped the input entirely, while the subdomain prompt asked "press Enter to keep 'lybel'" and accepted input. Confusing — the first two looked like display-only, the third like a normal prompt.

Unified all three to the same multi-line pattern:

```
Atlassian email
  current: diego@example.com
  new (press Enter to keep, or type a new value): _

API token
  current: ATAT****…
  new (press Enter to keep, or paste a new token): _

Confluence subdomain (e.g. 'mycompany' for mycompany.atlassian.net)
  current: lybel
  new (press Enter to keep, or type a new value): _
```

Press Enter → keeps. Type → overrides. Works the same for all fields. First-time setup (no existing values) shows the simpler single-line prompt for each.

### Bug 2 — setup wizard still showed the internal hash for spaces

v0.10.1 fixed `space list/current` to use `currentActiveAlias` instead of the internal `key` field, but the setup wizard's `fetchAccessibleSpaces` has its own inline JSON parser that still only read `key`. So during setup the space list looked like:

```
1. Lybel (f53b318e3ee044c49c76ddaae276f180, id 131352)
```

instead of `Lybel (lybel, id 131352)`. Mirrored the v0.10.1 fix into the setup parser.

## v0.10.1 (2026-05-12) — fix space key + setup token prefill

Two regressions from v0.10.0 reported by Diego right after release.

### Bug 1 — `space.key` was the internal hash instead of the human URL key

The v2 API `/wiki/api/v2/spaces` returns TWO distinct fields per space:

- `key` — an internal hex hash for non-personal spaces (e.g. `f53b318e3ee044c49c76ddaae276f180`)
- `currentActiveAlias` — the human-readable key used in URLs and CQL queries (e.g. `lybel`)

v0.10.0 persisted `key`, so `space list` / `space current` showed the internal hash. Operational impact was zero (Confluence accepted both forms internally), but the UX was confusing and `--space lybel` flag could in theory mismatch a stored hash.

Fix: `SpaceResult.Key` now sources from `currentActiveAlias` first, falling back to `key` (which is fine for personal spaces where both fields are identical with a `~` prefix).

Existing users on v0.10.0 with a stored hash: re-run `confluence-docs space use <human-key>` once to rewrite the config with the correct alias. v0.10.1+ wizard writes it correctly from the start.

### Bug 2 — `setup` wizard did not prefill an existing token

Running `confluence-docs setup` after a v0.9.x install (credentials file already had email + token) prompted for the token again instead of showing it pre-filled. v0.9.x correctly prefilled both fields and only asked for the new ones; the v0.10.0 refactor accidentally restricted prefill to `--reconfigure` only.

Fix: prefill applies on every interactive run, not just `--reconfigure`. The wizard masks the token, lets the user press Enter to keep, or paste a new one to override.

### Feedback noted

Saving to project memory: when subagent will code against an external API (Atlassian, GitHub, etc.) and credentials are available, validate the actual JSON response shape first (1 curl/Python request) and pass it as the contract to the subagent. Don't trust the doc alone. Would have caught Bug 1 in 30 seconds.

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
