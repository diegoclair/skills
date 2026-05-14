// cmd_index_test.go — unit tests for `confluence-docs index` subcommands.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diegoclair/skills/pkg/atlassian/adf"
	"github.com/diegoclair/skills/pkg/atlassian/setup"
)

// ── ADF fixture helpers ───────────────────────────────────────────────────────

// indexDocWithTable builds an ADF doc JSON with a heading and table that
// contains an optional set of rows. Each row is [title, pageID].
func indexDocWithTable(heading string, rows [][2]string) string {
	var rowsJSON strings.Builder
	for i, r := range rows {
		if i > 0 {
			rowsJSON.WriteString(",")
		}
		rowsJSON.WriteString(`{"type":"tableRow","content":[`)
		rowsJSON.WriteString(`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"` + r[0] + `"}]}]},`)
		rowsJSON.WriteString(`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"` + r[1] + `"}]}]}`)
		rowsJSON.WriteString(`]}`)
	}
	return `{"type":"doc","attrs":{"version":1},"content":[` +
		`{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"` + heading + `"}]},` +
		`{"type":"table","attrs":{"isNumberColumnEnabled":false,"layout":"default"},"content":[` +
		rowsJSON.String() +
		`]}` +
		`]}`
}

// docWithNoTable builds an ADF doc with a heading but no table.
func docWithNoTable() string {
	return `{"type":"doc","attrs":{"version":1},"content":[` +
		`{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Index"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"no table here"}]}` +
		`]}`
}

