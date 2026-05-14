// Package jira is the Atlassian Jira Cloud REST v3 client used by the
// jira-tickets skill. Designed for slim, LLM-friendly outputs: digest of
// an issue in ~500 bytes, JQL search as TSV, transitions that don't fetch
// the full ADF body.
//
// Auth: Basic auth — base64(email:token) — using an Atlassian API token
// generated at https://id.atlassian.com/manage-profile/security/api-tokens.
//
// Retry policy: idempotent GETs are retried up to 3 times on 5xx responses
// with exponential back-off (1s, 2s, 4s). Write methods (POST/PUT) are NOT
// retried automatically.
//
// Error handling: non-2xx responses are unwrapped from Atlassian's JSON error
// envelope {"errorMessages":[...], "errors":{...}} and returned as *APIError.
// Callers can type-assert to inspect StatusCode.
package jira

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
// call yet. Verify the credentials by calling Myself after construction.
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

// ---------- Auth ----------

// basicAuth returns a Base64-encoded Basic auth header value for the client's
// email and token.
func (c *Client) basicAuth() string {
	raw := c.Email + ":" + c.Token
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

// ---------- Error types ----------

// APIError is returned when the Jira API responds with a non-2xx status.
// Callers can type-assert to inspect StatusCode (e.g. 404 = not found,
// 401 = bad credentials, 409 = conflict).
type APIError struct {
	StatusCode    int
	ErrorMessages []string
	Errors        map[string]string
	RawBody       string
}

func (e *APIError) Error() string {
	parts := e.ErrorMessages
	for k, v := range e.Errors {
		parts = append(parts, k+": "+v)
	}
	if len(parts) > 0 {
		return fmt.Sprintf("jira API error %d: %s", e.StatusCode, strings.Join(parts, "; "))
	}
	if e.RawBody != "" {
		body := e.RawBody
		if len(body) > 300 {
			body = body[:300] + "..."
		}
		return fmt.Sprintf("jira API error %d: %s", e.StatusCode, body)
	}
	return fmt.Sprintf("jira API error %d", e.StatusCode)
}

// IsNotFound reports whether err (or any error it wraps) is a 404 from the
// Jira API.
func IsNotFound(err error) bool {
	return apiStatusCode(err) == 404
}

// IsUnauthorized reports whether err (or any error it wraps) is a 401 from
// the Jira API.
func IsUnauthorized(err error) bool {
	return apiStatusCode(err) == 401
}

// IsConflict reports whether err (or any error it wraps) is a 409 from the
// Jira API.
func IsConflict(err error) bool {
	return apiStatusCode(err) == 409
}

// apiStatusCode walks the error chain and returns the StatusCode of the first
// *APIError found. Returns 0 if none is found.
func apiStatusCode(err error) int {
	for err != nil {
		if e, ok := err.(*APIError); ok {
			return e.StatusCode
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
			continue
		}
		return 0
	}
	return 0
}

// AsAPIError extracts the first *APIError from the error chain, analogous to
// errors.As. Returns (nil, false) when the chain contains no *APIError.
func AsAPIError(err error) (*APIError, bool) {
	for err != nil {
		if e, ok := err.(*APIError); ok {
			return e, true
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
			continue
		}
		return nil, false
	}
	return nil, false
}

// parseAPIError attempts to decode the Atlassian error envelope and returns
// an *APIError. Falls back to the raw body when the envelope doesn't parse.
func parseAPIError(statusCode int, body []byte) *APIError {
	var envelope struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		return &APIError{
			StatusCode:    statusCode,
			ErrorMessages: envelope.ErrorMessages,
			Errors:        envelope.Errors,
		}
	}
	return &APIError{
		StatusCode: statusCode,
		RawBody:    string(body),
	}
}

// ---------- Core HTTP ----------

const (
	maxRetries      = 3
	retryBaseDelay  = time.Second
)

