# install.ps1 — PowerShell installer shared by every skill in this monorepo.
#
# This file is NOT the entry point users hit directly. Each skill keeps a
# tiny stub at <skill>/install/install.ps1 that sets the three required
# environment variables and then dot-sources this file. Users always hit the
# per-skill URL, so the one-liner stays familiar:
#
#   iwr -useb https://raw.githubusercontent.com/diegoclair/skills/main/<skill>/install/install.ps1 | iex
#
# REQUIRED environment (set by the stub before invoking this script):
#   $env:SKILL_NAME         e.g. "jira-tickets" / "confluence-docs"
#   $env:SKILL_TAG_PREFIX   e.g. "jira-v" / "confluence-v"
#   $env:SKILL_REPO         e.g. "diegoclair/skills"
#
# OPTIONAL environment:
#   $env:SKILL_VERSION      Specific release tag to install.
#   $env:CLAUDE_HOME        Override Claude home dir.

#Requires -Version 5.0
$ErrorActionPreference = 'Stop'

# ── required env validation ───────────────────────────────────────────────────

if (-not $env:SKILL_NAME)       { Write-Error "SKILL_NAME must be set by the calling stub"; exit 1 }
if (-not $env:SKILL_TAG_PREFIX) { Write-Error "SKILL_TAG_PREFIX must be set by the calling stub"; exit 1 }
if (-not $env:SKILL_REPO)       { $env:SKILL_REPO = 'diegoclair/skills' }

$SkillName   = $env:SKILL_NAME
$TagPrefix   = $env:SKILL_TAG_PREFIX
$Repo        = $env:SKILL_REPO

# ── config ────────────────────────────────────────────────────────────────────

$ClaudeHome  = if ($env:CLAUDE_HOME) { $env:CLAUDE_HOME } else { Join-Path $env:USERPROFILE '.claude' }
$SkillDir    = Join-Path $ClaudeHome (Join-Path 'skills' $SkillName)
$BinDir      = Join-Path $SkillDir 'bin'
$BinName     = "$SkillName.exe"
$GithubBase  = "https://github.com/$Repo"

# ── detect arch ───────────────────────────────────────────────────────────────

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64'  { 'amd64' }
    'x86'    { 'amd64' }
    'ARM64'  { 'arm64' }
    default  { 'amd64' }
}
$Platform = "windows-$Arch"

# ── determine version ─────────────────────────────────────────────────────────
#
# Mono-repo nuance: the /releases/latest redirect points at the make_latest:true
# release — singleton per repo. We list releases via the GitHub API and pick the
# first one whose tag_name starts with $TagPrefix.

if ($env:SKILL_VERSION) {
    $Version = $env:SKILL_VERSION
} else {
    $ReleasesApi = "https://api.github.com/repos/$Repo/releases?per_page=30"
    $UA = "$SkillName-installer (https://github.com/$Repo)"
    try {
        # Invoke-WebRequest (not -RestMethod) so we can inspect response
        # headers — x-ratelimit-reset lets us tell the user *when* the
        # limit resets instead of a vague "wait an hour".
        $Resp = Invoke-WebRequest -Uri $ReleasesApi -UseBasicParsing -UserAgent $UA
        $Releases = $Resp.Content | ConvertFrom-Json
        $Match = $Releases | Where-Object { $_.tag_name -like "$TagPrefix*" } | Select-Object -First 1
        if ($Match) {
            $Version = $Match.tag_name
        } elseif ($Releases) {
            Write-Error "No $TagPrefix* release found on $Repo. The repo has releases under other prefixes only. Set `$env:SKILL_VERSION explicitly if you know the tag."
            exit 1
        }
    } catch {
        $msg = $_.Exception.Message
        $resp = $_.Exception.Response
        $isRateLimit = $false
        $resetMins = $null
        if ($resp) {
            $body = ''
            try {
                $stream = $resp.GetResponseStream()
                $reader = New-Object System.IO.StreamReader($stream)
                $body = $reader.ReadToEnd()
                $reader.Close()
            } catch {}
            if ($body -match 'rate limit' -or $msg -match 'rate limit') {
                $isRateLimit = $true
                $resetHeader = $resp.Headers['X-RateLimit-Reset']
                if ($resetHeader) {
                    $resetSec = [int64]$resetHeader
                    $nowSec = [int64](Get-Date -UFormat %s)
                    $resetMins = [math]::Max(1, [math]::Floor(($resetSec - $nowSec) / 60) + 1)
                }
            }
        }
        if ($isRateLimit) {
            if ($resetMins) {
                Write-Error "GitHub API rate limit hit (60 req/hour unauthenticated). Resets in ~$resetMins min. Override: `$env:SKILL_VERSION=$TagPrefix<X.Y.Z>"
            } else {
                Write-Error "GitHub API rate limit hit (60 req/hour unauthenticated). Override: `$env:SKILL_VERSION=$TagPrefix<X.Y.Z>"
            }
        } else {
            Write-Error "GitHub API call failed: $msg"
        }
        exit 1
    }
    if (-not $Version) {
        Write-Error "Could not determine latest $TagPrefix* release on $Repo. Set `$env:SKILL_VERSION explicitly."
        exit 1
    }
}

