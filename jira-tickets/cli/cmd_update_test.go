package main

import (
	"strings"
	"testing"
)

func TestRunUpdate_HelpFlag(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		t.Run(flag, func(t *testing.T) {
			var out strings.Builder
			var errOut strings.Builder
			code, err := runUpdate([]string{flag}, &out, &errOut)
			if err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if code != exitOK {
				t.Fatalf("expected exit code %d, got %d", exitOK, code)
			}
			if !strings.Contains(out.String(), "update") {
				t.Errorf("expected help output to contain 'update', got:\n%s", out.String())
			}
		})
	}
}

func TestRunUpdate_UnknownFlag(t *testing.T) {
	var out strings.Builder
	var errOut strings.Builder
	code, err := runUpdate([]string{"--unknown-flag"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected non-nil error for unknown flag")
	}
	if code == exitOK {
		t.Fatalf("expected non-zero exit code for unknown flag, got %d", code)
	}
	if !strings.Contains(errOut.String(), "unknown flag") {
		t.Errorf("expected stderr to contain 'unknown flag', got:\n%s", errOut.String())
	}
}
