package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func runSearch(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		query  string
		rawCQL string
		space  string
		limit  int
		asJSON bool
	)

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--cql":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--cql requires a value")
				return exitInputErr, errInvalidUsage
			}
			rawCQL = remaining[i+1]
			i++
		case "--space":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--space requires a value")
				return exitInputErr, errInvalidUsage
			}
			space = remaining[i+1]
			i++
		case "--limit":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--limit requires a value")
				return exitInputErr, errInvalidUsage
			}
			n, sErr := strconv.Atoi(remaining[i+1])
			if sErr != nil || n < 1 || n > 250 {
				fmt.Fprintln(stderr, "--limit must be an integer between 1 and 250")
				return exitInputErr, errInvalidUsage
			}
			limit = n
			i++
		case "--json":
			asJSON = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "search — CQL search via the Confluence v1 search API. TSV output by default.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  confluence-docs search \"term\"                  # title or text match in default space")
			fmt.Fprintln(stdout, "  confluence-docs search --cql 'space=lybel AND label=\"adr\"'")
			fmt.Fprintln(stdout, "  confluence-docs search \"term\" --limit 5 --json")
			return exitOK, nil
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintln(stderr, "unknown flag:", a)
				return exitInputErr, errInvalidUsage
			}
			query = a
		}
	}

	if rawCQL == "" && query == "" {
		fmt.Fprintln(stderr, "search: provide a query term or --cql RAW")
		return exitInputErr, errInvalidUsage
	}
	if space == "" {
		// Default to the configured active space key.
		if key, keyErr := currentSpaceKey(); keyErr == nil {
			space = key
		}
	}
	if space == "" {
		fmt.Fprintln(stderr, "search: no space specified and no active space configured")
		fmt.Fprintln(stderr, "  use --space <key> or run `confluence-docs setup` / `space use <key>`")
		return exitInputErr, errInvalidUsage
	}
	if limit == 0 {
		limit = 10
	}

	cql := rawCQL
	if cql == "" {
		// CQL string-literals use double quotes; escape any in the query.
		safe := strings.ReplaceAll(query, `"`, `\"`)
		cql = fmt.Sprintf(`space = "%s" AND type = "page" AND (title ~ "%s" OR text ~ "%s")`,
			space, safe, safe)
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	results, err := client.SearchCQL(cql, limit)
	if err != nil {
		fmt.Fprintln(stderr, "search error:", err)
		return exitUnknownErr, err
	}

	if asJSON {
		out, _ := json.MarshalIndent(results, "", "  ")
		fmt.Fprintln(stdout, string(out))
		return exitOK, nil
	}

	if len(results) == 0 {
		fmt.Fprintln(stderr, "no results")
		return exitOK, nil
	}
	for _, r := range results {
		// TSV: id\ttitle\turl\texcerpt — newlines in excerpt already collapsed
		excerpt := r.Excerpt
		if len(excerpt) > 200 {
			excerpt = excerpt[:200] + "…"
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", r.PageID, r.Title, r.URL, excerpt)
	}
	return exitOK, nil
}

// runHome manages the local Home cache. Verbs: refresh (force GET), status
// (print metadata), show (print rendered text), query (search content),
// digest (print cached digest).
//
// The cache is read-only for navigation. Writes to the Home (or any page)
// always go through `page apply`, which always GETs fresh ADF before PUT —
// the cache is never the source of truth for an update.
