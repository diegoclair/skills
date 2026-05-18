// Package main — cmd_check implements the `social-carousel check` sub-command.
//
// Usage:
//
//	social-carousel check <input.yaml> [--json] [--strict]
//
// check loads a Carousel YAML, resolves the matching Theme from the embedded
// templates, runs [LintCarousel], and prints either human-readable text or
// JSON output. Exit codes follow the exitOK / exitLintFailed constants.
package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// runCheck is the testable entry point for `social-carousel check`.
// It parses flags, loads the carousel, resolves the theme, lints, and
// prints results to stdout / stderr as appropriate.
func runCheck(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		inputFile string
		jsonOut   bool
		strict    bool
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--strict":
			strict = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "check — lint a carousel YAML against 32 viral-carousel rules.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  social-carousel check <input.yaml>           # human text output")
			fmt.Fprintln(stdout, "  social-carousel check <input.yaml> --json    # machine-readable JSON")
			fmt.Fprintln(stdout, "  social-carousel check <input.yaml> --strict  # warnings also fail")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "EXIT CODES:")
			fmt.Fprintln(stdout, "  0  no errors (warnings are ok unless --strict)")
			fmt.Fprintln(stdout, "  4  errors found (or --strict and warnings found)")
			return exitOK, nil
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				fmt.Fprintln(stderr, "unknown flag:", args[i])
				return exitInputErr, errInvalidUsage
			}
			if inputFile != "" {
				fmt.Fprintln(stderr, "unexpected argument:", args[i])
				return exitInputErr, errInvalidUsage
			}
			inputFile = args[i]
		}
	}

	if inputFile == "" {
		fmt.Fprintln(stderr, "check: missing <input.yaml> argument")
		fmt.Fprintln(stderr, "usage: social-carousel check <input.yaml> [--json] [--strict]")
		return exitInputErr, errInvalidUsage
	}

	// Load carousel.
	c, err := loadCarousel(inputFile)
	if err != nil {
		fmt.Fprintln(stderr, "check:", err)
		return exitUnknownErr, err
	}

	// Resolve theme using the shared loader in theme.go.
	// Theme load failure is non-fatal for lint — contrast rules are skipped.
	theme, err := loadTheme(c.Theme)
	if err != nil {
		fmt.Fprintf(stderr, "check: warning: could not load theme %q: %v — contrast rules skipped\n", c.Theme, err)
		theme = nil
	}

	// Run linter.
	report := LintCarousel(c, theme)

	if jsonOut {
		return printCheckJSON(report, strict, stdout, stderr)
	}
	return printCheckText(report, strict, stdout)
}

// ---------------------------------------------------------------------------
// Output: human-readable text
// ---------------------------------------------------------------------------

// printCheckText writes the lint report in human-readable format.
//
// Example output:
//
//	✗ slide-1 [C1]: hook has 18 words (max 12) — use H-01 or H-13
//	⚠ carousel [AP-07]: last slide is not a CTA — ...
//
//	2 errors, 1 warning. Render blocked. Use --force to ignore.
func printCheckText(report LintReport, strict bool, stdout io.Writer) (int, error) {
	for _, issue := range report.Issues {
		loc := slideLabel(issue.SlideIdx)
		var prefix string
		if issue.Severity == SeverityErr {
			prefix = "✗"
		} else {
			prefix = "⚠"
		}
		line := fmt.Sprintf("%s %s [%s]: %s", prefix, loc, issue.Code, issue.Message)
		if issue.Hint != "" {
			line += " — " + issue.Hint
		}
		fmt.Fprintln(stdout, line)
	}

	if len(report.Issues) > 0 {
		fmt.Fprintln(stdout, "")
	}

	blocked := report.ErrCount > 0 || (strict && report.WarnCount > 0)

	summary := fmt.Sprintf("%d errors, %d warnings.", report.ErrCount, report.WarnCount)
	switch {
	case blocked:
		summary += " Render blocked. Use --force to ignore."
	case report.WarnCount > 0:
		summary += " Render allowed (warnings do not block)."
	default:
		summary += " All good."
	}
	fmt.Fprintln(stdout, summary)

	if blocked {
		return exitLintFailed, nil
	}
	return exitOK, nil
}

// slideLabel returns a human-readable slide reference for the given 0-based
// index. Index -1 means carousel-level.
func slideLabel(idx int) string {
	if idx == -1 {
		return "carousel"
	}
	return fmt.Sprintf("slide-%d", idx+1)
}

// ---------------------------------------------------------------------------
// Output: JSON
// ---------------------------------------------------------------------------

// jsonIssue is the JSON representation of a single lint issue.
type jsonIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	SlideIdx int    `json:"slide_idx"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
}

// jsonReport is the top-level JSON output structure.
type jsonReport struct {
	Issues    []jsonIssue `json:"issues"`
	ErrCount  int         `json:"err_count"`
	WarnCount int         `json:"warn_count"`
}

// printCheckJSON serialises the report to JSON on stdout.
func printCheckJSON(report LintReport, strict bool, stdout, stderr io.Writer) (int, error) {
	issues := make([]jsonIssue, 0, len(report.Issues))
	for _, issue := range report.Issues {
		sev := "warn"
		if issue.Severity == SeverityErr {
			sev = "error"
		}
		issues = append(issues, jsonIssue{
			Code:     issue.Code,
			Severity: sev,
			SlideIdx: issue.SlideIdx,
			Message:  issue.Message,
			Hint:     issue.Hint,
		})
	}
	out := jsonReport{
		Issues:    issues,
		ErrCount:  report.ErrCount,
		WarnCount: report.WarnCount,
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(stderr, "check: JSON encode:", err)
		return exitUnknownErr, err
	}
	blocked := report.ErrCount > 0 || (strict && report.WarnCount > 0)
	if blocked {
		return exitLintFailed, nil
	}
	return exitOK, nil
}
