# Design System — Viral Carousels

> Research compiled: 2026-05-17. Sources: Adobe Blog, Canva, HubSpot, Postnitro, Pineable, Hootsuite, Typewolf, WebAIM, SocialInsider. Serves as the canonical spec for the carousel generation skill.

---

## 1. Specs by Platform

### Instagram Feed Carousel

| Format | Dimensions | Aspect Ratio | Recommended use |
|---|---|---|---|
| Square | 1080×1080 px | 1:1 | Clean grid, symmetric looks |
| Portrait (recommended) | 1080×1350 px | 4:5 | +33% feed real estate; higher engagement |
| Reels cover | 1080×1920 px | 9:16 | Cover only, never internal slides |

**Safe zones — Instagram 1080×1350:**
- Top: avoid the first **162 px** (top 15%) — cropped on profile grid
- Bottom: avoid the last **162 px** — overlapped by caption and buttons
- Sides: minimum **60 px** margin on each side
- Real safe zone for critical text: central area of **960×1026 px**
- 3:4 grid crop: if the first slide is 4:5, the grid crops to 3:4 — never place faces/logos/critical text in the top or bottom **270 px**

**Export:**
- Format: JPG (photos), PNG (slides with text/graphics)
- DPI: 72 dpi for web; export at **2×** (yields 2160×2700 px) for Retina sharpness
- Max 20 slides per carousel; sweet spot: **8–10 slides**
- Max file size per slide: not specified, keep below 10 MB to be safe

---

### LinkedIn Document Carousel

| Format | Dimensions | Aspect Ratio |
|---|---|---|
| Portrait (recommended) | 1080×1350 px | 4:5 |
| Square | 1080×1080 px | 1:1 |

**Safe zones — LinkedIn:**
- Minimum margin: **60 px** on all sides
- Safe text zone: **960×1230 px** (portrait) or **960×960 px** (square)
- Avoid the bottom **100 px** — the PDF progress bar overlays it

**Export:**
- Format: **PDF** (the only format accepted as a document post). PNG/JPG won't produce the swipeable carousel effect — they post as static images.
- DPI: **150 dpi** is the sweet spot: sharp without excessive weight
- Max 300 pages; max 100 MB — in practice keep below **10 MB total**
- Slide sweet spot: **6–12** (LinkedIn previews the first 3)
- A 10-slide carousel at 1080×1080 and 150 dpi should land under **5 MB**

---

### X / Twitter — Multi-image Post

X has no native swipeable carousel. It supports up to **4 images** per post with automatic layouts:

| Image count | Layout | Aspect ratio per image |
|---|---|---|
| 1 | Full width | 16:9 or 4:3 recommended |
| 2 | Side by side | 7:8 each |
| 3 | 1 large + 2 stacked | Large: 7:8 / Small: 4:7 |
| 4 | 2×2 grid | 2:1 each |

**Recommended dimensions per image:** 1200×675 px (X center-crops)
**Safe zone:** keep critical content in the **central 60%** — X aggressively crops the edges
**Format:** JPG or PNG. Max 5 MB on mobile, 15 MB on web. Above 5 MB it auto-compresses.
**Export:** 72 dpi, save as lossless PNG if it contains text

> X isn't a native carousel platform — use it for "visual threads" (4 images per tweet, multiple chained tweets). Low priority for skill v1.

---

### TikTok Photo Carousel (Photo Mode)

Launched in 2023–2024, TikTok Photo Mode supports carousels of static images with music.

| Format | Dimensions | Aspect Ratio |
|---|---|---|
| Recommended | 1080×1920 px | 9:16 |
| Alternate portrait | 1080×1350 px | 4:5 |
| Square (supported) | 1080×1080 px | 1:1 — letterboxed on mobile |

**Safe zones — TikTok 1080×1920:**
- Top: avoid the first **150 px** — user info and navigation
- Bottom: avoid the last **300–400 px** — caption, music, engagement buttons
- Safe text zone: **1080×1370 px** centered (from top 150 px to bottom 400 px)

**Export:**
- Format: JPG (photos), PNG (graphics with text)
- Max per image: 20 MB individual; 500 MB total per post
- Count: 4–35 images per carousel
- DPI: 72 dpi; export at 2× for Retina

---

## 2. Typography — CSS-Codifiable Rules

### Minimum sizes by hierarchy

