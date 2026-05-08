package adf

import (
	"fmt"
	"regexp"
	"strings"
)

// TableAddRow adds a row to the first table inside the section with the given
// heading. The row cells are provided as a pipe-separated string (e.g.
// "col1|col2|col3"). Use \| to include a literal pipe in a cell value.
//
//   - atLevel: if > 0, only match headings at this exact level; 0 = first match wins.
//   - afterRowText: insert after the row whose first cell contains this text.
//     Empty string means append at the end.
//   - ifMissing: if true, skip silently if a row with the same first cell already exists.
//
// Returns (updatedDoc, alreadyExisted, error).
func TableAddRow(doc Node, headingText string, atLevel int, rowText string, afterRowText string, ifMissing bool) (Node, bool, error) {
	cells, err := parseRowCells(rowText)
	if err != nil {
		return Node{}, false, err
	}
	if len(cells) == 0 {
		return Node{}, false, fmt.Errorf("row must have at least one cell")
	}

	idx, end, ok := findSectionBoundsAtLevel(doc.Content, headingText, atLevel)
	if !ok {
		return Node{}, false, sectionNotFoundError(doc.Content, headingText)
	}

	// Find the first table in the section.
	tableIdx := -1
	for i := idx; i < end; i++ {
		if doc.Content[i].Type == "table" {
			tableIdx = i
			break
		}
	}
	if tableIdx == -1 {
		return Node{}, false, fmt.Errorf("no table found in section %q", headingText)
	}

	table := doc.Content[tableIdx]

	// Check idempotency if ifMissing is set.
	firstCell := cells[0]
	if ifMissing {
		for _, row := range table.Content {
			if row.Type != "tableRow" {
				continue
			}
			if len(row.Content) > 0 {
				existing := strings.TrimSpace(cellText(row.Content[0]))
				if existing == strings.TrimSpace(firstCell) {
					return doc, true, nil // already exists
				}
			}
		}
	}

	// Build the new row node.
	newRow := buildTableRow(cells)

	// Determine where to insert.
	var newRows []Node
	if afterRowText == "" {
		// Append at end.
		newRows = append(append([]Node{}, table.Content...), newRow)
	} else {
		inserted := false
		for _, row := range table.Content {
			newRows = append(newRows, row)
			if !inserted && row.Type == "tableRow" && len(row.Content) > 0 {
				if strings.Contains(cellText(row.Content[0]), afterRowText) {
					newRows = append(newRows, newRow)
					inserted = true
				}
			}
		}
		if !inserted {
			// afterRowText not found — append at end with a notice.
			newRows = append(newRows, newRow)
		}
	}

	// Rebuild the table with the new rows.
	updatedTable := table
	updatedTable.Content = newRows

	// Rebuild the doc content.
	newContent := make([]Node, len(doc.Content))
	copy(newContent, doc.Content)
	newContent[tableIdx] = updatedTable

	out := doc
	out.Content = newContent
	return out, false, nil
}

// TableUpdateRow replaces the entire row whose first cell contains matchText
// with new cells parsed from rowText (pipe-separated).
//
//   - atLevel: 0 = first matching heading; >0 = require exact level
//   - matchText: substring searched in the first cell of each row
//
// Returns updated doc, or error (section/table/row not found, parse error).
func TableUpdateRow(doc Node, headingText string, atLevel int, matchText string, rowText string) (Node, error) {
	cells, err := parseRowCells(rowText)
	if err != nil {
		return Node{}, err
	}
	if len(cells) == 0 {
		return Node{}, fmt.Errorf("row must have at least one cell")
	}
	idx, end, ok := findSectionBoundsAtLevel(doc.Content, headingText, atLevel)
	if !ok {
		return Node{}, sectionNotFoundError(doc.Content, headingText)
	}
	tableIdx := -1
	for i := idx; i < end; i++ {
		if doc.Content[i].Type == "table" {
			tableIdx = i
			break
		}
	}
	if tableIdx == -1 {
		return Node{}, fmt.Errorf("no table found in section %q", headingText)
	}
	table := doc.Content[tableIdx]
	newRow := buildTableRow(cells)
	updated := false
	newRows := make([]Node, 0, len(table.Content))
	for _, row := range table.Content {
		if !updated && row.Type == "tableRow" && len(row.Content) > 0 &&
			strings.Contains(cellText(row.Content[0]), matchText) {
			newRows = append(newRows, newRow)
			updated = true
			continue
		}
		newRows = append(newRows, row)
	}
	if !updated {
		return Node{}, fmt.Errorf("no row with first cell containing %q found in table", matchText)
	}
	updatedTable := table
	updatedTable.Content = newRows
	newContent := make([]Node, len(doc.Content))
	copy(newContent, doc.Content)
	newContent[tableIdx] = updatedTable
	out := doc
	out.Content = newContent
	return out, nil
}

