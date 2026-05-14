package jira

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// ---------- test helpers ----------

// roundTripFunc is an http.RoundTripper that delegates to a function, allowing
// inline mock responses in tests. This is the same pattern as confluence_test.go.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// newTestClient returns a *Client wired with a mock transport. The transport
// function receives the outgoing *http.Request and returns whatever the test
// needs to simulate.
func newTestClient(t *testing.T, transport roundTripFunc) *Client {
	t.Helper()
	c, err := NewClient("testcloud", "test@example.com", "testtoken")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.HTTPClient = &http.Client{Transport: transport}
	return c
}

// jsonResp builds an *http.Response with a JSON body and the given status code.
func jsonResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// emptyResp returns a 204 No Content response with no body.
func emptyResp(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

// decodeBody reads the request body and unmarshals it into v.
func decodeBody(t *testing.T, r *http.Request, v any) {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("unmarshal request body: %v\nbody: %s", err, string(b))
	}
}

// assertHeader checks that the request has the expected header value.
func assertHeader(t *testing.T, r *http.Request, key, want string) {
	t.Helper()
	if got := r.Header.Get(key); got != want {
		t.Errorf("header %q: want %q, got %q", key, want, got)
	}
}

// ---------- TestNewClient ----------

func TestNewClient_RequiredFields(t *testing.T) {
	cases := []struct {
		name, cloud, email, token string
		wantErr                   string
	}{
		{"missing cloud", "", "e@e.com", "tok", "cloud subdomain"},
		{"missing email", "cloud", "", "tok", "email"},
		{"missing token", "cloud", "e@e.com", "", "API token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClient(tc.cloud, tc.email, tc.token)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("want error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestNewClient_Success(t *testing.T) {
	c, err := NewClient("mycloud", "user@example.com", "supersecret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Cloud != "mycloud" {
		t.Errorf("Cloud: want %q, got %q", "mycloud", c.Cloud)
	}
	if c.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}
}

func TestBaseURL(t *testing.T) {
	c, _ := NewClient("acme", "u@e.com", "tok")
	want := "https://acme.atlassian.net/rest/api/3"
	if got := c.BaseURL(); got != want {
		t.Errorf("BaseURL: want %q, got %q", want, got)
	}
}

func TestAgileBaseURL(t *testing.T) {
	c, _ := NewClient("acme", "u@e.com", "tok")
	want := "https://acme.atlassian.net/rest/agile/1.0"
	if got := c.AgileBaseURL(); got != want {
		t.Errorf("AgileBaseURL: want %q, got %q", want, got)
	}
}

// ---------- TestMyself ----------

func TestMyself_HappyPath(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Errorf("method: want GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/rest/api/3/myself") {
			t.Errorf("path: want /rest/api/3/myself, got %s", r.URL.Path)
		}
		assertHeader(t, r, "Accept", "application/json")
		// Authorization header must be Basic auth.
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("Authorization: want Basic ..., got %q", auth)
		}

		return jsonResp(200, `{
			"accountId":    "abc123",
			"displayName":  "Test User",
			"emailAddress": "test@example.com"
		}`), nil
	})

	user, err := c.Myself()
	if err != nil {
		t.Fatalf("Myself: %v", err)
	}
	if user.AccountID != "abc123" {
		t.Errorf("AccountID: want %q, got %q", "abc123", user.AccountID)
	}
	if user.DisplayName != "Test User" {
		t.Errorf("DisplayName: want %q, got %q", "Test User", user.DisplayName)
	}
	if user.EmailAddress != "test@example.com" {
		t.Errorf("EmailAddress: want %q, got %q", "test@example.com", user.EmailAddress)
	}
}

func TestMyself_401Unauthorized(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(401, `{"errorMessages":["You do not have the permission to see the specified issue."],"errors":{}}`), nil
	})

	_, err := c.Myself()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsUnauthorized(err) {
		t.Errorf("expected IsUnauthorized, err = %v", err)
	}
}

func TestMyself_500Retries(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if calls < 3 {
			return jsonResp(500, `{"errorMessages":["Internal server error"]}`), nil
		}
		return jsonResp(200, `{"accountId":"xyz","displayName":"Retry User","emailAddress":"r@e.com"}`), nil
	})
	// Disable real sleep in tests by overriding the client with zero-delay transport
	// (retries still happen, they just succeed on the 3rd attempt above).
	user, err := c.Myself()
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 attempts, got %d", calls)
	}
	if user.AccountID != "xyz" {
		t.Errorf("AccountID: want %q, got %q", "xyz", user.AccountID)
	}
}

func TestMyself_500ExhaustsRetries(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		return jsonResp(500, `{"errorMessages":["always down"]}`), nil
	})

	_, err := c.Myself()
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if calls != maxRetries {
		t.Errorf("expected %d attempts, got %d", maxRetries, calls)
	}
	apiErr, ok := AsAPIError(err)
	if !ok {
		t.Fatalf("expected *APIError in chain, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode: want 500, got %d", apiErr.StatusCode)
	}
}

