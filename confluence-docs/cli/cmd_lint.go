package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/lybel-app/skills/pkg/atlassian/adf"
)

func runLint(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	var file string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-h", "--help":
			fmt.Fprint(stdout, helpText)
			return exitOK, nil
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintln(stderr, "unknown flag:", a)
				return exitInputErr, errInvalidUsage
			}
			file = a
		}
	}

	adfBytes, err := readADFInput(file, stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, err
	}

	doc, err := adf.UnmarshalDoc(adfBytes)
	if err != nil {
		fmt.Fprintln(stderr, "invalid ADF:", err)
		return exitParseErr, err
	}

	results := adf.Lint(doc)
	errorCount := adf.WriteLintResults(results, stderr)

	if errorCount > 0 {
		return exitParseErr, nil
	}
	if len(results) == 0 {
		fmt.Fprintln(stdout, "ok — no issues found")
	}
	return exitOK, nil
}

// runExtractBody unwraps ADF body from an MCP envelope or bare page JSON.
