package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: run the CLI and capture stdout/stderr
func runCLI(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code, _ = run(args, bytes.NewReader(nil), &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

// helper: run with stdin
func runCLIStdin(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code, _ = run(args, strings.NewReader(stdin), &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

// ── version / help ──────────────────────────────────────────────────────────

func TestVersion(t *testing.T) {
	out, _, code := runCLI(t, "--version")
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(out, "lybel-docs") {
		t.Errorf("want 'lybel-docs' in output, got %q", out)
	}
}

func TestHelp(t *testing.T) {
	out, _, code := runCLI(t, "--help")
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(out, "COMMANDS:") {
		t.Errorf("want COMMANDS: in help, got %q", out)
	}
}

// ── adf ─────────────────────────────────────────────────────────────────────

func TestADF_Stdin(t *testing.T) {
	out, _, code := runCLIStdin(t, "# Hello\n\nworld", "adf")
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	if doc["type"] != "doc" {
		t.Errorf("want type=doc, got %v", doc["type"])
	}
}

func TestADF_File(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "page.md")
	os.WriteFile(f, []byte("# Title\n\nBody text."), 0644)

	out, _, code := runCLI(t, "adf", "--file", f)
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(out, `"heading"`) {
		t.Errorf("expected heading node in output: %s", out)
	}
}

// ── edit ─────────────────────────────────────────────────────────────────────

func writeTestADF(t *testing.T, dir string) string {
	t.Helper()
	// Build a simple doc with two sections
	doc := `{"type":"doc","attrs":{"version":1},"content":[` +
		`{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Alpha"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"body A"}]},` +
		`{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Bravo"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"body B"}]}` +
		`]}`
	path := filepath.Join(dir, "doc.json")
	os.WriteFile(path, []byte(doc), 0644)
	return path
}

func writeFragmentMD(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "frag.md")
	os.WriteFile(path, []byte(content), 0644)
	return path
}

func TestEdit_Append(t *testing.T) {
	dir := t.TempDir()
	docPath := writeTestADF(t, dir)
	fragPath := writeFragmentMD(t, dir, "## Charlie\n\nbody C")

	out, _, code := runCLI(t, "edit", "--input", docPath, "--append", fragPath)
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(out, "Charlie") {
		t.Errorf("expected Charlie in output: %s", out)
	}
}

func TestEdit_ReplaceSection_AtLevel(t *testing.T) {
	dir := t.TempDir()
	// Doc with h2 "Ops" and h3 "Ops"
	doc := `{"type":"doc","attrs":{"version":1},"content":[` +
		`{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Ops"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"h2 body"}]},` +
		`{"type":"heading","attrs":{"level":3},"content":[{"type":"text","text":"Ops"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"h3 body"}]}` +
		`]}`
	docPath := filepath.Join(dir, "doc.json")
	os.WriteFile(docPath, []byte(doc), 0644)
	fragPath := writeFragmentMD(t, dir, "### Ops v2\n\nnew h3")

	out, _, code := runCLI(t, "edit", "--input", docPath,
		"--replace-section", "Ops", "--at-level", "3", fragPath)
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	// h2 "Ops" and "h2 body" should still be present
	if !strings.Contains(out, "h2 body") {
		t.Errorf("h2 body missing from output: %s", out)
	}
	// h3 should be replaced — goldmark may emit "Ops" and " v2" as separate text nodes
	if !strings.Contains(out, "Ops") || !strings.Contains(out, "v2") {
		t.Errorf("Ops v2 not in output: %s", out)
	}
	// old h3 body should be gone
	if strings.Contains(out, "h3 body") {
		t.Errorf("old h3 body should be replaced: %s", out)
	}
}

func TestEdit_TableAddRow(t *testing.T) {
	dir := t.TempDir()
	// Doc with a section containing a table
	doc := `{"type":"doc","attrs":{"version":1},"content":[` +
		`{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Index"}]},` +
		`{"type":"table","attrs":{"isNumberColumnEnabled":false,"layout":"default"},"content":[` +
		`{"type":"tableRow","content":[` +
		`{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"Página"}]}]},` +
		`{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"pageId"}]}]}` +
		`]},` +
		`{"type":"tableRow","content":[` +
		`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"Home"}]}]},` +
		`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"164232"}]}]}` +
		`]}` +
		`]}` +
		`]}`
	docPath := filepath.Join(dir, "doc.json")
	os.WriteFile(docPath, []byte(doc), 0644)

	out, _, code := runCLI(t, "edit", "--input", docPath,
		"--table-add-row", "Index", "--row", "New Page|999")
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(out, "New Page") {
		t.Errorf("expected 'New Page' in output: %s", out)
	}
	if !strings.Contains(out, "999") {
		t.Errorf("expected '999' in output: %s", out)
	}
}

func TestEdit_TableAddRow_IfMissing(t *testing.T) {
	dir := t.TempDir()
	doc := `{"type":"doc","attrs":{"version":1},"content":[` +
		`{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Index"}]},` +
		`{"type":"table","attrs":{"isNumberColumnEnabled":false,"layout":"default"},"content":[` +
		`{"type":"tableRow","content":[` +
		`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"Home"}]}]},` +
		`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"164232"}]}]}` +
		`]}` +
		`]}` +
		`]}`
	docPath := filepath.Join(dir, "doc.json")
	os.WriteFile(docPath, []byte(doc), 0644)

	_, errOut, code := runCLI(t, "edit", "--input", docPath,
		"--table-add-row", "Index", "--row", "Home|999", "--if-missing")
	if code != 0 {
		t.Fatalf("want exit 0 even when skipped, got %d", code)
	}
	if !strings.Contains(errOut, "already exists") {
		t.Errorf("expected 'already exists' notice in stderr: %s", errOut)
	}
}

func TestEdit_TableRemoveRow(t *testing.T) {
	dir := t.TempDir()
	doc := `{"type":"doc","attrs":{"version":1},"content":[` +
		`{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Index"}]},` +
		`{"type":"table","attrs":{"isNumberColumnEnabled":false,"layout":"default"},"content":[` +
		`{"type":"tableRow","content":[` +
		`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"Home"}]}]},` +
		`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"164232"}]}]}` +
		`]},` +
		`{"type":"tableRow","content":[` +
		`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"Roadmap"}]}]},` +
		`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"222"}]}]}` +
		`]}` +
		`]}` +
		`]}`
	docPath := filepath.Join(dir, "doc.json")
	os.WriteFile(docPath, []byte(doc), 0644)

	out, _, code := runCLI(t, "edit", "--input", docPath,
		"--table-remove-row", "Index", "--match-cell", "Home")
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if strings.Contains(out, "Home") {
		t.Errorf("'Home' row should be removed: %s", out)
	}
	if !strings.Contains(out, "Roadmap") {
		t.Errorf("'Roadmap' row should remain: %s", out)
	}
}

// ── lint ─────────────────────────────────────────────────────────────────────

func TestLint_CleanFile(t *testing.T) {
	dir := t.TempDir()
	docPath := writeTestADF(t, dir)

	out, errOut, code := runCLI(t, "lint", docPath)
	if code != 0 {
		t.Fatalf("want exit 0 for clean doc, got %d\nstderr: %s", code, errOut)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("want 'ok' in output, got %q", out)
	}
}

func TestLint_BadFile(t *testing.T) {
	dir := t.TempDir()
	// Heading with no text = lint error
	doc := `{"type":"doc","attrs":{"version":1},"content":[` +
		`{"type":"heading","attrs":{"level":2},"content":[]}` +
		`]}`
	docPath := filepath.Join(dir, "bad.json")
	os.WriteFile(docPath, []byte(doc), 0644)

	_, errOut, code := runCLI(t, "lint", docPath)
	if code != exitParseErr {
		t.Fatalf("want exit %d for lint errors, got %d", exitParseErr, code)
	}
	if !strings.Contains(errOut, "heading has no text") {
		t.Errorf("expected heading-no-text error in stderr: %s", errOut)
	}
}

// ── extract-body ──────────────────────────────────────────────────────────────

func TestExtractBody_BarePageJSON(t *testing.T) {
	innerDoc := `{"type":"doc","attrs":{"version":1},"content":[]}`
	innerDocJSON, _ := json.Marshal(innerDoc)
	pageJSON := `{"id":"123","body":{"atlas_doc_format":{"value":` + string(innerDocJSON) + `,"representation":"atlas_doc_format"}}}`

	out, _, code := runCLIStdin(t, pageJSON, "extract-body")
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(out, `"type":"doc"`) {
		t.Errorf("expected doc type in output: %s", out)
	}
}

func TestExtractBody_MCPEnvelope(t *testing.T) {
	innerDoc := `{"type":"doc","attrs":{"version":1},"content":[]}`
	innerDocJSON, _ := json.Marshal(innerDoc)
	pageJSON := `{"id":"123","body":{"atlas_doc_format":{"value":` + string(innerDocJSON) + `,"representation":"atlas_doc_format"}}}`
	envelopeItems := []map[string]string{{"type": "text", "text": pageJSON}}
	envelopeBytes, _ := json.Marshal(envelopeItems)

	out, _, code := runCLIStdin(t, string(envelopeBytes), "extract-body")
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(out, `"type":"doc"`) {
		t.Errorf("expected doc type in output: %s", out)
	}
}

// ── unknown command ───────────────────────────────────────────────────────────

func TestUnknownCommand(t *testing.T) {
	_, _, code := runCLI(t, "bogus")
	if code != exitInputErr {
		t.Errorf("want exit %d for unknown command, got %d", exitInputErr, code)
	}
}

func TestNoArgs(t *testing.T) {
	_, _, code := runCLI(t)
	if code != exitInputErr {
		t.Errorf("want exit %d for no args, got %d", exitInputErr, code)
	}
}

// ── page subcommand validation ────────────────────────────────────────────────

func TestPageGet_MissingPageID(t *testing.T) {
	// Should fail with a clear message (no real HTTP call)
	_, errOut, code := runCLI(t, "page", "get")
	if code == 0 {
		t.Fatal("expected non-zero exit when --page-id missing")
	}
	if !strings.Contains(errOut, "--page-id") {
		t.Errorf("expected --page-id mention in error: %s", errOut)
	}
}

func TestPageGet_SectionRejectsHTMLFormats(t *testing.T) {
	_, errOut, code := runCLI(t, "page", "get",
		"--page-id", "123", "--section", "X", "--format", "html")
	if code == 0 {
		t.Fatal("expected non-zero exit when --section with --format html")
	}
	if !strings.Contains(errOut, "--section is only supported with") {
		t.Errorf("expected --section format constraint in error: %s", errOut)
	}
}

func TestPage_ChildrenIsAliasOfListChildren(t *testing.T) {
	// Both `page children` and `page list-children` should hit the same
	// validation path. Without --page-id, both must reject with the new
	// canonical error message.
	for _, verb := range []string{"children", "list-children"} {
		_, errOut, code := runCLI(t, "page", verb)
		if code == 0 {
			t.Errorf("verb %q: expected non-zero exit when --page-id missing", verb)
		}
		if !strings.Contains(errOut, "--page-id") {
			t.Errorf("verb %q: expected --page-id in error: %s", verb, errOut)
		}
	}
}

func TestPage_UnknownVerbListsChildren(t *testing.T) {
	// The "valid verbs" list should include `children` (canonical) — verifies
	// help message is in sync with the dispatcher.
	_, errOut, code := runCLI(t, "page", "bogus-verb")
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown verb")
	}
	if !strings.Contains(errOut, "children") {
		t.Errorf("expected 'children' in valid verbs list: %s", errOut)
	}
}

func TestPageGet_UnknownFormat(t *testing.T) {
	_, errOut, code := runCLI(t, "page", "get",
		"--page-id", "123", "--format", "bogus")
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown format")
	}
	if !strings.Contains(errOut, "unknown format") {
		t.Errorf("expected 'unknown format' in error: %s", errOut)
	}
}