// ---------- TestSearchJQL ----------

func TestSearchJQL_HappyPath(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("method: want POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/rest/api/3/search/jql") {
			t.Errorf("path: want /rest/api/3/search/jql, got %s", r.URL.Path)
		}
		assertHeader(t, r, "Content-Type", "application/json")

		var payload map[string]any
		decodeBody(t, r, &payload)

		if payload["jql"] != "project = PROJ ORDER BY created DESC" {
			t.Errorf("jql: got %v", payload["jql"])
		}
		if payload["maxResults"] == nil {
			t.Error("maxResults should be set")
		}
		fields, ok := payload["fields"].([]any)
		if !ok || len(fields) == 0 {
			t.Errorf("fields should be a non-empty array, got %T %v", payload["fields"], payload["fields"])
		}

		return jsonResp(200, `{
			"issues": [
				{
					"id":  "10001",
					"key": "PROJ-1",
					"self": "https://testcloud.atlassian.net/rest/api/3/issue/10001",
					"fields": {
						"summary": "First issue",
						"status": {
							"name": "To Do",
							"statusCategory": {"key": "new", "name": "To Do"}
						},
						"issuetype": {"name": "Task", "subtask": false},
						"assignee": null,
						"priority": {"name": "Medium"},
						"labels": ["backend"],
						"project": {"key": "PROJ", "name": "My Project"},
						"updated": "2024-01-15T10:00:00.000+0000"
					}
				}
			],
			"nextPageToken": "page2token"
		}`), nil
	})

	result, err := c.SearchJQL(SearchOpts{
		JQL:        "project = PROJ ORDER BY created DESC",
		MaxResults: 10,
	})
	if err != nil {
		t.Fatalf("SearchJQL: %v", err)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("Issues: want 1, got %d", len(result.Issues))
	}
	issue := result.Issues[0]
	if issue.Key != "PROJ-1" {
		t.Errorf("Key: want %q, got %q", "PROJ-1", issue.Key)
	}
	if issue.Fields.Summary != "First issue" {
		t.Errorf("Summary: want %q, got %q", "First issue", issue.Fields.Summary)
	}
	if issue.Fields.Status.Name != "To Do" {
		t.Errorf("Status.Name: want %q, got %q", "To Do", issue.Fields.Status.Name)
	}
	if issue.Fields.Status.StatusCategory.Key != "new" {
		t.Errorf("StatusCategory.Key: want %q, got %q", "new", issue.Fields.Status.StatusCategory.Key)
	}
	if issue.Fields.Priority == nil || issue.Fields.Priority.Name != "Medium" {
		t.Errorf("Priority: want Medium, got %v", issue.Fields.Priority)
	}
	if result.NextPageToken != "page2token" {
		t.Errorf("NextPageToken: want %q, got %q", "page2token", result.NextPageToken)
	}
}

func TestSearchJQL_DefaultFields(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		decodeBody(t, r, &payload)

		// Default fields should be the standard set.
		rawFields, _ := payload["fields"].([]any)
		if len(rawFields) == 0 {
			t.Error("expected default fields to be sent")
		}
		// Check that summary is in there.
		found := false
		for _, f := range rawFields {
			if f == "summary" {
				found = true
			}
		}
		if !found {
			t.Errorf("default fields should include 'summary', got %v", rawFields)
		}
		return jsonResp(200, `{"issues":[],"nextPageToken":""}`), nil
	})

	_, err := c.SearchJQL(SearchOpts{JQL: "project = X"})
	if err != nil {
		t.Fatalf("SearchJQL: %v", err)
	}
}

func TestSearchJQL_NextPageToken(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		decodeBody(t, r, &payload)
		if payload["nextPageToken"] != "cursor123" {
			t.Errorf("nextPageToken: want %q, got %v", "cursor123", payload["nextPageToken"])
		}
		return jsonResp(200, `{"issues":[]}`), nil
	})

	_, err := c.SearchJQL(SearchOpts{JQL: "project = X", NextPageToken: "cursor123"})
	if err != nil {
		t.Fatalf("SearchJQL: %v", err)
	}
}

func TestSearchJQL_MaxResultsCapped(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		decodeBody(t, r, &payload)
		// 200 should be capped to 100.
		mr, _ := payload["maxResults"].(float64)
		if mr != 100 {
			t.Errorf("maxResults: want 100 (capped), got %v", mr)
		}
		return jsonResp(200, `{"issues":[]}`), nil
	})

	_, err := c.SearchJQL(SearchOpts{JQL: "project = X", MaxResults: 200})
	if err != nil {
		t.Fatalf("SearchJQL: %v", err)
	}
}

