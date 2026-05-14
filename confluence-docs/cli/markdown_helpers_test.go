package main

import (
	"strings"
	"testing"

	"github.com/lybel-app/skills/pkg/atlassian/adf"
)

// ── TestParseATXHeading_Extended ─────────────────────────────────────────────
// main_test.go already covers basic cases; these add the remaining edge cases.

func TestParseATXHeading_Extended(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		wantLevel int
		wantText  string
		wantOK    bool
	}{
		// All six valid levels
		{name: "H1", line: "# Hello", wantLevel: 1, wantText: "Hello", wantOK: true},
		{name: "H2", line: "## World", wantLevel: 2, wantText: "World", wantOK: true},
		{name: "H3", line: "### Three", wantLevel: 3, wantText: "Three", wantOK: true},
		{name: "H4", line: "#### Four", wantLevel: 4, wantText: "Four", wantOK: true},
		{name: "H5", line: "##### Five", wantLevel: 5, wantText: "Five", wantOK: true},
		{name: "H6", line: "###### Six", wantLevel: 6, wantText: "Six", wantOK: true},

		// Trailing # tokens stripped
		{name: "trailing ## stripped", line: "## Foo ##", wantLevel: 2, wantText: "Foo", wantOK: true},
		{name: "trailing ### stripped", line: "### Bar ###", wantLevel: 3, wantText: "Bar", wantOK: true},

		// Title with extra interior spaces trimmed
		{name: "extra spaces in title", line: "#   Spaced Title", wantLevel: 1, wantText: "Spaced Title", wantOK: true},

		// 7 hashes — too many
		{name: "7 hashes", line: "####### Seven", wantLevel: 0, wantText: "", wantOK: false},

		// No space after hash
		{name: "no space H1", line: "#NoSpace", wantLevel: 0, wantText: "", wantOK: false},
		{name: "no space H2", line: "##NoSpace", wantLevel: 0, wantText: "", wantOK: false},

		// Empty / blank title
		{name: "hash + space only", line: "## ", wantLevel: 0, wantText: "", wantOK: false},
		{name: "title reduces to empty after trailing strip", line: "## ##", wantLevel: 0, wantText: "", wantOK: false},

		// Not a heading
		{name: "plain text", line: "just some text", wantLevel: 0, wantText: "", wantOK: false},
		{name: "empty string", line: "", wantLevel: 0, wantText: "", wantOK: false},
		{name: "dash list", line: "- item", wantLevel: 0, wantText: "", wantOK: false},

		// 4-space indented: starts with spaces, not '#' — not a heading
		{name: "4-space indented", line: "    # Foo", wantLevel: 0, wantText: "", wantOK: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			level, text, ok := parseATXHeading(c.line)
			if ok != c.wantOK {
				t.Errorf("ok = %v, want %v (line %q)", ok, c.wantOK, c.line)
			}
			if level != c.wantLevel {
				t.Errorf("level = %d, want %d", level, c.wantLevel)
			}
			if text != c.wantText {
				t.Errorf("text = %q, want %q", text, c.wantText)
			}
		})
	}
}

// ── TestMdSectionFullText ─────────────────────────────────────────────────────

func TestMdSectionFullText(t *testing.T) {
	cases := []struct {
		name     string
		section  mdSection
		wantText string
	}{
		{
			name:     "body present",
			section:  mdSection{level: 2, title: "Foo", headLine: "## Foo", body: "Some body.\n"},
			wantText: "## Foo\nSome body.\n",
		},
		{
			name:     "empty body",
			section:  mdSection{level: 1, title: "Bar", headLine: "# Bar", body: ""},
			wantText: "# Bar\n",
		},
		{
			name:     "multi-line body",
			section:  mdSection{level: 3, title: "Baz", headLine: "### Baz", body: "Line one.\nLine two.\n"},
			wantText: "### Baz\nLine one.\nLine two.\n",
		},
		{
			name:     "H6 heading",
			section:  mdSection{level: 6, title: "Deep", headLine: "###### Deep", body: ""},
			wantText: "###### Deep\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.section.fullText()
			if got != c.wantText {
				t.Errorf("fullText() = %q, want %q", got, c.wantText)
			}
		})
	}
}

