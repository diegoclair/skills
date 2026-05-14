# confluence-docs

Confluence ADF toolkit and Claude skill. Edits Confluence Cloud pages without destroying macros; consolidates a knowledge base into a classified Knowledge Map; supports `:::properties`, smart links, page labels, and `@mentions`.

Designed for any team running a Confluence Cloud space alongside Claude (or any other LLM agent) that maintains docs daily.

---

## Install

**Linux / macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/diegoclair/skills/main/confluence-docs/install/install.sh | bash
```

**Windows:**

```powershell
iwr -useb https://raw.githubusercontent.com/diegoclair/skills/main/confluence-docs/install/install.ps1 | iex
```

The script downloads the latest release, places the binary at `~/.claude/skills/confluence-docs/bin/confluence-docs` (or `.exe` on Windows), and adds it to your `$PATH`.

**Open a new terminal** (or `source ~/.zshrc`) for the `$PATH` change to take effect.

Inspect the scripts before piping if you want: [`../install/install.sh`](../install/install.sh) · [`../install/install.ps1`](../install/install.ps1).

## Configure credentials

```bash
confluence-docs setup
```

Interactive wizard: asks for Atlassian email, API token, Confluence subdomain, then auto-detects accessible spaces. Generate an API token at https://id.atlassian.com/manage-profile/security/api-tokens.

Validate at any time:

```bash
confluence-docs setup --check
```

Exit code `0` means you're ready. For non-interactive / scripted setup (CI, AI agents), see [`SETUP.md`](SETUP.md).

## Update

```bash
confluence-docs update          # download + install latest release
confluence-docs update --check  # report current vs latest without upgrading
```

Credentials and home cache are preserved across updates.

---

## What it does

- **Drives Claude's Confluence skill** (`~/.claude/skills/confluence-docs/SKILL.md`) so the assistant can hit Confluence directly without paying the token cost of the Atlassian MCP. Most operations return sub-KB output instead of full ADF bodies — typically 10–50× cheaper across a multi-edit session.
- **Local cache of the Confluence Home** (`home` command) at `~/.cache/confluence-docs/home.json`. One `home --refresh` per session pulls the Home from Confluence; subsequent `home --query`, `--show`, `--digest` read from disk.
- **Slim page digests** (`page digest`) — title, version, heading outline, macros — in ~500 bytes. Replaces 10–40 KB ADF reads for "what's in this page?" questions.
- **Atomic page updates** (`page apply`): GET fresh ADF → apply section or table op → PUT, with automatic refetch-and-retry on 409 conflict. Full ADF never leaves the binary. Supports `--append`, `--insert-after/before`, `--replace-section`, `--delete-section`, `--table-add-row`, `--table-update-cell`, `--table-move-row`.
- **Page lifecycle** (`page move`, `page reorder`, `page delete`): rename, reparent, reorder siblings, soft-delete (restorable).
- **CQL search** (`search`) returning compact TSV instead of MCP's verbose JSON.
- **Macro-safe Markdown → ADF conversion**: `[TOC]`, `:::expand`, `:::warning`, `:::info`, `:::properties`, smart links (YouTube/Loom/GitHub/Linear), page labels, `@mentions` — all first-class.
- **Section-level edits without touching surrounding macros** — a property the Confluence MCP tool does not guarantee.
- **Direct REST API v2 calls**, bypassing MCP for large pages (>50 kB) where tool calls may time out.

## Commands

| Command | Description |
|---|---|
| `setup` | Interactive credential wizard. `--check` validates, `--print-config-path` shows the file path |
| `update` | Self-update to the latest release. `--check` reports current vs latest without upgrading |
| `adf` | Convert Markdown (+ Confluence macro extensions) to ADF JSON |
| `edit` | Apply a section-level or table-level operation to existing ADF without touching macros |
| `page get` | Fetch a Confluence page via HTTP. Formats: `adf`, `text`/`markdown`, `storage`, `view`/`html`, `export_view`. Slice with `--section "Heading" [--at-level N]` |
| `page digest` | Print a ~500-byte summary: title, version, status emoji, outline, macros |
| `page apply` | Atomic page update: GET → apply op → PUT with 409 retry. See list of operations above |
| `page upload` | Push a local ADF/Markdown file to an existing page |
| `page create` | Create a new page from markdown or ADF |
| `page move` | Rename and/or reparent a page (one PUT) |
| `page reorder` | Reposition among siblings or append under a different parent (v1 endpoint, body untouched) |
| `page delete` | Soft-delete a page (sends to Confluence trash, restorable). Requires `--yes` |
| `page children` | List direct children of a page (TSV: id, title) |
| `search` | CQL search via the v1 API. TSV output (`pageId\ttitle\turl\texcerpt`) |
| `home` | Local Home-page cache. `--refresh`, `--status`, `--show`, `--query "X"`, `--digest` |
| `lint` | Validate ADF structure and report errors/warnings |
| `extract-body` | Unwrap the ADF body from an MCP `getConfluencePage` response |
| `index` | Manage the Page ID Index table on your project's Home page |
| `check` | Detect duplicates before creating a page (trigram fuzzy match by title) |
| `new` | Generate a starter markdown template for one of the five canonical doc types |
| `km` | Knowledge Map regeneration from triage JSON batches |

Run `confluence-docs --help` or `confluence-docs <command> --help` for full flag documentation.

**Exit codes** — `adf`/`edit`/`lint`/`extract-body`: `0` success, `1` parse error, `2` invalid input, `3` HTTP/unknown error. `setup --check`: `0` valid, `1` missing, `2` invalid, `3` network error.

---

## Build from source

Requires Go 1.26.2+. No other runtime dependencies.

```bash
make build     # builds bin/confluence-docs
make install   # builds and copies to ~/.claude/skills/confluence-docs/bin/confluence-docs
               # (override with INSTALL_DIR=/custom/path)
```

## Repository layout

This is the **CLI source dir** (`confluence-docs/cli/`). The skill payload lives one level up at `confluence-docs/` (SKILL.md + reference/), and the end-user install scripts live at `confluence-docs/install/`.

```
confluence-docs/
├── SKILL.md                Skill entrypoint (read by Claude at runtime)
├── reference/              Skill reference docs (templates, taxonomy, workflows)
├── install/
│   ├── install.sh          POSIX install script (Linux / macOS)
│   └── install.ps1         Windows install script
└── cli/                    ← you are here
    ├── README.md           This file
    ├── SETUP.md            Non-interactive credential setup (CI, AI agents)
    ├── sample.md           Markdown fixture for `confluence-docs adf < sample.md`
    ├── Makefile            build / build-all / test / install
    ├── main.go             CLI entry: main() + run() dispatcher
    ├── cmd_*.go            One file per top-level subcommand (page, search, home, etc.)
    ├── main_test.go        CLI integration tests (via run())
    ├── go.mod / go.sum
    ├── setup/              Credential wizard package (confluence-docs setup)
    └── adf/
        ├── builder.go      ADF node + mark types and constructor helpers
        ├── converter.go    goldmark AST → ADF walker
        ├── macros.go       Pre-processing for [TOC] and ::: container blocks
        ├── edit.go         Section-level edit ops (append/insert/replace/delete)
        ├── table_edit.go   Table-level ops + --at-level support
        ├── confluence.go   REST API v2 HTTP client + creds + CQL search + 409 detection
        ├── digest.go       Slim page-summary builder (heading outline, macros, words)
        ├── render.go       ADF → markdown-ish plain text (used by home cache)
        ├── cache.go        HomeCache type + load/save
        ├── lint.go         ADF structure validator
        └── *_test.go       Tests
```
