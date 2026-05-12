package adf

import (
	"strings"
	"testing"
)

func TestParsePropertiesBlock_basic(t *testing.T) {
	body := `tipo: reference
status: ativo
owner: @diego`
	entries := ParsePropertiesBlock(body)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(entries), entries)
	}
	if entries[0].Key != "tipo" || entries[0].Value != "reference" {
		t.Fatalf("entry[0] = %v", entries[0])
	}
	if entries[2].Key != "owner" || entries[2].Value != "@diego" {
		t.Fatalf("entry[2] = %v", entries[2])
	}
}

func TestParsePropertiesBlock_skipBlank(t *testing.T) {
	body := `
tipo: reference

status: ativo
`
	entries := ParsePropertiesBlock(body)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestParsePropertiesBlock_skipNoColon(t *testing.T) {
	body := `tipo: reference
this is not a kv line
status: rascunho`
	entries := ParsePropertiesBlock(body)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), entries)
	}
}

func TestParsePropertiesBlock_valueWithColon(t *testing.T) {
	// Value contains a colon — only the FIRST colon splits key from value.
	body := `url: https://example.com/foo`
	entries := ParsePropertiesBlock(body)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Value != "https://example.com/foo" {
		t.Fatalf("value = %q, want https://example.com/foo", entries[0].Value)
	}
}

func TestParsePropertiesBlock_links(t *testing.T) {
	body := `relacionados: [[PageA]], [[id:999]]`
	entries := ParsePropertiesBlock(body)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Value != "[[PageA]], [[id:999]]" {
		t.Fatalf("value = %q", entries[0].Value)
	}
}

func TestPropertiesBlockToStorageXML(t *testing.T) {
	body := `tipo: reference
status: ativo
relacionados: [[Página Principal]]`
	out := PropertiesBlockToStorageXML(body)
	if !strings.Contains(out, `ac:name="page-properties"`) {
		t.Fatalf("missing macro: %s", out)
	}
	if !strings.Contains(out, "<th>tipo</th>") {
		t.Fatalf("missing key 'tipo': %s", out)
	}
	if !strings.Contains(out, `ri:content-title="Página Principal"`) {
		t.Fatalf("missing page link: %s", out)
	}
}

func TestPropertiesBlockToStorageXML_empty(t *testing.T) {
	out := PropertiesBlockToStorageXML("   \n\n   ")
	if out != "" {
		t.Fatalf("expected empty string for empty block, got %q", out)
	}
}