func TestPageUpload_MissingArgs(t *testing.T) {
	_, errOut, code := runCLI(t, "page", "upload", "--page-id", "123")
	if code == 0 {
		t.Fatal("expected non-zero exit when --adf missing")
	}
	if !strings.Contains(errOut, "--adf") {
		t.Errorf("expected --adf mention in error: %s", errOut)
	}
}

func TestPageCreate_MissingTitle(t *testing.T) {
	_, errOut, code := runCLI(t, "page", "create", "--space-id", "123", "--parent-id", "456")
	if code == 0 {
		t.Fatal("expected non-zero exit when --title missing")
	}
	if !strings.Contains(errOut, "--title") {
		t.Errorf("expected --title mention in error: %s", errOut)
	}
}

// ── page digest / apply / search validation ────────────────────────────────────

func TestPageDigest_MissingPageID(t *testing.T) {
	_, errOut, code := runCLI(t, "page", "digest")
	if code == 0 {
		t.Fatal("expected non-zero exit when --page-id missing")
	}
	if !strings.Contains(errOut, "--page-id") {
		t.Errorf("expected --page-id in error: %s", errOut)
	}
}

func TestPageApply_NoOperation(t *testing.T) {
	_, errOut, code := runCLI(t, "page", "apply", "--page-id", "123")
	if code == 0 {
		t.Fatal("expected non-zero exit when no operation specified")
	}
	if !strings.Contains(errOut, "no operation") {
		t.Errorf("expected 'no operation' in error: %s", errOut)
	}
}

