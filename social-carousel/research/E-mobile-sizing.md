# E — Mobile Sizing: Typography and Proportions for 1080×1350 Carousels

> Research conducted in May 2026. Sources: Buffer, Hootsuite, Carouselli, PostNitro, UseVisuals, L-Visual, Carouselmaker, fontfyi, Material Design 3, Apple HIG, iosref.com.

---

## 1. Absolute Minimum Font Size (px on a 1080 canvas)

The benchmark most often cited by specialist carousel guides is that **anything rendered on a 1080 px canvas shows up at roughly a 1:2.9 to 1:2.88 scale on mobile** (iPhone display width ~375–393 logical pt, physical scale @3x). So `48 px` on the canvas translates to `~16–17 pt` on screen — exactly the minimum body text size that Apple HIG considers legible without zoom [(Apple Developer Typography)](https://developer.apple.com/design/human-interface-guidelines/typography).

### Recommended minimums by category

| Category | Absolute minimum (1080 px canvas) | Primary source |
|---|---|---|
| **Body / running text** | **32 px** | [Carouselli "LinkedIn Carousel Dimensions"](https://carouselli.com/blog/linkedin-carousel-dimensions) — "anything below 32px on a 1080px canvas becomes hard to read on a phone screen" |
| **Slide headline** | **48 px** | [Carouselli "LinkedIn Carousel Font"](https://carouselli.com/blog/linkedin-carousel-font) — floor for the headline of an inner slide |
| **Cover headline** | **60–72 px** | [UseVisuals "Ideal LinkedIn Carousel Aspect Ratio"](https://usevisuals.com/blog/ideal-linkedin-carousel-aspect-ratio-for-mobile) — minimum 60 pt on the canvas |
| **Label / tag / caption** | **20–22 px** | [Carouselli font guide](https://carouselli.com/blog/linkedin-carousel-font); Apple HIG minimum = 11 pt native → ~32 px canvas |
| **Slide number / micro** | **18–20 px** | [Carouselli font guide](https://carouselli.com/blog/linkedin-carousel-font) (18 px canvas = ~6 pt on screen — the edge of tolerable) |

**Why 32 px is the absolute floor for body text**: on a 1080 px canvas rendered at ~375 pt width on the iPhone, the scale is roughly 1:2.88. That makes `32 px canvas ≈ 11 pt on screen` — precisely the iOS Caption 2 floor [(Apple HIG)](https://developer.apple.com/design/human-interface-guidelines/typography). Material Design 3 sets 12 sp as the floor for labels [(M3 Type Scale)](https://m3.material.io/styles/typography/type-scale-tokens), which works out to ~35 px on the canvas.

**Rule of thumb cited by multiple guides**: shrink your slide preview down to 20% of its original size (which is roughly how it appears in the feed during a scroll). If you can read it without squinting, the font is sized appropriately. [(Carouselli Instagram Size Guide)](https://carouselli.com/blog/instagram-carousel-size)

---

## 2. How Much of the Canvas the Text Should Occupy

### Single sentence — what vertical % should it fill?

The emerging rule of thumb in viral design guides: **a single sentence (transition slide, pull-quote, bridge) should take up between 40% and 65% of the usable canvas height**, i.e. the area inside the padding. On a 1080×1350 canvas with 80 px of padding, the usable area is ~1190 px tall. So the text block should be **~476–773 px tall in total** (text + immediate breathing space).

The logic mirrors typographic poster design: if the designer has an entire canvas and drops a single sentence at 56 px with a max-width of 760 px, the text block occupies ~56 px out of ~1190 usable px = **less than 5%** of the height. That's invisible on mobile. Successful creators treat single-sentence slides as posters — the text fills the stage. [(PostNitro "15 Strategies for Viral Instagram Carousels")](https://postnitro.ai/blog/post/viral-instagram-carousels-strategies-2025)

### Horizontal occupation

Single-sentence text should occupy **70–85% of the usable canvas width**. With a 1080 px canvas and 80 px of side padding, the usable area is 920 px. The ideal max-width for a single sentence lands between **640–780 px** (70–85%).

### "Pull-quote scale" — rule or anti-pattern?

**A rule when executed well.** Carousel design guides confirm pull-quote scale (large centered text, few words) as a high-impact pattern because:
- It creates a rhythmic pause between denser slides
- It's naturally shareable (it feels like a notebook entry or viral quote)
- It increases time-on-slide

The anti-pattern is timid execution: a small centered sentence with lots of empty space reads as a design mistake, not deliberate minimalism. The line between "minimalism that works" and "an empty slide" is the font size. [(Social Champ "Carousel Slides Tips 2026")](https://www.socialchamp.com/blog/carousel-slides/)

---

## 3. Instagram vs LinkedIn

### Instagram

- Canvas: 1080×1350 px → rendered in the mobile feed at ~**393–414 CSS px wide** (the feed takes up nearly the full screen width in portrait).
- Implicit scale: `1080 / 393 ≈ 2.75x`. A `48 px` canvas text turns into ~`17.5 pt` on screen.
- **No zoom**: users don't zoom the Instagram feed. What you see is what you get.
- Time constraint: consumption is passive during scroll; text competes with UI motion.
- Result: **larger safety margins**, more aggressive size hierarchy, less body text per slide.

### LinkedIn (document post = PDF)

- Canvas: 1080×1350 px → rendered in the mobile viewer at ~**375 CSS px wide** [(Carouselli LinkedIn Dimensions)](https://carouselli.com/blog/linkedin-carousel-dimensions). Scale: `1080 / 375 ≈ 2.88x`.
- **Zoom available**: the LinkedIn document viewer on mobile allows pinch-to-zoom. LinkedIn users tend to tolerate higher content density and are more willing to zoom in to read.
- The PDF renders as a vector: text and edges stay crisp at any zoom level, with no aliasing.
- **That relaxes the font floor** by ~10–15%: while `32 px` is the absolute floor on Instagram, `28–30 px` can work for secondary elements on LinkedIn because the user *can* zoom.
- That said, the main guides recommend designing for legibility **without** relying on zoom, since most users won't bother [(Carouselli)](https://carouselli.com/blog/linkedin-carousel-dimensions): "over 70% of LinkedIn users are on their phones, and if your body text is under 18 px, most people won't bother pinch-zooming."
- CTAs and headlines should use the same floors as Instagram — there's no excuse to relax the rules where impact matters most.

### Practical summary of the differences

| Aspect | Instagram | LinkedIn |
|---|---|---|
| User zoom | Not available in feed | Yes (pinch-to-zoom in viewer) |
| Format | PNG/JPG (bitmap) | PDF (crisp vector) |
| Approx display width | ~393–414 CSS px | ~375 CSS px |
| Canvas → mobile scale | ~2.75x | ~2.88x |
| Body text floor | **32 px** (hard) | **28–30 px** (tolerable) |
| Headline floor | **48 px** | **48 px** |
| Audience behavior | Fast, passive scroll | More intentional, tolerates more text |

---

## 4. The Common Mistake: A Tiny Sentence in the Center of the Canvas

### The problem with a 56 px "transition" slide

The current `text` slide with a 56 px font, font-weight 500, vertically centered on a 1350 px tall canvas occupies **less than 5% of the usable height** for a single line of text. Scaled to mobile (÷2.88), that text shows up at ~19 pt — readable, but visually weak. The canvas looks like a post with a mistake: as if the designer placed the caption in the wrong spot.

### How viral creators handle "light" slides

**Justin Welsh** is the reference model for LinkedIn. His style uses:
- 1 idea per slide, never more than 30–50 words
- A large, headline-style font for statements
- Transition slides do exist, but **the font scales with the brevity of the content** — fewer words means a larger font. [(Welsh style analysis via Expandi)](https://expandi.io/blog/linkedin-carousel/)

The principle documented across multiple guides: **slides with less text should use a larger font, not a smaller one**. A single transition sentence should be treated like a poster, not like a footnote.

### Options for "light" transition slides

1. **Scale the font**: a short sentence (5–12 words) on a transition slide should use 96–120 px on the canvas, not 56 px. That makes the text fill 50–65% of the available width and creates visual impact.

2. **Add an anchor element**: a large chapter/slide number, a decorative line, a simple icon, or a colored background that contrasts with the rest of the deck. The visual element fills the space so the slide doesn't feel incomplete.

3. **Eliminate or merge**: if the sentence is just context/bridge for the next idea, consider folding it into the subtitle of the previous slide or the headline of the next one. Pure context-transition slides have low engagement because they don't deliver standalone value.

4. **Use a layout variation**: switch the alignment to left, add an accent background color, or use bold/display typography instead of weight 500 for the same sentence. That creates the poster effect without changing the size.

The condensed rule from the guides: **whitespace around small text = empty slide. Whitespace around large text = professional design.** [(Social Champ)](https://www.socialchamp.com/blog/carousel-slides/) [(BrandGhost LinkedIn Carousel Design)](https://blog.brandghost.ai/posts/linkedin-carousel-design-templates/)

---

## 5. Concrete px Recommendations by Layout

### Calculation baseline

Canvas: 1080×1350. Base padding: 80 px. Usable area: 920×1190 px.
Mobile scale factor: ~2.88x (LinkedIn) / ~2.75x (Instagram).

### Layout: `cover` (hook, slide 1)

| Element | Current | Minimum | Recommended | % of canvas height used |
|---|---|---|---|---|
| Hook headline | 96 px / w800 | 72 px | **96–120 px** | 40–55% of usable height |
| Subtitle/eyebrow | — | 32 px | **40–48 px** | — |

The full block (headline + subtitle + inner padding) should occupy **45–60% of the total canvas height** to fill the above-the-fold with impact. With a 96 px headline of 2–3 lines plus a subtitle, that target is already met.

### Layout: `text` (single transition sentence)

| Element | Current | Minimum | Recommended | % of canvas height |
|---|---|---|---|---|
| Main sentence | 56 px / w500 | 72 px | **96–128 px / w700–800** | 40–55% of the width, block ~30–40% of the height |

**Action required**: raise from 56 px to 96–128 px and swap weight 500 for 700–800. The current max-width of 760 px is appropriate for 5–12 words at the larger size.

### Layout: `list` (title + items)

| Element | Current | Minimum | Recommended |
|---|---|---|---|
| Slide title | 64 px | 56 px | **64–72 px** |
| List items | 36 px | 32 px | **36–40 px** |
| Bullet/number label | — | 20 px | **24–28 px** |

The `list` with 36 px items sits at the edge of acceptable (it scales to ~12–13 pt on mobile). Going up to 40 px is safer. The 64 px title is fine; it can climb to 72 px when the slide has only a few items.

### Layout: `big-number`

| Element | Current | Minimum | Recommended |
|---|---|---|---|
| Main number | 200–300 px | 180 px | **240–300 px** (keep) |
| Caption | 36–44 px | 32 px | **40–48 px** |
| Sub-caption | — | 24 px | **28–32 px** |

The 200–300 px hero number is correct — it's the visual entry point. The 36 px caption is at the minimum; pushing it to 44–48 px gives the explanatory context proper weight.

### Layout: `quote`

| Element | Current | Minimum | Recommended |
|---|---|---|---|
| Quote text | 48–60 px / italic | 48 px | **60–80 px** |
| Attribution | 28 px | 24 px | **32–36 px** |
| Decorative quote marks | — | visual only | 80–120 px ornamental |

A pull-quote should fill 50–65% of the usable width. With 7–15 words at 72–80 px, that's reached naturally in 2–3 lines. The 28 px attribution is below recommended — bump it up to 32 px.

### Layout: `cta` (headline)

| Element | Current | Minimum | Recommended |
|---|---|---|---|
| CTA headline | 64–72 px | 60 px | **72–88 px** |
| CTA sub/instruction | — | 32 px | **40–48 px** |

### Layout: `cta` (button/action box)

| Element | Current | Minimum | Recommended |
|---|---|---|---|
| Button text | — | 28 px | **36–44 px** |
| Button inner padding | — | 16–20 px v / 32–40 px h | keep the ratio |
| Minimum button-block height | — | 60 px | **72–80 px** |

The button / CTA box should be at least 60 px tall (= ~21 pt on screen = Material Design's 48 dp touch target). A minimum 36 px inner text size ensures both legibility and visual hierarchy.

---

## 6. LinkedIn Mobile: PDF Display Width

The LinkedIn document viewer on mobile displays content at **~375 CSS px wide** [(Carouselli LinkedIn Carousel Dimensions)](https://carouselli.com/blog/linkedin-carousel-dimensions). That's consistent with the iPhone's default design width (375 pt, used as the baseline across the iOS ecosystem).

### Template scaling math

```
Scale factor: 1080 / 375 = 2.88x

Examples:
  - 32 px canvas  →  11.1 pt on screen  (floor — legible but small)
  - 48 px canvas  →  16.7 pt on screen  (comfortable body text)
  - 64 px canvas  →  22.2 pt on screen  (comfortable slide headline)
  - 96 px canvas  →  33.3 pt on screen  (comfortable cover headline)
  - 128 px canvas →  44.4 pt on screen  (pull-quote / single sentence — impactful)
```

**Instagram** uses ~393 CSS px feed width (iPhone 15/16), so the scale factor is ~2.75x — slightly more forgiving for smaller fonts, but the difference is marginal (<5%) and doesn't justify separate templates.

**Practical takeaway**: a single set of sizes works for both platforms. Designing against the LinkedIn floor (375 pt) guarantees legibility on Instagram too.

---

## 7. TL;DR — Adjustment Table by Layout

| Layout | Element | Current value | Recommended value | Action |
|---|---|---|---|---|
| **cover** | hook headline | 96 px / w800 | 96–120 px / w800 | **keep** (can climb for 1–3 word hooks) |
| **text** | single sentence | 56 px / w500 | **96–128 px / w700–800** | **raise (critical)** |
| **text** | max-width | 760 px | 640–800 px | **keep** |
| **text** | centering | vertical center | vertical center + extra 10% top padding | optional fine-tune |
| **list** | title | 64 px | **64–72 px** | keep / optional raise |
| **list** | items | 36 px | **40–44 px** | **raise** |
| **big-number** | number | 200–300 px | 240–300 px | **keep** |
| **big-number** | caption | 36–44 px | **44–48 px** | **raise** |
| **quote** | quote text | 48–60 px / italic | **64–80 px** | **raise** |
| **quote** | attribution | 28 px | **32–36 px** | **raise** |
| **cta** | headline | 64–72 px | **72–88 px** | **raise** |
| **cta** | button text | — | **36–44 px** | define explicitly |
| **cta** | button height | — | **72–80 px minimum** | define explicitly |

### Rules of thumb to apply in CSS

1. **Absolute floor for any legible text**: `32 px` (Instagram, no zoom) / `28 px` (LinkedIn, zoom tolerated).
2. **Single-sentence slides**: minimum font `96 px`, preferably `112–128 px`. Treat them as typographic posters.
3. **Fewer words = larger font**: size should scale inversely with the amount of text on the slide.
4. **Attributions / labels / micro-text**: minimum `28–32 px` — never below 24 px for anything the user needs to read.
5. **The 20% test**: shrink the preview to 20% of its original size. If you can read it effortlessly, it passes.

---

## Sources Consulted

- [Carouselli — LinkedIn Carousel Font: Best Choices](https://carouselli.com/blog/linkedin-carousel-font)
- [Carouselli — LinkedIn Carousel Dimensions](https://carouselli.com/blog/linkedin-carousel-dimensions)
- [Carouselli — Instagram Carousel Size & Dimensions 2026](https://carouselli.com/blog/instagram-carousel-size)
- [UseVisuals — Ideal LinkedIn Carousel Aspect Ratio for Mobile](https://usevisuals.com/blog/ideal-linkedin-carousel-aspect-ratio-for-mobile)
- [L-Visual — LinkedIn Carousel Specs Guide (Safe Zones)](https://l-visual.com/blog/linkedin-carousel-specs-guide)
- [PostNitro — Carousel Typography: Font Sizes and Spacing](https://postnitro.ai/blog/post/carousel-typography-guide-perfecting-font-sizes-and-spacing)
- [PostNitro — 15 Strategies for Viral Instagram Carousels](https://postnitro.ai/blog/post/viral-instagram-carousels-strategies-2025)
- [TryMyPost — LinkedIn PDF Carousel Design Guide 2026](https://www.trymypost.com/blog/linkedin-pdf-carousel-design-guide-2026)
- [Social Champ — Carousel Slides Tips 2026](https://www.socialchamp.com/blog/carousel-slides/)
- [BrandGhost — LinkedIn Carousel Design Best Practices](https://blog.brandghost.ai/posts/linkedin-carousel-design-templates/)
- [Expandi — How to Create a LinkedIn Carousel in 2026](https://expandi.io/blog/linkedin-carousel/)
- [Pineable — Social Media Carousel Design Best Practices](https://pineable.com/blog/social-media-carousel-design-best-practices)
- [Fontfyi — Mobile Typography Accessibility: Minimum Font Sizes](https://fontfyi.com/blog/mobile-typography-accessibility/)
- [Apple Human Interface Guidelines — Typography](https://developer.apple.com/design/human-interface-guidelines/typography)
- [Material Design 3 — Type Scale Tokens](https://m3.material.io/styles/typography/type-scale-tokens)
- [iosref.com — iOS Device Resolution Table](https://iosref.com/res)
- [Buffer — Instagram Image Size Guide 2026](https://buffer.com/resources/instagram-image-size/)
- [Hootsuite — Social Media Image Sizes Guide May 2026](https://blog.hootsuite.com/social-media-image-sizes-guide/)
- [Oktopost — LinkedIn Carousel Best Practices 2026](https://www.oktopost.com/blog/linkedin-carousel-pdf-best-practices/)
