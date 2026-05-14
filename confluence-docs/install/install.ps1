# install.ps1 — bootstrap stub for the confluence-docs skill.
#
# Real install logic lives in pkg/install/install.ps1, shared by every skill
# in this monorepo. This stub just sets the three required parameters and
# fetches + invokes the shared installer. See pkg/install/install.ps1 for
# the full pipeline.

#Requires -Version 5.0
$ErrorActionPreference = 'Stop'

$env:SKILL_NAME = 'confluence-docs'
$env:SKILL_TAG_PREFIX = 'confluence-v'
if (-not $env:SKILL_REPO) { $env:SKILL_REPO = 'diegoclair/skills' }
# Back-compat: forward the legacy per-skill version override if set.
if ($env:CONFLUENCE_DOCS_VERSION -and -not $env:SKILL_VERSION) {
    $env:SKILL_VERSION = $env:CONFLUENCE_DOCS_VERSION
}

$SharedUrl = "https://raw.githubusercontent.com/$($env:SKILL_REPO)/main/pkg/install/install.ps1"

# Download + invoke. Invoke-Expression runs the script in the current scope
# so env vars set above are visible to it.
Invoke-Expression (Invoke-WebRequest -UseBasicParsing -Uri $SharedUrl).Content