func TestSearchJQL_400InvalidJQL(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(400, `{"errorMessages":["The value 'INVALID @@' does not exist for the field 'project'."],"errors":{}}`), nil
	})

	_, err := c.SearchJQL(SearchOpts{JQL: "project = INVALID @@"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := AsAPIError(err)
	if !ok {
		t.Fatalf("expected *APIError in chain, got %T", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("StatusCode: want 400, got %d", apiErr.StatusCode)
	}
}

// ---------- TestGetIssue ----------

const sampleIssueJSON = `{
	"id":   "10042",
	"key":  "PROJ-42",
	"self": "https://testcloud.atlassian.net/rest/api/3/issue/10042",
	"fields": {
		"summary": "Fix the thing",
		"status": {
			"name": "In Progress",
			"statusCategory": {"key": "indeterminate", "name": "In Progress"}
		},
		"issuetype": {"name": "Bug", "subtask": false},
		"assignee": {
			"accountId":    "user001",
			"displayName":  "Jane Doe",
			"emailAddress": "jane@example.com"
		},
		"reporter": {
			"accountId":    "user002",
			"displayName":  "John Smith",
			"emailAddress": "john@example.com"
		},
		"priority": {"name": "High"},
		"labels":   ["urgent", "production"],
		"project":  {"key": "PROJ", "name": "My Project"},
		"parent":   {"key": "PROJ-10", "id": "10010"},
		"description": {"type": "doc", "version": 1, "content": []},
		"created": "2024-01-01T09:00:00.000+0000",
		"updated": "2024-01-15T10:00:00.000+0000",
		"duedate": "2024-02-01",
		"customfield_10020": [
			{"id": 5, "name": "Sprint 3", "state": "active", "originBoardId": 2}
		],
		"customfield_99999": "some_custom_value"
	}
}`

func TestGetIssue_HappyPath(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Errorf("method: want GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/rest/api/3/issue/PROJ-42") {
			t.Errorf("path: want .../issue/PROJ-42, got %s", r.URL.Path)
		}
		return jsonResp(200, sampleIssueJSON), nil
	})

	issue, err := c.GetIssue("PROJ-42", nil)
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if issue.Key != "PROJ-42" {
		t.Errorf("Key: want PROJ-42, got %q", issue.Key)
	}
	if issue.ID != "10042" {
		t.Errorf("ID: want 10042, got %q", issue.ID)
	}
	if issue.Fields.Summary != "Fix the thing" {
		t.Errorf("Summary: want %q, got %q", "Fix the thing", issue.Fields.Summary)
	}
	if issue.Fields.Status.Name != "In Progress" {
		t.Errorf("Status.Name: want %q, got %q", "In Progress", issue.Fields.Status.Name)
	}
	if issue.Fields.Assignee == nil {
		t.Fatal("Assignee should not be nil")
	}
	if issue.Fields.Assignee.AccountID != "user001" {
		t.Errorf("Assignee.AccountID: want %q, got %q", "user001", issue.Fields.Assignee.AccountID)
	}
	if issue.Fields.Reporter == nil {
		t.Fatal("Reporter should not be nil")
	}
	if issue.Fields.Reporter.DisplayName != "John Smith" {
		t.Errorf("Reporter.DisplayName: want %q, got %q", "John Smith", issue.Fields.Reporter.DisplayName)
	}
	if issue.Fields.Priority == nil || issue.Fields.Priority.Name != "High" {
		t.Errorf("Priority: want High, got %v", issue.Fields.Priority)
	}
	if len(issue.Fields.Labels) != 2 || issue.Fields.Labels[0] != "urgent" {
		t.Errorf("Labels: want [urgent production], got %v", issue.Fields.Labels)
	}
	if issue.Fields.Parent == nil {
		t.Fatal("Parent should not be nil")
	}
	if issue.Fields.Parent.Key != "PROJ-10" {
		t.Errorf("Parent.Key: want %q, got %q", "PROJ-10", issue.Fields.Parent.Key)
	}
	if issue.Fields.DueDate != "2024-02-01" {
		t.Errorf("DueDate: want %q, got %q", "2024-02-01", issue.Fields.DueDate)
	}
	// Sprint should be parsed from customfield_10020.
	if issue.Fields.Sprint == nil {
		t.Fatal("Sprint should not be nil")
	}
	if issue.Fields.Sprint.Name != "Sprint 3" {
		t.Errorf("Sprint.Name: want %q, got %q", "Sprint 3", issue.Fields.Sprint.Name)
	}
	if issue.Fields.Sprint.State != "active" {
		t.Errorf("Sprint.State: want %q, got %q", "active", issue.Fields.Sprint.State)
	}
	if issue.Fields.Sprint.BoardID != 2 {
		t.Errorf("Sprint.BoardID: want 2, got %d", issue.Fields.Sprint.BoardID)
	}
	// Custom fields should be collected.
	if issue.Fields.Custom == nil {
		t.Error("Custom should not be nil")
	}
	if _, ok := issue.Fields.Custom["customfield_99999"]; !ok {
		t.Error("customfield_99999 should appear in Custom map")
	}
}

