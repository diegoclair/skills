# confluence-docs

Confluence ADF toolkit and skill for Claude. Edits Confluence Cloud pages
without destroying macros; consolidates a knowledge base into a classified
Knowledge Map; supports `:::properties`, smart links, page labels, and user
`@mentions` out of the box. Designed for any team running a Confluence Cloud
space alongside Claude (or any other LLM agent) that maintains docs.

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
curl -fsSL https://raw.githubusercontent.com/lybel-app/skills/main/confluence-docs/install/install.sh | bash
```

**Windows:**

```powershell
iwr -useb https://raw.githubusercontent.com/lybel-app/skills/main/confluence-docs/install/install.ps1 | iex
```

The script downloads the correct binary, places it at
`~/.claude/skills/confluence-docs/bin/confluence-docs` (or `.exe` on Windows), and also
extracts the SKILL.md and reference files from the same release archive.
The scripts live at [`../install/install.sh`](../install/install.sh) and
[`../install/install.ps1`](../install/install.ps1) — read them before piping
to `bash`/`iex` if you want to inspect what runs.

### Step 3 — Verify the binary

```bash
confluence-docs --version
```

Expected output: `confluence-docs v<x.y.z>`. If the command is not found, check
that `~/.claude/skills/confluence-docs/bin/` is on `$PATH` (the install script
handles this, but a new shell session may be needed).

### Step 4 — Check credentials

```bash
confluence-docs setup --check
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

1. Tell the user (in their language — match whatever they wrote to you; for a Portuguese-speaking user, use Portuguese):

   > "Preciso de um token do Atlassian pra conectar ao Confluence. Abre essa
   > URL no navegador, clica em **Create API token**, dá um nome (ex:
   > `confluence-docs`), copia o token e me cola aqui. Também me diz o teu email
   > do Atlassian."
   >
   > URL: `https://id.atlassian.com/manage-profile/security/api-tokens`

2. Wait for the user to paste their **email** and **token** in chat.

3. Get the OS-correct config file path:

   ```bash
   confluence-docs setup --print-config-path
   ```

4. Get the exact file format:

   ```bash
   confluence-docs setup --print-config-format
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
   confluence-docs setup --check
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

**confluence-docs** is a command-line tool that:

- Drives Claude's Confluence skill (`~/.claude/skills/confluence-docs/SKILL.md`) so
  the assistant can hit Confluence directly without paying the token cost of
  the Atlassian MCP. Most operations return sub-KB output instead of full ADF
  bodies — typically 10–50× cheaper across a multi-edit session.
- Maintains a **local cache of the Confluence Home** (`home` command) at
  `~/.cache/confluence-docs/home.json`. One `home --refresh` per session pulls the
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
- **Page lifecycle** (`page move`, `page reorder`, `page delete`): rename a
  page, reparent it under a new ancestor, reorder it among siblings, or trash
  it (soft delete, restorable). `move` handles rename + reparent in one PUT;
  `reorder` calls the v1 `move/{position}/{targetId}` endpoint to shift
  sibling order without touching body or title.
- **CQL search** (`search`) returning compact TSV (id, title, url, excerpt)
  instead of MCP's verbose JSON.
- Converts extended Markdown to Atlassian Document Format (ADF) JSON, including
  Confluence macros (`[TOC]`, `:::expand`, `:::warning`, etc.).
- Applies section-level edits to existing ADF pages **without touching macros**
  outside the edited section — a property the Confluence MCP tool does not
  guarantee.
- Talks directly to the Confluence Cloud REST API v2, bypassing MCP for large
  pages (>50 kB) where tool calls may time out.
- Manages the Page ID Index table on your project's Home page.
- Validates ADF structure and reports errors.

### Quick install

**Linux / macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/lybel-app/skills/main/confluence-docs/install/install.sh | bash
```

**Windows:**

```powershell
iwr -useb https://raw.githubusercontent.com/lybel-app/skills/main/confluence-docs/install/install.ps1 | iex
```

After the script finishes, configure your credentials:

```bash
confluence-docs setup
```

Or ask Claude to do it for you — paste this page's URL in a Claude conversation
and say "instala essa skill".

### Manual install (build from source)

Requires Go 1.26.2+. No other runtime dependencies.

