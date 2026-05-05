package adf

import (
	"strings"
	"testing"
)

func TestBuildDigest_BasicSections(t *testing.T) {
	doc := Doc(
		Heading(2, Text("Contexto")),
		Paragraph(Text("Esta é a primeira seção com algumas palavras.")),
		Heading(2, Text("Problema")),
		Paragraph(Text("Outra seção.")),
		Panel("warning", "", Paragraph(Text("Atenção"))),
	)

	d := BuildDigest(doc, "12345", "Test Page", "https://example.com/p/12345", 7)

	if d.PageID != "12345" {
		t.Errorf("want PageID=12345, got %q", d.PageID)
	}
	if d.Title != "Test Page" {
		t.Errorf("want Title=Test Page, got %q", d.Title)
	}
	if d.Version != 7 {
		t.Errorf("want Version=7, got %d", d.Version)
	}
	if len(d.Sections) != 2 {
		t.Fatalf("want 2 sections, got %d: %+v", len(d.Sections), d.Sections)
	}
	if d.Sections[0].Heading != "Contexto" {
		t.Errorf("section 0 heading: want Contexto, got %q", d.Sections[0].Heading)
	}
	if d.Sections[1].Heading != "Problema" {
		t.Errorf("section 1 heading: want Problema, got %q", d.Sections[1].Heading)
	}
	// "Problema" section contains the warning panel
	found := false
	for _, m := range d.Sections[1].Macros {
		if m == "panel-warning" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected panel-warning macro on Problema section, got %v", d.Sections[1].Macros)
	}
	if d.MacroCounts["panel-warning"] != 1 {
		t.Errorf("want panel-warning count = 1, got %d", d.MacroCounts["panel-warning"])
	}
}

func TestBuildDigest_FormatTextIsCompact(t *testing.T) {
	// Build a page that would be ~5 KB as ADF JSON
	body := []Node{
		Heading(2, Text("Section A")),
		Paragraph(Text(strings.Repeat("word ", 200))),
		Heading(2, Text("Section B")),
		Paragraph(Text(strings.Repeat("foo ", 200))),
		Heading(3, Text("Sub B1")),
		Paragraph(Text(strings.Repeat("bar ", 100))),
	}
	doc := Doc(body...)

	d := BuildDigest(doc, "999", "Big Page", "https://example.com/p/999", 1)
	text := d.FormatText()

	// Should be tiny — way under 1 KB
	if len(text) > 1000 {
		t.Errorf("digest text too long: %d bytes\n%s", len(text), text)
	}
	if !strings.Contains(text, "Section A") {
		t.Errorf("expected Section A in output:\n%s", text)
	}
	if !strings.Contains(text, "Section B") {
		t.Errorf("expected Section B in output:\n%s", text)
	}
	if !strings.Contains(text, "Sub B1") {
		t.Errorf("expected Sub B1 in output:\n%s", text)
	}
}

func TestBuildDigest_LinkCount(t *testing.T) {
	doc := Doc(
		Heading(2, Text("Refs")),
		Paragraph(
			Text("see "),
			Text("here", Link("https://a.com")),
			Text(" and "),
			Text("there", Link("https://b.com")),
		),
	)
	d := BuildDigest(doc, "1", "t", "u", 1)
	if d.LinksCount != 2 {
		t.Errorf("want 2 links, got %d", d.LinksCount)
	}
}

func TestBuildDigest_TOCMacro(t *testing.T) {
	doc := Doc(
		TOC(2, 3),
		Heading(2, Text("Body")),
		Paragraph(Text("hello")),
	)
	d := BuildDigest(doc, "1", "t", "u", 1)
	if d.MacroCounts["toc"] != 1 {
		t.Errorf("want toc count=1, got %v", d.MacroCounts)
	}
}

