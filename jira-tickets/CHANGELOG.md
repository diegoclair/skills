# Changelog — jira-tickets

## v0.1.0 (TBD) — scaffold release

First entry into the `diegoclair/skills` monorepo. Reuses the shared
`pkg/atlassian` module (HTTP client, ADF parsing, credentials setup)
from `confluence-docs`.

### What ships in v0.1.0

- Skill structure: `SKILL.md` + `reference/` placeholders + `install/`
  scripts (mirror of `confluence-docs/install/` patterns)
- CLI: `setup`, `update`, `--version`, `--help` (Atlassian credentials
  shared with `confluence-docs` at `~/.config/atlassian/credentials`)
- Go module `github.com/diegoclair/skills/jira-tickets/cli` consuming
  `pkg/atlassian` via Go workspace + replace directive
- Release pipeline `release-jira.yml` triggered by `jira-v*` tags
- Tag convention: `jira-v0.1.0` (sibling to `confluence-v*` for the
  Confluence skill, both shipped from the same monorepo)

### What does NOT ship in v0.1.0

Real Jira commands. The agent contract documents that for read/write
operations the skill falls back to the Atlassian MCP until those
commands land. Subsequent minor versions fill them in:

- v0.2: `search "JQL"`, `issue digest`, `issue get`, `myself`, `project list`
- v0.3: `issue create`, `update`, `transition`, `comment`, `assign`, `link`
- v0.4: `epic add-child`, `sprint move`
- v0.5+: custom-field handling, attachments, batch ops

### Why the scaffold release exists

To validate the packaging path end-to-end — `go.work` resolving three
modules, the CI release pipeline producing `jira-v*` artifacts, the
install script copying both binary and skill payload to
`~/.claude/skills/jira-tickets/`, and the SKILL.md showing up to
Claude — before pouring effort into command implementation. If any
of that breaks, it's much cheaper to fix on an empty skill than on a
skill that already has 15 commands.
