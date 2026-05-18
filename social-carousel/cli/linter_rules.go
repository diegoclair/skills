// Package main — linter_rules implements all 32 codified viral-carousel rules.
//
// Rules are organised by scope:
//
//   - Carousel-level rules (idx = -1): slide count, CTA placement, handle, fonts, contrast.
//   - Universal per-slide: word count hard limit.
//   - Layout-specific: cover (C*), list (L*), big-number (N*), quote (Q*),
//     comparison (R*), screenshot (S*), cta (A*).
//   - Anti-pattern extras: AP-04, AP-07 (already covered as carousel-level).
//
// Rule codes follow the research document (D-design-system-viral.md §TEMPLATE CHECKLIST).
package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Helper utilities
// ---------------------------------------------------------------------------

// countWords returns the number of whitespace-separated tokens in s.
// An empty string returns 0.
func countWords(s string) int {
	return len(strings.Fields(s))
}

// slideWords returns the total word count across all text fields of a slide.
// Fields: Hook, Sub, Title, Body, Caption, Quote, Attribution, Headline,
// CTAText, Label, and all Items (list), BeforeItems, AfterItems (comparison).
func slideWords(s Slide) int {
	n := 0
	for _, f := range []string{
		s.Hook, s.Sub, s.Title, s.Body, s.Caption,
		s.Quote, s.Attribution, s.Headline, s.CTAText, s.Label,
	} {
		n += countWords(f)
	}
	for _, item := range s.Items {
		n += countWords(item)
	}
	for _, item := range s.BeforeItems {
		n += countWords(item)
	}
	for _, item := range s.AfterItems {
		n += countWords(item)
	}
	return n
}

// slideKind is a human-readable label for an error message, including the
// layout name, e.g. "slide 3 (big-number)".
func slideKind(idx int, s Slide) string {
	return fmt.Sprintf("slide %d (%s)", idx+1, s.Layout)
}

// ---------------------------------------------------------------------------
// WCAG contrast helpers
// ---------------------------------------------------------------------------

