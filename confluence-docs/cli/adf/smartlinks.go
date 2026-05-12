// Package adf — Smart Link markdown detection helpers.
//
// Smart links are ADF nodes (blockCard, embedCard) that replace ordinary
// markdown images and standalone URL lines in the converter pipeline.
//
// Detection rules (applied at the line level, before goldmark parsing):
//
//  1. `![embed](URL)` — image with alt-text "embed" → embedCard(URL, "wide")
//  2. A line that contains only `[text](URL)` and nothing else → blockCard(URL)
//  3. A line that is only a bare URL (no brackets) → blockCard(URL)
//  4. URL in the middle of a paragraph → stays as a regular link (no change)
//
// The preprocessor lifts matching lines into macros before goldmark sees them,
// following the same placeholder mechanism used for TOC/panel/expand.
package adf

import (
	"regexp"
	"strings"
)

var (
	// embedImageRe matches ![embed](URL) where alt is exactly "embed" (case-insensitive).
	embedImageRe = regexp.MustCompile(`(?i)^!\[embed\]\(([^)]+)\)\s*$`)

	// standaloneMarkdownLinkRe matches a line that is only [text](URL) — a single
	// markdown link with optional surrounding whitespace.
	standaloneMarkdownLinkRe = regexp.MustCompile(`^\[([^\]]*)\]\(([^)]+)\)\s*$`)

	// bareURLRe matches a line that is only a bare URL (http/https).
	bareURLRe = regexp.MustCompile(`^https?://\S+$`)
)

// classifySmartLinkLine examines a single trimmed markdown line and returns
// the smart link action to take, or (smartLinkNone, "") if no action applies.
type smartLinkKind int

const (
	smartLinkNone  smartLinkKind = iota
	smartLinkEmbed               // embedCard
	smartLinkBlock               // blockCard
)

// classifySmartLinkLine returns the kind of smart link for this trimmed line
// and the URL to use, or (smartLinkNone, "").
//
// Rules applied (in order):
//
//  1. `![embed](URL)` — image with alt "embed" (case-insensitive) → embedCard.
//  2. Bare URL line (`https://...` with nothing else) → blockCard.
//  3. Standalone `[text](URL)` where text == URL or text is "" → blockCard.
//     (Excludes normal named links like [click here](url) to avoid breaking
//     prose links placed on their own paragraph.)
func classifySmartLinkLine(line string) (smartLinkKind, string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return smartLinkNone, ""
	}

	// Rule 1: ![embed](URL)
	if m := embedImageRe.FindStringSubmatch(trimmed); m != nil {
		return smartLinkEmbed, strings.TrimSpace(m[1])
	}

	// Rule 2: bare URL line
	if bareURLRe.MatchString(trimmed) {
		return smartLinkBlock, trimmed
	}

	// Rule 3: standalone [text](URL) where text == URL (i.e. auto-pasted URL
	// wrapped in link syntax) or text is empty.
	if m := standaloneMarkdownLinkRe.FindStringSubmatch(trimmed); m != nil {
		linkText := strings.TrimSpace(m[1])
		linkURL := strings.TrimSpace(m[2])
		if linkText == "" || linkText == linkURL {
			return smartLinkBlock, linkURL
		}
	}

	return smartLinkNone, ""
}

// preprocessSmartLinks scans markdown source lines and lifts smart link lines
// out, replacing them with macro placeholders. It is called inside preprocess()
// BEFORE the normal TOC/panel/expand scan so that smart links are captured
// first.
//
// Returns the modified source and appends new macros to the provided slice.
// The returned source has placeholders in place of smart link lines.
func preprocessSmartLinks(lines []string, macros []macro) ([]string, []macro) {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		kind, url := classifySmartLinkLine(line)
		switch kind {
		case smartLinkEmbed:
			idx := len(macros)
			capturedURL := url
			macros = append(macros, macro{
				kind: "embedCard",
				render: func() Node {
					return EmbedCard(capturedURL, "wide")
				},
			})
			out = append(out, placeholder(idx))
		case smartLinkBlock:
			idx := len(macros)
			capturedURL := url
			macros = append(macros, macro{
				kind: "blockCard",
				render: func() Node {
					return BlockCard(capturedURL)
				},
			})
			out = append(out, placeholder(idx))
		default:
			out = append(out, line)
		}
	}
	return out, macros
}