func TestGetIssue_WithFields(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		q := r.URL.Query()
		fields := q.Get("fields")
		if !strings.Contains(fields, "summary") || !strings.Contains(fields, "status") {
			t.Errorf("fields param: want summary,status; got %q", fields)
		}
		return jsonResp(200, `{"id":"1","key":"X-1","self":"","fields":{"summary":"s","status":{"name":"Open","statusCategory":{"key":"new","name":"New"}},"issuetype":{"name":"Task","subtask":false},"project":{"key":"X","name":"X"},"labels":[]}}`), nil
	})

	_, err := c.GetIssue("X-1", []string{"summary", "status"})
	if err != nil {
		t.Fatalf("GetIssue with fields: %v", err)
	}
}

func TestGetIssue_EmptyFields(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.RawQuery != "fields=%2Anone" && r.URL.RawQuery != "fields=*none" {
			t.Errorf("want fields=*none in query, got %q", r.URL.RawQuery)
		}
		return jsonResp(200, `{"id":"1","key":"X-1","self":"","fields":{"summary":"","status":{"name":"","statusCategory":{"key":"","name":""}},"issuetype":{"name":"","subtask":false},"project":{"key":"","name":""},"labels":[]}}`), nil
	})

	_, err := c.GetIssue("X-1", []string{})
	if err != nil {
		t.Fatalf("GetIssue with empty fields: %v", err)
	}
}

func TestGetIssue_404NotFound(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(404, `{"errorMessages":["Issue does not exist or you do not have permission to see it."],"errors":{}}`), nil
	})

	_, err := c.GetIssue("NOPROJECT-999", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, err = %v", err)
	}
}

func TestGetIssue_WithComments(t *testing.T) {
	body := `{
		"id": "10001", "key": "PROJ-1", "self": "https://x.atlassian.net/rest/api/3/issue/10001",
		"fields": {
			"summary": "Issue with comments",
			"status": {"name": "Open", "statusCategory": {"key": "new", "name": "New"}},
			"issuetype": {"name": "Task", "subtask": false},
			"project": {"key": "PROJ", "name": "Project"},
			"labels": [],
			"comment": {
				"comments": [
					{
						"id": "c1",
						"author": {"accountId": "u1", "displayName": "Alice", "emailAddress": "alice@example.com"},
						"body": {"type": "doc", "version": 1, "content": []},
						"created": "2024-01-01T00:00:00.000+0000",
						"updated": "2024-01-01T00:00:00.000+0000"
					}
				],
				"total": 1,
				"maxResults": 50,
				"startAt": 0
			}
		}
	}`
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(200, body), nil
	})

	issue, err := c.GetIssue("PROJ-1", []string{"summary", "comment"})
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if issue.Fields.Comment.Total != 1 {
		t.Errorf("Comment.Total: want 1, got %d", issue.Fields.Comment.Total)
	}
	if len(issue.Fields.Comment.Comments) != 1 {
		t.Fatalf("Comment.Comments: want 1, got %d", len(issue.Fields.Comment.Comments))
	}
	if issue.Fields.Comment.Comments[0].Author.DisplayName != "Alice" {
		t.Errorf("Comment author: want Alice, got %q", issue.Fields.Comment.Comments[0].Author.DisplayName)
	}
}

// ---------- TestGetTransitions ----------

const transitionsJSON = `{
	"transitions": [
		{
			"id": "11",
			"name": "Start Progress",
			"to": {
				"name": "In Progress",
				"statusCategory": {"key": "indeterminate", "name": "In Progress"}
			},
			"hasScreen":   false,
			"isGlobal":    true,
			"isAvailable": true
		},
		{
			"id": "31",
			"name": "Done",
			"to": {
				"name": "Done",
				"statusCategory": {"key": "done", "name": "Done"}
			},
			"hasScreen":   false,
			"isGlobal":    true,
			"isAvailable": true
		}
	]
}`

func TestGetTransitions_HappyPath(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Errorf("method: want GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/rest/api/3/issue/PROJ-1/transitions") {
			t.Errorf("path: want .../transitions, got %s", r.URL.Path)
		}
		return jsonResp(200, transitionsJSON), nil
	})

	transitions, err := c.GetTransitions("PROJ-1")
	if err != nil {
		t.Fatalf("GetTransitions: %v", err)
	}
	if len(transitions) != 2 {
		t.Fatalf("want 2 transitions, got %d", len(transitions))
	}
	if transitions[0].ID != "11" {
		t.Errorf("transitions[0].ID: want %q, got %q", "11", transitions[0].ID)
	}
	if transitions[0].Name != "Start Progress" {
		t.Errorf("transitions[0].Name: want %q, got %q", "Start Progress", transitions[0].Name)
	}
	if transitions[0].To.Name != "In Progress" {
		t.Errorf("transitions[0].To.Name: want %q, got %q", "In Progress", transitions[0].To.Name)
	}
	if transitions[0].To.StatusCategory.Key != "indeterminate" {
		t.Errorf("To.StatusCategory.Key: want %q, got %q", "indeterminate", transitions[0].To.StatusCategory.Key)
	}
	if !transitions[0].IsGlobal {
		t.Error("IsGlobal should be true")
	}
	if transitions[1].To.StatusCategory.Key != "done" {
		t.Errorf("transitions[1] category: want done, got %q", transitions[1].To.StatusCategory.Key)
	}
}

