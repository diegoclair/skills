// Package release helps a CLI in a multi-skill monorepo (like
// `diegoclair/skills`) discover its own latest GitHub release without
// relying on `/releases/latest`.
//
// Why this package exists
//
// GitHub's `/releases/latest` redirect is a *singleton* per repo — it
// points at whichever release was published with `make_latest: true`.
// In a monorepo that ships multiple CLIs from distinct tag prefixes
// (e.g. `confluence-v*` and `jira-v*`), only one product can claim that
// "latest" pointer. Every other product's `update` command would
// resolve to the wrong tag.
//
// The fix is what every monorepo with this shape does (Projektor,
// Streamdal, release-please consumers): query the GitHub Releases
// API, filter the list by tag prefix on the client, take the newest
// match. GitHub returns releases newest-first by default, so the first
// match is always the latest of that product.
//
// Reference: https://github.com/orgs/community/discussions/5579
// (open since 2023, no native filter yet)
package release

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultTimeout is the HTTP timeout used by FindLatestByPrefix when the
// caller doesn't pass an explicit *http.Client.
const DefaultTimeout = 10 * time.Second

// FindLatestByPrefix returns the tag name of the most recent release on
// github.com/<repoOwnerRepo> whose tag starts with prefix.
//
// Example:
//
//	tag, err := release.FindLatestByPrefix("diegoclair/skills", "jira-v")
//	// tag == "jira-v0.1.0"
//
// httpClient is optional — pass nil to use a default client with a
// DefaultTimeout. Tests can inject a client whose transport returns
// canned responses.
//
// Errors:
//
//   - The HTTP call failed (network, DNS, etc.) — wrapped error.
//   - The API returned non-2xx — error includes status code + body snippet.
//   - The response body wasn't valid JSON — wrapped error.
//   - No release matched the prefix — sentinel-style error message
//     mentioning the repo + prefix so users see what to set if they
//     want to override.
//
// Rate limit: GitHub's unauthenticated REST API allows 60 requests/hour
// per IP. Plenty for `update` / `update --check` from a human, fine for
// occasional CI runs. CI matrices that hammer the API should pin the
// version explicitly via the per-CLI env var instead of resolving.
func FindLatestByPrefix(repoOwnerRepo, prefix string, httpClient *http.Client) (string, error) {
	if repoOwnerRepo == "" {
		return "", fmt.Errorf("release: repoOwnerRepo is required")
	}
	if prefix == "" {
		return "", fmt.Errorf("release: prefix is required (e.g. \"confluence-v\")")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=30", repoOwnerRepo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("release: build request: %w", err)
	}
	// Ask for the v3 response shape explicitly so the parser below is
	// stable even if GitHub flips defaults in the future.
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	// GitHub asks every script/CLI hitting the API to identify itself.
	// Skipping this gives 403 in some edge cases; setting it is free.
	req.Header.Set("User-Agent", "diegoclair-skills-release-resolver")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("release: GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("release: read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		// Special-case the most common 403: API rate limit. The body usually
		// contains the literal "API rate limit exceeded" — surface a
		// friendlier hint pointing at the env-var escape hatch.
		if resp.StatusCode == 403 && strings.Contains(string(body), "rate limit") {
			return "", fmt.Errorf("release: GitHub API rate limit hit (60 req/hour unauthenticated). Wait an hour or pin the version explicitly")
		}
		return "", fmt.Errorf("release: GitHub API %d for %s: %s", resp.StatusCode, repoOwnerRepo, snippet)
	}

	var releases []struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("release: parse JSON: %w", err)
	}

	for _, r := range releases {
		if r.Draft || r.Prerelease {
			continue
		}
		if strings.HasPrefix(r.TagName, prefix) {
			return r.TagName, nil
		}
	}

	return "", fmt.Errorf("release: no published release with tag prefix %q found on %s (checked latest %d)", prefix, repoOwnerRepo, len(releases))
}

// NormalizeVersion canonicalizes a tag/version string so two forms compare
// equal, regardless of mono-repo prefix or leading "v".
//
//   - "0.13.0"            → "0.13.0"
//   - "v0.13.0"           → "0.13.0"
//   - "confluence-v0.13.0" → "0.13.0"
//   - "jira-v0.1.0"        → "0.1.0"
//   - "dev"               → "dev"   (no v<digit> → untouched)
//   - "v0.3.0-3-g7f5e-dirty" → "0.3.0-3-g7f5e-dirty" (preserved)
//
// The rule: strip an optional "<prefix>-" only when it precedes a "v"
// that's immediately followed by a digit. Without that guard, strings
// like "dev" would be mangled (the loop would split at the inner "v").
//
// This is the function every CLI's `update` command needs after
// FindLatestByPrefix returns a prefixed tag like "jira-v0.1.0" — strip
// it to compare against the ldflags-stamped binary version which carries
// no prefix.
func NormalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	for i := 0; i < len(v)-1; i++ {
		if v[i] == 'v' && v[i+1] >= '0' && v[i+1] <= '9' {
			if i == 0 {
				break // no prefix to strip; fall through to TrimPrefix
			}
			if v[i-1] == '-' {
				v = v[i:]
				break
			}
		}
	}
	v = strings.TrimPrefix(v, "v")
	return v
}
