# Configuration — credentials, spaces, cache, install check

> **When to read:** at setup / install time, when switching between Confluence spaces, when troubleshooting "credentials invalid" or stale-cache issues, or when the user asks how the skill resolves the active space.

## Contents

- [Credentials and config files](#credentials-and-config-files)
- [Space management](#space-management)
- [Home cache lifecycle](#home-cache-lifecycle)
- [CLI installation check](#cli-installation-check)

---

## Credentials and config files

Since v0.10.0 the CLI uses two separate config files:

- **Credentials** (`~/.config/confluence-docs/credentials`, perms `0600`): `email` + `token` only. Never read raw — use `setup --check` to validate.
- **Config** (`~/.config/confluence-docs/config`, perms `0644`): `cloud`, `active_space_id`, `active_space_key`, `active_space_name`, `active_home_page_id`.

All values are set automatically during `confluence-docs setup` (which auto-detects accessible spaces and asks the user to pick one). **Agents must not read or write these files directly.** Use the CLI commands:

```bash
confluence-docs setup --check              # validate everything is configured
confluence-docs space current              # show active space
confluence-docs space list                 # list all accessible spaces
confluence-docs space use <key>            # switch active space
confluence-docs setup --set cloud acmecorp # change a single config key
```

The active space provides defaults for all commands that need `--space-id` or space key (CQL search, `index`, `home`, `page create`, `check`). Commands that previously required explicit `--space-id <ID>` flags now use the configured space automatically.

## Space management

```bash
# List all spaces (cached 1h; shows active space with ✓)
confluence-docs space list

# Switch active space
confluence-docs space use eng

# Force cache refresh
confluence-docs space list --refresh

# JSON output
confluence-docs space list --json
confluence-docs space current --json
```

After `space use <key>`, all subsequent commands use the new space. The switch is persistent (written to `~/.config/confluence-docs/config`).

## Home cache lifecycle

The local cache at `~/.cache/confluence-docs/home.json` is **shared across all Claude sessions on the same machine** — so if one session refreshes (manually or via auto-refresh-on-write), every other session reading it next sees the updated state automatically. No per-session bookkeeping.

Three rules govern when the cache is updated:

| Trigger | Behavior |
|---|---|
| Read with stale cache (>1h old) or missing | **Auto-refresh** before serving. Caller doesn't have to think about it. |
| Write to the Home via CLI (`page apply` / `index *` on the Home pageId) | **Auto-refresh after PUT** succeeds. Your session sees the new state immediately. |
| Explicit `home --refresh` | **Always fetches**, ignores TTL. Use only when you know another machine just edited the Home and you don't want to wait for the TTL. |

What this means in practice: in a typical session, you never call `home --refresh` explicitly. You just query/show/digest, and writes refresh themselves.

**WRITE SAFETY (critical):** the cache is **read-only for navigation**. It is **NEVER** the source for an update. Any mutation of the Home (or any page) goes through `page apply`, which always GETs fresh ADF before PUT — ensuring you never overwrite changes someone made on another machine.

### Quick reference

- **Reads** (`home --query/--show/--digest`): auto-refresh when stale (>1h) or missing.
- **Writes** to the Home (`page apply` / `index *` on the Home pageId): auto-refresh the cache after the PUT.
- **Explicit `home --refresh`**: always fetches, regardless of cache age.
- **Override TTL**: `--max-age 30m` (more aggressive) or `--max-age 6h` (more relaxed) on any read command.

This invariant — **read-only cache, fresh fetch before every write** — is the core safety property. Don't bypass it (e.g. don't try to PUT the cached ADF directly).

## CLI installation check

Before running any `confluence-docs` command, verify the binary exists and credentials are valid. The bootstrap flow is:

```bash
confluence-docs --version          # binary present?
confluence-docs setup --check      # exit 0 = creds valid
```

Exit codes for `setup --check`:

- `0` — credentials valid AND active space configured → proceed
- `1` — no credentials OR no active space configured → run `confluence-docs setup` interactively (or guide the user through Step 5 of `confluence-docs/cli/README.md`)
- `2` — credentials invalid (token revoked or mistyped) → ask the user to regenerate the token at `https://id.atlassian.com/manage-profile/security/api-tokens` and re-run setup
- `3` — network error → retry once; if it persists, surface the error to the user and fall back to MCP

If the binary is absent entirely, fall back to MCP for the current request and tell the user how to install: `confluence-docs/cli/README.md` has the one-shot install URL.
