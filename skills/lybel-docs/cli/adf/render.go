// Package adf - ADF -> plain-text renderer.
//
// Used by the `home` cache to produce a searchable text view of a page
// without depending on Confluence's export_view (which would require a second
// API call). Output is markdown-ish: headings as `## X`, lists as `- X`,
// tables as pipe-rows. Macros are flagged so the LLM can see they're there
// without their content being garbled.
package adf

import (
	"strings"
)

// RenderText walks an ADF document tree and returns a flat plain-text
// rendering. Designed for grep / LLM consumption — not round-trip safe.
func RenderText(doc Node) string {
	var sb strings.Builder
	for _, n := range doc.Content {
		renderBlock(n, &sb, "")
	}
	return strings.TrimRight(sb.String(), "\n") + "\n"
}

// renderBlock writes one block-level node to sb, recursing as needed.
// `prefix` is the running indent (used by lists and blockquotes).
func renderBlock(n Node, sb *strings.Builder, prefix string) {
	switch n.Type {
	case "heading":
		level := headingLevel(n)
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		sb.WriteString("\n")
		sb.WriteString(strings.Repeat("#", level))
		sb.WriteString(" ")
		writeInline(n, sb)
		sb.WriteString("\n\n")

	case "paragraph":
		if prefix != "" {
			sb.WriteString(prefix)
		}
		writeInline(n, sb)
		sb.WriteString("\n")

	case "bulletList":
		for _, item := range n.Content {
			sb.WriteString(prefix)
			sb.WriteString("- ")
			renderListItemBody(item, sb, prefix+"  ")
		}
		sb.WriteString("\n")

	case "orderedList":
		i := 1
		for _, item := range n.Content {
			sb.WriteString(prefix)
			sb.WriteString(itoa(i))
			sb.WriteString(". ")
			renderListItemBody(item, sb, prefix+"   ")
			i++
		}
		sb.WriteString("\n")

	case "blockquote":
		for _, c := range n.Content {
			renderBlock(c, sb, prefix+"> ")
		}

	case "table":
		renderTable(n, sb)

	case "codeBlock":
		lang := ""
		if n.Attrs != nil {
			if v, ok := n.Attrs["language"].(string); ok {
				lang = v
			}
		}
		sb.WriteString("\n```")
		sb.WriteString(lang)
		sb.WriteString("\n")
		sb.WriteString(plainText(n))
		sb.WriteString("\n```\n\n")

	case "rule":
		sb.WriteString("\n---\n\n")

	case "expand":
		title := ""
		if n.Attrs != nil {
			if t, ok := n.Attrs["title"].(string); ok {
				title = t
			}
		}
		sb.WriteString("\n[expand: ")
		sb.WriteString(title)
		sb.WriteString("]\n")
		for _, c := range n.Content {
			renderBlock(c, sb, prefix)
		}
		sb.WriteString("[/expand]\n\n")

	case "panel":
		ptype := "panel"
		if n.Attrs != nil {
			if v, ok := n.Attrs["panelType"].(string); ok && v != "" {
				ptype = v
			}
		}
		sb.WriteString("\n[")
		sb.WriteString(ptype)
		sb.WriteString("]\n")
		for _, c := range n.Content {
			renderBlock(c, sb, prefix)
		}
		sb.WriteString("[/")
		sb.WriteString(ptype)
		sb.WriteString("]\n\n")

	case "extension":
		key := "extension"
		if n.Attrs != nil {
			if k, ok := n.Attrs["extensionKey"].(string); ok && k != "" {
				key = k
			}
		}
		sb.WriteString("[")
		sb.WriteString(key)
		sb.WriteString(" macro]\n")

	case "inlineCard", "card":
		// Inline card: render the URL if present.
		if n.Attrs != nil {
			if url, ok := n.Attrs["url"].(string); ok && url != "" {
				sb.WriteString(url)
			}
		}

	default:
		// Unknown block: write the inline text and recurse.
		writeInline(n, sb)
		for _, c := range n.Content {
			if isBlockNode(c.Type) {
				renderBlock(c, sb, prefix)
			}
		}
	}
}