func TestBuildDigest_StatusEmoji(t *testing.T) {
	cases := []struct {
		title       string
		wantEmoji   string
		wantLabel   string
	}{
		{"🟢 Acme - Em andamento", "🟢", "active"},
		{"🟡 Banco X - Negociação", "🟡", "in-progress"},
		{"🟠 Serasa - Avaliação", "🟠", "evaluating"},
		{"🔴 Projeto Y - Bloqueado", "🔴", "blocked"},
		{"🔵 Distrito - Pesquisado", "🔵", "researched"},
		{"⚪ Antiga Iniciativa", "⚪", "idle"},
		{"✅ Migração - Concluída", "✅", "done"},
		{"Página sem emoji", "", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		doc := Doc(Heading(2, Text("Body")))
		d := BuildDigest(doc, "1", c.title, "u", 1)
		if d.StatusEmoji != c.wantEmoji {
			t.Errorf("title=%q: emoji=%q, want %q", c.title, d.StatusEmoji, c.wantEmoji)
		}
		if d.Status != c.wantLabel {
			t.Errorf("title=%q: label=%q, want %q", c.title, d.Status, c.wantLabel)
		}
	}
}

func TestBuildDigest_StatusInFormatText(t *testing.T) {
	doc := Doc(Heading(2, Text("Body")))
	d := BuildDigest(doc, "1", "🟠 Serasa - Inovação", "u", 1)
	out := d.FormatText()
	if !strings.Contains(out, "Status: 🟠 evaluating") {
		t.Errorf("expected status line in output:\n%s", out)
	}
}

func TestComputeTreePrefixes_FlatTopLevel(t *testing.T) {
	// Top-level h2 sections get no tree decoration.
	secs := []SectionSummary{
		{Level: 2, Heading: "A"},
		{Level: 2, Heading: "B"},
		{Level: 2, Heading: "C"},
	}
	got := computeTreePrefixes(secs)
	for i, p := range got {
		if p != "" {
			t.Errorf("[%d] expected empty prefix for h2, got %q", i, p)
		}
	}
}

func TestComputeTreePrefixes_H2WithThreeH3(t *testing.T) {
	// h2 A
	//   h3 A1   ├─
	//   h3 A2   ├─
	//   h3 A3   └─
	// h2 B
	//   h3 B1   └─
	secs := []SectionSummary{
		{Level: 2, Heading: "A"},
		{Level: 3, Heading: "A1"},
		{Level: 3, Heading: "A2"},
		{Level: 3, Heading: "A3"},
		{Level: 2, Heading: "B"},
		{Level: 3, Heading: "B1"},
	}
	got := computeTreePrefixes(secs)
	want := []string{"", "├─ ", "├─ ", "└─ ", "", "└─ "}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] %q: got %q, want %q", i, secs[i].Heading, got[i], want[i])
		}
	}
}

func TestComputeTreePrefixes_DeepNesting(t *testing.T) {
	// h2 A
	//   h3 A1
	//     h4 A1a    ├─
	//     h4 A1b    └─    (uses "│  " ancestor column because A1 has sibling A2 below)
	//   h3 A2
	secs := []SectionSummary{
		{Level: 2, Heading: "A"},
		{Level: 3, Heading: "A1"},
		{Level: 4, Heading: "A1a"},
		{Level: 4, Heading: "A1b"},
		{Level: 3, Heading: "A2"},
	}
	got := computeTreePrefixes(secs)
	// h2 A: ""
	// h3 A1: "├─ "  (A2 follows at same level)
	// h4 A1a: "│  ├─ "  (h3 A1 has sibling A2 → vertical bar in col 3; A1a has sibling A1b at level 4)
	// h4 A1b: "│  └─ "  (last h4 under A1)
	// h3 A2: "└─ "  (last h3)
	want := []string{"", "├─ ", "│  ├─ ", "│  └─ ", "└─ "}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] %q: got %q, want %q", i, secs[i].Heading, got[i], want[i])
		}
	}
}

func TestComputeTreePrefixes_IntroPrelude(t *testing.T) {
	// The synthetic "(intro)" section (Level=0) has no decoration and is
	// not counted as an ancestor of anything below.
	secs := []SectionSummary{
		{Level: 0, Heading: "(intro)"},
		{Level: 2, Heading: "A"},
		{Level: 3, Heading: "A1"},
	}
	got := computeTreePrefixes(secs)
	want := []string{"", "", "└─ "}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] %q: got %q, want %q", i, secs[i].Heading, got[i], want[i])
		}
	}
}
