package main

import (
	"strings"

	"github.com/diegoclair/skills/pkg/atlassian/adf"
)

type mdSection struct {
	level    int    // 1..6
	title    string // heading text (trimmed)
	headLine string // the original heading line, e.g. "## Foo"
	body     string // body text after the heading, may contain blank lines
}

// fullText returns the section serialized back to markdown including its
// heading line, suitable for writing as a fragment file.
func (s mdSection) fullText() string {
	if s.body == "" {
		return s.headLine + "\n"
	}
	return s.headLine + "\n" + s.body
}

// splitMarkdownByHeadings parses raw markdown bytes and returns:
//   - intro: text before the first heading line (may be empty)
//   - sections: one entry per heading, in document order
//
// Headings are detected as ATX-style lines beginning with 1-6 '#' chars
// followed by a space. Setext headings (=== / ---) are NOT supported.
func splitMarkdownByHeadings(src []byte) (intro string, sections []mdSection, err error) {
	lines := strings.Split(string(src), "\n")
	type pending struct {
		level    int
		title    string
		headLine string
		bodyBuf  []string
	}
	var introBuf []string
	var cur *pending
	flush := func() {
		if cur == nil {
			return
		}
		body := strings.Join(cur.bodyBuf, "\n")
		// Trim trailing blank lines from body so fragments are tidy, but
		// preserve a trailing newline.
		body = strings.TrimRight(body, "\n") + "\n"
		if strings.TrimSpace(body) == "" {
			body = ""
		}
		sections = append(sections, mdSection{
			level:    cur.level,
			title:    cur.title,
			headLine: cur.headLine,
			body:     body,
		})
		cur = nil
	}
	for _, line := range lines {
		level, title, ok := parseATXHeading(line)
		if ok {
			flush()
			cur = &pending{
				level:    level,
				title:    title,
				headLine: line,
			}
			continue
		}
		if cur == nil {
			introBuf = append(introBuf, line)
		} else {
			cur.bodyBuf = append(cur.bodyBuf, line)
		}
	}
	flush()
	intro = strings.Join(introBuf, "\n")
	intro = strings.TrimRight(intro, "\n")
	if strings.TrimSpace(intro) != "" {
		intro += "\n"
	} else {
		intro = ""
	}
	return intro, sections, nil
}

// parseATXHeading returns (level, trimmedTitle, true) if line is an ATX
// heading like "## Foo". Otherwise returns (0, "", false).
func parseATXHeading(line string) (int, string, bool) {
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i == 0 || i > 6 {
		return 0, "", false
	}
	if i >= len(line) || line[i] != ' ' {
		return 0, "", false
	}
	title := strings.TrimSpace(line[i+1:])
	// Strip optional trailing # tokens (e.g. "## Foo ##").
	title = strings.TrimRight(title, "#")
	title = strings.TrimSpace(title)
	if title == "" {
		return 0, "", false
	}
	return i, title, true
}

// headingLevelFromNode is a thin wrapper to expose heading level for the
// section-list error message in runPageApply. Mirrors adf.headingLevel.
func headingLevelFromNode(n adf.Node) int {
	if n.Attrs == nil {
		return 1
	}
	switch v := n.Attrs["level"].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 1
}

// allText collects the inline text of a node tree (for printing heading text).
func allText(n adf.Node) string {
	var sb strings.Builder
	collectAllText(n, &sb)
	return sb.String()
}

func collectAllText(n adf.Node, sb *strings.Builder) {
	if n.Text != "" {
		sb.WriteString(n.Text)
	}
	for _, c := range n.Content {
		collectAllText(c, sb)
	}
}

// runSearch runs a CQL query against Confluence and prints results as TSV
// (pageId\ttitle\turl\texcerpt). Defaults the space filter to `lybel`.
