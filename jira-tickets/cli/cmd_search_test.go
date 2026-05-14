package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunSearch_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runSearch([]string{"--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}
	out := stdout.String()
	if !strings.Contains(out, "search") {
		t.Errorf("help output missing 'search'; got: %s", out)
	}
	if !strings.Contains(out, "--limit") {
		t.Errorf("help output missing '--limit'; got: %s", out)
	}
	if !strings.Contains(out, "--json") {
		t.Errorf("help output missing '--json'; got: %s", out)
	}
}

func TestRunSearch_HelpShortFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runSearch([]string{"-h"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}
}

func TestRunSearch_MissingJQL(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runSearch([]string{}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing JQL, got nil")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "JQL") {
		t.Errorf("stderr should mention 'JQL'; got: %s", errOut)
	}
}

func TestRunSearch_MissingJQL_OnlyFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Only flags, no positional JQL
	code, err := runSearch([]string{"--limit", "10"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing JQL, got nil")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

func TestRunSearch_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runSearch([]string{"project = PROJ", "--bogus-flag"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "unknown flag") {
		t.Errorf("stderr should mention 'unknown flag'; got: %s", errOut)
	}
}

func TestRunSearch_LimitOutOfRange(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runSearch([]string{"project = PROJ", "--limit", "999"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for out-of-range limit, got nil")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

func TestRunSearch_LimitNotAnInt(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runSearch([]string{"project = PROJ", "--limit", "abc"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for non-integer limit, got nil")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

func TestRunSearch_EmptyCreds(t *testing.T) {
	// Unset env vars so buildClient falls back to file, which won't exist in CI.
	t.Setenv("ATLASSIAN_CLOUD", "")
	t.Setenv("ATLASSIAN_EMAIL", "")
	t.Setenv("ATLASSIAN_API_TOKEN", "")

	var stdout, stderr bytes.Buffer
	code, _ := runSearch([]string{"project = PROJ"}, &stdout, &stderr)
	// Should fail with non-zero exit (no credentials available).
	if code == exitOK {
		t.Fatal("expected non-zero exit code when no credentials are configured")
	}
	errOut := stderr.String()
	if errOut == "" {
		t.Error("expected helpful message in stderr when credentials are missing")
	}
}

func TestRunSearch_LimitMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runSearch([]string{"project = PROJ", "--limit"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --limit has no value")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

func TestRunSearch_NextPageTokenMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runSearch([]string{"project = PROJ", "--next-page-token"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --next-page-token has no value")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}
