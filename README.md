# Lybel Skills

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![confluence-docs](https://img.shields.io/github/v/release/diegoclair/skills?filter=confluence-v*&color=11C47E&label=confluence-docs)](https://github.com/diegoclair/skills/releases?q=tag%3Aconfluence-v)
[![jira-tickets](https://img.shields.io/github/v/release/diegoclair/skills?filter=jira-v*&color=11C47E&label=jira-tickets)](https://github.com/diegoclair/skills/releases?q=tag%3Ajira-v)
[![social-carousel](https://img.shields.io/github/v/release/diegoclair/skills?filter=carousel-v*&color=11C47E&label=social-carousel)](https://github.com/diegoclair/skills/releases?q=tag%3Acarousel-v)
[![Claude Skills](https://img.shields.io/badge/Claude-Skills-11C47E)](https://docs.claude.com/en/docs/claude-code/skills)

> Open-source Claude Skills maintained by the **Lybel** team. Works for any company — point each skill at your own Confluence / Jira / etc. PRs welcome.

## Available skills

| Skill | Summary | Docs |
|---|---|---|
| **`confluence-docs`** | Search, create, classify and update Confluence Cloud pages in natural language. Ships a local Go CLI that returns page digests / single sections instead of full ADF bodies — 10–50× cheaper in tokens than the raw MCP path (which remains as fallback). Includes a `km` subcommand that consolidates a whole space into a typed Knowledge Map, owner `@mention` resolution, real Confluence labels from `:::properties` tags, smart links, and a canonical 5-doc-types spec (`reference/doc-types.md`). | [SKILL.md](./confluence-docs/SKILL.md) |
| **`jira-tickets`** | Token-efficient Jira Cloud assistant. Shares `pkg/atlassian` with `confluence-docs` (same Atlassian API token via `~/.config/atlassian/credentials`, same ADF format). Commands: `myself`, `search "JQL"`, `issue digest` (~500 B summary), `issue get`, `issue create/update/transition/comment`, `issue transitions` list, `project list/get/update`, `update` (self-update). Epic linking, sprints, boards, attachments, worklogs parked — see [ROADMAP.md](./jira-tickets/ROADMAP.md). | [SKILL.md](./jira-tickets/SKILL.md) |
| **`social-carousel`** | Generates viral Instagram and LinkedIn carousels from a small YAML brief. Renders locally via headless Chrome (`chromedp`) — zero per-image cost, zero account, no SaaS round-trip. Ships 5 design presets, 7 layout templates (cover, list, big-number, quote, comparison, screenshot, cta), and a linter with 27 research-backed rules (`slide-3 must be value bomb`, `≤12-word hook`, `single CTA`, contrast ≥4.5:1) that blocks render unless `--force`. Commands: `new <kind>`, `check`, `render`, `preview`, `theme list/show/create`, `setup`, `update`. | [SKILL.md](./social-carousel/SKILL.md) |

Next candidates: `figma-files`, `analytics`.

---

## How it works

Skills here are **timeless**: the repo only ships structure, workflows, templates, and a canonical spec. **Zero project-specific data** (no advisors, no investors, no hardcoded page IDs). At runtime, Claude reads each project's Confluence Home page, which is the source of truth for taxonomy and the page index. That is why this repo is safe to be public.

**To adopt for your company:** run `confluence-docs setup` once — the wizard asks for your Atlassian email, API token, and Confluence subdomain (e.g. `mycompany` for `mycompany.atlassian.net`) and writes them to a credentials file. Create a Home page in your Confluence space following the same conventions described in [SKILL.md](./confluence-docs/SKILL.md), then point your team at the skill. See [Contributing](#contributing) for why no company-specific data is allowed in the skill body.

### Why CLI in addition to MCP

The Atlassian MCP returns the full ADF body of every page (10–40 KB of JSON). In a research + edit session, that burns the context window fast. The CLI lives in `~/.claude/skills/confluence-docs/bin/` and offers:

- **`home --refresh`** — fetches the Home once per hour and caches locally; subsequent queries are offline.
- **`page digest --page-id ID`** — title, version, outline, macro count, word count — all in ~500 bytes.
- **`page apply --replace-section`** — atomic section edit (GET → PUT with 409 retry). Macros outside the targeted section are preserved byte-for-byte.
- **`search "term"`** — CQL with compact TSV output.
- **`new <type>`**, **`check`**, **`km generate`** — doc-type templates, fuzzy duplicate detection before creating, and automated Knowledge Map regeneration.

Every write does a fresh GET before the PUT, so the cache never causes accidental overwrite.

---

## Installation

Each skill has its own one-liner. The two installers share `pkg/install/install.{sh,ps1}` under the hood, so flags and layout are consistent.

**`confluence-docs`** (macOS / Linux):
```bash
curl -fsSL https://raw.githubusercontent.com/diegoclair/skills/main/confluence-docs/install/install.sh | bash
```

**`jira-tickets`** (macOS / Linux):
```bash
curl -fsSL https://raw.githubusercontent.com/diegoclair/skills/main/jira-tickets/install/install.sh | bash
```

**Windows (PowerShell):** swap `install.sh | bash` for `install.ps1 | iex` and prefix with `iwr -useb`.

Each installer is idempotent: it resolves the latest release for its tag prefix (`confluence-v*` / `jira-v*`) via the GitHub API, places everything in `~/.claude/skills/<skill>/`, symlinks the binary into `~/.local/bin`, and reports whether credentials are already configured. **Open a new shell** afterwards (or `source ~/.zshrc`) for the PATH change to take effect. To pin a specific release: `SKILL_VERSION=confluence-v0.14.0 bash`.

Then create an Atlassian token at https://id.atlassian.com/manage-profile/security/api-tokens. Credentials are **shared** across both skills via `~/.config/atlassian/credentials` — configure once, both work:

```bash
confluence-docs setup                                      # interactive wizard (email + token + subdomain)
confluence-docs setup --email X --token Y                  # non-interactive (CI / agent)
confluence-docs setup --check                              # validates current credentials
jira-tickets setup --check                                 # reuses the shared credentials
```

Reopen Claude Code and ask: *"where is the governance page?"*, *"create a Jira task for the bug I just hit"*, *"which competitors are we tracking?"*.

**Update:** `confluence-docs update` / `jira-tickets update` (each self-updates via the GitHub API, filtered by its own tag prefix). **Uninstall:** delete `~/.claude/skills/<skill>/`. Remove `~/.config/atlassian/` only if you're uninstalling **both**.

### AI-assisted installation

Paste this into any AI agent:

> I want to install the `confluence-docs` skill. Follow the runbook at https://github.com/diegoclair/skills/blob/main/confluence-docs/reference/install-for-ai.md

The [`reference/install-for-ai.md`](./confluence-docs/reference/install-for-ai.md) is a runbook with deterministic exit codes and token-handling safety rules.

---

## Typical usage

```
You: where is the governance page?

Claude: Found it on Confluence:
- Governance — committee structure, board cadence, RACI
  https://mycompany.atlassian.net/wiki/spaces/<space>/pages/229891
```

The skill activates automatically when the prompt matches its scope (search, create, list, update, page status).

---

## Developing

```
skills/
├── <skill-name>/
│   ├── SKILL.md          # Frontmatter + instructions
│   ├── reference/        # Canonical spec, workflows, bootstrap
│   ├── cli/              # (optional) Go CLI the skill drives
│   ├── install/          # (optional) install.sh / install.ps1
│   └── bin/              # Generated by `make install` — gitignored
├── pkg/atlassian/                  # Shared Atlassian primitives (adf, setup, jira, release)
├── pkg/install/                    # Shared install.sh / install.ps1 (parameterized per skill)
└── .github/workflows/              # One release-<skill>.yml per skill (tagged <prefix>-v*)
└── README.md
```

Each skill is self-contained. No CLI? Skip `cli/` and `install/` — `SKILL.md` + `reference/` is the minimum. Release assets are produced by CI and never committed.

## Contributing

This repo is open-source and the skills here must work for any company that clones them. PR rules:

- **Skills must be company-agnostic.** No data specific to Lybel (or any other company) hardcoded in the skill body, in `reference/`, or in the CLI source. No people names, advisors, investors, partners, specific page IDs, instance URLs, product lists, etc.
- **Configurable defaults.** If a skill needs a value to function (cloud subdomain, root pageId, Atlassian instance), expose it via setup wizard, frontmatter, or environment variable. Document how to override.
- **"Home page is the source of truth" pattern.** For data that changes (taxonomy, indexes, lists of entities), the skill must **query the external system at runtime** (Confluence, Jira, etc.), not cache it in the repo. This is what keeps the repo timeless and safe to publish.
- **Acceptable exceptions:** README, CHANGELOG, and commit messages may freely mention Lybel — it's the maintaining company. Only the skill **content** has to stay generic.

Before opening a PR, grep your diff for company-specific leakage: `git diff main | grep -iE 'lybel|11C47E|164232'`. If anything shows up outside README / CHANGELOG / documented configurable defaults, refactor.

### Adding a new skill

1. Create `<name>/SKILL.md` following the [Claude Skills format](https://docs.claude.com/en/docs/claude-code/skills).
2. Put workflows / canonical specs in `<name>/reference/`.
3. If a CLI is needed, create `<name>/cli/` with `main.go` + `Makefile`.
4. To test locally without reinstalling on every change:
   ```bash
   ln -s "$(pwd)/<name>" ~/.claude/skills/<name>
   ```
   (Windows: `mklink /J`. Some AI sandboxes block symlinks in `~/.claude/skills/` — copy in that case.)
5. PR + tag `<prefix>-vX.Y.Z` (e.g. `confluence-v0.14.0`, `jira-v0.2.0`) → CI publishes the release for that skill. Each skill has its own `.github/workflows/release-<skill>.yml`; only one carries `make_latest: true` (currently `confluence-docs`).

### Conventions

- `name` field in frontmatter: lowercase with hyphens, max 64 chars.
- `description`: max 1024 chars, including triggers (phrases that activate the skill).
- Skill body in **English** (for Claude reasoning quality). The agent replies in whatever language the user wrote in.
- References use relative paths (`reference/foo.md`), never absolute URLs.

---

## License

[MIT](./LICENSE) © 2026 Lybel
