package jira

import "encoding/json"

// Issue represents a Jira issue as returned by GET /rest/api/3/issue/{key}.
type Issue struct {
	// ID is the numeric string Jira assigns internally (e.g. "10042").
	ID string
	// Key is the project-prefixed identifier displayed to users (e.g. "PROJ-42").
	Key string
	// Self is the canonical API URL for this issue.
	Self string
	// Fields contains the issue field values.
	Fields IssueFields
}

// IssueFields holds the per-issue field values. Fields not requested via the
// fields filter are left at their zero value.
type IssueFields struct {
	Summary     string
	Status      Status
	Issuetype   IssueType
	Assignee    *User           // nil = unassigned
	Reporter    *User
	Priority    *Priority       // nil = no priority set
	Labels      []string
	Project     Project
	Parent      *IssueRef       // nil = no parent issue
	Description json.RawMessage // ADF node tree as raw JSON
	Comment     CommentList     // populated only when "comment" is in the fields filter
	Sprint      *Sprint         // nil when not in an active sprint
	Created     string          // ISO 8601
	Updated     string          // ISO 8601
	DueDate     string          // YYYY-MM-DD; "" means none
	Custom      map[string]any  // catch-all for customfield_* values
}

// Status represents the workflow status of an issue.
type Status struct {
	Name           string
	StatusCategory StatusCategory
}

// StatusCategory groups statuses into broad lifecycle buckets.
type StatusCategory struct {
	// Key is one of "new", "indeterminate", or "done".
	Key  string
	Name string
}

// IssueType describes the kind of work item (Task, Bug, Story, Epic, …).
type IssueType struct {
	Name    string
	Subtask bool
}

// Priority holds the priority level of an issue (e.g. "High", "Medium").
type Priority struct {
	Name string
}

// Project holds the key and name of the project an issue belongs to.
type Project struct {
	Key  string
	Name string
}

// IssueRef is a lightweight reference to another issue (used for Parent).
type IssueRef struct {
	Key string
	ID  string
}

// User represents a Jira Cloud user account.
type User struct {
	AccountID    string
	DisplayName  string
	EmailAddress string // may be hidden when profile visibility is restricted
}

// Transition represents a workflow transition the authenticated user can apply.
type Transition struct {
	ID          string
	Name        string
	To          Status
	HasScreen   bool
	IsGlobal    bool
	IsAvailable bool
}

// Comment is a single comment on an issue.
type Comment struct {
	ID      string
	Author  User
	Body    json.RawMessage // ADF node tree as raw JSON
	Created string
	Updated string
}

// CommentList wraps a page of comments returned inside an issue's fields.
type CommentList struct {
	Comments   []Comment
	Total      int
	MaxResults int
	StartAt    int
}

// Sprint represents a Jira Agile sprint.
type Sprint struct {
	ID      int
	Name    string
	State   string // "active" / "closed" / "future"
	BoardID int
}

// SearchOpts controls a JQL search request.
type SearchOpts struct {
	// JQL is the query string (required).
	JQL string
	// Fields lists which issue fields to return. Empty defaults to a standard
	// set. Pass []string{} explicitly only if you want the API default.
	Fields []string
	// NextPageToken is the cursor for retrieving the next page. Empty on the
	// first call; the value comes from SearchResult.NextPageToken.
	NextPageToken string
	// MaxResults caps the page size (default 50, cap 100).
	MaxResults int
}

// SearchResult holds one page of JQL search results.
type SearchResult struct {
	Issues []Issue
	// NextPageToken is the cursor for the next page. Empty when all results
	// have been returned.
	NextPageToken string
}

