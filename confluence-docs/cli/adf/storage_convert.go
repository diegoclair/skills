// Package adf — markdown → Confluence storage XML conversion.
//
// This path is activated automatically by the CLI when the source markdown
// contains a :::properties block (or other macros that have no pure-ADF
// equivalent). In that case the whole page body is sent to Confluence as
// representation: "storage" (XHTML + macro XML) instead of atlas_doc_format.
//
// Strategy:
//  1. Scan the markdown for fenced extension blocks (:::properties, :::info,
//     :::note, :::warning, :::success, :::error, :::tip, :::expand).
//  2. Replace each with a unique sentinel placeholder.
//  3. Feed the sanitised markdown to goldmark's HTML renderer.
//  4. Swap the sentinels back for the storage XML equivalents (macro body
//     recursively rendered via goldmark inline).
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

const storageMacroPlaceholderPrefix = "<!-- __LYBEL_MACRO_"
const storageMacroPlaceholderSuffix = "__ -->"

// RequiresStorageFormat reports whether the given markdown source contains
// constructs that require the Confluence storage representation (XHTML +
// macro XML) instead of atlas_doc_format (ADF JSON).
//
// Currently this returns true when the markdown contains a :::properties
// fenced block (which has no pure-ADF equivalent).
func RequiresStorageFormat(markdown string) bool {
	return propertiesOpenRe.MatchString(markdown)
}

// propertiesOpenRe matches lines that open a :::properties block, with or
// without a trailing modifier (e.g. `:::properties collapsed`).
var propertiesOpenRe = regexp.MustCompile(`(?im)^[ \t]*:::[ \t]*properties(?:[ \t][^\n]*)?$`)

// extensionOpenRe matches any opening :::<name>[ "title"] line for supported macros.
// Captures: 1 = name (lowercase), 2 = optional title (without quotes).
var extensionOpenRe = regexp.MustCompile(`^[ \t]*:::[ \t]*(properties|info|note|warning|success|error|tip|expand)\b[ \t]*(?:"([^"]*)"|(.*))?$`)

// extensionCloseRe matches a bare ::: line that closes a fenced block.
var extensionCloseRe = regexp.MustCompile(`^[ \t]*:::[ \t]*$`)

// renderInlineMD renders a markdown fragment through goldmark's HTML renderer
// for use inside a macro body. Returns the rendered XHTML (no surrounding tags).
func renderInlineMD(body string) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Strikethrough),
		goldmark.WithRendererOptions(html.WithUnsafe(), html.WithXHTML()),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(body), &buf); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// macroBlockToStorage converts a parsed fenced block into Confluence storage XML.
// `name` is lowercase; `title` is the optional title from the opening line;
// `body` is the raw markdown inside the block.
func macroBlockToStorage(name, title, body string) (string, error) {
	switch name {
	case "properties":
		xml := PropertiesBlockToStorageXML(body)
		if xml == "" {
			return "", nil
		}
		// If the opening line had a "collapsed" modifier (`:::properties collapsed`),
		// wrap the details macro in an expand so it renders collapsed by default.
		if strings.EqualFold(strings.TrimSpace(title), "collapsed") {
			var sb strings.Builder
			sb.WriteString(`<ac:structured-macro ac:name="expand" ac:schema-version="1">`)
			sb.WriteString(`<ac:parameter ac:name="title">Metadados</ac:parameter>`)
			sb.WriteString(`<ac:rich-text-body>`)
			sb.WriteString(xml)
			sb.WriteString(`</ac:rich-text-body></ac:structured-macro>`)
			return sb.String(), nil
		}
		return xml, nil

	case "info", "note", "warning", "success", "error", "tip":
		// Confluence supports info/note/warning/tip directly; success/error
		// fall back to the closest semantic match.
		macroName := name
		switch name {
		case "success":
			macroName = "tip" // green-styled, closest to success
		case "error":
			macroName = "warning" // red-styled, closest to error
		}
		inner, err := renderInlineMD(body)
		if err != nil {
			return "", fmt.Errorf("rendering %s body: %w", name, err)
		}
		var sb strings.Builder
		sb.WriteString(`<ac:structured-macro ac:name="`)
		sb.WriteString(macroName)
		sb.WriteString(`" ac:schema-version="1">`)
		if title != "" {
			sb.WriteString(`<ac:parameter ac:name="title">`)
			sb.WriteString(escapeXMLText(title))
			sb.WriteString(`</ac:parameter>`)
		}
		sb.WriteString(`<ac:rich-text-body>`)
		sb.WriteString(inner)
		sb.WriteString(`</ac:rich-text-body></ac:structured-macro>`)
		return sb.String(), nil

	case "expand":
		inner, err := renderInlineMD(body)
		if err != nil {
			return "", fmt.Errorf("rendering expand body: %w", err)
		}
		var sb strings.Builder
		sb.WriteString(`<ac:structured-macro ac:name="expand" ac:schema-version="1">`)
		if title != "" {
			sb.WriteString(`<ac:parameter ac:name="title">`)
			sb.WriteString(escapeXMLText(title))
			sb.WriteString(`</ac:parameter>`)
		}
		sb.WriteString(`<ac:rich-text-body>`)
		sb.WriteString(inner)
		sb.WriteString(`</ac:rich-text-body></ac:structured-macro>`)
		return sb.String(), nil
	}
	return "", fmt.Errorf("unknown macro: %s", name)
}

