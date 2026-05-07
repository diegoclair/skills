// Package adf - Page digest builder.
//
// Produces a compact, LLM-friendly summary of a Confluence page from its ADF
// body plus PageMeta. The goal is to answer most "what's in this page?"
// questions without round-tripping the full ADF (which can be tens of KB).
package adf

import (
	"fmt"
	"sort"
	"strings"
)

// Digest is the slim summary of a page.
type Digest struct {
	PageID      string           `json:"pageId"`
	Title       string           `json:"title"`
	URL         string           `json:"url"`
	Version     int              `json:"version"`
	Sections    []SectionSummary `json:"sections"`
	TotalWords  int              `json:"totalWords"`
	MacroCounts map[string]int   `json:"macroCounts"` // e.g. "expand": 2, "panel-warning": 1, "toc": 1
	LinksCount  int              `json:"linksCount"`
	// Status is a Lybel convention: page titles often start with a status
	// emoji (🟢/🟡/🔴/🟠/🔵/⚪/✅) that conveys where the page sits in its
	// lifecycle. When present, this field carries the parsed semantic label
	// ("active", "evaluating", "blocked", etc.) so callers can answer
	// "qual o status de X" without opening the page.
	// Empty when the title has no recognized status emoji.
	Status      string `json:"status,omitempty"`
	StatusEmoji string `json:"statusEmoji,omitempty"`
}

// statusFromTitle scans the leading characters of a title for a Lybel
// status emoji and returns (emoji, label). Returns ("","") when no
// recognized emoji is present.
//
// The mapping reflects the conventions documented across the Lybel
// Confluence space (see Home → "Regras de organização"):
//
//	🟢 active       — em andamento / contratado / aprovado
//	🟡 in-progress  — em negociação / em tratativa
//	🟠 evaluating   — em avaliação / análise / processo seletivo
//	🔴 blocked      — bloqueado / não aplica / parado
//	🔵 researched   — pesquisado / mapeado, sem ação
//	⚪ idle         — sem ação / arquivado / dormant
//	✅ done         — concluído
func statusFromTitle(title string) (string, string) {
	t := strings.TrimSpace(title)
	if t == "" {
		return "", ""
	}
	pairs := []struct {
		emoji, label string
	}{
		{"🟢", "active"},
		{"🟡", "in-progress"},
		{"🟠", "evaluating"},
		{"🔴", "blocked"},
		{"🔵", "researched"},
		{"⚪", "idle"},
		{"✅", "done"},
	}
	for _, p := range pairs {
		if strings.HasPrefix(t, p.emoji) {
			return p.emoji, p.label
		}
	}
	return "", ""
}

// SectionSummary describes one top-level heading and its body.
type SectionSummary struct {
	Level    int      `json:"level"`              // 1-6
	Heading  string   `json:"heading"`            // trimmed text
	Words    int      `json:"words"`              // word count of body (excluding heading)
	Macros   []string `json:"macros,omitempty"`   // macros present inside this section, e.g. ["expand", "panel-warning"]
	HasTable bool     `json:"hasTable,omitempty"`
}