// CreateIssueInput carries all fields for POST /rest/api/3/issue.
type CreateIssueInput struct {
	// ProjectKey is the Jira project key (e.g. "PROJ"). Required.
	ProjectKey string
	// IssueType is the issue type name (e.g. "Task", "Bug", "Story"). Required.
	IssueType string
	// Summary is the one-line title. Required.
	Summary string
	// Description is the issue body as ADF JSON (raw). Optional.
	Description json.RawMessage
	// Labels is an optional list of label strings.
	Labels []string
	// AssigneeID is the accountId of the assignee. Optional.
	AssigneeID string
	// ParentKey is the key of the parent issue (for subtasks / epic children). Optional.
	ParentKey string
	// DueDate is the due date in YYYY-MM-DD format. Optional.
	DueDate string
	// PriorityName is the priority level name (e.g. "High"). Optional.
	PriorityName string
	// Custom holds raw customfield_* key-value pairs. Optional.
	Custom map[string]any
}

// ProjectFull holds detailed information about a Jira project returned by
// GET /rest/api/3/project/{key}.
type ProjectFull struct {
	ID               string
	Key              string
	Name             string
	ProjectTypeKey   string
	Simplified       bool
	Lead             *User
	DefaultAssignee  string
	AvatarURL        string
}

// ProjectSearchResult holds one page of results from GET /rest/api/3/project/search.
type ProjectSearchResult struct {
	Projects   []ProjectFull
	Total      int
	StartAt    int
	MaxResults int
	IsLast     bool
}

