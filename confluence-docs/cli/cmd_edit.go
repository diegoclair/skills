package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/diegoclair/skills/pkg/atlassian/adf"
)

// editOp identifies which operation the edit subcommand will apply.
type editOp int

const (
	opNone editOp = iota
	opAppend
	opInsertAfter
	opInsertBefore
	opReplaceSection
	opDeleteSection
	opTableAddRow
	opTableRemoveRow
	opReplaceIntro
	opTableUpdateRow
	opTableUpdateCell
	opTableMoveRow
)

// buildMatchSpec resolves the --match-cell / --match-col / --match-value
// triplet into a single adf.MatchSpec. Returns (spec, provided, error):
//
//   - provided is false when no match flag was given (caller decides whether
//     that's an error for the current op).
//   - When --match-cell is set, --match-col / --match-value must be empty.
//   - When --match-col is set, --match-value is required (and vice-versa).
func buildMatchSpec(matchCell, matchCol, matchValue string) (adf.MatchSpec, bool, error) {
	hasCell := matchCell != ""
	hasCol := matchCol != ""
	hasVal := matchValue != ""

	if hasCell && (hasCol || hasVal) {
		return adf.MatchSpec{}, false, fmt.Errorf(
			"--match-cell is mutually exclusive with --match-col / --match-value; pick one mode")
	}
	if hasCol != hasVal {
		return adf.MatchSpec{}, false, fmt.Errorf(
			"--match-col and --match-value must be used together (both required for column-based match)")
	}
	if hasCell {
		return adf.FirstCellMatch(matchCell), true, nil
	}
	if hasCol {
		return adf.MatchSpec{Col: matchCol, Value: matchValue}, true, nil
	}
	return adf.MatchSpec{}, false, nil
}

