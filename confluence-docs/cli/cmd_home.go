package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/lybel-app/skills/confluence-docs/cli/adf"
)

func runHome(args []string, stdout, stderr io.Writer) (int, error) {
	// Default TTL: how long the cache is "fresh enough" without re-fetching.
	// Read paths (--query/--show/--digest) auto-refresh if the cache is older
	// than this; --refresh always fetches regardless. Cross-session staleness
	// is bounded by this value.
	const defaultMaxAge = 1 * time.Hour

	var (
		refresh    bool
		status     bool
		show       bool
		showDigest bool
		query      string
		maxAge     time.Duration = defaultMaxAge
		pageID     string        // set below from config
	)

	// Default to configured home page ID (can be overridden by --page-id flag below).
	if id, cfgErr := currentHomePageID(); cfgErr == nil {
		pageID = id
	}

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--refresh":
			refresh = true
		case "--status":
			status = true
		case "--show":
			show = true
		case "--digest":
			showDigest = true
		case "--query":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--query requires a value")
				return exitInputErr, errInvalidUsage
			}
			query = remaining[i+1]
			i++
		case "--max-age":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--max-age requires a duration (e.g. 1h, 30m)")
				return exitInputErr, errInvalidUsage
			}
			d, dErr := time.ParseDuration(remaining[i+1])
			if dErr != nil {
				fmt.Fprintln(stderr, "--max-age:", dErr)
				return exitInputErr, errInvalidUsage
			}
			maxAge = d
			i++
		case "--page-id":
			// Allow override for advanced users / testing
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--page-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			pageID = remaining[i+1]
			i++
		case "-h", "--help":
			fmt.Fprintln(stdout, "home — Lybel Confluence Home page cache.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  confluence-docs home --refresh             # always fetch + overwrite cache")
			fmt.Fprintln(stdout, "  confluence-docs home --status              # show cache metadata (read-only)")
			fmt.Fprintln(stdout, "  confluence-docs home --show                # print cached text")
			fmt.Fprintln(stdout, "  confluence-docs home --query \"advisor\"     # grep cached content")
			fmt.Fprintln(stdout, "  confluence-docs home --digest              # print cached page digest")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Auto-refresh rules:")
			fmt.Fprintln(stdout, "  --query/--show/--digest auto-refresh when the cache is missing OR")
			fmt.Fprintln(stdout, "  older than --max-age (default 1h). Callers don't need to think about it.")
			fmt.Fprintln(stdout, "  --refresh ALWAYS fetches, ignoring TTL — use it after another machine")
			fmt.Fprintln(stdout, "  edited the Home and you want immediate sync.")
			fmt.Fprintln(stdout, "  Writes to the Home (page apply, index add/remove/sync) auto-refresh")
			fmt.Fprintln(stdout, "  the cache after the PUT succeeds.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "The cache is read-only for navigation: writes always GET fresh ADF")
			fmt.Fprintln(stdout, "before PUT (atomic), so the cache is never used as the source for an update.")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	// Default action: if no verb given, --status
	if !refresh && !status && !show && !showDigest && query == "" {
		status = true
	}

	// --refresh: always fetch, never short-circuit on cache freshness.
	// The whole point of an explicit --refresh is "I know I want fresh data".
	if refresh {
		if pageID == "" {
			fmt.Fprintln(stderr, "home: no home page configured — run `confluence-docs setup` or `confluence-docs space use <key>`")
			return exitInputErr, errInvalidUsage
		}
		client, ok := buildClient(cloud, email, token, stderr)
		if !ok {
			return exitUnknownErr, nil
		}
		c, fErr := fetchHomeCache(client, pageID)
		if fErr != nil {
			fmt.Fprintln(stderr, "refresh failed:", fErr)
			return exitUnknownErr, fErr
		}
		if sErr := adf.SaveHomeCache(c); sErr != nil {
			fmt.Fprintln(stderr, "saving cache:", sErr)
			return exitUnknownErr, sErr
		}
		path, _ := adf.HomeCachePath()
		fmt.Fprintf(stdout, "ok — %s (cached at %s)\n", c.FormatStatus(), path)
		return exitOK, nil
	}

	// Read paths: load cache; auto-refresh if missing or stale (TTL).
	// --status is a read-only metadata check and should NOT auto-refresh.
	cache, loadErr := adf.LoadHomeCache()
	needRefresh := false
	if loadErr != nil {
		if os.IsNotExist(loadErr) {
			needRefresh = (show || showDigest || query != "")
			if !needRefresh {
				fmt.Fprintln(stderr, "no home cache. Run: confluence-docs home --refresh")
				return exitUnknownErr, loadErr
			}
		} else {
			fmt.Fprintln(stderr, "loading cache:", loadErr)
			return exitUnknownErr, loadErr
		}
	} else if !status && cache.IsStale(maxAge) {
		needRefresh = true
	}

	if needRefresh {
		if pageID == "" {
			fmt.Fprintln(stderr, "home: no home page configured — run `confluence-docs setup` or `confluence-docs space use <key>`")
			return exitInputErr, errInvalidUsage
		}
		why := "missing"
		if cache != nil {
			why = fmt.Sprintf("stale by %s", formatDurationCompact(cache.Age()))
		}
		fmt.Fprintf(stderr, "(home cache auto-refreshed: %s)\n", why)
		client, ok := buildClient(cloud, email, token, stderr)
		if !ok {
			return exitUnknownErr, nil
		}
		c, fErr := fetchHomeCache(client, pageID)
		if fErr != nil {
			fmt.Fprintln(stderr, "auto-refresh failed:", fErr)
			return exitUnknownErr, fErr
		}
		if sErr := adf.SaveHomeCache(c); sErr != nil {
			fmt.Fprintln(stderr, "saving cache:", sErr)
			return exitUnknownErr, sErr
		}
		cache = c
	}

	if status {
		fmt.Fprintln(stdout, cache.FormatStatus())
		path, _ := adf.HomeCachePath()
		fmt.Fprintf(stdout, "  path: %s\n", path)
		fmt.Fprintf(stdout, "  url:  %s\n", cache.URL)
		fmt.Fprintf(stdout, "  size: %d bytes (text content)\n", len(cache.TextContent))
		return exitOK, nil
	}
	if showDigest {
		fmt.Fprint(stdout, cache.Digest.FormatText())
		return exitOK, nil
	}
	if show {
		fmt.Fprint(stdout, cache.TextContent)
		return exitOK, nil
	}
	if query != "" {
		matches := grepHome(cache.TextContent, query)
		if len(matches) == 0 {
			fmt.Fprintf(stderr, "no matches for %q in cached home (v%d)\n", query, cache.Version)
			return exitOK, nil
		}
		for _, m := range matches {
			if m.heading != "" {
				fmt.Fprintf(stdout, "## %s\n", m.heading)
			}
			fmt.Fprintf(stdout, "  %s\n", m.line)
		}
		return exitOK, nil
	}

	return exitOK, nil
}