// renderListItemBody renders the content of a listItem with continuation indent.
func renderListItemBody(item Node, sb *strings.Builder, contPrefix string) {
	first := true
	for _, c := range item.Content {
		if first {
			// Skip the prefix on the first paragraph (already written by caller)
			if c.Type == "paragraph" {
				writeInline(c, sb)
				sb.WriteString("\n")
				first = false
				continue
			}
		}
		renderBlock(c, sb, contPrefix)
		first = false
	}
}

// renderTable writes a table as pipe-rows. Header cells get a separator line.
func renderTable(n Node, sb *strings.Builder) {
	sb.WriteString("\n")
	hasHeader := false
	for ri, row := range n.Content {
		if row.Type != "tableRow" {
			continue
		}
		isHeaderRow := true
		sb.WriteString("|")
		cells := row.Content
		for _, c := range cells {
			if c.Type != "tableHeader" {
				isHeaderRow = false
			}
			sb.WriteString(" ")
			sb.WriteString(strings.ReplaceAll(plainText(c), "|", "\\|"))
			sb.WriteString(" |")
		}
		sb.WriteString("\n")
		if ri == 0 && isHeaderRow {
			hasHeader = true
			sb.WriteString("|")
			for range cells {
				sb.WriteString(" --- |")
			}
			sb.WriteString("\n")
		}
	}
	if !hasHeader {
		// Make sure there's a blank line after a table without header
		sb.WriteString("\n")
	} else {
		sb.WriteString("\n")
	}
}

// writeInline writes inline text content of a block node, with link URLs
// preserved as `text (url)` so they're greppable.
func writeInline(n Node, sb *strings.Builder) {
	for _, c := range n.Content {
		writeInlineNode(c, sb)
	}
}

func writeInlineNode(n Node, sb *strings.Builder) {
	switch n.Type {
	case "text":
		// Detect link mark
		linkURL := ""
		for _, m := range n.Marks {
			if m.Type == "link" && m.Attrs != nil {
				if h, ok := m.Attrs["href"].(string); ok {
					linkURL = h
				}
			}
		}
		if linkURL != "" {
			sb.WriteString("[")
			sb.WriteString(n.Text)
			sb.WriteString("](")
			sb.WriteString(linkURL)
			sb.WriteString(")")
		} else {
			sb.WriteString(n.Text)
		}
	case "hardBreak":
		sb.WriteString(" ")
	case "inlineCard", "card":
		if n.Attrs != nil {
			if url, ok := n.Attrs["url"].(string); ok && url != "" {
				sb.WriteString(url)
			}
		}
	case "emoji":
		// Emoji's shortName / text is in attrs.
		if n.Attrs != nil {
			if t, ok := n.Attrs["text"].(string); ok && t != "" {
				sb.WriteString(t)
				return
			}
			if s, ok := n.Attrs["shortName"].(string); ok && s != "" {
				sb.WriteString(s)
				return
			}
		}
	case "mention":
		if n.Attrs != nil {
			if t, ok := n.Attrs["text"].(string); ok && t != "" {
				sb.WriteString(t)
				return
			}
		}
	default:
		// Unknown inline: descend
		for _, c := range n.Content {
			writeInlineNode(c, sb)
		}
	}
}

// plainText collects all inline text under a node (no marks, no formatting).
// Used by table cells to keep them on a single line.
func plainText(n Node) string {
	var sb strings.Builder
	collectInlineText(n, &sb)
	return strings.TrimSpace(strings.ReplaceAll(sb.String(), "\n", " "))
}

func collectInlineText(n Node, sb *strings.Builder) {
	if n.Text != "" {
		sb.WriteString(n.Text)
	}
	if n.Type == "inlineCard" || n.Type == "card" {
		if n.Attrs != nil {
			if url, ok := n.Attrs["url"].(string); ok && url != "" {
				sb.WriteString(url)
			}
		}
	}
	for _, c := range n.Content {
		collectInlineText(c, sb)
	}
}

func isBlockNode(t string) bool {
	switch t {
	case "heading", "paragraph", "bulletList", "orderedList", "listItem",
		"blockquote", "table", "tableRow", "tableHeader", "tableCell",
		"codeBlock", "rule", "expand", "panel", "extension":
		return true
	}
	return false
}
