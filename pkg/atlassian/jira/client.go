// Package jira is the Atlassian Jira Cloud REST v3 client used by the
// jira-tickets skill. Designed for slim, LLM-friendly outputs: digest of
// an issue in ~500 bytes, JQL search as TSV, transitions that don't fetch
// the full ADF body.
//
// Status: skeleton. The methods below are stubs that will be filled in
// across the v0.2.x – v0.5.x line of jira-tickets releases. The client
// type, auth contract, and base-URL convention are stable so command
// files (cmd_issue_get.go, cmd_search.go, ...) can be written against
// them in parallel.
package jira

import (
	"fmt"
	"net/http"
	"time"
)

// Client is an authenticated handle to a Jira Cloud REST v3 endpoint.
// Always constructed via NewClient — the zero value is not useful.
type Client struct {
	// Cloud is the Atlassian cloud subdomain (e.g. "mycompany" for
	// mycompany.atlassian.net). Required.
	Cloud string

	// Email is the Atlassian account email associated with the API token.
	Email string

	// Token is the Atlassian API token (NOT the password). Generated at
	// https://id.atlassian.com/manage-profile/security/api-tokens.
	Token string

	// HTTPClient is the underlying http.Client. Defaults to a 30s-timeout
	// client. Tests inject a custom transport via this field.
	HTTPClient *http.Client
}

// NewClient builds an authenticated Jira client. Returns an error only if
// required fields (cloud, email, token) are empty — does NOT make an HTTP
// call yet. Verify the credentials by running an actual API call after
// construction (e.g. Myself).
func NewClient(cloud, email, token string) (*Client, error) {
	if cloud == "" {
		return nil, fmt.Errorf("jira: cloud subdomain is required")
	}
	if email == "" {
		return nil, fmt.Errorf("jira: email is required")
	}
	if token == "" {
		return nil, fmt.Errorf("jira: API token is required")
	}
	return &Client{
		Cloud:      cloud,
		Email:      email,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// BaseURL returns the Jira Cloud REST v3 base URL for this client.
// Example: https://mycompany.atlassian.net/rest/api/3
func (c *Client) BaseURL() string {
	return fmt.Sprintf("https://%s.atlassian.net/rest/api/3", c.Cloud)
}

// AgileBaseURL returns the Jira Agile (v1) base URL for sprint/board ops.
// Sprint membership and board state live behind /rest/agile/1.0, not v3.
// Example: https://mycompany.atlassian.net/rest/agile/1.0
func (c *Client) AgileBaseURL() string {
	return fmt.Sprintf("https://%s.atlassian.net/rest/agile/1.0", c.Cloud)
}

// TODO(jira-v0.2): Myself returns the authenticated user's account info.
//   GET /rest/api/3/myself
//
// TODO(jira-v0.2): SearchJQL runs a JQL search and returns results.
//   POST /rest/api/3/search/jql (new endpoint as of 2025; old /search deprecated)
//
// TODO(jira-v0.2): GetIssue fetches a single issue with optional fields filter.
//   GET /rest/api/3/issue/{issueIdOrKey}?fields=<...>
//
// TODO(jira-v0.3): CreateIssue posts a new issue.
//   POST /rest/api/3/issue
//
// TODO(jira-v0.3): EditIssue updates fields.
//   PUT /rest/api/3/issue/{issueIdOrKey}
//
// TODO(jira-v0.3): GetTransitions lists transitions available from current status.
//   GET /rest/api/3/issue/{issueIdOrKey}/transitions
//
// TODO(jira-v0.3): TransitionIssue executes a transition.
//   POST /rest/api/3/issue/{issueIdOrKey}/transitions
//
// TODO(jira-v0.3): AddComment adds a comment to an issue (ADF body).
//   POST /rest/api/3/issue/{issueIdOrKey}/comment
//
// TODO(jira-v0.4): MoveSprint via the Agile v1 endpoint.
//   POST /rest/agile/1.0/sprint/{sprintId}/issue
