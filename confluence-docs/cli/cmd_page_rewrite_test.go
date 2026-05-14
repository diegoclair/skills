package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── countReports ──────────────────────────────────────────────────────────────

func TestCountReports_EmptyList(t *testing.T) {
	if got := countReports(nil, "✓"); got != 0 {
		t.Errorf("empty list: want 0, got %d", got)
	}
}

func TestCountReports_MultipleMarkers(t *testing.T) {
	rs := []rewriteReportLine{
		{"✓", "replaced section \"Intro\" (h2)"},
		{"✓", "replaced section \"Body\" (h2)"},
		{"⚠", "would add: \"Extra\" (h2) [pass --allow-add to apply]"},
		{"+", "added section \"New\" (h2) at end"},
		{"-", "removed section \"Old\" (h2)"},
	}

	cases := []struct {
		marker string
		want   int
	}{
		{"✓", 2},
		{"⚠", 1},
		{"+", 1},
		{"-", 1},
		// markers not present in any line
		{"✗", 0},
		{"", 0},
	}

	for _, c := range cases {
		t.Run("marker="+c.marker, func(t *testing.T) {
			got := countReports(rs, c.marker)
			if got != c.want {
				t.Errorf("countReports(rs, %q) = %d, want %d", c.marker, got, c.want)
			}
		})
	}
}

func TestCountReports_NoMatch(t *testing.T) {
	rs := []rewriteReportLine{
		{"⚠", "would add section"},
	}
	if got := countReports(rs, "✓"); got != 0 {
		t.Errorf("no-match case: want 0, got %d", got)
	}
}

func TestCountReports_AllSameMarker(t *testing.T) {
	rs := []rewriteReportLine{
		{"✓", "one"},
		{"✓", "two"},
		{"✓", "three"},
	}
	if got := countReports(rs, "✓"); got != 3 {
		t.Errorf("all-same marker: want 3, got %d", got)
	}
}

// ── bundleOutermostSections (new, non-duplicate cases) ────────────────────────

func makeSec(level int, title, body string) mdSection {
	hashes := strings.Repeat("#", level)
	return mdSection{
		level:    level,
		title:    title,
		headLine: hashes + " " + title,
		body:     body,
	}
}

func TestBundleOutermostSections_SingleSection(t *testing.T) {
	sections := []mdSection{
		makeSec(2, "Intro", "Some text.\n"),
	}
	got := bundleOutermostSections(sections)
	if len(got) != 1 {
		t.Fatalf("single section: want 1 bundle, got %d", len(got))
	}
	if got[0].level != 2 {
		t.Errorf("bundle level = %d, want 2", got[0].level)
	}
	if got[0].title != "Intro" {
		t.Errorf("bundle title = %q, want 'Intro'", got[0].title)
	}
	if !strings.Contains(got[0].body, "## Intro") {
		t.Errorf("body should contain heading line, got: %q", got[0].body)
	}
	if !strings.Contains(got[0].body, "Some text.") {
		t.Errorf("body should contain section body, got: %q", got[0].body)
	}
}

func TestBundleOutermostSections_NestedH2UnderH1(t *testing.T) {
	// h1 is outermost; h2 children are absorbed into it.
	sections := []mdSection{
		makeSec(1, "Chapter", "Intro paragraph.\n"),
		makeSec(2, "Section A", "Content A.\n"),
		makeSec(2, "Section B", "Content B.\n"),
	}
	got := bundleOutermostSections(sections)
	if len(got) != 1 {
		t.Fatalf("want 1 bundle (h1 absorbs h2s), got %d: %+v", len(got), got)
	}
	b := got[0]
	if b.level != 1 {
		t.Errorf("bundle level = %d, want 1", b.level)
	}
	if b.title != "Chapter" {
		t.Errorf("bundle title = %q, want 'Chapter'", b.title)
	}
	if !strings.Contains(b.body, "## Section A") {
		t.Errorf("body should contain '## Section A', got: %q", b.body)
	}
	if !strings.Contains(b.body, "## Section B") {
		t.Errorf("body should contain '## Section B', got: %q", b.body)
	}
}

func TestBundleOutermostSections_MultipleH1s(t *testing.T) {
	// Three independent h1 sections with no children.
	sections := []mdSection{
		makeSec(1, "Alpha", "a.\n"),
		makeSec(1, "Beta", "b.\n"),
		makeSec(1, "Gamma", "c.\n"),
	}
	got := bundleOutermostSections(sections)
	if len(got) != 3 {
		t.Fatalf("want 3 bundles, got %d", len(got))
	}
	for i, want := range []string{"Alpha", "Beta", "Gamma"} {
		if got[i].title != want {
			t.Errorf("bundle[%d].title = %q, want %q", i, got[i].title, want)
		}
	}
}

