package adf

import (
	"encoding/json"
	"strings"
	"testing"
)

// findFirst returns the first descendant node (depth-first) whose Type matches.
func findFirst(n Node, typ string) *Node {
	if n.Type == typ {
		return &n
	}
	for i := range n.Content {
		if got := findFirst(n.Content[i], typ); got != nil {
			return got
		}
	}
	return nil
}

// findAll returns every descendant whose Type matches (depth-first order).
func findAll(n Node, typ string) []Node {
	var out []Node
	if n.Type == typ {
		out = append(out, n)
	}
	for i := range n.Content {
		out = append(out, findAll(n.Content[i], typ)...)
	}
	return out
}

func mustConvert(t *testing.T, src string) Node {
	t.Helper()
	doc, err := Convert([]byte(src))
	if err != nil {
		t.Fatalf("Convert(%q) returned error: %v", src, err)
	}
	return doc
}

// hasMark returns true if any text node in n carries a mark of type t.
func hasMark(n Node, t string) bool {
	if n.Type == "text" {
		for _, m := range n.Marks {
			if m.Type == t {
				return true
			}
		}
	}
	for _, c := range n.Content {
		if hasMark(c, t) {
			return true
		}
	}
	return false
}

func TestHeadings(t *testing.T) {
	for level := 1; level <= 6; level++ {
		src := strings.Repeat("#", level) + " Heading " + string(rune('A'+level-1))
		doc := mustConvert(t, src)
		h := findFirst(doc, "heading")
		if h == nil {
			t.Fatalf("level %d: no heading node", level)
		}
		if got := h.Attrs["level"].(int); got != level {
			t.Fatalf("level %d: got level=%d", level, got)
		}
	}
}

func TestInlineMarks(t *testing.T) {
	tests := []struct {
		name, src, mark string
	}{
		{"bold", "**hi**", "strong"},
		{"italic", "*hi*", "em"},
		{"code", "`x`", "code"},
		{"link", "[hi](https://example.com)", "link"},
		{"strike", "~~hi~~", "strike"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := mustConvert(t, tc.src)
			if !hasMark(doc, tc.mark) {
				t.Fatalf("expected mark %s in %s", tc.mark, mustJSON(doc))
			}
		})
	}
}

func TestBoldLinkCombined(t *testing.T) {
	doc := mustConvert(t, "**[bold link](https://example.com)**")
	if !hasMark(doc, "strong") || !hasMark(doc, "link") {
		t.Fatalf("expected both strong and link marks: %s", mustJSON(doc))
	}
}

func TestBulletList(t *testing.T) {
	src := "- one\n- two\n- three"
	doc := mustConvert(t, src)
	bl := findFirst(doc, "bulletList")
	if bl == nil {
		t.Fatal("no bulletList")
	}
	if len(bl.Content) != 3 {
		t.Fatalf("want 3 items, got %d", len(bl.Content))
	}
}

func TestOrderedList(t *testing.T) {
	src := "1. one\n2. two"
	doc := mustConvert(t, src)
	ol := findFirst(doc, "orderedList")
	if ol == nil {
		t.Fatal("no orderedList")
	}
	if len(ol.Content) != 2 {
		t.Fatalf("want 2 items, got %d", len(ol.Content))
	}
}

func TestNestedList(t *testing.T) {
	src := "- top\n  - nested\n  - nested2\n- top2"
	doc := mustConvert(t, src)
	lists := findAll(doc, "bulletList")
	if len(lists) < 2 {
		t.Fatalf("expected nested bulletList, got %d lists: %s", len(lists), mustJSON(doc))
	}
}

func TestTable(t *testing.T) {
	src := "| A | B |\n|---|---|\n| 1 | 2 |\n"
	doc := mustConvert(t, src)
	tbl := findFirst(doc, "table")
	if tbl == nil {
		t.Fatalf("no table: %s", mustJSON(doc))
	}
	if hh := findAll(*tbl, "tableHeader"); len(hh) != 2 {
		t.Fatalf("expected 2 tableHeader, got %d", len(hh))
	}
	if cc := findAll(*tbl, "tableCell"); len(cc) != 2 {
		t.Fatalf("expected 2 tableCell, got %d", len(cc))
	}
}

func TestTOCMacro(t *testing.T) {
	doc := mustConvert(t, "[TOC]")
	ext := findFirst(doc, "extension")
	if ext == nil {
		t.Fatalf("no extension node: %s", mustJSON(doc))
	}
	if ext.Attrs["extensionKey"] != "toc" {
		t.Fatalf("expected extensionKey=toc, got %v", ext.Attrs["extensionKey"])
	}
}