```bash
make build     # builds bin/confluence-docs
make install   # builds and copies to ~/.claude/skills/confluence-docs/bin/confluence-docs
               # (override with INSTALL_DIR=/custom/path)
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
| `page move` | Rename and/or reparent a page. Flags: `--page-id ID` plus at least one of `--parent-id NEW_PARENT` / `--title NEW_TITLE`. Body is preserved (refetched and re-PUT, since v2 PUT requires it). Alias: `page rename` |
| `page reorder` | Reposition a page among its siblings or append it under a different parent. Flags: `--page-id ID` plus exactly one of `--before TARGET_ID` / `--after TARGET_ID` / `--append-to PARENT_ID`. Calls the v1 `move/{position}/{targetId}` endpoint; body and title are not touched |
| `page delete` | Soft-delete a page (sends to Confluence trash, restorable). Requires `--yes` to confirm. Alias: `page trash` |
| `page children` | List direct children of a page (TSV: id, title). Old name `list-children` still works as alias |
| `search` | CQL search via the v1 API. TSV output (`pageId\ttitle\turl\texcerpt`). Defaults to `space="<your space key>" AND type="page"` (auto-resolved from credentials) |
| `home` | Local Home-page cache. Verbs: `--refresh` (force GET + cache), `--status` (metadata), `--show` (text), `--query "X"` (grep), `--digest`. Cache at `~/.cache/confluence-docs/home.json`. Read-only — writes always GET fresh ADF first |
| `lint` | Validate ADF structure and report errors/warnings |
| `extract-body` | Unwrap the ADF body from an MCP `getConfluencePage` response |
| `index` | Manage the Page ID Index table on your project's Home page |

Run `confluence-docs --help` or `confluence-docs <command> --help` for full flag
documentation.

Exit codes for `adf`/`edit`/`lint`/`extract-body`: `0` success, `1` parse
error, `2` invalid input, `3` unknown/HTTP error.

Exit codes for `setup --check`: `0` valid, `1` missing, `2` invalid, `3`
network error.

### Documentation

| File | Purpose |
|---|---|
| [`SETUP.md`](SETUP.md) | Manual credential setup — alternative to the interactive wizard |
| [`../SKILL.md`](../SKILL.md) | How Claude uses this skill (installed by the install script to `~/.claude/skills/confluence-docs/SKILL.md`) |
| [`../install/install.sh`](../install/install.sh) / [`install.ps1`](../install/install.ps1) | The install scripts — inspect before piping to `bash`/`iex` |

### Repository layout

This is the **CLI source dir** (`confluence-docs/cli/`). The skill payload
lives one level up at `confluence-docs/` (SKILL.md + reference/), and the
end-user install scripts live at `confluence-docs/install/`. See the
[root README](../../README.md) for the full repo convention.

```
confluence-docs/
├── SKILL.md                Skill entrypoint (read by Claude at runtime)
├── reference/              Skill reference docs (templates, taxonomy, workflows)
├── install/
│   ├── install.sh          POSIX install script (Linux / macOS)
│   └── install.ps1         Windows install script
└── cli/                    ← you are here
    ├── README.md           This file
    ├── SETUP.md            Manual credential setup guide
    ├── Makefile            build / build-all / test / install
    ├── main.go             CLI entry, flag parsing, IO plumbing (incl. page digest/apply, search)
    ├── main_test.go        CLI integration tests
    ├── go.mod / go.sum
    ├── setup/              Credential wizard (confluence-docs setup)
    └── adf/
        ├── builder.go      ADF node + mark types and constructor helpers
        ├── converter.go    goldmark AST -> ADF walker
        ├── macros.go       Pre-processing for [TOC] and ::: container blocks
        ├── edit.go         Section-level edit ops (append/insert/replace/delete)
        ├── table_edit.go   Table-level ops + --at-level support
        ├── confluence.go   REST API v2 HTTP client + creds + CQL search + 409 detection
        ├── digest.go       Slim page-summary builder (heading outline, macros, words)
        ├── render.go       ADF -> markdown-ish plain text (used by home cache)
        ├── cache.go        HomeCache type + load/save (~/.cache/confluence-docs/home.json)
        ├── lint.go         ADF structure validator
        └── *_test.go       Tests
```
