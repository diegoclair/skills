package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/diegoclair/skills/pkg/atlassian/jira"
)

func TestRunIssueDigest_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueDigest([]string{"--help"}, &stdout, &stderr)
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
	if !strings.Contains(out, "digest") {
		t.Errorf("help output missing 'digest'; got: %s", out)
	}
}

func TestRunIssueDigest_HelpShortFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueDigest([]string{"-h"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}
}

func TestRunIssueDigest_MissingKey(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueDigest([]string{}, &stdout, &stderr)
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

func TestRunIssueDigest_MissingKey_WithJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueDigest([]string{"--json"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing --key, got nil")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

func TestRunIssueDigest_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueDigest([]string{"--key", "PROJ-1", "--no-such-flag"}, &stdout, &stderr)
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

func TestRunIssueDigest_EmptyCreds(t *testing.T) {
	t.Setenv("ATLASSIAN_CLOUD", "")
	t.Setenv("ATLASSIAN_EMAIL", "")
	t.Setenv("ATLASSIAN_API_TOKEN", "")

	var stdout, stderr bytes.Buffer
	code, _ := runIssueDigest([]string{"--key", "PROJ-1"}, &stdout, &stderr)
	if code == exitOK {
		t.Fatal("expected non-zero exit code when no credentials are configured")
	}
	errOut := stderr.String()
	if errOut == "" {
		t.Error("expected helpful message in stderr when credentials are missing")
	}
}

func TestRunIssueDigest_KeyMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runIssueDigest([]string{"--key"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --key has no value")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

// TestBuildDigest_TextOutput verifies the text rendering of buildDigest
// without making any HTTP calls.
func TestBuildDigest_TextOutput(t *testing.T) {
	issue := &jira.Issue{
		Key: "PROJ-42",
		Fields: jira.IssueFields{
			Summary: "Fix the login flow",
			Status: jira.Status{
				Name: "In Progress",
				StatusCategory: jira.StatusCategory{
					Key:  "indeterminate",
					Name: "In Progress",
				},
			},
			Issuetype: jira.IssueType{Name: "Task", Subtask: false},
			Priority:  &jira.Priority{Name: "High"},
			Assignee:  &jira.User{DisplayName: "Alice"},
			Reporter:  &jira.User{DisplayName: "Bob"},
			Parent:    &jira.IssueRef{Key: "EPIC-10"},
			Labels:    []string{"backend", "p1"},
			Updated:   "2026-05-13T09:15:42.000-0300",
			DueDate:   "2026-06-01",
		},
	}

	// buildDigest requires a client only for issueWebURL; pass nil-safe variant.
	d := buildDigest(nil, issue)

	if d.Key != "PROJ-42" {
		t.Errorf("Key mismatch: got %q", d.Key)
	}
	if d.Summary != "Fix the login flow" {
		t.Errorf("Summary mismatch: got %q", d.Summary)
	}
	if d.Status != "In Progress" {
		t.Errorf("Status mismatch: got %q", d.Status)
	}
	if d.Category != "indeterminate" {
		t.Errorf("Category mismatch: got %q", d.Category)
	}
	if d.Priority != "High" {
		t.Errorf("Priority mismatch: got %q", d.Priority)
	}
	if d.Assignee != "Alice" {
		t.Errorf("Assignee mismatch: got %q", d.Assignee)
	}
	if d.Reporter != "Bob" {
		t.Errorf("Reporter mismatch: got %q", d.Reporter)
	}
	if d.Parent != "EPIC-10" {
		t.Errorf("Parent mismatch: got %q", d.Parent)
	}
	if len(d.Labels) != 2 || d.Labels[0] != "backend" {
		t.Errorf("Labels mismatch: got %v", d.Labels)
	}
	if d.Updated != "2026-05-13" {
		t.Errorf("Updated should be date-only; got %q", d.Updated)
	}
	if d.DueDate != "2026-06-01" {
		t.Errorf("DueDate mismatch: got %q", d.DueDate)
	}

	// Check text output contains expected rows.
	var buf bytes.Buffer
	printDigestText(&buf, d)
	out := buf.String()

	for _, want := range []string{"PROJ-42", "Fix the login flow", "In Progress", "High", "Alice", "Bob", "EPIC-10", "backend", "2026-05-13", "2026-06-01"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q; got:\n%s", want, out)
		}
	}
}

// TestBuildDigest_SummaryTruncation verifies long summaries are truncated.
func TestBuildDigest_SummaryTruncation(t *testing.T) {
	longSummary := strings.Repeat("A", 130)
	issue := &jira.Issue{
		Key: "PROJ-1",
		Fields: jira.IssueFields{
			Summary: longSummary,
			Updated: "2026-05-13T00:00:00.000-0300",
		},
	}
	d := buildDigest(nil, issue)
	if len(d.Summary) > 123 { // 120 + "..."
		t.Errorf("Summary should be truncated to 123 chars max; got %d: %s", len(d.Summary), d.Summary)
	}
	if !strings.HasSuffix(d.Summary, "...") {
		t.Errorf("Truncated summary should end with '...'; got %q", d.Summary)
	}
}

// TestBuildDigest_NilOptionalFields verifies nil fields produce empty strings.
func TestBuildDigest_NilOptionalFields(t *testing.T) {
	issue := &jira.Issue{
		Key: "PROJ-99",
		Fields: jira.IssueFields{
			Summary:   "Minimal issue",
			Assignee:  nil,
			Reporter:  nil,
			Priority:  nil,
			Parent:    nil,
			Labels:    nil,
			Updated:   "2026-01-01T00:00:00.000Z",
			DueDate:   "",
		},
	}
	d := buildDigest(nil, issue)
	if d.Priority != "" {
		t.Errorf("Priority should be empty for nil Priority; got %q", d.Priority)
	}
	if d.Assignee != "" {
		t.Errorf("Assignee should be empty for nil Assignee; got %q", d.Assignee)
	}
	if d.Reporter != "" {
		t.Errorf("Reporter should be empty for nil Reporter; got %q", d.Reporter)
	}
	if d.Parent != "" {
		t.Errorf("Parent should be empty for nil Parent; got %q", d.Parent)
	}
	if d.Labels != nil {
		t.Errorf("Labels should be nil for empty labels; got %v", d.Labels)
	}
	if d.DueDate != "" {
		t.Errorf("DueDate should be empty string; got %q", d.DueDate)
	}

	// Text output: Assignee should still say "Unassigned"
	var buf bytes.Buffer
	printDigestText(&buf, d)
	out := buf.String()
	if !strings.Contains(out, "Unassigned") {
		t.Errorf("text output should say 'Unassigned' for nil assignee; got:\n%s", out)
	}
}
