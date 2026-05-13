package adf

import (
	"strings"
	"testing"
)

func TestTableUpdateRow_Replaces(t *testing.T) {
	doc := tableDoc()
	updated, err := TableUpdateRow(doc, "Sócios", 0, FirstCellMatch("Diego"), "Diego|999")
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
	_, err := TableUpdateRow(doc, "Sócios", 0, FirstCellMatch("Carolina"), "Carolina|111")
	if err == nil {
		t.Fatalf("expected error for missing row")
	}
	if !strings.Contains(err.Error(), "Carolina") {
		t.Errorf("expected error to mention search text, got %v", err)
	}
}

func TestTableUpdateCell_ByColName(t *testing.T) {
	doc := tableDoc()
	updated, err := TableUpdateCell(doc, "Sócios", 0, FirstCellMatch("Diego"), "pageId", "abc")
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
	_, err := TableUpdateCell(doc, "Sócios", 0, FirstCellMatch("Diego"), "Inexistente", "abc")
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
	updated, existed, err := TableAddRow(doc, "Sócios", 0, "Carolina|456", "", false, MatchSpec{})
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
	updated, _, err := TableAddRow(doc, "Sócios", 0, "Carolina|456", "Página", false, MatchSpec{})
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
	updated, existed, err := TableAddRow(doc, "Sócios", 0, "Diego|999", "", true, MatchSpec{})
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
	updated, _, err := TableAddRow(doc, "Index", 3, "newrow|val", "", false, MatchSpec{})
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
	updated, err := TableRemoveRow(doc, "Sócios", 0, FirstCellMatch("Diego"))
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
	_, err := TableRemoveRow(doc, "Sócios", 0, FirstCellMatch("Nonexistent"))
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

// rankTableDoc builds a doc where the first column is a non-unique rank, so
// the legacy first-cell match can't distinguish between rows.
//
//	## ICPs
//	| Rank | ICP             | Score |
//	|  1   | Personal        | 4.1   |
//	|  2   | Psicólogo       | 3.8   |
//	|  3   | Lash designer   | 2.4   |
func rankTableDoc() Node {
	return Doc(
		Heading(2, Text("ICPs")),
		Table(
			TableRow(
				TableHeader(Paragraph(Text("Rank"))),
				TableHeader(Paragraph(Text("ICP"))),
				TableHeader(Paragraph(Text("Score"))),
			),
			TableRow(
				TableCell(Paragraph(Text("1"))),
				TableCell(Paragraph(Text("Personal"))),
				TableCell(Paragraph(Text("4.1"))),
			),
			TableRow(
				TableCell(Paragraph(Text("2"))),
				TableCell(Paragraph(Text("Psicólogo"))),
				TableCell(Paragraph(Text("3.8"))),
			),
			TableRow(
				TableCell(Paragraph(Text("3"))),
				TableCell(Paragraph(Text("Lash designer"))),
				TableCell(Paragraph(Text("2.4"))),
			),
		),
	)
}

func TestTableUpdateCell_ByMatchCol(t *testing.T) {
	doc := rankTableDoc()
	// Match by the "ICP" column and update the "Score" column.
	updated, err := TableUpdateCell(doc, "ICPs", 0,
		MatchSpec{Col: "ICP", Value: "Lash designer"}, "Score", "5.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	table := updated.Content[1]
	// Lash row is the 4th (index 3).
	row := table.Content[3]
	if cellText(row.Content[2]) != "5.0" {
		t.Errorf("want Score=5.0, got %q", cellText(row.Content[2]))
	}
	// Other rows must be untouched.
	if cellText(table.Content[1].Content[2]) != "4.1" {
		t.Errorf("Personal Score should stay 4.1, got %q", cellText(table.Content[1].Content[2]))
	}
}

func TestTableUpdateRow_ByMatchCol(t *testing.T) {
	doc := rankTableDoc()
	updated, err := TableUpdateRow(doc, "ICPs", 0,
		MatchSpec{Col: "ICP", Value: "Psicólogo"}, "2|Psicólogo (atualizado)|3.9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	table := updated.Content[1]
	row := table.Content[2]
	if cellText(row.Content[1]) != "Psicólogo (atualizado)" {
		t.Errorf("want ICP cell updated, got %q", cellText(row.Content[1]))
	}
	if cellText(row.Content[2]) != "3.9" {
		t.Errorf("want Score=3.9, got %q", cellText(row.Content[2]))
	}
}

func TestTableRemoveRow_ByMatchCol(t *testing.T) {
	doc := rankTableDoc()
	updated, err := TableRemoveRow(doc, "ICPs", 0,
		MatchSpec{Col: "ICP", Value: "Lash designer"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	table := updated.Content[1]
	// 4 rows (header + 3 data) -> 3 rows after removal.
	if len(table.Content) != 3 {
		t.Fatalf("want 3 rows after removal, got %d", len(table.Content))
	}
	for _, row := range table.Content {
		if strings.Contains(cellText(row.Content[1]), "Lash") {
			t.Errorf("Lash row should be gone, still present: %s", cellText(row.Content[1]))
		}
	}
}

func TestTableUpdateCell_MatchCol_UnknownColumn(t *testing.T) {
	doc := rankTableDoc()
	_, err := TableUpdateCell(doc, "ICPs", 0,
		MatchSpec{Col: "Inexistente", Value: "Lash designer"}, "Score", "5.0")
	if err == nil {
		t.Fatal("expected error for unknown match column")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Inexistente") {
		t.Errorf("error should mention the missing column: %v", err)
	}
	for _, h := range []string{"Rank", "ICP", "Score"} {
		if !strings.Contains(msg, h) {
			t.Errorf("error should list available column %q: %v", h, err)
		}
	}
}

func TestTableUpdateRow_MatchCol_NoMatch(t *testing.T) {
	doc := rankTableDoc()
	_, err := TableUpdateRow(doc, "ICPs", 0,
		MatchSpec{Col: "ICP", Value: "Nonexistent"}, "9|Nonexistent|0.0")
	if err == nil {
		t.Fatal("expected error when no row matches the column value")
	}
	if !strings.Contains(err.Error(), "Nonexistent") {
		t.Errorf("error should mention the search value: %v", err)
	}
	if !strings.Contains(err.Error(), "ICP") {
		t.Errorf("error should mention the column name: %v", err)
	}
}

func TestTableAddRow_IfMissing_ByMatchCol(t *testing.T) {
	doc := rankTableDoc()
	// "Lash designer" already exists in column "ICP" — should be a no-op
	// even though the first-cell rank ("4") is unique.
	updated, existed, err := TableAddRow(doc, "ICPs", 0, "4|Lash designer|9.9", "", true,
		MatchSpec{Col: "ICP", Value: "Lash designer"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !existed {
		t.Fatal("expected existed=true when matching column already has the value")
	}
	// Doc untouched.
	if len(updated.Content[1].Content) != 4 {
		t.Errorf("table should be unchanged (4 rows), got %d", len(updated.Content[1].Content))
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

func TestTableMoveRow_ByPosition(t *testing.T) {
	doc := rankTableDoc()
	// Move "Lash designer" (currently row 3, last data row) to position 1
	// (immediately below the header).
	updated, err := TableMoveRow(doc, "ICPs", 0,
		MatchSpec{Col: "ICP", Value: "Lash designer"}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	table := updated.Content[1]
	if len(table.Content) != 4 {
		t.Fatalf("table should still have 4 rows (header + 3 data), got %d", len(table.Content))
	}
	// Row 0 = header, row 1 = should be Lash now
	if got := cellText(table.Content[1].Content[1]); !strings.Contains(got, "Lash designer") {
		t.Errorf("expected Lash designer at position 1, got %q", got)
	}
	// Row 2 should be Personal (was 1)
	if got := cellText(table.Content[2].Content[1]); !strings.Contains(got, "Personal") {
		t.Errorf("expected Personal at position 2, got %q", got)
	}
	// Row 3 should be Psicólogo (was 2)
	if got := cellText(table.Content[3].Content[1]); !strings.Contains(got, "Psicólogo") {
		t.Errorf("expected Psicólogo at position 3, got %q", got)
	}
}

func TestTableMoveRow_PositionClamp(t *testing.T) {
	doc := rankTableDoc()
	// Asking for position 999 should clamp to the last data row.
	updated, err := TableMoveRow(doc, "ICPs", 0,
		MatchSpec{Col: "ICP", Value: "Personal"}, 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	table := updated.Content[1]
	// Personal should be at position 3 (last data row, idx 3 in full slice).
	if got := cellText(table.Content[3].Content[1]); !strings.Contains(got, "Personal") {
		t.Errorf("expected Personal at last position after clamp, got %q", got)
	}
}

func TestTableMoveRow_HeaderRefused(t *testing.T) {
	doc := rankTableDoc()
	// Try to move the header row by matching its first cell "Rank".
	_, err := TableMoveRow(doc, "ICPs", 0,
		FirstCellMatch("Rank"), 2)
	if err == nil {
		t.Fatal("expected error when targeting the header row, got nil")
	}
	if !strings.Contains(err.Error(), "header") {
		t.Errorf("error should mention header row: %v", err)
	}
}

func TestTableMoveRow_NoMatch(t *testing.T) {
	doc := rankTableDoc()
	_, err := TableMoveRow(doc, "ICPs", 0,
		MatchSpec{Col: "ICP", Value: "Nonexistent"}, 1)
	if err == nil {
		t.Fatal("expected error for non-existent value, got nil")
	}
}
