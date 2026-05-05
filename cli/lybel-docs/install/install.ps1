# install.ps1 — PowerShell 5+ installer for lybel-docs (Windows)
#
# Usage (one-liner):
#   iwr -useb https://raw.githubusercontent.com/lybel-app/skills/main/cli/lybel-docs/install/install.ps1 | iex
#
# Environment variables (all optional):
#   $env:LYBEL_DOCS_REPO      GitHub "owner/repo" (default: lybel-app/skills)
#   $env:CLAUDE_HOME          Override Claude home dir (default: $env:USERPROFILE\.claude)
#   $env:LYBEL_DOCS_VERSION   Specific release tag (default: latest)

#Requires -Version 5.0
$ErrorActionPreference = 'Stop'

# ── config ────────────────────────────────────────────────────────────────────

$Repo        = if ($env:LYBEL_DOCS_REPO)    { $env:LYBEL_DOCS_REPO }    else { 'lybel-app/skills' }
$ClaudeHome  = if ($env:CLAUDE_HOME)         { $env:CLAUDE_HOME }         else { Join-Path $env:USERPROFILE '.claude' }
$SkillDir    = Join-Path $ClaudeHome 'skills\lybel-docs'
$BinDir      = Join-Path $SkillDir 'bin'
$BinName     = 'lybel-docs.exe'
$GithubBase  = "https://github.com/$Repo"
$GithubRaw   = "https://raw.githubusercontent.com/$Repo/main"

# ── detect arch ───────────────────────────────────────────────────────────────

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64'  { 'amd64' }
    'x86'    { 'amd64' }  # 32-bit host is rare; download 64-bit
    'ARM64'  { 'arm64' }
    default  { 'amd64' }
}
$Platform = "windows-$Arch"

# ── determine version ─────────────────────────────────────────────────────────

if ($env:LYBEL_DOCS_VERSION) {
    $Version = $env:LYBEL_DOCS_VERSION
} else {
    # Follow the GitHub /releases/latest redirect to find the tag.
    try {
        $Resp = Invoke-WebRequest -Uri "$GithubBase/releases/latest" `
            -MaximumRedirection 0 -ErrorAction SilentlyContinue
        $Location = $Resp.Headers['Location']
        $Version = $Location -replace '.*/tag/', ''
    } catch {
        $Location = $_.Exception.Response.Headers['Location']
        if ($Location) {
            $Version = $Location -replace '.*/tag/', ''
        }
    }
    if (-not $Version) {
        Write-Error "Could not determine latest version. Set `$env:LYBEL_DOCS_VERSION explicitly."
        exit 1
    }
}

Write-Host "Installing lybel-docs $Version for $Platform..."

# ── prepare directories ───────────────────────────────────────────────────────

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

# ── download and extract binary ───────────────────────────────────────────────

$Archive     = "lybel-docs-$Platform.zip"
$DownloadUrl = "$GithubBase/releases/download/$Version/$Archive"
$TmpDir      = Join-Path $env:TEMP "lybel-docs-install-$(Get-Random)"
New-Item -ItemType Directory -Force -Path $TmpDir | Out-Null
$TmpArchive  = Join-Path $TmpDir $Archive

Write-Host "  Downloading $DownloadUrl"
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TmpArchive -UseBasicParsing
} catch {
    Write-Error "Download failed: $DownloadUrl`n$($_.Exception.Message)"
    exit 1
}

Write-Host "  Extracting..."
try {
    Expand-Archive -Path $TmpArchive -DestinationPath $TmpDir -Force
} catch {
    Write-Error "Extraction failed. Archive at: $TmpArchive`n$($_.Exception.Message)"
    exit 1
}

# The binary inside the zip is lybel-docs-windows-amd64.exe (or similar).
$ExtractedBin = Get-ChildItem -Path $TmpDir -Filter 'lybel-docs*.exe' | Select-Object -First 1
if (-not $ExtractedBin) {
    Write-Error "Binary not found in archive. Contents: $(Get-ChildItem $TmpDir | Select-Object -ExpandProperty Name)"
    exit 1
}

$Destination = Join-Path $BinDir $BinName
Copy-Item -Path $ExtractedBin.FullName -Destination $Destination -Force

