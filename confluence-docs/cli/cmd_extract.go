package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/lybel-app/skills/confluence-docs/cli/adf"
)

func runExtractBody(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
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

	data, err := readInput(file, stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, err
	}

	bodyJSON, err := adf.ExtractBodyFromMCPResponse(data)
	if err != nil {
		fmt.Fprintln(stderr, "extract-body:", err)
		return exitParseErr, err
	}

	if _, err := stdout.Write(bodyJSON); err != nil {
		return exitUnknownErr, err
	}
	fmt.Fprintln(stdout)
	return exitOK, nil
}
