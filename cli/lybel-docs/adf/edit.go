package adf

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ConvertFragment parses markdown and returns the ADF block-level nodes,
// without wrapping them in a document. Useful when splicing into an existing
// document (see Append, InsertAfter, ReplaceSection, etc.).
func ConvertFragment(src []byte) ([]Node, error) {
	doc, err := Convert(src)
	if err != nil {
		return nil, err
	}
	return doc.Content, nil
}

// UnmarshalDoc parses a JSON-encoded ADF doc node. The input is expected to be
// what Confluence's REST API returns under the page body: a single object with
// type "doc" and a content array.
func UnmarshalDoc(data []byte) (Node, error) {
	var n Node
	if err := json.Unmarshal(data, &n); err != nil {
		return Node{}, fmt.Errorf("parse adf: %w", err)
	}
	if n.Type != "doc" {
		return Node{}, fmt.Errorf("expected top-level node type \"doc\", got %q", n.Type)
	}
	return n, nil
}

// Append returns a copy of doc with fragment nodes appended to its content.
func Append(doc Node, fragment []Node) Node {
	out := doc
	out.Content = append(append([]Node{}, doc.Content...), fragment...)
	return out
}

// InsertAfter returns a copy of doc with fragment nodes inserted immediately
// after the section whose heading text matches headingText. The "section"
// covers the heading plus all following siblings up to (but not including) the
// next heading node of the same or higher level. Exact case-sensitive match on
// trimmed heading text.
//
// Returns an error if no heading matches.
func InsertAfter(doc Node, headingText string, fragment []Node) (Node, error) {
	idx, end, ok := findSectionBounds(doc.Content, headingText)
	if !ok {
		return Node{}, fmt.Errorf("section not found: %q", headingText)
	}
	_ = idx
	out := doc
	out.Content = spliceNodes(doc.Content, end, end, fragment)
	return out, nil
}

// InsertBefore returns a copy of doc with fragment nodes inserted immediately
// before the matched heading. See InsertAfter for matching semantics.
func InsertBefore(doc Node, headingText string, fragment []Node) (Node, error) {
	idx, _, ok := findSectionBounds(doc.Content, headingText)
	if !ok {
		return Node{}, fmt.Errorf("section not found: %q", headingText)
	}
	out := doc
	out.Content = spliceNodes(doc.Content, idx, idx, fragment)
	return out, nil
}

// ReplaceSection returns a copy of doc where the section matched by
// headingText (the heading plus all nodes until the next heading of same or
// higher level) is replaced by fragment.
func ReplaceSection(doc Node, headingText string, fragment []Node) (Node, error) {
	idx, end, ok := findSectionBounds(doc.Content, headingText)
	if !ok {
		return Node{}, fmt.Errorf("section not found: %q", headingText)
	}
	out := doc
	out.Content = spliceNodes(doc.Content, idx, end, fragment)
	return out, nil
}

// DeleteSection returns a copy of doc with the matched section removed.
func DeleteSection(doc Node, headingText string) (Node, error) {
	idx, end, ok := findSectionBounds(doc.Content, headingText)
	if !ok {
		return Node{}, fmt.Errorf("section not found: %q", headingText)
	}
	out := doc
	out.Content = spliceNodes(doc.Content, idx, end, nil)
	return out, nil
}

// SectionContent returns a sub-document containing only the matched section
// (heading + body until the next heading of equal-or-higher level). Useful
// for `page get --section` where the caller wants just the slice without
// fetching the whole page through a separate code path.
//
// The returned Node is a doc node so the result is renderable by the same
// machinery that handles full pages (Marshal, RenderText, etc.).
func SectionContent(doc Node, headingText string, atLevel int) (Node, error) {
	idx, end, ok := findSectionBoundsAtLevel(doc.Content, headingText, atLevel)
	if !ok {
		return Node{}, fmt.Errorf("section not found: %q", headingText)
	}
	out := doc
	out.Content = append([]Node{}, doc.Content[idx:end]...)
	return out, nil
}

// findSectionBounds locates the top-level heading whose inline text matches
// target (case-sensitive, trimmed). Returns [start, end) indices such that
// start is the heading index and end is the first index that does NOT belong
// to the section. When no match is found, ok is false.
//
// A section ends at the next heading of equal-or-lower level number (higher
// importance), or at the end of the slice.
func findSectionBounds(nodes []Node, target string) (int, int, bool) {
	target = strings.TrimSpace(target)
	for i, n := range nodes {
		if n.Type != "heading" {
			continue
		}
		if strings.TrimSpace(headingText(n)) != target {
			continue
		}
		level := headingLevel(n)
		end := len(nodes)
		for j := i + 1; j < len(nodes); j++ {
			if nodes[j].Type == "heading" && headingLevel(nodes[j]) <= level {
				end = j
				break
			}
		}
		return i, end, true
	}
	return 0, 0, false
}

// headingText collects the inline text content of a heading node.
func headingText(n Node) string {
	var sb strings.Builder
	collectText(n, &sb)
	return sb.String()
}

func collectText(n Node, sb *strings.Builder) {
	if n.Type == "text" {
		sb.WriteString(n.Text)
		return
	}
	for _, c := range n.Content {
		collectText(c, sb)
	}
}

// headingLevel returns the heading level (1-6), defaulting to 1 when missing.
func headingLevel(n Node) int {
	if n.Attrs == nil {
		return 1
	}
	switch v := n.Attrs["level"].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i)
		}
	}
	return 1
}

// spliceNodes returns a new slice equal to nodes with nodes[start:end]
// replaced by insert. Does not mutate the input.
func spliceNodes(nodes []Node, start, end int, insert []Node) []Node {
	out := make([]Node, 0, len(nodes)-(end-start)+len(insert))
	out = append(out, nodes[:start]...)
	out = append(out, insert...)
	out = append(out, nodes[end:]...)
	return out
}