# ── download SKILL.md + reference/ files ─────────────────────────────────────
#
# The skill source-of-truth lives at skills/lybel-docs/ in this repo (NOT
# cli/lybel-docs/). We fetch SKILL.md plus the reference files so the
# installed skill is complete on its own. Best-effort: warn but continue
# on individual file failures.

$SkillBase  = "$GithubRaw/skills/lybel-docs"
$SkillFilesOk = 0
$SkillFilesFailed = 0

Write-Host "  Syncing skill files..."

# SKILL.md (main entry)
try {
    Invoke-WebRequest -Uri "$SkillBase/SKILL.md" `
        -OutFile (Join-Path $SkillDir 'SKILL.md') -UseBasicParsing
    $SkillFilesOk++
} catch {
    Write-Warning "Could not download SKILL.md (non-fatal): $($_.Exception.Message)"
    $SkillFilesFailed++
}

# reference/ files
$RefDir = Join-Path $SkillDir 'reference'
New-Item -ItemType Directory -Force -Path $RefDir | Out-Null
foreach ($ref in @('bootstrap.md', 'aliases.md', 'taxonomy.md', 'templates.md', 'workflows.md')) {
    try {
        Invoke-WebRequest -Uri "$SkillBase/reference/$ref" `
            -OutFile (Join-Path $RefDir $ref) -UseBasicParsing
        $SkillFilesOk++
    } catch {
        Write-Warning "Could not download reference/$ref (non-fatal): $($_.Exception.Message)"
        $SkillFilesFailed++
    }
}

if ($SkillFilesFailed -eq 0) {
    Write-Host "  Synced $SkillFilesOk skill files."
} else {
    Write-Host "  Synced $SkillFilesOk skill files; $SkillFilesFailed failed (see warnings above)."
}

# ── verify installation ───────────────────────────────────────────────────────

Write-Host ""
Write-Host "Verifying installation..."
try {
    & $Destination --version
} catch {
    Write-Error "Binary verification failed. Binary at: $Destination"
    exit 1
}

# ── check credentials ─────────────────────────────────────────────────────────

Write-Host ""
Write-Host "Checking credentials..."
$CheckResult = & $Destination setup --check 2>&1
$CheckCode   = $LASTEXITCODE

if ($CheckCode -eq 0) {
    Write-Host "  Already configured."
} else {
    Write-Host "  Not yet configured."
    Write-Host "  Run ``$Destination setup`` to configure credentials,"
    Write-Host "  or ask Claude to do it for you."
}

# ── cleanup ───────────────────────────────────────────────────────────────────

Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue

# ── add to PATH (current session + persistent User PATH) ─────────────────────
#
# Without this, every Claude tool call has to use the absolute path to the
# binary, which is friction the LLM has to deal with on every invocation.
# We register the bin dir on the persistent User PATH so future shells (and
# Claude's Bash tool) see it automatically.

$env:PATH = "$BinDir;$env:PATH"

$PathRegistered = $false
try {
    $UserPath = [System.Environment]::GetEnvironmentVariable('PATH', 'User')
    if (-not $UserPath) { $UserPath = '' }
    # Idempotent: only add if not already present (case-insensitive match).
    $Already = $UserPath -split ';' | Where-Object { $_ -ieq $BinDir }
    if (-not $Already) {
        $NewPath = if ($UserPath) { "$BinDir;$UserPath" } else { "$BinDir" }
        [System.Environment]::SetEnvironmentVariable('PATH', $NewPath, 'User')
        $PathRegistered = $true
    } else {
        $PathRegistered = $true  # already registered = success
    }
} catch {
    Write-Warning "Could not register $BinDir on User PATH: $($_.Exception.Message)"
}

# ── summary ───────────────────────────────────────────────────────────────────

Write-Host ""
Write-Host "Done. lybel-docs $Version installed to:"
Write-Host "  $Destination"
Write-Host ""
Write-Host "Skill directory: $SkillDir"

if ($PathRegistered) {
    Write-Host ""
    Write-Host "Ready to use: ``lybel-docs --version`` from any new shell."
    Write-Host "(Current shell already has it on PATH.)"
} else {
    Write-Host ""
    Write-Host "Could not register PATH automatically. Add manually:"
    Write-Host "  [System.Environment]::SetEnvironmentVariable('PATH', `"$BinDir;`$env:PATH`", 'User')"
}
