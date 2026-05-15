package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunProjectGet_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectGet([]string{"--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}
	out := stdout.String()
	if !strings.Contains(out, "--json") {
		t.Errorf("help output missing '--json'; got: %s", out)
	}
}

func TestRunProjectGet_HelpShortFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectGet([]string{"-h"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d", exitOK, code)
	}
}

func TestRunProjectGet_MissingKey(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectGet([]string{}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when project key is missing")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
	if !strings.Contains(stderr.String(), "KEY is required") {
		t.Errorf("stderr should mention KEY required; got: %s", stderr.String())
	}
}

func TestRunProjectGet_UnexpectedExtraArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runProjectGet([]string{"LYBEL", "EXTRA"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for extra positional arg")
	}
	if code != exitInputErr {
		t.Fatalf("expected exit code %d, got %d", exitInputErr, code)
	}
}