// expandHex expands shorthand #RGB to #RRGGBB.
func expandHex(h string) string {
	h = strings.TrimPrefix(h, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	return h
}

// hexToRGB parses a 6-char hex string (without #) into r, g, b in [0, 255].
// Returns 0,0,0 on parse error (safe degradation).
func hexToRGB(h string) (r, g, b float64) {
	if len(h) < 6 {
		return 0, 0, 0
	}
	ri, _ := strconv.ParseUint(h[0:2], 16, 8)
	gi, _ := strconv.ParseUint(h[2:4], 16, 8)
	bi, _ := strconv.ParseUint(h[4:6], 16, 8)
	return float64(ri), float64(gi), float64(bi)
}

// sRGBLinear converts a [0,255] channel value to linearised sRGB per WCAG 2.1.
func sRGBLinear(c float64) float64 {
	v := c / 255.0
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// relativeLuminance computes the WCAG relative luminance for a hex colour
// (with or without #). Returns 0 on parse error.
func relativeLuminance(hex string) float64 {
	h := expandHex(hex)
	r, g, b := hexToRGB(h)
	return 0.2126*sRGBLinear(r) + 0.7152*sRGBLinear(g) + 0.0722*sRGBLinear(b)
}

// contrastRatio returns the WCAG contrast ratio between two hex colours.
// The formula is (L1+0.05)/(L2+0.05) where L1 >= L2.
// Supports both #RRGGBB and #RGB.
func contrastRatio(hex1, hex2 string) float64 {
	l1 := relativeLuminance(hex1)
	l2 := relativeLuminance(hex2)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// ---------------------------------------------------------------------------
// CTA verb detection helper (rule A1)
// ---------------------------------------------------------------------------

// imperativeVerbs is the list of known CTA action words in PT and EN.
var imperativeVerbs = []string{
	"salva", "salve", "comenta", "comente", "compartilha", "compartilhe",
	"manda", "envie", "envia", "siga", "follow", "save", "comment", "share", "dm",
}

// countCTAVerbs returns the number of distinct imperative verbs found in text.
// Uses a simple whole-word, case-insensitive scan.
func countCTAVerbs(text string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, verb := range imperativeVerbs {
		if strings.Contains(lower, verb) {
			count++
		}
	}
	return count
}

// genericCTAVerbs are CTAs that ask for nothing specific — the algorithm
// barely weights them. Audit F-3.7 added the missing PT variants and English
// "like" so a "Curta o post" CTA gets flagged the same way "Follow me" does.
var genericCTAVerbs = []string{
	// follow-style
	"siga", "follow", "me siga", "me sigam", "siga-me", "follow me",
	// like-style
	"curta", "curte", "curtam", "like", "likes",
}

// isGenericCTA returns true when the cta text contains only generic follow-type
// verbs and none of the high-value action verbs.
func isGenericCTA(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, g := range genericCTAVerbs {
		if strings.Contains(lower, g) {
			// check that no high-value verb is also present
			highValue := []string{"salva", "salve", "comenta", "comente", "compartilha", "compartilhe", "manda", "envie", "envia", "dm", "save", "comment", "share"}
			for _, hv := range highValue {
				if strings.Contains(lower, hv) {
					return false
				}
			}
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Carousel-level rules
// ---------------------------------------------------------------------------

// lintSlideCount implements rule AN-01: optimal slide count.
// Research A §2 and D §Quantidade de Slides Ótima:
//   - <5 or 4-7: WARN (abandono curve worse than 8+)
//   - 8-10 or 11-15: OK
//   - >15: WARN (too long)
//   - 0: ERR (unusable)
func lintSlideCount(r *LintReport, c *Carousel) {
	n := len(c.Slides)
	switch {
	case n == 0:
		r.addErr("AN-01", -1, "carousel has no slides", "")
	case n < 5:
		r.addWarn("AN-01", -1,
			fmt.Sprintf("carousel has %d slides; abandonment curve is worst between 4–7", n),
			"consider 8–10 slides for educational content or ≤3 for a micro-carousel",
		)
	case n <= 7:
		r.addWarn("AN-01", -1,
			fmt.Sprintf("carousel has %d slides; algorithmic sweet spot is 8–10", n),
			"consider expanding to 8–10 slides or condensing to ≤3",
		)
	case n > 15:
		r.addWarn("AN-01", -1,
			fmt.Sprintf("carousel has %d slides; drop-off rises above 15", n),
			"consider splitting into two carousels or condensing",
		)
	}
}

// lintCoverFirst implements rule AP-01: first slide must be a cover.
func lintCoverFirst(r *LintReport, c *Carousel) {
	if len(c.Slides) == 0 {
		return
	}
	if c.Slides[0].Layout != "cover" {
		r.addWarn("AP-01", 0,
			fmt.Sprintf("first slide is '%s'; must be 'cover'", c.Slides[0].Layout),
			"the cover is the only slide visible in the feed before any interaction",
		)
	}
}

// lintSlide3ValueBomb implements rule ST-05: slide 3 must be a value bomb.
// Research A §3 ST-05: users who reach slide 3 have an 80% chance of completing.
// Layout "text" or "cover" on slide 3 = wasted the inflection point.
func lintSlide3ValueBomb(r *LintReport, c *Carousel) {
	if len(c.Slides) < 3 {
		return
	}
	s3 := c.Slides[2]
	switch s3.Layout {
	case "text", "cover":
		r.addWarn("ST-05", 2,
			fmt.Sprintf("slide 3 uses layout '%s'; this is the algorithmic inflection point", s3.Layout),
			"use big-number, list or quote to maximize retention on slide 3",
		)
	}
}

// lintLastSlideCTA implements rule AP-07: last slide must be a CTA.
func lintLastSlideCTA(r *LintReport, c *Carousel) {
	if len(c.Slides) == 0 {
		return
	}
	last := c.Slides[len(c.Slides)-1]
	if last.Layout != "cta" {
		lastIdx := len(c.Slides) - 1
		r.addWarn("AP-07", lastIdx,
			fmt.Sprintf("last slide uses layout '%s'; must be 'cta'", last.Layout),
			"the last slide is the lowest-abandonment moment; wasting it without a CTA loses the conversion",
		)
	}
}

// lintRhythm implements rule RH-01: visual rhythm check.
//
// Research D §7 ("Visual Rhythm") and audit F-2.2 prescribe varying layout
// and tone across a deck so the reader never sees 5 identical-looking slides
// in a row. The linter flags any run of 5+ consecutive slides that share the
// same Layout OR the same Tone (whichever streak hits 5 first wins). Empty
// Tone counts as its own bucket — if 5 in a row inherit the theme with no
// tone variation that is still rhythmically flat, but only the Layout streak
// is reported in that case to avoid double-warning.
func lintRhythm(r *LintReport, c *Carousel) {
	if len(c.Slides) < 5 {
		return
	}

	// Walk the slide list once, tracking the current run for layout and tone.
	report := func(kind, value string, startIdx, runLen int) {
		r.addWarn("RH-01", -1,
			fmt.Sprintf("monotone rhythm — %d+ consecutive '%s' slides break swipe-through; intersperse another %s",
				runLen, value, kind),
			fmt.Sprintf("vary the %s between slides %d and %d to keep the deck visually fresh",
				kind, startIdx+1, startIdx+runLen),
		)
	}

	// Layout streak.
	layoutRunStart := 0
	for i := 1; i <= len(c.Slides); i++ {
		if i == len(c.Slides) || c.Slides[i].Layout != c.Slides[layoutRunStart].Layout {
			runLen := i - layoutRunStart
			if runLen >= 5 {
				report("layout", c.Slides[layoutRunStart].Layout, layoutRunStart, runLen)
			}
			if i < len(c.Slides) {
				layoutRunStart = i
			}
		}
	}

	// Tone streak — only meaningful when at least one slide in the run has a
	// non-empty Tone, otherwise the whole carousel just inherits the theme
	// and the "monotone tone" warning is noise.
	toneRunStart := 0
	for i := 1; i <= len(c.Slides); i++ {
		if i == len(c.Slides) || c.Slides[i].Tone != c.Slides[toneRunStart].Tone {
			runLen := i - toneRunStart
			if runLen >= 5 && c.Slides[toneRunStart].Tone != "" {
				report("tone", c.Slides[toneRunStart].Tone, toneRunStart, runLen)
			}
			if i < len(c.Slides) {
				toneRunStart = i
			}
		}
	}
}

// lintHandle implements rule U10: handle must be present.
func lintHandle(r *LintReport, c *Carousel) {
	if c.Handle == "" {
		r.addWarn("U10", -1,
			"handle field is empty; brand anchor will not be displayed",
			"set handle (e.g. '@lybel.com.br') for consistent identity across slides",
		)
	}
}

// lintFontCount implements rule U7: maximum 2 font families.
// Counts distinct non-empty font family names across FontHeading, FontBody, FontQuote.
func lintFontCount(r *LintReport, theme *Theme) {
	if theme == nil {
		return
	}
	seen := map[string]bool{}
	for _, f := range []string{theme.FontHeading, theme.FontBody, theme.FontQuote} {
		if f != "" {
			seen[f] = true
		}
	}
	if len(seen) > 2 {
		r.addWarn("U7", -1,
			fmt.Sprintf("theme uses %d font families (%s, %s, %s); max recommended is 2",
				len(seen), theme.FontHeading, theme.FontBody, theme.FontQuote),
			"use heading + body; FontQuote should be a variation (weight/style) of heading or body",
		)
	}
}

// lintContrastBody implements rule U5: body contrast ≥ 4.5:1 (WCAG AA).
func lintContrastBody(r *LintReport, theme *Theme) {
	if theme.BgPrimary == "" || theme.FgPrimary == "" {
		return
	}
	ratio := contrastRatio(theme.BgPrimary, theme.FgPrimary)
	if ratio < 4.5 {
		r.addErr("U5", -1,
			fmt.Sprintf("bg/fg contrast is %.2f:1 (min WCAG AA is 4.5:1)", ratio),
			"adjust bg_primary or fg_primary to increase contrast",
		)
	}
}

// lintContrastHook implements rule C3: hook contrast (cover) ≥ 7:1.
// Only runs when there is a cover slide and the theme is available.
func lintContrastHook(r *LintReport, c *Carousel, theme *Theme) {
	if len(c.Slides) == 0 || c.Slides[0].Layout != "cover" {
		return
	}
	if theme.BgPrimary == "" || theme.FgPrimary == "" {
		return
	}
	ratio := contrastRatio(theme.BgPrimary, theme.FgPrimary)
	if ratio < 7.0 {
		r.addWarn("C3", 0,
			fmt.Sprintf("hook contrast on cover is %.2f:1; ideal is ≥7:1 for thumbnail legibility", ratio),
			"the cover is shown as a compressed thumbnail; high contrast is critical",
		)
	}
}

// ---------------------------------------------------------------------------
// Universal per-slide rule
// ---------------------------------------------------------------------------

// lintWordsPerSlide implements rule U9: ≤30 words of "reading" copy per slide.
//
// Labels, attributions, eyebrows and item bullets carry different cognitive
// weight than body prose — they are scanned, not read. We now warn over 35
// (still aggressive enough to flag a wall-of-text) and only error at >50,
// when the slide really has become a paragraph. Per-element rules (C1, L3,
// N2, Q1, S3, A5) carry the bulk of the discipline.
func lintWordsPerSlide(r *LintReport, idx int, s Slide) {
	total := slideWords(s)
	if total > 50 {
		r.addErr("U9", idx,
			fmt.Sprintf("%s has %d words total (cap 50)", slideKind(idx, s), total),
			"this is a paragraph, not a slide — split into two or move the explanation to the caption",
		)
	} else if total > 35 {
		r.addWarn("U9", idx,
			fmt.Sprintf("%s has %d words total (recommended ≤35)", slideKind(idx, s), total),
			"dense slides hurt swipe-through; consider splitting or trimming",
		)
	}
}

// lintSlide2Filler implements rule AP-04: slide 2 cannot be filler.
// A "filler" slide 2 is a "text" layout that mentions phrases like
// "in this carousel" or "I'll explain" — pure introduction without value.
func lintSlide2Filler(r *LintReport, idx int, s Slide, c *Carousel) {
	if idx != 1 {
		return
	}
	if s.Layout != "text" {
		return
	}
	fillerPhrases := []string{
		"in this carousel", "i'll explain", "i will explain",
		"i'll show", "i will show", "i'll talk", "i will talk",
		"i'll teach", "i will teach",
		"neste carrossel", "vou explicar", "vou mostrar",
		"vou falar", "nesse carrossel", "vou ensinar",
	}
	combined := strings.ToLower(s.Body + " " + s.Title)
	for _, phrase := range fillerPhrases {
		if strings.Contains(combined, phrase) {
			r.addWarn("AP-04", idx,
				"slide 2 looks like an empty intro ('in this carousel...', 'I'll explain...')",
				"slide 2 must deliver the first real value or deepen the hook immediately",
			)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Cover rules (C*)
// ---------------------------------------------------------------------------

// lintCover runs all cover-layout rules: C1 (hook words) and C6 (microcopy words).
//
// Research is split on the hook cap — Research A §10 Golden Rule #1 says ≤8,
// Research D and most LinkedIn references show 8–12. We warn at 12 (a stretch
// limit, allowed when extra words buy specificity like numbers or names) and
// only error past 16, where the cover stops working as a thumbnail.
func lintCover(r *LintReport, idx int, s Slide) {
	hw := countWords(s.Hook)
	if hw > 16 {
		r.addErr("C1", idx,
			fmt.Sprintf("hook has %d words (max 16)", hw),
			"a thumbnail-readable hook is ≤12 words; past 16 it stops stopping the scroll",
		)
	} else if hw > 12 {
		r.addWarn("C1", idx,
			fmt.Sprintf("hook has %d words (recommended ≤12)", hw),
			"consider H-01 (number + outcome) or H-13 (list with pain-point spoiler); stretch to 12+ only when extra words buy specificity",
		)
	}

	// C6: microcopy (Sub) ≤ 10 words — already a warning. Keep as-is.
	if sw := countWords(s.Sub); sw > 10 {
		r.addWarn("C6", idx,
			fmt.Sprintf("microcopy (sub) has %d words (recommended ≤10)", sw),
			"the cover subtitle should fit on a single line",
		)
	}
}

// ---------------------------------------------------------------------------
// List rules (L*)
// ---------------------------------------------------------------------------

// lintList runs list-layout rules.
//
// L1 was an error at >5. We now warn at 6 (the layout still renders cleanly
// thanks to length-responsive item sizing) and error at 8+, where items
// become too compressed to read. L3 stays a warning.
func lintList(r *LintReport, idx int, s Slide) {
	n := len(s.Items)
	if n > 7 {
		r.addErr("L1", idx,
			fmt.Sprintf("%s has %d items (max 7 per slide)", slideKind(idx, s), n),
			"split into two list slides so each item has room to breathe",
		)
	} else if n > 5 {
		r.addWarn("L1", idx,
			fmt.Sprintf("%s has %d items (recommended ≤5)", slideKind(idx, s), n),
			"5 items is the swipe-through sweet spot; consider splitting at 6+ for readability",
		)
	}

	for i, item := range s.Items {
		if iw := countWords(item); iw > 8 {
			r.addWarn("L3", idx,
				fmt.Sprintf("list item %d has %d words (recommended ≤8): %q", i+1, iw, item),
				"each list item should be a scannable line — cut it or split into two",
			)
		}
	}
}

// ---------------------------------------------------------------------------
// Big-number rules (N*)
// ---------------------------------------------------------------------------

// lintBigNumber runs big-number-layout rules.
//
// N2 (caption) is intentionally a WARNING, not a blocker. Research D's own
// canonical example reads "87% / of independent service providers lose
// clients due to lack of follow-up" — 14 words. The strict ≤8-word cap
// killed cold-readability ("25 MIN" / "pra zerar meu Claude Max." doesn't
// land without setup). For richer stories the agent now has `subhead` and
// `context` fields and the linter only nags at extreme lengths.
func lintBigNumber(r *LintReport, idx int, s Slide, theme *Theme) {
	// N2: caption — soft guidance now. Warn over 12, only ERR at clearly
	// runaway lengths (>20) where the number stops being the protagonist.
	if cw := countWords(s.Caption); cw > 20 {
		r.addErr("N2", idx,
			fmt.Sprintf("%s: caption has %d words (max 20)", slideKind(idx, s), cw),
			"if the explanation needs more than 20 words, move it to the `context` field or split into two slides",
		)
	} else if cw := countWords(s.Caption); cw > 12 {
		r.addWarn("N2", idx,
			fmt.Sprintf("%s: caption has %d words (recommended ≤12)", slideKind(idx, s), cw),
			"the number does the headline work; longer explanations belong in `context`",
		)
	}

	// N2b: subhead is the eyebrow above the number — keep it tight.
	if sw := countWords(s.Subhead); sw > 6 {
		r.addWarn("N2-subhead", idx,
			fmt.Sprintf("%s: subhead has %d words (recommended ≤6)", slideKind(idx, s), sw),
			"the subhead acts as a label above the number — keep it scannable",
		)
	}

	// N2c: context is the explanatory line below the caption. Allow more room.
	if cw := countWords(s.Context); cw > 20 {
		r.addWarn("N2-context", idx,
			fmt.Sprintf("%s: context has %d words (recommended ≤20)", slideKind(idx, s), cw),
			"context is your explainer slot but a long line still tires the eye — aim for ≤20",
		)
	}

	// N3: neutral background. Soft guidance — most presets ship without
	// background_effect anyway, and there is no per-slide override yet.
	if theme != nil && theme.BackgroundEffect != "" {
		r.addWarn("N3", idx,
			fmt.Sprintf("theme uses background_effect '%s'; big-number reads strongest on a neutral background", theme.BackgroundEffect),
			"consider an alternate theme for the big-number slide if the dots feel busy in your render",
		)
	}
}

// ---------------------------------------------------------------------------
// Quote rules (Q*)
// ---------------------------------------------------------------------------

// lintQuote runs quote-layout rules: Q1 (words ≤25) and Q3 (attribution required).
func lintQuote(r *LintReport, idx int, s Slide) {
	// Q1: quote body ≤ 25 words
	if qw := countWords(s.Quote); qw > 25 {
		r.addErr("Q1", idx,
			fmt.Sprintf("%s: quote has %d words (max 25)", slideKind(idx, s), qw),
			"trim the quote to at most 25 words; use '...' for ellipses if needed",
		)
	}

	// Q3: attribution (name + context) is required
	if strings.TrimSpace(s.Attribution) == "" {
		r.addErr("Q3", idx,
			fmt.Sprintf("%s: attribution field is empty", slideKind(idx, s)),
			"add attribution in 'Name, Role/Context' format to lend credibility",
		)
	}
}

// ---------------------------------------------------------------------------
// Comparison rules (R*)
// ---------------------------------------------------------------------------

// lintComparison runs comparison-layout rules: R2 (≤2 items per side),
// CM-04 (item-count parity) and CM-05 (labels must have textual context so
// the ✗/✓ icons read as a comparison and not as generic decoration).
//
// The old R1 ("labels must be present") was folded into CM-05 because the
// underlying check was identical and CM-05's message explains the visual
// failure mode the rule actually exists to prevent.
func lintComparison(r *LintReport, idx int, s Slide) {
	// R2: max 2 items per side
	if len(s.BeforeItems) > 2 {
		r.addErr("R2", idx,
			fmt.Sprintf("%s: 'before' side has %d items (max 2)", slideKind(idx, s), len(s.BeforeItems)),
			"use 2 comparison slides if you need to show more than 2 differences",
		)
	}
	if len(s.AfterItems) > 2 {
		r.addErr("R2", idx,
			fmt.Sprintf("%s: 'after' side has %d items (max 2)", slideKind(idx, s), len(s.AfterItems)),
			"use 2 comparison slides if you need to show more than 2 differences",
		)
	}

	// CM-04: item-count parity between the two sides.
	//
	// A comparison reads as a balanced "before / after" only when both
	// columns carry roughly the same weight. A delta of 1 is tolerable
	// (e.g. 2 vs 3) but anything wider — 1 vs 3, 0 vs 2 — looks lopsided
	// and the eye latches onto the extra row instead of the contrast.
	before := len(s.BeforeItems)
	after := len(s.AfterItems)
	delta := before - after
	if delta < 0 {
		delta = -delta
	}
	if delta > 1 {
		r.addWarn("CM-04", idx,
			fmt.Sprintf("%s: comparison has %d before / %d after items — keep within 1 for visual parity",
				slideKind(idx, s), before, after),
			"add or remove an item so both sides land within 1 of each other",
		)
	}

	// CM-05: labels are mandatory because the comparison layout renders ✗
	// and ✓ icons next to each side. Without a textual anchor the icons
	// read as generic decoration and the reader has to guess which side
	// is the "good" one.
	if strings.TrimSpace(s.BeforeLabel) == "" || strings.TrimSpace(s.AfterLabel) == "" {
		r.addErr("CM-05", idx,
			fmt.Sprintf("%s: comparison missing before_label or after_label — ✗/✓ icons need context",
				slideKind(idx, s)),
			"label each side with a 1-3 word tag (e.g. 'Before' / 'After', 'Manual' / 'Automated')",
		)
	}
}

// ---------------------------------------------------------------------------
// Screenshot rules (S*)
// ---------------------------------------------------------------------------

// lintScreenshot runs screenshot-layout rules: S2 (caption presente) and
// S3 (caption ≤16 palavras ≈ 2 linhas).
// S1 (image path exists) cannot be validated without the YAML file path at
// this stage — it is deferred to the renderer. See TODO in cmd_render.go.
func lintScreenshot(r *LintReport, idx int, s Slide) {
	// S2: caption present
	if strings.TrimSpace(s.Caption) == "" {
		r.addWarn("S2", idx,
			fmt.Sprintf("%s: caption is empty", slideKind(idx, s)),
			"add a caption explaining what the screenshot shows",
		)
	}

	// S3: caption ≤ 2 lines (~16 words)
	if cw := countWords(s.Caption); cw > 16 {
		r.addWarn("S3", idx,
			fmt.Sprintf("%s: caption has %d words (approx. >2 lines, max recommended ≤16)", slideKind(idx, s), cw),
			"screenshot caption must fit in 2 lines; cut to ≤16 words",
		)
	}
}

// ---------------------------------------------------------------------------
// CTA rules (A*)
// ---------------------------------------------------------------------------

// lintCTA runs CTA-layout rules: A1 (single action), A2 (specific verb),
// A5 (headline length).
//
// A5 is now a warning past 12 and only errors past 18 — the final slide is
// where you sometimes need a full sentence to set up the ask.
func lintCTA(r *LintReport, idx int, s Slide) {
	hw := countWords(s.Headline)
	if hw > 18 {
		r.addErr("A5", idx,
			fmt.Sprintf("%s: headline has %d words (max 18)", slideKind(idx, s), hw),
			"a CTA headline this long competes with the action box — split or trim",
		)
	} else if hw > 12 {
		r.addWarn("A5", idx,
			fmt.Sprintf("%s: headline has %d words (recommended ≤12)", slideKind(idx, s), hw),
			"shorter headlines convert better; cut to ≤12 if you can",
		)
	}

	// A1: keep the CTA to ONE action. Two verbs are tolerated only when
	// genuinely complementary (e.g. "save + DM me"); three+ is the usual
	// "save/comment/share/follow" wishlist that gets zero engagement.
	if verbCount := countCTAVerbs(s.CTAText); verbCount >= 3 {
		r.addErr("A1", idx,
			fmt.Sprintf("%s: cta_text has %d action verbs (max 2)", slideKind(idx, s), verbCount),
			"a single CTA per slide; at most 'Save' + 'DM me' when they are complementary",
		)
	} else if verbCount == 2 {
		r.addWarn("A1", idx,
			fmt.Sprintf("%s: cta_text has 2 action verbs", slideKind(idx, s)),
			"two verbs are fine only when complementary (save+share, save+DM) — otherwise pick one",
		)
	}

	// A2: specific CTA verb (not just "follow").
	if s.CTAText != "" && isGenericCTA(s.CTAText) {
		r.addWarn("A2", idx,
			fmt.Sprintf("%s: CTA '%s' uses a generic verb", slideKind(idx, s), s.CTAText),
			"use a specific high-conversion verb: 'Save', 'Comment WORD', 'DM me'",
		)
	}
}

// ---------------------------------------------------------------------------
// Spotlight rules (SP*)
// ---------------------------------------------------------------------------

// lintSpotlightOveruse implements rule SP-01: at most one `spotlight` tone
// per carousel.
//
// The spotlight tone is designed as a single moment-of-pause interstitial —
// an accent-color full-bleed slide that breaks the deck's rhythm and tells
// the reader "stop, this matters". A second spotlight reads as decoration;
// three or more turns the rhythmic device into noise and the eye stops
// registering it as a break at all.
//
// Severity escalates with usage:
//   - 1 spotlight  → silent (intended use)
//   - 2 spotlights → WARN at the second occurrence
//   - 3+ spotlights → ERR at every spotlight past the first
func lintSpotlightOveruse(r *LintReport, c *Carousel) {
	// Collect indices of spotlight slides in order.
	var spotIdx []int
	for i, s := range c.Slides {
		if s.Tone == "spotlight" {
			spotIdx = append(spotIdx, i)
		}
	}

	count := len(spotIdx)
	switch {
	case count <= 1:
		return
	case count == 2:
		// Warn at the second spotlight only — the first is the
		// intended interstitial.
		idx := spotIdx[1]
		r.addWarn("SP-01", idx,
			fmt.Sprintf("slide %d: spotlight tone used %d times (interstitials should be ≤1 per carousel)",
				idx+1, count),
			"keep one spotlight slide as the moment-of-pause; demote the rest to inherited theme",
		)
	default: // count >= 3
		// Error at every spotlight past the first.
		for _, idx := range spotIdx[1:] {
			r.addErr("SP-01", idx,
				fmt.Sprintf("slide %d: spotlight tone used %d times (interstitials should be ≤1 per carousel)",
					idx+1, count),
				"keep one spotlight slide as the moment-of-pause; demote the rest to inherited theme",
			)
		}
	}
}

// lintSpotlightBodyLen implements rule SP-02: text+spotlight body ≤ 12 words.
//
// When a slide combines `layout: text` with `tone: spotlight`, the renderer
// treats the Body as a pull-quote — oversized typography flanked by
// decorative quotation marks. The treatment only works when the body lands
// in 1–2 visual lines; beyond ~12 words the quotation marks crash into the
// text and the slide looks cramped, the opposite of the airy "pause" the
// spotlight is supposed to provide.
func lintSpotlightBodyLen(r *LintReport, idx int, s Slide) {
	if s.Layout != "text" || s.Tone != "spotlight" {
		return
	}
	bw := countWords(s.Body)
	if bw > 12 {
		r.addWarn("SP-02", idx,
			fmt.Sprintf("slide %d: text+spotlight body is %d words (≤12 recommended for pull-quote impact)",
				idx+1, bw),
			"trim to a pull-quote-length line, or drop the spotlight tone and use a regular text slide",
		)
	}
}
