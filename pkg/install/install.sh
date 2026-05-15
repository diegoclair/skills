#!/bin/sh
# install.sh — POSIX shell installer shared by every skill in this monorepo.
#
# This file is NOT the entry point users hit directly. Each skill keeps a
# tiny stub at <skill>/install/install.sh that exports the three required
# environment variables and then exec's this file. Users always hit the
# per-skill URL, so the one-liner stays familiar:
#
#   curl -fsSL https://raw.githubusercontent.com/diegoclair/skills/main/<skill>/install/install.sh | bash
#
# The split exists to keep one canonical install pipeline (download release
# → atomic binary install → symlink to ~/.local/bin → PATH wiring → setup
# check → summary) instead of duplicating ~340 lines per skill. Adding a
# new skill is now a ~12-line stub.
#
# REQUIRED environment (exported by the stub before exec'ing this script):
#   SKILL_NAME         e.g. "jira-tickets" / "confluence-docs"
#   SKILL_TAG_PREFIX   e.g. "jira-v" / "confluence-v"   (with the trailing dash)
#   SKILL_REPO         e.g. "diegoclair/skills"
#
# OPTIONAL environment:
#   SKILL_VERSION      Specific release tag to install (default: resolve latest
#                      via GitHub API filtered by SKILL_TAG_PREFIX).
#   CLAUDE_HOME        Override Claude home dir (default: $HOME/.claude).

set -e

# ── required env validation ───────────────────────────────────────────────────

: "${SKILL_NAME:?SKILL_NAME must be exported by the calling stub}"
: "${SKILL_TAG_PREFIX:?SKILL_TAG_PREFIX must be exported by the calling stub}"
SKILL_REPO="${SKILL_REPO:-diegoclair/skills}"

# ── config ────────────────────────────────────────────────────────────────────

CLAUDE_HOME="${CLAUDE_HOME:-$HOME/.claude}"
SKILL_DIR="$CLAUDE_HOME/skills/$SKILL_NAME"
BIN_DIR="$SKILL_DIR/bin"
BIN_NAME="$SKILL_NAME"
GITHUB_BASE="https://github.com/$SKILL_REPO"

# ── helpers ───────────────────────────────────────────────────────────────────

die() { echo "error: $*" >&2; exit 1; }

download() {
  url="$1"
  dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 --retry-delay 2 -o "$dest" "$url" || die "download failed: $url"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$dest" "$url" || die "download failed: $url"
  else
    die "neither curl nor wget found; please install one and retry"
  fi
}

# ── detect OS and arch ────────────────────────────────────────────────────────

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Linux)  os="linux" ;;
  Darwin) os="darwin" ;;
  *)      die "unsupported OS: $OS" ;;
esac

case "$ARCH" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) die "unsupported architecture: $ARCH" ;;
esac

PLATFORM="${os}-${arch}"

# ── determine version ─────────────────────────────────────────────────────────
#
# Mono-repo nuance: this repo (diegoclair/skills) hosts multiple skills behind
# distinct tag prefixes (confluence-v*, jira-v*). The GitHub /releases/latest
# redirect is a singleton per repo — it points at whichever release was
# published with make_latest:true. So we can't follow it.
#
# Instead: list the recent releases via the GitHub API and pick the first one
# whose tag_name starts with SKILL_TAG_PREFIX. GitHub returns releases
# newest-first by default, so the first match is always the latest.

if [ -n "$SKILL_VERSION" ]; then
  VERSION="$SKILL_VERSION"
