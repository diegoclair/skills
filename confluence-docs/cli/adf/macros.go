package adf

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Confluence macro pre-processing.
//
// Goldmark doesn't natively understand `[TOC]` lines or fenced `:::` container
// blocks, so we lift them out of the markdown before parsing. Each macro is
// replaced with a placeholder line that the converter recognises during AST
// walking and substitutes for the appropriate ADF node.
//
// We use placeholders rather than custom goldmark parsers because:
//   - Container blocks need recursive parsing of their inner markdown, which is
//     simpler to express by re-invoking the converter on the captured body.
//   - Keeps goldmark integration small and avoids subtle parser conflicts with
//     CommonMark indented code blocks and blockquotes.

const (
	macroPlaceholderPrefix = "%%LYBELDOC_MACRO_"
	macroPlaceholderSuffix = "%%"
)

// macro represents a single pre-processed Confluence macro. The converter
// replaces placeholder paragraphs with the ADF node returned by render().
type macro struct {
	kind   string // "toc", "expand", "panel"
	render func() Node
}

// preprocess scans the markdown source, lifts macros out, and returns:
//   - rewritten markdown with placeholder lines in place of each macro
//   - an ordered list of macros (placeholder index N -> macros[N])
//   - an error if a `:::` block is unterminated.
func preprocess(src string) (string, []macro, error) {
	lines := strings.Split(src, "\n")
	var macros []macro

	// Phase 0: lift smart link lines (![embed](...), standalone [text](url),
	// bare URL lines) into placeholders BEFORE any other preprocessing.
	// This ensures they don't get picked up by goldmark as images/links.
	lines, macros = preprocessSmartLinks(lines, macros)

	var out strings.Builder

	tocRe := regexp.MustCompile(`^\s*\[TOC(?:\s+([^\]]*))?\]\s*$`)
	openRe := regexp.MustCompile(`^:::\s*([a-zA-Z_-]+)(?:\s+(.*))?$`)
	closeRe := regexp.MustCompile(`^:::\s*$`)

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if m := tocRe.FindStringSubmatch(line); m != nil {
			minL, maxL := parseTOCParams(m[1])
			idx := len(macros)
			macros = append(macros, macro{
				kind:   "toc",
				render: func() Node { return TOC(minL, maxL) },
			})
			out.WriteString(placeholder(idx))
			out.WriteString("\n")
			continue
		}

		if m := openRe.FindStringSubmatch(line); m != nil && !closeRe.MatchString(line) {
			kind := strings.ToLower(strings.TrimSpace(m[1]))
			title := strings.TrimSpace(m[2])

			// Collect the body until a matching closing `:::`. We don't support
			// nesting of `:::` blocks (Confluence doesn't either) so a flat
			// scan is enough.
			var body []string
			closed := false
			for j := i + 1; j < len(lines); j++ {
				if closeRe.MatchString(lines[j]) {
					i = j
					closed = true
					break
				}
				body = append(body, lines[j])
			}
			if !closed {
				return "", nil, fmt.Errorf("unterminated ::: block opened at line %d (%s)", i+1, kind)
			}

			inner := strings.Join(body, "\n")
			idx := len(macros)

			// :::properties block → page-properties macro (storage XML).
			// It produces a "rawStorage" pseudo-node that the converter wraps
			// in a codeBlock with language "confluence-storage" so the storage
			// XML passes through the ADF pipeline without being parsed as ADF.
			// Callers using page create/upload with --markdown will need to be
			// aware that the resulting ADF wraps raw XML — this is the safest
			// round-trip for storage macros.
			if kind == "properties" {
				capturedInner := inner
				macros = append(macros, macro{
					kind: "properties",
					render: func() Node {
						xml := PropertiesBlockToStorageXML(capturedInner)
						if xml == "" {
							return Paragraph(Text("(empty properties block)"))
						}
						// Use a code block with a special language tag to signal
						// that this is raw storage XML. The CLI's page create/upload
						// path can later detect and handle this, or callers can use
						// the storage representation directly.
						return CodeBlock("confluence-storage", xml)
					},
				})
				out.WriteString(placeholder(idx))
				out.WriteString("\n")
				continue
			}

			macros = append(macros, macro{
				kind: kind,
				render: func() Node {
					innerDoc, err := convertString(inner)
					if err != nil {
						// Fall back to a paragraph with the raw body so we
						// don't lose content if the inner parse hiccups.
						return Paragraph(Text(inner))
					}
					switch kind {
					case "expand":
						return Expand(title, innerDoc.Content...)
					case "info", "warning", "note", "success", "error":
						return Panel(kind, title, innerDoc.Content...)
					default:
						// Unknown container -> render as a generic note panel
						// so the content surfaces visibly in Confluence.
						return Panel("note", title, innerDoc.Content...)
					}
				},
			})
			out.WriteString(placeholder(idx))
			out.WriteString("\n")
			continue
		}

		out.WriteString(line)
		if i < len(lines)-1 {
			out.WriteString("\n")
		}
	}

	return out.String(), macros, nil
}

// placeholder returns the unique sentinel paragraph for macro index idx.
func placeholder(idx int) string {
	return macroPlaceholderPrefix + strconv.Itoa(idx) + macroPlaceholderSuffix
}

// matchPlaceholder returns (idx, true) if s is exactly a macro placeholder.
func matchPlaceholder(s string) (int, bool) {
	if !strings.HasPrefix(s, macroPlaceholderPrefix) || !strings.HasSuffix(s, macroPlaceholderSuffix) {
		return 0, false
	}
	mid := s[len(macroPlaceholderPrefix) : len(s)-len(macroPlaceholderSuffix)]
	idx, err := strconv.Atoi(mid)
	if err != nil {
		return 0, false
	}
	return idx, true
}

// parseTOCParams parses optional `key=value` pairs from a [TOC ...] tag.
// Unknown keys are ignored. Defaults are minLevel=2, maxLevel=2 (matches the
// example in the spec).
func parseTOCParams(raw string) (minLevel, maxLevel int) {
	minLevel, maxLevel = 2, 2
	if raw == "" {
		return
	}
	for _, tok := range strings.Fields(raw) {
		kv := strings.SplitN(tok, "=", 2)
		if len(kv) != 2 {
			continue
		}
		val, err := strconv.Atoi(kv[1])
		if err != nil {
			continue
		}
		switch strings.ToLower(kv[0]) {
		case "minlevel":
			minLevel = val
		case "maxlevel":
			maxLevel = val
		}
	}
	return
}
