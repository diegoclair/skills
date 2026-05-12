package adf

import (
	"strings"
	"testing"
)

func TestRequiresStorageFormat_positive(t *testing.T) {
	cases := []string{
		":::properties\ntipo: reference\n:::\n",
		"  :::  properties  \ntipo: x\n:::",
		"## Heading\n\n:::properties\nstatus: ativo\n:::\n",
	}
	for _, c := range cases {
		if !RequiresStorageFormat(c) {
			t.Errorf("expected true for: %q", c)
		}
	}
}

func TestRequiresStorageFormat_negative(t *testing.T) {
	cases := []string{
		"## Heading\n\nNormal paragraph.",
		":::expand My Title\nSome content\n:::",
		":::info\nNote here\n:::",
		"[TOC]",
	}
	for _, c := range cases {
		if RequiresStorageFormat(c) {
			t.Errorf("expected false for: %q", c)
		}
	}
}

func TestMarkdownToStorage_basic(t *testing.T) {
	src := ":::properties\ntipo: reference\nstatus: ativo\n:::\n\n## Conteúdo\n\nTexto qualquer.\n"
	out, err := MarkdownToStorage([]byte(src))
	if err != nil {
		t.Fatalf("MarkdownToStorage error: %v", err)
	}
	// Must contain the page-properties macro.
	if !strings.Contains(out, `ac:name="page-properties"`) {
		t.Errorf("missing page-properties macro in output:\n%s", out)
	}
	// Must contain the heading.
	if !strings.Contains(out, "<h2>") {
		t.Errorf("missing h2 heading in output:\n%s", out)
	}
	// Must NOT contain a code block wrapping the XML.
	if strings.Contains(out, "<code") || strings.Contains(out, "<pre") {
		t.Errorf("output should not contain a code/pre block:\n%s", out)
	}
	// Must NOT wrap the macro in a paragraph.
	if strings.Contains(out, "<p><ac:structured-macro") {
		t.Errorf("macro must not be wrapped in a <p> tag:\n%s", out)
	}
}

func TestMarkdownToStorage_noProperties(t *testing.T) {
	src := "## Heading\n\nParagraph text.\n"
	out, err := MarkdownToStorage([]byte(src))
	if err != nil {
		t.Fatalf("MarkdownToStorage error: %v", err)
	}
	if !strings.Contains(out, "<h2>") {
		t.Errorf("missing heading:\n%s", out)
	}
	if !strings.Contains(out, "<p>") {
		t.Errorf("missing paragraph:\n%s", out)
	}
}

func TestMarkdownToStorage_multipleProperties(t *testing.T) {
	src := ":::properties\ntipo: reference\n:::\n\n## Section\n\n:::properties\nstatus: ativo\n:::\n"
	out, err := MarkdownToStorage([]byte(src))
	if err != nil {
		t.Fatalf("MarkdownToStorage error: %v", err)
	}
	count := strings.Count(out, `ac:name="page-properties"`)
	if count != 2 {
		t.Errorf("expected 2 page-properties macros, got %d:\n%s", count, out)
	}
}

func TestMarkdownToStorage_emptyPropertiesBlockSkipped(t *testing.T) {
	// An :::properties block with no valid key:value lines should be omitted.
	src := ":::properties\n\n\n:::\n\n## Heading\n\nText.\n"
	out, err := MarkdownToStorage([]byte(src))
	if err != nil {
		t.Fatalf("MarkdownToStorage error: %v", err)
	}
	if strings.Contains(out, `ac:name="page-properties"`) {
		t.Errorf("empty properties block should be omitted:\n%s", out)
	}
	if !strings.Contains(out, "<h2>") {
		t.Errorf("heading should still be present:\n%s", out)
	}
}