// doRequest executes an HTTP request against the given full URL and returns
// the response body on 2xx. Non-2xx responses are returned as *APIError.
// GET requests are retried up to maxRetries times on 5xx with exponential
// back-off. Non-GET (write) methods are executed exactly once.
func (c *Client) doRequest(method, fullURL string, body io.Reader) ([]byte, int, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, 0, fmt.Errorf("jira: read request body: %w", err)
		}
	}

	isIdempotent := method == http.MethodGet || method == http.MethodHead

	attempts := 1
	if isIdempotent {
		attempts = maxRetries
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			// Exponential back-off: 1s, 2s, 4s, …
			delay := retryBaseDelay * (1 << uint(attempt-1))
			time.Sleep(delay)
		}

		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequest(method, fullURL, reqBody)
		if err != nil {
			return nil, 0, fmt.Errorf("jira: build request: %w", err)
		}
		req.Header.Set("Authorization", c.basicAuth())
		req.Header.Set("Accept", "application/json")
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("jira: HTTP %s %s: %w", method, fullURL, err)
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("jira: read response body: %w", readErr)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, resp.StatusCode, nil
		}

		apiErr := parseAPIError(resp.StatusCode, respBody)
		// Only retry on 5xx for idempotent requests.
		if isIdempotent && resp.StatusCode >= 500 {
			lastErr = apiErr
			continue
		}
		return nil, resp.StatusCode, apiErr
	}
	return nil, 0, lastErr
}

// get is a convenience wrapper for GET requests to the v3 REST API.
func (c *Client) get(path string) ([]byte, error) {
	data, _, err := c.doRequest(http.MethodGet, c.BaseURL()+path, nil)
	return data, err
}

// post is a convenience wrapper for POST requests to the v3 REST API.
func (c *Client) post(path string, payload any) ([]byte, int, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("jira: marshal request: %w", err)
	}
	return c.doRequest(http.MethodPost, c.BaseURL()+path, bytes.NewReader(b))
}

// put is a convenience wrapper for PUT requests to the v3 REST API.
func (c *Client) put(path string, payload any) (int, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("jira: marshal request: %w", err)
	}
	_, status, err := c.doRequest(http.MethodPut, c.BaseURL()+path, bytes.NewReader(b))
	return status, err
}

// ---------- Methods ----------

// Myself returns the authenticated user's account information.
// Uses GET /rest/api/3/myself. This is the cheapest call to verify credentials.
// Returns *APIError with StatusCode 401 when the token or email is invalid.
func (c *Client) Myself() (*User, error) {
	data, err := c.get("/myself")
	if err != nil {
		return nil, fmt.Errorf("jira: myself: %w", err)
	}
	var wire struct {
		AccountID    string `json:"accountId"`
		DisplayName  string `json:"displayName"`
		EmailAddress string `json:"emailAddress"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("jira: myself: parse response: %w", err)
	}
	return &User{
		AccountID:    wire.AccountID,
		DisplayName:  wire.DisplayName,
		EmailAddress: wire.EmailAddress,
	}, nil
}

// defaultSearchFields is the set of fields returned when SearchOpts.Fields is nil.
var defaultSearchFields = []string{
	"summary", "status", "assignee", "issuetype",
	"priority", "labels", "parent", "updated",
}

// SearchJQL runs a JQL search using the cursor-paginated POST endpoint
// POST /rest/api/3/search/jql (introduced 2025; the old GET /search with
// startAt is deprecated). Caller iterates pages by passing
// SearchResult.NextPageToken back as opts.NextPageToken until it is empty.
//
// If opts.Fields is nil, the default field set is used. If opts.MaxResults is
// 0, it defaults to 50 (capped at 100 by this client before sending).
//
// Returns *APIError with StatusCode 400 on invalid JQL.
func (c *Client) SearchJQL(opts SearchOpts) (*SearchResult, error) {
	fields := opts.Fields
	if fields == nil {
		fields = defaultSearchFields
	}

	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}
	if maxResults > 100 {
		maxResults = 100
	}

	payload := map[string]any{
		"jql":        opts.JQL,
		"fields":     fields,
		"maxResults": maxResults,
	}
	if opts.NextPageToken != "" {
		payload["nextPageToken"] = opts.NextPageToken
	}

	data, _, err := c.post("/search/jql", payload)
	if err != nil {
		return nil, fmt.Errorf("jira: searchJQL: %w", err)
	}

	var resp struct {
		Issues        []json.RawMessage `json:"issues"`
		NextPageToken string            `json:"nextPageToken"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("jira: searchJQL: parse response: %w", err)
	}

	result := &SearchResult{
		NextPageToken: resp.NextPageToken,
		Issues:        make([]Issue, 0, len(resp.Issues)),
	}
	for _, raw := range resp.Issues {
		issue, err := parseIssueRaw(raw)
		if err != nil {
			return nil, fmt.Errorf("jira: searchJQL: parse issue: %w", err)
		}
		result.Issues = append(result.Issues, issue)
	}
	return result, nil
}

