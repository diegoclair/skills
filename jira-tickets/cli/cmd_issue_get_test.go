package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunIssueGet_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueGet([]string{"--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}
	out := stdout.String()
	if !strings.Contains(out, "--key") {
		t.Errorf("help output missing '--key'; got: %s", out)
	}
	if !strings.Contains(out, "--json") {
		t.Errorf("help output missing '--json'; got: %s", out)
	}
	if !strings.Contains(out, "--fields") {
		t.Errorf("help output missing '--fields'; got: %s", out)
	}
}

func TestRunIssueGet_HelpShortFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueGet([]string{"-h"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}
}

func TestRunIssueGet_MissingKey(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueGet([]string{}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing --key, got nil")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "--key") {
		t.Errorf("stderr should mention '--key'; got: %s", errOut)
	}
}

func TestRunIssueGet_MissingKey_WithOtherFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueGet([]string{"--json"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing --key, got nil")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

func TestRunIssueGet_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueGet([]string{"--key", "PROJ-1", "--unknown"}, &stdout, &stderr)
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

func TestRunIssueGet_EmptyCreds(t *testing.T) {
	t.Setenv("ATLASSIAN_CLOUD", "")
	t.Setenv("ATLASSIAN_EMAIL", "")
	t.Setenv("ATLASSIAN_API_TOKEN", "")

	var stdout, stderr bytes.Buffer
	code, _ := runIssueGet([]string{"--key", "PROJ-1"}, &stdout, &stderr)
	if code == exitOK {
		t.Fatal("expected non-zero exit code when no credentials are configured")
	}
	errOut := stderr.String()
	if errOut == "" {
		t.Error("expected helpful message in stderr when credentials are missing")
	}
}

func TestRunIssueGet_KeyMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueGet([]string{"--key"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --key has no value")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

func TestRunIssueGet_FieldsMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueGet([]string{"--key", "PROJ-1", "--fields"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --fields has no value")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}
