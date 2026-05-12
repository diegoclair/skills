package adf

import (
	"strings"
	"testing"
)

// ---------- Status ----------

func TestStatus(t *testing.T) {
	n := Status("Em andamento", StatusGreen, "abc-123")
	if n.Type != "status" {
		t.Fatalf("type = %q, want status", n.Type)
	}
	if n.Attrs["text"] != "Em andamento" {
		t.Fatalf("text = %v, want Em andamento", n.Attrs["text"])
	}
	if n.Attrs["color"] != "green" {
		t.Fatalf("color = %v, want green", n.Attrs["color"])
	}
	if n.Attrs["localId"] != "abc-123" {
		t.Fatalf("localId = %v, want abc-123", n.Attrs["localId"])
	}
}

func TestStatusNoLocalId(t *testing.T) {
	n := Status("Draft", StatusNeutral, "")
	if _, ok := n.Attrs["localId"]; ok {
		t.Fatal("localId should be absent when empty string passed")
	}
}

// ---------- Smart Links ----------

func TestInlineCard(t *testing.T) {
	n := InlineCard("https://example.com")
	if n.Type != "inlineCard" {
		t.Fatalf("type = %q, want inlineCard", n.Type)
	}
	if n.Attrs["url"] != "https://example.com" {
		t.Fatalf("url = %v", n.Attrs["url"])
	}
}

func TestBlockCard(t *testing.T) {
	n := BlockCard("https://linear.app/issue/ENG-1")
	if n.Type != "blockCard" {
		t.Fatalf("type = %q, want blockCard", n.Type)
	}
}

func TestEmbedCard(t *testing.T) {
	n := EmbedCard("https://youtube.com/watch?v=xyz", "wide")
	if n.Type != "embedCard" {
		t.Fatalf("type = %q, want embedCard", n.Type)
	}
	if n.Attrs["layout"] != "wide" {
		t.Fatalf("layout = %v, want wide", n.Attrs["layout"])
	}
}

func TestEmbedCardDefaultLayout(t *testing.T) {
	n := EmbedCard("https://youtube.com/watch?v=xyz", "")
	if n.Attrs["layout"] != "wide" {
		t.Fatalf("default layout should be wide, got %v", n.Attrs["layout"])
	}
}

// ---------- Layout ----------

func TestLayoutTwoEqual(t *testing.T) {
	col1 := []Node{Paragraph(Text("left"))}
	col2 := []Node{Paragraph(Text("right"))}
	layout := Layout(LayoutTwoEqual, col1, col2)
	if layout.Type != "layoutSection" {
		t.Fatalf("type = %q, want layoutSection", layout.Type)
	}
	if len(layout.Content) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(layout.Content))
	}
	for _, col := range layout.Content {
		if col.Type != "layoutColumn" {
			t.Fatalf("child type = %q, want layoutColumn", col.Type)
		}
		w, _ := col.Attrs["width"].(float64)
		if w != 50 {
			t.Fatalf("width = %v, want 50", col.Attrs["width"])
		}
	}
}

func TestLayoutSingle(t *testing.T) {
	col := []Node{Paragraph(Text("content"))}
	layout := Layout(LayoutSingle, col)
	if len(layout.Content) != 1 {
		t.Fatalf("expected 1 column, got %d", len(layout.Content))
	}
	w, _ := layout.Content[0].Attrs["width"].(float64)
	if w != 100 {
		t.Fatalf("width = %v, want 100", w)
	}
}

func TestLayoutThreeEqual(t *testing.T) {
	col := []Node{Paragraph(Text("x"))}
	layout := Layout(LayoutThreeEqual, col, col, col)
	if len(layout.Content) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(layout.Content))
	}
}

// ---------- Page Properties ----------

func TestPagePropertiesToStorage_basic(t *testing.T) {
	entries := []PagePropertiesEntry{
		{Key: "tipo", Value: "reference"},
		{Key: "status", Value: "ativo"},
	}
	out := PagePropertiesToStorage(entries)
	if !strings.Contains(out, `ac:name="page-properties"`) {
		t.Fatalf("missing macro name: %s", out)
	}
	if !strings.Contains(out, "<th>tipo</th>") {
		t.Fatalf("missing key 'tipo': %s", out)
	}
	if !strings.Contains(out, "<td>reference</td>") {
		t.Fatalf("missing value 'reference': %s", out)
	}
}

func TestPagePropertiesToStorage_link(t *testing.T) {
	entries := []PagePropertiesEntry{
		{Key: "relacionados", Value: "[[Análise Stripe Brasil]]"},
	}
	out := PagePropertiesToStorage(entries)
	if !strings.Contains(out, `ri:content-title="Análise Stripe Brasil"`) {
		t.Fatalf("missing page link: %s", out)
	}
	if !strings.Contains(out, "<ac:link>") {
		t.Fatalf("missing ac:link: %s", out)
	}
}

func TestPagePropertiesToStorage_idLink(t *testing.T) {
	entries := []PagePropertiesEntry{
		{Key: "relacionados", Value: "[[id:12345]]"},
	}
	out := PagePropertiesToStorage(entries)
	if !strings.Contains(out, `ri:content-title="12345"`) {
		t.Fatalf("missing id-based link: %s", out)
	}
}

func TestPagePropertiesToStorage_xmlEscape(t *testing.T) {
	entries := []PagePropertiesEntry{
		{Key: "desc", Value: "a & b <test> \"quoted\""},
	}
	out := PagePropertiesToStorage(entries)
	if strings.Contains(out, `<test>`) {
		t.Fatalf("raw < > should be escaped: %s", out)
	}
	if !strings.Contains(out, "&amp;") {
		t.Fatalf("& should become &amp;: %s", out)
	}
}

func TestPagePropertiesToStorage_multipleLinks(t *testing.T) {
	entries := []PagePropertiesEntry{
		{Key: "relacionados", Value: "[[PageA]], [[id:999]]"},
	}
	out := PagePropertiesToStorage(entries)
	if !strings.Contains(out, `ri:content-title="PageA"`) {
		t.Fatalf("missing PageA link: %s", out)
	}
	if !strings.Contains(out, `ri:content-title="999"`) {
		t.Fatalf("missing id:999 link: %s", out)
	}
}

// ---------- MarshalBodyValue ----------

func TestMarshalBodyValue(t *testing.T) {
	doc := Doc(Paragraph(Text("hello")))
	val, err := MarshalBodyValue(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(val, `{"type":"doc"`) {
		t.Fatalf("unexpected value prefix: %s", val[:min(50, len(val))])
	}
	// Must be a plain JSON string (not nested object).
	if val[0] == '{' && strings.Count(val, `"type":"doc"`) != 1 {
		t.Fatalf("value should be a serialised JSON string, got: %s", val[:50])
	}
}
