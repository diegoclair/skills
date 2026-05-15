package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunProjectUpdate_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectUpdate([]string{"--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}
	out := stdout.String()
	if !strings.Contains(out, "--name") {
		t.Errorf("help output missing '--name'; got: %s", out)
	}
	if !strings.Contains(out, "--key") {
		t.Errorf("help output missing '--key'; got: %s", out)
	}
	if !strings.Contains(out, "--description") {
		t.Errorf("help output missing '--description'; got: %s", out)
	}
	if !strings.Contains(out, "WARNING") {
		t.Errorf("help output missing key-rename WARNING; got: %s", out)
	}
	if !strings.Contains(out, "--dry-run") {
		t.Errorf("help output missing '--dry-run'; got: %s", out)
	}
}

func TestRunProjectUpdate_HelpShortFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectUpdate([]string{"-h"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}
}

func TestRunProjectUpdate_MissingProjectKey(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectUpdate([]string{"--name", "New Name"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when project key is missing")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

func TestRunProjectUpdate_NoFieldsProvided(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectUpdate([]string{"LYBEL"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when no --name/--key/--description provided")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
	if !strings.Contains(stderr.String(), "at least one of") {
		t.Errorf("stderr should mention 'at least one of'; got: %s", stderr.String())
	}
}

func TestRunProjectUpdate_InvalidNewKey(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{"lowercase", "lybel"},
		{"too short", "A"},
		{"too long", "ABCDEFGHIJK"},
		{"starts with digit", "1LYBEL"},
		{"contains space", "LY BEL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code, err := runProjectUpdate([]string{"LYBEL", "--key", tc.key}, &stdout, &stderr)
			if err == nil {
				t.Fatalf("expected error for invalid key %q, got nil", tc.key)
			}
			if code != exitInputErr {
				t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
			}
			if !strings.Contains(stderr.String(), "--key must match") {
				t.Errorf("stderr should mention key validation; got: %s", stderr.String())
			}
		})
	}
}

func TestRunProjectUpdate_ValidKeyFormats(t *testing.T) {
	cases := []string{"AB", "LYBEL", "LY_BEL", "AB0123456789"[0:10], "A1"}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			if !validProjectKey.MatchString(key) {
				t.Errorf("expected %q to be valid", key)
			}
		})
	}
}

func TestRunProjectUpdate_DryRun_NameOnly(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectUpdate([]string{"LYBEL", "--name", "Lybel Platform", "--dry-run"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}
	out := stdout.String()

	var result map[string]any
	if jErr := json.Unmarshal([]byte(out), &result); jErr != nil {
		t.Fatalf("dry-run output is not valid JSON: %v\nout: %s", jErr, out)
	}
	if result["dry_run"] != true {
		t.Errorf("dry_run: want true, got %v", result["dry_run"])
	}
	if result["method"] != "PUT" {
		t.Errorf("method: want PUT, got %v", result["method"])
	}
	body, _ := result["body"].(map[string]any)
	if body["name"] != "Lybel Platform" {
		t.Errorf("body.name: want 'Lybel Platform', got %v", body["name"])
	}
	if _, hasKey := body["key"]; hasKey {
		t.Error("body.key should be absent when --key not set")
	}
}

func TestRunProjectUpdate_DryRun_AllFields(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectUpdate([]string{
		"LYBEL",
		"--name", "New Name",
		"--key", "NEWKEY",
		"--description", "A description",
		"--dry-run",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}

	var result map[string]any
	if jErr := json.Unmarshal([]byte(stdout.String()), &result); jErr != nil {
		t.Fatalf("dry-run output is not valid JSON: %v", jErr)
	}
	body, _ := result["body"].(map[string]any)
	if body["name"] != "New Name" {
		t.Errorf("body.name: want 'New Name', got %v", body["name"])
	}
	if body["key"] != "NEWKEY" {
		t.Errorf("body.key: want 'NEWKEY', got %v", body["key"])
	}
	if body["description"] != "A description" {
		t.Errorf("body.description: want 'A description', got %v", body["description"])
	}
}

func TestRunProjectUpdate_DryRun_SkipsCreds(t *testing.T) {
	t.Setenv("ATLASSIAN_CLOUD", "")
	t.Setenv("ATLASSIAN_EMAIL", "")
	t.Setenv("ATLASSIAN_API_TOKEN", "")

	var stdout, stderr bytes.Buffer
	code, err := runProjectUpdate([]string{"LYBEL", "--name", "X", "--dry-run"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("dry-run should not require credentials; got error: %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d (stderr: %s)", exitOK, code, stderr.String())
	}
}

func TestRunProjectUpdate_EmptyCreds(t *testing.T) {
	t.Setenv("ATLASSIAN_CLOUD", "")
	t.Setenv("ATLASSIAN_EMAIL", "")
	t.Setenv("ATLASSIAN_API_TOKEN", "")

	var stdout, stderr bytes.Buffer
	code, _ := runProjectUpdate([]string{"LYBEL", "--name", "X"}, &stdout, &stderr)
	if code == exitOK {
		t.Fatal("expected non-zero exit code when no credentials are configured")
	}
	if stderr.String() == "" {
		t.Error("expected helpful message in stderr when credentials are missing")
	}
}

func TestRunProjectUpdate_NameMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectUpdate([]string{"LYBEL", "--name"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --name has no value")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

func TestRunProjectUpdate_KeyMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectUpdate([]string{"LYBEL", "--key"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --key has no value")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}