// TableUpdateCell updates a single cell's text in the row whose first cell
// contains matchText. The target column is identified by colName, matched
// against the first row of the table (assumed to be the header).
//
//   - atLevel: 0 = first matching heading; >0 = require exact level
//   - matchText: substring searched in the first cell of each row (skips header)
//   - colName: substring matched against the first row's cells (column header)
//   - newValue: replacement text for the matched cell
//
// Returns updated doc, or error if section/table/row/column not found.
func TableUpdateCell(doc Node, headingText string, atLevel int, matchText string, colName string, newValue string) (Node, error) {
	idx, end, ok := findSectionBoundsAtLevel(doc.Content, headingText, atLevel)
	if !ok {
		return Node{}, sectionNotFoundError(doc.Content, headingText)
	}
	tableIdx := -1
	for i := idx; i < end; i++ {
		if doc.Content[i].Type == "table" {
			tableIdx = i
			break
		}
	}
	if tableIdx == -1 {
		return Node{}, fmt.Errorf("no table found in section %q", headingText)
	}
	table := doc.Content[tableIdx]
	if len(table.Content) == 0 {
		return Node{}, fmt.Errorf("table in section %q is empty", headingText)
	}

	// Find column index by matching colName against the header row.
	headerRow := table.Content[0]
	if headerRow.Type != "tableRow" {
		return Node{}, fmt.Errorf("first row of table is not a tableRow")
	}
	colIdx := -1
	for j, cell := range headerRow.Content {
		if strings.Contains(cellText(cell), colName) {
			colIdx = j
			break
		}
	}
	if colIdx == -1 {
		var headers []string
		for _, cell := range headerRow.Content {
			headers = append(headers, strings.TrimSpace(cellText(cell)))
		}
		return Node{}, fmt.Errorf("column %q not found in table header (columns: %v)", colName, headers)
	}

	// Find target row (skip the header row).
	rowIdx := -1
	for i := 1; i < len(table.Content); i++ {
		row := table.Content[i]
		if row.Type != "tableRow" || len(row.Content) == 0 {
			continue
		}
		if strings.Contains(cellText(row.Content[0]), matchText) {
			rowIdx = i
			break
		}
	}
	if rowIdx == -1 {
		return Node{}, fmt.Errorf("no row with first cell containing %q found in table", matchText)
	}
	if colIdx >= len(table.Content[rowIdx].Content) {
		return Node{}, fmt.Errorf("row matching %q has only %d cells; column %q is at index %d",
			matchText, len(table.Content[rowIdx].Content), colName, colIdx)
	}

	// Build the new cell.
	var inline []Node
	if strings.HasPrefix(newValue, "`") && strings.HasSuffix(newValue, "`") && len(newValue) >= 2 {
		inline = []Node{Text(newValue[1:len(newValue)-1], Code())}
	} else {
		inline = []Node{Text(newValue)}
	}
	newCell := TableCell(Paragraph(inline...))

	// Rebuild rows, replacing only the targeted cell in the targeted row.
	newRows := make([]Node, len(table.Content))
	copy(newRows, table.Content)
	updatedRow := newRows[rowIdx]
	updatedCells := make([]Node, len(updatedRow.Content))
	copy(updatedCells, updatedRow.Content)
	updatedCells[colIdx] = newCell
	updatedRow.Content = updatedCells
	newRows[rowIdx] = updatedRow

	updatedTable := table
	updatedTable.Content = newRows
	newContent := make([]Node, len(doc.Content))
	copy(newContent, doc.Content)
	newContent[tableIdx] = updatedTable
	out := doc
	out.Content = newContent
	return out, nil
}