// writeADFFile writes ADF JSON to a temp file and returns its path.
func writeADFFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "doc.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// runIndex is a thin wrapper to exercise the `index` dispatcher.
func runIndexCmd(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code, _ = run(append([]string{"index"}, args...), strings.NewReader(""), &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

// ── removeIndexRow ────────────────────────────────────────────────────────────

func TestRemoveIndexRow_NoTable(t *testing.T) {
	// Doc with no table — removeIndexRow should return error.
	doc := adf.Doc(
		adf.Heading(2, adf.Text("Index")),
		adf.Paragraph(adf.Text("no table")),
	)
	_, err := removeIndexRow(doc, "12345")
	if err == nil {
		t.Fatal("expected error when no table in doc, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got %q", err.Error())
	}
}

func TestRemoveIndexRow_NoMatchingPageID(t *testing.T) {
	// Doc with a table but pageID not present.
	row1 := adf.TableRow(
		adf.TableCell(adf.Paragraph(adf.Text("Page One"))),
		adf.TableCell(adf.Paragraph(adf.Text("11111"))),
	)
	doc := adf.Doc(
		adf.Heading(2, adf.Text("Index")),
		adf.Table(row1),
	)
	_, err := removeIndexRow(doc, "99999")
	if err == nil {
		t.Fatal("expected error when pageID not found")
	}
	if !strings.Contains(err.Error(), "99999") {
		t.Errorf("expected pageID in error message, got %q", err.Error())
	}
}

func TestRemoveIndexRow_SingleMatchingRow(t *testing.T) {
	row1 := adf.TableRow(
		adf.TableCell(adf.Paragraph(adf.Text("Page One"))),
		adf.TableCell(adf.Paragraph(adf.Text("11111"))),
	)
	row2 := adf.TableRow(
		adf.TableCell(adf.Paragraph(adf.Text("Page Two"))),
		adf.TableCell(adf.Paragraph(adf.Text("22222"))),
	)
	doc := adf.Doc(
		adf.Heading(2, adf.Text("Index")),
		adf.Table(row1, row2),
	)

	updated, err := removeIndexRow(doc, "11111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the table in updated doc and verify row count.
	var table *adf.Node
	for i := range updated.Content {
		if updated.Content[i].Type == "table" {
			table = &updated.Content[i]
			break
		}
	}
	if table == nil {
		t.Fatal("table missing from updated doc")
	}
	if len(table.Content) != 1 {
		t.Errorf("expected 1 row remaining, got %d", len(table.Content))
	}
	// The remaining row should be row2.
	if nodeText(table.Content[0]) != "Page Two22222" {
		t.Errorf("unexpected remaining row text: %q", nodeText(table.Content[0]))
	}
}

func TestRemoveIndexRow_MultipleRowsOneMatches(t *testing.T) {
	rows := []adf.Node{
		adf.TableRow(
			adf.TableCell(adf.Paragraph(adf.Text("Alpha"))),
			adf.TableCell(adf.Paragraph(adf.Text("1001"))),
		),
		adf.TableRow(
			adf.TableCell(adf.Paragraph(adf.Text("Beta"))),
			adf.TableCell(adf.Paragraph(adf.Text("1002"))),
		),
		adf.TableRow(
			adf.TableCell(adf.Paragraph(adf.Text("Gamma"))),
			adf.TableCell(adf.Paragraph(adf.Text("1003"))),
		),
	}
	doc := adf.Doc(adf.Heading(2, adf.Text("Index")), adf.Table(rows...))

	updated, err := removeIndexRow(doc, "1002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var table *adf.Node
	for i := range updated.Content {
		if updated.Content[i].Type == "table" {
			table = &updated.Content[i]
			break
		}
	}
	if table == nil {
		t.Fatal("table missing")
	}
	if len(table.Content) != 2 {
		t.Errorf("expected 2 rows remaining, got %d", len(table.Content))
	}
	combined := nodeText(table.Content[0]) + nodeText(table.Content[1])
	if strings.Contains(combined, "1002") {
		t.Error("removed row should not appear in updated doc")
	}
	if !strings.Contains(combined, "1001") || !strings.Contains(combined, "1003") {
		t.Errorf("remaining rows missing: %q", combined)
	}
}

func TestRemoveIndexRow_TableAfterHeading(t *testing.T) {
	// Heading, paragraph, then table — tests that scan skips non-table nodes.
	row := adf.TableRow(
		adf.TableCell(adf.Paragraph(adf.Text("Target"))),
		adf.TableCell(adf.Paragraph(adf.Text("555"))),
	)
	doc := adf.Doc(
		adf.Heading(2, adf.Text("Index")),
		adf.Paragraph(adf.Text("Some text before table")),
		adf.Table(row),
	)

	updated, err := removeIndexRow(doc, "555")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var table *adf.Node
	for i := range updated.Content {
		if updated.Content[i].Type == "table" {
			table = &updated.Content[i]
			break
		}
	}
	if table == nil {
		t.Fatal("table missing")
	}
	if len(table.Content) != 0 {
		t.Errorf("expected 0 rows after remove, got %d", len(table.Content))
	}
}

func TestRemoveIndexRow_MultipleTablesRemovesFromFirst(t *testing.T) {
	// Two tables: pageID only in second. removeIndexRow finds first table
	// containing the id, which here is the second table (first has no match).
	row1a := adf.TableRow(
		adf.TableCell(adf.Paragraph(adf.Text("A"))),
		adf.TableCell(adf.Paragraph(adf.Text("aaa"))),
	)
	row2a := adf.TableRow(
		adf.TableCell(adf.Paragraph(adf.Text("B"))),
		adf.TableCell(adf.Paragraph(adf.Text("bbb"))),
	)
	doc := adf.Doc(
		adf.Table(row1a),
		adf.Table(row2a),
	)

	updated, err := removeIndexRow(doc, "bbb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The first table should be untouched; the second should have 0 rows.
	tables := 0
	for _, n := range updated.Content {
		if n.Type == "table" {
			tables++
		}
	}
	if tables != 2 {
		t.Errorf("expected 2 tables, got %d", tables)
	}
}

// ── nodeText / collectNodeText ────────────────────────────────────────────────

func TestNodeText_PlainText(t *testing.T) {
	n := adf.Text("hello world")
	got := nodeText(n)
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestNodeText_TextWithMarks(t *testing.T) {
	// Marks don't affect Text extraction — only the Text field matters.
	n := adf.Text("bold text", adf.Bold())
	if got := nodeText(n); got != "bold text" {
		t.Errorf("got %q, want %q", got, "bold text")
	}
}

func TestNodeText_NestedInlineParagraph(t *testing.T) {
	p := adf.Paragraph(adf.Text("foo"), adf.Text("bar"))
	got := nodeText(p)
	if got != "foobar" {
		t.Errorf("got %q, want %q", got, "foobar")
	}
}

func TestNodeText_EmptyNode(t *testing.T) {
	n := adf.Node{Type: "paragraph"}
	got := nodeText(n)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestNodeText_DeepNesting(t *testing.T) {
	inner := adf.TableCell(adf.Paragraph(adf.Text("deep")))
	got := nodeText(inner)
	if got != "deep" {
		t.Errorf("got %q, want %q", got, "deep")
	}
}

func TestNodeText_TableRow(t *testing.T) {
	row := adf.TableRow(
		adf.TableCell(adf.Paragraph(adf.Text("col1"))),
		adf.TableCell(adf.Paragraph(adf.Text("col2"))),
	)
	got := nodeText(row)
	if got != "col1col2" {
		t.Errorf("got %q, want %q", got, "col1col2")
	}
}

// ── currentHomePageID / currentSpaceID / currentSpaceKey ─────────────────────

func TestCurrentHomePageID_FromConfig(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	writeTestConfig(t, dir, setup.Config{
		Cloud:      "mycloud",
		HomePageID: "98765",
	})

	got, err := currentHomePageID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "98765" {
		t.Errorf("got %q, want %q", got, "98765")
	}
}

func TestCurrentHomePageID_Missing(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	// Config with no HomePageID.
	writeTestConfig(t, dir, setup.Config{
		Cloud:    "mycloud",
		SpaceKey: "ENG",
	})

	_, err := currentHomePageID()
	if err == nil {
		t.Fatal("expected error when HomePageID not set")
	}
	if err != errConfigNotSet {
		t.Errorf("expected errConfigNotSet, got %v", err)
	}
}

func TestCurrentHomePageID_NoConfig(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	// No config file at all.

	_, err := currentHomePageID()
	if err == nil {
		t.Fatal("expected error with missing config")
	}
}

func TestCurrentSpaceID_FromConfig(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	writeTestConfig(t, dir, setup.Config{
		Cloud:   "mycloud",
		SpaceID: "SP-001",
	})

	got, err := currentSpaceID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "SP-001" {
		t.Errorf("got %q, want %q", got, "SP-001")
	}
}

func TestCurrentSpaceID_Missing(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	writeTestConfig(t, dir, setup.Config{Cloud: "mycloud"})

	_, err := currentSpaceID()
	if err != errConfigNotSet {
		t.Errorf("expected errConfigNotSet, got %v", err)
	}
}

func TestCurrentSpaceKey_FromConfig(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	writeTestConfig(t, dir, setup.Config{
		Cloud:    "mycloud",
		SpaceKey: "ENG",
	})

	got, err := currentSpaceKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ENG" {
		t.Errorf("got %q, want %q", got, "ENG")
	}
}

func TestCurrentSpaceKey_Missing(t *testing.T) {
	dir := t.TempDir()
	overrideConfigDirMain(t, dir)
	writeTestConfig(t, dir, setup.Config{Cloud: "mycloud", SpaceID: "99"})

	_, err := currentSpaceKey()
	if err != errConfigNotSet {
		t.Errorf("expected errConfigNotSet, got %v", err)
	}
}

// ── runIndex dispatcher ───────────────────────────────────────────────────────

func TestRunIndex_NoVerb(t *testing.T) {
	_, errOut, code := runIndexCmd(t)
	if code == exitOK {
		t.Fatal("expected non-zero exit when no verb given")
	}
	if !strings.Contains(errOut, "requires a verb") {
		t.Errorf("expected 'requires a verb' message, got %q", errOut)
	}
}

func TestRunIndex_UnknownVerb(t *testing.T) {
	_, errOut, code := runIndexCmd(t, "bogus")
	if code == exitOK {
		t.Fatal("expected non-zero exit for unknown verb")
	}
	if !strings.Contains(errOut, "unknown verb") {
		t.Errorf("expected 'unknown verb' message, got %q", errOut)
	}
	if !strings.Contains(errOut, "bogus") {
		t.Errorf("expected verb name in error, got %q", errOut)
	}
}

func TestRunIndex_ValidVerbsListed(t *testing.T) {
	_, errOut, _ := runIndexCmd(t, "nope")
	for _, verb := range []string{"add", "remove", "sync"} {
		if !strings.Contains(errOut, verb) {
			t.Errorf("expected verb %q listed in error: %s", verb, errOut)
		}
	}
}

// ── runIndexAdd flag validation ───────────────────────────────────────────────

func TestRunIndexAdd_MissingPageID(t *testing.T) {
	_, errOut, code := runIndexCmd(t, "add", "--title", "MyPage", "--under", "Index")
	if code == exitOK {
		t.Fatal("expected non-zero exit when --page-id missing")
	}
	if !strings.Contains(errOut, "--page-id") {
		t.Errorf("expected '--page-id' in error: %s", errOut)
	}
}

func TestRunIndexAdd_MissingTitle(t *testing.T) {
	_, errOut, code := runIndexCmd(t, "add", "--page-id", "123", "--under", "Index")
	if code == exitOK {
		t.Fatal("expected non-zero exit when --title missing")
	}
	if !strings.Contains(errOut, "--title") {
		t.Errorf("expected '--title' in error: %s", errOut)
	}
}

func TestRunIndexAdd_MissingUnder(t *testing.T) {
	_, errOut, code := runIndexCmd(t, "add", "--page-id", "123", "--title", "MyPage")
	if code == exitOK {
		t.Fatal("expected non-zero exit when --under missing")
	}
	if !strings.Contains(errOut, "--under") {
		t.Errorf("expected '--under' in error: %s", errOut)
	}
}

func TestRunIndexAdd_InvalidIndent(t *testing.T) {
	_, errOut, code := runIndexCmd(t, "add",
		"--page-id", "123", "--title", "MyPage", "--under", "Index",
		"--indent", "9",
	)
	if code == exitOK {
		t.Fatal("expected non-zero exit for invalid --indent")
	}
	if !strings.Contains(errOut, "--indent") {
		t.Errorf("expected '--indent' in error: %s", errOut)
	}
}

func TestRunIndexAdd_UnknownFlag(t *testing.T) {
	_, errOut, code := runIndexCmd(t, "add", "--bogus-flag")
	if code == exitOK {
		t.Fatal("expected non-zero exit for unknown flag")
	}
	if !strings.Contains(errOut, "unknown flag") {
		t.Errorf("expected 'unknown flag' in error: %s", errOut)
	}
}

// ── runIndexAdd with --input file (no HTTP) ───────────────────────────────────

func TestRunIndexAdd_InputFile_AddsRow(t *testing.T) {
	dir := t.TempDir()
	docJSON := indexDocWithTable("Index", [][2]string{
		{"Home", "164232"},
	})
	docPath := writeADFFile(t, dir, docJSON)

	out, errOut, code := runIndexCmd(t, "add",
		"--input", docPath,
		"--page-id", "99001",
		"--title", "Roadmap",
		"--under", "Index",
		"--dry-run",
	)
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nstderr: %s", code, errOut)
	}
	// Dry-run on input-file mode writes to stderr notice, does NOT modify file.
	if !strings.Contains(errOut, "dry-run") {
		t.Errorf("expected dry-run notice in stderr: %s", errOut)
	}
	// stdout should confirm
	var result map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", out, err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", result["status"])
	}
	if result["pageId"] != "99001" {
		t.Errorf("expected pageId=99001, got %q", result["pageId"])
	}
}

func TestRunIndexAdd_InputFile_WritesFile(t *testing.T) {
	dir := t.TempDir()
	docJSON := indexDocWithTable("Specs", [][2]string{
		{"Existing", "100"},
	})
	docPath := writeADFFile(t, dir, docJSON)

	out, errOut, code := runIndexCmd(t, "add",
		"--input", docPath,
		"--page-id", "200",
		"--title", "NewSpec",
		"--under", "Specs",
	)
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nstderr: %s", code, errOut)
	}

	// Verify the file was updated.
	updated, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "200") {
		t.Errorf("expected new pageID 200 in updated file")
	}
	if !strings.Contains(string(updated), "NewSpec") {
		t.Errorf("expected new title in updated file")
	}
	_ = out
}

func TestRunIndexAdd_InputFile_IndentLevel1(t *testing.T) {
	dir := t.TempDir()
	docJSON := indexDocWithTable("Index", [][2]string{})
	docPath := writeADFFile(t, dir, docJSON)

	out, errOut, code := runIndexCmd(t, "add",
		"--input", docPath,
		"--page-id", "555",
		"--title", "SubPage",
		"--under", "Index",
		"--indent", "1",
	)
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nstderr: %s", code, errOut)
	}

	// File should contain the indent prefix.
	updated, _ := os.ReadFile(docPath)
	if !strings.Contains(string(updated), indentLevel1+"SubPage") {
		t.Errorf("expected indent prefix %q in file: %s", indentLevel1, string(updated))
	}
	_ = out
}

