---
name: social-carousel
version: 0.1.0
description: Generates viral Instagram and LinkedIn carousels from a small YAML brief using a local Go CLI. Drives headless Chrome via chromedp to produce PNGs (Instagram) or a combined PDF (LinkedIn) — zero per-image cost, zero account, zero round-trip to paid SaaS APIs. Ships with 5 design presets, 7 layout templates (cover, list, big-number, quote, comparison, screenshot, cta), and a linter that validates copy against 30 codified rules from viral-carousel research (slide-3 value bomb, ≤12-word hook, single CTA, contrast ≥4.5:1, etc.). Use this skill whenever the user asks to create, design, draft or generate a carousel for Instagram, LinkedIn, or any social platform — even when they don't explicitly say "carousel" but ask for a "post serie", "swipe post", "slides for X", or "8 slides about Y". Replies match the user's language and tone.
allowed-tools: |
  Bash(social-carousel *)
  Bash(make build)
  Bash(make fonts)
  Read
  Write
  Edit
---

# Social Carousel — Token-Efficient Carousel Generator

## Overview

`social-carousel` is a local CLI that turns a small YAML brief into a rendered Instagram/LinkedIn carousel. It exists because every alternative (Canva API = Enterprise-only, Bannerbear = $0.04/image, Predis/aiCarousels = UI-only) is fundamentally agent-hostile: they require accounts, browsers, or per-image billing, and none of them validate the copy against viral-carousel rules before rendering.

This skill solves all three problems with one binary:
- **HTML+CSS templates** versioned in Git, rendered by `chromedp` (headless Chrome). No SaaS account, no per-image cost.
- **5 design presets** + a `theme create` command to author custom themes from a brief you already iterated.
- **30-rule linter** (`check` command) that catches copy issues before render — `slide 3 must be value bomb`, `cta has 3 verbs`, `hook has 18 words (max 12)`. Blocks render unless `--force`. This is the single biggest differentiator over every SaaS competitor.

## Language rule

This document is in English because Claude reasons more robustly with English instructions. User-facing output, however, must match the user's language and tone (formal/informal). Slide content (hooks, copy, hashtags) stays in whatever language the user wrote — only technical scaffolding names (layout types, theme keys) stay in English.

## Four rules to internalize once

1. **Read `reference/examples.md` BEFORE writing slide copy.** Not "if unsure" — *always*. The linter encodes the WHAT (word counts, structure, contrast). It cannot encode the HOW-IT-LOOKS-WHEN-GOOD. `examples.md` carries 4 verified canonical carousels (Alić / Welsh / Broekema / Chris Do) with engagement numbers, hook formulas, and per-archetype layout sequences. Pick the archetype that matches the user's intent BEFORE scaffolding — otherwise you optimize for lint compliance and ship a slide that "looks fine but feels off." If you cannot name which reference your carousel is mirroring, you skipped this step.

2. **Always run `check` before `render`.** The linter encodes viral-carousel research that is not obvious from looking at a YAML. A YAML that "looks fine" can still fail rule `ST-05` (slide 3 should be a value bomb) or `A1` (CTA has multiple verbs). `render` runs `check` by default and blocks on errors. Do NOT pass `--force` to silence the linter unless the user explicitly asked you to ship a non-conforming carousel.

3. **Always ask the user where to save the output before rendering.** The CLI accepts `--out DIR` and defaults to a sibling folder of the input YAML, but that default is almost never what the user actually wants — most people want the PNGs/PDF inside the current project (so they can preview them in their IDE) or in their Documents folder (so they can drag-and-drop to Instagram). Ask once with a clear default suggestion:
   > "Where do you want to save the files? Default: inside the current project at `./carousel-out/`. Another common option: `~/Documents/social-carousel/<topic>/`. You can also name a different folder."

   Pass the chosen path with `--out <PATH>`. If the user is on WSL and asks for "Documents", translate to `/mnt/c/Users/<user>/Documents/...` automatically. If the user says "current project" or "here", use the directory you're invoked from. Don't ask again on subsequent renders in the same session unless the user changes topic.

4. **Headless Chrome is auto-downloaded** on first use (Chrome for Testing, ~80 MB). No sudo, no apt-get. On Linux only, it may require a one-time `apt-get install` for system libs (libnss3 et al.) — the `postinstall` command detects exactly which are missing and prints the minimal install command. macOS and Windows work zero-touch.

## Cross-cutting visual patterns (don't break these)

Every top reference in `examples.md` obeys these — the linter can't catch them, but the rendered slide will feel wrong if you violate them. Internalize and apply when writing copy:

