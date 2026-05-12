// cmd_check.go — `confluence-docs check` subcommand.
//
// Performs a fuzzy title search in the Confluence space before creating a new
// page, so the agent can detect duplicates early.
//
// Usage:
//
//	confluence-docs check --title "Análise Stripe Brasil" [--type reference] [--tags psp,concorrente] [--threshold 0.4] [--json]
//
// Output (JSON):
//
//	{
//	  "exists": false,
//	  "similar": [
//	    {"id": "123", "title": "Análise Stripe", "url": "https://...", "similarity": 0.82}
//	  ],
//	  "suggestion": "create" | "update_existing"
//	}
//
// The suggestion field is "update_existing" when any result's similarity score
// is above the threshold (default 0.4), otherwise "create".
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"
)

// checkResult is the JSON output of `confluence-docs check`.
type checkResult struct {
	Exists     bool            `json:"exists"`
	Similar    []checkSimilar  `json:"similar"`
	Suggestion string          `json:"suggestion"` // "create" | "update_existing"
}

// checkSimilar is one entry in the similar list.
type checkSimilar struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	Similarity float64 `json:"similarity_score"`
}

// runCheck implements `confluence-docs check`.
func runCheck(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		title     string
		docType   string
		tags      string
		threshold float64 = 0.4
		space     string
		limit     int    = 20
		asJSON    bool   = true // default output is JSON for machine consumption
	)

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--title":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--title requires a value")
				return exitInputErr, errInvalidUsage
			}
			title = remaining[i+1]
			i++
		case "--type":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--type requires a value")
				return exitInputErr, errInvalidUsage
			}
			docType = remaining[i+1]
			i++
		case "--tags":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--tags requires a value")
				return exitInputErr, errInvalidUsage
			}
			tags = remaining[i+1]
			i++
		case "--threshold":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--threshold requires a value (0.0-1.0)")
				return exitInputErr, errInvalidUsage
			}
			var t float64
			if _, scanErr := fmt.Sscanf(remaining[i+1], "%f", &t); scanErr != nil || t < 0 || t > 1 {
				fmt.Fprintln(stderr, "--threshold must be a float between 0 and 1")
				return exitInputErr, errInvalidUsage
			}
			threshold = t
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
			var n int
			if _, scanErr := fmt.Sscanf(remaining[i+1], "%d", &n); scanErr != nil || n < 1 {
				fmt.Fprintln(stderr, "--limit must be a positive integer")
				return exitInputErr, errInvalidUsage
			}
			limit = n
			i++
		case "--json":
			asJSON = true
		case "--text":
			asJSON = false
		case "-h", "--help":
			fmt.Fprintln(stdout, "check — fuzzy title search before creating a page.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  confluence-docs check --title \"Análise Stripe Brasil\"")
			fmt.Fprintln(stdout, "  confluence-docs check --title \"...\" --type reference --tags psp,concorrente")
			fmt.Fprintln(stdout, "  confluence-docs check --title \"...\" --threshold 0.8")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Output (JSON):")
			fmt.Fprintln(stdout, "  { exists, similar: [{id, title, url, similarity_score}], suggestion: create|update_existing }")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Options:")
			fmt.Fprintln(stdout, "  --title TEXT        title to search (required)")
			fmt.Fprintln(stdout, "  --type TYPE         doc type label (reference, decision, etc.) to filter by")
			fmt.Fprintln(stdout, "  --tags TAGS         comma-separated label names to filter by")
			fmt.Fprintln(stdout, "  --threshold FLOAT   similarity threshold for suggestion (default 0.4)")
			fmt.Fprintln(stdout, "  --space KEY         Confluence space key (default: from $ATLASSIAN_CLOUD or credentials config)")
			fmt.Fprintln(stdout, "  --limit N           max candidates to fetch (default: 20)")
			fmt.Fprintln(stdout, "  --text              plain-text output instead of JSON")
			return exitOK, nil
		default:
			// Accept title as a positional argument (for convenience).
			if !strings.HasPrefix(a, "-") && title == "" {
				title = a
				continue
			}
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if title == "" {
		fmt.Fprintln(stderr, "check: --title is required")
		return exitInputErr, errInvalidUsage
	}
	if space == "" {
		space = defaultCloud
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	// Build CQL: title fuzzy match, optionally filtered by label.
	safe := strings.ReplaceAll(title, `"`, `\"`)
	var cqlParts []string
	cqlParts = append(cqlParts, fmt.Sprintf(`space = "%s"`, space))
	cqlParts = append(cqlParts, `type = "page"`)
	cqlParts = append(cqlParts, fmt.Sprintf(`title ~ "%s"`, safe))
	if docType != "" {
		cqlParts = append(cqlParts, fmt.Sprintf(`label = "tipo:%s"`, strings.ReplaceAll(docType, `"`, `\"`)))
	}
	for _, tag := range splitTags(tags) {
		cqlParts = append(cqlParts, fmt.Sprintf(`label = "%s"`, strings.ReplaceAll(tag, `"`, `\"`)))
	}
	cql := strings.Join(cqlParts, " AND ")

	results, searchErr := client.SearchCQL(cql, limit)
	if searchErr != nil {
		// Fallback: try without label filters if they were applied
		if docType != "" || tags != "" {
			cql2 := fmt.Sprintf(`space = "%s" AND type = "page" AND title ~ "%s"`, space, safe)
			results, searchErr = client.SearchCQL(cql2, limit)
		}
		if searchErr != nil {
			fmt.Fprintln(stderr, "search error:", searchErr)
			return exitUnknownErr, searchErr
		}
	}

	// Compute similarity scores.
	var similar []checkSimilar
	titleLower := strings.ToLower(title)
	for _, r := range results {
		score := titleSimilarity(titleLower, strings.ToLower(r.Title))
		if score > 0.3 { // include anything above a low floor for listing
			similar = append(similar, checkSimilar{
				ID:         r.PageID,
				Title:      r.Title,
				URL:        r.URL,
				Similarity: math.Round(score*100) / 100,
			})
		}
	}

	// Sort by similarity descending (simple insertion sort — N is small).
	for i := 1; i < len(similar); i++ {
		for j := i; j > 0 && similar[j].Similarity > similar[j-1].Similarity; j-- {
			similar[j], similar[j-1] = similar[j-1], similar[j]
		}
	}

	// Determine exists (exact title match, case-insensitive) and suggestion.
	exists := false
	for _, s := range similar {
		if strings.EqualFold(s.Title, title) {
			exists = true
			break
		}
	}
	suggestion := "create"
	if exists {
		suggestion = "update_existing"
	} else {
		for _, s := range similar {
			if s.Similarity >= threshold {
				suggestion = "update_existing"
				break
			}
		}
	}

	res := checkResult{
		Exists:     exists,
		Similar:    similar,
		Suggestion: suggestion,
	}
	if similar == nil {
		res.Similar = []checkSimilar{}
	}

	if asJSON {
		out, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(stdout, string(out))
	} else {
		// Plain text output
		fmt.Fprintf(stdout, "Exists: %v\n", res.Exists)
		fmt.Fprintf(stdout, "Suggestion: %s\n", res.Suggestion)
		if len(res.Similar) == 0 {
			fmt.Fprintln(stdout, "Similar: (none)")
		} else {
			fmt.Fprintln(stdout, "Similar pages:")
			for _, s := range res.Similar {
				fmt.Fprintf(stdout, "  %.2f  %s  %s\n", s.Similarity, s.Title, s.URL)
			}
		}
	}
	return exitOK, nil
}

