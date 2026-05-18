// Package main — linter defines the engine and types for the viral-carousel
// linter. It encodes 32 rules derived from marketing research in
// _research/D-design-system-viral.md and _research/A-anatomia-viral.md.
//
// The linter runs synchronously and returns a [LintReport]. Callers decide
// whether to block rendering based on ErrCount > 0.
package main


// Severity classifies how critical a lint finding is.
type Severity int

const (
	// SeverityWarn is a suggestion that does not block rendering.
	SeverityWarn Severity = iota
	// SeverityErr is an error that blocks render unless --force is given.
	SeverityErr
)

// LintIssue is a single finding produced by the linter.
type LintIssue struct {
	// Code identifies the rule, e.g. "U9", "C1", "AP-04".
	Code string

	// Severity determines whether the issue blocks rendering.
	Severity Severity

	// SlideIdx is the 0-based index of the offending slide.
	// Use -1 for carousel-level (global) issues.
	SlideIdx int

	// Message is a human-readable description.
	// Example: "hook has 18 words (max 12)".
	Message string

	// Hint is an optional actionable tip, e.g. "use formula H-01 or H-13".
	Hint string
}

// LintReport is the aggregated result returned by [LintCarousel].
type LintReport struct {
	Issues    []LintIssue
	ErrCount  int
	WarnCount int
}

// addErr appends an error-level issue and increments ErrCount.
func (r *LintReport) addErr(code string, idx int, msg, hint string) {
	r.Issues = append(r.Issues, LintIssue{
		Code:     code,
		Severity: SeverityErr,
		SlideIdx: idx,
		Message:  msg,
		Hint:     hint,
	})
	r.ErrCount++
}

// addWarn appends a warning-level issue and increments WarnCount.
func (r *LintReport) addWarn(code string, idx int, msg, hint string) {
	r.Issues = append(r.Issues, LintIssue{
		Code:     code,
		Severity: SeverityWarn,
		SlideIdx: idx,
		Message:  msg,
		Hint:     hint,
	})
	r.WarnCount++
}

// LintCarousel runs all 32 viral-carousel rules against c and the resolved
// theme, returning a [LintReport]. The carousel must already be loaded (non-nil).
// If theme is nil the color-contrast rules are silently skipped.
func LintCarousel(c *Carousel, theme *Theme) LintReport {
	var r LintReport

	// --- carousel-level rules (idx = -1) ---
	lintSlideCount(&r, c)
	lintCoverFirst(&r, c)
	lintSlide3ValueBomb(&r, c)
	lintLastSlideCTA(&r, c)
	lintHandle(&r, c)
	lintRhythm(&r, c)
	lintSpotlightOveruse(&r, c)
	lintFontCount(&r, theme)
	if theme != nil {
		lintContrastBody(&r, theme)
		lintContrastHook(&r, c, theme)
	}

	// --- per-slide rules ---
	for i, s := range c.Slides {
		lintWordsPerSlide(&r, i, s)
		lintSlide2Filler(&r, i, s, c)
		lintSpotlightBodyLen(&r, i, s)

		switch s.Layout {
		case "cover":
			lintCover(&r, i, s)
		case "list":
			lintList(&r, i, s)
		case "big-number":
			lintBigNumber(&r, i, s, theme)
		case "quote":
			lintQuote(&r, i, s)
		case "comparison":
			lintComparison(&r, i, s)
		case "screenshot":
			lintScreenshot(&r, i, s)
		case "cta":
			lintCTA(&r, i, s)
		}
	}

	return r
}