1. **One idea per slide.** "Use a CRM **and** track follow-ups" is two ideas. Split.
2. **Single accent color.** One. Never two competing. The renderer enforces accent budget on layout primitives (big-number caption, cta_text) — don't fight it by piling on `**emphasis**`.
3. **Slide 3 is the surprising one.** If reading slide 3 doesn't make you stop, the carousel doesn't ship — even if `ST-05` passes.
4. **Numeric promise on the cover** if the carousel is a framework/listicle. "7 Steps" beats "Some Steps" every time.
5. **8–12 slides.** Verified high-engagement carousels cluster here. Outliers must justify length with TOC structure — don't pad to hit a count.
6. **Cover hook reads in <0.7s on a phone thumbnail.** If you can't read it muted on a 2-inch preview, it doesn't stop the scroll.

## Tool priority — CLI first, no MCP fallback

There is no MCP equivalent for carousel generation. The `social-carousel` Go CLI is the only tool needed; the operations and their token cost:

| Operation | CLI command | Token cost to agent |
|---|---|---|
| Scaffold a carousel | `social-carousel new <kind>` | input: ~10 tokens / output: ~800 tokens (one YAML) |
| Validate a carousel | `social-carousel check carousel.yaml` | output: 1 line per issue, no slide bodies returned |
| Render to PNG/PDF | `social-carousel render carousel.yaml` | output: file paths, no image bytes returned |
| List themes | `social-carousel theme list` | ~30 tokens |
| Show a theme | `social-carousel theme show dark-tech` | ~150 tokens (small YAML) |
| Create custom theme | `social-carousel theme create --from carousel.yaml --name mybrand` | ~30 tokens |

The agent never sees image bytes, never sees a full carousel body once the YAML is on disk, and never round-trips through a paid API.

## The YAML schema in one glance

```yaml
platform: instagram-4x5      # instagram-4x5 | instagram-1x1 | linkedin-4x5
theme: dark-tech             # preset name OR path to ~/.config/social-carousel/themes/*.yaml
handle: "@username"          # printed in footer; "" omits footer handle
logo: ./assets/logo.png      # optional, ≤60×60 px rendered
slides:
  - layout: cover            # one of: cover | list | big-number | quote | comparison | screenshot | cta | text
    label: "CATEGORIA"       # uppercase tag at the top
    hook: "Headline ≤12 words"
    sub: "microcopy line"
    tone: authority          # optional per-slide: authority | clarity | spotlight (rotation breaks monotony — RH-01)
    hook_style: gradient     # optional, cover only: solid (default) | gradient (needs theme.accent_alt)
  - layout: big-number       # cold-open slides should fill all four fields
    subhead: "WHAT THE NUMBER IS ABOUT"   # optional eyebrow ≤6 words
    number: "87%"
    caption: "of providers lose clients"   # short punchline ≤12 words
    context: "...due to missing follow-up systems."   # optional explainer ≤20 words
  - layout: comparison
    orientation: auto        # auto (default) | vertical | horizontal — auto picks vertical for ≤2 items/side
    before_label: "ANTES"
    before_items: ["..."]
    after_label: "DEPOIS"
    after_items: ["..."]
  - layout: cta
    headline: "Question or command ≤12 words"
    cta_text: "Comment X and I'll send you Y"
    swipe_back: true
caption_seed: |              # not rendered; printed by `render` for caption paste
  Suggested caption text.
hashtags: ["#foo", "#bar"]   # not rendered; printed by `render`
```

See `reference/schema.md` for the full per-field spec.

### Emphasis & layout knobs

- **Markdown emphasis** — wrap a key word or phrase with `**double asterisks**` in any prose field (`hook`, `sub`, `body`, `caption`, `context`, `headline`, list items, comparison items, quote, attribution). The wrapped text renders in the theme's accent color. Use once per slide to guide the eye; two+ highlights collapse the hierarchy.
  ```yaml
  body: "Provei a ideia nas docs. **E se a próxima skill gerasse carousel?** Saiu este post."
  ```
  **Accent budget:** the renderer auto-strips `**emphasis**` on fields where the layout primitive already uses accent — specifically `big-number.caption` (number owns the accent) and `cta.cta_text` (the button bg owns the accent). This is intentional, per Research D §3: one accent role per slide.
