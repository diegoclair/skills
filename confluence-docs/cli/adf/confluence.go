// Package adf - Confluence Cloud REST API v2 HTTP client.
//
// Auth is resolved in order:
//  1. Explicit token/email passed to NewClient
//  2. $ATLASSIAN_API_TOKEN and $ATLASSIAN_EMAIL env vars
//  3. Platform config dir / confluence-docs / credentials (key=value file)
//     Linux:   $XDG_CONFIG_HOME/confluence-docs/credentials
//     macOS:   ~/Library/Application Support/confluence-docs/credentials
//     Windows: %AppData%\confluence-docs\credentials
//     (legacy ~/.config/confluence-docs/credentials is read with a warning)
//
// Cloud subdomain is resolved in order:
//  1. Explicit cloud passed to NewClient
//  2. $ATLASSIAN_CLOUD env var
//  3. Default "lybel"
package adf

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// configPath returns the platform-appropriate credentials file path via
// os.UserConfigDir(), which resolves to:
//   - Linux:   $XDG_CONFIG_HOME/confluence-docs/credentials (or ~/.config/…)
//   - macOS:   ~/Library/Application Support/confluence-docs/credentials
//   - Windows: %AppData%\confluence-docs\credentials
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "confluence-docs", "credentials"), nil
}

// legacyConfigPath returns the pre-migration path ~/.config/confluence-docs/credentials.
func legacyConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "confluence-docs", "credentials"), nil
}

// ConfluenceCreds holds authentication credentials for the Confluence API.
type ConfluenceCreds struct {
	Email string
	Token string
}

// HTTPError is returned by client methods when the API responds with a non-2xx
// status. Callers can type-assert to inspect StatusCode (e.g. 409 conflict).
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("Confluence API error %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("Confluence API returned %d", e.StatusCode)
}

// IsConflict reports whether err (or any error it wraps) is a 409 from the
// Confluence API — typically a stale version on update.
func IsConflict(err error) bool {
	for err != nil {
		if h, ok := err.(*HTTPError); ok {
			return h.StatusCode == 409
		}
		// Unwrap manually since we don't import errors here in tight loops
		type unwrapper interface{ Unwrap() error }
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
			continue
		}
		return false
	}
	return false
}

// ConfluenceClient is a minimal Confluence Cloud REST API v2 client.
type ConfluenceClient struct {
	baseURL    string // e.g. https://lybel.atlassian.net/wiki
	creds      ConfluenceCreds
	httpClient *http.Client
}

