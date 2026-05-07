package adf

import (
	"strings"
	"testing"
)

func TestLint_CleanDoc(t *testing.T) {
	doc := sampleDoc()
	results := Lint(doc)
	if len(results) != 0 {
		t.Errorf("expected no lint issues for clean doc, got: %v", results)
	}
}

func TestLint_EmptyDoc(t *testing.T) {
	doc := Doc()
	results := Lint(doc)
	if len(results) == 0 {
		t.Fatal("expected warning for empty doc")
	}
	if results[0].Severity != LintWarning {
		t.Errorf("expected warning severity, got %q", results[0].Severity)
	}
}

func TestLint_HeadingNoText(t *testing.T) {
	doc := Doc(
		Heading(2),
		Paragraph(Text("body")),
	)
	results := Lint(doc)
	found := false
	for _, r := range results {
		if r.Severity == LintError && strings.Contains(r.Message, "heading has no text") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for empty heading, got: %v", results)
	}
}

func TestLint_TableNoRows(t *testing.T) {
	doc := Doc(
		Table(),
	)
	results := Lint(doc)
	found := false
	for _, r := range results {
		if r.Severity == LintError && strings.Contains(r.Message, "table has no rows") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for empty table, got: %v", results)
	}
}

func TestLint_ListNoItems(t *testing.T) {
	doc := Doc(
		BulletList(),
	)
	results := Lint(doc)
	found := false
	for _, r := range results {
		if r.Severity == LintError && strings.Contains(r.Message, "has no items") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for empty list, got: %v", results)
	}
}

func TestLint_DuplicateTopLevelHeadings(t *testing.T) {
	doc := Doc(
		Heading(2, Text("Alpha")),
		Paragraph(Text("body A")),
		Heading(2, Text("Alpha")),
		Paragraph(Text("body A2")),
	)
	results := Lint(doc)
	found := false
	for _, r := range results {
		if r.Severity == LintWarning && strings.Contains(r.Message, "duplicate top-level heading") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning for duplicate heading, got: %v", results)
	}
}

func TestLint_LinkNoHref(t *testing.T) {
	// Manually construct a node with a link mark that has no href
	nodeWithBadLink := Node{
		Type: "text",
		Text: "click",
		Marks: []Mark{
			{Type: "link", Attrs: map[string]any{"href": ""}},
		},
	}
	doc := Doc(Paragraph(nodeWithBadLink))
	results := Lint(doc)
	found := false
	for _, r := range results {
		if r.Severity == LintError && strings.Contains(r.Message, "link mark has no href") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for link with no href, got: %v", results)
	}
}

func TestLint_NonDocRoot(t *testing.T) {
	para := Paragraph(Text("not a doc"))
	results := Lint(para)
	if len(results) == 0 || results[0].Severity != LintError {
		t.Errorf("expected error for non-doc root, got: %v", results)
	}
}
