// Package adf — parser for the :::properties fenced block extension.
//
// Syntax (inside a :::properties ... ::: block):
//
//	tipo: reference
//	status: ativo
//	owner: @diego
//	tags: psp, cobranca, recorrencia
//	relacionados: [[outra-pagina]], [[id:12345]]
//	criado: 2026-05-12
//	atualizado: 2026-05-12
//
// Each line is "key: value". Blank lines and lines without ':' are ignored.
// Values may contain [[link]] syntax (parsed by PagePropertiesToStorage).
// The block is converted to the Confluence page-properties storage macro.
package adf

import (
	"strings"
)

// ParsePropertiesBlock parses the body of a :::properties block (the lines
// between the opening :::properties and the closing :::) into an ordered list
// of PagePropertiesEntry values.
//
// Rules:
//   - Lines with "key: value" are parsed (first colon splits key from value).
//   - Blank lines are skipped.
//   - Lines without a colon are skipped.
//   - Key and value are trimmed of leading/trailing whitespace.
func ParsePropertiesBlock(body string) []PagePropertiesEntry {
	var entries []PagePropertiesEntry
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			continue
		}
		entries = append(entries, PagePropertiesEntry{Key: key, Value: val})
	}
	return entries
}

// PropertiesBlockToStorageXML is a convenience wrapper that parses the block
// body and returns the Confluence storage XML for the page-properties macro.
func PropertiesBlockToStorageXML(body string) string {
	entries := ParsePropertiesBlock(body)
	if len(entries) == 0 {
		return ""
	}
	return PagePropertiesToStorage(entries)
}