// ── TestSplitMarkdownByHeadings_Extended ─────────────────────────────────────
// main_test.go covers the intro+two-section case and fullText.
// These cases add the remaining edge cases.

func TestSplitMarkdownByHeadings_Extended(t *testing.T) {
	cases := []struct {
		name          string
		src           string
		wantIntro     string
		wantSections  int
		wantTitles    []string
		wantBodyParts []string // substring expected in body of sections[i]
	}{
		{
			name:         "empty input",
			src:          "",
			wantIntro:    "",
			wantSections: 0,
		},
		{
			name:         "intro only, no headings",
			src:          "Just some text.\nNo headings here.\n",
			wantIntro:    "Just some text.\nNo headings here.\n",
			wantSections: 0,
		},
		{
			name:         "single heading no body",
			src:          "# Title\n",
			wantIntro:    "",
			wantSections: 1,
			wantTitles:   []string{"Title"},
		},
		{
			name:         "single heading with body",
			src:          "# Title\nSome body text.\n",
			wantIntro:    "",
			wantSections: 1,
			wantTitles:   []string{"Title"},
			wantBodyParts: []string{"Some body text."},
		},
		{
			name:          "intro then heading",
			src:           "Intro paragraph.\n\n# Section One\nBody here.\n",
			wantIntro:     "Intro paragraph.\n",
			wantSections:  1,
			wantTitles:    []string{"Section One"},
			wantBodyParts: []string{"Body here."},
		},
		{
			name: "three levels nested",
			src:  "# H1\nBody1.\n## H2\nBody2.\n### H3\nBody3.\n",
			wantIntro:     "",
			wantSections:  3,
			wantTitles:    []string{"H1", "H2", "H3"},
			wantBodyParts: []string{"Body1.", "Body2.", "Body3."},
		},
		{
			name:         "trailing blank lines trimmed from body",
			src:          "## Section\nContent here.\n\n\n",
			wantIntro:    "",
			wantSections: 1,
			wantTitles:   []string{"Section"},
			wantBodyParts: []string{"Content here."},
		},
		{
			name:         "whitespace-only intro becomes empty",
			src:          "\n\n# Heading\nBody.\n",
			wantIntro:    "",
			wantSections: 1,
			wantTitles:   []string{"Heading"},
		},
		{
			name:         "consecutive headings no bodies",
			src:          "# One\n# Two\n# Three\n",
			wantIntro:    "",
			wantSections: 3,
			wantTitles:   []string{"One", "Two", "Three"},
		},
		{
			name:         "body with blank lines inside preserved",
			src:          "## Section\nFirst para.\n\nSecond para.\n",
			wantIntro:    "",
			wantSections: 1,
			wantTitles:   []string{"Section"},
			wantBodyParts: []string{"First para."},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			intro, sections, err := splitMarkdownByHeadings([]byte(c.src))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if intro != c.wantIntro {
				t.Errorf("intro = %q, want %q", intro, c.wantIntro)
			}
			if len(sections) != c.wantSections {
				t.Errorf("len(sections) = %d, want %d", len(sections), c.wantSections)
			}
			for i, wantTitle := range c.wantTitles {
				if i >= len(sections) {
					t.Errorf("section[%d] missing (only %d sections)", i, len(sections))
					break
				}
				if sections[i].title != wantTitle {
					t.Errorf("sections[%d].title = %q, want %q", i, sections[i].title, wantTitle)
				}
			}
			for i, bodyPart := range c.wantBodyParts {
				if i >= len(sections) {
					break
				}
				if !strings.Contains(sections[i].body, bodyPart) {
					t.Errorf("sections[%d].body = %q, want it to contain %q", i, sections[i].body, bodyPart)
				}
			}
		})
	}
}

