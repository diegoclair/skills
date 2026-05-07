// Package adf provides typed builders for the Atlassian Document Format (ADF)
// JSON tree, plus a Markdown -> ADF converter with Confluence macro extensions.
package adf

import "encoding/json"

// Node is a single ADF node. Fields use omitempty so the JSON output stays
// compact and matches what Confluence's REST API expects.
type Node struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
}

// Mark is an inline mark applied to text nodes (bold, italic, link, code, ...).
type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// Marshal returns the ADF tree serialized to JSON. If pretty is true, output is
// indented for readability.
func Marshal(n Node, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(n, "", "  ")
	}
	return json.Marshal(n)
}

// ---------- Document & block builders ----------

// Doc creates a top-level ADF document node with version 1.
func Doc(content ...Node) Node {
	return Node{
		Type:    "doc",
		Attrs:   map[string]any{"version": 1},
		Content: dropEmpty(content),
	}
}

// Heading creates a heading node with the given level (1-6) and inline content.
func Heading(level int, content ...Node) Node {
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}
	return Node{
		Type:    "heading",
		Attrs:   map[string]any{"level": level},
		Content: dropEmpty(content),
	}
}

// Paragraph wraps inline content in a paragraph block.
func Paragraph(content ...Node) Node {
	return Node{Type: "paragraph", Content: dropEmpty(content)}
}

// Text creates a text leaf node, optionally decorated with marks.
func Text(s string, marks ...Mark) Node {
	n := Node{Type: "text", Text: s}
	if len(marks) > 0 {
		n.Marks = marks
	}
	return n
}

// HardBreak inserts a hard line break.
func HardBreak() Node {
	return Node{Type: "hardBreak"}
}

// Rule renders a horizontal rule.
func Rule() Node {
	return Node{Type: "rule"}
}

// CodeBlock creates a fenced code block with optional language.
func CodeBlock(language, code string) Node {
	attrs := map[string]any{}
	if language != "" {
		attrs["language"] = language
	}
	n := Node{Type: "codeBlock", Content: []Node{{Type: "text", Text: code}}}
	if len(attrs) > 0 {
		n.Attrs = attrs
	}
	return n
}

// Blockquote wraps block content in a blockquote.
func Blockquote(content ...Node) Node {
	return Node{Type: "blockquote", Content: dropEmpty(content)}
}

// BulletList creates an unordered list from list items.
func BulletList(items ...Node) Node {
	return Node{Type: "bulletList", Content: dropEmpty(items)}
}

// OrderedList creates an ordered list from list items.
func OrderedList(items ...Node) Node {
	return Node{Type: "orderedList", Content: dropEmpty(items)}
}

// ListItem wraps block content as a single list item.
func ListItem(content ...Node) Node {
	return Node{Type: "listItem", Content: dropEmpty(content)}
}

// Table creates a table node from rows.
func Table(rows ...Node) Node {
	return Node{
		Type:    "table",
		Attrs:   map[string]any{"isNumberColumnEnabled": false, "layout": "default"},
		Content: dropEmpty(rows),
	}
}

// TableRow creates a row from cell nodes.
func TableRow(cells ...Node) Node {
	return Node{Type: "tableRow", Content: dropEmpty(cells)}
}

// TableHeader creates a header cell.
func TableHeader(content ...Node) Node {
	return Node{Type: "tableHeader", Content: dropEmpty(content)}
}

// TableCell creates a body cell.
func TableCell(content ...Node) Node {
	return Node{Type: "tableCell", Content: dropEmpty(content)}
}

// ---------- Marks ----------

// Bold returns a strong mark.
func Bold() Mark { return Mark{Type: "strong"} }

// Italic returns an em mark.
func Italic() Mark { return Mark{Type: "em"} }

// Code returns an inline code mark.
func Code() Mark { return Mark{Type: "code"} }

// Strike returns a strikethrough mark.
func Strike() Mark { return Mark{Type: "strike"} }

// Link returns a link mark targeting href.
func Link(href string) Mark {
	return Mark{Type: "link", Attrs: map[string]any{"href": href}}
}

// ---------- Confluence macros ----------

// Expand creates a Confluence expand block with a title and inner content.
func Expand(title string, content ...Node) Node {
	attrs := map[string]any{}
	if title != "" {
		attrs["title"] = title
	}
	return Node{Type: "expand", Attrs: attrs, Content: dropEmpty(content)}
}

// Panel creates a Confluence panel of the given type (info, warning, note,
// success, error). Title is rendered as the first paragraph (Confluence's ADF
// schema doesn't carry a separate title attribute on panel nodes).
func Panel(panelType, title string, content ...Node) Node {
	body := dropEmpty(content)
	if title != "" {
		titlePara := Paragraph(Text(title, Bold()))
		body = append([]Node{titlePara}, body...)
	}
	return Node{
		Type:    "panel",
		Attrs:   map[string]any{"panelType": panelType},
		Content: body,
	}
}

// TOC builds the Confluence Table of Contents extension. minLevel/maxLevel are
// stringified for the macroParams shape Confluence expects.
func TOC(minLevel, maxLevel int) Node {
	if minLevel <= 0 {
		minLevel = 2
	}
	if maxLevel <= 0 {
		maxLevel = 2
	}
	return Node{
		Type: "extension",
		Attrs: map[string]any{
			"layout":        "default",
			"extensionType": "com.atlassian.confluence.macro.core",
			"extensionKey":  "toc",
			"parameters": map[string]any{
				"macroParams": map[string]any{
					"maxLevel": map[string]any{"value": itoa(maxLevel)},
					"minLevel": map[string]any{"value": itoa(minLevel)},
				},
				"macroMetadata": map[string]any{"title": "Table of Contents"},
			},
		},
	}
}

// dropEmpty returns content unchanged unless nil; ADF tolerates empty arrays
// but we'd rather omit them via omitempty.
func dropEmpty(nodes []Node) []Node {
	if len(nodes) == 0 {
		return nil
	}
	return nodes
}

// itoa is a tiny stdlib-free integer-to-string for small positive ints used in
// macro params. Keeps imports minimal in this file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