All values below are for the **export canvas** (1080 px wide). Multiply by 0.5 to get the print-point equivalent.

```css
/* === HIERARCHY SCALE — canvas 1080px === */

/* Slide 1: Giant hook */
.slide-hook {
  font-size: 96px;          /* min: 80px; never below this */
  line-height: 1.1;
  letter-spacing: -0.02em;  /* tight on large headers */
  max-width: 900px;
  font-weight: 800;
}

/* Subtitle / microcopy on slide 1 */
.slide-hook-sub {
  font-size: 36px;
  line-height: 1.4;
  font-weight: 400;
  letter-spacing: 0.01em;
}

/* Title for intermediate slides */
.slide-title {
  font-size: 64px;          /* min: 48px */
  line-height: 1.15;
  font-weight: 700;
  letter-spacing: -0.01em;
}

/* Body for intermediate slides */
.slide-body {
  font-size: 36px;          /* HARD MINIMUM: 32px — never below */
  line-height: 1.5;
  font-weight: 400;
  letter-spacing: 0.005em;
  max-width: 820px;         /* ~45–55 chars/line = ideal readability */
}

/* Label / small caption (slide number, handle) */
.slide-label {
  font-size: 24px;          /* HARD MINIMUM for any element */
  line-height: 1.3;
  font-weight: 500;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

/* CTA slide */
.slide-cta-headline {
  font-size: 72px;
  font-weight: 800;
  line-height: 1.1;
}
```

**Golden rule:** if text is below **32 px on a 1080 px canvas** (roughly ~16 pt on a phone screen), it's illegible. Hard delete.

### Maximum words per slide

| Slide type | Max words |
|---|---|
| Hook (slide 1) | 12 |
| List item (each) | 8 |
| Big number (caption) | 6 |
| Body slide | 25 |
| Final CTA | 15 |
| Any slide | 30 (hard ceiling) |

### Recommended typography combos (Google Fonts, free)

All available via `fonts.googleapis.com`:

| Combo | Heading | Body | Personality |
|---|---|---|---|
| **A — Modern editorial** | Playfair Display 700–900 | DM Sans 400/500 | Premium, creative, storytelling |
| **B — Humanist tech** | Space Grotesk 600–700 | Inter 400/500 | SaaS, tech, education |
| **C — Clean & bold** | Barlow Condensed 800 | Barlow 400/500 | Fitness, impact, fast lists |
| **D — Friendly professional** | Outfit 700–800 | DM Sans 400 | Service provider, light B2B |
| **E — Serif/sans contrast** | Yeseva One | Karla 400/500 | Personal brand, lifestyle |
| **F — Pure geometric** | Syne 700–800 | Source Sans 3 400 | Design, creative, innovation |

**Montserrat 800 is burned out.** Avoid Roboto + Open Sans (default Google Docs look). Avoid Raleway in light weights (illegible on mobile).

### Line-height and letter-spacing by size

```css
/* General rule: the larger the font, the smaller the line-height */
font-size >= 80px  → line-height: 1.05–1.1;  letter-spacing: -0.02em
font-size 48–79px  → line-height: 1.1–1.2;   letter-spacing: -0.01em
font-size 32–47px  → line-height: 1.25–1.35; letter-spacing: 0em
font-size 24–31px  → line-height: 1.4–1.5;   letter-spacing: 0.01em
font-size < 24px   → forbidden in critical areas
```

---

## 3. Color and Contrast

### Accessibility minimums (WCAG AA)

```
Normal text (< 18pt / < 24px on canvas): minimum contrast 4.5:1
Large text (>= 18pt / >= 32px on canvas): minimum contrast 3:1
Interactive elements / icons: minimum 3:1
Main hook (slide 1): aim for 7:1+ — text must be readable as a thumbnail
```

