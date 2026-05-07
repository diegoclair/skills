package adf

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// Convert reads markdown bytes and returns the root ADF document node.
// It pre-processes Confluence macros (`[TOC]`, `:::expand`, `:::warning`, ...)
// before invoking the goldmark parser, then walks the AST to emit ADF.
func Convert(src []byte) (Node, error) {
	doc, err := convertString(string(src))
	if err != nil {
		return Node{}, err
	}
	return doc, nil
}

// convertString is the internal entry point used both by Convert and by macro
// pre-processing (so `:::expand` bodies recurse cleanly).
func convertString(src string) (Node, error) {
	preprocessed, macros, err := preprocess(src)
	if err != nil {
		return Node{}, err
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Strikethrough,
		),
	)

	source := []byte(preprocessed)
	reader := text.NewReader(source)
	root := md.Parser().Parse(reader)

	c := &converter{source: source, macros: macros}
	blocks := c.walkBlocks(root)
	return Doc(blocks...), nil
}

// converter holds context needed while walking the goldmark AST.
type converter struct {
	source []byte
	macros []macro
}

// walkBlocks walks a container node's direct children and returns one ADF node
// per block-level child.
func (c *converter) walkBlocks(parent ast.Node) []Node {
	var out []Node
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		if node, ok := c.convertBlock(n); ok {
			out = append(out, node)
		}
	}
	return out
}

// convertBlock dispatches a goldmark block-level node to its ADF equivalent.
// Returns (node, true) on success; (Node{}, false) means the node was skipped.
func (c *converter) convertBlock(n ast.Node) (Node, bool) {
	switch v := n.(type) {
	case *ast.Heading:
		return Heading(v.Level, c.walkInline(v)...), true

	case *ast.Paragraph:
		// A paragraph that contains only a macro placeholder is replaced by
		// the macro's rendered ADF node directly.
		if node, ok := c.tryMacroParagraph(v); ok {
			return node, true
		}
		return Paragraph(c.walkInline(v)...), true

	case *ast.TextBlock:
		// TextBlock appears inside list items; same shape as paragraph for ADF.
		if node, ok := c.tryMacroParagraph(v); ok {
			return node, true
		}
		return Paragraph(c.walkInline(v)...), true

	case *ast.ThematicBreak:
		return Rule(), true

	case *ast.Blockquote:
		return Blockquote(c.walkBlocks(v)...), true

	case *ast.FencedCodeBlock:
		lang := string(v.Language(c.source))
		return CodeBlock(lang, c.collectCodeLines(v)), true

	case *ast.CodeBlock:
		return CodeBlock("", c.collectCodeLines(v)), true

	case *ast.List:
		items := c.walkBlocks(v)
		if v.IsOrdered() {
			return OrderedList(items...), true
		}
		return BulletList(items...), true

	case *ast.ListItem:
		return ListItem(c.walkBlocks(v)...), true

	case *extast.Table:
		return c.convertTable(v), true

	case *ast.HTMLBlock:
		// Render raw HTML as a code block to preserve the content visibly
		// rather than dropping it.
		return CodeBlock("html", string(v.Lines().Value(c.source))), true
	}
	return Node{}, false
}

// tryMacroParagraph returns the macro node if the paragraph wraps a single
// macro placeholder text segment.
func (c *converter) tryMacroParagraph(p ast.Node) (Node, bool) {
	// Collect all text within the paragraph and check if it's exactly a
	// placeholder. This handles the common case where pre-processing produced
	// a line like `%%LYBELDOC_MACRO_3%%`.
	var sb strings.Builder
	ast.Walk(p, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := n.(*ast.Text); ok {
			sb.Write(t.Segment.Value(c.source))
		}
		return ast.WalkContinue, nil
	})
	idx, ok := matchPlaceholder(strings.TrimSpace(sb.String()))
	if !ok {
		return Node{}, false
	}
	if idx < 0 || idx >= len(c.macros) {
		return Node{}, false
	}
	return c.macros[idx].render(), true
}

// walkInline returns the inline ADF nodes for a block's children.
func (c *converter) walkInline(parent ast.Node) []Node {
	return c.inlineChildren(parent, nil)
}

// inlineChildren walks inline children, propagating active marks down so e.g.
// `**[bold link](url)**` carries both strong and link marks on its text.
func (c *converter) inlineChildren(parent ast.Node, marks []Mark) []Node {
	var out []Node
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		out = append(out, c.convertInline(n, marks)...)
	}
	return out
}

