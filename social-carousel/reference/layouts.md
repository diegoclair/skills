# Layouts — when to use each

> When to read: when choosing a `layout` for a slide.

The skill ships 7 narrative layouts (`cover`, `list`, `big-number`, `quote`, `comparison`, `screenshot`, `cta`) plus a generic `text` fallback. Each was designed against a specific cognitive role within an 8–10 slide carousel.

## The structural contract (carousel-level)

Independent of which layouts you use, a carousel that doesn't follow this template fails the linter at the carousel level:

```
slide 1     → cover            (hook stops the scroll)
slide 2     → context / pain   (text or big-number)
slide 3     → VALUE BOMB       (big-number | list | quote) — algorithmic inflexion
slides 4–7  → meat             (list | comparison | screenshot)
slide 8–9   → payoff / proof   (quote | big-number | screenshot)
slide last  → cta              (one verb, one action)
```

The linter blocks render if:
- Slide 1 is NOT `cover` (AP-01)
- Slide 3 is `text` or `cover` (ST-05)
- Last slide is NOT `cta` (AP-07)

## cover

**Purpose:** stop the scroll in <0.7s. Compete with the entire feed.

**Required fields:** `hook` (≤12 words).
**Optional:** `label` (uppercase tag, e.g. "SOLO SERVICE PROVIDER"), `sub` (microcopy line, ≤10 words), `hook_formula` (H-01..H-13).

**Rules enforced:**
- C1: hook ≤ 12 words.
- C3: hook contrast ≥ 7:1 against background (thumbnail readability).
- C6: sub ≤ 10 words.

**When to skip:** never. Every carousel has exactly one cover, at slide 1.

## list

**Purpose:** deliver value as scannable numbered items. Each item ≈ one takeaway.

**Required:** `title` (≤8 words), `items` (1–5 items, each ≤8 words).
**Visual:** every item gets a numbered badge `[01]`, `[02]`, ... rendered via `add1` template func.

**Rules enforced:**
- L1: items ≤ 5 per slide. If you have 7 items, split into 2 list slides ("Items 1–4" + "Items 5–7").
- L3: each item ≤ 8 words (warning).

**When to use:** anywhere in slides 4–7 of a listicle. Also as recap at slide 9.

**When NOT to use:** at slide 3 — list is fine there, but `big-number` lands harder for value-bomb impact.

## big-number

**Purpose:** dramatic impact. Number is the headline.

**Required:** `number` (string, e.g. `"87%"`, `"5×"`, `"−42%"`), `caption` (≤6 words).
**Visual:** number is rendered at 200–300 px, accent color, top third of slide; caption below in body color.

**Rules enforced:**
- N2: caption ≤ 6 words (linter blocks at 8+).
- N3: if the active theme has a `background_effect` (dots/grid/halo), the linter warns — big-number needs neutral background to land.

**When to use:** slide 3 as value bomb; also as proof at slide 8/9.

**When NOT to use:** if you don't have a real number. Inventing statistics is anti-pattern AP-11 and erodes credibility fast.

## quote

**Purpose:** emotional connection, shareability, authority by association.

**Required:** `quote` (body, ≤25 words), `attribution` (`— Name, Context`).
**Visual:** decorative quotation marks at 200 px / 10% opacity behind the text. Quote is rendered in the `font_quote` family (defaults to Playfair Display italic).

**Rules enforced:**
- Q1: quote ≤ 25 words.
- Q3: attribution required.

**When to use:** transition slide (8 or 9), or as value bomb at slide 3 if the quote IS the surprising insight.

**Pitfall:** don't use as filler. If the quote isn't memorable, it's wasted real estate.

## comparison

**Purpose:** clarify a contrast. Wrong vs right, before vs after.

**Required:** `before_label`, `before_items` (1–2 items), `after_label`, `after_items` (1–2 items).
**Visual:** 50/50 split. Before column: soft red bg + ✗. After column: soft green bg + ✓.

**Rules enforced:**
- R1: both sides required (both labels must be non-empty).
- R2: ≤ 2 items per side. If you have 4 contrasts, use 2 comparison slides.

**When to use:** in counterintuitive (H-02) carousels; case-studies; refuting common advice.

**Pitfall:** the contrast must be sharp. Vague "wrong: X vs right: Y" without specifics reads as filler.

## screenshot

**Purpose:** proof via "show, don't tell". Concrete evidence.

**Required:** `title` (≤8 words), `image` (path relative to YAML), `caption` (≤16 words).
**Optional:** `device` (`"iphone"`, `"browser"`, `"android"`, or `""`).
**Visual:** screenshot rendered inside a CSS mockup frame (iPhone bezel, browser chrome, or none).

**Rules enforced:**
- S2: caption required (warn if empty).
- S3: caption ≤ 16 words.

**When to use:** to back up a claim with a real screenshot (dashboard, message, result). Not for stock photos.

**Pitfall:** never overlay text on the screenshot itself. The text always lives in the title above or caption below.

## cta

**Purpose:** convert attention into one specific action. Last slide.

**Required:** `headline` (question or command, ≤12 words), `cta_text` (single specific verb).
**Optional:** `swipe_back: true` to add a `← back to start` indicator.

**Rules enforced:**
- A1: ONE verb in `cta_text`. Multiple verbs (e.g. `"Save + Comment + Follow"`) is blocked. Combine at most TWO complementary verbs (e.g. `"Save and DM me"`).
- A2: avoid generic CTAs. `"Follow"` alone is warned — use specific action: `"Comment CHECKLIST"`, `"DM me the word X"`.
- A5: headline ≤ 12 words.

**CTA hierarchy by algorithmic weight (Instagram 2026):**
1. **Save + DM share** — highest. "Save to revisit. Send to someone who needs it."
2. **Comment a word** — generates DM lead via auto-reply bots.
3. **Specific comment question** — generates discussion threads.
4. **Follow** — low direct impact, useful in story carousels.

**Pitfall:** asking for "Save, share, comment, follow and click the link". Five asks = zero actions taken.

## text (fallback)

**Purpose:** generic free-form body text. Use sparingly.

**Required:** `body` (≤30 words).

**When to use:** slide 2 (context/setup) when you need a one-liner that's neither a number nor a list. Or slide 5 in a story carousel for a narrative beat.

**When NOT to use:** at slide 3 (linter blocks via ST-05) and never at slide 1 (the cover layout is required there).

## Mixing layouts within one carousel

Variety helps retention. A "best of" 10-slide listicle:

```
1. cover            ← hook
2. text             ← "why this matters" microcopy
3. big-number       ← value bomb
4. list             ← items 1–3
5. text             ← bridge ("but the worst is…")
6. list             ← items 4–7
7. comparison       ← right vs wrong way
8. quote            ← summary insight
9. big-number       ← proof of result
10. cta             ← one ask
```

A carousel where every slide is the same layout (8× list, 6× big-number) reads monotone. Linter doesn't block this — it's a judgment call you should make at scaffold time. The `new <kind>` scaffolds are already mixed by design.