// BuildDigest analyzes an ADF doc and returns a Digest. PageID/Title/URL/Version
// are passed in by the caller (the ADF doc itself doesn't carry them).
//
// Every top-level heading is reported as its own SectionSummary, with word
// count for the body until the NEXT heading of any level. This produces a
// flat outline view (parent doesn't double-count children's words), with the
// hierarchy preserved via the Level field for indented rendering.
func BuildDigest(doc Node, pageID, title, url string, version int) Digest {
	statusEmoji, statusLabel := statusFromTitle(title)
	d := Digest{
		PageID:      pageID,
		Title:       title,
		URL:         url,
		Version:     version,
		MacroCounts: map[string]int{},
		Status:      statusLabel,
		StatusEmoji: statusEmoji,
	}

	nodes := doc.Content
	i := 0
	preludeAcc := SectionSummary{Level: 0, Heading: "(intro)"}
	hasPrelude := false

	for i < len(nodes) {
		n := nodes[i]
		if n.Type == "heading" {
			if hasPrelude && (preludeAcc.Words > 0 || len(preludeAcc.Macros) > 0 || preludeAcc.HasTable) {
				d.Sections = append(d.Sections, preludeAcc)
				d.TotalWords += preludeAcc.Words
			}
			hasPrelude = false

			level := headingLevel(n)
			heading := strings.TrimSpace(headingText(n))
			sec := SectionSummary{Level: level, Heading: heading}

			// Body runs until the NEXT heading of ANY level — gives a flat
			// outline where each heading owns only its own paragraphs.
			j := i + 1
			for j < len(nodes) {
				if nodes[j].Type == "heading" {
					break
				}
				accumulateNode(nodes[j], &sec, &d)
				j++
			}
			d.Sections = append(d.Sections, sec)
			d.TotalWords += sec.Words
			i = j
			continue
		}

		hasPrelude = true
		accumulateNode(n, &preludeAcc, &d)
		i++
	}
	if hasPrelude && (preludeAcc.Words > 0 || len(preludeAcc.Macros) > 0 || preludeAcc.HasTable) {
		d.Sections = append(d.Sections, preludeAcc)
		d.TotalWords += preludeAcc.Words
	}

	return d
}

// accumulateNode walks a sub-tree counting words, links, macros, tables.
// Updates both the section summary and the document-wide totals.
func accumulateNode(n Node, sec *SectionSummary, d *Digest) {
	switch n.Type {
	case "text":
		if n.Text != "" {
			sec.Words += countWords(n.Text)
		}
		// Count link marks at the document level
		for _, m := range n.Marks {
			if m.Type == "link" {
				d.LinksCount++
			}
		}
		return
	case "table":
		sec.HasTable = true
	case "expand":
		addMacro(sec, "expand")
		d.MacroCounts["expand"]++
	case "panel":
		pType := "panel"
		if n.Attrs != nil {
			if v, ok := n.Attrs["panelType"].(string); ok && v != "" {
				pType = "panel-" + v
			}
		}
		addMacro(sec, pType)
		d.MacroCounts[pType]++
	case "extension":
		// Confluence macros (TOC, status, etc.) ride on `extension` nodes.
		key := "extension"
		if n.Attrs != nil {
			if k, ok := n.Attrs["extensionKey"].(string); ok && k != "" {
				key = k
			}
		}
		addMacro(sec, key)
		d.MacroCounts[key]++
	case "inlineCard", "card":
		d.LinksCount++
	}
	for _, c := range n.Content {
		accumulateNode(c, sec, d)
	}
}

func addMacro(sec *SectionSummary, name string) {
	for _, m := range sec.Macros {
		if m == name {
			return
		}
	}
	sec.Macros = append(sec.Macros, name)
}

// countWords returns a coarse word count: runs of non-whitespace separated
// by whitespace. Good enough for digest purposes.
func countWords(s string) int {
	n := 0
	inWord := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			inWord = false
			continue
		}
		if !inWord {
			n++
			inWord = true
		}
	}
	return n
}

