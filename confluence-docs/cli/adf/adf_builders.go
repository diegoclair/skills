// Package adf — additional ADF node builders for modern Confluence Cloud nodes:
// Status (inline), InlineCard, BlockCard, EmbedCard, Layout, and PageProperties
// storage macro.
//
// All builders are pure functions — no I/O, no global state.
package adf

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ---------- Status (inline) ----------

// StatusColor represents valid Confluence status colors.
type StatusColor string

const (
	StatusGreen   StatusColor = "green"
	StatusYellow  StatusColor = "yellow"
	StatusRed     StatusColor = "red"
	StatusBlue    StatusColor = "blue"
	StatusPurple  StatusColor = "purple"
	StatusNeutral StatusColor = "neutral"
)

// Status builds an ADF status inline node.
//
//	{"type":"status","attrs":{"text":"...","color":"green","localId":"<uuid>","style":""}}
//
// localId should be a UUID string; pass "" to omit (Confluence will assign one
// on save). style is typically "".
func Status(text string, color StatusColor, localId string) Node {
	attrs := map[string]any{
		"text":  text,
		"color": string(color),
		"style": "",
	}
	if localId != "" {
		attrs["localId"] = localId
	}
	return Node{Type: "status", Attrs: attrs}
}

// ---------- Smart Links ----------

// InlineCard builds an ADF inlineCard node (renders as an inline smart link).
//
//	{"type":"inlineCard","attrs":{"url":"..."}}
func InlineCard(url string) Node {
	return Node{Type: "inlineCard", Attrs: map[string]any{"url": url}}
}

// BlockCard builds an ADF blockCard node (renders as a block-level smart link
// with preview card appearance).
//
//	{"type":"blockCard","attrs":{"url":"..."}}
func BlockCard(url string) Node {
	return Node{Type: "blockCard", Attrs: map[string]any{"url": url}}
}

// EmbedCard builds an ADF embedCard node. layout can be "wide", "center", etc.
//
//	{"type":"embedCard","attrs":{"url":"...","layout":"wide"}}
func EmbedCard(url, layout string) Node {
	if layout == "" {
		layout = "wide"
	}
	return Node{
		Type:  "embedCard",
		Attrs: map[string]any{"url": url, "layout": layout},
	}
}

// ---------- Layout ----------

// LayoutType represents a named Confluence two/three column layout preset.
type LayoutType string

const (
	LayoutSingle           LayoutType = "single"
	LayoutTwoEqual         LayoutType = "two_equal"
	LayoutTwoLeftSidebar   LayoutType = "two_left_sidebar"
	LayoutTwoRightSidebar  LayoutType = "two_right_sidebar"
	LayoutThreeEqual       LayoutType = "three_equal"
	LayoutThreeWithSidebars LayoutType = "three_with_sidebars"
)

// columnWidthsForLayout returns the column widths (percentages) for the known
// layout presets. Returns nil for unknown layouts (caller handles).
func columnWidthsForLayout(lt LayoutType) []float64 {
	switch lt {
	case LayoutSingle:
		return []float64{100}
	case LayoutTwoEqual:
		return []float64{50, 50}
	case LayoutTwoLeftSidebar:
		return []float64{33.33, 66.67}
	case LayoutTwoRightSidebar:
		return []float64{66.67, 33.33}
	case LayoutThreeEqual:
		return []float64{33.33, 33.33, 33.33}
	case LayoutThreeWithSidebars:
		return []float64{25, 50, 25}
	}
	return nil
}

// LayoutColumn builds a single layoutColumn node with the given percentage width.
// content should be block nodes (paragraphs, headings, etc.).
func LayoutColumn(widthPct float64, content ...Node) Node {
	return Node{
		Type:    "layoutColumn",
		Attrs:   map[string]any{"width": widthPct},
		Content: dropEmpty(content),
	}
}

// Layout builds a layoutSection with layoutColumn children automatically sized
// according to the layout preset. len(columns) must match the expected count for
// the given LayoutType, otherwise the function returns an error-paragraph.
//
// Each element of columns is the content of one column (slice of block nodes).
func Layout(lt LayoutType, columns ...[]Node) Node {
	widths := columnWidthsForLayout(lt)
	if widths == nil || len(columns) != len(widths) {
		// Fallback: equal-width columns for whatever was passed.
		n := len(columns)
		if n == 0 {
			n = 1
		}
		w := 100.0 / float64(n)
		widths = make([]float64, n)
		for i := range widths {
			widths[i] = w
		}
	}
	var cols []Node
	for i, colContent := range columns {
		cols = append(cols, LayoutColumn(widths[i], colContent...))
	}
	return Node{
		Type:    "layoutSection",
		Content: cols,
	}
}