else
  RELEASES_API="https://api.github.com/repos/$SKILL_REPO/releases?per_page=30"
  UA="$SKILL_NAME-installer (https://github.com/$SKILL_REPO)"
  if ! command -v curl >/dev/null 2>&1; then
    die "curl not found; install curl or set SKILL_VERSION explicitly"
  fi

  # Capture headers + body + status in one request. Headers carry
  # x-ratelimit-reset (unix seconds), which we use to tell the user how
  # long until the limit resets — far more actionable than "wait an hour".
  HDR_FILE="$(mktemp)"
  BODY_FILE="$(mktemp)"
  HTTP_CODE="$(curl -sS --retry 2 --retry-delay 1 -A "$UA" \
    -D "$HDR_FILE" -o "$BODY_FILE" \
    -w '%{http_code}' "$RELEASES_API")" || HTTP_CODE="000"
  API_RESP="$(cat "$BODY_FILE" 2>/dev/null || true)"

  if [ "$HTTP_CODE" = "000" ] || [ -z "$HTTP_CODE" ]; then
    rm -f "$HDR_FILE" "$BODY_FILE"
    die "GitHub API call failed ($RELEASES_API). Network issue, or set SKILL_VERSION explicitly."
  fi

  if [ "$HTTP_CODE" -ge 400 ] 2>/dev/null; then
    # Rate-limit gets a dedicated, actionable message with reset time.
    if printf '%s' "$API_RESP" | grep -qi "rate limit"; then
      RESET="$(awk 'tolower($1)=="x-ratelimit-reset:" {print $2}' "$HDR_FILE" 2>/dev/null | tr -d '\r' | tail -1)"
      rm -f "$HDR_FILE" "$BODY_FILE"
      if [ -n "$RESET" ]; then
        NOW="$(date +%s)"
        MINS=$(( (RESET - NOW) / 60 + 1 ))
        [ "$MINS" -lt 1 ] && MINS=1
        die "GitHub API rate limit hit (60 req/hour unauthenticated). Resets in ~${MINS} min. Override: SKILL_VERSION=$SKILL_TAG_PREFIX<X.Y.Z> bash"
      fi
      die "GitHub API rate limit hit (60 req/hour unauthenticated). Override: SKILL_VERSION=$SKILL_TAG_PREFIX<X.Y.Z> bash"
    fi
    snippet="$(printf '%s' "$API_RESP" | head -c 200)"
    rm -f "$HDR_FILE" "$BODY_FILE"
    die "GitHub API returned HTTP $HTTP_CODE for $RELEASES_API. Response: $snippet"
  fi

  rm -f "$HDR_FILE" "$BODY_FILE"

  VERSION="$(printf '%s\n' "$API_RESP" \
    | grep -oE "\"tag_name\":[[:space:]]*\"${SKILL_TAG_PREFIX}[^\"]+\"" \
    | head -1 \
    | sed "s/.*\"\(${SKILL_TAG_PREFIX}[^\"]*\)\"/\1/")"
  if [ -z "$VERSION" ]; then
    case "$API_RESP" in
      *"\"tag_name\""*)
        die "no ${SKILL_TAG_PREFIX}* release found on $SKILL_REPO. The repo has releases under other prefixes only. Set SKILL_VERSION explicitly if you know the tag."
        ;;
      *)
        snippet="$(printf '%s' "$API_RESP" | head -c 200)"
        die "could not find any ${SKILL_TAG_PREFIX}* release on $SKILL_REPO. API response (first 200 chars): $snippet"
        ;;
    esac
  fi
fi

echo "Installing $SKILL_NAME $VERSION for $PLATFORM..."

# ── prepare directories ───────────────────────────────────────────────────────

mkdir -p "$BIN_DIR"

# ── download and extract release archive ──────────────────────────────────────
#
# The release archive (produced by .github/workflows/release-*.yml) is a
# single .zip per platform with this layout:
#
#   bin/$SKILL_NAME       (the binary; .exe on Windows)
#   SKILL.md              (skill entry point — read by Claude)
#   reference/*.md        (skill reference docs)

ARCHIVE="${BIN_NAME}-${PLATFORM}.zip"
DOWNLOAD_URL="$GITHUB_BASE/releases/download/$VERSION/$ARCHIVE"
TMP_DIR="$(mktemp -d)"
TMP_ARCHIVE="$TMP_DIR/$ARCHIVE"
EXTRACT_DIR="$TMP_DIR/extracted"
mkdir -p "$EXTRACT_DIR"

echo "  Downloading $DOWNLOAD_URL"
download "$DOWNLOAD_URL" "$TMP_ARCHIVE" || {
  echo "error: failed to download $DOWNLOAD_URL" >&2
  echo "       Archive saved to: $TMP_ARCHIVE" >&2
  exit 1
}