// escapeXMLText escapes the minimal set of characters needed inside an XML
// text node or attribute value.
func escapeXMLText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// MarkdownToStorage converts an extended markdown source (with :::properties,
// :::info, :::expand, etc) to a Confluence storage-format XHTML fragment.
func MarkdownToStorage(src []byte) (string, error) {
	text := string(src)

	// Extract all supported :::name fenced blocks and replace with placeholders.
	sanitised, replacements, err := extractMacroBlocks(text)
	if err != nil {
		return "", fmt.Errorf("extracting macro blocks: %w", err)
	}

	// Convert the sanitised markdown to XHTML via goldmark.
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Strikethrough),
		goldmark.WithRendererOptions(html.WithUnsafe(), html.WithXHTML()),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(sanitised), &buf); err != nil {
		return "", fmt.Errorf("goldmark HTML render: %w", err)
	}
	result := buf.String()

	// Restore macro placeholders. goldmark may wrap them inside <p>...</p>;
	// strip that wrapper so the macro sits at block level.
	for i, xml := range replacements {
		placeholder := fmt.Sprintf("%s%d%s", storageMacroPlaceholderPrefix, i, storageMacroPlaceholderSuffix)
		wrappedPlaceholder := "<p>" + placeholder + "</p>"
		if strings.Contains(result, wrappedPlaceholder) {
			result = strings.ReplaceAll(result, wrappedPlaceholder, xml)
		} else {
			result = strings.ReplaceAll(result, placeholder, xml)
		}
	}

	return strings.TrimSpace(result), nil
}

// extractMacroBlocks scans the markdown source for all supported :::name ... :::
// fenced blocks (properties/info/note/warning/success/error/tip/expand),
// replaces each with a sentinel placeholder, and returns the sanitised markdown
// plus the ordered storage XML strings.
func extractMacroBlocks(src string) (sanitised string, xmlBlocks []string, err error) {
	lines := strings.Split(src, "\n")
	var out []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		m := extensionOpenRe.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			continue
		}
		name := strings.ToLower(m[1])
		title := m[2]
		if title == "" && m[3] != "" {
			// Title without quotes — e.g. `:::expand Ver as 50 páginas`
			title = strings.TrimSpace(m[3])
		}

		// Collect the body until the closing :::.
		var body []string
		closed := false
		for j := i + 1; j < len(lines); j++ {
			if extensionCloseRe.MatchString(lines[j]) {
				i = j
				closed = true
				break
			}
			body = append(body, lines[j])
		}
		if !closed {
			return "", nil, fmt.Errorf("unterminated :::%s block at line %d", name, i+1)
		}

		xml, err := macroBlockToStorage(name, title, strings.Join(body, "\n"))
		if err != nil {
			return "", nil, err
		}
		if xml == "" {
			continue
		}

		idx := len(xmlBlocks)
		xmlBlocks = append(xmlBlocks, xml)
		placeholder := fmt.Sprintf("%s%d%s", storageMacroPlaceholderPrefix, idx, storageMacroPlaceholderSuffix)
		out = append(out, "")
		out = append(out, placeholder)
		out = append(out, "")
	}
	return strings.Join(out, "\n"), xmlBlocks, nil
}
