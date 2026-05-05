package adf

import (
	"encoding/json"
	"strings"
	"testing"
)

// sampleDoc builds a three-section doc used in most edit tests:
//
//	## Alpha
//	body A
//	## Bravo
//	body B
//	## Charlie
//	body C
func sampleDoc() Node {
	return Doc(
		Heading(2, Text("Alpha")),
		Paragraph(Text("body A")),
		Heading(2, Text("Bravo")),
		Paragraph(Text("body B")),
		Heading(2, Text("Charlie")),
		Paragraph(Text("body C")),
	)
}

func TestSectionContent(t *testing.T) {
	doc := sampleDoc()
	sub, err := SectionContent(doc, "Bravo", 0)
	if err != nil {
		t.Fatalf("SectionContent: %v", err)
	}
	if sub.Type != "doc" {
		t.Errorf("expected doc node, got %q", sub.Type)
	}
	// Sub should contain heading "Bravo" + its body, nothing else.
	if len(sub.Content) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(sub.Content))
	}
	if sub.Content[0].Type != "heading" {
		t.Errorf("[0] expected heading, got %q", sub.Content[0].Type)
	}
	out, _ := json.Marshal(sub)
	if !strings.Contains(string(out), "Bravo") {
		t.Errorf("expected Bravo in output: %s", out)
	}
	if strings.Contains(string(out), "body A") || strings.Contains(string(out), "body C") {
		t.Errorf("output leaked sibling sections: %s", out)
	}
}

func TestSectionContent_NotFound(t *testing.T) {
	doc := sampleDoc()
	_, err := SectionContent(doc, "Nonexistent", 0)
	if err == nil {
		t.Fatal("expected error for missing section")
	}
	if !strings.Contains(err.Error(), "Nonexistent") {
		t.Errorf("error should mention the missing heading: %v", err)
	}
}

func TestSectionContent_NestedSubsections(t *testing.T) {
	// h2 + h3 should include the h3 in the section body
	doc := Doc(
		Heading(2, Text("Parent")),
		Paragraph(Text("parent body")),
		Heading(3, Text("Child")),
		Paragraph(Text("child body")),
		Heading(2, Text("Sibling")),
		Paragraph(Text("sibling body")),
	)
	sub, err := SectionContent(doc, "Parent", 2)
	if err != nil {
		t.Fatalf("SectionContent: %v", err)
	}
	out, _ := json.Marshal(sub)
	if !strings.Contains(string(out), "child body") {
		t.Errorf("expected nested h3 to be included: %s", out)
	}
	if strings.Contains(string(out), "sibling body") {
		t.Errorf("sibling h2 should NOT be included: %s", out)
	}
}

func TestAppend(t *testing.T) {
	doc := sampleDoc()
	frag := []Node{Heading(2, Text("Delta")), Paragraph(Text("body D"))}

	got := Append(doc, frag)

	if len(got.Content) != 8 {
		t.Fatalf("want 8 top-level nodes, got %d", len(got.Content))
	}
	if h := headingText(got.Content[6]); h != "Delta" {
		t.Errorf("want appended heading Delta, got %q", h)
	}
	if len(doc.Content) != 6 {
		t.Errorf("Append mutated input doc: now has %d nodes", len(doc.Content))
	}
}

func TestReplaceSection(t *testing.T) {
	doc := sampleDoc()
	frag := []Node{Heading(2, Text("Bravo v2")), Paragraph(Text("new body"))}

	got, err := ReplaceSection(doc, "Bravo", frag)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantTitles := []string{"Alpha", "Bravo v2", "Charlie"}
	gotTitles := headingTitles(got)
	if !eqStrings(gotTitles, wantTitles) {
		t.Errorf("want headings %v, got %v", wantTitles, gotTitles)
	}

	// Alpha and Charlie sections must remain intact.
	if txt := paraText(got.Content[1]); txt != "body A" {
		t.Errorf("Alpha body corrupted: %q", txt)
	}
	if txt := paraText(got.Content[5]); txt != "body C" {
		t.Errorf("Charlie body corrupted: %q", txt)
	}
}

