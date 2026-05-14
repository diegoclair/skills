#!/bin/sh
# install.sh — POSIX shell installer for jira-tickets (Linux + macOS)
#
# Usage (one-liner):
#   curl -fsSL https://raw.githubusercontent.com/diegoclair/skills/main/jira-tickets/install/install.sh | bash
#
# Environment variables (all optional):
#   JIRA_TICKETS_REPO   GitHub "owner/repo" (default: diegoclair/skills)
#   CLAUDE_HOME       Override Claude home dir (default: $HOME/.claude)
#   JIRA_TICKETS_VERSION  Specific release tag (default: latest)

set -e

# ── config ────────────────────────────────────────────────────────────────────

REPO="${JIRA_TICKETS_REPO:-diegoclair/skills}"
CLAUDE_HOME="${CLAUDE_HOME:-$HOME/.claude}"
SKILL_DIR="$CLAUDE_HOME/skills/jira-tickets"
BIN_DIR="$SKILL_DIR/bin"
BIN_NAME="jira-tickets"
GITHUB_BASE="https://github.com/$REPO"
GITHUB_RAW="https://raw.githubusercontent.com/$REPO/main"

# ── helpers ───────────────────────────────────────────────────────────────────

die() { echo "error: $*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

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

if [ -n "$JIRA_TICKETS_VERSION" ]; then
  VERSION="$JIRA_TICKETS_VERSION"
else
  # Resolve the latest release tag via GitHub redirect.
  LATEST_URL="$GITHUB_BASE/releases/latest"
  if command -v curl >/dev/null 2>&1; then
    VERSION="$(curl -fsSL -o /dev/null -w '%{url_effective}' "$LATEST_URL" 2>/dev/null | sed 's|.*/tag/||')" || true
  fi
  if [ -z "$VERSION" ] || [ "$VERSION" = "$LATEST_URL" ]; then
    die "could not determine latest version; set JIRA_TICKETS_VERSION explicitly"
  fi
fi

echo "Installing jira-tickets $VERSION for $PLATFORM..."

# ── prepare directories ───────────────────────────────────────────────────────

mkdir -p "$BIN_DIR"

# ── download and extract release archive ──────────────────────────────────────
#
# The release archive (produced by the .github/workflows/release.yml workflow)
# is a single .zip per platform with this layout:
#
#   bin/jira-tickets        (the binary; .exe on Windows)
#   SKILL.md              (skill entry point — read by Claude)
#   reference/*.md        (skill reference docs)
#
# So one download + one extract gives us everything: the binary AND the skill
# payload. No separate fetches from raw.githubusercontent.com — the archive
# is self-contained, atomic, and version-pinned to the release tag.

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
  # Fallback for systems without unzip (rare on Linux/macOS but possible).
  if python3 -c "import zipfile, sys; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])" \
       "$TMP_ARCHIVE" "$EXTRACT_DIR"; then
    extracted=1
  fi
fi
if [ "$extracted" = "0" ]; then
  die "extraction failed: archive at $TMP_ARCHIVE; install 'unzip' or 'python3' and retry"
fi

# Locate the binary inside the archive. The workflow places it at bin/jira-tickets
# (or bin/jira-tickets.exe on Windows). Fall back to a recursive search if the
# layout ever changes.
EXTRACTED_BIN="$EXTRACT_DIR/bin/$BIN_NAME"
if [ ! -f "$EXTRACTED_BIN" ]; then
  EXTRACTED_BIN="$(find "$EXTRACT_DIR" -type f -name "$BIN_NAME" | head -1)"
fi
[ -f "$EXTRACTED_BIN" ] || die "binary not found in archive; contents: $(find "$EXTRACT_DIR" | head -20)"

# Install the binary atomically.
#
# `cp` over the existing binary fails with "Text file busy" (ETXTBSY) when
# the binary is currently being executed — exactly what happens during
# `jira-tickets update`, which shells out to this script while running from
# $BIN_DIR/$BIN_NAME. The fix: cp into a sibling temp file in the SAME
# directory, then rename. rename(2) is allowed even on running executables
# (the live process keeps using the old inode, now anonymous; the new file
# takes the path). Same-filesystem requirement is why the temp lives in
# $BIN_DIR, not next to $EXTRACTED_BIN (which is in $TMPDIR — likely a
# different filesystem on Linux).
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
  # Clean slate: drop any stale reference files from previous installs
  # (files that existed in older releases but were renamed or removed in
  # newer ones would otherwise linger forever, since the copy below only
  # overwrites and never deletes). Safe to run unconditionally here because
  # we already verified the new reference/ is in the extracted archive,
  # so we'll always repopulate immediately after.
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
# Without this step, every Claude tool call has to use an absolute path or a
# `cd`+relative path, which is friction the LLM has to deal with on every
# invocation. Symlinking into ~/.local/bin (which is on PATH by default on
# most modern Linux distros and macOS user setups) lets the agent just call
# `jira-tickets ...`.

USER_BIN="$HOME/.local/bin"
SYMLINK="$USER_BIN/$BIN_NAME"

mkdir -p "$USER_BIN"
# Replace any existing symlink/file at the target so re-runs are idempotent.
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

# Detect whether ~/.local/bin is on PATH so we can warn (or not).
case ":$PATH:" in
  *":$USER_BIN:"*) USER_BIN_ON_PATH=1 ;;
  *)               USER_BIN_ON_PATH=0 ;;
