package main

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// ptr returns a pointer to the given bool value.
func ptr(b bool) *bool { return &b }

// ---------------------------------------------------------------------------
// Helper: collect issue codes by severity from a report.
// ---------------------------------------------------------------------------

func errCodes(r LintReport) []string {
	var codes []string
	for _, issue := range r.Issues {
		if issue.Severity == SeverityErr {
			codes = append(codes, issue.Code)
		}
	}
	return codes
}

func warnCodes(r LintReport) []string {
	var codes []string
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarn {
			codes = append(codes, issue.Code)
		}
	}
	return codes
}

func hasCode(r LintReport, code string) bool {
	for _, issue := range r.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// darkTech returns a minimal theme that passes U5 / C3 contrast checks.
// Contrast ratio of #0D1117 vs #FFFFFF ≈ 19:1.
// ---------------------------------------------------------------------------

func darkTech() *Theme {
	return &Theme{
		Name:        "dark-tech",
		BgPrimary:   "#0D1117",
		FgPrimary:   "#FFFFFF",
		Accent:      "#11C47E",
		FontHeading: "Outfit",
		FontBody:    "DM Sans",
	}
}

// ---------------------------------------------------------------------------
// C1: Hook 13-16 words → WARN, >16 → ERR. Loosened from the old strict
// ≤12 ERR after audit F-3.something — research splits on 8 vs 12 as the
// hard cap, and forcing ERR at 13 was too aggressive for hooks where
// extra words buy specificity (numbers, named entities).
// ---------------------------------------------------------------------------

func TestLint_C1_HookWarnAt13(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "This is a hook with thirteen words inside the carousel right here today"},
		},
	}
	r := LintCarousel(c, darkTech())
	found := false
	for _, issue := range r.Issues {
		if issue.Code == "C1" && issue.Severity == SeverityWarn {
			found = true
		}
	}
	if !found {
		t.Errorf("expected C1 WARN for 13-word hook; issues=%+v", r.Issues)
	}
}

func TestLint_C1_HookErrPast16(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "This is an absurdly long hook with way more than sixteen words and it keeps going further still"},
		},
	}
	r := LintCarousel(c, darkTech())
	found := false
	for _, issue := range r.Issues {
		if issue.Code == "C1" && issue.Severity == SeverityErr {
			found = true
		}
	}
	if !found {
		t.Errorf("expected C1 ERR for >16-word hook; issues=%+v", r.Issues)
	}
}

// Hook with exactly 12 words → no C1 issue.
func TestLint_C1_HookExactly12(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "One two three four five six seven eight nine ten eleven twelve"},
		},
	}
	r := LintCarousel(c, darkTech())
	if hasCode(r, "C1") {
		t.Error("expected no C1 for hook with exactly 12 words")
	}
}

// ---------------------------------------------------------------------------
// L1: 6-7 items → WARN, ≥8 items → ERR. Loosened so 6-item lists render
// (templates now scale font with item count) and only over-stuffed slides
// block render.
// ---------------------------------------------------------------------------

func TestLint_L1_WarnAt6Items(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{Layout: "list", Title: "Title", Items: []string{
				"Item 1", "Item 2", "Item 3", "Item 4", "Item 5", "Item 6",
			}},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	found := false
	for _, issue := range r.Issues {
		if issue.Code == "L1" && issue.Severity == SeverityWarn {
			found = true
		}
	}
	if !found {
		t.Errorf("expected L1 WARN for 6-item list; issues=%+v", r.Issues)
	}
}

func TestLint_L1_ErrAt8Items(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{Layout: "list", Title: "Title", Items: []string{
				"Item 1", "Item 2", "Item 3", "Item 4", "Item 5", "Item 6", "Item 7", "Item 8",
			}},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	found := false
	for _, issue := range r.Issues {
		if issue.Code == "L1" && issue.Severity == SeverityErr {
			found = true
		}
	}
	if !found {
		t.Errorf("expected L1 ERR for 8-item list; issues=%+v", r.Issues)
	}
}

// ---------------------------------------------------------------------------
// N2: big-number caption is now WARN past 12 and ERR past 20. The old
// strict ≤8 ERR fought Research D's own canonical example ("87% / of
// independent service providers lose clients due to lack of follow-up"
// is 14 words). Cold-readability beats the linter on this one.
// ---------------------------------------------------------------------------