// TableRemoveRow removes the row whose first cell contains matchText from
// the first table inside the section with the given heading.
func TableRemoveRow(doc Node, headingText string, atLevel int, matchText string) (Node, error) {
	idx, end, ok := findSectionBoundsAtLevel(doc.Content, headingText, atLevel)
	if !ok {
		return Node{}, sectionNotFoundError(doc.Content, headingText)
	}

	// Find the first table in the section.
	tableIdx := -1
	for i := idx; i < end; i++ {
		if doc.Content[i].Type == "table" {
			tableIdx = i
			break
		}
	}
	if tableIdx == -1 {
		return Node{}, fmt.Errorf("no table found in section %q", headingText)
	}

	table := doc.Content[tableIdx]
	var newRows []Node
	removed := false
	for _, row := range table.Content {
		if row.Type == "tableRow" && len(row.Content) > 0 &&
			strings.Contains(cellText(row.Content[0]), matchText) && !removed {
			removed = true
			continue
		}
		newRows = append(newRows, row)
	}
	if !removed {
		return Node{}, fmt.Errorf("no row with first cell containing %q found in table", matchText)
	}

	updatedTable := table
	updatedTable.Content = newRows

	newContent := make([]Node, len(doc.Content))
	copy(newContent, doc.Content)
	newContent[tableIdx] = updatedTable

	out := doc
	out.Content = newContent
	return out, nil
}

