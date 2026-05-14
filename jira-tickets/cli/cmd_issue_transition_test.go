package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunIssueTransition_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueTransition([]string{"--help"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d", code)
	}
	if !strings.Contains(stdout.String(), "--key") {
		t.Errorf("help output missing --key: %s", stdout.String())
	}
}

func TestRunIssueTransition_MissingKey(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueTransition([]string{"--to", "In Progress"}, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--key") {
		t.Errorf("expected --key error, got: %s", stderr.String())
	}
}

func TestRunIssueTransition_MissingTo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueTransition([]string{"--key", "PROJ-1"}, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--to") {
		t.Errorf("expected --to error, got: %s", stderr.String())
	}
}

func TestRunIssueTransition_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueTransition([]string{"--not-a-flag"}, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
}

func TestRunIssueTransition_DryRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueTransition([]string{
		"--dry-run",
		"--key", "PROJ-123",
		"--to", "In Progress",
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d (stderr: %s)", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"status":"dry-run"`) {
		t.Errorf("expected dry-run status in output: %s", out)
	}
	if !strings.Contains(out, `"action":"transition"`) {
		t.Errorf("expected action:transition in output: %s", out)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected no stderr on dry-run, got: %s", stderr.String())
	}
}

func TestRunIssueTransition_DryRun_NoCreds(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// dry-run should bypass buildClient entirely.
	code, _ := runIssueTransition([]string{
		"--dry-run",
		"--key", "PROJ-999",
		"--to", "Done",
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("dry-run should bypass creds, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status":"dry-run"`) {
		t.Errorf("expected dry-run JSON, got: %s", stdout.String())
	}
}
