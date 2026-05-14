package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/lybel-app/skills/pkg/atlassian/adf"
)

type multiOp struct {
	Kind       string `json:"kind"`
	Heading    string `json:"heading,omitempty"`
	AtLevel    int    `json:"atLevel,omitempty"`
	Fragment   string `json:"fragment,omitempty"`
	Row        string `json:"row,omitempty"`
	AfterRow   string `json:"afterRow,omitempty"`
	MatchCell  string `json:"matchCell,omitempty"`
	MatchCol   string `json:"matchCol,omitempty"`
	MatchValue string `json:"matchValue,omitempty"`
	ColName    string `json:"colName,omitempty"`
	Value      string `json:"value,omitempty"`
	IfMissing  bool   `json:"ifMissing,omitempty"`
}

// multiSpec is the top-level schema for an --multi JSON file.
type multiSpec struct {
	Message    string    `json:"message,omitempty"`
	Operations []multiOp `json:"operations"`
}

// applyOp runs a single batch op against a doc and returns the mutated doc.
// The returned (skipped, error): skipped is true for benign no-ops (e.g.
// table-add-row with --if-missing where the row already exists). error is
// non-nil for hard failures that must abort the batch.
func applyOp(doc adf.Node, op multiOp, fragment []adf.Node) (adf.Node, bool, error) {
	switch op.Kind {
	case "append":
		return adf.Append(doc, fragment), false, nil
	case "replace-intro":
		out, err := adf.ReplaceIntro(doc, fragment)
		return out, false, err
	case "insert-after":
		out, err := adf.InsertAfterAtLevel(doc, op.Heading, op.AtLevel, fragment)
		return out, false, err
	case "insert-before":
		out, err := adf.InsertBeforeAtLevel(doc, op.Heading, op.AtLevel, fragment)
		return out, false, err
	case "replace-section":
		out, err := adf.ReplaceSectionAtLevel(doc, op.Heading, op.AtLevel, fragment)
		return out, false, err
	case "delete-section":
		out, err := adf.DeleteSectionAtLevel(doc, op.Heading, op.AtLevel)
		return out, false, err
	case "table-add-row":
		spec, _, mErr := buildMatchSpec(op.MatchCell, op.MatchCol, op.MatchValue)
		if mErr != nil {
			return doc, false, mErr
		}
		out, existed, err := adf.TableAddRow(doc, op.Heading, op.AtLevel, op.Row, op.AfterRow, op.IfMissing, spec)
		if existed {
			return doc, true, nil
		}
		return out, false, err
	case "table-remove-row":
		spec, _, mErr := buildMatchSpec(op.MatchCell, op.MatchCol, op.MatchValue)
		if mErr != nil {
			return doc, false, mErr
		}
		out, err := adf.TableRemoveRow(doc, op.Heading, op.AtLevel, spec)
		return out, false, err
	case "table-update-row":
		spec, _, mErr := buildMatchSpec(op.MatchCell, op.MatchCol, op.MatchValue)
		if mErr != nil {
			return doc, false, mErr
		}
		out, err := adf.TableUpdateRow(doc, op.Heading, op.AtLevel, spec, op.Row)
		return out, false, err
	case "table-update-cell":
		spec, _, mErr := buildMatchSpec(op.MatchCell, op.MatchCol, op.MatchValue)
		if mErr != nil {
			return doc, false, mErr
		}
		out, err := adf.TableUpdateCell(doc, op.Heading, op.AtLevel, spec, op.ColName, op.Value)
		return out, false, err
	default:
		return doc, false, fmt.Errorf("unknown op kind %q", op.Kind)
	}
}

