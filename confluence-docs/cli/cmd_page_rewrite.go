package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lybel-app/skills/confluence-docs/cli/adf"
)

func runPageRewrite(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		pageID       string
		markdownFile string
		strategy     = "headings"
		message      string
		dryRun       bool
		allowAdd     bool
		allowRemove  bool
	)

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--page-id":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--page-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			pageID = remaining[i+1]
			i++
		case "--markdown":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--markdown requires a file path")
				return exitInputErr, errInvalidUsage
			}
			markdownFile = remaining[i+1]
			i++
		case "--strategy":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--strategy requires a value")
				return exitInputErr, errInvalidUsage
			}
			strategy = remaining[i+1]
			i++
		case "--message":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--message requires a value")
				return exitInputErr, errInvalidUsage
			}
			message = remaining[i+1]
			i++
		case "--dry-run":
			dryRun = true
		case "--allow-add":
			allowAdd = true
		case "--allow-remove":
			allowRemove = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "page rewrite — replace matched sections of a page from a markdown file.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  --page-id ID           target page (required)")
			fmt.Fprintln(stdout, "  --markdown FILE        new content (required)")
			fmt.Fprintln(stdout, "  --strategy headings    (default; only value)")
			fmt.Fprintln(stdout, "  --message MSG          version comment")
			fmt.Fprintln(stdout, "  --dry-run              print proposed ops without writing")
			fmt.Fprintln(stdout, "  --allow-add            also append headings present in markdown but not in page")
			fmt.Fprintln(stdout, "  --allow-remove         also delete headings present in page but not in markdown")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page rewrite: --page-id is required")
		return exitInputErr, errInvalidUsage
	}
	if markdownFile == "" {
		fmt.Fprintln(stderr, "page rewrite: --markdown FILE is required")
		return exitInputErr, errInvalidUsage
	}
	if strategy != "headings" {
		fmt.Fprintf(stderr, "page rewrite: unknown strategy %q (only 'headings' supported)\n", strategy)
		return exitInputErr, errInvalidUsage
	}

	mdBytes, err := os.ReadFile(markdownFile)
	if err != nil {
		fmt.Fprintln(stderr, "reading markdown:", err)
		return exitInputErr, err
	}

	// Split markdown by headings; produce a list of (level, title, body) plus
	// a pre-heading intro slice. Body excludes the heading line itself.
	mdIntro, mdSections, err := splitMarkdownByHeadings(mdBytes)
	if err != nil {
		fmt.Fprintln(stderr, "split markdown:", err)
		return exitParseErr, err
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	// Fetch current page & its top-level headings.
	meta, err := client.GetPage(pageID, "atlas_doc_format")
	if err != nil {
		fmt.Fprintln(stderr, "fetching page:", err)
		return exitUnknownErr, err
	}
	if meta.Body.AtlasDocFormat.Value == "" {
		fmt.Fprintln(stderr, "page has no ADF body")
		return exitUnknownErr, fmt.Errorf("empty ADF body")
	}
	doc, err := adf.UnmarshalDoc([]byte(meta.Body.AtlasDocFormat.Value))
	if err != nil {
		fmt.Fprintln(stderr, "parse ADF:", err)
		return exitParseErr, err
	}

	// Collect (level, title) for each top-level heading in the page.
	type pageHeading struct {
		level int
		title string
	}
	var pageHeadings []pageHeading
	for _, n := range doc.Content {
		if n.Type == "heading" {
			pageHeadings = append(pageHeadings, pageHeading{
				level: headingLevelFromNode(n),
				title: strings.TrimSpace(allText(n)),
			})
		}
	}
	pageHeadingKey := func(h pageHeading) string {
		return fmt.Sprintf("%d:%s", h.level, h.title)
	}
	mdHeadingKey := func(level int, title string) string {
		return fmt.Sprintf("%d:%s", level, title)
	}
	pageHeadingSet := make(map[string]bool)
	for _, h := range pageHeadings {
		pageHeadingSet[pageHeadingKey(h)] = true
	}
	mdHeadingSet := make(map[string]bool)
	for _, s := range mdSections {
		mdHeadingSet[mdHeadingKey(s.level, s.title)] = true
	}

	// Build temp dir for fragment files.
	tmpDir, err := os.MkdirTemp("", "lybel-rewrite-")
	if err != nil {
		fmt.Fprintln(stderr, "tempdir:", err)
		return exitUnknownErr, err
	}
	defer os.RemoveAll(tmpDir)

	writeFrag := func(name, content string) (string, error) {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return "", err
		}
		return path, nil
	}

	// Build ops + reporting lines.
	var ops []multiOp
	var report []rewriteReportLine
	wouldAdd, wouldRemove := 0, 0

	// 1. Intro (markdown pre-heading content)
	if strings.TrimSpace(mdIntro) != "" {
		path, err := writeFrag("intro.md", mdIntro)
		if err != nil {
			fmt.Fprintln(stderr, "writing intro frag:", err)
			return exitUnknownErr, err
		}
		ops = append(ops, multiOp{Kind: "replace-intro", Fragment: path})
		report = append(report, rewriteReportLine{"✓", "replaced intro"})
	}

	// 2. Walk markdown sections in order, but emit ops only for *outermost*
	//    sections — sections at the shallowest level present in the markdown.
	//    Each outermost section's fragment bundles all of its deeper-level
	//    descendants (sub-headings + their bodies) until the next same-or-
	//    shallower-level heading.
	//
	//    Why: replace-section uses ADF section bounds, which include the
	//    heading + everything until the next heading of equal-or-higher
	//    level. So replace-section on an h2 wipes the h2 and all its h3
	//    children. If we emit a separate replace-section for each h3, the
	//    parent's op runs first and removes those h3s; subsequent h3 ops
	//    fail with "section not found". Bundling fixes this.
	bundles := bundleOutermostSections(mdSections)
	mdOutermostLevel := 0
	if len(bundles) > 0 {
		mdOutermostLevel = bundles[0].level
	}

	for i, b := range bundles {
		fragName := fmt.Sprintf("section-%d.md", i)
		path, err := writeFrag(fragName, b.body)
		if err != nil {
			fmt.Fprintln(stderr, "writing section frag:", err)
			return exitUnknownErr, err
		}
		key := mdHeadingKey(b.level, b.title)
		if pageHeadingSet[key] {
			ops = append(ops, multiOp{
				Kind:     "replace-section",
				Heading:  b.title,
				AtLevel:  b.level,
				Fragment: path,
			})
			report = append(report, rewriteReportLine{"✓",
				fmt.Sprintf("replaced section %q (h%d)", b.title, b.level)})
		} else {
			if allowAdd {
				ops = append(ops, multiOp{Kind: "append", Fragment: path})
				report = append(report, rewriteReportLine{"+",
					fmt.Sprintf("added section %q (h%d) at end", b.title, b.level)})
			} else {
				report = append(report, rewriteReportLine{"⚠",
					fmt.Sprintf("would add: %q (h%d) [pass --allow-add to apply]", b.title, b.level)})
				wouldAdd++
			}
		}
	}

	// 3. Headings in page but NOT in markdown → would-remove (or remove if flag).
	//    Only consider page headings at the *outermost* markdown level. Deeper
	//    page headings inside a section being replaced are removed implicitly
	//    when the parent's replace-section op runs (its fragment doesn't
	//    contain them). Emitting an explicit delete-section for a child
	//    heading would fail because the heading is already gone after the
	//    parent's replace.
	for _, h := range pageHeadings {
		if h.level != mdOutermostLevel {
			continue
		}
		key := pageHeadingKey(h)
		if mdHeadingSet[key] {
			continue
		}
		if allowRemove {
			ops = append(ops, multiOp{
				Kind:    "delete-section",
				Heading: h.title,
				AtLevel: h.level,
			})
			report = append(report, rewriteReportLine{"-",
				fmt.Sprintf("removed section %q (h%d)", h.title, h.level)})
		} else {
			report = append(report, rewriteReportLine{"⚠",
				fmt.Sprintf("would remove: %q (h%d) [pass --allow-remove to apply]", h.title, h.level)})
			wouldRemove++
		}
	}

	if len(ops) == 0 {
		fmt.Fprintln(stderr, "page rewrite: no matching headings — nothing to do")
		for _, r := range report {
			fmt.Fprintf(stderr, "  %s %s\n", r.mark, r.text)
		}
		return exitOK, nil
	}

	// Pre-load fragments for the multi-apply.
	fragments := make([][]adf.Node, len(ops))
	for i, op := range ops {
		if err := validateMultiOp(op); err != nil {
			fmt.Fprintf(stderr, "internal: built invalid op %d (%s): %v\n", i+1, op.Kind, err)
			return exitUnknownErr, err
		}
		frag, err := loadMultiFragment(op)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitInputErr, err
		}
		fragments[i] = frag
	}

	if dryRun {
		fmt.Fprintf(stdout, "Rewriting page %s (v%d → v%d) [dry-run]\n",
			pageID, meta.Version.Number, meta.Version.Number+1)
		for _, r := range report {
			fmt.Fprintf(stdout, "  %s %s\n", r.mark, r.text)
		}
		fmt.Fprintf(stdout, "%d sections replaced, %d add skipped, %d remove skipped\n",
			countReports(report, "✓"), wouldAdd, wouldRemove)
		return exitOK, nil
	}

	fromV, toV, title, applied, err := runMultiApply(client, pageID, message, ops, fragments, false, stdout, stderr)
	if err != nil {
		return exitInputErr, err
	}
	refreshHomeCacheAfterWrite(pageID, client, stderr)

	fmt.Fprintf(stdout, "Rewriting page %s (v%d → v%d)\n", pageID, fromV, toV)
	for _, r := range report {
		fmt.Fprintf(stdout, "  %s %s\n", r.mark, r.text)
	}
	fmt.Fprintf(stdout, "%d sections replaced, %d add skipped, %d remove skipped\n",
		countReports(report, "✓"), wouldAdd, wouldRemove)
	url := pageWebURL(client, pageID)
	fmt.Fprintf(stdout, "URL: %s\n", url)
	_ = title
	_ = applied
	return exitOK, nil
}

