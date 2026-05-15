package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunProjectList_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectList([]string{"--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}
	out := stdout.String()
	if !strings.Contains(out, "--limit") {
		t.Errorf("help output missing '--limit'; got: %s", out)
	}
	if !strings.Contains(out, "--json") {
		t.Errorf("help output missing '--json'; got: %s", out)
	}
}

func TestRunProjectList_HelpShortFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectList([]string{"-h"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}
}

func TestRunProjectList_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectList([]string{"--unknown"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
	if !strings.Contains(stderr.String(), "unknown flag") {
		t.Errorf("stderr should mention 'unknown flag'; got: %s", stderr.String())
	}
}

func TestRunProjectList_LimitMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectList([]string{"--limit"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --limit has no value")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

func TestRunProjectList_LimitInvalidValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectList([]string{"--limit", "0"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --limit is 0")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

func TestRunProjectList_LimitTooHigh(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectList([]string{"--limit", "101"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --limit > 100")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

func TestRunProjectList_StartAtMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectList([]string{"--start-at"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --start-at has no value")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