// GetIssue fetches a single issue by key or ID.
// Uses GET /rest/api/3/issue/{key}?fields=<csv>.
//
//   - fields == nil  → omit the fields parameter (API returns everything).
//   - fields == []string{} → send fields=*none (minimal response).
//   - fields == ["summary","status"] → only those fields.
//
// Pass "comment" in fields to populate IssueFields.Comment.
// Pass "sprint" or leave fields nil to populate IssueFields.Sprint.
//
// Returns *APIError with StatusCode 404 when the issue does not exist.
func (c *Client) GetIssue(key string, fields []string) (*Issue, error) {
	path := "/issue/" + url.PathEscape(key)
	if fields != nil {
		if len(fields) == 0 {
			path += "?fields=*none"
		} else {
			path += "?fields=" + url.QueryEscape(strings.Join(fields, ","))
		}
	}

	data, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("jira: getIssue %s: %w", key, err)
	}

	issue, err := parseIssueRaw(data)
	if err != nil {
		return nil, fmt.Errorf("jira: getIssue %s: %w", key, err)
	}
	return &issue, nil
}

// parseIssueRaw decodes a raw JSON issue object. It does a two-pass decode:
// first into the typed issueWire struct, then into a raw map to capture
// customfield_* entries that the typed struct doesn't know about.
func parseIssueRaw(raw json.RawMessage) (Issue, error) {
	var wire issueWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Issue{}, fmt.Errorf("decode issue: %w", err)
	}

	// Second pass: raw map to capture customfield_* values.
	var rawMap struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	_ = json.Unmarshal(raw, &rawMap) // best-effort; ignore errors

	return wire.toIssue(rawMap.Fields), nil
}