// loadMultiFragment loads the markdown fragment file for an op, or returns
// nil if the op kind doesn't take a fragment.
func loadMultiFragment(op multiOp) ([]adf.Node, error) {
	needsFragment := false
	switch op.Kind {
	case "append", "replace-intro", "insert-after", "insert-before", "replace-section":
		needsFragment = true
	}
	if !needsFragment {
		return nil, nil
	}
	if op.Fragment == "" {
		return nil, fmt.Errorf("op kind %q requires a fragment file", op.Kind)
	}
	src, err := os.ReadFile(op.Fragment)
	if err != nil {
		return nil, fmt.Errorf("reading fragment %s: %w", op.Fragment, err)
	}
	nodes, err := adf.ConvertFragment(src)
	if err != nil {
		return nil, fmt.Errorf("parse fragment %s: %w", op.Fragment, err)
	}
	return nodes, nil
}

// validateMultiOp returns an error if required per-kind fields are missing.
func validateMultiOp(op multiOp) error {
	switch op.Kind {
	case "append":
		if op.Fragment == "" {
			return fmt.Errorf("append requires fragment")
		}
	case "replace-intro":
		if op.Fragment == "" {
			return fmt.Errorf("replace-intro requires fragment")
		}
	case "insert-after", "insert-before", "replace-section":
		if op.Heading == "" {
			return fmt.Errorf("%s requires heading", op.Kind)
		}
		if op.Fragment == "" {
			return fmt.Errorf("%s requires fragment", op.Kind)
		}
	case "delete-section":
		if op.Heading == "" {
			return fmt.Errorf("delete-section requires heading")
		}
	case "table-add-row":
		if op.Heading == "" {
			return fmt.Errorf("table-add-row requires heading")
		}
		if op.Row == "" {
			return fmt.Errorf("table-add-row requires row")
		}
	case "table-remove-row":
		if op.Heading == "" {
			return fmt.Errorf("table-remove-row requires heading")
		}
		if _, provided, mErr := buildMatchSpec(op.MatchCell, op.MatchCol, op.MatchValue); mErr != nil {
			return fmt.Errorf("table-remove-row: %w", mErr)
		} else if !provided {
			return fmt.Errorf("table-remove-row requires matchCell (or matchCol + matchValue)")
		}
	case "table-update-row":
		if op.Heading == "" {
			return fmt.Errorf("table-update-row requires heading")
		}
		if _, provided, mErr := buildMatchSpec(op.MatchCell, op.MatchCol, op.MatchValue); mErr != nil {
			return fmt.Errorf("table-update-row: %w", mErr)
		} else if !provided {
			return fmt.Errorf("table-update-row requires matchCell (or matchCol + matchValue)")
		}
		if op.Row == "" {
			return fmt.Errorf("table-update-row requires row")
		}
	case "table-update-cell":
		if op.Heading == "" {
			return fmt.Errorf("table-update-cell requires heading")
		}
		if _, provided, mErr := buildMatchSpec(op.MatchCell, op.MatchCol, op.MatchValue); mErr != nil {
			return fmt.Errorf("table-update-cell: %w", mErr)
		} else if !provided {
			return fmt.Errorf("table-update-cell requires matchCell (or matchCol + matchValue)")
		}
		if op.ColName == "" {
			return fmt.Errorf("table-update-cell requires colName")
		}
		if op.Value == "" {
			return fmt.Errorf("table-update-cell requires value")
		}
	default:
		return fmt.Errorf("unknown op kind %q", op.Kind)
	}
	return nil
}

// opMatchDescription renders the human-friendly form of a multiOp's row
// match (first-cell or by-column) for log/summary output.
func opMatchDescription(op multiOp) string {
	if op.MatchCol != "" {
		return fmt.Sprintf("col %q=%q", op.MatchCol, op.MatchValue)
	}
	return fmt.Sprintf("%q", op.MatchCell)
}