// findSectionBoundsAtLevel is like findSectionBounds but optionally filters by
// exact heading level. atLevel=0 means first match wins (backward compat).
func findSectionBoundsAtLevel(nodes []Node, target string, atLevel int) (int, int, bool) {
	target = strings.TrimSpace(target)
	for i, n := range nodes {
		if n.Type != "heading" {
			continue
		}
		if strings.TrimSpace(headingText(n)) != target {
			continue
		}
		if atLevel > 0 && headingLevel(n) != atLevel {
			continue // skip headings at wrong level
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

// sectionNotFoundError builds an error listing all headings present.
// When target lacks `$` but a document heading contains `$`-prefixed
// patterns (likely shell variables), append a hint about probable
// bash variable expansion.
func sectionNotFoundError(nodes []Node, target string) error {
	var found []string
	var headings []string
	for _, n := range nodes {
		if n.Type == "heading" {
			txt := strings.TrimSpace(headingText(n))
			if txt != "" {
				found = append(found, fmt.Sprintf("  (h%d) %s", headingLevel(n), txt))
				headings = append(headings, txt)
			}
		}
	}
	hint := detectShellExpansionHint(target, headings)
	if len(found) == 0 {
		return fmt.Errorf("section not found: %q (document has no headings)%s", target, hint)
	}
	return fmt.Errorf("section not found: %q\nHeadings found in document:\n%s%s",
		target, strings.Join(found, "\n"), hint)
}

// shellDollarRe matches a `$` followed by a digit or letter/underscore — the
// telltale sign of a heading that bash would partially or fully expand if
// the user forgot to single-quote the section name.
var shellDollarRe = regexp.MustCompile(`\$[\w\d]`)

// detectShellExpansionHint returns a hint string when the target probably
// had bash variable expansion applied. Heuristic: target lacks `$`, but
// some heading in the document has `$<word|digit>` (the pattern bash chews
// on). We don't try to mimic bash's exact mangling — we just flag the
// probable cause. Cost of false positives is low (it's a hint, not a
// rejection) and the failure mode it diagnoses is silent and confusing,
// especially in a domain that talks about money (R$X, US$X) constantly.
func detectShellExpansionHint(target string, headings []string) string {
	if strings.Contains(target, "$") {
		return "" // user passed $; shell didn't strip it
	}
	for _, h := range headings {
		if !shellDollarRe.MatchString(h) {
			continue
		}
		// Loose match: target's prefix (up to the first $) should appear
		// in the heading. Avoids flagging completely unrelated typos.
		dollarIdx := strings.Index(h, "$")
		if dollarIdx <= 0 {
			continue
		}
		prefix := strings.TrimSpace(h[:dollarIdx])
		if prefix != "" && strings.Contains(target, prefix) {
			return fmt.Sprintf("\n\nHint: shell may have expanded variables in your section name.\n"+
				"  Received: %q\n"+
				"  Heading:  %q\n"+
				"  Wrap the section name in single quotes to prevent expansion, e.g. --replace-section '...'.", target, h)
		}
	}
	return ""
}

// parseRowCells splits a pipe-separated cell string, respecting \| escapes.
func parseRowCells(row string) ([]string, error) {
	var cells []string
	var cur strings.Builder
	runes := []rune(row)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) && runes[i+1] == '|' {
			cur.WriteRune('|')
			i++
			continue
		}
		if runes[i] == '|' {
			cells = append(cells, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(runes[i])
	}
	cells = append(cells, cur.String())
	return cells, nil
}

// buildTableRow creates a tableRow ADF node from cell strings.
// The first row is treated as a regular cell (not header), preserving order.
// Cell text supports a simple `code:value` prefix to wrap in code marks.
func buildTableRow(cells []string) Node {
	var adfCells []Node
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		var inline []Node
		if strings.HasPrefix(cell, "`") && strings.HasSuffix(cell, "`") && len(cell) >= 2 {
			// Inline code
			inner := cell[1 : len(cell)-1]
			inline = []Node{Text(inner, Code())}
		} else {
			inline = []Node{Text(cell)}
		}
		adfCells = append(adfCells, TableCell(Paragraph(inline...)))
	}
	return TableRow(adfCells...)
}

// cellText extracts the text content of a table cell node.
func cellText(cell Node) string {
	var sb strings.Builder
	collectText(cell, &sb)
	return sb.String()
}

// -- Exported wrappers that respect atLevel for section operations --

// ReplaceSectionAtLevel is like ReplaceSection but requires the heading to be
// at the specified level. atLevel=0 is identical to ReplaceSection (first match).
func ReplaceSectionAtLevel(doc Node, headingText string, atLevel int, fragment []Node) (Node, error) {
	idx, end, ok := findSectionBoundsAtLevel(doc.Content, headingText, atLevel)
	if !ok {
		return Node{}, sectionNotFoundError(doc.Content, headingText)
	}
	out := doc
	out.Content = spliceNodes(doc.Content, idx, end, fragment)
	return out, nil
}

// InsertAfterAtLevel is like InsertAfter with optional level filter.
func InsertAfterAtLevel(doc Node, headingText string, atLevel int, fragment []Node) (Node, error) {
	_, end, ok := findSectionBoundsAtLevel(doc.Content, headingText, atLevel)
	if !ok {
		return Node{}, sectionNotFoundError(doc.Content, headingText)
	}
	out := doc
	out.Content = spliceNodes(doc.Content, end, end, fragment)
	return out, nil
}

// InsertBeforeAtLevel is like InsertBefore with optional level filter.
func InsertBeforeAtLevel(doc Node, headingText string, atLevel int, fragment []Node) (Node, error) {
	idx, _, ok := findSectionBoundsAtLevel(doc.Content, headingText, atLevel)
	if !ok {
		return Node{}, sectionNotFoundError(doc.Content, headingText)
	}
	out := doc
	out.Content = spliceNodes(doc.Content, idx, idx, fragment)
	return out, nil
}

// DeleteSectionAtLevel is like DeleteSection with optional level filter.
func DeleteSectionAtLevel(doc Node, headingText string, atLevel int) (Node, error) {
	idx, end, ok := findSectionBoundsAtLevel(doc.Content, headingText, atLevel)
	if !ok {
		return Node{}, sectionNotFoundError(doc.Content, headingText)
	}
	out := doc
	out.Content = spliceNodes(doc.Content, idx, end, nil)
	return out, nil
}