// PageMeta holds the minimal page metadata needed for updates.
type PageMeta struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
	Body struct {
		Storage struct {
			Value          string `json:"value"`
			Representation string `json:"representation"`
		} `json:"storage"`
		AtlasDocFormat struct {
			Value          string `json:"value"`
			Representation string `json:"representation"`
		} `json:"atlas_doc_format"`
		View struct {
			Value          string `json:"value"`
			Representation string `json:"representation"`
		} `json:"view"`
		ExportView struct {
			Value          string `json:"value"`
			Representation string `json:"representation"`
		} `json:"export_view"`
	} `json:"body"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

// PageCreateResult is returned by page create.
type PageCreateResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

// ResolveCloud returns the effective cloud subdomain, checking (in order):
// the explicit override, $ATLASSIAN_CLOUD, then defaulting to "lybel".
func ResolveCloud(override string) string {
	if override != "" {
		return override
	}
	if env := os.Getenv("ATLASSIAN_CLOUD"); env != "" {
		return env
	}
	return "lybel"
}

// ResolveCreds reads credentials from the first source that provides both
// email and token: explicit args → env vars → config file.
// Returns an error with actionable text if no credentials are found.
func ResolveCreds(email, token string) (ConfluenceCreds, error) {
	// 1. Explicit flags
	if email != "" && token != "" {
		return ConfluenceCreds{Email: email, Token: token}, nil
	}

	// 2. Environment variables
	envToken := os.Getenv("ATLASSIAN_API_TOKEN")
	envEmail := os.Getenv("ATLASSIAN_EMAIL")
	if envToken != "" && envEmail != "" {
		return ConfluenceCreds{Email: envEmail, Token: envToken}, nil
	}

	// 3. Config file
	cfgCreds, err := readCredsFile()
	if err == nil && cfgCreds.Email != "" && cfgCreds.Token != "" {
		return cfgCreds, nil
	}

	// Build a helpful path hint using the actual platform config dir.
	cfgPathHint := "~/.config/confluence-docs/credentials"
	if p, err := configPath(); err == nil {
		cfgPathHint = p
	}
	return ConfluenceCreds{}, fmt.Errorf(
		"no Confluence credentials found.\n"+
			"Options (use any one):\n"+
			"  1. Flags:   --email you@example.com --token <api-token>\n"+
			"  2. Env:     ATLASSIAN_EMAIL=you@example.com ATLASSIAN_API_TOKEN=<api-token>\n"+
			"  3. File:    %s\n"+
			"              email=you@example.com\n"+
			"              token=<api-token>\n"+
			"Run `confluence-docs setup` to create this file interactively.\n"+
			"See SETUP.md for how to generate an API token.",
		cfgPathHint,
	)
}

// readCredsFile parses the platform credentials file (key=value format).
// Falls back to the legacy ~/.config/confluence-docs/credentials with a warning
// printed to stderr if the new path does not exist but the old one does.
// On Linux the two paths are identical so no fallback is attempted.
func readCredsFile() (ConfluenceCreds, error) {
	newPath, err := configPath()
	if err != nil {
		return ConfluenceCreds{}, err
	}

	data, readErr := os.ReadFile(newPath)
	if readErr != nil && os.IsNotExist(readErr) {
		legacyPath, legacyErr := legacyConfigPath()
		if legacyErr == nil && legacyPath != newPath {
			if legacyData, legacyReadErr := os.ReadFile(legacyPath); legacyReadErr == nil {
				// Print warning to stderr. We use os.Stderr directly here because
				// readCredsFile has no io.Writer param (keeping the existing API).
				fmt.Fprintf(os.Stderr,
					"warning: credentials found at legacy path %s — run `confluence-docs setup` to migrate to %s\n",
					legacyPath, newPath)
				data = legacyData
				readErr = nil
			}
		}
	}
	if readErr != nil {
		return ConfluenceCreds{}, readErr
	}

	var creds ConfluenceCreds
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "email":
			creds.Email = val
		case "token":
			creds.Token = val
		}
	}
	return creds, nil
}

// NewClient creates a ConfluenceClient for the given cloud subdomain and creds.
func NewClient(cloud string, creds ConfluenceCreds) *ConfluenceClient {
	return &ConfluenceClient{
		baseURL:    fmt.Sprintf("https://%s.atlassian.net/wiki", cloud),
		creds:      creds,
		httpClient: &http.Client{},
	}
}

// basicAuth returns a base64-encoded Basic auth header value.
func (c *ConfluenceClient) basicAuth() string {
	raw := c.creds.Email + ":" + c.creds.Token
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

// doRequest executes an HTTP request, returning the response body bytes.
// Non-2xx responses are returned as errors with the body included.
func (c *ConfluenceClient) doRequest(method, path string, body io.Reader) ([]byte, int, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", c.basicAuth())
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("HTTP %s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to extract a human-readable error message from the response.
		var apiErr struct {
			Message string `json:"message"`
			Errors  []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if jerr := json.Unmarshal(respBody, &apiErr); jerr == nil && apiErr.Message != "" {
			return nil, resp.StatusCode, &HTTPError{StatusCode: resp.StatusCode, Message: apiErr.Message}
		}
		// Truncate body for readability
		body := string(respBody)
		if len(body) > 300 {
			body = body[:300] + "..."
		}
		return nil, resp.StatusCode, &HTTPError{StatusCode: resp.StatusCode, Message: body}
	}

	return respBody, resp.StatusCode, nil
}

// GetPage fetches a page by ID with its body in the given representation.
// representation: "atlas_doc_format" for ADF, "storage" for HTML/XHTML,
// "export_view" for rendered HTML. Use "atlas_doc_format" for ADF edits.
func (c *ConfluenceClient) GetPage(pageID, bodyFormat string) (*PageMeta, error) {
	if bodyFormat == "" {
		bodyFormat = "atlas_doc_format"
	}
	path := fmt.Sprintf("/api/v2/pages/%s?body-format=%s", pageID, bodyFormat)
	data, _, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get page %s: %w", pageID, err)
	}
	var meta PageMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse page response: %w", err)
	}
	return &meta, nil
}

// GetPageChildren returns the IDs and titles of a page's direct children.
func (c *ConfluenceClient) GetPageChildren(pageID string) ([]PageCreateResult, error) {
	path := fmt.Sprintf("/api/v2/pages/%s/children?limit=250", pageID)
	data, _, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get children of %s: %w", pageID, err)
	}
	var resp struct {
		Results []PageCreateResult `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse children response: %w", err)
	}
	return resp.Results, nil
}

