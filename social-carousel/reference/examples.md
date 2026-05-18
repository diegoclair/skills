# Reference Carousels — What good looks like

> When to read: before scaffolding any carousel that needs to perform — and any time the rendered output feels "fine but not viral." These references are the calibration target. The linter tells you when something is wrong; this file tells you what right looks like.

The lint rules encode the WHAT (measurable: word counts, contrast ratios, layout structure). They cannot encode the HOW-IT-LOOKS-WHEN-GOOD. These references close that gap. Engagement numbers below were verified at compile time (May 2026) by direct WebFetch of the post page. Where a number says "engagement-not-verifiable," the platform (Instagram) does not expose per-post counts publicly — we still include the reference but flag the limitation.

---

## Reference #1 — Jasmin Alić — typographic system

**Use as model for:** the visual ceiling. If a generated carousel looks 70% as polished as Alić's it ships.

**Verified carousel:**
- ["7 Ways to Format LinkedIn Posts for Readability"](https://www.linkedin.com/posts/alicjasmin_7-ways-to-format-linkedin-posts-for-readability-activity-7127618219866996737-OwqJ) — **2,778 likes / 1,151 comments**
- Self-demonstrating: Alić spent 10+ hours designing this carousel and explicitly formatted every slide to demonstrate the principles discussed. The carousel IS the proof.

**Why it works:**
- **One idea per slide**, period. Body copy stays under ~15 words.
- **Single accent color** (yellow) used only as underline + arrow. Cream/black base. Never two competing colors.
- **Oversized hook**, deliberate negative space, copy left-aligned, never centered "Canva-style."
- **No illustration, no stock photo** — pure typographic hierarchy.

**Hook formulas borrowed from his catalog:**
- "[N] Ways to [verb] [thing] for [outcome]"
- "My biggest [X] ever (how I [outcome] in [timeframe])"
- "This might be my [superlative] [thing] ever"

**Layout sequence to mirror:**
```
cover           → giant hook, one accent stroke
text or quote   → reframe / setup
big-number      → the reveal (stat, % or specific number)
list 4–5 items  → the playbook
cta             → single ask
```

**Mapping to our YAML:**
- `theme: minimal-mono` (cream base, single accent) — closest preset.
- `comparison` layout works well to render his "right vs wrong" frame slides.
- Use `**emphasis**` once per slide max — Alić's restraint is the lesson.

**Caveat:** he spends 10-20h per carousel. Match the *structure*, not the polish. Also: many Alić *text* posts on LinkedIn are referenced as carousels online but are actually text-only — check that any cited URL contains the `ugcPost-` token or visually loads a document viewer before treating it as a design reference.

---

## Reference #2 — Justin Welsh — "[N] Steps to [outcome]" framework

**Use as model for:** listicle / framework carousels. The default LinkedIn structure for educational content.

**Reference is at the level of content strategy, NOT specific carousel design:**
- [LinkedIn growth guide 2026](https://www.justinwelsh.me/article/linkedin-guide-2026) — Welsh's long-form article codifying the "[N] Steps" framework, hook anatomy, and content rotation.
- [Anatomy of a Viral LinkedIn Post](https://www.justinwelsh.me/newsletter/the-anatomy-of-a-viral-linkedin-post) — companion piece on post structure.

**Caveat — be honest about the source:** these are **articles about carousels**, not specific verified carousels. The "[N] Steps" structure is industry-standard and Welsh codified it cleanly, but the slide-level visual claims below come from his article-level guidance + third-party teardowns, not from a single audited Welsh carousel post. Don't tell a user "this is exactly how Welsh designs slide 3" — say "this is the standard listicle structure Welsh teaches, applied by hundreds of LinkedIn creators."

**Why the structure works:**
- **Numeric promise in cover** — "7 Steps", "5 Frameworks". Sets a contract.
- **One step per slide** with oversized numeral as the visual anchor (numeral takes ~40% of slide).
- **Single muted accent.** No gradient as identity.
- **~30-50 words per slide.** Each step has a one-line takeaway + a 1-2 sentence why.

**Hook formulas:**
- "[N] [hard things] every [role] should [verb]"
- "How I [achieved] in [N] [moves/steps]"

**Layout sequence to mirror:**
```
cover           → "[N] Steps to [outcome]"
text or quote   → why this matters
big-number      → first step's value/proof (slide-3 value bomb)
list × 2       → steps 1-4 then steps 5-7 (split if >5)
big-number     → proof of outcome
cta            → newsletter / save / DM
```

**Mapping to our YAML:**
- `theme: light-editorial` for Welsh's clean look, or `minimal-mono` for stricter discipline.
- `list` layout proportions auto-scale by item count — keep ≤5 per slide.
- Numeric promise in cover hook is non-negotiable for this pattern.

---

## Reference #3 — Nick Broekema — meta-design carousel (B2B/SaaS founder fit)

**Use as model for:** B2B/SaaS/founder content. Closest analog to data-led posts about products, engineering, market research, or operator playbooks.

**Verified carousel** (`ugcPost-` token confirms document format):
- ["Create carousels that attract clients and followers"](https://www.linkedin.com/posts/nbroekema_create-carousels-that-attract-clients-and-ugcPost-7237051775151673345-5p6v) — **906 likes / 379 comments**
- Companion: ["Mini-guide to carousels"](https://www.linkedin.com/posts/nbroekema_a-mini-guide-to-carousels-that-attract-followers-activity-7152555062190383104-FrIY) — **715 likes / 376 comments**

**Why it works:**
- **Meta-design** — Broekema makes carousels about making carousels, so the format itself is the proof.
- **Brand-locked palette and font system** across every post — instantly recognizable in feed.
- **Documents his own results inside the carousel** ("90% of my inbound leads come through carousels"). Numbers serve as embedded proof, not afterthoughts.
- **One step per slide** with consistent badge/numeral position. Reads like a manual, not a magazine.
- **Sources/methodology at the end**, never in the middle. Builds credibility without breaking the swipe rhythm.

**Hook formulas:**
- "[Outcome verb] [things] that [secondary outcome]"
- "I analyzed [N] [thing] and found [counterintuitive claim]"
- "[N]% of [audience] [does surprising thing]. Here's why."

**Layout sequence to mirror:**
```
cover            → outcome promise ("Create [X] that [Y] and [Z]")
text             → why this matters / the problem
big-number       → the proof stat with subhead + context
list             → step 1-4 of the framework
list or comparison → before/after, or steps 5-7
big-number       → final proof number
cta              → book a call / DM keyword
```

**Mapping to our YAML:**
- `theme: dark-tech` or `minimal-mono` — both work; dark feels more "operator playbook," lighter feels more "guide."
- Use `big-number` with all four fields (`subhead`, `number`, `caption`, `context`) — this is what they're for.
- The slide-3 value bomb is the strongest stat. Order other stats descending in surprise value.
- Apply `tone: authority` to slides 1/3/8 and let the others stay `clarity` to mirror his rotation.

---

## Reference #4 (optional) — Chris Do — Instagram type-heavy carousel

**Use as model for:** Instagram type-only carousels, especially when teaching/educating.

**Reference is at profile + teardown level, NOT a single verified post:**
- [@thechrisdo profile](https://www.instagram.com/thechrisdo/) — his Instagram catalog.
- [The Futur's "Instagram Carousel Design Clinic"](https://thefutur.com/instagram-carousel-design-clinic) — the design-language teardown that documents how his carousels are constructed.

**Caveat — Instagram engagement is private-by-design:** Instagram strips per-post like/save/comment numbers from logged-out HTML and doesn't expose them via public API. We cannot verify "this specific Chris Do post got X saves" the way we can verify a LinkedIn carousel. The reference is here for design language, not for measured performance.

**Why his approach works (per The Futur teardown):**
- **Tight grid system** — every slide built on the same 12-column grid. Position never wanders.
- **Two-color discipline** — black/white + one accent. Same rule as Alić.
- **Bold serif + light sans-serif pairing** for hierarchy — more editorial than tech feel.
- **Quote-style centerpieces** — many slides are a single pull-quote with attribution. Maps to our `quote` layout almost 1:1.

**Mapping to our YAML:**
- `theme: light-editorial` (closest preset; cream/white base, single accent).
- Lean on `quote` layout more than usual — Chris Do's carousels are quote-heavy.
- Don't try to replicate his serif pairing unless you author a custom theme with serif `font_heading`.

---

## Cross-cutting patterns confirmed by every top reference

These are the few rules the references *always* obey. If your carousel breaks one of them, the linter probably won't catch it but the result will feel off:

1. **Single accent color.** One. Never two competing. Our themes already enforce this; honour it by not adding `**highlight**` on top of layout primitives that already use accent (CTA box, big-number numeral, quote decorative mark). The renderer now strips highlight on those slides automatically — don't fight it.
2. **One idea per slide.** If you have a "Use a CRM **and** track follow-ups" item, it's two ideas. Split.
3. **Slide-3 is the surprising one.** Read your slide 3. If it doesn't make you stop, the carousel doesn't ship.
4. **Numeric promise on the cover** if the carousel is a framework/listicle. "7 Steps" beats "Some Steps" every time.
5. **8-12 slides.** Verified high-engagement carousels cluster here. Outliers (Matt Barker's 23-slide) justify length with TOC structure — don't pad to hit a count.
6. **Cover hook reads in <0.7s on a phone thumbnail.** If you can't read it muted on a 2-inch preview, it doesn't stop the scroll.

---

## What we explicitly don't do (yet)

The references use these patterns; our skill doesn't render them today. Worth knowing so you don't try to fake them:

- **Charts/data viz** — some references include bar charts or screenshots of analytics. Our `screenshot` layout accepts an image path but doesn't generate charts. If you need a chart, render it elsewhere (Figma, code, Plotly) and embed via `screenshot`.
- **Face inset on cover** — Bloom, Donnelly, Acosta all put a small headshot bottom-right of the cover. Not supported as a field yet; doable manually via `screenshot` layout for a single slide.
- **Brand-locked illustration system** — Alić's hand-drawn arrows are part of his identity. Our skill is typography-only.

When the user asks for one of these, say so explicitly — don't render a worse-looking text-only fallback and pass it off as the same thing.

---

## Brazilian/PT-language gap

We searched specifically for Bruno Perini / Caio Carneiro / Tallis Gomes carousels and found nothing with verifiable engagement at the level of the references above. They're stronger on text posts and video. If the user wants a PT-language reference style, **the carousel needs to be built from scratch** — there's no canonical PT-language carousel creator to mirror at the time of writing.

That doesn't mean PT carousels can't perform; it means the references in this file are all EN-language and the user is responsible for adapting voice. Hook formulas translate directly. Hierarchy and pacing translate directly. Pretending a PT example exists when it doesn't is worse than admitting the gap.

---

## Methodology note (so future audits can re-verify)

- **LinkedIn URLs** with token `ugcPost-` strongly indicate document/carousel format. URLs with `activity-` are ambiguous — they're used for both text posts and document posts. When in doubt, check that the page loads a document viewer; if WebFetch returns only body text with no viewer markup, the post is probably text-only.
- **LinkedIn engagement numbers** are publicly visible on the post page and can be verified by WebFetch.
- **Instagram per-post engagement** is NOT publicly verifiable. Numbers in third-party roundups (Amra & Elma, Later, Predis) are typically projected/estimated, not measured. Treat them as directional only.
- **Visual design of any specific post** cannot be independently verified through WebFetch alone — the slide media is in an embedded viewer that gets stripped during HTML-to-markdown conversion. For Alić, Welsh, Broekema we cross-checked third-party teardowns (Contentdrips, Favikon, MagicPost) to corroborate the design-discipline claims above.
