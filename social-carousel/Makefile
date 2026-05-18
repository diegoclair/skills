# Makefile — social-carousel skill
# ----------------------------------
# Most-used targets:
#   make install-local   One-shot: fonts → build → install to ~/.claude/skills/social-carousel/ →
#                        symlink to ~/.local/bin/. No GitHub release needed. Works on Linux/macOS/WSL.
#   make uninstall-local Remove everything install-local created.
#
# Building blocks:
#   make fonts           Download all WOFF2/TTF assets to cli/templates/fonts/ (~12 MB).
#   make clean-fonts     Remove downloaded fonts.
#   make build           Compile binary to bin/social-carousel.
#   make test            Run all Go tests in cli/.
#   make check-chrome    Verify Chrome / Chromium is reachable; print per-OS install hint if not.

.PHONY: fonts clean-fonts build build-all test install-local uninstall-local check-chrome _ensure-fonts clean clean-dist

FONTS_DIR        := cli/templates/fonts
BIN_DIR          := bin
DIST_DIR         := cli/dist
SKILL_NAME       := social-carousel
TAG_PREFIX       := carousel-v
VERSION          ?= $(shell git describe --tags --match "$(TAG_PREFIX)*" --always --dirty 2>/dev/null || echo dev)
LDFLAGS          := -s -w -X main.version=$(VERSION)
CLAUDE_HOME      := $(HOME)/.claude
INSTALL_DIR      := $(CLAUDE_HOME)/skills/$(SKILL_NAME)
USER_BIN_DIR     := $(HOME)/.local/bin
SYMLINK          := $(USER_BIN_DIR)/$(SKILL_NAME)

# Sentinel: presence of Outfit-700.woff2 stands in for "fonts are downloaded".
FONTS_SENTINEL   := $(FONTS_DIR)/Outfit-700.woff2