func TestTOCWithParams(t *testing.T) {
	doc := mustConvert(t, "[TOC maxLevel=4 minLevel=1]")
	ext := findFirst(doc, "extension")
	if ext == nil {
		t.Fatal("no extension")
	}
	params := ext.Attrs["parameters"].(map[string]any)
	mp := params["macroParams"].(map[string]any)
	if mp["maxLevel"].(map[string]any)["value"] != "4" {
		t.Fatalf("bad maxLevel: %v", mp["maxLevel"])
	}
	if mp["minLevel"].(map[string]any)["value"] != "1" {
		t.Fatalf("bad minLevel: %v", mp["minLevel"])
	}
}

func TestExpandBlock(t *testing.T) {
	src := ":::expand Click me\nHello **world**\n\n- a\n- b\n:::"
	doc := mustConvert(t, src)
	exp := findFirst(doc, "expand")
	if exp == nil {
		t.Fatalf("no expand: %s", mustJSON(doc))
	}
	if exp.Attrs["title"] != "Click me" {
		t.Fatalf("bad title: %v", exp.Attrs["title"])
	}
	if findFirst(*exp, "bulletList") == nil {
		t.Fatalf("expected bulletList inside expand: %s", mustJSON(*exp))
	}
}

func TestWarningPanel(t *testing.T) {
	src := ":::warning Heads up\nBe careful.\n:::"
	doc := mustConvert(t, src)
	p := findFirst(doc, "panel")
	if p == nil {
		t.Fatalf("no panel: %s", mustJSON(doc))
	}
	if p.Attrs["panelType"] != "warning" {
		t.Fatalf("bad type: %v", p.Attrs["panelType"])
	}
}

func TestInfoPanelNoTitle(t *testing.T) {
	src := ":::info\nJust info.\n:::"
	doc := mustConvert(t, src)
	p := findFirst(doc, "panel")
	if p == nil {
		t.Fatal("no panel")
	}
	if p.Attrs["panelType"] != "info" {
		t.Fatalf("bad type: %v", p.Attrs["panelType"])
	}
}

func TestExpandContainingTable(t *testing.T) {
	src := ":::expand Details\n| A | B |\n|---|---|\n| 1 | 2 |\n:::"
	doc := mustConvert(t, src)
	exp := findFirst(doc, "expand")
	if exp == nil {
		t.Fatalf("no expand: %s", mustJSON(doc))
	}
	if findFirst(*exp, "table") == nil {
		t.Fatalf("expected table inside expand: %s", mustJSON(*exp))
	}
}

func TestCodeBlockWithLang(t *testing.T) {
	src := "```go\nfmt.Println(\"hi\")\n```"
	doc := mustConvert(t, src)
	cb := findFirst(doc, "codeBlock")
	if cb == nil {
		t.Fatalf("no codeBlock: %s", mustJSON(doc))
	}
	if cb.Attrs["language"] != "go" {
		t.Fatalf("bad lang: %v", cb.Attrs["language"])
	}
}

func TestBlockquote(t *testing.T) {
	doc := mustConvert(t, "> quoted")
	if findFirst(doc, "blockquote") == nil {
		t.Fatalf("no blockquote: %s", mustJSON(doc))
	}
}

func TestRule(t *testing.T) {
	doc := mustConvert(t, "before\n\n---\n\nafter")
	if findFirst(doc, "rule") == nil {
		t.Fatalf("no rule: %s", mustJSON(doc))
	}
}

func TestEmptyInput(t *testing.T) {
	doc := mustConvert(t, "")
	if doc.Type != "doc" {
		t.Fatalf("want doc, got %v", doc.Type)
	}
	if len(doc.Content) != 0 {
		t.Fatalf("expected empty content, got %d", len(doc.Content))
	}
}

func TestWhitespaceOnly(t *testing.T) {
	doc := mustConvert(t, "   \n\n  \t  \n")
	if doc.Type != "doc" {
		t.Fatalf("want doc")
	}
}

func TestUnclosedBlockErrors(t *testing.T) {
	_, err := Convert([]byte(":::expand Oops\nno close here"))
	if err == nil {
		t.Fatal("expected error for unterminated ::: block")
	}
}

func mustJSON(n Node) string {
	b, _ := json.MarshalIndent(n, "", "  ")
	return string(b)
}