func TestGetTransitions_404(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(404, `{"errorMessages":["Issue Not Found"],"errors":{}}`), nil
	})

	_, err := c.GetTransitions("NONEXISTENT-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

// ---------- TestTransitionIssue ----------

func TestTransitionIssue_HappyPath(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("method: want POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/rest/api/3/issue/PROJ-1/transitions") {
			t.Errorf("path: want .../transitions, got %s", r.URL.Path)
		}

		var body map[string]any
		decodeBody(t, r, &body)
		transition, ok := body["transition"].(map[string]any)
		if !ok {
			t.Fatalf("body.transition: expected map, got %T", body["transition"])
		}
		if transition["id"] != "31" {
			t.Errorf("transition.id: want %q, got %v", "31", transition["id"])
		}

		return emptyResp(204), nil
	})

	if err := c.TransitionIssue("PROJ-1", "31"); err != nil {
		t.Fatalf("TransitionIssue: %v", err)
	}
}

func TestTransitionIssue_400InvalidTransition(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(400, `{"errorMessages":["It seems that you have tried to perform an operation which is not applicable for the current state of issue PROJ-1"],"errors":{}}`), nil
	})

	err := c.TransitionIssue("PROJ-1", "999")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := AsAPIError(err)
	if !ok {
		t.Fatalf("expected *APIError in chain, got %T", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("StatusCode: want 400, got %d", apiErr.StatusCode)
	}
}

// TestGetTransitionsAndApply is an integration-style test that wires together
// GetTransitions + TransitionIssue: list transitions, pick the "Done" one, apply it.
func TestGetTransitionsAndApply(t *testing.T) {
	step := 0
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		step++
		switch step {
		case 1: // GetTransitions
			if r.Method != http.MethodGet {
				t.Errorf("step1: want GET, got %s", r.Method)
			}
			return jsonResp(200, transitionsJSON), nil
		case 2: // TransitionIssue
			if r.Method != http.MethodPost {
				t.Errorf("step2: want POST, got %s", r.Method)
			}
			var body map[string]any
			decodeBody(t, r, &body)
			transition := body["transition"].(map[string]any)
			// Should have picked the "Done" transition (id=31)
			if transition["id"] != "31" {
				t.Errorf("step2: expected transition id=31 (Done), got %v", transition["id"])
			}
			return emptyResp(204), nil
		default:
			t.Fatalf("unexpected extra HTTP call (step %d)", step)
			return nil, nil
		}
	})

	transitions, err := c.GetTransitions("PROJ-5")
	if err != nil {
		t.Fatalf("GetTransitions: %v", err)
	}

	// Find "Done" transition.
	var doneID string
	for _, tr := range transitions {
		if tr.To.StatusCategory.Key == "done" {
			doneID = tr.ID
			break
		}
	}
	if doneID == "" {
		t.Fatal("no Done transition found in test data")
	}

	if err := c.TransitionIssue("PROJ-5", doneID); err != nil {
		t.Fatalf("TransitionIssue: %v", err)
	}
	if step != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", step)
	}
}

// ---------- TestCreateIssue ----------

