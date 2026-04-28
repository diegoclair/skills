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

- Converts extended Markdown to Atlassian Document Format (ADF) JSON, including
  Confluence macros (`[TOC]`, `:::expand`, `:::warning`, etc.).
- Applies section-level edits to existing ADF pages **without touching macros**
  outside the edited section — a property the Confluence MCP tool does not
  guarantee.
- Talks directly to the Confluence Cloud REST API v2, bypassing MCP for large
  pages (>50 kB) where tool calls may time out.
- Manages the Page ID Index table on the Lybel Home page.
- Validates ADF structure and reports errors.
- Drives Claude's Confluence skill (`~/.claude/skills/lybel-docs/SKILL.md`) with
  a binary that Claude can call without an MCP server.

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
| `adf` | Convert Markdown (+ Confluence macro extensions) to ADF JSON |
| `edit` | Apply a section-level or table-level operation to existing ADF without touching macros |
| `page` | Fetch, upload, or create Confluence pages via HTTP (bypasses MCP) |
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
  confluence.go             Confluence Cloud REST API v2 HTTP client + creds
  lint.go                   ADF structure validator
  *_test.go                 Tests
main.go                     CLI entry, flag parsing, IO plumbing
main_test.go                CLI integration tests
```

---

> **Repo placeholder:** all URLs above reference `lybel-app/skills`. Once the
> final GitHub repository is confirmed, a single find-replace on this file and
> `install/README.md` is all that is needed.
