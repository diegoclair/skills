package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunIssueUpdate_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueUpdate([]string{"--help"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d", code)
	}
	if !strings.Contains(stdout.String(), "--key") {
		t.Errorf("help output missing --key: %s", stdout.String())
	}
}

func TestRunIssueUpdate_MissingKey(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueUpdate([]string{"--set", "summary=New"}, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--key") {
		t.Errorf("expected --key error, got: %s", stderr.String())
	}
}

func TestRunIssueUpdate_MissingSet(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueUpdate([]string{"--key", "PROJ-1"}, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--set") {
		t.Errorf("expected --set error, got: %s", stderr.String())
	}
}

func TestRunIssueUpdate_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueUpdate([]string{"--not-a-flag"}, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
}

func TestRunIssueUpdate_DryRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueUpdate([]string{
		"--dry-run",
		"--key", "PROJ-42",
		"--set", "summary=Updated title",
		"--set", "priority=High",
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d (stderr: %s)", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"status":"dry-run"`) {
		t.Errorf("expected dry-run status in output: %s", out)
	}
	if !strings.Contains(out, `"updated"`) {
		t.Errorf("expected updated field in output: %s", out)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected no stderr on dry-run, got: %s", stderr.String())
	}
}

func TestRunIssueUpdate_DryRun_NoCreds(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// dry-run with --set but no --key: should still bypass buildClient.
	code, _ := runIssueUpdate([]string{
		"--dry-run",
		"--set", "summary=test",
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("dry-run should bypass creds, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status":"dry-run"`) {
		t.Errorf("expected dry-run JSON, got: %s", stdout.String())
	}
}

func TestRunIssueUpdate_InvalidSetFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueUpdate([]string{
		"--dry-run",
		"--set", "noseparator",
	}, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr for invalid --set format, got %d", code)
	}
}

func TestRunIssueUpdate_LabelListParsed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueUpdate([]string{
		"--dry-run",
		"--key", "PROJ-1",
		"--set", "labels=bug,ux,accessibility",
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d (stderr: %s)", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "bug") {
		t.Errorf("expected label bug in output: %s", out)
	}
}
