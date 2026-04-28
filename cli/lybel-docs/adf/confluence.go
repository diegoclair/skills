// Package adf - Confluence Cloud REST API v2 HTTP client.
//
// Auth is resolved in order:
//  1. Explicit token/email passed to NewClient
//  2. $ATLASSIAN_API_TOKEN and $ATLASSIAN_EMAIL env vars
//  3. Platform config dir / lybel-docs / credentials (key=value file)
//     Linux:   $XDG_CONFIG_HOME/lybel-docs/credentials
//     macOS:   ~/Library/Application Support/lybel-docs/credentials
//     Windows: %AppData%\lybel-docs\credentials
//     (legacy ~/.config/lybel-docs/credentials is read with a warning)
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
	"os"
	"path/filepath"
	"strings"
)

// configPath returns the platform-appropriate credentials file path via
// os.UserConfigDir(), which resolves to:
//   - Linux:   $XDG_CONFIG_HOME/lybel-docs/credentials (or ~/.config/…)
//   - macOS:   ~/Library/Application Support/lybel-docs/credentials
//   - Windows: %AppData%\lybel-docs\credentials
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lybel-docs", "credentials"), nil
}

// legacyConfigPath returns the pre-migration path ~/.config/lybel-docs/credentials.
func legacyConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lybel-docs", "credentials"), nil
}

// ConfluenceCreds holds authentication credentials for the Confluence API.
type ConfluenceCreds struct {
	Email string
	Token string
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
	cfgPathHint := "~/.config/lybel-docs/credentials"
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
			"Run `lybel-docs setup` to create this file interactively.\n"+
			"See SETUP.md for how to generate an API token.",
		cfgPathHint,
	)
}

// readCredsFile parses the platform credentials file (key=value format).
// Falls back to the legacy ~/.config/lybel-docs/credentials with a warning
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
					"warning: credentials found at legacy path %s — run `lybel-docs setup` to migrate to %s\n",
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
			return nil, resp.StatusCode, fmt.Errorf("Confluence API error %d: %s", resp.StatusCode, apiErr.Message)
		}
		// Truncate body for readability
		body := string(respBody)
		if len(body) > 300 {
			body = body[:300] + "..."
		}
		return nil, resp.StatusCode, fmt.Errorf("Confluence API returned %d: %s", resp.StatusCode, body)
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

// BaseURL returns the base URL of this client.
func (c *ConfluenceClient) BaseURL() string {
	return c.baseURL
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