func TestLint_N2_CaptionWarnAt13(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook here"},
			{Layout: "big-number", Number: "87%", Caption: "of independent service providers lose clients every year due to lack of follow up systems"},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	found := false
	for _, issue := range r.Issues {
		if issue.Code == "N2" && issue.Severity == SeverityWarn {
			found = true
		}
	}
	if !found {
		t.Errorf("expected N2 WARN for 13-word caption; issues=%+v", r.Issues)
	}
}

func TestLint_N2_CaptionErrPast20(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook here"},
			{Layout: "big-number", Number: "87%", Caption: "of independent service providers in tech and SaaS lose clients each year due to a total lack of automated follow up systems and processes"},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	found := false
	for _, issue := range r.Issues {
		if issue.Code == "N2" && issue.Severity == SeverityErr {
			found = true
		}
	}
	if !found {
		t.Errorf("expected N2 ERR for >20-word caption; issues=%+v", r.Issues)
	}
}

// ---------------------------------------------------------------------------
// AP-07: carousel without CTA as last slide → WARN
// ---------------------------------------------------------------------------

func TestLint_AP07_NoCTAAtEnd(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{Layout: "list", Title: "List", Items: []string{"Item 1", "Item 2"}},
			{Layout: "text", Body: "Free text here with no cta at the end"},
		},
	}
	r := LintCarousel(c, darkTech())
	if !hasCode(r, "AP-07") {
		t.Errorf("expected AP-07 WARN for carousel not ending with cta; warnCodes=%v", warnCodes(r))
	}
	for _, issue := range r.Issues {
		if issue.Code == "AP-07" && issue.Severity != SeverityWarn {
			t.Error("AP-07 must be SeverityWarn")
		}
	}
}

// ---------------------------------------------------------------------------
// A1: CTA with 3+ distinct imperative verbs → ERR
// ---------------------------------------------------------------------------

func TestLint_A1_MultipleCTAVerbs(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook here"},
			{
				Layout:   "cta",
				Headline: "Now it's on you",
				CTAText:  "Save, comment and share this content now",
			},
		},
	}
	r := LintCarousel(c, darkTech())
	if !hasCode(r, "A1") {
		t.Errorf("expected A1 ERR for CTA with 3+ verbs; issues=%+v", r.Issues)
	}
	for _, issue := range r.Issues {
		if issue.Code == "A1" && issue.Severity != SeverityErr {
			t.Error("A1 must be SeverityErr")
		}
	}
}

// ---------------------------------------------------------------------------
// U5: low contrast theme → ERR
// ---------------------------------------------------------------------------

func TestLint_U5_LowContrast(t *testing.T) {
	// White bg (#FFFFFF) vs light grey fg (#CCCCCC) — very low contrast.
	lowContrastTheme := &Theme{
		Name:        "low-contrast",
		BgPrimary:   "#FFFFFF",
		FgPrimary:   "#CCCCCC",
		Accent:      "#11C47E",
		FontHeading: "Outfit",
		FontBody:    "DM Sans",
	}
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook here"},
			{Layout: "cta", Headline: "Save this now", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, lowContrastTheme)
	if !hasCode(r, "U5") {
		t.Errorf("expected U5 ERR for low contrast theme; issues=%+v", r.Issues)
	}
	for _, issue := range r.Issues {
		if issue.Code == "U5" && issue.Severity != SeverityErr {
			t.Error("U5 must be SeverityErr")
		}
	}
}

// ---------------------------------------------------------------------------
// AN-01: 5 slides → WARN (4-7 range)
// ---------------------------------------------------------------------------