// GetTransitions returns the workflow transitions the authenticated user can
// apply to the issue in its current status.
// Uses GET /rest/api/3/issue/{key}/transitions.
//
// Returns *APIError with StatusCode 404 when the issue does not exist.
func (c *Client) GetTransitions(key string) ([]Transition, error) {
	data, err := c.get("/issue/" + url.PathEscape(key) + "/transitions")
	if err != nil {
		return nil, fmt.Errorf("jira: getTransitions %s: %w", key, err)
	}

	var resp struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name           string `json:"name"`
				StatusCategory struct {
					Key  string `json:"key"`
					Name string `json:"name"`
				} `json:"statusCategory"`
			} `json:"to"`
			HasScreen   bool `json:"hasScreen"`
			IsGlobal    bool `json:"isGlobal"`
			IsAvailable bool `json:"isAvailable"`
		} `json:"transitions"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("jira: getTransitions %s: parse response: %w", key, err)
	}

	out := make([]Transition, 0, len(resp.Transitions))
	for _, t := range resp.Transitions {
		out = append(out, Transition{
			ID:   t.ID,
			Name: t.Name,
			To: Status{
				Name: t.To.Name,
				StatusCategory: StatusCategory{
					Key:  t.To.StatusCategory.Key,
					Name: t.To.StatusCategory.Name,
				},
			},
			HasScreen:   t.HasScreen,
			IsGlobal:    t.IsGlobal,
			IsAvailable: t.IsAvailable,
		})
	}
	return out, nil
}

// TransitionIssue applies a workflow transition to an issue.
// Uses POST /rest/api/3/issue/{key}/transitions.
// transitionID must come from GetTransitions — do not guess IDs.
//
// Returns nil on success (204 No Content from Jira).
// Returns a descriptive *APIError when the transition is not available from
// the current status (Jira returns 400 in that case).
func (c *Client) TransitionIssue(key, transitionID string) error {
	payload := map[string]any{
		"transition": map[string]any{
			"id": transitionID,
		},
	}
	_, status, err := c.post("/issue/"+url.PathEscape(key)+"/transitions", payload)
	if err != nil {
		return fmt.Errorf("jira: transitionIssue %s (id=%s): %w", key, transitionID, err)
	}
	// 204 = success; 200 with empty body is also accepted.
	_ = status
	return nil
}

// CreateIssue creates a new issue and returns it with the server-assigned key
// and ID populated.
// Uses POST /rest/api/3/issue.
//
// For sub-tasks, set input.ParentKey to the parent issue's key.
// For epics or stories that are children of an epic, set input.ParentKey to
// the epic's key (Jira Next-Gen / team-managed projects support this).
//
// Returns *APIError with StatusCode 400 on validation errors (e.g. missing
// required field, invalid project key).
func (c *Client) CreateIssue(input CreateIssueInput) (*Issue, error) {
	fields := map[string]any{
		"project":   map[string]any{"key": input.ProjectKey},
		"issuetype": map[string]any{"name": input.IssueType},
		"summary":   input.Summary,
	}

	if len(input.Description) > 0 && string(input.Description) != "null" {
		fields["description"] = json.RawMessage(input.Description)
	}
	if len(input.Labels) > 0 {
		fields["labels"] = input.Labels
	}
	if input.AssigneeID != "" {
		fields["assignee"] = map[string]any{"accountId": input.AssigneeID}
	}
	if input.ParentKey != "" {
		fields["parent"] = map[string]any{"key": input.ParentKey}
	}
	if input.DueDate != "" {
		fields["duedate"] = input.DueDate
	}
	if input.PriorityName != "" {
		fields["priority"] = map[string]any{"name": input.PriorityName}
	}
	for k, v := range input.Custom {
		fields[k] = v
	}

	payload := map[string]any{"fields": fields}
	data, _, err := c.post("/issue", payload)
	if err != nil {
		return nil, fmt.Errorf("jira: createIssue: %w", err)
	}

	var created struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Self string `json:"self"`
	}
	if err := json.Unmarshal(data, &created); err != nil {
		return nil, fmt.Errorf("jira: createIssue: parse response: %w", err)
	}

	return &Issue{
		ID:   created.ID,
		Key:  created.Key,
		Self: created.Self,
	}, nil
}

// EditIssue updates one or more fields on an existing issue in place.
// Uses PUT /rest/api/3/issue/{key}.
// The fields map values must match the Jira REST v3 field shapes exactly
// (e.g. {"assignee": {"accountId": "..."}, "labels": [...]}).
//
// Returns nil on success (204 No Content from Jira).
// Returns *APIError with StatusCode 400 when a field value is invalid.
func (c *Client) EditIssue(key string, fields map[string]any) error {
	payload := map[string]any{"fields": fields}
	_, err := c.put("/issue/"+url.PathEscape(key), payload)
	if err != nil {
		return fmt.Errorf("jira: editIssue %s: %w", key, err)
	}
	return nil
}

// AddComment posts a new comment on the given issue.
// Uses POST /rest/api/3/issue/{key}/comment.
// adfBody must be valid ADF JSON (the caller is responsible for conversion
// from markdown; use the adf package in the calling layer so this package
// stays free of that dependency).
//
// Returns the newly created Comment with server-assigned ID, Created, and
// Updated timestamps.
func (c *Client) AddComment(key string, adfBody json.RawMessage) (*Comment, error) {
	payload := map[string]any{
		"body": json.RawMessage(adfBody),
	}
	data, _, err := c.post("/issue/"+url.PathEscape(key)+"/comment", payload)
	if err != nil {
		return nil, fmt.Errorf("jira: addComment %s: %w", key, err)
	}

	var wire struct {
		ID     string `json:"id"`
		Author struct {
			AccountID    string `json:"accountId"`
			DisplayName  string `json:"displayName"`
			EmailAddress string `json:"emailAddress"`
		} `json:"author"`
		Body    json.RawMessage `json:"body"`
		Created string          `json:"created"`
		Updated string          `json:"updated"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("jira: addComment %s: parse response: %w", key, err)
	}

	return &Comment{
		ID: wire.ID,
		Author: User{
			AccountID:    wire.Author.AccountID,
			DisplayName:  wire.Author.DisplayName,
			EmailAddress: wire.Author.EmailAddress,
		},
		Body:    wire.Body,
		Created: wire.Created,
		Updated: wire.Updated,
	}, nil
}
