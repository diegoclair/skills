package main

import (
	"fmt"
	"io"
	"os"

	"github.com/lybel-app/skills/pkg/atlassian/adf"
)

func writeJSON(n adf.Node, pretty bool, stdout, stderr io.Writer) (int, error) {
	out, err := adf.Marshal(n, pretty)
	if err != nil {
		fmt.Fprintln(stderr, "marshal error:", err)
		return exitUnknownErr, err
	}
	if _, err := stdout.Write(out); err != nil {
		return exitUnknownErr, err
	}
	if pretty {
		fmt.Fprintln(stdout)
	}
	return exitOK, nil
}

// readInput returns markdown bytes from file (if provided) or from stdin.
func readInput(file string, stdin io.Reader) ([]byte, error) {
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", file, err)
		}
		return b, nil
	}
	return io.ReadAll(stdin)
}

// readADFInput returns ADF JSON bytes from file or stdin. "-" or empty means stdin.
func readADFInput(path string, stdin io.Reader) ([]byte, error) {
	if path == "" || path == "-" {
		return io.ReadAll(stdin)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return b, nil
}