func TestRunIndexAdd_InputFile_IfMissing_Skips(t *testing.T) {
	dir := t.TempDir()
	// Page "Home" is already in the table.
	docJSON := indexDocWithTable("Index", [][2]string{
		{"Home", "164232"},
	})
	docPath := writeADFFile(t, dir, docJSON)

	out, errOut, code := runIndexCmd(t, "add",
		"--input", docPath,
		"--page-id", "164232",
		"--title", "Home",
		"--under", "Index",
		"--if-missing",
	)
	if code != exitOK {
		t.Fatalf("want exit 0 even when skipped, got %d", code)
	}
	if !strings.Contains(errOut, "skipped") || !strings.Contains(errOut, "already") {
		t.Errorf("expected 'already'/'skipped' notice in stderr: %s", errOut)
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", out, err)
	}
	if result["status"] != "skipped" {
		t.Errorf("expected status=skipped, got %q", result["status"])
	}
}

func TestRunIndexAdd_InputFile_SectionNotFound(t *testing.T) {
	dir := t.TempDir()
	docJSON := indexDocWithTable("Index", [][2]string{})
	docPath := writeADFFile(t, dir, docJSON)

	_, errOut, code := runIndexCmd(t, "add",
		"--input", docPath,
		"--page-id", "999",
		"--title", "Page",
		"--under", "NonExistentSection",
	)
	if code == exitOK {
		t.Fatal("expected non-zero exit when section not found")
	}
	_ = errOut // just verify no panic
}