# ─── fonts ────────────────────────────────────────────────────────────────────
#
# Downloads font files from google-webfonts-helper and renames them to the
# canonical names expected by base.css @font-face declarations.
# Also downloads Noto Color Emoji TTF from the googlefonts/noto-emoji repo.
#
# Requires: curl, unzip
fonts:
	@echo "→ Creating fonts directory…"
	@mkdir -p $(FONTS_DIR)

	@echo "→ Downloading Outfit (700, 800)…"
	@curl -fsSL "https://gwfh.mranftl.com/api/fonts/outfit?download=zip&subsets=latin&variants=700,800" \
		-o /tmp/sc-outfit.zip
	@unzip -jo /tmp/sc-outfit.zip "*.woff2" -d $(FONTS_DIR)
	@mv $(FONTS_DIR)/*outfit*700*.woff2  $(FONTS_DIR)/Outfit-700.woff2  2>/dev/null || true
	@mv $(FONTS_DIR)/*outfit*800*.woff2  $(FONTS_DIR)/Outfit-800.woff2  2>/dev/null || true
	@rm /tmp/sc-outfit.zip

	@echo "→ Downloading DM Sans (regular/400, 500, 700)…"
	@# gwfh API quirk: use `regular` (not `400`) — the numeric form silently drops the variant.
	@curl -fsSL "https://gwfh.mranftl.com/api/fonts/dm-sans?download=zip&subsets=latin&variants=regular,500,700" \
		-o /tmp/sc-dmsans.zip
	@unzip -jo /tmp/sc-dmsans.zip "*.woff2" -d $(FONTS_DIR)
	@mv $(FONTS_DIR)/*dm-sans*regular*.woff2  $(FONTS_DIR)/DMSans-400.woff2  2>/dev/null || true
	@mv $(FONTS_DIR)/*dm-sans*500*.woff2      $(FONTS_DIR)/DMSans-500.woff2  2>/dev/null || true
	@mv $(FONTS_DIR)/*dm-sans*700*.woff2      $(FONTS_DIR)/DMSans-700.woff2  2>/dev/null || true
	@rm /tmp/sc-dmsans.zip

	@echo "→ Downloading Playfair Display (700, 700italic)…"
	@curl -fsSL "https://gwfh.mranftl.com/api/fonts/playfair-display?download=zip&subsets=latin&variants=700,700italic" \
		-o /tmp/sc-playfair.zip
	@unzip -jo /tmp/sc-playfair.zip "*.woff2" -d $(FONTS_DIR)
	@mv $(FONTS_DIR)/*playfair*700italic*.woff2  $(FONTS_DIR)/PlayfairDisplay-700italic.woff2  2>/dev/null || true
	@mv $(FONTS_DIR)/*playfair*700*.woff2        $(FONTS_DIR)/PlayfairDisplay-700.woff2        2>/dev/null || true
	@rm /tmp/sc-playfair.zip

	@echo "→ Downloading Space Grotesk (600, 700)…"
	@curl -fsSL "https://gwfh.mranftl.com/api/fonts/space-grotesk?download=zip&subsets=latin&variants=600,700" \
		-o /tmp/sc-spacegrotesk.zip
	@unzip -jo /tmp/sc-spacegrotesk.zip "*.woff2" -d $(FONTS_DIR)
	@mv $(FONTS_DIR)/*space-grotesk*600*.woff2  $(FONTS_DIR)/SpaceGrotesk-600.woff2  2>/dev/null || true
	@mv $(FONTS_DIR)/*space-grotesk*700*.woff2  $(FONTS_DIR)/SpaceGrotesk-700.woff2  2>/dev/null || true
	@rm /tmp/sc-spacegrotesk.zip

	@echo "→ Downloading Inter (regular/400, 500)…"
	@curl -fsSL "https://gwfh.mranftl.com/api/fonts/inter?download=zip&subsets=latin&variants=regular,500" \
		-o /tmp/sc-inter.zip
	@unzip -jo /tmp/sc-inter.zip "*.woff2" -d $(FONTS_DIR)
	@mv $(FONTS_DIR)/*inter*regular*.woff2  $(FONTS_DIR)/Inter-400.woff2  2>/dev/null || true
	@mv $(FONTS_DIR)/*inter*500*.woff2      $(FONTS_DIR)/Inter-500.woff2  2>/dev/null || true
	@rm /tmp/sc-inter.zip

	@echo "→ Downloading Noto Color Emoji (TTF)…"
	@curl -fsSL \
		"https://github.com/googlefonts/noto-emoji/raw/main/fonts/NotoColorEmoji.ttf" \
		-o $(FONTS_DIR)/NotoColorEmoji-Regular.ttf

	@echo ""
	@echo "✓ Fonts downloaded:"
	@ls -1 $(FONTS_DIR)/*.woff2 $(FONTS_DIR)/*.ttf 2>/dev/null | sed 's|$(FONTS_DIR)/||'

# Internal: only run `fonts` if the sentinel is missing.
# Other targets depend on this so we never re-download by mistake.
_ensure-fonts:
	@if [ ! -f "$(FONTS_SENTINEL)" ]; then \
		echo "→ Fonts not found, downloading…"; \
		$(MAKE) --no-print-directory fonts; \
	else \
		echo "✓ Fonts already present at $(FONTS_DIR)/"; \
	fi

# ─── clean-fonts ──────────────────────────────────────────────────────────────
clean-fonts:
	@echo "→ Removing font assets…"
	@rm -f $(FONTS_DIR)/*.woff2 $(FONTS_DIR)/*.ttf
	@echo "✓ Fonts cleaned."

# ─── build ────────────────────────────────────────────────────────────────────
# Depends on _ensure-fonts so the bundled go:embed assets always include the
# fonts. (Without the fonts embedded, slides render in fallback sans-serif.)
build: _ensure-fonts
	@mkdir -p $(BIN_DIR)
	cd cli && go build -ldflags "$(LDFLAGS)" -o ../$(BIN_DIR)/$(SKILL_NAME) .
	@echo "✓ Built: $(BIN_DIR)/$(SKILL_NAME) ($(VERSION))"

# ─── build-all ────────────────────────────────────────────────────────────────
# Cross-compile for the 5 platforms our release ships. Fonts must already
# be present (CI runs `make fonts` before this target).
build-all: _ensure-fonts clean-dist
	@mkdir -p $(DIST_DIR)
	cd cli && GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(SKILL_NAME)-darwin-amd64    .
	cd cli && GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(SKILL_NAME)-darwin-arm64    .
	cd cli && GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(SKILL_NAME)-linux-amd64     .
	cd cli && GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(SKILL_NAME)-linux-arm64     .
	cd cli && GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(SKILL_NAME)-windows-amd64.exe .
	@echo "✓ Cross-built $(VERSION) for 5 platforms in $(DIST_DIR)/"

# ─── test ─────────────────────────────────────────────────────────────────────
test:
	cd cli && go test ./...

# ─── clean ────────────────────────────────────────────────────────────────────
clean: clean-dist
	rm -rf $(BIN_DIR)

clean-dist:
	rm -rf $(DIST_DIR)

# ─── check-chrome ─────────────────────────────────────────────────────────────
#
# Validates that headless Chrome is reachable. Prints the per-OS install hint
# on failure but never auto-installs (apt/brew need sudo; we won't ambush the
# user with privilege escalation).
check-chrome:
	@if command -v google-chrome >/dev/null 2>&1 || \
	    command -v google-chrome-stable >/dev/null 2>&1 || \
	    command -v chromium >/dev/null 2>&1 || \
	    command -v chromium-browser >/dev/null 2>&1; then \
		printf "✓ Chrome / Chromium found: "; \
		command -v google-chrome 2>/dev/null || \
		command -v google-chrome-stable 2>/dev/null || \
		command -v chromium 2>/dev/null || \
		command -v chromium-browser; \
	elif [ -x "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" ]; then \
		echo "✓ Chrome found at /Applications/Google Chrome.app/"; \
	else \
		echo "✗ Chrome / Chromium NOT found."; \
		echo ""; \
		os="$$(uname -s)"; \
		if [ "$$os" = "Linux" ]; then \
			echo "  Install on Debian/Ubuntu/WSL:"; \
			echo "    sudo apt-get update && sudo apt-get install -y chromium-browser"; \
			echo "  Or on Fedora/RHEL:"; \
			echo "    sudo dnf install -y chromium"; \
			echo "  Or Google Chrome stable:"; \
			echo "    https://www.google.com/chrome/"; \
		elif [ "$$os" = "Darwin" ]; then \
			echo "  Install on macOS:"; \
			echo "    brew install --cask google-chrome"; \
			echo "  Or download from https://www.google.com/chrome/"; \
		else \
			echo "  Install Google Chrome from https://www.google.com/chrome/"; \
		fi; \
		echo ""; \
		echo "  After installing, re-run: make install-local"; \
		exit 1; \
	fi

# ─── install-local ────────────────────────────────────────────────────────────
#
# One-shot local installer. Mirrors what pkg/install/install.sh does for a
# published release, but uses the locally-built binary instead of downloading
# from GitHub. Idempotent — safe to re-run after edits.
#
# Steps:
#   1. fonts (if missing)
#   2. build (Go binary with fonts embedded)
#   3. check-chrome (informational)
#   4. copy bin + SKILL.md + reference/ to ~/.claude/skills/social-carousel/
#   5. symlink ~/.local/bin/social-carousel -> the binary
#   6. ensure ~/.local/bin is on PATH (writes to shell profile if not)
#   7. social-carousel --version sanity check
#
# Re-run after code edits to push the new binary. Re-run with --no-rebuild
# style? Not yet — `make build` is fast enough (~5s with chromedp cached).
install-local: build
	@echo ""
	@echo "→ Installing $(SKILL_NAME) to $(INSTALL_DIR)…"
	@mkdir -p $(INSTALL_DIR)/bin
	@cp $(BIN_DIR)/$(SKILL_NAME) $(INSTALL_DIR)/bin/$(SKILL_NAME)
	@chmod +x $(INSTALL_DIR)/bin/$(SKILL_NAME)
	@cp SKILL.md $(INSTALL_DIR)/SKILL.md
	@rm -rf $(INSTALL_DIR)/reference
	@cp -r reference $(INSTALL_DIR)/reference
	@echo "  Installed binary + SKILL.md + reference/"
	@echo ""
	@echo "→ Symlinking $(SYMLINK) → $(INSTALL_DIR)/bin/$(SKILL_NAME)…"
	@mkdir -p $(USER_BIN_DIR)
	@rm -f $(SYMLINK)
	@ln -s $(INSTALL_DIR)/bin/$(SKILL_NAME) $(SYMLINK)
	@echo "  Symlinked."
	@echo ""
	@case ":$$PATH:" in \
	  *":$(USER_BIN_DIR):"*) \
	    echo "✓ $(USER_BIN_DIR) is already on PATH." ;; \
	  *) \
	    profile=""; \
	    case "$$SHELL" in \
	      */zsh)  profile="$$HOME/.zshrc" ;; \
	      */bash) \
	        if [ "$$(uname -s)" = "Darwin" ] && [ -f "$$HOME/.bash_profile" ]; then \
	          profile="$$HOME/.bash_profile"; \
	        else profile="$$HOME/.bashrc"; fi ;; \
	      *) profile="$$HOME/.profile" ;; \
	    esac; \
	    marker="# Added by $(SKILL_NAME) install-local"; \
	    if [ -f "$$profile" ] && grep -Fq "$$marker" "$$profile" 2>/dev/null; then \
	      echo "✓ PATH already wired in $$profile (open a new shell or 'source $$profile')."; \
	    else \
	      printf '\n%s\nexport PATH="$$HOME/.local/bin:$$PATH"\n' "$$marker" >> "$$profile" 2>/dev/null && \
	        echo "✓ Added PATH export to $$profile (open a new shell or 'source $$profile')." || \
	        echo "⚠ Could not write to $$profile. Add manually: export PATH=\"$$HOME/.local/bin:\$$PATH\""; \
	    fi ;; \
	esac
	@echo ""
	@echo "→ Verifying install…"
	@$(INSTALL_DIR)/bin/$(SKILL_NAME) --version
	@echo ""
	@echo "→ Running post-install checks (same as remote installer)…"
	@$(INSTALL_DIR)/bin/$(SKILL_NAME) postinstall || \
	    echo "  (post-install reported issues — see hints above; install itself succeeded)"
	@echo ""
	@echo "──────────────────────────────────────────────────────────────"
	@echo "✓ $(SKILL_NAME) installed locally."
	@echo ""
	@echo "  Binary:    $(INSTALL_DIR)/bin/$(SKILL_NAME)"
	@echo "  Skill:     $(INSTALL_DIR)/SKILL.md"
	@echo "  Symlink:   $(SYMLINK)"
	@echo ""
	@echo "  Open a new shell, then try:"
	@echo "    $(SKILL_NAME) --version"
	@echo "    $(SKILL_NAME) new listicle --out /tmp/test.yaml"
	@echo "    $(SKILL_NAME) check /tmp/test.yaml"
	@echo "    $(SKILL_NAME) render /tmp/test.yaml --out /tmp/test-out"
	@echo "──────────────────────────────────────────────────────────────"

# ─── uninstall-local ──────────────────────────────────────────────────────────
#
# Removes everything install-local created. Leaves fonts and the local build
# alone (they're inside the repo). User config (~/.config/social-carousel/)
# is preserved too — re-install gets back to the same state.
uninstall-local:
	@echo "→ Removing $(INSTALL_DIR)…"
	@rm -rf $(INSTALL_DIR)
	@echo "→ Removing symlink $(SYMLINK)…"
	@rm -f $(SYMLINK)
	@echo ""
	@echo "✓ Uninstalled. (Repo build/fonts and ~/.config/$(SKILL_NAME)/ left untouched.)"