**Verification tools:** [WebAIM Contrast Checker](https://webaim.org/resources/contrastchecker/)

### Attention-retaining combinations

| Style | Background | Primary text | Accent | When to use |
|---|---|---|---|---|
| **Dark high-contrast** | #0D0D0D or #111827 | #FFFFFF | Saturated brand color | Tech, finance, authority |
| **Cream editorial** | #F5F0E8 | #1A1A1A | Terracotta/rust/sage | Lifestyle, wellness, personal creator |
| **Duotone brand** | Brand primary | White or black | Neutral or lighter shade | Highlight slides, cover |
| **Light minimal** | #F8FAFC or #FFFFFF | #111827 | One accent color | Education, tips, clarity |
| **Mid-tone bold** | Mid tone (e.g. #1E3A5F) | #FFFFFF | Light contrasting color | CTA, big-number, urgency |

**Dark mode performs better** for authority/tech carousels. Light mode performs better for wellness/lifestyle. **Don't mix** the two inside the same carousel.

**Duotone:** uses two tones from the same family (e.g. dark indigo + saturated light indigo) — adds depth without complexity.

### Palettes for solo creator vs. established brand

**Solo creator (mono-brand):**
- 1 primary color + black/white + 1 accent color
- Background alternates between dark and light every 2–3 slides for visual rhythm
- The accent appears **only on keywords, numbers and CTAs** — never as a recurring background

**Brand (corporate palette):**
- Use brand colors as the background on highlight slides (cover, CTA, big-number)
- Body slides stay neutral (white/cream) with dark text — easier to read
- Never use more than 3 colors per slide

### How to highlight a keyword

```css
/* Technique 1: Highlighter strip */
.keyword-highlight {
  background: #FFE066;             /* highlighter yellow */
  color: #111827;
  padding: 2px 8px;
  border-radius: 4px;
  display: inline;
}

/* Technique 2: Color swap — word in accent color */
.keyword-accent {
  color: var(--brand-primary);     /* e.g. #11C47E */
  font-weight: 800;
}

/* Technique 3: Decorative underline (simulated stroke) */
.keyword-underline {
  text-decoration: underline;
  text-decoration-color: var(--brand-primary);
  text-decoration-thickness: 4px;
  text-underline-offset: 6px;
}

/* Technique 4: Box border (attention frame) */
.keyword-box {
  border: 3px solid var(--brand-primary);
  padding: 4px 12px;
  border-radius: 6px;
  display: inline-block;
}
```

---

## 4. Layout Patterns

All examples use a **1080×1350 px** canvas. Base padding: **80 px** on all sides.

### Cover (Slide 1) — Hook + Microcopy + Indicator

**Goal:** stop the scroll in < 0.7 seconds. The text must be legible as a thumbnail.

```
Visual layout:
┌─────────────────────────────────┐
│  [category label — 24px ALL]    │  ← top 120px, uppercase, spaced
│                                 │
│                                 │
│  GIANT HOOK                     │  ← upper-center, 96–144px, bold
│  IN UP TO 2 LINES               │
│  (max 12 words)                 │
│                                 │
│  Explanatory microcopy          │  ← 36px, weight 400, max 1 line
│                                 │
│  [handle / small logo]    →     │  ← bottom 80px; right arrow = swipe cue
└─────────────────────────────────┘
```

**Conceptual CSS rules:**
- Hook: `font-size: 96–144px; font-weight: 800; line-height: 1.05`
- Contrast: minimum **7:1** (the cover is seen as a compressed thumbnail)
- Swipe indicator: `→` arrow or chevron in the bottom-right corner, `opacity: 0.6`
- Never more than 12 words in the hook
- Never use a logo larger than 60×60 px — the hook is the star

---

### List Slide — Numbered list with badges

**Goal:** deliver value fast. Each item = one scannable line.

```
Visual layout:
┌─────────────────────────────────┐
│  Slide title (64px, bold)       │
│                                 │
│  [01]  First list item          │
│  [02]  Second list item         │
│  [03]  Third list item          │
│  [04]  Fourth item              │
│                                 │
│  [handle]                  [N/T]│
└─────────────────────────────────┘
```

**Rules:**
- Max 5 items per slide; if there are more, split into 2 slides
- Number badge: circle or rounded square, accent color, `width: 56px; height: 56px; font-size: 28px; font-weight: 700`
- Item text: `font-size: 36px; font-weight: 500; line-height: 1.3`
- Gap between items: `gap: 32px` (never less than 24 px)
- Optional divider between items: `border-bottom: 1px solid rgba(0,0,0,0.08)`

---

### Big-Number Slide — Outrageous number + short caption

**Goal:** immediate impact. The number must read before the context does.

```
Visual layout:
┌─────────────────────────────────┐
│                                 │
│                                 │
│         87%                     │  ← 200–300px, bold, accent color
│                                 │
│   of independent service        │  ← 40px, weight 400
│   providers lose clients        │
│   due to lack of follow-up      │
│                                 │
│  [handle]                  [N/T]│
└─────────────────────────────────┘
```

**Rules:**
- Main number: `font-size: 200–300px; font-weight: 900; color: var(--accent)`
- Caption: `font-size: 36–44px; max-width: 700px; line-height: 1.4`
- Max 6 words in the caption
- Background: neutral (white or near-black) — don't compete with the number
- Position: vertically centered, or number in the upper third and caption just below

---

### Quote Slide — Decorative quote marks + attribution

**Goal:** emotional connection, shareability, authority by association.

```
Visual layout:
┌─────────────────────────────────┐
│  "                              │  ← decorative quotes 200px, opacity: 0.1
│                                 │
│   The quote text                │  ← 48–60px, italic, line-height: 1.3
│   sits here across two          │
│   or three lines                │
│                                 │
│                    "            │
│                                 │
│   — Name, Role/Context          │  ← 28px, weight 500, accent color
│  [handle]                  [N/T]│
└─────────────────────────────────┘
```

**Rules:**
- Decorative quotes: `font-size: 180–240px; opacity: 0.08–0.12; color: var(--brand-primary)`; placed via `::before` / `::after` pseudo-elements
- Quote text: a different font from the rest of the carousel (if combo A = Playfair+DM Sans, use Playfair italic here)
- Attribution: `font-size: 28px; font-weight: 600; color: var(--accent)`; separate with em-dash `—`
- Max 3 lines of quote text
- Background: subtle texture or a soft radial gradient behind the text

---

### Comparison Slide — Two sides (before/after, wrong/right)

**Goal:** clarity through contrast. The "wrong" reinforces the "right".

```
Visual layout:
┌──────────────┬──────────────────┐
│  ✗ BEFORE    │  ✓ AFTER         │
│  (light      │  (light          │
│  red         │  green           │
│  background) │  background)     │
│              │                  │
│  Short       │  Short           │
│  negative    │  positive        │
│  description │  description     │
└──────────────┴──────────────────┘
```

**Rules:**
- Split: exactly centered (50/50) with a divider line or `8px` gap
- Bad side: background `#FFF1F0` or `rgba(239,68,68,0.08)` — never pure red
- Good side: background `#F0FFF4` or `rgba(16,185,129,0.08)` — never pure green
- ✗ / ✓ icon: `font-size: 48px` at the top of each column
- Label: `font-size: 24px; font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase`
- Text: `font-size: 36px; line-height: 1.4`
- Never more than 2 differences per slide — if you have 4, use 2 comparison slides

---

### Screenshot Slide — Device mockup + caption

**Goal:** social proof, "show don't tell", concreteness.

```
Visual layout:
┌─────────────────────────────────┐
│  Short title (48px)             │
│                                 │
│  ┌─────────────────────────┐    │
│  │   [screenshot inside    │    │  ← iPhone/browser mockup
│  │    the device mockup]   │    │
│  └─────────────────────────┘    │
│                                 │
│  Caption explaining what        │  ← 32px, max 2 lines
│  the screenshot shows           │
│  [handle]                  [N/T]│
└─────────────────────────────────┘
```

**Rules:**
- Mockup: use a PNG device frame (iPhone, browser, Android) with shadow `box-shadow: 0 32px 80px rgba(0,0,0,0.25)`
- Screenshot inside the mockup must be sharp — use 2× resolution
- Caption: `font-size: 32px; font-weight: 400; color: var(--text-secondary)`
- Never place text over the screenshot — always below or beside the mockup
- Background: a neutral color that contrasts with the mockup (white for a dark mockup, dark for a white one)

---

### CTA Slide (Last) — Question + Action + Swipe-back indicator

**Goal:** convert attention into action. This slide is the "conversion point".

```
Visual layout:
┌─────────────────────────────────┐
│                                 │
│   Do you want [outcome]         │  ← question, 64–72px, bold
│   without [pain]?               │
│                                 │
│  ┌─────────────────────────┐    │
│  │  Save this carousel    │    │  ← button/box, accent color
│  │  and DM me ✉          │    │
│  └─────────────────────────┘    │
│                                 │
│  ← Back to the start            │  ← left arrow, 24px, opacity: 0.5
│  [handle]    [logo]             │
└─────────────────────────────────┘
```

**Rules:**
- Headline: `font-size: 64–72px; font-weight: 800; line-height: 1.1`; max 12 words
- Action box: `background: var(--brand-primary); color: white; border-radius: 16px; padding: 24px 40px`
- One single CTA per slide — never two
- Swipe-back indicator: `←` arrow or "Back to slide 1" — reminds that the algorithm rewards revisits
- Handle and logo always visible — this slide gets shared the most
- Don't use a generic CTA ("Follow the profile") — use a specific action verb ("Save", "Comment", "DM me")

---

## 5. Visual Tricks — Motion Feel Without GIFs

### Gradients as depth

```css
/* Radial "halo" gradient behind a highlight element */
.halo-effect {
  background: radial-gradient(
    ellipse 60% 40% at 50% 50%,
    rgba(17, 196, 126, 0.15) 0%,   /* brand color, low opacity */
    transparent 70%
  );
}

/* Linear gradient for directional backgrounds */
.gradient-bg {
  background: linear-gradient(
    135deg,
    #0D1117 0%,
    #1A2332 50%,
    #0D1117 100%
  );
}

/* Gradient on text (hero) */
.gradient-text {
  background: linear-gradient(90deg, #11C47E, #0EA5E9);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}
```

**Rule:** background gradient = monochromatic (same color family). Text gradient = reserved for high-impact hooks, never on body copy.

### Dotted / Grid background

```css
/* Subtle dots */
.dot-bg {
  background-image: radial-gradient(circle, rgba(255,255,255,0.12) 1px, transparent 1px);
  background-size: 32px 32px;
}

/* Grid lines */
.grid-bg {
  background-image:
    linear-gradient(rgba(255,255,255,0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255,255,255,0.04) 1px, transparent 1px);
  background-size: 60px 60px;
}
```

**Use sparingly:** dots and grids disappear in thumbnails — they're details that only show up at full size. Don't place them on a light background (contrast too weak).

### Shadows: realistic vs. flat

```css
/* Realistic shadow — for device mockups, floating cards */
.shadow-realistic {
  box-shadow:
    0 4px 6px rgba(0,0,0,0.07),
    0 10px 15px rgba(0,0,0,0.10),
    0 20px 25px rgba(0,0,0,0.07),
    0 40px 55px rgba(0,0,0,0.05);
}

/* Flat (colored) shadow — for buttons and action elements */
.shadow-flat-brand {
  box-shadow: 6px 6px 0px var(--brand-primary);  /* neo-brutalist */
}

/* Soft shadow (glassmorphism) */
.shadow-glass {
  box-shadow: 0 8px 32px rgba(0,0,0,0.12);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(255,255,255,0.12);
}
```

### Borders as a design element

```css
/* Frame-style: full border as a "picture frame" */
.frame-border {
  border: 3px solid var(--brand-primary);
  border-radius: 12px;
  padding: 24px;
}

/* Accent line: side border as a category indicator */
.accent-line {
  border-left: 6px solid var(--brand-primary);
  padding-left: 24px;
}

/* Neo-brutalist: solid border + offset shadow */
.neo-brutalist {
  border: 3px solid #111827;
  box-shadow: 5px 5px 0px #111827;
  border-radius: 8px;
}
```

### Asymmetry vs. centering

- **Centered:** works for big-number, quote, CTA — direct impact, no cognitive friction
- **Asymmetric:** works for cover and comparison — creates visual tension that forces the eye to move. Rule: the large element takes **60%** of the slide; the smaller one (secondary text or visual) takes **40%**
- **F-pattern:** headline top-left, support just below-left, visual on the right — natural Western scan
- **Z-pattern:** headline top, visual center, CTA/takeaway bottom-right — works for single-concept slides

---

## 6. Visual Anti-Patterns — What Kills Engagement

| Anti-pattern | Why it kills | Fix |
|---|---|---|
| Text glued to the edge (padding < 40 px) | Looks amateur; cropped in preview | Minimum 60–80 px on all sides |
| Giant corner logo (> 80 px) | Steals space from the hook; looks like an ad | Logo max 48–60 px, bottom corner, `opacity: 0.7` |
| Intrusive center watermark | Blocks content; reduces saves | Subtle corner watermark, never over text |
| Fancy / script font in body | Illegible below 48 px; never on mobile | Script/decorative only on isolated words (max 2 words) |
| Background pattern that fights the text | Visual noise; uneven contrast | Dark overlay `rgba(0,0,0,0.5)` between pattern and text |
| More than 30 words per slide | Nobody reads; instant drop-off | Cut down to 1 core idea per slide |
| Light-weight fonts (100–300) on mobile | Disappear below 32 px | Min weight 400 for body; 600+ for headlines |
| Mixed aspect ratios in the same carousel | Unpredictable Instagram crop | Pick one ratio for the whole carousel |
| No swipe indicator | User doesn't know more slides exist | Add `→` or "Swipe" on slide 1 and the first 3 |
| Generic CTA ("Follow me") | No urgency, no specificity | Specific action verb + clear benefit |
| More than 3 colors per slide | Visual pollution, no hierarchy | 2 colors per slide: background + text. Accent on a single element |
| Gradient as primary visual identity | Inconsistent across thumbnails and screens | Gradient only as background (monochromatic) or selectively on hero text |

---

## 7. Brand Consistency Across 8–10 Slides Without Monotony

### Controlled-variation system

The trick is to define **3 slide templates** and rotate between them:

| Template | Background | Text | When to use |
|---|---|---|---|
| **A — Dark (authority)** | `#0D1117` | `#FFFFFF` + colored accent | Cover, big-number, CTA |
| **B — Light (clarity)** | `#F8FAFC` | `#111827` + colored accent | List, body, how-to |
| **C — Brand color (spotlight)** | `var(--brand-primary)` | `#FFFFFF` | Transition slide, quote, visual surprise |

**Suggested rotation for 10 slides:**

```
Slide 1  → Template A (cover, impact)
Slide 2  → Template B (context/pain)
Slide 3  → Template A (big-number)
Slides 4–7 → Template B (content, list, how-to)
Slide 8  → Template C (quote or turning point)
Slide 9  → Template A (proof / outcome)
Slide 10 → Template A (CTA)
```

### Fixed elements across all slides (brand anchors)

```
1. Handle / username: bottom-left, 24px, opacity: 0.6
2. Slide number: bottom-right, "3/10" or "—3—", 24px, opacity: 0.5
3. Logo: integrated into the handle OR absent (don't duplicate)
4. Standard margin: 80px on all sides — NEVER changes
5. Heading font: the same on every slide (only weight varies)
```

### Variation allowed without breaking consistency

- Background color: may rotate between the 3 templates
- Accent text color: may vary within the palette (e.g. use `--accent-1` on slides 1–5, `--accent-2` on slides 6–10 to build a visual arc)
- Visual element size: varies by layout; position may shift
- Font weight: headline can be 700 or 800 depending on desired impact

### What **never** varies

- Typeface family
- Minimum sizes (body never below 36 px on canvas)
- Base margin (80 px)
- Position of handle and slide number
- Canvas aspect ratio

---

## TEMPLATE CHECKLIST — 20 Codifiable Rules

Below are the rules per layout. Each item is programmatically verifiable by a skill.

### Universal Rules (every slide)

```
U1. canvas.width >= 1080 && canvas.height >= 1080 (never below)
U2. padding_all >= 60px (recommended 80px)
U3. font_size_min_body >= 32px (hard limit: never below)
U4. font_size_min_label >= 24px
U5. contrast_ratio_body >= 4.5:1 (WCAG AA)
U6. contrast_ratio_hook >= 7:1 (thumbnail legibility)
U7. fonts_used <= 2 (one heading, one body)
U8. colors_per_slide <= 3 (bg + text + accent)
U9. words_per_slide <= 30 (hard limit)
U10. handle_present = true (bottom corner, opacity >= 0.5)
U11. slide_number_present = true ("N/T" format, bottom-right corner)
U12. aspect_ratio = constant across the carousel (no mixing)
```

### Rules by Layout

```
COVER (slide 1):
C1. hook_words <= 12
C2. hook_font_size >= 80px (recommended 96–144px)
C3. hook_contrast >= 7:1
C4. swipe_indicator_present = true (→ arrow or "Swipe" text)
C5. logo_size <= 60px (if present)
C6. microcopy_words <= 10 (cover subtitle)

LIST:
L1. items_per_slide <= 5
L2. badge_size = 48–60px with item number
L3. item_font_size >= 36px
L4. gap_between_items >= 24px

BIG-NUMBER:
N1. number_font_size >= 160px
N2. caption_words <= 8
N3. background = neutral (white/near-black), never a pattern

QUOTE:
Q1. quote_words <= 25 (quote body)
Q2. decorative_quotes = true (CSS quote marks, opacity: 0.08–0.12)
Q3. attribution_present = true (name + context)
Q4. different_weight_from_body = true (e.g. italic if body = regular)

COMPARISON:
R1. sides = 2 (never 3)
R2. items_per_side <= 2
R3. side_bg_negative = rgba(red, 0.08) (never pure red)
R4. side_bg_positive = rgba(green, 0.08) (never pure green)
R5. icon_present = true (✗ and ✓, 48px)

SCREENSHOT:
S1. device_mockup_present = true (device frame)
S2. no_text_overlaid_on_screenshot = true
S3. caption_lines <= 2
S4. mockup_shadow_present = true

CTA (last):
A1. cta_count = 1 (single action)
A2. cta_verb_specific = true (not "Follow me" — use "Save", "Comment X", "DM Y")
A3. swipe_back_indicator_present = true (← or "Back to the start")
A4. handle_and_logo_visible = true
A5. headline_words <= 12
```

---

## References and Sources

- [Pineable — Social Media Carousel Design Best Practices](https://pineable.com/blog/social-media-carousel-design-best-practices)
- [Postnitro — 15 Strategies for Viral Instagram Carousels 2025](https://postnitro.ai/blog/post/viral-instagram-carousels-strategies-2025)
- [Postnitro — Color Trends 2025 Carousel Design](https://postnitro.ai/blog/post/color-trends-2025-design-carousel)
- [Postnitro — 10 Carousel Design Mistakes](https://postnitro.ai/blog/post/carousel-design-mistakes)
- [Zeely — Master Instagram Safe Zones 2026](https://zeely.ai/blog/master-instagram-safe-zones/)
- [Hootsuite — Social Media Image Sizes May 2026](https://blog.hootsuite.com/social-media-image-sizes-guide/)
- [Panocollages — 15 Design Tips for Eye-Catching Instagram Carousels](https://panocollages.com/blog/15-design-tips-for-eye-catching-instagram-carousels)
- [Ligosocial — LinkedIn Carousel Dimensions & Specs Guide 2025](https://ligosocial.com/blog/linkedin-carousel-post-size-guide-2025-dimensions-formats-and-best-practices)
- [Oktopost — LinkedIn Carousel Best Practices 2026](https://www.oktopost.com/blog/linkedin-carousel-pdf-best-practices/)
- [Postwaffle — TikTok Carousel Size Guide 2026](https://www.postwaffle.com/blog/tiktok-carousel-size)
- [WebAIM — Contrast and Color Accessibility WCAG](https://webaim.org/articles/contrast/)
- [Influencer Marketing Hub — X Twitter Image Sizes 2026](https://influencermarketinghub.com/twitter-image-size/)
- [Typewolf — 40 Best Google Fonts 2026](https://www.typewolf.com/google-fonts)
- [OrangeBlueWeb — Best Google Fonts 2025 Modern Combos](https://orangeblueweb.com/best-google-fonts-in-2025-20-modern-serif-sans-serif-combos-that-convert-visitors-into-customers/)
- [SocialRails — Text Overlays on Instagram Carousels 2026](https://socialrails.com/social-media-terms/text-overlays-on-instagram-carousels)
- [Resont — Best Hooks for Instagram Carousel](https://resont.com/blog/top-instagram-carousel-hooks/)
- [Futuristic Marketing — Instagram Carousel Design Complete Guide](https://futuristicmarketingservices.com/Blogs/graphic-designing/instagram-carousel-design-guide/)
- [Carouselli — Instagram Carousel Tips 2026](https://carouselli.com/blog/instagram-carousel-tips)
- [Panocollages — Common Instagram Carousel Mistakes](https://panocollages.com/blog/common-instagram-carousel-mistakes-and-how-to-fix-them)
- [UseVisuals — 12 LinkedIn Carousel Post Best Practices 2025](https://usevisuals.com/blog/linkedin-carousel-post-best-practices)

---

*File generated: 2026-05-17. Update when platforms change their specs (review every six months).*
