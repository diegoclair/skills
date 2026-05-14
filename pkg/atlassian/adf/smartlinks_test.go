package adf

import (
	"testing"
)

func TestClassifySmartLinkLine_embed(t *testing.T) {
	cases := []struct {
		line string
	}{
		{"![embed](https://youtube.com/watch?v=abc)"},
		{"  ![embed](https://youtube.com/watch?v=abc)  "},
		{"![EMBED](https://vimeo.com/123)"},
	}
	for _, tc := range cases {
		kind, url := classifySmartLinkLine(tc.line)
		if kind != smartLinkEmbed {
			t.Errorf("line %q: expected smartLinkEmbed, got %d", tc.line, kind)
			continue
		}
		if url == "" {
			t.Errorf("line %q: expected non-empty URL", tc.line)
		}
	}
}

func TestClassifySmartLinkLine_block(t *testing.T) {
	cases := []struct {
		line string
	}{
		// Bare URL — always blockCard
		{"https://github.com/foo/bar"},
		{"https://linear.app/issue/ENG-1"},
		// Standalone link where text == URL (auto-pasted URL wrapped in link)
		{"[https://notion.so/page-id](https://notion.so/page-id)"},
	}
	for _, tc := range cases {
		kind, _ := classifySmartLinkLine(tc.line)
		if kind != smartLinkBlock {
			t.Errorf("line %q: expected smartLinkBlock, got %d", tc.line, kind)
		}
	}
}

func TestClassifySmartLinkLine_none(t *testing.T) {
	cases := []struct {
		line string
	}{
		// Inline link embedded in text — not a standalone line
		{"Check out [this link](https://example.com) for details."},
		// Named standalone link (text != url) — stays as regular link
		{"[Análise Stripe](https://linear.app/issue/ENG-1)"},
		{"[click here](https://example.com)"},
		// Normal image with non-embed alt
		{"![logo](https://example.com/logo.png)"},
		// Plain text
		{"This is just a sentence."},
		// Empty
		{""},
		{"   "},
	}
	for _, tc := range cases {
		kind, _ := classifySmartLinkLine(tc.line)
		if kind != smartLinkNone {
			t.Errorf("line %q: expected smartLinkNone, got %d", tc.line, kind)
		}
	}
}

func TestPreprocessSmartLinks_embed(t *testing.T) {
	lines := []string{
		"Some intro text",
		"![embed](https://youtube.com/watch?v=xyz)",
		"After the embed.",
	}
	out, macros := preprocessSmartLinks(lines, nil)
	if len(out) != 3 {
		t.Fatalf("expected 3 output lines, got %d", len(out))
	}
	if len(macros) != 1 {
		t.Fatalf("expected 1 macro, got %d", len(macros))
	}
	if macros[0].kind != "embedCard" {
		t.Fatalf("expected macro kind embedCard, got %q", macros[0].kind)
	}
	// The placeholder line should replace the original embed line
	if out[1] != placeholder(0) {
		t.Fatalf("line[1] = %q, want %q", out[1], placeholder(0))
	}
	// Surrounding lines unchanged
	if out[0] != "Some intro text" || out[2] != "After the embed." {
		t.Fatalf("surrounding lines were altered")
	}
	// Render produces an embedCard node
	node := macros[0].render()
	if node.Type != "embedCard" {
		t.Fatalf("rendered type = %q, want embedCard", node.Type)
	}
	if node.Attrs["url"] != "https://youtube.com/watch?v=xyz" {
		t.Fatalf("url = %v", node.Attrs["url"])
	}
}

func TestPreprocessSmartLinks_block(t *testing.T) {
	// Bare URL on its own line → blockCard
	lines := []string{
		"https://linear.app/team/issue/ENG-42",
	}
	out, macros := preprocessSmartLinks(lines, nil)
	if len(macros) != 1 {
		t.Fatalf("expected 1 macro, got %d", len(macros))
	}
	if macros[0].kind != "blockCard" {
		t.Fatalf("expected blockCard, got %q", macros[0].kind)
	}
	_ = out
	node := macros[0].render()
	if node.Type != "blockCard" {
		t.Fatalf("rendered type = %q, want blockCard", node.Type)
	}
}

func TestPreprocessSmartLinks_noneUnchanged(t *testing.T) {
	lines := []string{
		"# Heading",
		"Paragraph with [inline link](https://example.com) inside.",
		"![logo](https://example.com/img.png)",
	}
	out, macros := preprocessSmartLinks(lines, nil)
	if len(macros) != 0 {
		t.Fatalf("expected 0 macros, got %d", len(macros))
	}
	for i, l := range lines {
		if out[i] != l {
			t.Errorf("line[%d] changed: %q → %q", i, l, out[i])
		}
	}
}