// ProjectUpdate carries the fields to change on PUT /rest/api/3/project/{key}.
// Only non-nil fields are included in the JSON body sent to the API.
type ProjectUpdate struct {
	Name        *string `json:"name,omitempty"`
	Key         *string `json:"key,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ---------- internal wire types ----------

// issueWire is the shape returned by GET /rest/api/3/issue/{key}.
type issueWire struct {
	ID     string          `json:"id"`
	Key    string          `json:"key"`
	Self   string          `json:"self"`
	Fields issueFieldsWire `json:"fields"`
}

// issueFieldsWire mirrors the "fields" object from the Jira REST v3 response.
// Custom fields are captured by the RawMessage map in a second pass.
type issueFieldsWire struct {
	Summary   string `json:"summary"`
	Status    struct {
		Name           string `json:"name"`
		StatusCategory struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"statusCategory"`
	} `json:"status"`
	Issuetype struct {
		Name    string `json:"name"`
		Subtask bool   `json:"subtask"`
	} `json:"issuetype"`
	Assignee *struct {
		AccountID    string `json:"accountId"`
		DisplayName  string `json:"displayName"`
		EmailAddress string `json:"emailAddress"`
	} `json:"assignee"`
	Reporter *struct {
		AccountID    string `json:"accountId"`
		DisplayName  string `json:"displayName"`
		EmailAddress string `json:"emailAddress"`
	} `json:"reporter"`
	Priority *struct {
		Name string `json:"name"`
	} `json:"priority"`
	Labels  []string `json:"labels"`
	Project struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"project"`
	Parent *struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	} `json:"parent"`
	Description json.RawMessage `json:"description"`
	Comment     struct {
		Comments []struct {
			ID     string `json:"id"`
			Author struct {
				AccountID    string `json:"accountId"`
				DisplayName  string `json:"displayName"`
				EmailAddress string `json:"emailAddress"`
			} `json:"author"`
			Body    json.RawMessage `json:"body"`
			Created string          `json:"created"`
			Updated string          `json:"updated"`
		} `json:"comments"`
		Total      int `json:"total"`
		MaxResults int `json:"maxResults"`
		StartAt    int `json:"startAt"`
	} `json:"comment"`
	// Sprint is a custom field; we handle it via customfield_10020.
	// It may be a single object or an array depending on the Jira config.
	Created string `json:"created"`
	Updated string `json:"updated"`
	DueDate string `json:"duedate"`
}

func (w issueWire) toIssue(rawFields map[string]json.RawMessage) Issue {
	f := w.Fields
	issue := Issue{
		ID:   w.ID,
		Key:  w.Key,
		Self: w.Self,
	}
	fields := IssueFields{
		Summary: f.Summary,
		Status: Status{
			Name: f.Status.Name,
			StatusCategory: StatusCategory{
				Key:  f.Status.StatusCategory.Key,
				Name: f.Status.StatusCategory.Name,
			},
		},
		Issuetype: IssueType{
			Name:    f.Issuetype.Name,
			Subtask: f.Issuetype.Subtask,
		},
		Labels:      f.Labels,
		Description: f.Description,
		Created:     f.Created,
		Updated:     f.Updated,
		DueDate:     f.DueDate,
		Project: Project{
			Key:  f.Project.Key,
			Name: f.Project.Name,
		},
	}

	if f.Assignee != nil {
		fields.Assignee = &User{
			AccountID:    f.Assignee.AccountID,
			DisplayName:  f.Assignee.DisplayName,
			EmailAddress: f.Assignee.EmailAddress,
		}
	}
	if f.Reporter != nil {
		fields.Reporter = &User{
			AccountID:    f.Reporter.AccountID,
			DisplayName:  f.Reporter.DisplayName,
			EmailAddress: f.Reporter.EmailAddress,
		}
	}
	if f.Priority != nil {
		fields.Priority = &Priority{Name: f.Priority.Name}
	}
	if f.Parent != nil {
		fields.Parent = &IssueRef{Key: f.Parent.Key, ID: f.Parent.ID}
	}

	// Comments
	if len(f.Comment.Comments) > 0 || f.Comment.Total > 0 {
		cl := CommentList{
			Total:      f.Comment.Total,
			MaxResults: f.Comment.MaxResults,
			StartAt:    f.Comment.StartAt,
		}
		for _, c := range f.Comment.Comments {
			cl.Comments = append(cl.Comments, Comment{
				ID: c.ID,
				Author: User{
					AccountID:    c.Author.AccountID,
					DisplayName:  c.Author.DisplayName,
					EmailAddress: c.Author.EmailAddress,
				},
				Body:    c.Body,
				Created: c.Created,
				Updated: c.Updated,
			})
		}
		fields.Comment = cl
	}

	// Sprint — try customfield_10020 from the raw fields map.
	if rawFields != nil {
		if sprintRaw, ok := rawFields["customfield_10020"]; ok {
			fields.Sprint = parseSprint(sprintRaw)
		}

		// Collect remaining customfields into Custom map.
		custom := make(map[string]any)
		for k, v := range rawFields {
			if len(k) > len("customfield_") && k[:len("customfield_")] == "customfield_" && k != "customfield_10020" {
				var val any
				if err := json.Unmarshal(v, &val); err == nil {
					custom[k] = val
				}
			}
		}
		if len(custom) > 0 {
			fields.Custom = custom
		}
	}

	issue.Fields = fields
	return issue
}

// parseSprint parses the sprint custom field. Jira returns an array of sprint
// objects for customfield_10020; we return the first active one if present,
// otherwise the first element.
func parseSprint(raw json.RawMessage) *Sprint {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	type sprintWire struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		State         string `json:"state"`
		OriginBoardID int    `json:"originBoardId"`
	}

	// Try array form first.
	var arr []sprintWire
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		// Prefer the active sprint; fall back to last element.
		chosen := arr[len(arr)-1]
		for _, s := range arr {
			if s.State == "active" {
				chosen = s
				break
			}
		}
		return &Sprint{
			ID:      chosen.ID,
			Name:    chosen.Name,
			State:   chosen.State,
			BoardID: chosen.OriginBoardID,
		}
	}

	// Try single-object form.
	var single sprintWire
	if err := json.Unmarshal(raw, &single); err == nil && single.ID != 0 {
		return &Sprint{
			ID:      single.ID,
			Name:    single.Name,
			State:   single.State,
			BoardID: single.OriginBoardID,
		}
	}

	return nil
}