func TestBundleOutermostSections_MixedLevels_H1_H3_H2(t *testing.T) {
	// h1 "A" is the outermost root; h3 and h2 that follow (before next h1)
	// are deeper than h1, so they get absorbed into A's bundle.
	// h1 "B" is a second independent root.
	sections := []mdSection{
		makeSec(1, "A", "body a.\n"),
		makeSec(3, "A.1", "body a1.\n"),
		makeSec(2, "A.2", "body a2.\n"),
		makeSec(1, "B", "body b.\n"),
	}
	got := bundleOutermostSections(sections)
	if len(got) != 2 {
		t.Fatalf("want 2 bundles (A absorbs h3+h2, B is own), got %d: %+v", len(got), got)
	}
	if got[0].title != "A" {
		t.Errorf("bundle[0].title = %q, want 'A'", got[0].title)
	}
	if got[1].title != "B" {
		t.Errorf("bundle[1].title = %q, want 'B'", got[1].title)
	}
	if !strings.Contains(got[0].body, "### A.1") {
		t.Errorf("A bundle should contain h3 child, got: %q", got[0].body)
	}
	if !strings.Contains(got[0].body, "## A.2") {
		t.Errorf("A bundle should contain h2 child, got: %q", got[0].body)
	}
}

// ── runPageRewrite flag validation (no HTTP) ──────────────────────────────────

func TestPageRewrite_Help(t *testing.T) {
	out, _, code := runCLI(t, "page", "rewrite", "--help")
	if code != exitOK {
		t.Fatalf("--help: want exit 0, got %d", code)
	}
	for _, want := range []string{"--page-id", "--markdown", "--dry-run", "--allow-add", "--allow-remove"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q", want)
		}
	}
}

func TestPageRewrite_UnknownFlag(t *testing.T) {
	_, errOut, code := runCLI(t, "page", "rewrite", "--bogus")
	if code == exitOK {
		t.Fatal("want non-zero exit for unknown flag")
	}
	if !strings.Contains(errOut, "unknown flag") {
		t.Errorf("expected 'unknown flag' in error, got: %q", errOut)
	}
}

func TestPageRewrite_MarkdownFileNotFound(t *testing.T) {
	_, errOut, code := runCLI(t, "page", "rewrite",
		"--page-id", "123",
		"--markdown", "/no/such/file.md",
	)
	if code == exitOK {
		t.Fatal("want non-zero exit when markdown file is missing")
	}
	if !strings.Contains(errOut, "reading markdown") {
		t.Errorf("expected 'reading markdown' in error, got: %q", errOut)
	}
}

func TestPageRewrite_AllowAddAndAllowRemoveFlagsAccepted(t *testing.T) {
	// Flags are valid; failure must NOT be "unknown flag" —
	// it should be a missing-credentials error or similar.
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(mdFile, []byte("## Hello\nworld\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, errOut, _ := runCLI(t, "page", "rewrite",
		"--page-id", "123",
		"--markdown", mdFile,
		"--allow-add",
		"--allow-remove",
	)
	if strings.Contains(errOut, "unknown flag") {
		t.Errorf("--allow-add / --allow-remove should not produce 'unknown flag', got: %q", errOut)
	}
}

func TestPageRewrite_DryRunFlagAccepted(t *testing.T) {
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(mdFile, []byte("## Hello\nworld\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, errOut, _ := runCLI(t, "page", "rewrite",
		"--page-id", "123",
		"--markdown", mdFile,
		"--dry-run",
	)
	if strings.Contains(errOut, "unknown flag") {
		t.Errorf("--dry-run should not produce 'unknown flag', got: %q", errOut)
	}
}

func TestPageRewrite_MessageFlagAccepted(t *testing.T) {
	dir := t.TempDir()
	mdFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(mdFile, []byte("## Hello\nworld\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, errOut, _ := runCLI(t, "page", "rewrite",
		"--page-id", "123",
		"--markdown", mdFile,
		"--message", "my version comment",
	)
	if strings.Contains(errOut, "unknown flag") {
		t.Errorf("--message should not produce 'unknown flag', got: %q", errOut)
	}
}