func TestLint_AN01_FiveSlides(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{Layout: "list", Title: "List", Items: []string{"Item 1"}},
			{Layout: "big-number", Number: "87%", Caption: "of providers"},
			{Layout: "quote", Quote: "A quote here inside", Attribution: "Name"},
			{Layout: "cta", Headline: "Save this carousel", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	if !hasCode(r, "AN-01") {
		t.Errorf("expected AN-01 WARN for 5-slide carousel; warnCodes=%v", warnCodes(r))
	}
	for _, issue := range r.Issues {
		if issue.Code == "AN-01" && issue.Severity != SeverityWarn {
			t.Error("AN-01 must be SeverityWarn")
		}
	}
}

// ---------------------------------------------------------------------------
// Golden path: 9 slides with good structure → 0 linter errors.
// Slides: cover, big-number, list, list, list, quote, list, screenshot, cta
// ---------------------------------------------------------------------------

func TestLint_GoldenPath_NoErrors(t *testing.T) {
	c := &Carousel{
		Handle:   "@lybel.com.br",
		Platform: "instagram-4x5",
		Slides: []Slide{
			// 1 cover
			{
				Layout: "cover",
				Hook:   "Seven mistakes that kill your carousel",
				Sub:    "See what to avoid now",
			},
			// 2 big-number (value bomb candidate — not text/cover)
			{
				Layout:  "big-number",
				Number:  "87%",
				Caption: "of carousels fail at conversion",
			},
			// 3 list (slide 3 = value bomb: OK layout)
			{
				Layout: "list",
				Title:  "Most common mistakes",
				Items:  []string{"Weak hook", "Slide 2 filler", "No CTA"},
			},
			// 4 list
			{
				Layout: "list",
				Title:  "What works",
				Items:  []string{"Hook with number", "Value bomb on slide 3", "Single CTA"},
			},
			// 5 list
			{
				Layout: "list",
				Title:  "Quick checklist",
				Items:  []string{"High contrast", "Max 30 words", "Visible handle"},
			},
			// 6 quote
			{
				Layout:      "quote",
				Quote:       "Good design isn't what looks pretty but what converts attention into action",
				Attribution: "Justin Welsh, LinkedIn 500k+",
			},
			// 7 list
			{
				Layout: "list",
				Title:  "Recommended fonts",
				Items:  []string{"Outfit + DM Sans", "Space Grotesk + Inter"},
			},
			// 8 screenshot
			{
				Layout:  "screenshot",
				Image:   "result.png",
				Caption: "Real result from an optimized carousel",
			},
			// 9 cta (last)
			{
				Layout:   "cta",
				Headline: "Save it for your next carousel",
				CTAText:  "Save this post now",
			},
		},
	}
	r := LintCarousel(c, darkTech())
	if r.ErrCount > 0 {
		t.Errorf("golden-path carousel should have 0 errors; got %d: %+v", r.ErrCount, r.Issues)
	}
}

// ---------------------------------------------------------------------------
// contrastRatio: unit tests for the WCAG helper.
// ---------------------------------------------------------------------------

func TestContrastRatio(t *testing.T) {
	tests := []struct {
		hex1, hex2 string
		wantMin    float64
	}{
		{"#FFFFFF", "#000000", 21.0}, // max contrast
		{"#0D1117", "#FFFFFF", 18.0}, // near-max, dark-tech theme
		{"#FFFFFF", "#CCCCCC", 1.0},  // low contrast
	}
	for _, tt := range tests {
		ratio := contrastRatio(tt.hex1, tt.hex2)
		if ratio < tt.wantMin {
			t.Errorf("contrastRatio(%q, %q) = %.2f; want >= %.1f", tt.hex1, tt.hex2, ratio, tt.wantMin)
		}
	}
}

// ---------------------------------------------------------------------------
// A2: generic CTA verb ("Follow the profile") → WARN
// ---------------------------------------------------------------------------

func TestLint_A2_GenericCTA(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook here"},
			{
				Layout:   "cta",
				Headline: "Liked the content",
				CTAText:  "Follow the profile for more tips",
			},
		},
	}
	r := LintCarousel(c, darkTech())
	if !hasCode(r, "A2") {
		t.Errorf("expected A2 WARN for generic CTA verb; issues=%+v", r.Issues)
	}
	for _, issue := range r.Issues {
		if issue.Code == "A2" && issue.Severity != SeverityWarn {
			t.Error("A2 must be SeverityWarn")
		}
	}
}

// ---------------------------------------------------------------------------
// Q3: missing attribution → ERR
// ---------------------------------------------------------------------------