func TestReplaceSection_KeepsMacrosOutsideTarget(t *testing.T) {
	// A common real-world case: page has an Expand between sections; replacing
	// a later section must not touch the Expand.
	doc := Doc(
		Heading(2, Text("Alpha")),
		Paragraph(Text("body A")),
		Heading(2, Text("Bravo")),
		Expand("keep me", Paragraph(Text("inside expand"))),
		Heading(2, Text("Charlie")),
		Paragraph(Text("body C")),
	)

	got, err := ReplaceSection(doc, "Charlie", []Node{
		Heading(2, Text("Charlie v2")),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Content[3].Type != "expand" {
		t.Fatalf("expand at index 3 was corrupted: type=%q", got.Content[3].Type)
	}
	if got.Content[3].Attrs["title"] != "keep me" {
		t.Errorf("expand title changed: %v", got.Content[3].Attrs["title"])
	}
}

func TestInsertAfter(t *testing.T) {
	doc := sampleDoc()
	frag := []Node{Heading(2, Text("Alpha.5"))}

	got, err := InsertAfter(doc, "Alpha", frag)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"Alpha", "Alpha.5", "Bravo", "Charlie"}
	if titles := headingTitles(got); !eqStrings(titles, want) {
		t.Errorf("want %v, got %v", want, titles)
	}
}

func TestInsertBefore(t *testing.T) {
	doc := sampleDoc()
	frag := []Node{Heading(2, Text("Pre-Bravo"))}

	got, err := InsertBefore(doc, "Bravo", frag)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"Alpha", "Pre-Bravo", "Bravo", "Charlie"}
	if titles := headingTitles(got); !eqStrings(titles, want) {
		t.Errorf("want %v, got %v", want, titles)
	}
}

func TestDeleteSection(t *testing.T) {
	doc := sampleDoc()

	got, err := DeleteSection(doc, "Bravo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"Alpha", "Charlie"}
	if titles := headingTitles(got); !eqStrings(titles, want) {
		t.Errorf("want %v, got %v", want, titles)
	}
}

func TestSectionBoundary_StopsAtEqualLevel(t *testing.T) {
	// An H3 inside a section should NOT terminate an H2 section; only H2 or
	// higher does. This protects nested structure when replacing by H2.
	doc := Doc(
		Heading(2, Text("Alpha")),
		Heading(3, Text("Alpha child")),
		Paragraph(Text("nested")),
		Heading(2, Text("Bravo")),
	)

	got, err := DeleteSection(doc, "Alpha")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Everything from Alpha through "nested" paragraph should be gone; only
	// Bravo remains.
	if len(got.Content) != 1 {
		t.Fatalf("want 1 remaining node, got %d", len(got.Content))
	}
	if headingText(got.Content[0]) != "Bravo" {
		t.Errorf("want Bravo remaining, got %q", headingText(got.Content[0]))
	}
}

func TestSectionNotFound(t *testing.T) {
	doc := sampleDoc()
	_, err := ReplaceSection(doc, "Nonexistent", nil)
	if err == nil {
		t.Fatal("want error for missing section, got nil")
	}
	if !strings.Contains(err.Error(), "Nonexistent") {
		t.Errorf("error should mention the heading: %v", err)
	}
}

func TestHeadingMatch_Trims(t *testing.T) {
	doc := Doc(Heading(2, Text("  Alpha  ")))
	if _, _, ok := findSectionBounds(doc.Content, "Alpha"); !ok {
		t.Error("want match with trimmed heading text")
	}
}

func TestUnmarshalDoc_RoundTrip(t *testing.T) {
	// Build a doc, marshal, unmarshal: should produce an equivalent tree that
	// survives another marshal without shape drift.
	orig := sampleDoc()
	data, err := Marshal(orig, false)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := UnmarshalDoc(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Type != "doc" {
		t.Errorf("want doc type, got %q", got.Type)
	}
	if len(got.Content) != len(orig.Content) {
		t.Errorf("content length drift: got %d want %d", len(got.Content), len(orig.Content))
	}
	// Level should survive as int-compatible even though JSON -> float64.
	if l := headingLevel(got.Content[0]); l != 2 {
		t.Errorf("heading level drift: got %d want 2", l)
	}
}

func TestUnmarshalDoc_RejectsNonDoc(t *testing.T) {
	data, _ := json.Marshal(Paragraph(Text("hi")))
	if _, err := UnmarshalDoc(data); err == nil {
		t.Error("want error for non-doc root, got nil")
	}
}

func TestConvertFragment_NoDocWrapper(t *testing.T) {
	nodes, err := ConvertFragment([]byte("# Hello\n\nworld"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes (heading + paragraph), got %d", len(nodes))
	}
	if nodes[0].Type != "heading" {
		t.Errorf("want heading first, got %q", nodes[0].Type)
	}
	if nodes[1].Type != "paragraph" {
		t.Errorf("want paragraph second, got %q", nodes[1].Type)
	}
}

// ---------- helpers ----------

func headingTitles(doc Node) []string {
	var out []string
	for _, n := range doc.Content {
		if n.Type == "heading" {
			out = append(out, headingText(n))
		}
	}
	return out
}

func paraText(n Node) string {
	var sb strings.Builder
	collectText(n, &sb)
	return sb.String()
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