// ── runIndexRemove flag validation ────────────────────────────────────────────

func TestRunIndexRemove_MissingPageID(t *testing.T) {
	_, errOut, code := runIndexCmd(t, "remove")
	if code == exitOK {
		t.Fatal("expected non-zero exit when --page-id missing")
	}
	if !strings.Contains(errOut, "--page-id") {
		t.Errorf("expected '--page-id' in error: %s", errOut)
	}
}

func TestRunIndexRemove_UnknownFlag(t *testing.T) {
	_, errOut, code := runIndexCmd(t, "remove", "--unknown")
	if code == exitOK {
		t.Fatal("expected non-zero exit for unknown flag")
	}
	if !strings.Contains(errOut, "unknown flag") {
		t.Errorf("expected 'unknown flag' in error: %s", errOut)
	}
}

// ── runIndexRemove with --input file (no HTTP) ────────────────────────────────

func TestRunIndexRemove_InputFile_RemovesRow(t *testing.T) {
	dir := t.TempDir()
	docJSON := indexDocWithTable("Index", [][2]string{
		{"Home", "164232"},
		{"Roadmap", "999"},
	})
	docPath := writeADFFile(t, dir, docJSON)

	out, errOut, code := runIndexCmd(t, "remove",
		"--input", docPath,
		"--page-id", "164232",
	)
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nstderr: %s", code, errOut)
	}

	// Verify row removed from file.
	updated, _ := os.ReadFile(docPath)
	if strings.Contains(string(updated), "164232") {
		t.Errorf("expected 164232 to be removed from file")
	}
	if !strings.Contains(string(updated), "999") {
		t.Errorf("expected 999 (Roadmap) to remain in file")
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("expected JSON output: %v, got %q", err, out)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", result["status"])
	}
	if result["removed"] != "164232" {
		t.Errorf("expected removed=164232, got %q", result["removed"])
	}
}