- **`big-number` carries four fields, not two** — `subhead` (eyebrow above the number), `number`, `caption` (short punchline), and `context` (explanatory line, ≤20 words). Use all four on a cold-open slide; use just `number + caption` for a proof slide near the end. The old "8-word caption max" rule is now a warning past 12 words — the [Broekema reference](reference/examples.md#reference-3--nick-broekema--meta-design-carousel-b2bsaas-founder-fit) routinely uses long contextual lines under the number.
- **Comparison `orientation`** — auto by default: ≤2 items/side renders **vertical** (antes on top, depois below, each filling full-width). ≥3 items renders **horizontal** (50/50 columns). Override with `orientation: vertical | horizontal` when you need to force one.
- **Length-responsive font sizes** — every prose layout (`cover`, `cta`, `quote`, `text`, `list`, `big-number` caption) scales font size by word count. Write naturally; the renderer picks the tier. A 4-word hook becomes a poster; a 12-word hook stays legible without wrapping to 4 lines.

### Before you ship — 4 taste questions

The linter checks compliance. It cannot check whether the carousel is *good*. After lint passes, do this 30-second self-audit:

1. **Read slide 1 aloud.** Does it stop you? If it sounds like every other post, rewrite the hook with a number, named entity, or counterintuitive claim. See [hook decision tree](reference/hooks.md).
2. **Read slide 3.** Is it the most surprising slide in the deck? If not, you wrote it to satisfy the value-bomb rule, not the reader. Swap with whichever slide *would* surprise.
3. **Skim slides 4–7.** Do they all sound alike? Mix layouts — alternate `list`/`comparison`/`big-number`/`text`. Same layout three+ slides in a row reads monotone.
4. **Read the CTA aloud.** Does it ask for ONE specific thing the reader can do in 5 seconds? "Save + comment + share + follow + DM me" gets zero of those actions. Pick one verb.

Anchor every carousel to one of the four archetypes in [reference/examples.md](reference/examples.md) — see rule 1 above. The four are: **Alić** (typographic system / pure visual ceiling), **Welsh** (numbered framework / listicle), **Broekema** (data-led B2B/SaaS operator playbook), **Chris Do** (Instagram type-heavy / pull-quote teaching).

## Quick workflows

### "Create a carousel about X"

1. **Pick a kind.** Map the user's intent to one of the 6 scaffolds:
   - `listicle` — "N things that…", "N mistakes that…", "N tips for…"
   - `framework` — "how to do X", "step-by-step for Y", "method Z"
   - `case-study` — "how I went from A to B", "what I learned from C"
   - `comparison` — "before vs after", "X is better than Y", counterintuitive takes
   - `story` — personal narratives, transformation ("I used to think…")
   - `data-drop` — "I analyzed N posts and discovered…", borrowed authority

2. **Scaffold the YAML.** `social-carousel new <kind> --out /tmp/c.yaml`. This produces a lint-clean skeleton with placeholder copy.

3. **Fill in real copy.** Edit the YAML directly. Keep these rules in mind (they ARE the linter):
   - Slide 1 (cover) hook ≤ 12 words.
   - Slide 3 should be `big-number`, `list`, or `quote` — never `cover` or `text` (algorithmic inflexion point).
   - Last slide must be `cta` with ONE verb (`save`, `comment X`, or `share` — not all three).
   - Max 30 words per slide, anywhere.
   - 8–10 slides is the sweet spot. 4–7 has the worst drop-off curve.

4. **Validate.** `social-carousel check /tmp/c.yaml`. Fix every red mark. Yellow warnings are OK but consider them.

5. **Ask where to save** (see rule #2 above). Don't render into `/tmp/` unless the user explicitly asked — they can't easily open files from `/tmp` in their IDE or attach to a draft.

6. **Render.** `social-carousel render carousel.yaml --out <user-chosen-path>`. Produces PNGs (Instagram) or PDF (LinkedIn). Path is printed on stdout.

7. **Print the caption.** The render command prints `caption_seed` and hashtags to stdout — paste those when uploading.

### "I want a custom theme for my brand"

1. **Pick a starting preset** that's visually close. List with `social-carousel theme show dark-tech`.
2. **Author an inline theme block** in a carousel YAML — replace the `theme: dark-tech` string with a mapping:
   ```yaml
   theme:
     name: my-brand
     bg_primary: "#0A1929"
     fg_primary: "#FFFFFF"
     accent: "#FFB84D"
     font_heading: "Outfit"
     font_body: "DM Sans"
   ```
3. **Iterate** by re-rendering. When you like it, persist: `social-carousel theme create --from carousel.yaml --name my-brand`. Saved to `~/.config/social-carousel/themes/my-brand.yaml`.
4. **Reuse** in any future carousel: `theme: my-brand`.

### "Quick preview without invoking Chrome"

`social-carousel preview /tmp/c.yaml --port 7777` — serves the rendered HTML on localhost. Iterate copy/theme without paying chromedp cold start every time. Stop with Ctrl+C.

## Gate 2 — visual QA after render

The 30-rule linter is **gate 1**: it catches WHAT (word counts, layout structure, structural compliance). The cross-cutting patterns and 4 taste questions above cover **gate 1.5**: editorial judgment on copy and structure before render. Gate 2 is narrower — it only owns artifacts that **the rendered pixels alone can reveal**.

**When to run gate 2:**

- The user signals ship-intent: "ship", "post", "final", "ready", "done", "publish", or equivalents in any language.
- OR the user asks to save outside `/tmp/` (rendering to a real project folder or Documents implies committing to this output).
- OR the carousel has ≥10 slides (volume increases the chance of cross-slide inconsistency the eye misses).

Skip gate 2 on disposable preview iterations during scaffolding — the user has not declared this version "the one".

**Who runs gate 2:**

- **Default — you, the parent agent, read the PNGs yourself**. You already have image-reading capability and the context of what was authored. No subagent dispatch, no extra latency, no token tax.
- **Subagent dispatch only on `--strict` flag OR carousels ≥10 slides** — the volume justifies fresh eyes and the parent's working window benefits from offloading 10+ image reads.

**The 5-item gate 2 checklist** (only things pixels reveal, not stuff already covered by gate 1 / 1.5):

1. **Text overflow / clipping / orphans** — any slide where words run off-canvas, clip the safe-area, or wrap into 5+ awkward lines? Include the offending slide number AND quote the visible text fragment.
2. **Font fallback / missing glyphs** — accents (PT/ES é í ã ç), special characters, em-dashes rendering as boxes or wrong family? Quote the affected slide and the character.
3. **Contrast in practice** — the linter computes theoretical contrast from theme tokens; the real render can be lower because of tints, overlays, or text-over-bg-effect. Any slide where you have to squint to read the body? Specify which text on which background.
4. **Cross-slide consistency** — accent color drifts (one slide uses theme accent, another a one-off color), label casing inconsistent (CAPS vs Title Case), footer position shifts, slide-number rendering changes. The most valuable check the linter cannot do.
5. **Whitespace balance** — top-heavy or bottom-heavy slide where the eye lands in a void. Cite which slide and where the dead space lives (top half, bottom-right quadrant, etc.).

**Severity: FIX / NOTE (binary, no middle ground).**

- **FIX** = re-render is needed before declaring ready. Requires grounded evidence: quote the exact text or describe the precise canvas region. No "feels off" — if you cannot quote it, do not call it FIX.
- **NOTE** = mention to the user as a known limitation, let them decide if it ships.

If no FIX and ≤2 NOTEs: declare ready, paste the caption_seed + hashtags, hand over.

**Anti-hallucination guardrail:** every FIX must include either a verbatim quote of text on the slide OR a specific canvas region (e.g. "top-right quadrant of slide 4"). A FIX without grounded evidence is wrong by default — re-check the image before reporting.

## Reference files

The body above is the entry point — daily use, mental model, common workflows. Detail lives in `reference/`.

- **`reference/examples.md` — 4 canonical reference carousels (Alić / Welsh / Broekema / Chris Do) with verified engagement, links, hook formulas, layout sequences, and YAML mapping. MANDATORY read before writing slide copy (see rule 1 above). Pick the archetype that matches the user's brief BEFORE scaffolding.**
- `reference/layouts.md` — when to use each of the 7 layouts; what fields are required vs optional; visual examples
- `reference/linter-rules.md` — every linter rule with code, severity, the research it codifies, and how to fix
- `reference/hooks.md` — the 13 hook formulas (H-01..H-13) plus a 4-question decision tree to narrow before you read all of them

## Brand-agnostic by design

This skill ships zero project-specific data. The 5 presets are intentionally generic (`dark-tech`, `light-editorial`, `cream-lifestyle`-style — none of them have "Lybel" colors hardcoded). Users author their own themes via `theme create`. The same skill works for a personal trainer, a B2B SaaS, or a content creator — only the theme YAML changes.

## What this skill is NOT for

- **Video carousels / Reels.** Use Reels / TikTok / CapCut. This skill renders static images.
- **Single posts (1 image).** Overkill. Use a regular image editor.
- **Carousel ideation from scratch.** Use Claude conversation to brainstorm the topic and structure; then use this skill to scaffold + render.
- **Brand kit management at enterprise scale.** Single-user / single-brand custom themes are supported, but multi-tenant brand systems should use a full DAM.