// UpdatePage uploads a new ADF body to an existing page.
// title and versionNumber are fetched automatically if not provided (0/"").
// dryRun prints a summary to dryRunOut without calling the API.
func (c *ConfluenceClient) UpdatePage(pageID, title string, versionNumber int, adfBody Node, versionMessage string, dryRun bool, dryRunOut io.Writer) error {
	// Auto-fetch title and version if not provided.
	if title == "" || versionNumber == 0 {
		meta, err := c.GetPage(pageID, "atlas_doc_format")
		if err != nil {
			return fmt.Errorf("auto-fetch page metadata: %w", err)
		}
		if title == "" {
			title = meta.Title
		}
		if versionNumber == 0 {
			versionNumber = meta.Version.Number + 1
		}
	}

	bodyJSON, err := json.Marshal(adfBody)
	if err != nil {
		return fmt.Errorf("marshal ADF body: %w", err)
	}

	if dryRun {
		fmt.Fprintf(dryRunOut, "[dry-run] Would update page ID %s:\n", pageID)
		fmt.Fprintf(dryRunOut, "  Title:   %s\n", title)
		fmt.Fprintf(dryRunOut, "  Version: %d\n", versionNumber)
		if versionMessage != "" {
			fmt.Fprintf(dryRunOut, "  Message: %s\n", versionMessage)
		}
		fmt.Fprintf(dryRunOut, "  Body size: %d bytes\n", len(bodyJSON))
		fmt.Fprintf(dryRunOut, "[dry-run] No changes made.\n")
		return nil
	}

	payload := map[string]any{
		"id":     pageID,
		"status": "current",
		"title":  title,
		"version": map[string]any{
			"number":  versionNumber,
			"message": versionMessage,
		},
		"body": map[string]any{
			"representation": "atlas_doc_format",
			"value":          string(bodyJSON),
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal update payload: %w", err)
	}

	path := fmt.Sprintf("/api/v2/pages/%s", pageID)
	_, _, err = c.doRequest("PUT", path, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("update page %s: %w", pageID, err)
	}
	return nil
}

// CreatePageStorage creates a new page with a body in Confluence storage
// (XHTML + macro XML) format instead of ADF. Use this when the markdown source
// contains macros that have no pure-ADF equivalent (e.g. page-properties).
func (c *ConfluenceClient) CreatePageStorage(spaceID, parentID, title string, storageBody string) (*PageCreateResult, error) {
	payload := map[string]any{
		"spaceId":  spaceID,
		"parentId": parentID,
		"status":   "current",
		"title":    title,
		"body": map[string]any{
			"representation": "storage",
			"value":          storageBody,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal create payload: %w", err)
	}

	data, _, err := c.doRequest("POST", "/api/v2/pages", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("create page (storage): %w", err)
	}

	var result PageCreateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse create response: %w", err)
	}
	return &result, nil
}

// UpdatePageStorage updates an existing page body in Confluence storage
// (XHTML + macro XML) format instead of ADF. Use this when the markdown source
// contains macros that have no pure-ADF equivalent (e.g. page-properties).
// title and versionNumber are fetched automatically if not provided (0/"").
func (c *ConfluenceClient) UpdatePageStorage(pageID, title string, versionNumber int, storageBody string, versionMessage string, dryRun bool, dryRunOut io.Writer) error {
	// Auto-fetch title and version if not provided.
	if title == "" || versionNumber == 0 {
		meta, err := c.GetPage(pageID, "storage")
		if err != nil {
			return fmt.Errorf("auto-fetch page metadata: %w", err)
		}
		if title == "" {
			title = meta.Title
		}
		if versionNumber == 0 {
			versionNumber = meta.Version.Number + 1
		}
	}

	if dryRun {
		fmt.Fprintf(dryRunOut, "[dry-run] Would update page ID %s (storage format):\n", pageID)
		fmt.Fprintf(dryRunOut, "  Title:   %s\n", title)
		fmt.Fprintf(dryRunOut, "  Version: %d\n", versionNumber)
		if versionMessage != "" {
			fmt.Fprintf(dryRunOut, "  Message: %s\n", versionMessage)
		}
		fmt.Fprintf(dryRunOut, "  Body size: %d bytes\n", len(storageBody))
		fmt.Fprintf(dryRunOut, "[dry-run] No changes made.\n")
		return nil
	}

	payload := map[string]any{
		"id":     pageID,
		"status": "current",
		"title":  title,
		"version": map[string]any{
			"number":  versionNumber,
			"message": versionMessage,
		},
		"body": map[string]any{
			"representation": "storage",
			"value":          storageBody,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal update payload: %w", err)
	}

	path := fmt.Sprintf("/api/v2/pages/%s", pageID)
	_, _, err = c.doRequest("PUT", path, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("update page %s (storage): %w", pageID, err)
	}
	return nil
}

// MovePage moves a page to a new parent and/or renames it. The body is
// preserved (refetched and re-PUT) since the v2 PUT requires it.
// If newParentID is "", parent is unchanged. If newTitle is "", title is
// unchanged. At least one must be non-empty.
func (c *ConfluenceClient) MovePage(pageID, newParentID, newTitle, versionMessage string, dryRun bool, dryRunOut io.Writer) error {
	if newParentID == "" && newTitle == "" {
		return fmt.Errorf("MovePage requires --parent-id and/or --title")
	}

	meta, err := c.GetPage(pageID, "atlas_doc_format")
	if err != nil {
		return fmt.Errorf("fetch current page: %w", err)
	}

	title := newTitle
	if title == "" {
		title = meta.Title
	}
	versionNumber := meta.Version.Number + 1
	bodyValue := meta.Body.AtlasDocFormat.Value

	if dryRun {
		fmt.Fprintf(dryRunOut, "[dry-run] Would move/rename page ID %s:\n", pageID)
		fmt.Fprintf(dryRunOut, "  Old title: %s\n", meta.Title)
		fmt.Fprintf(dryRunOut, "  New title: %s\n", title)
		if newParentID != "" {
			fmt.Fprintf(dryRunOut, "  New parent: %s\n", newParentID)
		} else {
			fmt.Fprintf(dryRunOut, "  Parent: unchanged\n")
		}
		fmt.Fprintf(dryRunOut, "  Version: %d\n", versionNumber)
		if versionMessage != "" {
			fmt.Fprintf(dryRunOut, "  Message: %s\n", versionMessage)
		}
		fmt.Fprintf(dryRunOut, "[dry-run] No changes made.\n")
		return nil
	}

	payload := map[string]any{
		"id":     pageID,
		"status": "current",
		"title":  title,
		"version": map[string]any{
			"number":  versionNumber,
			"message": versionMessage,
		},
		"body": map[string]any{
			"representation": "atlas_doc_format",
			"value":          bodyValue,
		},
	}
	if newParentID != "" {
		payload["parentId"] = newParentID
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal move payload: %w", err)
	}

	path := fmt.Sprintf("/api/v2/pages/%s", pageID)
	_, _, err = c.doRequest("PUT", path, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("move page %s: %w", pageID, err)
	}
	return nil
}

// DeletePage trashes a page (soft delete). The page can be restored from
// the trash within the Confluence retention window.
func (c *ConfluenceClient) DeletePage(pageID string) error {
	path := fmt.Sprintf("/api/v2/pages/%s", pageID)
	_, _, err := c.doRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("delete page %s: %w", pageID, err)
	}
	return nil
}

// ReorderPage repositions a page among its siblings (or appends it as the
// last child of a target parent). Uses the v1 endpoint
// `PUT /wiki/rest/api/content/{pageId}/move/{position}/{targetId}` since the
// v2 API doesn't expose sibling-order control.
//
// position must be one of:
//
//	"before" — place pageID immediately before targetID (same parent).
//	"after"  — place pageID immediately after targetID (same parent).
//	"append" — append pageID as the last child of targetID (re-parents).
//
// Body and title are untouched.
func (c *ConfluenceClient) ReorderPage(pageID, position, targetID string) error {
	switch position {
	case "before", "after", "append":
	default:
		return fmt.Errorf("ReorderPage: invalid position %q (want before|after|append)", position)
	}
	if pageID == "" || targetID == "" {
		return fmt.Errorf("ReorderPage: pageID and targetID are required")
	}
	path := fmt.Sprintf("/rest/api/content/%s/move/%s/%s", pageID, position, targetID)
	_, _, err := c.doRequest("PUT", path, nil)
	if err != nil {
		return fmt.Errorf("reorder page %s %s %s: %w", pageID, position, targetID, err)
	}
	return nil
}

// CreatePage creates a new page under the given parent in the given space.
// Content can be provided as ADF (adfBody != nil) or left empty.
func (c *ConfluenceClient) CreatePage(spaceID, parentID, title string, adfBody *Node) (*PageCreateResult, error) {
	payload := map[string]any{
		"spaceId":  spaceID,
		"parentId": parentID,
		"status":   "current",
		"title":    title,
	}

	if adfBody != nil {
		bodyJSON, err := json.Marshal(adfBody)
		if err != nil {
			return nil, fmt.Errorf("marshal ADF body: %w", err)
		}
		payload["body"] = map[string]any{
			"representation": "atlas_doc_format",
			"value":          string(bodyJSON),
		}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal create payload: %w", err)
	}

	data, _, err := c.doRequest("POST", "/api/v2/pages", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("create page: %w", err)
	}

	var result PageCreateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse create response: %w", err)
	}
	return &result, nil
}