// opSummary returns a short human-readable description for a multi op (used
// in --dry-run and rewrite output).
func opSummary(op multiOp) string {
	switch op.Kind {
	case "append":
		return "append fragment to end"
	case "replace-intro":
		return "replace intro (pre-heading content)"
	case "insert-after":
		return fmt.Sprintf("insert after %q", op.Heading)
	case "insert-before":
		return fmt.Sprintf("insert before %q", op.Heading)
	case "replace-section":
		return fmt.Sprintf("replace section %q", op.Heading)
	case "delete-section":
		return fmt.Sprintf("delete section %q", op.Heading)
	case "table-add-row":
		return fmt.Sprintf("add row to table in %q", op.Heading)
	case "table-remove-row":
		return fmt.Sprintf("remove row from table in %q", op.Heading)
	case "table-update-row":
		return fmt.Sprintf("update row in table in %q (match %s)", op.Heading, opMatchDescription(op))
	case "table-update-cell":
		return fmt.Sprintf("update cell in table in %q (row %s, col %q)", op.Heading, opMatchDescription(op), op.ColName)
	default:
		return op.Kind
	}
}

// runMultiApply runs a batch of ops atomically against a page: GET → apply
// all in-memory → PUT. Returns (fromVersion, toVersion, title, applied, error).
//
// On 409 conflict it refetches and replays the WHOLE batch once (matching the
// single-op retry policy). If any op fails the batch is aborted (no PUT).
//
// fragments is parallel to ops and pre-loaded; pass nil entries for ops that
// don't take a fragment. dryRun skips the PUT.
func runMultiApply(client *adf.ConfluenceClient, pageID, message string, ops []multiOp, fragments [][]adf.Node, dryRun bool, stdout, stderr io.Writer) (fromVersion, toVersion int, title string, opsApplied int, err error) {
	for attempt := 0; attempt < 2; attempt++ {
		meta, gErr := client.GetPage(pageID, "atlas_doc_format")
		if gErr != nil {
			return 0, 0, "", 0, fmt.Errorf("fetching page: %w", gErr)
		}
		if meta.Body.AtlasDocFormat.Value == "" {
			return 0, 0, "", 0, fmt.Errorf("page has no ADF body")
		}
		doc, dErr := adf.UnmarshalDoc([]byte(meta.Body.AtlasDocFormat.Value))
		if dErr != nil {
			return 0, 0, "", 0, fmt.Errorf("parse ADF: %w", dErr)
		}

		// Apply each op sequentially in memory.
		current := doc
		applied := 0
		for i, op := range ops {
			next, skipped, opErr := applyOp(current, op, fragments[i])
			if opErr != nil {
				// Abort: report which op (1-indexed) failed. Error message
				// from adf already includes the heading list + shell-expansion
				// hint when applicable (see adf.sectionNotFoundError).
				fmt.Fprintf(stderr, "op %d (%s) failed: %v\n", i+1, op.Kind, opErr)
				return meta.Version.Number, 0, meta.Title, 0, fmt.Errorf("op %d (%s): %w", i+1, op.Kind, opErr)
			}
			current = next
			if skipped {
				fmt.Fprintf(stderr, "notice: op %d (%s in %q) — skipped (already exists)\n", i+1, op.Kind, op.Heading)
			} else {
				applied++
			}
		}

		title = meta.Title
		fromVersion = meta.Version.Number
		toVersion = meta.Version.Number + 1
		opsApplied = applied

		uErr := client.UpdatePage(pageID, meta.Title, toVersion, current, message, dryRun, stderr)
		if uErr == nil {
			return fromVersion, toVersion, title, opsApplied, nil
		}
		if adf.IsConflict(uErr) && attempt == 0 {
			fmt.Fprintln(stderr, "notice: page version changed during apply — refetching and retrying once")
			continue
		}
		return fromVersion, 0, title, 0, fmt.Errorf("update failed: %w", uErr)
	}
	// Unreachable.
	return 0, 0, "", 0, fmt.Errorf("retry exhausted")
}