esac

# If not on PATH, persist an export line in the user's shell profile so future
# shells (and Claude's Bash tool) see the binary without further config. Keeps
# re-runs idempotent via a marker comment, and never touches a profile that
# already mentions $USER_BIN (might be intentionally commented out).
PATH_PROFILE_STATE=""   # added | already | already-mentioned | skipped
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

  MARKER="# Added by jira-tickets installer (https://github.com/$REPO)"

  if [ -f "$PATH_PROFILE_PATH" ] && grep -Fq "$MARKER" "$PATH_PROFILE_PATH" 2>/dev/null; then
    PATH_PROFILE_STATE="already"
  elif [ -f "$PATH_PROFILE_PATH" ] && grep -Fq "$USER_BIN" "$PATH_PROFILE_PATH" 2>/dev/null; then
    # User's profile already mentions ~/.local/bin (likely commented out).
    # Don't surprise them — leave a hint in the summary instead.
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

# Skill files (SKILL.md + reference/) are bundled inside the platform zip
# above and have already been extracted. No separate raw.githubusercontent.com
# fetches are needed — eliminates an entire round-trip and avoids version
# drift between the binary and its docs.

# ── verify installation ───────────────────────────────────────────────────────

echo ""
echo "Verifying installation..."
if ! "$BIN_DIR/$BIN_NAME" --version; then
  die "binary verification failed; binary at $BIN_DIR/$BIN_NAME may be corrupted"
fi

# ── check credentials ─────────────────────────────────────────────────────────

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

# ── cleanup ───────────────────────────────────────────────────────────────────

rm -rf "$TMP_DIR"

# ── summary ───────────────────────────────────────────────────────────────────

echo ""
echo "Done. jira-tickets $VERSION installed to:"
echo "  $BIN_DIR/$BIN_NAME"
echo ""
echo "Skill directory: $SKILL_DIR"

if [ "$PATH_LINK_OK" = "1" ]; then
  if [ "$USER_BIN_ON_PATH" = "1" ]; then
    echo ""
    echo "Ready to use: \`jira-tickets --version\` from any directory."
  else
    echo ""
    echo "Symlink installed at: $SYMLINK"
    case "$PATH_PROFILE_STATE" in
      added)
        echo ""
        echo "Added to $PATH_PROFILE_PATH:"
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
        echo ""
        echo "Open a new terminal — or run \`source $PATH_PROFILE_PATH\` — to use \`jira-tickets\` directly."
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
  echo "Or add to your shell profile:"
  echo "  export PATH=\"$BIN_DIR:\$PATH\""
fi