// PageURL returns a full Confluence page URL from the base URL and webui path.
func (c *ConfluenceClient) PageURL(webuiPath string) string {
	if strings.HasPrefix(webuiPath, "http") {
		return webuiPath
	}
	return strings.TrimRight(c.baseURL, "/") + "/" + strings.TrimLeft(webuiPath, "/")
}

// SearchResult is one row from a CQL search.
type SearchResult struct {
	PageID  string
	Title   string
	URL     string
	Excerpt string // plain-text excerpt, HTML highlight tags stripped
}

// SearchCQL runs a CQL query via the v1 Confluence search REST endpoint and
// returns up to `limit` results (max 250). The v2 API has no search endpoint,
// so we use /rest/api/search which has been stable for years.
//
// CQL is passed verbatim — caller is responsible for escaping. Typical shape:
//
//	space = "lybel" AND (title ~ "term" OR text ~ "term")
func (c *ConfluenceClient) SearchCQL(cql string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 250 {
		limit = 250
	}
	// URL-encode the CQL query
	q := url.Values{}
	q.Set("cql", cql)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("expand", "content.history,content.version")
	path := "/rest/api/search?" + q.Encode()

	data, _, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	var resp struct {
		Results []struct {
			Content struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Type  string `json:"type"`
				Links struct {
					WebUI string `json:"webui"`
				} `json:"_links"`
			} `json:"content"`
			Title    string `json:"title"`
			Excerpt  string `json:"excerpt"`
			URL      string `json:"url"`
			EntityType string `json:"entityType"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	out := make([]SearchResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		// Skip non-page results (attachments, comments, etc.)
		if r.EntityType != "" && r.EntityType != "content" {
			continue
		}
		if r.Content.Type != "" && r.Content.Type != "page" {
			continue
		}

		pageID := r.Content.ID
		title := r.Content.Title
		if title == "" {
			title = r.Title
		}
		webui := r.Content.Links.WebUI
		if webui == "" {
			webui = r.URL
		}
		out = append(out, SearchResult{
			PageID:  pageID,
			Title:   title,
			URL:     c.PageURL(webui),
			Excerpt: stripExcerptHTML(r.Excerpt),
		})
	}
	return out, nil
}

// stripExcerptHTML removes Confluence's <b>highlight</b> tags and other
// minimal HTML noise from search excerpts. Not a full HTML parser — just
// enough to make the output readable as plain text.
func stripExcerptHTML(s string) string {
	if s == "" {
		return ""
	}
	// Drop common tags
	for _, tag := range []string{"<b>", "</b>", "<i>", "</i>", "<em>", "</em>", "<strong>", "</strong>", "<mark>", "</mark>"} {
		s = strings.ReplaceAll(s, tag, "")
	}
	// Collapse newlines and multiple spaces
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// BaseURL returns the base URL of this client.
func (c *ConfluenceClient) BaseURL() string {
	return c.baseURL
}

// ---------- Page Properties (appearance) ----------

// PageAppearance represents the content appearance of a page.
type PageAppearance string

const (
	// PageAppearanceFullWidth sets the page to full-width layout.
	PageAppearanceFullWidth PageAppearance = "full-width"
	// PageAppearanceFixedWidth sets the page to fixed-width layout (default).
	PageAppearanceFixedWidth PageAppearance = "fixed-width"
)

// SetPageAppearance posts the content-appearance-draft and
// content-appearance-published page properties to apply a full-width or
// fixed-width layout to the page.
//
// This requires two POST calls to /wiki/api/v2/pages/{pageId}/properties —
// one for the draft state and one for the published state.
//
// IMPORTANT: Confluence page properties use a "create or update" pattern.
// The v2 API does not support PATCH; if the property already exists, a
// subsequent POST returns 409. To handle this we try POST first and fall back
// to PUT if we get a conflict.
func (c *ConfluenceClient) SetPageAppearance(pageID string, appearance PageAppearance) error {
	keys := []string{
		"content-appearance-draft",
		"content-appearance-published",
	}
	for _, key := range keys {
		if err := c.upsertPageProperty(pageID, key, string(appearance)); err != nil {
			return fmt.Errorf("set page appearance (%s): %w", key, err)
		}
	}
	return nil
}

// upsertPageProperty creates or updates a single page property by key.
// It tries POST first; on 409 it lists existing properties to find the
// property version and retries with PUT.
func (c *ConfluenceClient) upsertPageProperty(pageID, key, value string) error {
	payload := map[string]any{
		"key":   key,
		"value": value,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal property: %w", err)
	}

	path := fmt.Sprintf("/api/v2/pages/%s/properties", pageID)
	_, statusCode, postErr := c.doRequest("POST", path, bytes.NewReader(payloadBytes))
	if postErr == nil {
		return nil
	}

	// 409 = property already exists; fetch it to get its ID and version, then PUT.
	if statusCode == 409 {
		propID, version, getErr := c.getPagePropertyIDAndVersion(pageID, key)
		if getErr != nil {
			return fmt.Errorf("get existing property %q: %w", key, getErr)
		}
		updatePayload := map[string]any{
			"key":   key,
			"value": value,
			"version": map[string]any{
				"number": version + 1,
			},
		}
		updateBytes, _ := json.Marshal(updatePayload)
		putPath := fmt.Sprintf("/api/v2/pages/%s/properties/%s", pageID, propID)
		_, _, putErr := c.doRequest("PUT", putPath, bytes.NewReader(updateBytes))
		return putErr
	}

	return postErr
}

// getPagePropertyIDAndVersion fetches a page property by key and returns
// its ID and current version number.
func (c *ConfluenceClient) getPagePropertyIDAndVersion(pageID, key string) (id string, version int, err error) {
	path := fmt.Sprintf("/api/v2/pages/%s/properties?key=%s", pageID, url.QueryEscape(key))
	data, _, reqErr := c.doRequest("GET", path, nil)
	if reqErr != nil {
		return "", 0, reqErr
	}
	var resp struct {
		Results []struct {
			ID      string `json:"id"`
			Version struct {
				Number int `json:"number"`
			} `json:"version"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", 0, fmt.Errorf("parse properties response: %w", err)
	}
	for _, r := range resp.Results {
		return r.ID, r.Version.Number, nil
	}
	return "", 0, fmt.Errorf("property %q not found for page %s", key, pageID)
}