Write-Host "Installing $SkillName $Version for $Platform..."

# ── prepare directories ───────────────────────────────────────────────────────

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

# ── download and extract binary ───────────────────────────────────────────────

$Archive     = "$SkillName-$Platform.zip"
$DownloadUrl = "$GithubBase/releases/download/$Version/$Archive"
$TmpDir      = Join-Path $env:TEMP "$SkillName-install-$(Get-Random)"
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

# Layout produced by release-*.yml:
#   bin/<skill>.exe  +  SKILL.md  +  reference/*.md
$ExtractedBin = Join-Path $ExtractDir (Join-Path 'bin' $BinName)
if (-not (Test-Path $ExtractedBin)) {
    $found = Get-ChildItem -Path $ExtractDir -Recurse -Filter "$SkillName*.exe" | Select-Object -First 1
    if ($found) { $ExtractedBin = $found.FullName }
}
if (-not (Test-Path $ExtractedBin)) {
    Write-Error "Binary not found in archive. Contents: $(Get-ChildItem $ExtractDir -Recurse | Select-Object -ExpandProperty FullName)"
    exit 1
}

$Destination = Join-Path $BinDir $BinName

# Install the binary atomically — Windows allows renaming a running .exe.
$OldDestination = "$Destination.old"
if (Test-Path $Destination) {
    if (Test-Path $OldDestination) {
        Remove-Item -Force $OldDestination -ErrorAction SilentlyContinue
    }
    try {
        Move-Item -Path $Destination -Destination $OldDestination -Force
    } catch {
        # fall through; Copy-Item below will surface the real error.
    }
}
Copy-Item -Path $ExtractedBin -Destination $Destination -Force

if (Test-Path $OldDestination) {
    Remove-Item -Force $OldDestination -ErrorAction SilentlyContinue
}

# Install the skill payload bundled in the same archive.
$SkillFilesOk = 0
$SkillMd = Join-Path $ExtractDir 'SKILL.md'
if (Test-Path $SkillMd) {
    Copy-Item -Path $SkillMd -Destination (Join-Path $SkillDir 'SKILL.md') -Force
    $SkillFilesOk++
}
$ExtractedRef = Join-Path $ExtractDir 'reference'
if (Test-Path $ExtractedRef) {
    $RefDir = Join-Path $SkillDir 'reference'
    if (Test-Path $RefDir) {
        Remove-Item -Recurse -Force $RefDir
    }
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

# ── check credentials (best-effort) ────────────────────────────────────────────

Write-Host ""
Write-Host "Checking credentials..."
$null = & $Destination setup --check 2>&1
$CheckCode = $LASTEXITCODE

if ($CheckCode -eq 0) {
    Write-Host "  Already configured."
} else {
    Write-Host "  Not yet configured."
    Write-Host "  Run ``$Destination setup`` to configure credentials,"
    Write-Host "  or ask Claude to do it for you."
}

# ── add to PATH ────────────────────────────────────────────────────────────────

$env:PATH = "$BinDir;$env:PATH"

$PathRegistered = $false
try {
    $UserPath = [System.Environment]::GetEnvironmentVariable('PATH', 'User')
    if (-not $UserPath) { $UserPath = '' }
    $Already = $UserPath -split ';' | Where-Object { $_ -ieq $BinDir }
    if (-not $Already) {
        $NewPath = if ($UserPath) { "$BinDir;$UserPath" } else { "$BinDir" }
        [System.Environment]::SetEnvironmentVariable('PATH', $NewPath, 'User')
        $PathRegistered = $true
    } else {
        $PathRegistered = $true
    }
} catch {
    Write-Warning "Could not register $BinDir on User PATH: $($_.Exception.Message)"
}

# ── summary ───────────────────────────────────────────────────────────────────

Write-Host ""
Write-Host "Done. $SkillName $Version installed to:"
Write-Host "  $Destination"
Write-Host ""
Write-Host "Skill directory: $SkillDir"

if ($PathRegistered) {
    Write-Host ""
    Write-Host "Ready to use: ``$SkillName --version`` from any new shell."
    Write-Host "(Current shell already has it on PATH.)"
} else {
    Write-Host ""
    Write-Host "Could not register PATH automatically. Add manually:"
    Write-Host "  [System.Environment]::SetEnvironmentVariable('PATH', `"$BinDir;`$env:PATH`", 'User')"
}
