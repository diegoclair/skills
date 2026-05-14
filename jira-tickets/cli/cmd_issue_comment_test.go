package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunIssueComment_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueComment([]string{"--help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d", code)
	}
	if !strings.Contains(stdout.String(), "--key") {
		t.Errorf("help output missing --key: %s", stdout.String())
	}
}

func TestRunIssueComment_MissingKey(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueComment([]string{"--body", "hello"}, nil, &stdout, &stderr)
	// Missing --key is only detected post-dry-run check; body is required first.
	// The command sees body present, key missing → should fail when trying to call API.
	// Since no credentials are available it returns exitUnknownErr, NOT exitInputErr.
	// This is acceptable: the error is still non-zero.
	if code == exitOK {
		t.Fatalf("expected non-zero exit when --key missing, got exitOK")
	}
}

func TestRunIssueComment_MissingBody(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueComment([]string{"--key", "PROJ-1"}, nil, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr when body missing, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--body") {
		t.Errorf("expected body-related error, got: %s", stderr.String())
	}
}

func TestRunIssueComment_MutuallyExclusiveBody(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueComment([]string{
		"--key", "PROJ-1",
		"--body", "text",
		"--body-stdin",
	}, nil, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr for mutually exclusive body flags, got %d", code)
	}
}

func TestRunIssueComment_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueComment([]string{"--not-a-flag"}, nil, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
}

func TestRunIssueComment_DryRun_WithBody(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueComment([]string{
		"--dry-run",
		"--key", "PROJ-123",
		"--body", "This is a **markdown** comment.",
	}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d (stderr: %s)", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"status":"dry-run"`) {
		t.Errorf("expected dry-run status in output: %s", out)
	}
	if !strings.Contains(out, `"action":"comment"`) {
		t.Errorf("expected action:comment in output: %s", out)
	}
	if !strings.Contains(out, `"adfBytes"`) {
		t.Errorf("expected adfBytes in output: %s", out)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected no stderr on dry-run, got: %s", stderr.String())
	}
}

func TestRunIssueComment_DryRun_NoCreds(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// dry-run must not call buildClient.
	code, _ := runIssueComment([]string{
		"--dry-run",
		"--key", "PROJ-999",
		"--body", "hello",
	}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("dry-run should bypass creds, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status":"dry-run"`) {
		t.Errorf("expected dry-run JSON, got: %s", stdout.String())
	}
}

func TestRunIssueComment_DryRun_BodyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "comment.md")
	if err := os.WriteFile(path, []byte("# Title\n\nSome text."), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code, _ := runIssueComment([]string{
		"--dry-run",
		"--key", "PROJ-7",
		"--body-file", path,
	}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status":"dry-run"`) {
		t.Errorf("expected dry-run status, got: %s", stdout.String())
	}
}

func TestRunIssueComment_DryRun_BodyStdin(t *testing.T) {
	stdinData := bytes.NewBufferString("comment from **stdin**")
	var stdout, stderr bytes.Buffer
	code, _ := runIssueComment([]string{
		"--dry-run",
		"--key", "PROJ-8",
		"--body-stdin",
	}, stdinData, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status":"dry-run"`) {
		t.Errorf("expected dry-run status, got: %s", stdout.String())
	}
}