// ---------- Page Properties macro (storage XML) ----------

// PagePropertiesEntry is one key-value row in a :::properties block.
type PagePropertiesEntry struct {
	Key   string
	Value string // may contain [[links]] syntax
}

// PagePropertiesToStorage converts a slice of key-value pairs into the
// Confluence storage XML for the page-properties macro. Link syntax:
//
//	[[titulo]]  →  <ac:link><ri:page ri:content-title="titulo"/></ac:link>
//	[[id:N]]    →  same (title must be resolved externally; here we embed the id
//	               as ri:content-title for callers that do their own lookup)
//
// The returned string is suitable for use as a storage body fragment.
func PagePropertiesToStorage(entries []PagePropertiesEntry) string {
	var sb strings.Builder
	// Confluence Cloud's Page Properties macro is stored with ac:name="details"
	// — "page-properties" is the legacy Server name and renders as "Unknown
	// macro" in Cloud. The corresponding aggregator macro is "detailssummary".
	sb.WriteString(`<ac:structured-macro ac:name="details" ac:schema-version="1">`)
	sb.WriteString(`<ac:rich-text-body><table><tbody>`)
	for _, e := range entries {
		sb.WriteString("<tr><th>")
		sb.WriteString(xmlEscape(e.Key))
		sb.WriteString("</th><td>")
		sb.WriteString(renderPropertiesValue(e.Value))
		sb.WriteString("</td></tr>")
	}
	sb.WriteString("</tbody></table></ac:rich-text-body></ac:structured-macro>")
	return sb.String()
}

// renderPropertiesValue converts a properties value string, turning [[link]]
// syntax into Confluence page-link storage XML and leaving plain text alone.
func renderPropertiesValue(val string) string {
	// Find all [[...]] occurrences and replace them.
	var out strings.Builder
	remaining := val
	for {
		start := strings.Index(remaining, "[[")
		if start == -1 {
			out.WriteString(xmlEscape(remaining))
			break
		}
		out.WriteString(xmlEscape(remaining[:start]))
		remaining = remaining[start+2:]
		end := strings.Index(remaining, "]]")
		if end == -1 {
			// Unterminated — treat as literal text
			out.WriteString("[[")
			out.WriteString(xmlEscape(remaining))
			break
		}
		inner := strings.TrimSpace(remaining[:end])
		remaining = remaining[end+2:]
		out.WriteString(confluencePageLink(inner))
	}
	return out.String()
}

// confluencePageLink builds a Confluence page link storage snippet.
// inner is either "titulo" or "id:12345".
func confluencePageLink(inner string) string {
	var title string
	if strings.HasPrefix(inner, "id:") {
		// For id-based references, ri:content-title is left empty;
		// callers that resolve IDs should pass the resolved title instead.
		// We encode it as-is so round-tripping is lossless.
		title = inner[3:] // the raw id; consumers can do a lookup
		// Use ri:page with a space-key-less reference by content-id.
		// Confluence also supports ri:page ri:content-id="..." but
		// content-title is more universally readable in diffs.
		return fmt.Sprintf(`<ac:link><ri:page ri:content-title="%s"/></ac:link>`, xmlEscapeAttr(title))
	}
	title = inner
	return fmt.Sprintf(`<ac:link><ri:page ri:content-title="%s"/></ac:link>`, xmlEscapeAttr(title))
}

// xmlEscape escapes & < > for XML text content.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// xmlEscapeAttr escapes & < > " for XML attribute values.
func xmlEscapeAttr(s string) string {
	s = xmlEscape(s)
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// ---------- ADF double-encoding helper ----------

// MarshalBodyValue returns the ADF doc as a JSON string suitable for use as
// the "value" field of a Confluence API body object. Confluence's API v2
// requires `body.value` to be a JSON-serialized string (double-encoded), NOT
// a nested JSON object.
func MarshalBodyValue(doc Node) (string, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal ADF body: %w", err)
	}
	return string(b), nil
}
