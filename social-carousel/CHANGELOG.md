# Changelog

All notable changes to `social-carousel` will be documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- `Slide.tone` (optional) — per-slide visual tone override (`authority` / `clarity` / `spotlight`) for visual-rhythm rotation across a 10-slide deck (Research D §7, audit F-2.2). Slides without `tone` inherit the theme.
- `Slide.hook_style` (optional, cover only) — `gradient` opts in to the linear accent → accent_alt hook treatment. Default is solid.
- Linter rule **RH-01** (warning) — flags any run of 5+ consecutive slides sharing the same `layout` or `tone`.
- `data-tone` attribute on the slide root + base.css tone overrides (bg/fg + accent-derived primitives).

### Changed
- Cover gradient is now opt-in per slide via `hook_style: gradient` instead of activating automatically whenever the theme exposes `accent_alt`. Existing carousels that relied on the implicit gradient will now render with a solid accent hook; add `hook_style: gradient` on the cover slide to restore the previous look.

## [0.1.0] - 2026-05-17

Initial release.

### Added
- CLI binary `social-carousel` with subcommands: `render`, `check`, `new`, `preview`, `theme`, `setup`, `update`.
- YAML schema for carousels with 8 slide layouts: `cover`, `list`, `big-number`, `quote`, `comparison`, `screenshot`, `cta`, `text`.
- 3 platform targets: `instagram-4x5` (1080×1350 PNG), `instagram-1x1` (1080×1080 PNG), `linkedin-4x5` (1080×1350 PDF).
- Renderer via `chromedp` (headless Chrome, Go-native) at 2× device scale (final output 2160×2700).
- PDF combiner via `pdfcpu`.
- 5 design presets: `dark-tech`, `light-editorial`, `minimal-mono`, `neo-brutalist`, `duotone-deep`.
- `theme create` to author custom themes from an iterated carousel YAML; persisted to `~/.config/social-carousel/themes/`.
- 6 scaffold kinds in `new`: `listicle`, `case-study`, `framework`, `comparison`, `story`, `data-drop` — each lint-clean by construction.
- Linter (`check`) with 27 rules across universals, anti-patterns, and per-layout constraints. Blocks render unless `--force`.
- Embedded font assets pipeline: Outfit, DM Sans, Playfair Display, Space Grotesk, Inter, Noto Color Emoji (via `make fonts`).
- `preview` HTTP server for quick visual iteration without `chromedp`.
- Self-update via GitHub API filtered by `carousel-v*` tag prefix.
- Installer stub reusing `pkg/install/install.sh` shared across the monorepo.