func TestPageApply_ReplaceSection_RequiresFragment(t *testing.T) {
	_, errOut, code := runCLI(t, "page", "apply", "--page-id", "123",
		"--replace-section", "Roadmap")
	if code == 0 {
		t.Fatal("expected non-zero exit when --fragment missing")
	}
	if !strings.Contains(errOut, "--fragment") {
		t.Errorf("expected --fragment in error: %s", errOut)
	}
}

func TestPageApply_DeleteSection_NoFragmentOK(t *testing.T) {
	// Delete-section is the only op that doesn't need a fragment. The flag
	// validation should pass; the command will then fail on missing creds,
	// which is fine — we only care that the validation gate let it through.
	_, _, code := runCLI(t, "page", "apply", "--page-id", "123",
		"--delete-section", "Old")
	// Will fail on credentials (no real call), but NOT on flag validation.
	// exitInputErr (2) == flag-validation failure; anything else means we got past it.
	if code == exitInputErr {
		t.Errorf("delete-section should not require --fragment, got exit %d", code)
	}
}

func TestSearch_NoArgs(t *testing.T) {
	_, errOut, code := runCLI(t, "search")
	if code == 0 {
		t.Fatal("expected non-zero exit when no query")
	}
	if !strings.Contains(errOut, "query") {
		t.Errorf("expected 'query' in error: %s", errOut)
	}
}

