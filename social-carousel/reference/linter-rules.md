# Linter rules — the 30 codified rules

> When to read: when `social-carousel check` reports an issue you don't recognize, or when authoring custom logic and you want to know what the linter checks.

Each rule has: code, severity, the research behind it, and how to fix.

Severities:
- **ERR** — blocks render unless `--force` is passed.
- **WARN** — printed but doesn't block. Address before publishing.

## Universal rules (run on every slide / the whole carousel)

### U5 — Body contrast ≥ 4.5:1 — ERR
WCAG AA. Computed against the theme's `bg_primary` and `fg_primary`.
**Fix:** pick or author a theme with stronger contrast. Pure white on pure black = 21:1, easy.

### U7 — At most 2 font families — WARN
Theme uses `font_heading` + `font_body` (+ optional `font_quote` if used). Adding a 4th font is amateur signal.
**Fix:** drop `font_quote` if not needed, or merge heading + body into the same family.

### U9 — Words per slide ≤ 30 — ERR
Hard ceiling. If a slide reads like a paragraph, it belongs in the caption.
**Fix:** split into two slides, or move detail to `caption_seed`.

### U10 — Handle present — WARN
`carousel.handle` should be a non-empty string. Brand anchor.
**Fix:** add `handle: "@username"` at the top of the YAML.

### C3 — Cover hook contrast ≥ 7:1 — WARN
The slide 1 must be readable as a thumbnail. 4.5:1 is enough for body but not for cover, which gets compressed.
**Fix:** use a higher-contrast theme for the cover (or accept the warning).

## Carousel-level structural rules

### AN-01 — Slide count — WARN
Sweet spot 8–10. 4–7 has the worst drop-off (the hook promise is incomplete but the reader runs out of slides). 11–15 is OK. >15 is rare.
**Fix:** if under 8, add value/proof slides. If under 4, write a single image post.

### AP-01 — First slide must be cover — WARN
Algorithmic and cognitive: the cover layout has the visual hierarchy the feed expects.
**Fix:** make slide 1 layout `cover`.