// runPageApply atomically applies a section-level edit to a Confluence page:
// GET (fresh ADF) → edit (in memory) → PUT. On 409 conflict (someone else
// updated the page in the meantime), it refetches and retries once. The full
// ADF never leaves the binary — the caller only sees a tiny status line.
//
// Supports the same operations as `edit` (--append, --insert-after,
// --insert-before, --replace-section, --delete-section), but takes a page ID
// instead of an ADF file.
func runPageApply(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		pageID           string
		op               editOp
		heading          string
		fragmentPath     string
		atLevel          int
		message          string
		dryRun           bool
		rowText          string
		afterRow         string
		matchCell        string
		matchCol         string
		matchValue       string
		colName          string
		newValue         string
		ifMissing        bool
		multiPath        string
		tablePosition    int  // 1-indexed data-row position for --table-move-row
		tablePositionSet bool // tracks whether --position was passed
	)

	setOp := func(newOp editOp) error {
		if op != opNone {
			return fmt.Errorf("multiple operations specified; use only one")
		}
		op = newOp
		return nil
	}

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--page-id":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--page-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			pageID = remaining[i+1]
			i++
		case "--message":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--message requires a value")
				return exitInputErr, errInvalidUsage
			}
			message = remaining[i+1]
			i++
		case "--at-level":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--at-level requires a value (1-6)")
				return exitInputErr, errInvalidUsage
			}
			n, atErr := strconv.Atoi(remaining[i+1])
			if atErr != nil || n < 1 || n > 6 {
				fmt.Fprintln(stderr, "--at-level must be an integer between 1 and 6")
				return exitInputErr, errInvalidUsage
			}
			atLevel = n
			i++
		case "--fragment":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--fragment requires a file path")
				return exitInputErr, errInvalidUsage
			}
			fragmentPath = remaining[i+1]
			i++
		case "--multi":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--multi requires a JSON file path")
				return exitInputErr, errInvalidUsage
			}
			multiPath = remaining[i+1]
			i++
		case "--dry-run":
			dryRun = true
		case "--row":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--row requires a value")
				return exitInputErr, errInvalidUsage
			}
			rowText = remaining[i+1]
			i++
		case "--after-row":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--after-row requires a value")
				return exitInputErr, errInvalidUsage
			}
			afterRow = remaining[i+1]
			i++
		case "--match-cell":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--match-cell requires a value")
				return exitInputErr, errInvalidUsage
			}
			matchCell = remaining[i+1]
			i++
		case "--match-col":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--match-col requires a value")
				return exitInputErr, errInvalidUsage
			}
			matchCol = remaining[i+1]
			i++
		case "--match-value":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--match-value requires a value")
				return exitInputErr, errInvalidUsage
			}
			matchValue = remaining[i+1]
			i++
		case "--col-name":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--col-name requires a value")
				return exitInputErr, errInvalidUsage
			}
			colName = remaining[i+1]
			i++
		case "--value":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--value requires a value")
				return exitInputErr, errInvalidUsage
			}
			newValue = remaining[i+1]
			i++
		case "--if-missing":
			ifMissing = true
		case "--append":
			if err := setOp(opAppend); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
		case "--replace-intro":
			if err := setOp(opReplaceIntro); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
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
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = remaining[i+1]
			i++
		case "--delete-section":
			if err := setOp(opDeleteSection); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = remaining[i+1]
			i++
		case "--table-add-row":
			if err := setOp(opTableAddRow); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = remaining[i+1]
			i++
		case "--table-remove-row":
			if err := setOp(opTableRemoveRow); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = remaining[i+1]
			i++
		case "--table-update-row":
			if err := setOp(opTableUpdateRow); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = remaining[i+1]
			i++
		case "--table-update-cell":
			if err := setOp(opTableUpdateCell); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = remaining[i+1]
			i++
		case "--table-move-row":
			if err := setOp(opTableMoveRow); err != nil {
				fmt.Fprintln(stderr, err)
				return exitInputErr, errInvalidUsage
			}
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, a, `requires "Heading"`)
				return exitInputErr, errInvalidUsage
			}
			heading = remaining[i+1]
			i++
		case "--position":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "flag --position requires an integer")
				return exitInputErr, errInvalidUsage
			}
			n, perr := strconv.Atoi(remaining[i+1])
			if perr != nil {
				fmt.Fprintln(stderr, "flag --position requires an integer, got:", remaining[i+1])
				return exitInputErr, errInvalidUsage
			}
			tablePosition = n
			tablePositionSet = true
			i++
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page apply: --page-id is required")
		return exitInputErr, errInvalidUsage
	}

	// --multi is mutually exclusive with single-op flags.
	if multiPath != "" {
		if op != opNone {
			fmt.Fprintln(stderr, "page apply: --multi is mutually exclusive with single-op flags")
			return exitInputErr, errInvalidUsage
		}
		return runPageApplyMulti(pageID, multiPath, message, dryRun, cloud, email, token, stdout, stderr)
	}

	if op == opNone {
		fmt.Fprintln(stderr, "page apply: no operation specified")
		fmt.Fprintln(stderr, "  use one of: --append, --insert-after, --insert-before, --replace-section, --delete-section, --replace-intro, --table-add-row, --table-remove-row, --multi")
		return exitInputErr, errInvalidUsage
	}
	// Operation-specific validation:
	switch op {
	case opAppend, opInsertAfter, opInsertBefore, opReplaceSection, opReplaceIntro:
		if fragmentPath == "" {
			fmt.Fprintln(stderr, "page apply: --fragment FILE is required for this operation")
			return exitInputErr, errInvalidUsage
		}
	case opTableAddRow:
		if rowText == "" {
			fmt.Fprintln(stderr, "page apply: --table-add-row requires --row \"col1|col2|...\"")
			return exitInputErr, errInvalidUsage
		}
	case opTableRemoveRow, opTableUpdateRow, opTableUpdateCell:
		// match-flag validation is shared below via buildMatchSpec.
	}

	// Resolve --match-cell vs --match-col/--match-value once for table ops.
	matchSpec, matchProvided, mErr := buildMatchSpec(matchCell, matchCol, matchValue)
	if mErr != nil {
		fmt.Fprintln(stderr, "page apply:", mErr)
		return exitInputErr, errInvalidUsage
	}
	switch op {
	case opTableRemoveRow:
		if !matchProvided {
			fmt.Fprintln(stderr, "page apply: --table-remove-row requires --match-cell \"text\" (or --match-col COL --match-value V)")
			return exitInputErr, errInvalidUsage
		}
	case opTableUpdateRow:
		if !matchProvided {
			fmt.Fprintln(stderr, "page apply: --table-update-row requires --match-cell \"text\" (or --match-col COL --match-value V)")
			return exitInputErr, errInvalidUsage
		}
		if rowText == "" {
			fmt.Fprintln(stderr, "page apply: --table-update-row requires --row \"col1|col2|...\"")
			return exitInputErr, errInvalidUsage
		}
	case opTableUpdateCell:
		if !matchProvided {
			fmt.Fprintln(stderr, "page apply: --table-update-cell requires --match-cell \"text\" (or --match-col COL --match-value V)")
			return exitInputErr, errInvalidUsage
		}
		if colName == "" {
			fmt.Fprintln(stderr, "page apply: --table-update-cell requires --col-name \"Header\"")
			return exitInputErr, errInvalidUsage
		}
		if newValue == "" {
			fmt.Fprintln(stderr, "page apply: --table-update-cell requires --value \"text\"")
			return exitInputErr, errInvalidUsage
		}
	case opTableMoveRow:
		if !matchProvided {
			fmt.Fprintln(stderr, "page apply: --table-move-row requires --match-cell \"text\" (or --match-col COL --match-value V)")
			return exitInputErr, errInvalidUsage
		}
		if !tablePositionSet {
			fmt.Fprintln(stderr, "page apply: --table-move-row requires --position N (1-indexed data-row position)")
			return exitInputErr, errInvalidUsage
		}
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	// Load the fragment once (its content doesn't change between retries).
	var fragment []adf.Node
	if fragmentPath != "" {
		src, frErr := os.ReadFile(fragmentPath)
		if frErr != nil {
			fmt.Fprintln(stderr, "reading fragment:", frErr)
			return exitInputErr, frErr
		}
		nodes, frErr := adf.ConvertFragment(src)
		if frErr != nil {
			fmt.Fprintln(stderr, "parse fragment:", frErr)
			return exitParseErr, frErr
		}
		fragment = nodes
	}

	// Attempt up to 2 times: first try, then one retry on 409.
	var lastFromVersion, lastToVersion int
	var lastTitle string
	for attempt := 0; attempt < 2; attempt++ {
		// 1. Always fetch fresh ADF before each PUT — never mutate stale state.
		meta, gErr := client.GetPage(pageID, "atlas_doc_format")
		if gErr != nil {
			fmt.Fprintln(stderr, "fetching page:", gErr)
			return exitUnknownErr, gErr
		}
		if meta.Body.AtlasDocFormat.Value == "" {
			fmt.Fprintln(stderr, "page has no ADF body — aborting")
			return exitUnknownErr, fmt.Errorf("empty ADF body")
		}
		doc, dErr := adf.UnmarshalDoc([]byte(meta.Body.AtlasDocFormat.Value))
		if dErr != nil {
			fmt.Fprintln(stderr, "parse ADF:", dErr)
			return exitParseErr, dErr
		}

		// 2. Apply the operation against fresh ADF.
		var result adf.Node
		var opErr error
		var rowExisted bool
		switch op {
		case opAppend:
			result = adf.Append(doc, fragment)
		case opReplaceIntro:
			result, opErr = adf.ReplaceIntro(doc, fragment)
		case opInsertAfter:
			result, opErr = adf.InsertAfterAtLevel(doc, heading, atLevel, fragment)
		case opInsertBefore:
			result, opErr = adf.InsertBeforeAtLevel(doc, heading, atLevel, fragment)
		case opReplaceSection:
			result, opErr = adf.ReplaceSectionAtLevel(doc, heading, atLevel, fragment)
		case opDeleteSection:
			result, opErr = adf.DeleteSectionAtLevel(doc, heading, atLevel)
		case opTableAddRow:
			result, rowExisted, opErr = adf.TableAddRow(doc, heading, atLevel, rowText, afterRow, ifMissing, matchSpec)
			if rowExisted {
				dedup := strings.SplitN(rowText, "|", 2)[0]
				if matchProvided && matchSpec.Col != "" {
					dedup = fmt.Sprintf("column %q=%q", matchSpec.Col, matchSpec.Value)
				} else if matchProvided {
					dedup = matchSpec.Value
				}
				fmt.Fprintf(stderr, "notice: row matching %s already exists in %q — skipped (--if-missing)\n",
					dedup, heading)
				fmt.Fprintf(stdout, `{"status":"skipped","reason":"row already exists","pageId":%q}`+"\n", pageID)
				return exitOK, nil
			}
		case opTableRemoveRow:
			result, opErr = adf.TableRemoveRow(doc, heading, atLevel, matchSpec)
		case opTableUpdateRow:
			result, opErr = adf.TableUpdateRow(doc, heading, atLevel, matchSpec, rowText)
		case opTableUpdateCell:
			result, opErr = adf.TableUpdateCell(doc, heading, atLevel, matchSpec, colName, newValue)
		case opTableMoveRow:
			result, opErr = adf.TableMoveRow(doc, heading, atLevel, matchSpec, tablePosition)
		}
		if opErr != nil {
			// For section ops, list the current top-level headings to help
			// All section error messages from adf now embed the heading
			// list and the shell-expansion hint themselves (see
			// adf.sectionNotFoundError), so we just surface the error.
			fmt.Fprintln(stderr, "operation failed:", opErr)
			// Return errInvalidUsage so main() prints a terse "confluence-docs:
			// invalid usage" instead of re-printing the (potentially long)
			// embedded heading list.
			return exitInputErr, errInvalidUsage
		}

		lastTitle = meta.Title
		lastFromVersion = meta.Version.Number
		lastToVersion = meta.Version.Number + 1

		// 3. Push the new ADF.
		uErr := client.UpdatePage(pageID, meta.Title, lastToVersion, result, message, dryRun, stderr)
		if uErr == nil {
			break // success
		}
		if adf.IsConflict(uErr) && attempt == 0 {
			// Someone else updated the page; retry once with fresh state.
			fmt.Fprintln(stderr, "notice: page version changed during apply — refetching and retrying once")
			continue
		}
		fmt.Fprintln(stderr, "update failed:", uErr)
		return exitUnknownErr, uErr
	}

	if dryRun {
		return exitOK, nil
	}
	// Auto-refresh the home cache if this write touched the Home page.
	refreshHomeCacheAfterWrite(pageID, client, stderr)

	url := pageWebURL(client, pageID)
	fmt.Fprintf(stdout, `{"status":"ok","pageId":%q,"title":%q,"fromVersion":%d,"toVersion":%d,"url":%q}`+"\n",
		pageID, lastTitle, lastFromVersion, lastToVersion, url)
	return exitOK, nil
}

