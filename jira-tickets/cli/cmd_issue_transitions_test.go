package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunIssueTransitions_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueTransitions([]string{"--help"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exitOK, got %d", code)
	}
	if !strings.Contains(stdout.String(), "--key") {
		t.Errorf("help output missing --key: %s", stdout.String())
	}
}

func TestRunIssueTransitions_MissingKey(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueTransitions([]string{}, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--key") {
		t.Errorf("expected --key error, got: %s", stderr.String())
	}
}

func TestRunIssueTransitions_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, _ := runIssueTransitions([]string{"--not-a-flag"}, &stdout, &stderr)
	if code != exitInputErr {
		t.Fatalf("expected exitInputErr, got %d", code)
	}
}

// Note: transitions is a read command — no --dry-run flag; real API path
// requires live credentials. The tests above cover the no-API paths.