func TestCreateIssue_HappyPath(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("method: want POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/rest/api/3/issue") {
			t.Errorf("path: want /rest/api/3/issue, got %s", r.URL.Path)
		}
		assertHeader(t, r, "Content-Type", "application/json")

		var payload struct {
			Fields map[string]any `json:"fields"`
		}
		decodeBody(t, r, &payload)

		// Verify the expected REST body shape.
		project, _ := payload.Fields["project"].(map[string]any)
		if project["key"] != "PROJ" {
			t.Errorf("project.key: want PROJ, got %v", project["key"])
		}
		issuetype, _ := payload.Fields["issuetype"].(map[string]any)
		if issuetype["name"] != "Bug" {
			t.Errorf("issuetype.name: want Bug, got %v", issuetype["name"])
		}
		if payload.Fields["summary"] != "Regression in payment flow" {
			t.Errorf("summary: want %q, got %v", "Regression in payment flow", payload.Fields["summary"])
		}
		priority, _ := payload.Fields["priority"].(map[string]any)
		if priority["name"] != "High" {
			t.Errorf("priority.name: want High, got %v", priority["name"])
		}
		assignee, _ := payload.Fields["assignee"].(map[string]any)
		if assignee["accountId"] != "acc123" {
			t.Errorf("assignee.accountId: want acc123, got %v", assignee["accountId"])
		}
		labels, _ := payload.Fields["labels"].([]any)
		if len(labels) != 2 || labels[0] != "regression" {
			t.Errorf("labels: want [regression, payment], got %v", labels)
		}
		parent, _ := payload.Fields["parent"].(map[string]any)
		if parent["key"] != "PROJ-10" {
			t.Errorf("parent.key: want PROJ-10, got %v", parent["key"])
		}
		if payload.Fields["duedate"] != "2024-03-01" {
			t.Errorf("duedate: want 2024-03-01, got %v", payload.Fields["duedate"])
		}

		return jsonResp(201, `{"id":"10099","key":"PROJ-99","self":"https://testcloud.atlassian.net/rest/api/3/issue/10099"}`), nil
	})

	issue, err := c.CreateIssue(CreateIssueInput{
		ProjectKey:   "PROJ",
		IssueType:    "Bug",
		Summary:      "Regression in payment flow",
		Labels:       []string{"regression", "payment"},
		AssigneeID:   "acc123",
		ParentKey:    "PROJ-10",
		DueDate:      "2024-03-01",
		PriorityName: "High",
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if issue.Key != "PROJ-99" {
		t.Errorf("Key: want PROJ-99, got %q", issue.Key)
	}
	if issue.ID != "10099" {
		t.Errorf("ID: want 10099, got %q", issue.ID)
	}
}

func TestCreateIssue_WithCustomFields(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		var payload struct {
			Fields map[string]any `json:"fields"`
		}
		decodeBody(t, r, &payload)

		if payload.Fields["customfield_10016"] != float64(5) {
			t.Errorf("customfield_10016: want 5, got %v", payload.Fields["customfield_10016"])
		}
		return jsonResp(201, `{"id":"10100","key":"PROJ-100","self":"https://testcloud.atlassian.net/rest/api/3/issue/10100"}`), nil
	})

	_, err := c.CreateIssue(CreateIssueInput{
		ProjectKey: "PROJ",
		IssueType:  "Story",
		Summary:    "Story with story points",
		Custom: map[string]any{
			"customfield_10016": 5,
		},
	})
	if err != nil {
		t.Fatalf("CreateIssue with custom fields: %v", err)
	}
}

func TestCreateIssue_WithADFDescription(t *testing.T) {
	adf := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Hello world"}]}]}`)

	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		var payload struct {
			Fields map[string]json.RawMessage `json:"fields"`
		}
		decodeBody(t, r, &payload)

		desc, ok := payload.Fields["description"]
		if !ok {
			t.Fatal("description field missing from request")
		}
		if !strings.Contains(string(desc), "Hello world") {
			t.Errorf("description ADF: want 'Hello world', got %s", string(desc))
		}
		return jsonResp(201, `{"id":"10101","key":"PROJ-101","self":"https://testcloud.atlassian.net/rest/api/3/issue/10101"}`), nil
	})

	_, err := c.CreateIssue(CreateIssueInput{
		ProjectKey:  "PROJ",
		IssueType:   "Task",
		Summary:     "Task with ADF description",
		Description: adf,
	})
	if err != nil {
		t.Fatalf("CreateIssue with ADF description: %v", err)
	}
}