func TestRunIndexRemove_InputFile_DryRun(t *testing.T) {
	dir := t.TempDir()
	docJSON := indexDocWithTable("Index", [][2]string{
		{"Home", "164232"},
	})
	docPath := writeADFFile(t, dir, docJSON)

	origContent, _ := os.ReadFile(docPath)

	_, errOut, code := runIndexCmd(t, "remove",
		"--input", docPath,
		"--page-id", "164232",
		"--dry-run",
	)
	if code != exitOK {
		t.Fatalf("want exit 0, got %d\nstderr: %s", code, errOut)
	}

	// dry-run on input-file should write the updated ADF back (no --dry-run skip
	// for input-file mode; dryRun only skips API upload). Verify at least no crash.
	_ = origContent
}

func TestRunIndexRemove_InputFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	docJSON := indexDocWithTable("Index", [][2]string{
		{"Home", "164232"},
	})
	docPath := writeADFFile(t, dir, docJSON)

	_, errOut, code := runIndexCmd(t, "remove",
		"--input", docPath,
		"--page-id", "nonexistent-id",
	)
	if code == exitOK {
		t.Fatal("expected non-zero exit when pageID not found")
	}
	if !strings.Contains(errOut, "nonexistent-id") {
		t.Errorf("expected pageID in error: %s", errOut)
	}
}

// ── runIndexSync flag validation ──────────────────────────────────────────────

