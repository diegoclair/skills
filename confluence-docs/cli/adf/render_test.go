package adf

import (
	"strings"
	"testing"
)

func TestRenderText_HeadingsAndParagraphs(t *testing.T) {
	doc := Doc(
		Heading(2, Text("Title")),
		Paragraph(Text("Some body text.")),
		Heading(3, Text("Subsection")),
		Paragraph(Text("More text.")),
	)
	out := RenderText(doc)
	if !strings.Contains(out, "## Title") {
		t.Errorf("missing h2: %q", out)
	}
	if !strings.Contains(out, "### Subsection") {
		t.Errorf("missing h3: %q", out)
	}
	if !strings.Contains(out, "Some body text.") {
		t.Errorf("missing body text: %q", out)
	}
}

func TestRenderText_BulletList(t *testing.T) {
	doc := Doc(
		BulletList(
			ListItem(Paragraph(Text("first"))),
			ListItem(Paragraph(Text("second"))),
		),
	)
	out := RenderText(doc)
	if !strings.Contains(out, "- first") {
		t.Errorf("missing first bullet: %q", out)
	}
	if !strings.Contains(out, "- second") {
		t.Errorf("missing second bullet: %q", out)
	}
}

func TestRenderText_TableWithHeader(t *testing.T) {
	doc := Doc(
		Table(
			TableRow(
				TableHeader(Paragraph(Text("Name"))),
				TableHeader(Paragraph(Text("ID"))),
			),
			TableRow(
				TableCell(Paragraph(Text("Home"))),
				TableCell(Paragraph(Text("164232"))),
			),
		),
	)
	out := RenderText(doc)
	if !strings.Contains(out, "| Name | ID |") {
		t.Errorf("missing header row: %q", out)
	}
	if !strings.Contains(out, "| --- | --- |") {
		t.Errorf("missing separator row: %q", out)
	}
	if !strings.Contains(out, "| Home | 164232 |") {
		t.Errorf("missing body row: %q", out)
	}
}

func TestRenderText_LinkInline(t *testing.T) {
	doc := Doc(
		Paragraph(
			Text("see "),
			Text("here", Link("https://example.com")),
		),
	)
	out := RenderText(doc)
	if !strings.Contains(out, "[here](https://example.com)") {
		t.Errorf("link not rendered: %q", out)
	}
}

func TestRenderText_PanelWithTitle(t *testing.T) {
	doc := Doc(
		Panel("warning", "", Paragraph(Text("be careful"))),
	)
	out := RenderText(doc)
	if !strings.Contains(out, "[warning]") {
		t.Errorf("missing panel marker: %q", out)
	}
	if !strings.Contains(out, "be careful") {
		t.Errorf("missing panel body: %q", out)
	}
}