func TestCreateIssue_400Validation(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(400, `{"errorMessages":[],"errors":{"project":"project is required"}}`), nil
	})

	_, err := c.CreateIssue(CreateIssueInput{
		ProjectKey: "",
		IssueType:  "Task",
		Summary:    "No project",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := AsAPIError(err)
	if !ok {
		t.Fatalf("expected *APIError in chain, got %T", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("StatusCode: want 400, got %d", apiErr.StatusCode)
	}
	if apiErr.Errors["project"] != "project is required" {
		t.Errorf("Errors[project]: want 'project is required', got %q", apiErr.Errors["project"])
	}
}

// ---------- TestEditIssue ----------

func TestEditIssue_HappyPath(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPut {
			t.Errorf("method: want PUT, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/rest/api/3/issue/PROJ-5") {
			t.Errorf("path: want .../issue/PROJ-5, got %s", r.URL.Path)
		}

		var payload struct {
			Fields map[string]any `json:"fields"`
		}
		decodeBody(t, r, &payload)

		if payload.Fields["summary"] != "Updated summary" {
			t.Errorf("summary: want %q, got %v", "Updated summary", payload.Fields["summary"])
		}
		labels, _ := payload.Fields["labels"].([]any)
		if len(labels) != 1 || labels[0] != "new-label" {
			t.Errorf("labels: want [new-label], got %v", labels)
		}

		return emptyResp(204), nil
	})

	err := c.EditIssue("PROJ-5", map[string]any{
		"summary": "Updated summary",
		"labels":  []string{"new-label"},
	})
	if err != nil {
		t.Fatalf("EditIssue: %v", err)
	}
}

func TestEditIssue_404NotFound(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(404, `{"errorMessages":["Issue Does Not Exist"],"errors":{}}`), nil
	})

	err := c.EditIssue("PROJ-NOTEXIST", map[string]any{"summary": "x"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

func TestEditIssue_400InvalidField(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(400, `{"errorMessages":[],"errors":{"labels":"Label 'UNKNOWN' does not exist."}}`), nil
	})

	err := c.EditIssue("PROJ-5", map[string]any{"labels": []string{"UNKNOWN"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := AsAPIError(err)
	if !ok {
		t.Fatalf("expected *APIError in chain, got %T", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("StatusCode: want 400, got %d", apiErr.StatusCode)
	}
}

// ---------- TestAddComment ----------

func TestAddComment_HappyPath(t *testing.T) {
	adfBody := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"This is a comment."}]}]}`)

	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("method: want POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/rest/api/3/issue/PROJ-7/comment") {
			t.Errorf("path: want .../comment, got %s", r.URL.Path)
		}

		var payload map[string]json.RawMessage
		decodeBody(t, r, &payload)

		body, ok := payload["body"]
		if !ok {
			t.Fatal("body field missing from request")
		}
		if !strings.Contains(string(body), "This is a comment") {
			t.Errorf("body: want ADF with comment text, got %s", string(body))
		}

		return jsonResp(201, `{
			"id": "cmt001",
			"author": {
				"accountId":    "u1",
				"displayName":  "Commenter",
				"emailAddress": "commenter@example.com"
			},
			"body": {"type":"doc","version":1,"content":[]},
			"created": "2024-01-15T12:00:00.000+0000",
			"updated": "2024-01-15T12:00:00.000+0000"
		}`), nil
	})

	comment, err := c.AddComment("PROJ-7", adfBody)
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if comment.ID != "cmt001" {
		t.Errorf("ID: want %q, got %q", "cmt001", comment.ID)
	}
	if comment.Author.DisplayName != "Commenter" {
		t.Errorf("Author.DisplayName: want %q, got %q", "Commenter", comment.Author.DisplayName)
	}
	if comment.Created != "2024-01-15T12:00:00.000+0000" {
		t.Errorf("Created: want %q, got %q", "2024-01-15T12:00:00.000+0000", comment.Created)
	}
	if comment.Body == nil {
		t.Error("Body should not be nil")
	}
}

func TestAddComment_401Unauthorized(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(401, `{"errorMessages":["You are not authenticated."],"errors":{}}`), nil
	})

	_, err := c.AddComment("PROJ-7", json.RawMessage(`{"type":"doc","version":1,"content":[]}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsUnauthorized(err) {
		t.Errorf("expected IsUnauthorized, got %v", err)
	}
}