// runEdit parses edit-subcommand flags and applies one section-level or
// table-level operation to the ADF doc read from stdin or --input.
//
// Fragment file path for section ops (--append, --insert-after, --insert-before,
// --replace-section) may be passed either:
//   - Immediately after the heading (legacy): --replace-section "H" frag.md
//   - As a trailing positional after all flags: --replace-section "H" --at-level 3 frag.md
//
// Both forms are accepted. The trailing positional takes priority if present.
func runEdit(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	var (
		input            string
		pretty           bool
		op               editOp
		heading          string
		fragmentPath     string
		atLevel          int
		rowText          string
		afterRow         string
		matchCell        string
		matchCol         string
		matchValue       string
		colName          string
		newValue         string
		ifMissing        bool
		tablePosition    int  // 1-indexed data-row position for --table-move-row
		tablePositionSet bool // tracks whether --position was passed
		// positionals collects non-flag arguments (only used for fragment path)
		positionals []string
	)

	setOp := func(newOp editOp) error {
		if op != opNone {
			return fmt.Errorf("multiple operations specified; use only one")
		}
		op = newOp
		return nil
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-i", "--input":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag", a, "requires a value")
				return exitInputErr, errInvalidUsage
			}
			input = args[i+1]
			i++
		case "--pretty":
			pretty = true
		case "--if-missing":
			ifMissing = true
		case "-h", "--help":
			fmt.Fprint(stdout, helpText)
			return exitOK, nil

		case "--at-level":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag --at-level requires a value (1-6)")
				return exitInputErr, errInvalidUsage
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > 6 {
				fmt.Fprintln(stderr, "--at-level must be an integer between 1 and 6")
				return exitInputErr, errInvalidUsage
			}
			atLevel = n
			i++

		case "--row":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag --row requires a value")
				return exitInputErr, errInvalidUsage
			}
			rowText = args[i+1]
			i++

		case "--after-row":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag --after-row requires a value")
				return exitInputErr, errInvalidUsage
			}
			afterRow = args[i+1]
			i++

		case "--match-cell":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag --match-cell requires a value")
				return exitInputErr, errInvalidUsage
			}
			matchCell = args[i+1]
			i++

		case "--match-col":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag --match-col requires a value")
				return exitInputErr, errInvalidUsage
			}
			matchCol = args[i+1]
			i++

		case "--match-value":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag --match-value requires a value")
				return exitInputErr, errInvalidUsage
			}
			matchValue = args[i+1]
			i++

		case "--append":
			if err := setOp(opAppend); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			// Fragment may follow immediately or be a trailing positional.
			// Peek at next arg: if it looks like a file (not a flag), grab it.
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				fragmentPath = args[i+1]
				i++
			}

		case "--replace-intro":
			if err := setOp(opReplaceIntro); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			// Fragment may follow immediately or be a trailing positional.
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				fragmentPath = args[i+1]
				i++
			}

		case "--insert-after", "--insert-before", "--replace-section":
			var newOp editOp
			switch a {
			case "--insert-after":
				newOp = opInsertAfter
			case "--insert-before":
				newOp = opInsertBefore
			case "--replace-section":
				newOp = opReplaceSection
			}
			if err := setOp(newOp); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = args[i+1]
			i++
			// Fragment may follow immediately (legacy) or as a trailing positional.
			// Only grab it if the next arg is not a flag.
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				fragmentPath = args[i+1]
				i++
			}

		case "--delete-section":
			if err := setOp(opDeleteSection); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = args[i+1]
			i++

		case "--table-add-row":
			if err := setOp(opTableAddRow); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = args[i+1]
			i++

		case "--table-remove-row":
			if err := setOp(opTableRemoveRow); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = args[i+1]
			i++

		case "--table-update-row":
			if err := setOp(opTableUpdateRow); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = args[i+1]
			i++

		case "--table-update-cell":
			if err := setOp(opTableUpdateCell); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = args[i+1]
			i++

		case "--table-move-row":
			if err := setOp(opTableMoveRow); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = args[i+1]
			i++

		case "--position":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag --position requires an integer (1-indexed data-row position)")
				return exitInputErr, errInvalidUsage
			}
			n, perr := strconv.Atoi(args[i+1])
			if perr != nil {
				fmt.Fprintln(stderr, "flag --position requires an integer, got:", args[i+1])
				return exitInputErr, errInvalidUsage
			}
			tablePosition = n
			tablePositionSet = true
			i++

		case "--col-name":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag --col-name requires a value")
				return exitInputErr, errInvalidUsage
			}
			colName = args[i+1]
			i++

		case "--value":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "flag --value requires a value")
				return exitInputErr, errInvalidUsage
			}
			newValue = args[i+1]
			i++

		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintln(stderr, "unknown flag:", a)
				return exitInputErr, errInvalidUsage
			}
			// Trailing positional (fragment path for section ops)
			positionals = append(positionals, a)
		}
	}

	// If a trailing positional was provided and fragmentPath wasn't set inline, use it.
	if len(positionals) > 0 && fragmentPath == "" {
		fragmentPath = positionals[0]
	} else if len(positionals) > 1 {
		fmt.Fprintln(stderr, "too many positional arguments")
		return exitInputErr, errInvalidUsage
	}

	if op == opNone {
		fmt.Fprintln(stderr, "edit: no operation specified")
		fmt.Fprint(stderr, helpText)
		return exitInputErr, errInvalidUsage
	}

	// Validate operation-specific required flags
	if op == opTableAddRow && rowText == "" {
		fmt.Fprintln(stderr, "--table-add-row requires --row \"col1|col2|...\"")
		return exitInputErr, errInvalidUsage
	}

	// Resolve the row-match flags once (shared across the 4 table ops).
	// --match-cell is the legacy "match first column" mode; --match-col +
	// --match-value picks an arbitrary column by header name. The two modes
	// are mutually exclusive.
	matchSpec, matchProvided, mErr := buildMatchSpec(matchCell, matchCol, matchValue)
	if mErr != nil {
		fmt.Fprintln(stderr, mErr)
		return exitInputErr, errInvalidUsage
	}

	if op == opTableAddRow && ifMissing && matchProvided {
		// Allowed: caller wants column-based dedup. Nothing to validate here.
	}
	if op == opTableRemoveRow && !matchProvided {
		fmt.Fprintln(stderr, "--table-remove-row requires --match-cell \"text\" (or --match-col COL --match-value V)")
		return exitInputErr, errInvalidUsage
	}
	if op == opTableUpdateRow {
		if !matchProvided {
			fmt.Fprintln(stderr, "--table-update-row requires --match-cell \"text\" (or --match-col COL --match-value V)")
			return exitInputErr, errInvalidUsage
		}
		if rowText == "" {
			fmt.Fprintln(stderr, "--table-update-row requires --row \"col1|col2|...\"")
			return exitInputErr, errInvalidUsage
		}
	}
	if op == opTableUpdateCell {
		if !matchProvided {
			fmt.Fprintln(stderr, "--table-update-cell requires --match-cell \"text\" (or --match-col COL --match-value V)")
			return exitInputErr, errInvalidUsage
		}
		if colName == "" {
			fmt.Fprintln(stderr, "--table-update-cell requires --col-name \"Header\"")
			return exitInputErr, errInvalidUsage
		}
		if newValue == "" {
			fmt.Fprintln(stderr, "--table-update-cell requires --value \"text\"")
			return exitInputErr, errInvalidUsage
		}
	}
	if op == opTableMoveRow {
		if !matchProvided {
			fmt.Fprintln(stderr, "--table-move-row requires --match-cell \"text\" (or --match-col COL --match-value V)")
			return exitInputErr, errInvalidUsage
		}
		if !tablePositionSet {
			fmt.Fprintln(stderr, "--table-move-row requires --position N (1-indexed data-row position)")
			return exitInputErr, errInvalidUsage
		}
	}

	adfBytes, err := readADFInput(input, stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, err
	}

	doc, err := adf.UnmarshalDoc(adfBytes)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitParseErr, err
	}

	var fragment []adf.Node
	if fragmentPath != "" {
		src, err := os.ReadFile(fragmentPath)
		if err != nil {
			fmt.Fprintln(stderr, "reading fragment:", err)
			return exitInputErr, err
		}
		nodes, err := adf.ConvertFragment(src)
		if err != nil {
			fmt.Fprintln(stderr, "parse fragment:", err)
			return exitParseErr, err
		}
		fragment = nodes
	}

	var result adf.Node
	switch op {
	case opAppend:
		result = adf.Append(doc, fragment)
	case opReplaceIntro:
		result, err = adf.ReplaceIntro(doc, fragment)
	case opInsertAfter:
		result, err = adf.InsertAfterAtLevel(doc, heading, atLevel, fragment)
	case opInsertBefore:
		result, err = adf.InsertBeforeAtLevel(doc, heading, atLevel, fragment)
	case opReplaceSection:
		result, err = adf.ReplaceSectionAtLevel(doc, heading, atLevel, fragment)
	case opDeleteSection:
		result, err = adf.DeleteSectionAtLevel(doc, heading, atLevel)
	case opTableAddRow:
		var existed bool
		result, existed, err = adf.TableAddRow(doc, heading, atLevel, rowText, afterRow, ifMissing, matchSpec)
		if existed {
			dedup := strings.SplitN(rowText, "|", 2)[0]
			if matchProvided && matchSpec.Col != "" {
				dedup = fmt.Sprintf("column %q=%q", matchSpec.Col, matchSpec.Value)
			} else if matchProvided {
				dedup = matchSpec.Value
			}
			fmt.Fprintf(stderr, "notice: row matching %s already exists in %q — skipped (--if-missing)\n",
				dedup, heading)
			// Still write the unchanged doc to stdout so callers can pipe
		}
	case opTableRemoveRow:
		result, err = adf.TableRemoveRow(doc, heading, atLevel, matchSpec)
	case opTableUpdateRow:
		result, err = adf.TableUpdateRow(doc, heading, atLevel, matchSpec, rowText)
	case opTableUpdateCell:
		result, err = adf.TableUpdateCell(doc, heading, atLevel, matchSpec, colName, newValue)
	case opTableMoveRow:
		result, err = adf.TableMoveRow(doc, heading, atLevel, matchSpec, tablePosition)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, err
	}

	return writeJSON(result, pretty, stdout, stderr)
}
