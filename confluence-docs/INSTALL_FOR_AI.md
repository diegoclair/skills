# Install Guide for AI Agents

> **Audience:** AI assistants (Claude, Gemini, ChatGPT, Cursor, …) helping a
> human install the `confluence-docs` skill. Humans installing it themselves
> should read the [main README](./../README.md#install) instead.

This file is a self-contained runbook. A user can paste its URL into any AI
agent and the agent follows these steps end to end. Keep it that way:
**self-contained, low-ambiguity, deterministic exit codes**.

---

## Prerequisites the user must already have

- An Atlassian account with access to the target Confluence cloud
  (default: `lybel.atlassian.net`).
- Internet access (the installer pulls a release archive from GitHub).
- One of: macOS, Linux, or Windows 10+.

If the user does not have an Atlassian account yet, **stop**. Ask them to
create one and join their company's workspace first.

---

## Step 1 — Detect the OS

Pick the installer based on what the user is running:

| OS                | Installer                | Shell to use            |
|-------------------|--------------------------|-------------------------|
| macOS             | `install.sh`             | bash / zsh              |
| Linux (any distro)| `install.sh`             | bash / sh               |
| Windows 10/11     | `install.ps1`            | PowerShell 5+           |

**Do not** use WSL's `install.sh` if the user is going to use the skill from
Claude Desktop on Windows — install it from PowerShell so the binary lives
under `%USERPROFILE%\.claude\skills\…`.

---

## Step 2 — Run the installer

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/lybel-app/skills/main/confluence-docs/install/install.sh | bash
```

### Windows (PowerShell)

```powershell
iwr -useb https://raw.githubusercontent.com/lybel-app/skills/main/confluence-docs/install/install.ps1 | iex
```

The installer:

1. Downloads the latest release archive for the user's platform.
2. Extracts the binary + `SKILL.md` + `reference/` into the Claude skills dir
   (`~/.claude/skills/confluence-docs/` or `%USERPROFILE%\.claude\skills\confluence-docs\`).
3. Symlinks the binary into `~/.local/bin/` (Linux/macOS) or registers it on
   the User PATH (Windows).
4. **Adds `~/.local/bin` to the user's shell profile** if it is not already on
   `$PATH` (`.zshrc` / `.bashrc` / `.bash_profile` / `.profile`, picked from
   `$SHELL`). Idempotent — re-runs do nothing.
5. Verifies the binary by running `--version`.
6. Reports whether credentials are configured.

Tell the user to **open a new terminal** after the installer finishes (so the
updated PATH takes effect). On the same shell session, they can also run
`source ~/.zshrc` (or whichever profile the installer modified — check the
installer's output line "Added to …").

---

## Step 3 — Generate an Atlassian API token

Walk the user through this **interactively in chat** — do not run any command
yet:

> 1. Open this URL in a new tab: **https://id.atlassian.com/manage-profile/security/api-tokens**
> 2. Click **Create API token**.
> 3. Give it a name like `confluence-docs`. **Scopes:** leave at default
>    (legacy token) — this is what the CLI expects.
> 4. Copy the token. **It is only shown once.** Paste it here.

Also ask for their **email** if you don't already have it. It is the email
attached to their Atlassian account — usually their work email, e.g.
`name@company.com.br`.

### Security rules — non-negotiable

- **Never echo the token back** to the chat after the user pastes it. Do not
  include it in any summary, log line, or file you write.
- **Never commit it** to any repo or paste it into any external system.
- **Do not save it to memory** if you have a memory tool. The CLI stores it
  on the local filesystem with `0600` permissions (Unix) or user-only ACL
  (Windows) — that is the only place it should live.

---

## Step 4 — Save credentials non-interactively

```bash
confluence-docs setup --email "<USER_EMAIL>" --token "<USER_TOKEN>"
```

> **Critical:** **do not run `confluence-docs setup` without flags in an AI
> session.** It enters an interactive wizard that reads from stdin and will
> hang the agent shell.

If `confluence-docs` is not yet on PATH (the user opened a new terminal but
their session inherits the old PATH), use the absolute path the installer
printed in its summary, e.g.:

```bash
~/.claude/skills/confluence-docs/bin/confluence-docs setup --email "..." --token "..."
```

The command prints `credentials saved to <path>` on success.

---

## Step 5 — Validate

```bash
confluence-docs setup --check
```

Exit codes:

| Code | Meaning                                    | What to do                                        |
|------|--------------------------------------------|---------------------------------------------------|
| `0`  | Credentials valid — proceed.               | Move to Step 6.                                   |
| `1`  | No credentials file present.               | Re-run Step 4.                                    |
| `2`  | Credentials rejected by Atlassian.         | Token wrong/revoked — generate a new one (Step 3) and re-run Step 4. |
| `3`  | Network error.                             | Confirm internet/VPN; retry in a moment.          |

---

## Step 6 — Smoke test

```bash
confluence-docs home --refresh
```

This downloads the Confluence home page and caches it locally. Successful
output prints a digest of the home (title, headings, link count). If it
errors, capture the message and fall back to:

```bash
confluence-docs home --refresh --verbose
```

…and report the verbose output to the user.

---

## Common failure modes

| Symptom                                                | Cause                                              | Fix                                                                                       |
|--------------------------------------------------------|----------------------------------------------------|-------------------------------------------------------------------------------------------|
| `command not found: confluence-docs`                   | New PATH not picked up by current shell.           | Tell user to open a new terminal, or run `source ~/.zshrc` (or whichever the installer mentioned). Until then, use the absolute path `~/.claude/skills/confluence-docs/bin/confluence-docs`. |
| `Permission denied` when running the binary (Linux/macOS) | Binary lost its execute bit (rare).               | `chmod +x ~/.claude/skills/confluence-docs/bin/confluence-docs`                           |
| Windows SmartScreen blocks the binary                  | Binary is unsigned.                                | User clicks **More info** → **Run anyway** in the SmartScreen dialog.                     |
| `setup --check` exit code `2`                          | Token typed wrong, revoked, or copied with whitespace. | Regenerate token (Step 3), re-run Step 4 with the fresh value.                            |
| `setup --check` exit code `3`                          | No internet, VPN required, or Atlassian outage.    | Have the user verify they can open `https://lybel.atlassian.net` in a browser, then retry. |
| Cloning the repo to "do it yourself"                   | You followed the dev install path by mistake.      | Stop. The dev path is for repo contributors. Use the installer (Step 2) instead.          |

---

## Things you should NOT do

- **Do not** run `confluence-docs setup` without `--email` and `--token` flags
  in an AI session. The interactive wizard will hang.
- **Do not** clone the repo to install the skill. The dev install (clone +
  symlink + `make install`) is for contributors, not end users. It also tends
  to fail in sandboxed AI environments because creating a symlink under
  `~/.claude/skills/` is treated as agent self-modification.
- **Do not** edit the user's shell profile (`~/.zshrc`, `~/.bashrc`, …)
  directly. The installer handles PATH. If something is wrong with PATH after
  the installer ran, surface it to the user and let them edit their profile.
- **Do not** save the user's API token to your memory, scratchpad, or any
  file you write. The CLI persists it to the right place with the right
  permissions — that is enough.
- **Do not** push the user toward `confluence-docs setup` (interactive) in
  chat. Always pass `--email` and `--token` explicitly.

---

## When you are done

Confirm with the user that they can ask things like:

- *"onde fica a página de governança?"*
- *"quais aceleradoras a Lybel está mapeando?"*
- *"cria uma ata da reunião com o Itaú"*

…and the skill responds. If any of these silently fail, re-run Step 5 to make
sure credentials still validate, then Step 6 for connectivity.