// FormatText renders the digest as compact human/LLM-readable plain text.
// Designed to fit in <1 KB for typical pages — replaces a 10-40 KB ADF read.
func (d Digest) FormatText() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Page: %s\n", d.Title)
	fmt.Fprintf(&sb, "PageID: %s\n", d.PageID)
	if d.URL != "" {
		fmt.Fprintf(&sb, "URL: %s\n", d.URL)
	}
	if d.Version > 0 {
		fmt.Fprintf(&sb, "Version: %d\n", d.Version)
	}
	if d.Status != "" {
		fmt.Fprintf(&sb, "Status: %s %s\n", d.StatusEmoji, d.Status)
	}
	fmt.Fprintf(&sb, "Words: %d | Sections: %d | Links: %d\n", d.TotalWords, len(d.Sections), d.LinksCount)

	if len(d.Sections) > 0 {
		sb.WriteString("\nSections:\n")
		prefixes := computeTreePrefixes(d.Sections)
		for i, s := range d.Sections {
			lvl := "h" + itoa(s.Level)
			if s.Level == 0 {
				lvl = "--"
			}
			fmt.Fprintf(&sb, "  %s%s %s  (%dw", prefixes[i], lvl, s.Heading, s.Words)
			if s.HasTable {
				sb.WriteString(", table")
			}
			if len(s.Macros) > 0 {
				fmt.Fprintf(&sb, ", %s", strings.Join(s.Macros, "+"))
			}
			sb.WriteString(")\n")
		}
	}

	if len(d.MacroCounts) > 0 {
		sb.WriteString("\nMacros:")
		// Stable order
		keys := make([]string, 0, len(d.MacroCounts))
		for k := range d.MacroCounts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&sb, " %s×%d", k, d.MacroCounts[k])
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// computeTreePrefixes returns, for each section in order, the visual prefix
// to render before its label so the parent → child hierarchy is unambiguous.
//
// Top-level sections (level 1 or 2, plus the synthetic level-0 "(intro)") get
// no tree decoration — they render flush left. Sub-headings (level >= 3)
// get a tree-style prefix:
//
//	├─    when more siblings of the same level follow under the same parent
//	└─    when this is the last sibling at its level under the parent
//	│     in the column of an ancestor that still has more siblings to come
//	(spaces) in the column of an ancestor whose branch is already terminated
//
// Example output for a typical h2 with three h3 children:
//
//	## A
//	├─ ### A1
//	├─ ### A2
//	└─ ### A3
//	## B
//	└─ ### B1
//
// The implementation makes one pass to determine "is this the last sibling
// at its level within its parent's scope" for every section, then a second
// pass to build each prefix using ancestors' last-sibling status.
func computeTreePrefixes(sections []SectionSummary) []string {
	n := len(sections)
	prefixes := make([]string, n)
	if n == 0 {
		return prefixes
	}

	// Pass 1: for each section, is it the last sibling at its level before
	// its parent's scope ends? Parent scope ends when we encounter a section
	// at a strictly shallower level.
	isLast := make([]bool, n)
	for i := 0; i < n; i++ {
		L := sections[i].Level
		isLast[i] = true
		for j := i + 1; j < n; j++ {
			jL := sections[j].Level
			if jL < L {
				break // exited parent's scope
			}
			if jL == L {
				isLast[i] = false
				break
			}
		}
	}

	// Pass 2: build the prefix for each section by walking back to find each
	// ancestor's index and checking that ancestor's isLast[].
	for i := 0; i < n; i++ {
		L := sections[i].Level
		// Top-level (level <= 2) and the synthetic "(intro)" (level 0) get
		// no decoration. They render flush left.
		if L <= 2 {
			continue
		}

		// For each ancestor level from 3 up to L-1, find the most recent
		// section at that level that comes before i and is still in i's
		// chain (i.e. no shallower section has interrupted it).
		ancestors := map[int]int{}
		minSeenLevel := L
		for j := i - 1; j >= 0; j-- {
			jL := sections[j].Level
			if jL == 0 {
				continue // intro pseudo-section is not an ancestor
			}
			if jL < minSeenLevel {
				ancestors[jL] = j
				minSeenLevel = jL
			}
			if minSeenLevel <= 2 {
				break
			}
		}

		var prefix strings.Builder
		// Ancestor columns: levels 3 .. L-1.
		for ancL := 3; ancL < L; ancL++ {
			if idx, ok := ancestors[ancL]; ok && !isLast[idx] {
				prefix.WriteString("│  ")
			} else {
				prefix.WriteString("   ")
			}
		}
		// This section's own connector.
		if isLast[i] {
			prefix.WriteString("└─ ")
		} else {
			prefix.WriteString("├─ ")
		}
		prefixes[i] = prefix.String()
	}
	return prefixes
}