echo "  Extracting..."
extracted=0
if command -v unzip >/dev/null 2>&1; then
  if unzip -q -o "$TMP_ARCHIVE" -d "$EXTRACT_DIR"; then
    extracted=1
  fi
fi
if [ "$extracted" = "0" ] && command -v python3 >/dev/null 2>&1; then
  if python3 -c "import zipfile, sys; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])" \
       "$TMP_ARCHIVE" "$EXTRACT_DIR"; then
    extracted=1
  fi
fi
if [ "$extracted" = "0" ]; then
  die "extraction failed: archive at $TMP_ARCHIVE; install 'unzip' or 'python3' and retry"
fi

# Locate the binary inside the archive.
EXTRACTED_BIN="$EXTRACT_DIR/bin/$BIN_NAME"
if [ ! -f "$EXTRACTED_BIN" ]; then
  EXTRACTED_BIN="$(find "$EXTRACT_DIR" -type f -name "$BIN_NAME" | head -1)"
fi
[ -f "$EXTRACTED_BIN" ] || die "binary not found in archive; contents: $(find "$EXTRACT_DIR" | head -20)"

# Install the binary atomically.
#
# `cp` over the existing binary fails with "Text file busy" (ETXTBSY) when
# the binary is currently being executed — exactly what happens during
# `<skill> update`, which shells out to this script while running from
# $BIN_DIR/$BIN_NAME. The fix: cp into a sibling temp file in the SAME
# directory, then rename. rename(2) is allowed even on running executables.
TMP_BIN="$BIN_DIR/.${BIN_NAME}.new"
cp "$EXTRACTED_BIN" "$TMP_BIN"
chmod +x "$TMP_BIN"
mv -f "$TMP_BIN" "$BIN_DIR/$BIN_NAME"

# Install the skill payload (SKILL.md + reference/) bundled in the same archive.
SKILL_FILES_OK=0
if [ -f "$EXTRACT_DIR/SKILL.md" ]; then
  cp "$EXTRACT_DIR/SKILL.md" "$SKILL_DIR/SKILL.md"
  SKILL_FILES_OK=$((SKILL_FILES_OK + 1))
fi
if [ -d "$EXTRACT_DIR/reference" ]; then
  # Clean slate: drop any stale reference files from previous installs.
  rm -rf "$SKILL_DIR/reference"
  mkdir -p "$SKILL_DIR/reference"
  for f in "$EXTRACT_DIR/reference/"*.md; do
    [ -f "$f" ] || continue
    cp "$f" "$SKILL_DIR/reference/$(basename "$f")"
    SKILL_FILES_OK=$((SKILL_FILES_OK + 1))
  done
fi
echo "  Installed binary + $SKILL_FILES_OK skill file(s) from archive."

rm -rf "$TMP_DIR"

# ── put on PATH via ~/.local/bin symlink ──────────────────────────────────────
#
# Without this step, every Claude tool call has to use an absolute path,
# which is friction the LLM has to deal with on every invocation.

USER_BIN="$HOME/.local/bin"
SYMLINK="$USER_BIN/$BIN_NAME"

mkdir -p "$USER_BIN"
if [ -e "$SYMLINK" ] || [ -L "$SYMLINK" ]; then
  rm -f "$SYMLINK"
fi
if ln -s "$BIN_DIR/$BIN_NAME" "$SYMLINK" 2>/dev/null; then
  echo "  Symlinked: $SYMLINK -> $BIN_DIR/$BIN_NAME"
  PATH_LINK_OK=1
else
  echo "  warning: could not symlink to $SYMLINK (non-fatal)" >&2
  PATH_LINK_OK=0
fi

# Detect whether ~/.local/bin is on PATH.
case ":$PATH:" in
  *":$USER_BIN:"*) USER_BIN_ON_PATH=1 ;;
  *)               USER_BIN_ON_PATH=0 ;;
esac

