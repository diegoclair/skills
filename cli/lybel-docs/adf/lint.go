package adf

import (
	"fmt"
	"io"
	"strings"
)

// LintSeverity indicates how serious a lint finding is.
type LintSeverity string

const (
	LintError   LintSeverity = "error"
	LintWarning LintSeverity = "warning"
)

// LintResult represents a single lint finding.
type LintResult struct {
	Severity LintSeverity
	Path     string // e.g. "content[2]" or "content[2].content[0]"
	Message  string
}

func (r LintResult) String() string {
	return fmt.Sprintf("%s [%s] %s", r.Severity, r.Path, r.Message)
}

// Lint validates the ADF document structure and returns a list of findings.
// Returns errors for structural problems and warnings for style issues.
func Lint(doc Node) []LintResult {
	var results []LintResult
	ctx := &lintCtx{}
	ctx.lintDoc(doc, &results)
	return results
}

// WriteLintResults writes lint results to w. Returns the number of errors.
func WriteLintResults(results []LintResult, w io.Writer) int {
	errorCount := 0
	for _, r := range results {
		fmt.Fprintln(w, r.String())
		if r.Severity == LintError {
			errorCount++
		}
	}
	return errorCount
}

type lintCtx struct {
	seenTopLevelHeadings map[string]int // heading text → first index (for duplicate detection)
}

func (c *lintCtx) lintDoc(doc Node, results *[]LintResult) {
	if doc.Type != "doc" {
		*results = append(*results, LintResult{
			Severity: LintError,
			Path:     "root",
			Message:  fmt.Sprintf("root node must be type 'doc', got %q", doc.Type),
		})
		return
	}
	if len(doc.Content) == 0 {
		*results = append(*results, LintResult{
			Severity: LintWarning,
			Path:     "root",
			Message:  "document is empty (no content nodes)",
		})
		return
	}

	c.seenTopLevelHeadings = make(map[string]int)
	for i, n := range doc.Content {
		c.lintNode(n, fmt.Sprintf("content[%d]", i), results)
		// Check for duplicate top-level headings
		if n.Type == "heading" {
			txt := strings.TrimSpace(headingText(n))
			if prev, seen := c.seenTopLevelHeadings[txt]; seen {
				*results = append(*results, LintResult{
					Severity: LintWarning,
					Path:     fmt.Sprintf("content[%d]", i),
					Message:  fmt.Sprintf("duplicate top-level heading %q (first seen at content[%d])", txt, prev),
				})
			} else {
				c.seenTopLevelHeadings[txt] = i
			}
		}
	}
}

func (c *lintCtx) lintNode(n Node, path string, results *[]LintResult) {
	switch n.Type {
	case "heading":
		c.lintHeading(n, path, results)
	case "table":
		c.lintTable(n, path, results)
	case "bulletList", "orderedList":
		c.lintList(n, path, results)
	case "text":
		// text nodes are leaves — only check their marks
		c.lintLinks(n, path, results)
	default:
		// Recurse into children for all other nodes
		for i, child := range n.Content {
			c.lintNode(child, fmt.Sprintf("%s.content[%d]", path, i), results)
		}
	}
}

func (c *lintCtx) lintHeading(n Node, path string, results *[]LintResult) {
	txt := strings.TrimSpace(headingText(n))
	if txt == "" {
		*results = append(*results, LintResult{
			Severity: LintError,
			Path:     path,
			Message:  "heading has no text content",
		})
	}
	level := headingLevel(n)
	if level < 1 || level > 6 {
		*results = append(*results, LintResult{
			Severity: LintError,
			Path:     path,
			Message:  fmt.Sprintf("heading level %d is out of range (1-6)", level),
		})
	}
}

func (c *lintCtx) lintTable(n Node, path string, results *[]LintResult) {
	if len(n.Content) == 0 {
		*results = append(*results, LintResult{
			Severity: LintError,
			Path:     path,
			Message:  "table has no rows",
		})
		return
	}
	for i, row := range n.Content {
		if row.Type != "tableRow" {
			*results = append(*results, LintResult{
				Severity: LintError,
				Path:     fmt.Sprintf("%s.content[%d]", path, i),
				Message:  fmt.Sprintf("expected tableRow, got %q", row.Type),
			})
			continue
		}
		if len(row.Content) == 0 {
			*results = append(*results, LintResult{
				Severity: LintError,
				Path:     fmt.Sprintf("%s.content[%d]", path, i),
				Message:  "tableRow has no cells",
			})
		}
		// Check for links with no href
		for j, cell := range row.Content {
			c.lintLinks(cell, fmt.Sprintf("%s.content[%d].content[%d]", path, i, j), results)
		}
	}
}

func (c *lintCtx) lintList(n Node, path string, results *[]LintResult) {
	if len(n.Content) == 0 {
		*results = append(*results, LintResult{
			Severity: LintError,
			Path:     path,
			Message:  fmt.Sprintf("%s has no items", n.Type),
		})
		return
	}
	for i, item := range n.Content {
		if item.Type != "listItem" {
			*results = append(*results, LintResult{
				Severity: LintError,
				Path:     fmt.Sprintf("%s.content[%d]", path, i),
				Message:  fmt.Sprintf("expected listItem, got %q", item.Type),
			})
		}
	}
}

func (c *lintCtx) lintLinks(n Node, path string, results *[]LintResult) {
	// Check inline marks for links without href
	for _, mark := range n.Marks {
		if mark.Type == "link" {
			href, _ := mark.Attrs["href"].(string)
			if href == "" {
				*results = append(*results, LintResult{
					Severity: LintError,
					Path:     path,
					Message:  "link mark has no href",
				})
			}
		}
	}
	for i, child := range n.Content {
		c.lintLinks(child, fmt.Sprintf("%s.content[%d]", path, i), results)
	}
}
