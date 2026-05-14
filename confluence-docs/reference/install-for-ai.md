# Install Guide for AI Agents

> **Audience:** AI assistants helping a human install the `confluence-docs` skill.
> Humans installing it themselves should read the [main README](../README.md).

Self-contained, low-ambiguity, deterministic exit codes. Keep it that way.

## Prerequisites

- An Atlassian account with access to the target Confluence cloud.
- Internet access.
- macOS, Linux, or Windows 10+.

If the user has no Atlassian account, **stop** — they need to create one and join their workspace first.

---

## Step 1 — Install

**macOS / Linux** (bash/zsh):
```bash
curl -fsSL https://raw.githubusercontent.com/lybel-app/skills/main/confluence-docs/install/install.sh | bash
```

**Windows** (PowerShell 5+):
```powershell
iwr -useb https://raw.githubusercontent.com/lybel-app/skills/main/confluence-docs/install/install.ps1 | iex
```

> Do not use WSL's `install.sh` if the user runs Claude Desktop on Windows — install from PowerShell so the binary lands under `%USERPROFILE%\.claude\skills\…`.

The installer downloads the latest release, extracts to `~/.claude/skills/confluence-docs/` (or `%USERPROFILE%\.claude\skills\…` on Windows), and adds the binary to PATH idempotently. After it finishes, tell the user to **open a new terminal** (or `source ~/.zshrc` — the installer prints which profile it touched).

---

## Step 2 — Generate an Atlassian API token

Walk the user through this **in chat**:

> 1. Open: https://id.atlassian.com/manage-profile/security/api-tokens
> 2. Click **Create API token**, name it `confluence-docs`, leave scopes default.
> 3. Copy the token (only shown once) and paste it here.

Also ask for their Atlassian email if you don't have it.

### Security — non-negotiable

- **Never echo the token** back to the chat after the user pastes it. Not in summaries, not in logs, not in files you write.
- **Never save the token to memory** if you have a memory tool. The CLI stores it at `0600` permissions — that's the only place it should live.

---

## Step 3 — Save credentials

Always non-interactively. **Do not run `confluence-docs setup` without flags** — the interactive wizard reads stdin and will hang the agent shell.

```bash
confluence-docs setup --email "<USER_EMAIL>" --token "<USER_TOKEN>"
```

If `confluence-docs` is not on PATH yet (new terminal not opened), use the absolute path the installer printed:

```bash
~/.claude/skills/confluence-docs/bin/confluence-docs setup --email "..." --token "..."
```

---

## Step 4 — Validate

```bash
confluence-docs setup --check
```

| Exit | Meaning | Action |
|---|---|---|
| `0` | Credentials valid | Proceed to Step 5. |
| `1` | No credentials file | Re-run Step 3. |
| `2` | Rejected by Atlassian | Token wrong/revoked — regenerate (Step 2), redo Step 3. |
| `3` | Network error | Confirm internet/VPN, retry. |

---

## Step 5 — Smoke test

```bash
confluence-docs home --refresh
```

Successful output prints a digest of the home (title, headings, link count). On error, retry with `--verbose` and report the output to the user.

---

## Common failures

| Symptom | Fix |
|---|---|
| `command not found: confluence-docs` | New terminal, or `source ~/.zshrc`. Until then, use the absolute path `~/.claude/skills/confluence-docs/bin/confluence-docs`. |
| Windows SmartScreen blocks the binary | User clicks **More info** → **Run anyway**. (Binary is unsigned.) |
| `setup --check` exit `2` | Token typed wrong or copied with whitespace. Regenerate (Step 2). |

---

## Do not

- **Do not** clone the repo to install — that path is for contributors and tends to fail in sandboxed AI environments (symlinks under `~/.claude/skills/` get flagged as agent self-modification).
- **Do not** edit the user's shell profile directly. The installer handles PATH.

---

## When done

Confirm the user can ask things like *"where is the governance page?"* or *"create meeting notes for the Acme call"* and get a response. If something silently fails, re-run Step 4 (credentials) then Step 5 (connectivity).