// ── TestHeadingLevelFromNode ──────────────────────────────────────────────────

func TestHeadingLevelFromNode(t *testing.T) {
	cases := []struct {
		name      string
		node      adf.Node
		wantLevel int
	}{
		{
			name:      "H1 int level",
			node:      adf.Heading(1, adf.Text("Title")),
			wantLevel: 1,
		},
		{
			name:      "H2 int level",
			node:      adf.Heading(2, adf.Text("Title")),
			wantLevel: 2,
		},
		{
			name:      "H3 int level",
			node:      adf.Heading(3, adf.Text("Title")),
			wantLevel: 3,
		},
		{
			name:      "H4 int level",
			node:      adf.Heading(4, adf.Text("Title")),
			wantLevel: 4,
		},
		{
			name:      "H5 int level",
			node:      adf.Heading(5, adf.Text("Title")),
			wantLevel: 5,
		},
		{
			name:      "H6 int level",
			node:      adf.Heading(6, adf.Text("Title")),
			wantLevel: 6,
		},
		{
			name:      "nil Attrs defaults to 1",
			node:      adf.Node{Type: "heading", Attrs: nil},
			wantLevel: 1,
		},
		{
			name:      "float64 level (JSON round-trip)",
			node:      adf.Node{Type: "heading", Attrs: map[string]any{"level": float64(3)}},
			wantLevel: 3,
		},
		{
			name:      "non-heading node without level defaults to 1",
			node:      adf.Paragraph(adf.Text("text")),
			wantLevel: 1,
		},
		{
			name:      "unknown level type defaults to 1",
			node:      adf.Node{Type: "heading", Attrs: map[string]any{"level": "bad"}},
			wantLevel: 1,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := headingLevelFromNode(c.node)
			if got != c.wantLevel {
				t.Errorf("headingLevelFromNode() = %d, want %d", got, c.wantLevel)
			}
		})
	}
}

// ── TestAllText ───────────────────────────────────────────────────────────────

func TestAllText(t *testing.T) {
	cases := []struct {
		name     string
		node     adf.Node
		wantText string
	}{
		{
			name:     "simple text node",
			node:     adf.Text("hello"),
			wantText: "hello",
		},
		{
			name:     "empty text node",
			node:     adf.Text(""),
			wantText: "",
		},
		{
			name:     "paragraph with single child",
			node:     adf.Paragraph(adf.Text("world")),
			wantText: "world",
		},
		{
			name:     "paragraph with multiple children concatenated",
			node:     adf.Paragraph(adf.Text("foo"), adf.Text("bar")),
			wantText: "foobar",
		},
		{
			name:     "heading node",
			node:     adf.Heading(2, adf.Text("Section Title")),
			wantText: "Section Title",
		},
		{
			name:     "empty paragraph",
			node:     adf.Paragraph(),
			wantText: "",
		},
		{
			name: "doc with two paragraphs",
			node: adf.Doc(
				adf.Paragraph(adf.Text("First")),
				adf.Paragraph(adf.Text("Second")),
			),
			wantText: "FirstSecond",
		},
		{
			name: "heading + paragraph in doc",
			node: adf.Doc(
				adf.Heading(1, adf.Text("Title")),
				adf.Paragraph(adf.Text("Body ")),
			),
			wantText: "TitleBody ",
		},
		{
			name:     "text with mark (bold) returns text unchanged",
			node:     adf.Text("bold text", adf.Mark{Type: "strong"}),
			wantText: "bold text",
		},
		{
			name: "deeply nested via blockquote",
			node: adf.Node{
				Type: "blockquote",
				Content: []adf.Node{
					adf.Paragraph(adf.Text("quoted")),
				},
			},
			wantText: "quoted",
		},
		{
			name:     "doc with no content",
			node:     adf.Doc(),
			wantText: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := allText(c.node)
			if got != c.wantText {
				t.Errorf("allText() = %q, want %q", got, c.wantText)
			}
		})
	}
}
