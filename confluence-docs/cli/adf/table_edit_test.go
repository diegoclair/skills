package adf

import (
	"strings"
	"testing"
)

func TestTableUpdateRow_Replaces(t *testing.T) {
	doc := tableDoc()
	updated, err := TableUpdateRow(doc, "Sócios", 0, "Diego", "Diego|999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	table := updated.Content[1]
	if len(table.Content) != 2 {
		t.Fatalf("want 2 rows, got %d", len(table.Content))
	}
	row := table.Content[1]
	if cellText(row.Content[0]) != "Diego" {
		t.Errorf("want first cell 'Diego', got %q", cellText(row.Content[0]))
	}
	if cellText(row.Content[1]) != "999" {
		t.Errorf("want second cell '999', got %q", cellText(row.Content[1]))
	}
}

func TestTableUpdateRow_NoMatch(t *testing.T) {
	doc := tableDoc()
	_, err := TableUpdateRow(doc, "Sócios", 0, "Carolina", "Carolina|111")
	if err == nil {
		t.Fatalf("expected error for missing row")
	}
	if !strings.Contains(err.Error(), "Carolina") {
		t.Errorf("expected error to mention search text, got %v", err)
	}
}

func TestTableUpdateCell_ByColName(t *testing.T) {
	doc := tableDoc()
	updated, err := TableUpdateCell(doc, "Sócios", 0, "Diego", "pageId", "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	table := updated.Content[1]
	row := table.Content[1]
	if cellText(row.Content[0]) != "Diego" {
		t.Errorf("first cell should be unchanged, got %q", cellText(row.Content[0]))
	}
	if cellText(row.Content[1]) != "abc" {
		t.Errorf("want second cell 'abc', got %q", cellText(row.Content[1]))
	}
}

func TestTableUpdateCell_UnknownColumn(t *testing.T) {
	doc := tableDoc()
	_, err := TableUpdateCell(doc, "Sócios", 0, "Diego", "Inexistente", "abc")
	if err == nil {
		t.Fatalf("expected error for unknown column")
	}
	if !strings.Contains(err.Error(), "Inexistente") {
		t.Errorf("expected error to mention column name, got %v", err)
	}
}

func TestDetectShellExpansionHint_DollarStripped(t *testing.T) {
	// Page heading "Cálculo (R$200 = R$100k GMV)" → after bash ate $200 and $100k,
	// CLI receives "Cálculo (R = Rk GMV)". Hint should fire.
	headings := []string{"Cálculo (R$200 = R$100k GMV)", "Other heading"}
	hint := detectShellExpansionHint("Cálculo (R = Rk GMV)", headings)
	if hint == "" {
		t.Fatalf("expected non-empty hint when headings contain $ patterns matching target with vars stripped")
	}
	if !strings.Contains(hint, "single quotes") {
		t.Errorf("hint should suggest single quotes, got %q", hint)
	}
}

func TestDetectShellExpansionHint_TargetHasDollar_NoHint(t *testing.T) {
	headings := []string{"Cálculo (R$200 GMV)"}
	hint := detectShellExpansionHint("Cálculo (R$200 GMV)", headings)
	if hint != "" {
		t.Errorf("expected empty hint when target already contains $, got %q", hint)
	}
}

func TestDetectShellExpansionHint_NoMatch_NoHint(t *testing.T) {
	headings := []string{"Section", "Another"}
	hint := detectShellExpansionHint("Foo", headings)
	if hint != "" {
		t.Errorf("expected empty hint when no $ patterns relate, got %q", hint)
	}
}

// tableDoc builds a doc with two sections, each containing a table.
//
//	## Sócios
//	| Página | pageId |
//	| Diego  | 123    |
//	## Outro
//	| Foo | Bar |
//	| A   | B   |
func tableDoc() Node {
	return Doc(
		Heading(2, Text("Sócios")),
		Table(
			TableRow(TableHeader(Paragraph(Text("Página"))), TableHeader(Paragraph(Text("pageId")))),
			TableRow(TableCell(Paragraph(Text("Diego"))), TableCell(Paragraph(Text("123")))),
		),
		Heading(2, Text("Outro")),
		Table(
			TableRow(TableHeader(Paragraph(Text("Foo"))), TableHeader(Paragraph(Text("Bar")))),
			TableRow(TableCell(Paragraph(Text("A"))), TableCell(Paragraph(Text("B")))),
		),
	)
}

func TestTableAddRow_Append(t *testing.T) {
	doc := tableDoc()
	updated, existed, err := TableAddRow(doc, "Sócios", 0, "Carolina|456", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if existed {
		t.Fatalf("expected existed=false")
	}

	// Table is at index 1 (after the heading)
	table := updated.Content[1]
	if table.Type != "table" {
		t.Fatalf("expected table at index 1, got %q", table.Type)
	}
	// Original had 2 rows (header + 1 data); now should have 3
	if len(table.Content) != 3 {
		t.Fatalf("want 3 rows, got %d", len(table.Content))
	}
	lastRow := table.Content[2]
	if cellText(lastRow.Content[0]) != "Carolina" {
		t.Errorf("want first cell 'Carolina', got %q", cellText(lastRow.Content[0]))
	}
	if cellText(lastRow.Content[1]) != "456" {
		t.Errorf("want second cell '456', got %q", cellText(lastRow.Content[1]))
	}
}

func TestTableAddRow_AfterRow(t *testing.T) {
	doc := tableDoc()
	// The Sócios table has rows: header, Diego. Insert after the header row.
	updated, _, err := TableAddRow(doc, "Sócios", 0, "Carolina|456", "Página", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	table := updated.Content[1]
	if len(table.Content) != 3 {
		t.Fatalf("want 3 rows, got %d", len(table.Content))
	}
	// After "Página" header row, the new row should be at index 1
	if cellText(table.Content[1].Content[0]) != "Carolina" {
		t.Errorf("want inserted row after header, got %q", cellText(table.Content[1].Content[0]))
	}
}

func TestTableAddRow_IfMissing_NoOp(t *testing.T) {
	doc := tableDoc()
	// "Diego" already exists in first cell
	updated, existed, err := TableAddRow(doc, "Sócios", 0, "Diego|999", "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !existed {
		t.Fatalf("expected existed=true for duplicate first cell")
	}
	// Table should be unchanged
	table := updated.Content[1]
	if len(table.Content) != 2 {
		t.Fatalf("want 2 rows (unchanged), got %d", len(table.Content))
	}
}

func TestTableAddRow_AtLevel(t *testing.T) {
	// Doc has two headings with same text but different levels
	doc := Doc(
		Heading(2, Text("Index")),
		Table(
			TableRow(TableHeader(Paragraph(Text("H1"))), TableHeader(Paragraph(Text("H2")))),
			TableRow(TableCell(Paragraph(Text("row1a"))), TableCell(Paragraph(Text("row1b")))),
		),
		Heading(3, Text("Index")),
		Table(
			TableRow(TableHeader(Paragraph(Text("H1"))), TableHeader(Paragraph(Text("H2")))),
			TableRow(TableCell(Paragraph(Text("row2a"))), TableCell(Paragraph(Text("row2b")))),
		),
	)
	// Target the h3 "Index"
	updated, _, err := TableAddRow(doc, "Index", 3, "newrow|val", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// h2 table (index 1) should be unchanged
	if len(updated.Content[1].Content) != 2 {
		t.Fatalf("h2 table should be unchanged, got %d rows", len(updated.Content[1].Content))
	}
	// h3 table (index 3) should have 3 rows
	if len(updated.Content[3].Content) != 3 {
		t.Fatalf("h3 table should have 3 rows, got %d", len(updated.Content[3].Content))
	}
}

func TestTableRemoveRow(t *testing.T) {
	doc := tableDoc()
	updated, err := TableRemoveRow(doc, "Sócios", 0, "Diego")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	table := updated.Content[1]
	if len(table.Content) != 1 {
		t.Fatalf("want 1 row (header only), got %d", len(table.Content))
	}
}

func TestTableRemoveRow_NotFound(t *testing.T) {
	doc := tableDoc()
	_, err := TableRemoveRow(doc, "Sócios", 0, "Nonexistent")
	if err == nil {
		t.Fatal("expected error for missing row, got nil")
	}
}

func TestParseRowCells(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"a|b|c", []string{"a", "b", "c"}},
		{`a\|b|c`, []string{"a|b", "c"}},
		{"single", []string{"single"}},
		{"", []string{""}},
	}
	for _, tc := range cases {
		got, err := parseRowCells(tc.input)
		if err != nil {
			t.Errorf("parseRowCells(%q): unexpected error %v", tc.input, err)
			continue
		}
		if !eqStrings(got, tc.want) {
			t.Errorf("parseRowCells(%q): want %v, got %v", tc.input, tc.want, got)
		}
	}
}

func TestFindSectionBoundsAtLevel(t *testing.T) {
	doc := Doc(
		Heading(2, Text("Alpha")),
		Paragraph(Text("body A")),
		Heading(3, Text("Alpha")),
		Paragraph(Text("body A-sub")),
		Heading(2, Text("Bravo")),
	)

	// Without level filter: first match (h2)
	start, end, ok := findSectionBoundsAtLevel(doc.Content, "Alpha", 0)
	if !ok {
		t.Fatal("expected match without level filter")
	}
	if start != 0 || end != 4 {
		t.Errorf("no-level filter: want [0,4), got [%d,%d)", start, end)
	}

	// With level 3: should match h3 "Alpha"
	start, end, ok = findSectionBoundsAtLevel(doc.Content, "Alpha", 3)
	if !ok {
		t.Fatal("expected match at level 3")
	}
	if start != 2 || end != 4 {
		t.Errorf("level-3 filter: want [2,4), got [%d,%d)", start, end)
	}
}

func TestSectionNotFoundError_ListsHeadings(t *testing.T) {
	doc := sampleDoc() // Alpha, Bravo, Charlie
	err := sectionNotFoundError(doc.Content, "Nonexistent")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	msg := err.Error()
	for _, h := range []string{"Alpha", "Bravo", "Charlie"} {
		if !strings.Contains(msg, h) {
			t.Errorf("error message missing heading %q: %v", h, msg)
		}
	}
}

func TestReplaceSectionAtLevel(t *testing.T) {
	doc := Doc(
		Heading(2, Text("Ops")),
		Paragraph(Text("h2 body")),
		Heading(3, Text("Ops")),
		Paragraph(Text("h3 body")),
	)
	frag := []Node{Heading(3, Text("Ops v2")), Paragraph(Text("new h3 body"))}
	updated, err := ReplaceSectionAtLevel(doc, "Ops", 3, frag)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// h2 should be untouched
	if headingText(updated.Content[0]) != "Ops" {
		t.Errorf("h2 heading changed")
	}
	if paraText(updated.Content[1]) != "h2 body" {
		t.Errorf("h2 body changed")
	}
	// h3 should be replaced
	if headingText(updated.Content[2]) != "Ops v2" {
		t.Errorf("h3 heading not replaced: %q", headingText(updated.Content[2]))
	}
}