// fetchHomeCache fetches the Home page and builds a HomeCache (digest + text
// rendering) without writing to disk. Caller is responsible for SaveHomeCache.
func fetchHomeCache(client *adf.ConfluenceClient, pageID string) (*adf.HomeCache, error) {
	meta, err := client.GetPage(pageID, "atlas_doc_format")
	if err != nil {
		return nil, fmt.Errorf("get home page: %w", err)
	}
	if meta.Body.AtlasDocFormat.Value == "" {
		return nil, fmt.Errorf("home page has no ADF body")
	}
	doc, err := adf.UnmarshalDoc([]byte(meta.Body.AtlasDocFormat.Value))
	if err != nil {
		return nil, fmt.Errorf("parse home ADF: %w", err)
	}
	url := client.PageURL(meta.Links.WebUI)
	digest := adf.BuildDigest(doc, meta.ID, meta.Title, url, meta.Version.Number)
	textContent := adf.RenderText(doc)
	return &adf.HomeCache{
		FetchedAt:   time.Now().UTC(),
		PageID:      meta.ID,
		Title:       meta.Title,
		Version:     meta.Version.Number,
		URL:         url,
		TextContent: textContent,
		Digest:      digest,
	}, nil
}

// refreshHomeCacheAfterWrite re-fetches and saves the Home cache when a write
// operation has just modified the Home page. This is the auto-refresh-on-write
// path: the caller's session sees the new state immediately, without needing
// an explicit `home --refresh`.
//
// No-op when pageID isn't the configured Home (the only page we cache today).
// Errors are reported to stderr but don't fail the calling write — the write
// itself already succeeded; cache freshness is best-effort.
func refreshHomeCacheAfterWrite(pageID string, client *adf.ConfluenceClient, stderr io.Writer) {
	homeID, err := currentHomePageID()
	if err != nil || pageID != homeID {
		return
	}
	c, err := fetchHomeCache(client, homeID)
	if err != nil {
		fmt.Fprintf(stderr, "(warning: home cache refresh after write failed: %v)\n", err)
		return
	}
	if err := adf.SaveHomeCache(c); err != nil {
		fmt.Fprintf(stderr, "(warning: saving refreshed home cache failed: %v)\n", err)
		return
	}
	fmt.Fprintf(stderr, "(home cache auto-refreshed: v%d)\n", c.Version)
}

// homeMatch is a single hit from grepHome.
type homeMatch struct {
	heading string // closest preceding heading, "" if none
	line    string // the matched line, trimmed
}

// grepHome does a case-insensitive substring search over the cached Home text
// content. Each match carries the closest preceding heading as section context,
// so the LLM caller can see where it lives.
//
// Headings are detected by the markdown-ish output of adf.RenderText (lines
// starting with `# `, `## `, etc.). A 30-line break between matches with the
// same heading collapses them into a single bullet group.
func grepHome(content, query string) []homeMatch {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var matches []homeMatch
	currentHeading := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isMarkdownHeading(trimmed) {
			currentHeading = trimHashPrefix(trimmed)
			continue
		}
		if strings.Contains(strings.ToLower(trimmed), q) {
			matches = append(matches, homeMatch{heading: currentHeading, line: trimmed})
		}
	}
	// Dedupe: collapse consecutive matches under the same heading.
	if len(matches) <= 1 {
		return matches
	}
	out := make([]homeMatch, 0, len(matches))
	for i, m := range matches {
		if i > 0 && m.heading == matches[i-1].heading {
			out = append(out, homeMatch{heading: "", line: m.line})
			continue
		}
		out = append(out, m)
	}
	return out
}

func isMarkdownHeading(line string) bool {
	if len(line) < 2 || line[0] != '#' {
		return false
	}
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	return i >= 1 && i <= 6 && i < len(line) && line[i] == ' '
}

func trimHashPrefix(line string) string {
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	return strings.TrimSpace(line[i:])
}

// formatDurationCompact: 5s, 12m, 3h, 2d
func formatDurationCompact(d time.Duration) string {
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	if m < 60 {
		return fmt.Sprintf("%dm", m)
	}
	h := m / 60
	if h < 24 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dd", h/24)
}

// runLint validates an ADF file and prints findings.