// rewriteReportLine is a single line of human output for `page rewrite`.
type rewriteReportLine struct {
	mark string // "✓", "+", "-", "⚠"
	text string
}

// countReports counts report lines whose mark equals m.
func countReports(rs []rewriteReportLine, m string) int {
	n := 0
	for _, r := range rs {
		if r.mark == m {
			n++
		}
	}
	return n
}

// sectionBundle represents an outermost markdown section with all of its
// deeper-level descendants serialized into a single fragment body.
//
// `runPageRewrite` emits one op per bundle (rather than one per mdSection)
// so that nested h3+ sections inside an h2 don't lose to the parent's
// `replace-section` op (which would wipe them by ADF section bounds).
type sectionBundle struct {
	level int    // shallowest (outermost) level in the markdown
	title string // heading title at that outermost level
	body  string // serialized fragment: heading + body + nested children
}

// bundleOutermostSections groups flat-list mdSections into bundles where
// each bundle is one outermost section + all of its deeper-level descendants
// (until the next same-or-shallower-level heading).
//
// "Outermost level" = shallowest level present in mdSections. All sections
// at that level become bundle roots; deeper-level sections are absorbed
// into the preceding root's body. If the markdown has only h3+ sections
// (no h2), the shallowest level present is used and those become roots.
func bundleOutermostSections(sections []mdSection) []sectionBundle {
	if len(sections) == 0 {
		return nil
	}
	outermost := sections[0].level
	for _, s := range sections {
		if s.level < outermost {
			outermost = s.level
		}
	}
	var bundles []sectionBundle
	i := 0
	for i < len(sections) {
		s := sections[i]
		if s.level != outermost {
			// Orphan deeper-level section before any outermost root —
			// treat as its own root (same effect as bundle of one).
			bundles = append(bundles, sectionBundle{level: s.level, title: s.title, body: s.fullText()})
			i++
			continue
		}
		var parts []string
		parts = append(parts, s.fullText())
		j := i + 1
		for j < len(sections) && sections[j].level > s.level {
			parts = append(parts, sections[j].fullText())
			j++
		}
		bundles = append(bundles, sectionBundle{
			level: s.level,
			title: s.title,
			body:  strings.Join(parts, ""),
		})
		i = j
	}
	return bundles
}

// mdSection is a markdown section parsed by splitMarkdownByHeadings.
