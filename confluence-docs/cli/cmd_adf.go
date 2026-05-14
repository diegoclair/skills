package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/lybel-app/skills/pkg/atlassian/adf"
)

// runADF parses adf-subcommand flags and performs the conversion.
func runADF(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	var (
		file   string
		pretty bool
	)

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-f", "--file":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag", a, "requires a value")
				return exitInputErr, errInvalidUsage
			}
			file = args[i+1]
			i++
		case "--pretty":
			pretty = true
		case "-h", "--help":
			fmt.Fprint(stdout, helpText)
			return exitOK, nil
		default:
			if strings.HasPrefix(a, "--file=") {
				file = a[7:]
				continue
			}
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	src, err := readInput(file, stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, err
	}

	doc, err := adf.Convert(src)
	if err != nil {
		fmt.Fprintln(stderr, "parse error:", err)
		return exitParseErr, err
	}

	return writeJSON(doc, pretty, stdout, stderr)
}
