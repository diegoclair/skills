package release

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripperFunc adapts a function into an http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// mockClient returns an *http.Client whose transport returns the given
// status code and body for every request.
func mockClient(statusCode int, body string) *http.Client {
	return &http.Client{
		Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
}

func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// Already clean
		{"0.13.0", "0.13.0"},
		// Plain v-prefix
		{"v0.13.0", "0.13.0"},
		{" v1.0.0 ", "1.0.0"},
		// Mono-repo prefixes (the whole point of this package)
		{"confluence-v0.13.0", "0.13.0"},
		{"jira-v0.1.0", "0.1.0"},
		{"some-other-skill-v1.2.3", "1.2.3"},
		// Strings that contain 'v' but not as a version marker — must be untouched
		{"dev", "dev"},
		// Dev-build markers from `git describe --tags --dirty`
		{"v0.3.0-3-g7f5e-dirty", "0.3.0-3-g7f5e-dirty"},
		{"confluence-v0.13.0-2-gabcd-dirty", "0.13.0-2-gabcd-dirty"},
		// Edge cases
		{"", ""},
		{"v", ""},                       // bare "v" — TrimPrefix strips it; degenerate input
		{"vNotAVersion", "NotAVersion"}, // "vN..." starts with v but N isn't a digit; TrimPrefix strips the leading v
	}
	for _, c := range cases {
		got := NormalizeVersion(c.in)
		if got != c.want {
			t.Errorf("NormalizeVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFindLatestByPrefix_HappyPath(t *testing.T) {
	// Two products in the same repo. confluence is the "latest" (make_latest:true);
	// jira ships a sibling tag. The API returns them newest-first.
	body := `[
		{"tag_name":"jira-v0.2.0","draft":false,"prerelease":false},
		{"tag_name":"confluence-v0.13.0","draft":false,"prerelease":false},
		{"tag_name":"jira-v0.1.0","draft":false,"prerelease":false},
		{"tag_name":"confluence-v0.12.2","draft":false,"prerelease":false}
	]`
	client := mockClient(http.StatusOK, body)

	got, err := FindLatestByPrefix("diegoclair/skills", "jira-v", client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "jira-v0.2.0" {
		t.Errorf("FindLatestByPrefix jira-v = %q, want jira-v0.2.0", got)
	}

	got, err = FindLatestByPrefix("diegoclair/skills", "confluence-v", client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "confluence-v0.13.0" {
		t.Errorf("FindLatestByPrefix confluence-v = %q, want confluence-v0.13.0", got)
	}
}

func TestFindLatestByPrefix_SkipsDraftAndPrerelease(t *testing.T) {
	// A newer draft and a prerelease should both be skipped in favor of
	// the older stable.
	body := `[
		{"tag_name":"jira-v0.3.0","draft":true,"prerelease":false},
		{"tag_name":"jira-v0.2.0-rc1","draft":false,"prerelease":true},
		{"tag_name":"jira-v0.1.0","draft":false,"prerelease":false}
	]`
	client := mockClient(http.StatusOK, body)

	got, err := FindLatestByPrefix("diegoclair/skills", "jira-v", client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "jira-v0.1.0" {
		t.Errorf("got %q, want jira-v0.1.0 (drafts/prereleases must be skipped)", got)
	}
}

func TestFindLatestByPrefix_NoMatch(t *testing.T) {
	body := `[
		{"tag_name":"confluence-v0.13.0","draft":false,"prerelease":false}
	]`
	client := mockClient(http.StatusOK, body)

	_, err := FindLatestByPrefix("diegoclair/skills", "jira-v", client)
	if err == nil {
		t.Fatal("expected an error when prefix matches no release")
	}
	if !strings.Contains(err.Error(), "no published release") {
		t.Errorf("expected 'no published release' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "jira-v") {
		t.Errorf("expected the prefix in error message, got: %v", err)
	}
}

func TestFindLatestByPrefix_EmptyResponse(t *testing.T) {
	client := mockClient(http.StatusOK, `[]`)
	_, err := FindLatestByPrefix("diegoclair/skills", "jira-v", client)
	if err == nil {
		t.Fatal("expected an error for empty release list")
	}
}

func TestFindLatestByPrefix_RateLimit(t *testing.T) {
	// 403 + "rate limit" in body gets a dedicated, friendlier error
	// message pointing at the manual-pin escape hatch.
	client := mockClient(http.StatusForbidden, `{"message":"API rate limit exceeded"}`)
	_, err := FindLatestByPrefix("diegoclair/skills", "jira-v", client)
	if err == nil {
		t.Fatal("expected an error for 403 rate-limit")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected 'rate limit' phrasing, got: %v", err)
	}
	if !strings.Contains(err.Error(), "pin the version") {
		t.Errorf("expected escape-hatch hint in error, got: %v", err)
	}
}

func TestFindLatestByPrefix_OtherHTTPError(t *testing.T) {
	// Non-rate-limit 4xx/5xx falls through to the generic "status + body
	// snippet" error so the caller still gets enough to debug.
	client := mockClient(http.StatusInternalServerError, `{"message":"upstream blew up"}`)
	_, err := FindLatestByPrefix("diegoclair/skills", "jira-v", client)
	if err == nil {
		t.Fatal("expected an error for 5xx")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected status code in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "upstream blew up") {
		t.Errorf("expected body snippet in error, got: %v", err)
	}
}

func TestFindLatestByPrefix_MalformedJSON(t *testing.T) {
	client := mockClient(http.StatusOK, `not json`)
	_, err := FindLatestByPrefix("diegoclair/skills", "jira-v", client)
	if err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse JSON") {
		t.Errorf("expected 'parse JSON' in error, got: %v", err)
	}
}

func TestFindLatestByPrefix_RequiredArgs(t *testing.T) {
	_, err := FindLatestByPrefix("", "jira-v", nil)
	if err == nil || !strings.Contains(err.Error(), "repoOwnerRepo") {
		t.Errorf("expected required-field error for empty repo, got: %v", err)
	}
	_, err = FindLatestByPrefix("diegoclair/skills", "", nil)
	if err == nil || !strings.Contains(err.Error(), "prefix") {
		t.Errorf("expected required-field error for empty prefix, got: %v", err)
	}
}

func TestFindLatestByPrefix_RequestHeaders(t *testing.T) {
	// Verify the helper sends the v3 API headers we expect.
	var capturedAccept, capturedAPIVersion string
	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			capturedAccept = r.Header.Get("Accept")
			capturedAPIVersion = r.Header.Get("X-GitHub-Api-Version")
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`[{"tag_name":"jira-v0.1.0"}]`)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	_, _ = FindLatestByPrefix("diegoclair/skills", "jira-v", client)
	if capturedAccept != "application/vnd.github+json" {
		t.Errorf("Accept = %q, want application/vnd.github+json", capturedAccept)
	}
	if capturedAPIVersion != "2022-11-28" {
		t.Errorf("X-GitHub-Api-Version = %q, want 2022-11-28", capturedAPIVersion)
	}
}