func TestRunIndexSync_MissingParentPageID(t *testing.T) {
	_, errOut, code := runIndexCmd(t, "sync", "--under", "Index")
	if code == exitOK {
		t.Fatal("expected non-zero exit when --parent-page-id missing")
	}
	if !strings.Contains(errOut, "--parent-page-id") {
		t.Errorf("expected '--parent-page-id' in error: %s", errOut)
	}
}

func TestRunIndexSync_MissingUnder(t *testing.T) {
	_, errOut, code := runIndexCmd(t, "sync", "--parent-page-id", "123")
	if code == exitOK {
		t.Fatal("expected non-zero exit when --under missing")
	}
	if !strings.Contains(errOut, "--under") {
		t.Errorf("expected '--under' in error: %s", errOut)
	}
}

func TestRunIndexSync_UnknownFlag(t *testing.T) {
	_, errOut, code := runIndexCmd(t, "sync", "--random-flag")
	if code == exitOK {
		t.Fatal("expected non-zero exit for unknown flag")
	}
	if !strings.Contains(errOut, "unknown flag") {
		t.Errorf("expected 'unknown flag' in error: %s", errOut)
	}
}

// ── loadIndexPage with --input file ──────────────────────────────────────────

func TestLoadIndexPage_InputFile_ValidADF(t *testing.T) {
	dir := t.TempDir()
	docJSON := indexDocWithTable("Index", [][2]string{
		{"Home", "123"},
	})
	docPath := writeADFFile(t, dir, docJSON)

	ctx, err := loadIndexPage(docPath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.inputFile != docPath {
		t.Errorf("expected inputFile=%q, got %q", docPath, ctx.inputFile)
	}
	if ctx.pageID != "" {
		t.Errorf("expected empty pageID for file-based load, got %q", ctx.pageID)
	}
	if ctx.doc.Type != "doc" {
		t.Errorf("expected doc type, got %q", ctx.doc.Type)
	}
}

func TestLoadIndexPage_InputFile_Missing(t *testing.T) {
	_, err := loadIndexPage("/nonexistent/path/doc.json", nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("expected 'reading' context in error: %v", err)
	}
}

func TestLoadIndexPage_InputFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("not valid json {{{"), 0644)

	_, err := loadIndexPage(bad, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid ADF") {
		t.Errorf("expected 'invalid ADF' context in error: %v", err)
	}
}

// ── saveIndexPage with --input file ──────────────────────────────────────────

func TestSaveIndexPage_InputFile_DryRun_WritesNothing(t *testing.T) {
	dir := t.TempDir()
	docJSON := indexDocWithTable("Index", [][2]string{})
	docPath := writeADFFile(t, dir, docJSON)

	ctx := &indexPageContext{
		doc:       adf.Doc(),
		inputFile: docPath,
		pageID:    "",
	}
	newDoc := adf.Doc(adf.Heading(2, adf.Text("Updated")))

	var errBuf bytes.Buffer
	err := saveIndexPage(ctx, newDoc, nil, "test", true, &errBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(errBuf.String(), "dry-run") {
		t.Errorf("expected dry-run notice in stderr: %s", errBuf.String())
	}

	// File should be unchanged.
	content, _ := os.ReadFile(docPath)
	if strings.Contains(string(content), "Updated") {
		t.Error("dry-run should not modify file, but it did")
	}
}

func TestSaveIndexPage_InputFile_WritesUpdatedDoc(t *testing.T) {
	dir := t.TempDir()
	docJSON := indexDocWithTable("Index", [][2]string{})
	docPath := writeADFFile(t, dir, docJSON)

	ctx := &indexPageContext{
		doc:       adf.Doc(),
		inputFile: docPath,
		pageID:    "",
	}
	newDoc := adf.Doc(adf.Heading(2, adf.Text("NewSection")))

	var errBuf bytes.Buffer
	err := saveIndexPage(ctx, newDoc, nil, "test", false, &errBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(docPath)
	if !strings.Contains(string(content), "NewSection") {
		t.Errorf("expected updated content in file, got: %s", content)
	}
}

// ── errConfigNotSet sentinel ──────────────────────────────────────────────────

func TestErrConfigNotSet_IsDistinct(t *testing.T) {
	if errConfigNotSet == nil {
		t.Fatal("errConfigNotSet must not be nil")
	}
	if !strings.Contains(errConfigNotSet.Error(), "no active space") {
		t.Errorf("expected 'no active space' in error message: %v", errConfigNotSet)
	}
}