// ExtractBodyFromMCPResponse unwraps the body from an MCP getConfluencePage
// response, which may arrive in two shapes:
//
//  1. MCP envelope: [{type:"text", text:"<JSON string>"}]
//     The inner JSON string is parsed and .body extracted.
//
//  2. Bare page JSON: the response is already a page object with a .body field.
//
// Returns the raw JSON of the .body field, or an error.
func ExtractBodyFromMCPResponse(data []byte) ([]byte, error) {
	data = bytes.TrimSpace(data)

	// Shape 1: MCP envelope — starts with '['
	if len(data) > 0 && data[0] == '[' {
		var envelope []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, fmt.Errorf("parse MCP envelope: %w", err)
		}
		if len(envelope) == 0 {
			return nil, fmt.Errorf("MCP envelope is empty")
		}
		// Find the first text item
		for _, item := range envelope {
			if item.Type == "text" && item.Text != "" {
				return extractBodyFromPageJSON([]byte(item.Text))
			}
		}
		return nil, fmt.Errorf("no text item found in MCP envelope")
	}

	// Shape 2: bare page JSON
	return extractBodyFromPageJSON(data)
}

// extractBodyFromPageJSON extracts .body from a Confluence page JSON object.
func extractBodyFromPageJSON(data []byte) ([]byte, error) {
	var page struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parse page JSON: %w", err)
	}
	if page.Body == nil {
		return nil, fmt.Errorf("page JSON has no .body field")
	}

	// The body might be an object with atlas_doc_format or storage sub-keys,
	// or it might be the ADF doc directly. Try to get atlas_doc_format.value first.
	var bodyObj struct {
		AtlasDocFormat struct {
			Value string `json:"value"`
		} `json:"atlas_doc_format"`
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	}
	if err := json.Unmarshal(page.Body, &bodyObj); err == nil {
		if bodyObj.AtlasDocFormat.Value != "" {
			// The value is itself a JSON string (the ADF doc)
			return []byte(bodyObj.AtlasDocFormat.Value), nil
		}
		if bodyObj.Storage.Value != "" {
			return nil, fmt.Errorf("page body is in 'storage' (XHTML) format, not ADF. Re-fetch with body-format=atlas_doc_format")
		}
	}

	// Maybe body IS the ADF doc directly (type=doc)
	var docCheck struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(page.Body, &docCheck); err == nil && docCheck.Type == "doc" {
		return page.Body, nil
	}

	return nil, fmt.Errorf("could not extract ADF from body — unexpected shape: %s", string(page.Body)[:min(200, len(page.Body))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// AddLabels attaches one or more "global" labels to a page. Existing labels
// are preserved; duplicates are ignored by the Confluence API.
//
// Uses the v1 endpoint POST /wiki/rest/api/content/{id}/label — v2 exposes
// only a read endpoint for labels (mai/2026).
func (c *ConfluenceClient) AddLabels(pageID string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	type labelPayload struct {
		Prefix string `json:"prefix"`
		Name   string `json:"name"`
	}
	body := make([]labelPayload, 0, len(labels))
	for _, l := range labels {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		body = append(body, labelPayload{Prefix: "global", Name: l})
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}
	path := fmt.Sprintf("/rest/api/content/%s/label", pageID)
	_, _, err = c.doRequest("POST", path, bytes.NewReader(payload))
	return err
}

// GetLabels returns the current "global" labels attached to a page.
func (c *ConfluenceClient) GetLabels(pageID string) ([]string, error) {
	path := fmt.Sprintf("/api/v2/pages/%s/labels?prefix=global&limit=250", pageID)
	data, _, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode labels: %w", err)
	}
	out := make([]string, 0, len(resp.Results))
	for _, r := range resp.Results {
		out = append(out, r.Name)
	}
	return out, nil
}

// RemoveLabel detaches a single label from a page.
func (c *ConfluenceClient) RemoveLabel(pageID, label string) error {
	path := fmt.Sprintf("/rest/api/content/%s/label/%s", pageID, url.PathEscape(label))
	_, _, err := c.doRequest("DELETE", path, nil)
	return err
}
