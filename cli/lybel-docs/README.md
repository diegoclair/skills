# lybel-docs

Confluence ADF toolkit and skill for Claude. Edits Confluence pages without
destroying macros, and drives the Lybel knowledge base.

---

## For Claude (one-shot install)

> **If a user pasted this URL in a Claude conversation and asked you to install
> the skill (e.g. "Claude, instala essa skill"), follow these exact steps. Run
> every command yourself via the Bash tool — do NOT ask the user to open a
> terminal.**

### Step 1 — Detect OS

Read the `Platform:` field in your environment context.

### Step 2 — Run the install script

**Linux / macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/lybel-app/skills/main/cli/lybel-docs/install/install.sh | bash
```

**Windows:**

```powershell
iwr -useb https://raw.githubusercontent.com/lybel-app/skills/main/cli/lybel-docs/install/install.ps1 | iex
```

The script downloads the correct binary, places it at
`~/.claude/skills/lybel-docs/bin/lybel-docs` (or `.exe` on Windows), and also
fetches the SKILL.md and reference files. See
[`install/README.md`](install/README.md) for exactly what the scripts do.

### Step 3 — Verify the binary

```bash
lybel-docs --version
```

Expected output: `lybel-docs v<x.y.z>`. If the command is not found, check
that `~/.claude/skills/lybel-docs/bin/` is on `$PATH` (the install script
handles this, but a new shell session may be needed).

### Step 4 — Check credentials

```bash
lybel-docs setup --check
```

Route by exit code:

| Exit code | Meaning | What to do |
|---|---|---|
| `0` | Credentials valid | Confirm to user and stop. Setup is complete. |
| `1` | No credentials file | Continue to Step 5 (first-time setup). |
| `2` | Credentials present but invalid | Tell the user the token may have been revoked or mistyped; ask if they want to redo setup. If yes, continue to Step 5. |
| `3` | Network error | Retry once. If still failing, surface the error message to the user verbatim. |

### Step 5 — First-time setup (if exit 1 or 2)

Conduct entirely in chat — no terminal commands needed from the user.

1. Tell the user (in their language; for Lybel team, use Portuguese):

   > "Preciso de um token do Atlassian pra conectar ao Confluence. Abre essa
   > URL no navegador, clica em **Create API token**, dá um nome (ex:
   > `lybel-docs`), copia o token e me cola aqui. Também me diz o teu email
   > do Atlassian."
   >
   > URL: `https://id.atlassian.com/manage-profile/security/api-tokens`

2. Wait for the user to paste their **email** and **token** in chat.

3. Get the OS-correct config file path:

   ```bash
   lybel-docs setup --print-config-path
   ```

4. Get the exact file format:

   ```bash
   lybel-docs setup --print-config-format
   ```

5. Use the `Write` tool to create the file at the printed path with the
   content:

   ```
   email=<user's email>
   token=<user's token>
   ```

   (Use the exact key names and format returned by `--print-config-format`.
   The Write tool will create parent directories automatically.)

6. On **Linux / macOS**, tighten file permissions:

   ```bash
   chmod 600 <path from step 3>
   ```

   On **Windows**, the file inherits user-only ACL — no extra action needed.

7. Validate:

   ```bash
   lybel-docs setup --check
   ```

   Must exit `0`. If it still fails (exit `2`), tell the user:
   - "O token que guardei começa com `<first 5 chars>` e tem `<length>`
     caracteres — parece correto? Se quiser, revoga o token atual e cria um
     novo em `https://id.atlassian.com/manage-profile/security/api-tokens`."
   - Do not loop more than twice.
   - **Never echo the full token back.**

8. Confirm to the user:

   > "Pronto — conectado ao Confluence como `<email>`."

   Then continue with whatever task they originally asked for.

---

## For humans

**lybel-docs** is a command-line tool that:

- Drives Claude's Confluence skill (`~/.claude/skills/lybel-docs/SKILL.md`) so
  the assistant can hit Confluence directly without paying the token cost of
  the Atlassian MCP. Most operations return sub-KB output instead of full ADF
  bodies — typically 10–50× cheaper across a multi-edit session.
- Maintains a **local cache of the Confluence Home** (`home` command) at
  `~/.cache/lybel-docs/home.json`. One `home --refresh` per session pulls the
  Home from Confluence; every subsequent `home --query`, `--show`, `--digest`
  reads from disk — zero API calls. Writes always GET fresh ADF before PUT,
  so the cache is never the source for an update (avoids overwriting work
  done on another machine).
- Produces a **slim digest** of any page (`page digest`) — title, version,
  heading outline, macros — in ~500 bytes. Replaces 10–40 KB ADF reads for
  the common "what's in this page?" question.
- **Atomic page updates** (`page apply`): GET fresh ADF → apply section or
  table op → PUT, with automatic refetch-and-retry on 409 (when someone else
  updated mid-flight). The full ADF never leaves the binary — the caller
  only sees a tiny status line. Supports `--append`, `--insert-after/before`,
  `--replace-section`, `--delete-section`, `--table-add-row`,
  `--table-remove-row`.
- **CQL search** (`search`) returning compact TSV (id, title, url, excerpt)
  instead of MCP's verbose JSON.