func TestAddComment_404NotFound(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(404, `{"errorMessages":["Issue not found"],"errors":{}}`), nil
	})

	_, err := c.AddComment("PROJ-GHOST", json.RawMessage(`{"type":"doc","version":1,"content":[]}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

// ---------- TestAPIError ----------

func TestAPIError_MessageFormatting(t *testing.T) {
	e := &APIError{
		StatusCode:    400,
		ErrorMessages: []string{"Invalid value"},
		Errors:        map[string]string{"summary": "Summary is required"},
	}
	msg := e.Error()
	if !strings.Contains(msg, "400") {
		t.Errorf("error should contain status code, got: %s", msg)
	}
	if !strings.Contains(msg, "Invalid value") {
		t.Errorf("error should contain errorMessages, got: %s", msg)
	}
	if !strings.Contains(msg, "summary") {
		t.Errorf("error should contain field errors, got: %s", msg)
	}
}

func TestAPIError_RawBodyFallback(t *testing.T) {
	e := &APIError{StatusCode: 503, RawBody: "Service Unavailable"}
	msg := e.Error()
	if !strings.Contains(msg, "503") || !strings.Contains(msg, "Service Unavailable") {
		t.Errorf("unexpected error message: %s", msg)
	}
}

func TestAPIError_EmptyError(t *testing.T) {
	e := &APIError{StatusCode: 429}
	msg := e.Error()
	if !strings.Contains(msg, "429") {
		t.Errorf("unexpected error message: %s", msg)
	}
}

// ---------- TestParseSprint ----------

func TestParseSprint_ActivePreferred(t *testing.T) {
	raw := json.RawMessage(`[
		{"id":1,"name":"Sprint 1","state":"closed","originBoardId":10},
		{"id":2,"name":"Sprint 2","state":"active","originBoardId":10},
		{"id":3,"name":"Sprint 3","state":"future","originBoardId":10}
	]`)

	sprint := parseSprint(raw)
	if sprint == nil {
		t.Fatal("expected sprint, got nil")
	}
	if sprint.Name != "Sprint 2" || sprint.State != "active" {
		t.Errorf("expected active sprint, got %+v", sprint)
	}
}

func TestParseSprint_NullReturnsNil(t *testing.T) {
	if s := parseSprint(json.RawMessage("null")); s != nil {
		t.Errorf("expected nil for null, got %+v", s)
	}
	if s := parseSprint(json.RawMessage("")); s != nil {
		t.Errorf("expected nil for empty, got %+v", s)
	}
}

func TestParseSprint_SingleObject(t *testing.T) {
	raw := json.RawMessage(`{"id":7,"name":"Q1 Sprint","state":"active","originBoardId":3}`)
	sprint := parseSprint(raw)
	if sprint == nil {
		t.Fatal("expected sprint, got nil")
	}
	if sprint.ID != 7 || sprint.BoardID != 3 {
		t.Errorf("unexpected sprint: %+v", sprint)
	}
}

// ---------- TestBasicAuth ----------

func TestBasicAuth_Format(t *testing.T) {
	c, _ := NewClient("cloud", "user@example.com", "mytoken")
	auth := c.basicAuth()
	if !strings.HasPrefix(auth, "Basic ") {
		t.Errorf("basicAuth: want 'Basic ...', got %q", auth)
	}
	// Verify the token encodes correctly.
	import64 := strings.TrimPrefix(auth, "Basic ")
	if import64 == "" {
		t.Error("basicAuth: encoded part is empty")
	}
}

// TestGetIssue_Retry5xx verifies that GET /issue/{key} is retried on 5xx.
func TestGetIssue_Retry5xx(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if calls < 2 {
			return jsonResp(503, `{"errorMessages":["Service Unavailable"]}`), nil
		}
		return jsonResp(200, `{"id":"1","key":"PROJ-1","self":"","fields":{"summary":"ok","status":{"name":"Open","statusCategory":{"key":"new","name":"New"}},"issuetype":{"name":"Task","subtask":false},"project":{"key":"P","name":"P"},"labels":[]}}`), nil
	})

	issue, err := c.GetIssue("PROJ-1", nil)
	if err != nil {
		t.Fatalf("GetIssue after retry: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", calls)
	}
	if issue.Fields.Summary != "ok" {
		t.Errorf("Summary: want %q, got %q", "ok", issue.Fields.Summary)
	}
}

// TestTransitionIssue_NoRetry verifies that POST (write) methods are not retried on 5xx.
func TestTransitionIssue_NoRetry(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		return jsonResp(500, `{"errorMessages":["Internal server error"]}`), nil
	})

	err := c.TransitionIssue("PROJ-1", "31")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// POST is not retried — should be exactly 1 call.
	if calls != 1 {
		t.Errorf("POST should not be retried: expected 1 call, got %d", calls)
	}
}

// ---------- TestIssueUnassigned ----------

func TestGetIssue_Unassigned(t *testing.T) {
	body := `{"id":"1","key":"X-1","self":"","fields":{"summary":"Unassigned","status":{"name":"Open","statusCategory":{"key":"new","name":"New"}},"issuetype":{"name":"Task","subtask":false},"assignee":null,"priority":null,"project":{"key":"X","name":"X"},"labels":[]}}`
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(200, body), nil
	})

	issue, err := c.GetIssue("X-1", nil)
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if issue.Fields.Assignee != nil {
		t.Errorf("Assignee: want nil, got %+v", issue.Fields.Assignee)
	}
	if issue.Fields.Priority != nil {
		t.Errorf("Priority: want nil, got %+v", issue.Fields.Priority)
	}
}

// ---------- TestSearchJQL_EmptyResult ----------

func TestSearchJQL_EmptyResult(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		return jsonResp(200, `{"issues":[],"nextPageToken":""}`), nil
	})

	result, err := c.SearchJQL(SearchOpts{JQL: "project = EMPTY"})
	if err != nil {
		t.Fatalf("SearchJQL: %v", err)
	}
	if len(result.Issues) != 0 {
		t.Errorf("Issues: want 0, got %d", len(result.Issues))
	}
	if result.NextPageToken != "" {
		t.Errorf("NextPageToken: want empty, got %q", result.NextPageToken)
	}
}

// ---------- TestEditIssue_AssigneeShape ----------

func TestEditIssue_AssigneeShape(t *testing.T) {
	c := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		var payload struct {
			Fields map[string]any `json:"fields"`
		}
		decodeBody(t, r, &payload)
		assignee, _ := payload.Fields["assignee"].(map[string]any)
		if assignee["accountId"] != "newuser123" {
			t.Errorf("assignee.accountId: want newuser123, got %v", assignee["accountId"])
		}
		return emptyResp(204), nil
	})

	err := c.EditIssue("PROJ-8", map[string]any{
		"assignee": map[string]any{"accountId": "newuser123"},
	})
	if err != nil {
		t.Fatalf("EditIssue: %v", err)
	}
}