func TestPageApply_TableAddRow_RequiresRow(t *testing.T) {
	_, errOut, code := runCLI(t, "page", "apply", "--page-id", "123",
		"--table-add-row", "Index")
	if code == 0 {
		t.Fatal("expected non-zero exit when --row missing")
	}
	if !strings.Contains(errOut, "--row") {
		t.Errorf("expected --row in error: %s", errOut)
	}
}

func TestPageApply_TableRemoveRow_RequiresMatchCell(t *testing.T) {
	_, errOut, code := runCLI(t, "page", "apply", "--page-id", "123",
		"--table-remove-row", "Index")
	if code == 0 {
		t.Fatal("expected non-zero exit when --match-cell missing")
	}
	if !strings.Contains(errOut, "--match-cell") {
		t.Errorf("expected --match-cell in error: %s", errOut)
	}
}

func TestHome_HelpFlag(t *testing.T) {
	out, _, code := runCLI(t, "home", "--help")
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(out, "--refresh") {
		t.Errorf("expected --refresh in help: %s", out)
	}
}

func TestHome_StatusNoCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("HOME", tmp)

	_, errOut, code := runCLI(t, "home", "--status")
	if code == 0 {
		t.Fatal("expected non-zero exit when cache missing")
	}
	if !strings.Contains(errOut, "no home cache") {
		t.Errorf("expected 'no home cache' message: %s", errOut)
	}
}

func TestHome_QueryUnknownFlag(t *testing.T) {
	_, errOut, code := runCLI(t, "home", "--bogus")
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown flag")
	}
	if !strings.Contains(errOut, "unknown flag") {
		t.Errorf("expected 'unknown flag': %s", errOut)
	}
}

// ── index subcommand validation ────────────────────────────────────────────────

func TestIndexAdd_MissingArgs(t *testing.T) {
	_, errOut, code := runCLI(t, "index", "add", "--title", "Test")
	if code == 0 {
		t.Fatal("expected non-zero exit when required flags missing")
	}
	_ = errOut // just check it doesn't panic
}

func TestIndexRemove_MissingPageID(t *testing.T) {
	_, errOut, code := runCLI(t, "index", "remove")
	if code == 0 {
		t.Fatal("expected non-zero exit when --page-id missing")
	}
	if !strings.Contains(errOut, "--page-id") {
		t.Errorf("expected --page-id mention in error: %s", errOut)
	}
}