# If not on PATH, persist an export line in the user's shell profile.
PATH_PROFILE_STATE=""
PATH_PROFILE_PATH=""

if [ "$USER_BIN_ON_PATH" = "0" ]; then
  case "${SHELL:-}" in
    */zsh)
      PATH_PROFILE_PATH="$HOME/.zshrc"
      ;;
    */bash)
      if [ "$(uname -s)" = "Darwin" ] && [ -f "$HOME/.bash_profile" ]; then
        PATH_PROFILE_PATH="$HOME/.bash_profile"
      else
        PATH_PROFILE_PATH="$HOME/.bashrc"
      fi
      ;;
    *)
      PATH_PROFILE_PATH="$HOME/.profile"
      ;;
  esac

  MARKER="# Added by $SKILL_NAME installer (https://github.com/$SKILL_REPO)"

  if [ -f "$PATH_PROFILE_PATH" ] && grep -Fq "$MARKER" "$PATH_PROFILE_PATH" 2>/dev/null; then
    PATH_PROFILE_STATE="already"
  elif [ -f "$PATH_PROFILE_PATH" ] && grep -Fq "$USER_BIN" "$PATH_PROFILE_PATH" 2>/dev/null; then
    PATH_PROFILE_STATE="already-mentioned"
  else
    {
      printf '\n%s\n' "$MARKER"
      printf 'export PATH="$HOME/.local/bin:$PATH"\n'
    } >> "$PATH_PROFILE_PATH" 2>/dev/null && \
      PATH_PROFILE_STATE="added" || \
      PATH_PROFILE_STATE="skipped"
  fi
fi

# ── verify installation ───────────────────────────────────────────────────────

echo ""
echo "Verifying installation..."
if ! "$BIN_DIR/$BIN_NAME" --version; then
  die "binary verification failed; binary at $BIN_DIR/$BIN_NAME may be corrupted"
fi

# ── check credentials (best-effort; skill may not expose `setup --check`) ─────

echo ""
echo "Checking credentials..."
set +e
"$BIN_DIR/$BIN_NAME" setup --check >/dev/null 2>&1
CHECK_CODE=$?
set -e

if [ "$CHECK_CODE" -eq 0 ]; then
  echo "  Already configured."
else
  echo "  Not yet configured."
  echo "  Run \`$BIN_DIR/$BIN_NAME setup\` to configure credentials,"
  echo "  or ask Claude to do it for you."
fi

# ── summary ───────────────────────────────────────────────────────────────────

echo ""
echo "Done. $SKILL_NAME $VERSION installed to:"
echo "  $BIN_DIR/$BIN_NAME"
echo ""
echo "Skill directory: $SKILL_DIR"

if [ "$PATH_LINK_OK" = "1" ]; then
  if [ "$USER_BIN_ON_PATH" = "1" ]; then
    echo ""
    echo "Ready to use: \`$SKILL_NAME --version\` from any directory."
  else
    echo ""
    echo "Symlink installed at: $SYMLINK"
    case "$PATH_PROFILE_STATE" in
      added)
        echo ""
        echo "Added to $PATH_PROFILE_PATH:"
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
        echo ""
        echo "Open a new terminal — or run \`source $PATH_PROFILE_PATH\` — to use \`$SKILL_NAME\` directly."
        ;;
      already)
        echo ""
        echo "$PATH_PROFILE_PATH already has the entry from a previous install."
        echo "Open a new terminal, or run: source $PATH_PROFILE_PATH"
        ;;
      already-mentioned)
        echo ""
        echo "Note: $USER_BIN is referenced in $PATH_PROFILE_PATH but not on your live \$PATH."
        echo "It may be commented out — uncomment it, or run:"
        echo "  export PATH=\"$USER_BIN:\$PATH\""
        ;;
      *)
        echo ""
        echo "Note: $USER_BIN is NOT on your \$PATH. Add this to your shell profile:"
        echo "  export PATH=\"$USER_BIN:\$PATH\""
        ;;
    esac
  fi
else
  echo ""
  echo "Symlink could not be created. Use the absolute path:"
  echo "  $BIN_DIR/$BIN_NAME --version"
fi