// splitTags splits a comma-separated tag list, trimming whitespace.
func splitTags(tags string) []string {
	if tags == "" {
		return nil
	}
	var out []string
	for _, t := range strings.Split(tags, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// titleSimilarity returns a [0,1] similarity score between two lowercase title
// strings. Uses a trigram-based Jaccard similarity coefficient, which is fast
// and reasonably accurate for short strings.
//
// Falls back to Levenshtein-normalised distance for very short strings (< 3
// chars) where trigrams are unreliable.
func titleSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}
	ra, rb := []rune(a), []rune(b)
	if len(ra) < 3 || len(rb) < 3 {
		return levenshteinSimilarity(a, b)
	}
	setA := trigramSet(a)
	setB := trigramSet(b)
	return jaccardSimilarity(setA, setB)
}

// trigramSet builds the set of character trigrams for a string.
func trigramSet(s string) map[string]struct{} {
	runes := []rune(strings.ToLower(s))
	set := make(map[string]struct{}, len(runes))
	for i := 0; i+2 < len(runes); i++ {
		tg := string(runes[i : i+3])
		set[tg] = struct{}{}
	}
	return set
}

// jaccardSimilarity returns |A∩B| / |A∪B|.
func jaccardSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// levenshteinSimilarity returns a [0,1] score based on edit distance.
func levenshteinSimilarity(a, b string) float64 {
	dist := levenshtein(a, b)
	maxLen := utf8.RuneCountInString(a)
	if l := utf8.RuneCountInString(b); l > maxLen {
		maxLen = l
	}
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(dist)/float64(maxLen)
}

// levenshtein computes the edit distance between two strings (rune-based).
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	row := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		row[j] = j
	}
	for i := 1; i <= la; i++ {
		prev := row[0]
		row[0] = i
		for j := 1; j <= lb; j++ {
			old := row[j]
			if ra[i-1] == rb[j-1] {
				row[j] = prev
			} else {
				m := row[j-1]
				if old < m {
					m = old
				}
				if prev < m {
					m = prev
				}
				row[j] = m + 1
			}
			prev = old
		}
	}
	return row[lb]
}
