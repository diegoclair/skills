package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunIssueCreate_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueCreate([]string{"--help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d", code)
	}
	if !strings.Contains(stdout.String(), "--project") {
		t.Errorf("help output missing --project: %s", stdout.String())
	}
}

func TestRunIssueCreate_MissingProject(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueCreate([]string{"--type", "Task", "--summary", "test"}, nil, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--project") {
		t.Errorf("expected --project error, got: %s", stderr.String())
	}
}

func TestRunIssueCreate_MissingType(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueCreate([]string{"--project", "PROJ", "--summary", "test"}, nil, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--type") {
		t.Errorf("expected --type error, got: %s", stderr.String())
	}
}

func TestRunIssueCreate_MissingSummary(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueCreate([]string{"--project", "PROJ", "--type", "Task"}, nil, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--summary") {
		t.Errorf("expected --summary error, got: %s", stderr.String())
	}
}

func TestRunIssueCreate_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueCreate([]string{"--not-a-flag"}, nil, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
}

func TestRunIssueCreate_DryRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// --dry-run requires NO credentials.
	code, _ := runIssueCreate([]string{
		"--dry-run",
		"--project", "PROJ",
		"--type", "Task",
		"--summary", "Test issue",
	}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d (stderr: %s)", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"status":"dry-run"`) {
		t.Errorf("expected dry-run status in output: %s", out)
	}
	if !strings.Contains(out, `"action":"create"`) {
		t.Errorf("expected action:create in output: %s", out)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected no stderr on dry-run, got: %s", stderr.String())
	}
}

func TestRunIssueCreate_DryRun_NoCreds(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// No --project / --type / --summary — dry-run bypasses validation too.
	code, _ := runIssueCreate([]string{"--dry-run"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("dry-run without required flags should still exit OK, got %d", code)
	}
	if !strings.Contains(stdout.String(), `"status":"dry-run"`) {
		t.Errorf("expected dry-run JSON, got: %s", stdout.String())
	}
}

func TestRunIssueCreate_MutuallyExclusiveDescription(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueCreate([]string{
		"--project", "PROJ",
		"--type", "Task",
		"--summary", "test",
		"--description", "text",
		"--description-file", "path.md",
	}, nil, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr for mutually exclusive flags, got %d", code)
	}
}
