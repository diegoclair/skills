// Package adf — markdown → Confluence storage XML conversion.
//
// This path is activated automatically by the CLI when the source markdown
// contains a :::properties block (or other macros that have no pure-ADF
// equivalent). In that case the whole page body is sent to Confluence as
// representation: "storage" (XHTML + macro XML) instead of atlas_doc_format.
//
// Strategy:
//  1. Scan the markdown for :::properties fenced blocks and extract them.
//  2. Replace each block with a unique sentinel comment.
//  3. Feed the sanitised markdown to goldmark's HTML renderer.
//  4. Swap the sentinel comments back for the storage XML produced by
//     PropertiesBlockToStorageXML.
//
// The result is a Confluence storage-format fragment (no <html>/<body> wrapper)
// that can be submitted directly as the page body.
package adf

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

const storagePropertiesPlaceholderPrefix = "<!-- __LYBEL_PROPS_"
const storagePropertiesPlaceholderSuffix = "__ -->"

// RequiresStorageFormat reports whether the given markdown source contains
// constructs that require the Confluence storage representation (XHTML +
// macro XML) instead of atlas_doc_format (ADF JSON).
//
// Currently this returns true when the markdown contains a :::properties
// fenced block on its own line (e.g. ":::properties" at the start of a line,
// case-insensitive).
func RequiresStorageFormat(markdown string) bool {
	return propertiesOpenRe.MatchString(markdown)
}

// propertiesOpenRe matches lines that open a :::properties block.
// Allows optional leading/trailing whitespace on the line.
var propertiesOpenRe = regexp.MustCompile(`(?im)^[ \t]*:::[ \t]*properties[ \t]*$`)

// MarkdownToStorage converts an extended markdown source (including
// :::properties blocks) to a Confluence storage-format XHTML fragment.
//
// The fragment is suitable for use as the value in a Confluence API v2 body
// object with representation = "storage".
func MarkdownToStorage(src []byte) (string, error) {
	text := string(src)

	// Extract :::properties blocks and replace with placeholder comments.
	sanitised, replacements, err := extractPropertiesBlocks(text)
	if err != nil {
		return "", fmt.Errorf("extracting properties blocks: %w", err)
	}

	// Convert the sanitised markdown (no :::properties) to HTML via goldmark.
	// goldmark HTML output is valid XHTML that Confluence accepts as storage.
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Strikethrough,
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // allow raw HTML pass-through from source
			html.WithXHTML(), // produce self-closing tags for valid XHTML
		),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(sanitised), &buf); err != nil {
		return "", fmt.Errorf("goldmark HTML render: %w", err)
	}
	result := buf.String()

	// Restore :::properties blocks as their storage XML equivalents.
	// The goldmark HTML renderer wraps unknown inline content inside <p> tags;
	// our sentinel comments will appear inside <p><!-- ... --></p>. We strip
	// the wrapping <p>/<p> so the macro sits at block level as Confluence expects.
	for i, xml := range replacements {
		placeholder := fmt.Sprintf("%s%d%s", storagePropertiesPlaceholderPrefix, i, storagePropertiesPlaceholderSuffix)
		// goldmark may wrap the placeholder in a paragraph; handle both cases.
		wrappedPlaceholder := "<p>" + placeholder + "</p>"
		if strings.Contains(result, wrappedPlaceholder) {
			result = strings.ReplaceAll(result, wrappedPlaceholder, xml)
		} else {
			result = strings.ReplaceAll(result, placeholder, xml)
		}
	}

	return strings.TrimSpace(result), nil
}

// extractPropertiesBlocks scans the markdown source for :::properties ... :::
// fenced blocks, replaces each with a sentinel comment, and returns:
//   - the sanitised markdown (:::properties blocks removed)
//   - the ordered list of storage XML strings (one per block found)
//   - an error if a block is unterminated
//
// Only top-level :::properties blocks are extracted; nested/indented ones are
// left as-is (they are rare in practice).
func extractPropertiesBlocks(src string) (sanitised string, xmlBlocks []string, err error) {
	lines := strings.Split(src, "\n")
	var out []string

	// openPropsRe matches the opening line of a :::properties block.
	openPropsRe := regexp.MustCompile(`^[ \t]*:::[ \t]*properties[ \t]*$`)
	// closeRe matches a bare ::: closing line.
	closeRe := regexp.MustCompile(`^[ \t]*:::[ \t]*$`)

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if openPropsRe.MatchString(line) {
			// Collect the body until the closing :::.
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
				return "", nil, fmt.Errorf("unterminated :::properties block at line %d", i+1)
			}
			inner := strings.Join(body, "\n")
			xml := PropertiesBlockToStorageXML(inner)
			if xml == "" {
				// Empty block — skip entirely.
				continue
			}
			idx := len(xmlBlocks)
			xmlBlocks = append(xmlBlocks, xml)
			placeholder := fmt.Sprintf("%s%d%s", storagePropertiesPlaceholderPrefix, idx, storagePropertiesPlaceholderSuffix)
			// Emit the placeholder as a standalone paragraph (blank lines around
			// it ensure goldmark treats it as a paragraph, not inline content).
			out = append(out, "")
			out = append(out, placeholder)
			out = append(out, "")
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n"), xmlBlocks, nil
}
