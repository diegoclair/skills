# confluence-docs — Setup Guide

> **Most users:** run `confluence-docs setup` (interactive wizard) or ask Claude to
> set it up for you. This file is the manual fallback for developers or anyone
> who prefers to configure credentials by hand.

The `adf`, `edit`, `lint`, and `extract-body` commands work fully offline and
need no credentials. Credentials are only required for `page` and `index`
commands, which talk directly to Confluence.

---

## 1. Generate an Atlassian API Token

1. Open: **https://id.atlassian.com/manage-profile/security/api-tokens**
2. Click **Create API token**.
3. Give it a label — for example, `confluence-docs`.
4. Copy the token immediately. You will not be able to see it again.

Keep this token secret. Treat it like a password. **Never commit it to git.**

---

## 2. Provide Credentials

`confluence-docs` checks four sources in order. Use whichever method fits your
situation best.

### Option 1 — Interactive wizard (recommended for everyone)

```bash
confluence-docs setup
```

The wizard asks for your email and token, writes the credentials file in the
right place for your OS, and sets secure file permissions automatically.

If you prefer to pass values non-interactively (useful in CI or scripting):

```bash
confluence-docs setup --email you@yourcompany.com --token ATATT3xFfGF0...
```

---

### Option 2 — Config file (manual, all platforms)

Create the credentials file at the path for your OS:

| OS | Path |
|---|---|
| **Linux** | `~/.config/confluence-docs/credentials` (or `$XDG_CONFIG_HOME/confluence-docs/credentials` if set) |
| **macOS** | `~/Library/Application Support/confluence-docs/credentials` |
| **Windows** | `%APPDATA%\confluence-docs\credentials` |

Not sure where to look? Run:

```bash
confluence-docs setup --print-config-path
```

This prints the exact absolute path for your machine.

**File format** — one `key=value` per line, no spaces around `=`:

```
email=you@yourcompany.com
token=ATATT3xFfGF0...
```

Lines starting with `#` are ignored (comments).

**Creating the file on Linux / macOS:**

```bash
# Create parent directory
mkdir -p "$(confluence-docs setup --print-config-path | xargs dirname)"

# Write the file (replace values with your real ones)
cat > "$(confluence-docs setup --print-config-path)" << 'EOF'
email=you@yourcompany.com
token=ATATT3xFfGF0...
EOF

# Restrict access to your user only
chmod 600 "$(confluence-docs setup --print-config-path)"
```

**Creating the file on Windows (PowerShell):**

```powershell
# Get the path
$path = confluence-docs setup --print-config-path

# Create parent directory if needed
New-Item -ItemType Directory -Force -Path (Split-Path $path)

# Write the file
Set-Content -Path $path -Value "email=you@yourcompany.com`ntoken=ATATT3xFfGF0..."
```

On Windows, the file inherits your user's ACL from `%APPDATA%` — other users
on the same machine cannot read it. No extra step needed.

---

### Option 3 — Environment variables

Useful if you want credentials available to multiple tools, or in CI pipelines.

Add these to your shell profile (`~/.zshrc`, `~/.bashrc`, `.env`, etc.):

```bash
export ATLASSIAN_EMAIL="you@yourcompany.com"
export ATLASSIAN_API_TOKEN="ATATT3xFfGF0..."
```

On Windows (PowerShell profile or System environment variables):

```powershell
$env:ATLASSIAN_EMAIL = "you@yourcompany.com"
$env:ATLASSIAN_API_TOKEN = "ATATT3xFfGF0..."
```

---

### Option 4 — Per-command flags (scripting / one-off use)

Pass credentials directly on the command line. Good for scripts; avoid in
interactive use because the token may appear in shell history.

```bash
confluence-docs page get --page-id 164232 \
  --email you@yourcompany.com \
  --token ATATT3xFfGF0...
```

---

## 3. Smoke Test

Once credentials are configured, verify everything works:

```bash
confluence-docs setup --check
```

Exit codes:

| Code | Meaning |
|---|---|
| `0` | Credentials valid — you are good to go |
| `1` | No credentials found — run `confluence-docs setup` |
| `2` | Credentials present but rejected by Atlassian — see Troubleshooting |
| `3` | Network error — check internet connection and VPN |

---

## 4. Cloud Configuration

By default `confluence-docs` connects to `lybel.atlassian.net`. To use a different
Confluence instance:

```bash
# Via flag (single command)
confluence-docs page get --cloud mycompany --page-id 123

# Via environment variable (persists across commands)
export ATLASSIAN_CLOUD=mycompany
```

The value is the subdomain only (e.g. `lybel`, not `lybel.atlassian.net`).

---

## 5. Troubleshooting

### "Invalid credentials" / exit code 2

The token was probably revoked, mistyped, or copied with extra whitespace.

1. Go to **https://id.atlassian.com/manage-profile/security/api-tokens**.
2. Revoke the old `confluence-docs` token.
3. Click **Create API token**, create a new one with the same label.
4. Run `confluence-docs setup` (or edit the credentials file) with the new token.
5. Run `confluence-docs setup --check` — should now exit `0`.

### "Wrong email"

The email must match the Atlassian account that owns the token. It is usually
your work email (e.g. `you@yourcompany.com`). Check it at
**https://id.atlassian.com/manage-profile/profile-and-visibility**.

### Network error / exit code 3

- Confirm you can reach `https://lybel.atlassian.net` in a browser.
- If behind a VPN, make sure it is connected.
- Corporate proxies: set `HTTPS_PROXY` in your environment if needed.

### "Command not found: confluence-docs"

The binary is not on your `$PATH`. Run:

```bash
export PATH="$HOME/.claude/skills/confluence-docs/bin:$PATH"
```

Add that line to your shell profile to make it permanent. Or re-run the install
script, which adds it automatically.

---

## 6. Security Notes

- **Never commit your token to git.** The credentials file lives outside the
  repo by design (`~/.config/...`, `~/Library/...`, `%APPDATA%\...`).
- Set file permissions to `600` on Linux / macOS so only your user can read it.
  The setup wizard does this automatically.
- Rotate your token periodically. You can create multiple tokens; delete old
  ones at **https://id.atlassian.com/manage-profile/security/api-tokens**.
- The token grants access to everything your Atlassian account can see.
  Scope it accordingly.

---

## 7. Quick Reference

| What you want | Command |
|---|---|
| Interactive setup | `confluence-docs setup` |
| Validate credentials | `confluence-docs setup --check` |
| See credentials file path | `confluence-docs setup --print-config-path` |
| Fetch a page as ADF | `confluence-docs page get --page-id ID --format adf --output page.json` |
| Edit a page (local) | `confluence-docs edit --input page.json --append frag.md > updated.json` |
| Preview upload | `confluence-docs page upload --page-id ID --adf updated.json --dry-run` |
| Upload changes | `confluence-docs page upload --page-id ID --adf updated.json --message "what changed"` |
| Create a new page | `confluence-docs page create --space-id 131352 --parent-id PARENT --title "Title" --markdown content.md` |
| Add row to index | `confluence-docs index add --page-id 999 --title "Page Name" --under "Section Heading"` |
| Validate ADF file | `confluence-docs lint page.json` |
| Unwrap MCP response | `confluence-docs extract-body < mcp-response.json > body.json` |