// runPageApplyMulti loads a multi-op JSON file and applies it atomically.
// Called from runPageApply when --multi is set.
func runPageApplyMulti(pageID, multiPath, message string, dryRun bool, cloud, email, token string, stdout, stderr io.Writer) (int, error) {
	specBytes, err := os.ReadFile(multiPath)
	if err != nil {
		fmt.Fprintln(stderr, "reading multi spec:", err)
		return exitInputErr, err
	}
	var spec multiSpec
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		fmt.Fprintln(stderr, "parse multi spec:", err)
		return exitParseErr, err
	}
	if len(spec.Operations) == 0 {
		fmt.Fprintln(stderr, "page apply --multi: spec has no operations")
		return exitInputErr, errInvalidUsage
	}
	// CLI --message wins over spec.Message (per brief).
	if message == "" && spec.Message != "" {
		message = spec.Message
	}

	// Validate every op + load fragments up front.
	fragments := make([][]adf.Node, len(spec.Operations))
	for i, op := range spec.Operations {
		if err := validateMultiOp(op); err != nil {
			fmt.Fprintf(stderr, "op %d (%s): %v\n", i+1, op.Kind, err)
			return exitInputErr, errInvalidUsage
		}
		frag, err := loadMultiFragment(op)
		if err != nil {
			fmt.Fprintf(stderr, "op %d: %v\n", i+1, err)
			return exitInputErr, err
		}
		fragments[i] = frag
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	if dryRun {
		// Dry-run: still GET, apply in memory, print summary, but skip PUT.
		fromV, toV, title, applied, err := runMultiApply(client, pageID, message, spec.Operations, fragments, true, stdout, stderr)
		if err != nil {
			return exitInputErr, err
		}
		fmt.Fprintf(stderr, "dry-run: would apply %d ops to %q (v%d → v%d)\n",
			applied, title, fromV, toV)
		for i, op := range spec.Operations {
			fmt.Fprintf(stderr, "  %d. %s\n", i+1, opSummary(op))
		}
		return exitOK, nil
	}

	fromV, toV, title, applied, err := runMultiApply(client, pageID, message, spec.Operations, fragments, false, stdout, stderr)
	if err != nil {
		return exitInputErr, err
	}
	refreshHomeCacheAfterWrite(pageID, client, stderr)
	url := pageWebURL(client, pageID)
	fmt.Fprintf(stdout, `{"status":"ok","pageId":%q,"title":%q,"fromVersion":%d,"toVersion":%d,"opsApplied":%d,"url":%q}`+"\n",
		pageID, title, fromV, toV, applied, url)
	return exitOK, nil
}

// runPageRewrite splits a markdown file into sections by heading and matches
// against the current page's headings. For each match it emits a
// replace-section op; pre-heading content emits replace-intro. Mismatches
// (heading in markdown but not in page, or vice versa) are reported as
// warnings; --allow-add / --allow-remove flip them into actual ops.
//
// Then dispatches through the same multi-op atomic apply path.