func TestLint_Q3_MissingAttribution(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook here"},
			{
				Layout: "quote",
				Quote:  "Good design converts attention into action",
				// Attribution is empty
			},
			{Layout: "cta", Headline: "Save this content", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	if !hasCode(r, "Q3") {
		t.Errorf("expected Q3 ERR for missing attribution; issues=%+v", r.Issues)
	}
	for _, issue := range r.Issues {
		if issue.Code == "Q3" && issue.Severity != SeverityErr {
			t.Error("Q3 must be SeverityErr")
		}
	}
}

// ---------------------------------------------------------------------------
// RH-01: 5 consecutive slides of the same Layout → WARN
// ---------------------------------------------------------------------------

func TestLint_RH01_MonotoneLayouts(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{Layout: "list", Title: "L1", Items: []string{"a"}},
			{Layout: "list", Title: "L2", Items: []string{"a"}},
			{Layout: "list", Title: "L3", Items: []string{"a"}},
			{Layout: "list", Title: "L4", Items: []string{"a"}},
			{Layout: "list", Title: "L5", Items: []string{"a"}},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	if !hasCode(r, "RH-01") {
		t.Errorf("expected RH-01 WARN for 5 consecutive list slides; warnCodes=%v", warnCodes(r))
	}
	for _, issue := range r.Issues {
		if issue.Code == "RH-01" && issue.Severity != SeverityWarn {
			t.Error("RH-01 must be SeverityWarn")
		}
	}
}

// RH-01 must NOT fire when layouts are well mixed.
func TestLint_RH01_MixedLayouts_NoWarn(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{Layout: "list", Title: "L1", Items: []string{"a"}},
			{Layout: "big-number", Number: "9", Caption: "nine"},
			{Layout: "list", Title: "L2", Items: []string{"a"}},
			{Layout: "quote", Quote: "q", Attribution: "x"},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	if hasCode(r, "RH-01") {
		t.Errorf("did not expect RH-01 for well-mixed deck; issues=%+v", r.Issues)
	}
}

// ---------------------------------------------------------------------------
// Slide.Tone / Slide.HookStyle: schema-decode tests for the new fields.
// ---------------------------------------------------------------------------

func TestSlide_Tone_Accepts_Valid_Values(t *testing.T) {
	for _, tone := range []string{"authority", "clarity", "spotlight"} {
		src := []byte(
			"layout: cover\nhook: \"Hi\"\ntone: " + tone + "\n",
		)
		var s Slide
		if err := yaml.Unmarshal(src, &s); err != nil {
			t.Fatalf("yaml.Unmarshal tone=%q: %v", tone, err)
		}
		if s.Tone != tone {
			t.Errorf("Slide.Tone = %q, want %q", s.Tone, tone)
		}
		if s.Layout != "cover" {
			t.Errorf("Slide.Layout = %q, want %q", s.Layout, "cover")
		}
	}

	// Empty tone (default) decodes to "".
	var s Slide
	if err := yaml.Unmarshal([]byte("layout: cover\nhook: \"Hi\"\n"), &s); err != nil {
		t.Fatalf("yaml.Unmarshal no tone: %v", err)
	}
	if s.Tone != "" {
		t.Errorf("default Slide.Tone = %q, want \"\"", s.Tone)
	}
}

// ---------------------------------------------------------------------------
// SP-01: spotlight tone overuse.
//   - 1 spotlight  → no issue
//   - 2 spotlights → WARN at the second
//   - 3+ spotlights → ERR at every spotlight past the first
// ---------------------------------------------------------------------------

func TestLint_SP01_SingleSpotlight_NoIssue(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{Layout: "text", Body: "An interstitial line", Tone: "spotlight"},
			{Layout: "list", Title: "T", Items: []string{"a"}},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	if hasCode(r, "SP-01") {
		t.Errorf("did not expect SP-01 for single spotlight; issues=%+v", r.Issues)
	}
}

func TestLint_SP01_TwoSpotlights_Warn(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{Layout: "text", Body: "First pause", Tone: "spotlight"},
			{Layout: "list", Title: "T", Items: []string{"a"}},
			{Layout: "text", Body: "Second pause", Tone: "spotlight"},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	found := false
	for _, issue := range r.Issues {
		if issue.Code == "SP-01" && issue.Severity == SeverityWarn {
			found = true
		}
		if issue.Code == "SP-01" && issue.Severity == SeverityErr {
			t.Errorf("SP-01 should be WARN at count=2, not ERR; issue=%+v", issue)
		}
	}
	if !found {
		t.Errorf("expected SP-01 WARN for 2 spotlights; issues=%+v", r.Issues)
	}
}

func TestLint_SP01_ThreeSpotlights_Err(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{Layout: "text", Body: "First", Tone: "spotlight"},
			{Layout: "text", Body: "Second", Tone: "spotlight"},
			{Layout: "text", Body: "Third", Tone: "spotlight"},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	errCount := 0
	for _, issue := range r.Issues {
		if issue.Code == "SP-01" && issue.Severity == SeverityErr {
			errCount++
		}
	}
	// Expect ERR at every spotlight past the first → 2 errors for 3 spotlights.
	if errCount != 2 {
		t.Errorf("expected 2 SP-01 ERRs for 3 spotlights; got %d; issues=%+v", errCount, r.Issues)
	}
}

// ---------------------------------------------------------------------------
// SP-02: text+spotlight body length.
// >12 words on a text+spotlight slide → WARN. Other layouts/tones don't fire.
// ---------------------------------------------------------------------------

func TestLint_SP02_TextSpotlightLongBody_Warn(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{
				Layout: "text",
				Tone:   "spotlight",
				Body:   "This pull quote stretches well beyond twelve words and the decorative marks start crashing into the text now",
			},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	found := false
	for _, issue := range r.Issues {
		if issue.Code == "SP-02" && issue.Severity == SeverityWarn {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SP-02 WARN for long text+spotlight body; issues=%+v", r.Issues)
	}
}

func TestLint_SP02_TextSpotlightShortBody_NoIssue(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{Layout: "text", Tone: "spotlight", Body: "Short pull quote."},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	if hasCode(r, "SP-02") {
		t.Errorf("did not expect SP-02 for short body; issues=%+v", r.Issues)
	}
}

// Long body on a text slide WITHOUT spotlight → no SP-02.
func TestLint_SP02_TextWithoutSpotlight_NoIssue(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{
				Layout: "text",
				Body:   "This body has many words but no spotlight tone applied at all here today",
			},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	if hasCode(r, "SP-02") {
		t.Errorf("did not expect SP-02 without spotlight tone; issues=%+v", r.Issues)
	}
}

// ---------------------------------------------------------------------------
// CM-04: comparison item-count parity. Delta >1 → WARN.
// ---------------------------------------------------------------------------

func TestLint_CM04_AsymmetricItems_Warn(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{
				Layout:      "comparison",
				BeforeLabel: "Manual",
				BeforeItems: []string{"a"},
				AfterLabel:  "Auto",
				// 1 vs 0 would be delta=1 — tolerable. Use 0 vs 2 → delta=2.
				AfterItems: []string{"b", "c"},
			},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	// Adjust to 0 vs 2 by clearing BeforeItems:
	c.Slides[1].BeforeItems = nil
	r := LintCarousel(c, darkTech())
	found := false
	for _, issue := range r.Issues {
		if issue.Code == "CM-04" && issue.Severity == SeverityWarn {
			found = true
		}
	}
	if !found {
		t.Errorf("expected CM-04 WARN for 0 vs 2 items; issues=%+v", r.Issues)
	}
}

func TestLint_CM04_DeltaOne_NoIssue(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{
				Layout:      "comparison",
				BeforeLabel: "Manual",
				BeforeItems: []string{"a"},
				AfterLabel:  "Auto",
				AfterItems:  []string{"b", "c"},
			},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	if hasCode(r, "CM-04") {
		t.Errorf("did not expect CM-04 for delta=1; issues=%+v", r.Issues)
	}
}

// ---------------------------------------------------------------------------
// CM-05: comparison labels required → ERR.
// (Replaces the legacy R1 "both sides present" rule, which checked the same
// condition with a less actionable message.)
// ---------------------------------------------------------------------------

func TestLint_CM05_MissingLabels_Err(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{
				Layout:      "comparison",
				BeforeItems: []string{"a"},
				AfterItems:  []string{"b"},
				// BeforeLabel and AfterLabel intentionally empty
			},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	found := false
	for _, issue := range r.Issues {
		if issue.Code == "CM-05" && issue.Severity == SeverityErr {
			found = true
		}
	}
	if !found {
		t.Errorf("expected CM-05 ERR for missing labels; issues=%+v", r.Issues)
	}
}

func TestLint_CM05_LabelsPresent_NoIssue(t *testing.T) {
	c := &Carousel{
		Handle: "@test",
		Slides: []Slide{
			{Layout: "cover", Hook: "Short hook"},
			{
				Layout:      "comparison",
				BeforeLabel: "Manual",
				BeforeItems: []string{"a"},
				AfterLabel:  "Auto",
				AfterItems:  []string{"b"},
			},
			{Layout: "cta", Headline: "Save", CTAText: "Save"},
		},
	}
	r := LintCarousel(c, darkTech())
	if hasCode(r, "CM-05") {
		t.Errorf("did not expect CM-05 with both labels present; issues=%+v", r.Issues)
	}
}

func TestSlide_HookStyle_Decodes(t *testing.T) {
	src := []byte("layout: cover\nhook: \"Hi\"\nhook_style: gradient\n")
	var s Slide
	if err := yaml.Unmarshal(src, &s); err != nil {
		t.Fatalf("yaml.Unmarshal hook_style: %v", err)
	}
	if s.HookStyle != "gradient" {
		t.Errorf("Slide.HookStyle = %q, want %q", s.HookStyle, "gradient")
	}
}