### ST-05 — Slide 3 must be value bomb — WARN
80% of viewers who reach slide 3 finish the carousel. Slide 3 is the algorithmic inflexion point. Layouts that work as value bomb: `big-number`, `list`, `quote`. Layouts that fail there: `cover` (impossible — there's already one), `text` (no visual impact).
**Fix:** move your strongest data/insight to slide 3.

### AP-07 — Last slide must be CTA — WARN
The reader who reaches the end is the most engaged. Wasting the final slot with `text` or `quote` skips the conversion.
**Fix:** add a `cta` layout at the end.

### AP-04 — Slide 2 filler — WARN
Detection heuristic: if slide 2 is `text` and contains phrases like "in this carousel" or "I'll explain", linter warns. These phrases drain the hook's momentum.
**Fix:** rewrite slide 2 to deliver the first value or deepen the hook.

## Cover layout rules

### C1 — Hook ≤ 12 words — ERR
Past 12 words the hook stops being a hook and becomes a paragraph.
**Fix:** apply H-01 (numbers) or H-13 (listicle-with-spoiler) — both naturally compress.

### C6 — Sub ≤ 10 words — WARN
Microcopy line below the hook. Should be a single short sentence or "swipe →".

## List layout rules

### L1 — Items ≤ 5 per slide — ERR
6+ items on one slide reads as a paragraph and visually compresses each badge.
**Fix:** split into two list slides ("Items 1–4", "Items 5–7").

### L3 — Each item ≤ 8 words — WARN
Items should be scannable. >8 words and the eye doesn't catch the structure.

## Big-number layout rules

### N2 — Caption ≤ 6 words — ERR
Big-number's job is impact. Long captions dilute.

### N3 — Avoid theme background-effect — WARN
If the theme has `background_effect: dots` (or grid/halo), the big-number competes for visual attention with the pattern.
**Fix:** use a different theme for this slide (not yet supported per-slide; today: switch the carousel theme, or accept warning).

## Quote layout rules

### Q1 — Quote ≤ 25 words — ERR
Past 25 words quotes become essays.

### Q3 — Attribution required — ERR
Anonymous quotes lack the social proof that makes the layout worth using.
**Fix:** add `attribution: "— Name, Context"`.

## Comparison layout rules

### R2 — Items per side ≤ 2 — ERR
4+ items per side breaks the "stark contrast" effect.
**Fix:** if you have 4 contrasts, split into 2 comparison slides.

### CM-04 — Item-count parity — WARN
Both columns should land within 1 item of each other (1 vs 2 is fine, 0 vs 2 or 1 vs 3 is not). An asymmetric comparison reads as lopsided — the eye latches onto the extra row instead of the contrast.
**Fix:** add or remove an item so `len(before_items)` and `len(after_items)` stay within 1.

### CM-05 — Labels required for icon context — ERR
The comparison layout renders ✗ and ✓ icons next to each side. Without a textual label the icons read as decoration and the reader has to guess which side is the "good" one. (Replaces the legacy R1 "both sides present" rule, which checked the same condition with a less actionable message.)
**Fix:** label each side with a 1–3 word tag (e.g. `before_label: "Manual"`, `after_label: "Automated"`).

## Screenshot layout rules

### S2 — Caption present — WARN
A screenshot without context is just an image — no narrative payload.

### S3 — Caption ≤ 16 words — WARN
~2 lines max. More and it's a body slide with an image bolted on.

## CTA layout rules

### A1 — Single CTA verb — ERR
Heuristic: counts imperative-action keywords (`salva`, `comenta`, `manda`, `siga`, `compartilha`, `save`, `comment`, `share`, `follow`, `dm`) in `cta_text`. If more than one distinct verb is detected = ERR.
**Fix:** pick one verb. If two are truly complementary, you can combine with `"and"` ("Save and DM me") — the linter accepts that as one paired action.

### A2 — CTA verb specificity — WARN
`"Follow"` alone is generic. The linter warns and suggests specific action verbs.
**Fix:** "Comment [keyword]" or "DM me with X" or "Save to revisit [specific]".

### A5 — Headline ≤ 12 words — ERR
Same logic as C1 but for the CTA slide.

## Spotlight tone rules

### SP-01 — Spotlight tone overuse — WARN / ERR
`tone: spotlight` is meant as a single moment-of-pause interstitial — an accent-color full-bleed slide that breaks rhythm and tells the reader "stop, this matters". A second spotlight reads as decoration; three or more turns the device into noise and the reader stops registering it as a break.
- 1 spotlight → silent (intended use).
- 2 spotlights → **WARN** at the second occurrence.
- 3+ spotlights → **ERR** at every spotlight past the first.
**Fix:** keep one spotlight slide; demote the rest to inherited theme or another tone.

### SP-02 — Text+spotlight body ≤ 12 words — WARN
A `layout: text` + `tone: spotlight` slide renders the Body as a pull-quote, with oversized typography and decorative quotation marks. Past ~12 words the quotation marks crash into the text and the slide looks cramped — the opposite of the airy "pause" the spotlight is supposed to provide.
**Fix:** trim to a pull-quote-length line, or drop the spotlight tone and use a regular text slide.

## What the linter does NOT check

Visual / runtime rules that require the rendered HTML (and the renderer enforces structurally):

- C2: hook font-size ≥ 80 px (enforced by CSS class `.cover-hook`)
- C4: swipe indicator (rendered automatically by the cover layout)
- L2: badge size (CSS-fixed at 56 px)
- L4: gap between items (CSS-fixed at 32 px)
- N1: number font-size (CSS-fixed at 200–300 px)
- Q2: decorative quotation marks (rendered by quote layout)
- Q4: different font weight from body (theme's `font_quote` enforces)
- R3, R4: color tints for before/after (CSS hex values)
- R5: ✗ / ✓ icons (rendered by comparison layout)
- S1: screenshot file exists (validated by renderer, not linter — todo: lift into linter when YAML path is available)
- S4: mockup shadow (CSS-fixed)
- A3: swipe-back indicator (rendered when `swipe_back: true`)
- A4: handle + logo visible on CTA slide (footer is always rendered)
- U12: aspect ratio constant (guaranteed by single `platform` field per carousel)

## How to read a linter output line

```
✗ slide-3 [ST-05]: slide 3 is layout 'quote'; prefer big-number/list (algorithmic inflexion point)
```

- `✗` = ERR (blocks); `⚠` = WARN (informational)
- `slide-3` = 1-based slide index, or `slide-final`, or `carousel` for whole-carousel rules
- `[ST-05]` = rule code (this file is the index)
- Message ending with the underlying mechanic in parentheses

JSON output (via `social-carousel check --json`):

```json
{
  "issues": [
    {"code": "ST-05", "severity": "warn", "slide_idx": 2, "message": "..."}
  ],
  "err_count": 0,
  "warn_count": 1
}
```
