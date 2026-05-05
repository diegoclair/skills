#!/bin/sh
# install.sh — POSIX shell installer for lybel-docs (Linux + macOS)
#
# Usage (one-liner):
#   curl -fsSL https://raw.githubusercontent.com/lybel-app/skills/main/cli/lybel-docs/install/install.sh | bash
#
# Environment variables (all optional):
#   LYBEL_DOCS_REPO   GitHub "owner/repo" (default: lybel-app/skills)
#   CLAUDE_HOME       Override Claude home dir (default: $HOME/.claude)
#   LYBEL_DOCS_VERSION  Specific release tag (default: latest)

set -e

# ── config ────────────────────────────────────────────────────────────────────

REPO="${LYBEL_DOCS_REPO:-lybel-app/skills}"
CLAUDE_HOME="${CLAUDE_HOME:-$HOME/.claude}"
SKILL_DIR="$CLAUDE_HOME/skills/lybel-docs"
BIN_DIR="$SKILL_DIR/bin"
BIN_NAME="lybel-docs"
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

if [ -n "$LYBEL_DOCS_VERSION" ]; then
  VERSION="$LYBEL_DOCS_VERSION"
else
  # Resolve the latest release tag via GitHub redirect.
  LATEST_URL="$GITHUB_BASE/releases/latest"
  if command -v curl >/dev/null 2>&1; then
    VERSION="$(curl -fsSL -o /dev/null -w '%{url_effective}' "$LATEST_URL" 2>/dev/null | sed 's|.*/tag/||')" || true
  fi
  if [ -z "$VERSION" ] || [ "$VERSION" = "$LATEST_URL" ]; then
    die "could not determine latest version; set LYBEL_DOCS_VERSION explicitly"
  fi
fi

echo "Installing lybel-docs $VERSION for $PLATFORM..."

# ── prepare directories ───────────────────────────────────────────────────────

mkdir -p "$BIN_DIR"

# ── download and extract binary ───────────────────────────────────────────────

ARCHIVE="${BIN_NAME}-${PLATFORM}.tar.gz"
DOWNLOAD_URL="$GITHUB_BASE/releases/download/$VERSION/$ARCHIVE"
TMP_DIR="$(mktemp -d)"
TMP_ARCHIVE="$TMP_DIR/$ARCHIVE"

echo "  Downloading $DOWNLOAD_URL"
download "$DOWNLOAD_URL" "$TMP_ARCHIVE" || {
  echo "error: failed to download $DOWNLOAD_URL" >&2
  echo "       Archive saved to: $TMP_ARCHIVE" >&2
  exit 1
}

echo "  Extracting..."
tar -xzf "$TMP_ARCHIVE" -C "$TMP_DIR" || {
  echo "error: extraction failed" >&2
  echo "       Archive at: $TMP_ARCHIVE" >&2
  exit 1
}

# The binary inside the archive is named lybel-docs-<platform>.
EXTRACTED_BIN="$TMP_DIR/${BIN_NAME}-${PLATFORM}"
if [ ! -f "$EXTRACTED_BIN" ]; then
  # Fallback: look for any file named lybel-docs* in the temp dir.
  EXTRACTED_BIN="$(find "$TMP_DIR" -maxdepth 1 -name "${BIN_NAME}*" ! -name "*.tar.gz" | head -1)"
fi
[ -f "$EXTRACTED_BIN" ] || die "binary not found in archive; contents: $(ls "$TMP_DIR")"

cp "$EXTRACTED_BIN" "$BIN_DIR/$BIN_NAME"
chmod +x "$BIN_DIR/$BIN_NAME"

# ── put on PATH via ~/.local/bin symlink ──────────────────────────────────────
#
# Without this step, every Claude tool call has to use an absolute path or a
# `cd`+relative path, which is friction the LLM has to deal with on every
# invocation. Symlinking into ~/.local/bin (which is on PATH by default on
# most modern Linux distros and macOS user setups) lets the agent just call
# `lybel-docs ...`.

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

# ── download SKILL.md + reference/ files ─────────────────────────────────────
#
# The skill source-of-truth lives at skills/lybel-docs/ in this repo (NOT
# cli/lybel-docs/, which holds the binary source). We fetch SKILL.md plus
# the reference files (workflows, taxonomy, aliases, templates, bootstrap)
# so the installed skill is complete on its own — Claude can read them
# without the user needing to clone the repo.
#
# These downloads are best-effort: if they fail (network blip, file moved),
# we warn but keep the install going. Existing files at the destination are
# overwritten so re-runs always sync to the latest version on main.

# fetch_optional URL DEST — uses curl/wget directly (not the strict download()
# helper that calls die on failure). Returns 0 on success, 1 otherwise.
fetch_optional() {
  url="$1"
  dest="$2"
  mkdir -p "$(dirname "$dest")"
  if command -v curl >/dev/null 2>&1; then
    if curl -fsSL --retry 2 --retry-delay 1 -o "$dest" "$url"; then
      return 0
    fi
  elif command -v wget >/dev/null 2>&1; then
    if wget -q -O "$dest" "$url"; then
      return 0
    fi
  fi
  return 1
}

SKILL_BASE="$GITHUB_RAW/skills/lybel-docs"
SKILL_FILES_OK=0
SKILL_FILES_FAILED=0

echo "  Syncing skill files..."

# SKILL.md (main entry)
if fetch_optional "$SKILL_BASE/SKILL.md" "$SKILL_DIR/SKILL.md"; then
  SKILL_FILES_OK=$((SKILL_FILES_OK + 1))
else
  echo "  warning: could not download SKILL.md (non-fatal)" >&2
  SKILL_FILES_FAILED=$((SKILL_FILES_FAILED + 1))
fi

# reference/ files
for ref in bootstrap.md aliases.md taxonomy.md templates.md workflows.md; do
  if fetch_optional "$SKILL_BASE/reference/$ref" "$SKILL_DIR/reference/$ref"; then
    SKILL_FILES_OK=$((SKILL_FILES_OK + 1))
  else
    echo "  warning: could not download reference/$ref (non-fatal)" >&2
    SKILL_FILES_FAILED=$((SKILL_FILES_FAILED + 1))
  fi
done

if [ "$SKILL_FILES_FAILED" -eq 0 ]; then
  echo "  Synced $SKILL_FILES_OK skill files."
else
  echo "  Synced $SKILL_FILES_OK skill files; $SKILL_FILES_FAILED failed (see warnings above)."
fi

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
echo "Done. lybel-docs $VERSION installed to:"
echo "  $BIN_DIR/$BIN_NAME"
echo ""
echo "Skill directory: $SKILL_DIR"

if [ "$PATH_LINK_OK" = "1" ]; then
  if [ "$USER_BIN_ON_PATH" = "1" ]; then
    echo ""
    echo "Ready to use: \`lybel-docs --version\` from any directory."
  else
    echo ""
    echo "Symlink installed at: $SYMLINK"
    echo ""
    echo "Note: $USER_BIN is NOT on your \$PATH. To enable the bare \`lybel-docs\`"
    echo "command, add this to your shell profile (~/.bashrc, ~/.zshrc, ...):"
    echo "  export PATH=\"$USER_BIN:\$PATH\""
    echo "Then start a new shell, or for the current shell run:"
    echo "  export PATH=\"$USER_BIN:\$PATH\""
  fi
else
  echo ""
  echo "Symlink could not be created. Use the absolute path:"
  echo "  $BIN_DIR/$BIN_NAME --version"
  echo "Or add to your shell profile:"
  echo "  export PATH=\"$BIN_DIR:\$PATH\""
fi
