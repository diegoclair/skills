# install.ps1 — PowerShell 5+ installer for lybel-docs (Windows)
#
# Usage (one-liner):
#   iwr -useb https://raw.githubusercontent.com/lybel-app/skills/main/skills/lybel-docs/install/install.ps1 | iex
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
$ExtractDir = Join-Path $TmpDir 'extracted'
New-Item -ItemType Directory -Force -Path $ExtractDir | Out-Null
try {
    Expand-Archive -Path $TmpArchive -DestinationPath $ExtractDir -Force
} catch {
    Write-Error "Extraction failed. Archive at: $TmpArchive`n$($_.Exception.Message)"
    exit 1
}

# Layout produced by .github/workflows/release.yml:
#   bin/lybel-docs.exe
#   SKILL.md
#   reference/*.md
$ExtractedBin = Join-Path $ExtractDir 'bin\lybel-docs.exe'
if (-not (Test-Path $ExtractedBin)) {
    # Fallback: recursive search in case the layout ever changes.
    $found = Get-ChildItem -Path $ExtractDir -Recurse -Filter 'lybel-docs*.exe' | Select-Object -First 1
    if ($found) { $ExtractedBin = $found.FullName }
}
if (-not (Test-Path $ExtractedBin)) {
    Write-Error "Binary not found in archive. Contents: $(Get-ChildItem $ExtractDir -Recurse | Select-Object -ExpandProperty FullName)"
    exit 1
}

$Destination = Join-Path $BinDir $BinName

# Install the binary atomically.
#
# Copy-Item -Force fails with "file in use" when the .exe is currently
# being executed — exactly what happens during `lybel-docs update`, which
# shells out to this script while running from $Destination. Windows DOES
# allow renaming a running .exe though (the live process keeps its handle).
# So: rename the running file out of the way first, then copy the new one
# in. The .old file is unlinked on a later run when nothing holds it.
$OldDestination = "$Destination.old"
if (Test-Path $Destination) {
    if (Test-Path $OldDestination) {
        Remove-Item -Force $OldDestination -ErrorAction SilentlyContinue
    }
    try {
        Move-Item -Path $Destination -Destination $OldDestination -Force
    } catch {
        # Rename failed (very rare); fall through to Copy-Item which will
        # surface the real error.
    }
}
Copy-Item -Path $ExtractedBin -Destination $Destination -Force

# Best-effort cleanup of the previous binary. Silently skip if the OS
# still has a handle on it (process still running) — next install/update
# will sweep it.
if (Test-Path $OldDestination) {
    Remove-Item -Force $OldDestination -ErrorAction SilentlyContinue
}

# Install the skill payload (SKILL.md + reference/) bundled in the same
# archive — no separate raw.githubusercontent.com fetches needed.
$SkillFilesOk = 0
$SkillMd = Join-Path $ExtractDir 'SKILL.md'
if (Test-Path $SkillMd) {
    Copy-Item -Path $SkillMd -Destination (Join-Path $SkillDir 'SKILL.md') -Force
    $SkillFilesOk++
}
$ExtractedRef = Join-Path $ExtractDir 'reference'
if (Test-Path $ExtractedRef) {
    $RefDir = Join-Path $SkillDir 'reference'
    New-Item -ItemType Directory -Force -Path $RefDir | Out-Null
    Get-ChildItem -Path $ExtractedRef -Filter '*.md' | ForEach-Object {
        Copy-Item -Path $_.FullName -Destination (Join-Path $RefDir $_.Name) -Force
        $SkillFilesOk++
    }
}
Write-Host "  Installed binary + $SkillFilesOk skill file(s) from archive."

Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue

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