// convertInline returns ADF nodes for one inline AST node.
func (c *converter) convertInline(n ast.Node, marks []Mark) []Node {
	switch v := n.(type) {
	case *ast.Text:
		txt := string(v.Segment.Value(c.source))
		if txt == "" && !v.HardLineBreak() && !v.SoftLineBreak() {
			return nil
		}
		var nodes []Node
		if txt != "" {
			nodes = append(nodes, Text(txt, marks...))
		}
		if v.HardLineBreak() {
			nodes = append(nodes, HardBreak())
		} else if v.SoftLineBreak() {
			// Soft breaks render as a single space in ADF (matches HTML).
			nodes = append(nodes, Text(" ", marks...))
		}
		return nodes

	case *ast.String:
		return []Node{Text(string(v.Value), marks...)}

	case *ast.CodeSpan:
		return []Node{Text(c.collectInlineText(v), append(cloneMarks(marks), Code())...)}

	case *ast.Emphasis:
		var m Mark
		if v.Level == 2 {
			m = Bold()
		} else {
			m = Italic()
		}
		return c.inlineChildren(v, append(cloneMarks(marks), m))

	case *extast.Strikethrough:
		return c.inlineChildren(v, append(cloneMarks(marks), Strike()))

	case *ast.Link:
		return c.inlineChildren(v, append(cloneMarks(marks), Link(string(v.Destination))))

	case *ast.AutoLink:
		url := string(v.URL(c.source))
		return []Node{Text(url, append(cloneMarks(marks), Link(url))...)}

	case *ast.Image:
		// Render images as a link to the source so content survives even
		// without media-node uploads (which require Confluence-side IDs).
		url := string(v.Destination)
		alt := c.collectInlineText(v)
		if alt == "" {
			alt = url
		}
		return []Node{Text(alt, append(cloneMarks(marks), Link(url))...)}

	case *ast.RawHTML:
		return []Node{Text(c.collectInlineText(v), marks...)}
	}
	// Unknown inline node — recurse so we don't drop nested content silently.
	return c.inlineChildren(n, marks)
}

// cloneMarks returns a fresh slice so appends on caller side don't bleed into
// sibling subtrees.
func cloneMarks(in []Mark) []Mark {
	out := make([]Mark, len(in), len(in)+1)
	copy(out, in)
	return out
}

// collectInlineText flattens an inline subtree to its raw text.
func (c *converter) collectInlineText(n ast.Node) string {
	var sb strings.Builder
	ast.Walk(n, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := node.(type) {
		case *ast.Text:
			sb.Write(t.Segment.Value(c.source))
		case *ast.String:
			sb.Write(t.Value)
		case *ast.CodeSpan:
			// Recurse into children for the code span's inner text.
		}
		return ast.WalkContinue, nil
	})
	return sb.String()
}

// collectCodeLines returns the literal text inside a code block.
func (c *converter) collectCodeLines(n ast.Node) string {
	var sb strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		sb.Write(seg.Value(c.source))
	}
	// Trim a trailing newline because ADF code blocks render the text verbatim
	// and a trailing \n shows up as an extra empty line.
	return strings.TrimRight(sb.String(), "\n")
}

// convertTable converts a goldmark/extension Table to an ADF table.
func (c *converter) convertTable(t *extast.Table) Node {
	var rows []Node
	for n := t.FirstChild(); n != nil; n = n.NextSibling() {
		switch row := n.(type) {
		case *extast.TableHeader:
			rows = append(rows, c.convertTableRow(row, true))
		case *extast.TableRow:
			rows = append(rows, c.convertTableRow(row, false))
		}
	}
	return Table(rows...)
}

func (c *converter) convertTableRow(row ast.Node, header bool) Node {
	var cells []Node
	for n := row.FirstChild(); n != nil; n = n.NextSibling() {
		cell, ok := n.(*extast.TableCell)
		if !ok {
			continue
		}
		para := Paragraph(c.walkInline(cell)...)
		if header {
			cells = append(cells, TableHeader(para))
		} else {
			cells = append(cells, TableCell(para))
		}
	}
	return TableRow(cells...)
}

// debug helpers — kept for development convenience, not exported.
var _ = func() string {
	var b bytes.Buffer
	fmt.Fprintln(&b, "adf converter")
	return b.String()
}