- Converts extended Markdown to Atlassian Document Format (ADF) JSON, including
  Confluence macros (`[TOC]`, `:::expand`, `:::warning`, etc.).
- Applies section-level edits to existing ADF pages **without touching macros**
  outside the edited section — a property the Confluence MCP tool does not
  guarantee.
- Talks directly to the Confluence Cloud REST API v2, bypassing MCP for large
  pages (>50 kB) where tool calls may time out.
- Manages the Page ID Index table on the Lybel Home page.
- Validates ADF structure and reports errors.

### Quick install

**Linux / macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/lybel-app/skills/main/cli/lybel-docs/install/install.sh | bash
```

**Windows:**

```powershell
iwr -useb https://raw.githubusercontent.com/lybel-app/skills/main/cli/lybel-docs/install/install.ps1 | iex
```

After the script finishes, configure your credentials:

```bash
lybel-docs setup
```

Or ask Claude to do it for you — paste this page's URL in a Claude conversation
and say "instala essa skill".

### Manual install (build from source)

Requires Go 1.21+. No other runtime dependencies.

```bash
make build     # builds bin/lybel-docs
make install   # builds and copies to ../../skills/lybel-docs/bin/lybel-docs
```

### Commands

| Command | Description |
|---|---|
| `setup` | Interactive credential wizard; use `--check` to validate, `--print-config-path` to see where the file lives |
| `update` | Self-update to the latest release. `--check` reports current vs latest without upgrading. Credentials and home cache are preserved |
| `adf` | Convert Markdown (+ Confluence macro extensions) to ADF JSON |
| `edit` | Apply a section-level or table-level operation to existing ADF without touching macros |
| `page get` | Fetch a Confluence page via HTTP (bypasses MCP). Formats: `adf`, `text`/`markdown` (local render of ADF), `storage`, `view`/`html`, `export_view`. Slice with `--section "Heading" [--at-level N]` to get just one section. `--quiet` suppresses the "wrote N bytes" stderr line |
| `page digest` | Print a slim summary (~500 bytes) of a page: title, version, **status** (parsed from leading 🟢🟡🟠🔴🔵⚪✅ emoji in title), headings outline, macros. Designed to answer "what's in this page?" / "qual o status de X?" without a full ADF round-trip |
| `page apply` | Atomic page update: GET ADF → apply op → PUT, with automatic 409-conflict retry. Operations: `--append`, `--insert-after`, `--insert-before`, `--replace-section`, `--delete-section`, `--table-add-row`, `--table-remove-row` |
| `page upload` | Push a local ADF file to an existing page |
| `page create` | Create a new page (markdown or ADF source) |
| `page children` | List direct children of a page (TSV: id, title). Old name `list-children` still works as alias |
| `search` | CQL search via the v1 API. TSV output (`pageId\ttitle\turl\texcerpt`). Defaults to `space="lybel" AND type="page"` |
| `home` | Local Home-page cache. Verbs: `--refresh` (force GET + cache), `--status` (metadata), `--show` (text), `--query "X"` (grep), `--digest`. Cache at `~/.cache/lybel-docs/home.json`. Read-only — writes always GET fresh ADF first |
| `lint` | Validate ADF structure and report errors/warnings |
| `extract-body` | Unwrap the ADF body from an MCP `getConfluencePage` response |
| `index` | Manage the Page ID Index table on the Lybel Home page |

Run `lybel-docs --help` or `lybel-docs <command> --help` for full flag
documentation.

Exit codes for `adf`/`edit`/`lint`/`extract-body`: `0` success, `1` parse
error, `2` invalid input, `3` unknown/HTTP error.

Exit codes for `setup --check`: `0` valid, `1` missing, `2` invalid, `3`
network error.

### Documentation

| File | Purpose |
|---|---|
| [`SETUP.md`](SETUP.md) | Manual credential setup — alternative to the interactive wizard |
| `~/.claude/skills/lybel-docs/SKILL.md` | How Claude uses this skill (installed by the install script) |
| [`install/README.md`](install/README.md) | What the install scripts do, for humans who want to inspect before running |

### Repository layout

```
README.md                   This file
SETUP.md                    Manual credential setup guide
install/
  install.sh                POSIX install script (Linux / macOS)
  install.ps1               Windows install script
  README.md                 What the scripts do
adf/
  builder.go                ADF node + mark types and constructor helpers
  converter.go              goldmark AST -> ADF walker
  macros.go                 Pre-processing for [TOC] and ::: container blocks
  edit.go                   Section-level edit ops (append/insert/replace/delete)
  table_edit.go             Table-level ops + --at-level support
  confluence.go             REST API v2 HTTP client + creds + CQL search + 409 detection
  digest.go                 Slim page-summary builder (heading outline, macros, words)
  render.go                 ADF -> markdown-ish plain text (used by home cache)
  cache.go                  HomeCache type + load/save (~/.cache/lybel-docs/home.json)
  lint.go                   ADF structure validator
  *_test.go                 Tests
main.go                     CLI entry, flag parsing, IO plumbing (incl. page digest/apply, search)
main_test.go                CLI integration tests
```

---

> **Repo placeholder:** all URLs above reference `lybel-app/skills`. Once the
> final GitHub repository is confirmed, a single find-replace on this file and
> `install/README.md` is all that is needed.
