# social-carousel

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](../LICENSE)
[![social-carousel](https://img.shields.io/github/v/release/diegoclair/skills?filter=carousel-v*&color=11C47E&label=social-carousel)](https://github.com/diegoclair/skills/releases?q=tag%3Acarousel-v)

> Token-efficient CLI that turns a small YAML brief into a viral Instagram or LinkedIn carousel. Renders locally via headless Chrome (`chromedp`), validates copy against 27 research-backed rules before render, and ships 5 design presets + 7 layouts (cover, list, big-number, quote, comparison, screenshot, cta). Zero per-image cost, zero account, zero round-trip to paid SaaS APIs.

## Why it exists

Every existing carousel tool is agent-hostile:

| Tool | Why it fails for an autonomous agent |
|---|---|
| Canva API (Autofill) | Enterprise-only — minimum 30 members, contract via sales |
| Adobe Firefly API | $1k/month minimum + 80–120h integration before first render |
| Bannerbear / Placid / Templated | $0.03–$0.05 per image; templates created in their UI, not Git |
| Predis / aiCarousels / ContentDrips | UI-first, no native CLI, output not deterministic |
| Puppeteer scripts | Per-team boilerplate, no design system, no linter |

`social-carousel` is **CLI-first, offline, Git-versioned, free per render**. The schema is a YAML so terse an LLM agent fills it in a handful of tokens; the linter blocks the 27 most common viral-carousel mistakes before any render happens.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/diegoclair/skills/main/social-carousel/install/install.sh | bash
```

After install, run once:

```bash
social-carousel setup        # detects Chrome / Chromium on PATH
```

Chrome is **not bundled** (would be ~260 MB). Install Chrome (https://google.com/chrome) or Chromium first. On Linux CI: `apt-get install -y chromium-browser`.

For the typography presets to render with embedded fonts, download the fonts once:

```bash
git clone https://github.com/diegoclair/skills.git && cd skills/social-carousel
make fonts                   # downloads Outfit, DM Sans, Playfair, Space Grotesk, Inter, Noto Color Emoji
```

(Or skip this — the templates fall back to system sans-serif if the WOFF2 files are missing. The output will look generic; install the fonts for the real look.)

## Quick start

```bash
# Scaffold a 10-slide listicle (lint-clean by construction).
social-carousel new listicle --out my-carousel.yaml

# Edit the YAML: replace placeholder copy with real content.
$EDITOR my-carousel.yaml

# Validate against 27 viral-carousel rules.
social-carousel check my-carousel.yaml

# Render. Outputs 8–10 PNGs in `my-carousel.out/`.
social-carousel render my-carousel.yaml

# For LinkedIn (carousel-as-PDF):
social-carousel render my-carousel.yaml --pdf
```

## Schema in one glance

```yaml
platform: instagram-4x5   # instagram-4x5 | instagram-1x1 | linkedin-4x5
theme: dark-tech          # preset OR path to ~/.config/social-carousel/themes/*.yaml
handle: "@username"
slides:
  - layout: cover
    label: "SOLO SERVICE PROVIDER"
    hook: "7 mistakes that cost you 10 clients/month"   # ≤12 words enforced
    sub: "swipe →"
  - layout: big-number      # slide 3 = value bomb (algorithmic inflexion)
    number: "87%"
    caption: "lose clients due to missing follow-up"
  - layout: list
    title: "What's missing in your flow"
    items: ["D+7 reminder", "Client history", "Fixed schedule"]
  # ... more slides ...
  - layout: cta
    headline: "Want a system that does this for you?"
    cta_text: "Comment SYSTEM"
    swipe_back: true
caption_seed: "Suggested caption (printed when render completes)."
hashtags: ["#soloprovider"]
```

7 layouts available: `cover`, `list`, `big-number`, `quote`, `comparison`, `screenshot`, `cta`, `text`. See `reference/layouts.md` for required fields per layout.

## The linter — the real differentiator

```bash
$ social-carousel check my-carousel.yaml
✗ slide-1 [C1]: hook has 18 words (max 12) — H-01 or H-13 will compress it
✗ slide-3 [ST-05]: slide 3 is layout 'quote'; prefer big-number/list (algorithmic inflexion point)
⚠ slide-final [A2]: CTA "Follow" is generic — use "Save", "Comment X" or "DM me"

2 errors, 1 warning. Render blocked. Use --force to ignore.
```

The 27 rules encode published research from 15+ sources (Buffer, MarketingAgent, Carouselli, PostNitro, SearchEngineJournal, Influencer Marketing Hub, ThoughtLeadership.app, …). Each rule cites the underlying mechanic (algorithm signal, retention data, contrast threshold). See `reference/linter-rules.md`.

## Themes

5 presets ship embedded:

| Preset | Vibe |
|---|---|
| `dark-tech` | Authority. Black bg, Volt-Mint accent, Outfit + DM Sans. |
| `light-editorial` | Premium/wellness. Cream bg, terracotta accent, Playfair + DM Sans. |
| `minimal-mono` | Clarity. White bg, mono ink, Outfit + DM Sans. No accent color. |
| `neo-brutalist` | Bold. Yellow bg + black borders + offset shadows. Space Grotesk + Inter. |
| `duotone-deep` | Calm contrast. Deep-blue bg, soft-blue accent. Outfit + DM Sans. |

Custom theme:

```bash
# Author inline in a carousel YAML you liked, then promote it:
social-carousel theme create --from my-carousel.yaml --name mybrand
# → saved to ~/.config/social-carousel/themes/mybrand.yaml

# Reuse:
# theme: mybrand   (in any future carousel)
```

## Update / Uninstall

```bash
social-carousel update         # self-update via GitHub API (filtered by carousel-v* prefix)
social-carousel update --check # only check, don't install
rm -rf ~/.claude/skills/social-carousel ~/.config/social-carousel  # uninstall
```

## Status

v0.1.0 — Instagram (4:5 + 1:1) and LinkedIn (4:5 PDF) supported. TikTok Photo Mode + X multi-image are parked for v0.2.

See [CHANGELOG.md](CHANGELOG.md).
