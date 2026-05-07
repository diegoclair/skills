package adf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveCreds_Explicit(t *testing.T) {
	creds, err := ResolveCreds("test@example.com", "tok123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Email != "test@example.com" || creds.Token != "tok123" {
		t.Errorf("unexpected creds: %+v", creds)
	}
}

func TestResolveCreds_EnvVars(t *testing.T) {
	t.Setenv("ATLASSIAN_API_TOKEN", "envtok")
	t.Setenv("ATLASSIAN_EMAIL", "env@example.com")
	creds, err := ResolveCreds("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Email != "env@example.com" || creds.Token != "envtok" {
		t.Errorf("unexpected creds: %+v", creds)
	}
}

func TestResolveCreds_ConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "confluence-docs")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "credentials")
	content := "email=file@example.com\ntoken=filetok\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	t.Setenv("ATLASSIAN_API_TOKEN", "")
	t.Setenv("ATLASSIAN_EMAIL", "")

	creds, err := ResolveCreds("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Email != "file@example.com" || creds.Token != "filetok" {
		t.Errorf("unexpected creds from file: %+v", creds)
	}
}

func TestResolveCreds_NoneFound(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)
	t.Setenv("ATLASSIAN_API_TOKEN", "")
	t.Setenv("ATLASSIAN_EMAIL", "")

	_, err := ResolveCreds("", "")
	if err == nil {
		t.Fatal("expected error when no credentials found")
	}
	if !strings.Contains(err.Error(), "ATLASSIAN_API_TOKEN") {
		t.Errorf("error should mention env var, got: %v", err)
	}
}

func TestResolveCloud(t *testing.T) {
	if got := ResolveCloud("mycloud"); got != "mycloud" {
		t.Errorf("want 'mycloud', got %q", got)
	}

	t.Setenv("ATLASSIAN_CLOUD", "envcloud")
	if got := ResolveCloud(""); got != "envcloud" {
		t.Errorf("want 'envcloud', got %q", got)
	}

	t.Setenv("ATLASSIAN_CLOUD", "")
	if got := ResolveCloud(""); got != "lybel" {
		t.Errorf("want default 'lybel', got %q", got)
	}
}

func TestExtractBodyFromMCPResponse_Envelope(t *testing.T) {
	// Simulate MCP envelope: [{type:"text", text:"<page JSON string>"}]
	innerDoc := `{"type":"doc","attrs":{"version":1},"content":[]}`
	innerDocEncoded, _ := json.Marshal(innerDoc) // JSON-encode the doc as a string value

	innerPage := map[string]any{
		"id": "123",
		"body": map[string]any{
			"atlas_doc_format": map[string]any{
				"value":          string(innerDocEncoded[1 : len(innerDocEncoded)-1]), // strip outer quotes
				"representation": "atlas_doc_format",
			},
		},
	}
	// Build the inner page JSON string, then embed it in the envelope
	pageBytes, _ := json.Marshal(innerPage)
	// Fix: the value should be the raw string, not JSON-encoded again
	innerPageFixed := `{"id":"123","body":{"atlas_doc_format":{"value":` + string(mustJSONMarshal(innerDoc)) + `,"representation":"atlas_doc_format"}}}`

	envelope := []map[string]any{{"type": "text", "text": innerPageFixed}}
	envelopeBytes, _ := json.Marshal(envelope)
	_ = pageBytes

	result, err := ExtractBodyFromMCPResponse(envelopeBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != innerDoc {
		t.Errorf("want %q, got %q", innerDoc, string(result))
	}
}

func TestExtractBodyFromMCPResponse_BarePageJSON(t *testing.T) {
	innerDoc := `{"type":"doc","attrs":{"version":1},"content":[]}`
	pageJSON := `{"id":"123","body":{"atlas_doc_format":{"value":` + string(mustJSONMarshal(innerDoc)) + `,"representation":"atlas_doc_format"}}}`

	result, err := ExtractBodyFromMCPResponse([]byte(pageJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != innerDoc {
		t.Errorf("want %q, got %q", innerDoc, string(result))
	}
}

func TestExtractBodyFromMCPResponse_DirectDocBody(t *testing.T) {
	// Body is the ADF doc directly (not nested in atlas_doc_format)
	pageJSON := `{"id":"123","body":{"type":"doc","attrs":{"version":1},"content":[]}}`
	result, err := ExtractBodyFromMCPResponse([]byte(pageJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(result), `"type":"doc"`) {
		t.Errorf("expected doc body, got %q", string(result))
	}
}

// mustJSONMarshal returns the JSON encoding of v, panicking on error.
func mustJSONMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
