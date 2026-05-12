# confluence-docs — Setup Guide

> **Most users:** run `confluence-docs setup` (interactive wizard) or ask Claude to
> set it up for you. This file is the manual fallback for developers or anyone
> who prefers to configure credentials by hand.

The `adf`, `edit`, `lint`, and `extract-body` commands work fully offline and
need no credentials. Credentials are only required for `page`, `index`,
`search`, `home`, `space`, and `check` commands, which talk directly to
Confluence.

---

## What changed in v0.10.0

Previously all configuration (email, token, cloud subdomain) lived in a single
`credentials` file. **v0.10.0 splits this into two files:**

| File | Content | Permissions |
|---|---|---|
| `credentials` | `email` + `token` only | `0600` (secrets) |
| `config` | `cloud`, `active_space_*`, `active_home_page_id` | `0644` (non-sensitive) |

**Migration is automatic**: the first time you run `confluence-docs setup` after
upgrading, it rewrites both files cleanly. Until you do that, the CLI reads
`cloud=` from the old `credentials` file as a fallback — existing setups keep
working without any action.

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

The wizard:
1. Asks for your Atlassian email.
2. Asks for your API token.
3. Asks for your Confluence subdomain (e.g. `mycompany` for `mycompany.atlassian.net`).
4. Validates the connection.
5. **Auto-detects accessible spaces** via the API.
   - If 1 space: selects it automatically.
   - If multiple spaces: lists them and asks you to pick one.
6. Writes `credentials` (email + token) and `config` (cloud + active space).

If you prefer to pass values non-interactively (useful in CI or scripting):

```bash
confluence-docs setup --email you@yourcompany.com --token ATATT3xFfGF0...
```

(The cloud subdomain must be set via `$ATLASSIAN_CLOUD` for non-interactive mode, and the space can be configured separately with `confluence-docs space use <key>`.)

---

### Option 2 — Config files (manual, all platforms)

**Credentials file** (secrets, `0600`):

| OS | Path |
|---|---|
| **Linux** | `~/.config/confluence-docs/credentials` (or `$XDG_CONFIG_HOME/confluence-docs/credentials`) |
| **macOS** | `~/Library/Application Support/confluence-docs/credentials` |
| **Windows** | `%APPDATA%\confluence-docs\credentials` |

```
email=you@yourcompany.com
token=ATATT3xFfGF0...
```

**Config file** (non-sensitive, `0644`) — same directory as credentials:

```
cloud=yourcompany
active_space_id=131352
active_space_key=myspace
active_space_name=My Space
active_home_page_id=164232
```

Not sure where to look? Run:

```bash
confluence-docs setup --print-config-path   # credentials file path
```

---

### Option 3 — Environment variables

```bash
export ATLASSIAN_EMAIL="you@yourcompany.com"
export ATLASSIAN_API_TOKEN="ATATT3xFfGF0..."
export ATLASSIAN_CLOUD="yourcompany"   # cloud subdomain
```

---

### Option 4 — Per-command flags (scripting / one-off use)

```bash
confluence-docs page get --page-id 164232 \
  --email you@yourcompany.com \
  --token ATATT3xFfGF0... \
  --cloud yourcompany
```

---

## 3. Smoke Test

```bash
confluence-docs setup --check
```

Exit codes:

| Code | Meaning |
|---|---|
| `0` | Credentials valid AND active space configured — you are good to go |
| `1` | No credentials OR no active space configured — run `confluence-docs setup` |
| `2` | Credentials present but rejected by Atlassian — see Troubleshooting |
| `3` | Network error — check internet connection and VPN |

---

## 4. Space Configuration

After initial setup, the active space is stored in the config file and used
automatically by all commands. To change it:

```bash
# List all accessible spaces (cached 1h)
confluence-docs space list

# Switch to a different space
confluence-docs space use eng

# See the currently active space
confluence-docs space current

# Force refresh the space list from API
confluence-docs space list --refresh
```

To manually set a single config value:

```bash
confluence-docs setup --set active_space_key myspace
confluence-docs setup --set cloud acmecorp
```

Valid `--set` keys: `cloud`, `active_space_id`, `active_space_key`,
`active_space_name`, `active_home_page_id`. For space switching, prefer
`space use <key>` over manual `--set` since it also updates the home page ID.

---

## 5. Reconfigure

To re-run the full setup wizard (e.g. after a token rotation):

```bash
confluence-docs setup --reconfigure
```

This is identical to running `confluence-docs setup` fresh, but prefills all
fields with the current stored values so you only need to change what's
different.

---

## 6. Troubleshooting

### "Invalid credentials" / exit code 2

The token was probably revoked, mistyped, or copied with extra whitespace.

1. Go to **https://id.atlassian.com/manage-profile/security/api-tokens**.
2. Revoke the old `confluence-docs` token.
3. Click **Create API token**, create a new one with the same label.
4. Run `confluence-docs setup` (or edit the credentials file) with the new token.
5. Run `confluence-docs setup --check` — should now exit `0`.

### "No active space configured" / exit code 1 (no credentials)

This appears when credentials are valid but no space has been selected (common
for users upgrading from v0.9.x). Fix:

```bash
confluence-docs space list         # see what's available
confluence-docs space use <key>    # pick one
```

Or simply run `confluence-docs setup` again — it will auto-detect spaces and
ask you to pick.

### "Wrong email"

The email must match the Atlassian account that owns the token. It is usually
your work email (e.g. `you@yourcompany.com`). Check it at
**https://id.atlassian.com/manage-profile/profile-and-visibility**.

### Network error / exit code 3

- Confirm you can reach `https://<your-subdomain>.atlassian.net` in a browser.
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

## 7. Security Notes

- **Never commit your token to git.** The credentials file lives outside the
  repo by design (`~/.config/…`, `~/Library/…`, `%APPDATA%\…`).
- Set file permissions to `600` on Linux / macOS so only your user can read the
  `credentials` file. The setup wizard does this automatically.
- The `config` file (cloud + space settings) uses `0644` permissions — it
  contains no secrets and can be read by tools or copied between machines safely.
- Rotate your token periodically at **https://id.atlassian.com/manage-profile/security/api-tokens**.

---

## 8. Quick Reference

| What you want | Command |
|---|---|
| Interactive setup | `confluence-docs setup` |
| Re-run wizard keeping existing values | `confluence-docs setup --reconfigure` |
| Validate credentials + space | `confluence-docs setup --check` |
| See credentials file path | `confluence-docs setup --print-config-path` |
| Set a single config value | `confluence-docs setup --set <key> <value>` |
| List accessible spaces | `confluence-docs space list` |
| Switch active space | `confluence-docs space use <key>` |
| Show active space | `confluence-docs space current` |
| Fetch a page as ADF | `confluence-docs page get --page-id ID --format adf --output page.json` |
| Edit a page (local) | `confluence-docs edit --input page.json --append frag.md > updated.json` |
| Preview upload | `confluence-docs page upload --page-id ID --adf updated.json --dry-run` |
| Upload changes | `confluence-docs page upload --page-id ID --adf updated.json --message "what changed"` |
| Create a new page | `confluence-docs page create --parent-id PARENT --title "Title" --markdown content.md` |
| Add row to index | `confluence-docs index add --page-id 999 --title "Page Name" --under "Section Heading"` |
| Validate ADF file | `confluence-docs lint page.json` |
| Unwrap MCP response | `confluence-docs extract-body < mcp-response.json > body.json` |

Note: `page create` no longer requires an explicit `--space-id` flag when an
active space is configured — the CLI reads it from the config file.
