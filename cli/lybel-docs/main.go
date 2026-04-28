// lybel-docs converts extended markdown (with Confluence macros) into ADF
// JSON for use with the Atlassian Confluence REST API, edits existing
// ADF documents by section without losing macros, and talks directly to
// the Confluence Cloud REST API v2 to get/upload/create pages.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/lybel-app/skills/cli/lybel-docs/adf"
	"github.com/lybel-app/skills/cli/lybel-docs/setup"
)

// version is injected at build time via -ldflags "-X main.version=..."
var version = "dev"

const helpText = `lybel-docs — Confluence ADF toolkit: convert, edit, lint, and publish pages.

USAGE:
  lybel-docs setup        [--email X --token Y | --check | --print-config-path]
  lybel-docs adf          [--file PATH] [--pretty]
  lybel-docs edit         [--input PATH] OPERATION [--at-level N] [--pretty]
  lybel-docs page         VERB [flags]
  lybel-docs lint         FILE.json
  lybel-docs extract-body [< mcp-response.json]
  lybel-docs index        VERB [flags]
  lybel-docs --version
  lybel-docs --help

COMMANDS:
  setup         Interactive wizard to configure Atlassian API credentials.
  adf           Convert markdown (stdin or --file) to an ADF JSON document.
  edit          Apply a section-level or table edit to an existing ADF doc.
  page          Fetch, upload, or create Confluence pages via HTTP (bypasses MCP).
  lint          Validate ADF structure and report errors/warnings.
  extract-body  Unwrap body from an MCP getConfluencePage response.
  index         Manage the Page ID Index table on the Home page.

SETUP FLAGS:
  (no flags)              Interactive wizard — prompts for email and API token.
  --email X --token Y     Non-interactive: save credentials without prompting.
  --check                 Validate stored credentials. Exit 0=valid, 1=missing,
                          2=invalid auth, 3=network error.
  --print-config-path     Print absolute path to credentials file and exit.
  --print-config-format   Print the expected credentials file format and exit.

EDIT OPERATIONS (exactly one required):
  --append FRAGMENT.md                    Append the fragment's blocks to the end.
  --insert-after  "Heading" FRAGMENT.md   Insert blocks right after the section.
  --insert-before "Heading" FRAGMENT.md   Insert blocks right before the section.
  --replace-section "Heading" FRAGMENT.md Replace the section with the fragment.
  --delete-section  "Heading"             Remove the heading and its body.
  --table-add-row "Heading" --row "a|b|c" Add a row to the table in the section.
  --table-remove-row "Heading" --match-cell "text"  Remove a row from the table.

EDIT FLAGS:
  -i, --input PATH   Read ADF from PATH instead of stdin. Use - for stdin.
      --at-level N   Match the heading only at level N (1-6). Default: first match.
      --after-row "text"  (--table-add-row) Insert after row whose first cell contains text.
      --if-missing   (--table-add-row) Skip silently if row with same first cell exists.
      --pretty       Pretty-print the JSON output.

PAGE VERBS:
  page get     --page-id ID [--cloud SUBDOMAIN] [--output FILE]
               [--format adf|storage|view|export_view]
               (markdown is an alias of export_view; html is an alias of view)
  page upload  --page-id ID --adf FILE [--title TITLE] [--message MSG] [--dry-run]
               [--cloud SUBDOMAIN] [--email EMAIL] [--token TOKEN]
  page create  --space-id ID --parent-id ID --title TITLE
               [--markdown FILE | --adf FILE] [--cloud SUBDOMAIN]
               [--email EMAIL] [--token TOKEN]

LINT:
  lybel-docs lint page.json
    Exits 0 if clean, 1 if errors found. Diagnostics on stderr.

EXTRACT-BODY:
  lybel-docs extract-body < mcp-response.json > adf-body.json
    Reads MCP envelope [{type:"text",text:...}] or bare page JSON.
    Outputs the ADF body JSON ready for 'edit'.

INDEX VERBS:
  index add    --page-id PAGE_ID --title TITLE --under "Section Heading"
               [--indent 0|1|2] [--input FILE] [--if-missing]
  index remove --page-id PAGE_ID [--input FILE]
  index sync   --parent-page-id ID --under "Section Heading" [--input FILE]

FLAGS:
  -f, --file  PATH   (adf) Read markdown from PATH instead of stdin.
  -i, --input PATH   (edit) Read ADF from PATH instead of stdin.
      --pretty       Pretty-print the JSON output.
  -v, --version      Print version and exit.
  -h, --help         Show this help and exit.

MARKDOWN EXTENSIONS (adf & edit fragments):
  [TOC]                              Confluence Table of Contents macro.
  [TOC maxLevel=3 minLevel=1]        With explicit min/max levels.
  :::expand Title                    Expand block; close with :::
  :::warning Title                   Panel of type warning/info/note/success/error.

EXAMPLES:
  # Convert markdown to ADF
  lybel-docs adf < page.md > page.adf.json

  # Append a new section (preserves all macros)
  lybel-docs edit --input page.json --append new-section.md > updated.json

  # Replace only the h3 "Ops" (not the h2 "Ops")
  lybel-docs edit --input page.json --replace-section "Ops" --at-level 3 fragment.md > out.json

  # Add a row to a table inside a section
  lybel-docs edit --input page.json --table-add-row "Page ID Index" \
    --row "My Page|987654" --if-missing > updated.json

  # Fetch a page as ADF
  lybel-docs page get --page-id 164232 --format adf --output current.json

  # Upload edited ADF back to Confluence (preview first, then commit)
  lybel-docs page upload --page-id 164232 --adf updated.json --dry-run
  lybel-docs page upload --page-id 164232 --adf updated.json --message "add row"

  # Create a new page
  lybel-docs page create --space-id 131352 --parent-id 164232 \
    --title "New Page" --markdown content.md

  # Unwrap MCP response body
  lybel-docs extract-body < mcp-response.json > body.json

  # Validate ADF structure
  lybel-docs lint page.json

  # Add entry to Home page index
  lybel-docs index add --page-id 999 --title "My Page" --under "Sócios"

EXIT CODES:
  0  success
  1  parse error (markdown -> ADF or ADF unmarshal)
  2  invalid input (missing file, bad flags, section not found)
  3  unknown error / HTTP error
`

const (
	exitOK         = 0
	exitParseErr   = 1
	exitInputErr   = 2
	exitUnknownErr = 3
)

// errInvalidUsage is returned when CLI flag parsing detects bad input.
var errInvalidUsage = errors.New("invalid usage")

func main() {
	code, err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lybel-docs:", err)
	}
	os.Exit(code)
}

// run is the testable entry point.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprint(stderr, helpText)
		return exitInputErr, errInvalidUsage
	}

	switch args[0] {
	case "-h", "--help":
		fmt.Fprint(stdout, helpText)
		return exitOK, nil
	case "-v", "--version":
		fmt.Fprintln(stdout, "lybel-docs", version)
		return exitOK, nil
	case "setup":
		return setup.Run(args[1:], stdin, stdout, stderr)
	case "adf":
		return runADF(args[1:], stdin, stdout, stderr)
	case "edit":
		return runEdit(args[1:], stdin, stdout, stderr)
	case "page":
		return runPage(args[1:], stdin, stdout, stderr)
	case "lint":
		return runLint(args[1:], stdin, stdout, stderr)
	case "extract-body":
		return runExtractBody(args[1:], stdin, stdout, stderr)
	case "index":
		return runIndex(args[1:], stdin, stdout, stderr)
	}

	fmt.Fprintln(stderr, "unknown command:", args[0])
	fmt.Fprint(stderr, helpText)
	return exitInputErr, errInvalidUsage
}

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
)

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
		input        string
		pretty       bool
		op           editOp
		heading      string
		fragmentPath string
		atLevel      int
		rowText      string
		afterRow     string
		matchCell    string
		ifMissing    bool
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
	if op == opTableRemoveRow && matchCell == "" {
		fmt.Fprintln(stderr, "--table-remove-row requires --match-cell \"text\"")
		return exitInputErr, errInvalidUsage
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
		result, existed, err = adf.TableAddRow(doc, heading, atLevel, rowText, afterRow, ifMissing)
		if existed {
			fmt.Fprintf(stderr, "notice: row with first cell %q already exists in %q — skipped (--if-missing)\n",
				strings.SplitN(rowText, "|", 2)[0], heading)
			// Still write the unchanged doc to stdout so callers can pipe
		}
	case opTableRemoveRow:
		result, err = adf.TableRemoveRow(doc, heading, atLevel, matchCell)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, err
	}

	return writeJSON(result, pretty, stdout, stderr)
}

// runPage handles the `page` subcommand with verbs: get, upload, create.
func runPage(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "page: requires a verb: get, upload, create")
		return exitInputErr, errInvalidUsage
	}
	switch args[0] {
	case "get":
		return runPageGet(args[1:], stdout, stderr)
	case "upload":
		return runPageUpload(args[1:], stdout, stderr)
	case "create":
		return runPageCreate(args[1:], stdout, stderr)
	case "list-children":
		return runPageListChildren(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "page: unknown verb:", args[0])
		fmt.Fprintln(stderr, "  valid verbs: get, upload, create, list-children")
		return exitInputErr, errInvalidUsage
	}
}

// parseCommonPageFlags parses --cloud, --email, --token flags shared by page verbs.
// Returns remaining args and the parsed values.
func parseCommonPageFlags(args []string) (remaining []string, cloud, email, token string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--cloud":
			if i+1 >= len(args) {
				return nil, "", "", "", fmt.Errorf("--cloud requires a value")
			}
			cloud = args[i+1]
			i++
		case "--email":
			if i+1 >= len(args) {
				return nil, "", "", "", fmt.Errorf("--email requires a value")
			}
			email = args[i+1]
			i++
		case "--token":
			if i+1 >= len(args) {
				return nil, "", "", "", fmt.Errorf("--token requires a value")
			}
			token = args[i+1]
			i++
		default:
			remaining = append(remaining, a)
		}
	}
	return remaining, cloud, email, token, nil
}

// buildClient resolves cloud+creds and returns a ready-to-use ConfluenceClient.
func buildClient(cloud, email, token string, stderr io.Writer) (*adf.ConfluenceClient, bool) {
	resolvedCloud := adf.ResolveCloud(cloud)
	creds, err := adf.ResolveCreds(email, token)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, false
	}
	return adf.NewClient(resolvedCloud, creds), true
}

func runPageGet(args []string, stdout, stderr io.Writer) (int, error) {
	var pageID, format, outputFile string

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
		case "--format":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--format requires a value (adf|markdown|html)")
				return exitInputErr, errInvalidUsage
			}
			format = remaining[i+1]
			i++
		case "--output":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--output requires a file path")
				return exitInputErr, errInvalidUsage
			}
			outputFile = remaining[i+1]
			i++
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page get: --page-id is required")
		return exitInputErr, errInvalidUsage
	}
	if format == "" {
		format = "adf"
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	// Map user-facing format to (Confluence API body-format, body field to read).
	// Confluence Cloud has no native markdown — "markdown" is an alias for
	// export_view (rendered HTML). For real markdown, fetch ADF and convert
	// downstream.
	var bodyFormat, fieldName string
	switch format {
	case "adf":
		bodyFormat, fieldName = "atlas_doc_format", "atlas_doc_format"
	case "storage":
		bodyFormat, fieldName = "storage", "storage"
	case "view", "html":
		bodyFormat, fieldName = "view", "view"
	case "export_view", "markdown":
		bodyFormat, fieldName = "export_view", "export_view"
	default:
		fmt.Fprintf(stderr, "unknown format %q — use adf, storage, view, export_view (markdown alias), or html\n", format)
		return exitInputErr, errInvalidUsage
	}

	meta, err := client.GetPage(pageID, bodyFormat)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownErr, err
	}

	var out []byte
	switch fieldName {
	case "atlas_doc_format":
		out = []byte(meta.Body.AtlasDocFormat.Value)
	case "storage":
		out = []byte(meta.Body.Storage.Value)
	case "view":
		out = []byte(meta.Body.View.Value)
	case "export_view":
		out = []byte(meta.Body.ExportView.Value)
	}
	if len(out) == 0 {
		fmt.Fprintf(stderr, "page has no %s body — try a different --format\n", fieldName)
		return exitUnknownErr, fmt.Errorf("empty %s body", fieldName)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, out, 0644); err != nil {
			fmt.Fprintln(stderr, "writing output:", err)
			return exitUnknownErr, err
		}
		fmt.Fprintf(stderr, "wrote %d bytes to %s\n", len(out), outputFile)
	} else {
		if _, err := stdout.Write(out); err != nil {
			return exitUnknownErr, err
		}
	}
	return exitOK, nil
}

func runPageUpload(args []string, stdout, stderr io.Writer) (int, error) {
	var pageID, adfFile, title, message string
	var dryRun bool

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
		case "--adf":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--adf requires a file path")
				return exitInputErr, errInvalidUsage
			}
			adfFile = remaining[i+1]
			i++
		case "--title":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--title requires a value")
				return exitInputErr, errInvalidUsage
			}
			title = remaining[i+1]
			i++
		case "--message":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--message requires a value")
				return exitInputErr, errInvalidUsage
			}
			message = remaining[i+1]
			i++
		case "--dry-run":
			dryRun = true
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page upload: --page-id is required")
		return exitInputErr, errInvalidUsage
	}
	if adfFile == "" {
		fmt.Fprintln(stderr, "page upload: --adf FILE is required")
		return exitInputErr, errInvalidUsage
	}

	adfBytes, err := os.ReadFile(adfFile)
	if err != nil {
		fmt.Fprintln(stderr, "reading ADF file:", err)
		return exitInputErr, err
	}

	doc, err := adf.UnmarshalDoc(adfBytes)
	if err != nil {
		fmt.Fprintln(stderr, "invalid ADF:", err)
		return exitParseErr, err
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	if err := client.UpdatePage(pageID, title, 0, doc, message, dryRun, stderr); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownErr, err
	}

	if !dryRun {
		fmt.Fprintf(stdout, `{"status":"ok","pageId":%q}`+"\n", pageID)
	}
	return exitOK, nil
}

func runPageCreate(args []string, stdout, stderr io.Writer) (int, error) {
	var spaceID, parentID, title, markdownFile, adfFile string

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--space-id":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--space-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			spaceID = remaining[i+1]
			i++
		case "--parent-id":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--parent-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			parentID = remaining[i+1]
			i++
		case "--title":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--title requires a value")
				return exitInputErr, errInvalidUsage
			}
			title = remaining[i+1]
			i++
		case "--markdown":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--markdown requires a file path")
				return exitInputErr, errInvalidUsage
			}
			markdownFile = remaining[i+1]
			i++
		case "--adf":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--adf requires a file path")
				return exitInputErr, errInvalidUsage
			}
			adfFile = remaining[i+1]
			i++
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if spaceID == "" {
		fmt.Fprintln(stderr, "page create: --space-id is required")
		return exitInputErr, errInvalidUsage
	}
	if parentID == "" {
		fmt.Fprintln(stderr, "page create: --parent-id is required")
		return exitInputErr, errInvalidUsage
	}
	if title == "" {
		fmt.Fprintln(stderr, "page create: --title is required")
		return exitInputErr, errInvalidUsage
	}
	if markdownFile != "" && adfFile != "" {
		fmt.Fprintln(stderr, "page create: specify either --markdown or --adf, not both")
		return exitInputErr, errInvalidUsage
	}

	var body *adf.Node
	if markdownFile != "" {
		src, err := os.ReadFile(markdownFile)
		if err != nil {
			fmt.Fprintln(stderr, "reading markdown:", err)
			return exitInputErr, err
		}
		doc, err := adf.Convert(src)
		if err != nil {
			fmt.Fprintln(stderr, "parse markdown:", err)
			return exitParseErr, err
		}
		body = &doc
	} else if adfFile != "" {
		adfBytes, err := os.ReadFile(adfFile)
		if err != nil {
			fmt.Fprintln(stderr, "reading ADF:", err)
			return exitInputErr, err
		}
		doc, err := adf.UnmarshalDoc(adfBytes)
		if err != nil {
			fmt.Fprintln(stderr, "invalid ADF:", err)
			return exitParseErr, err
		}
		body = &doc
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	result, err := client.CreatePage(spaceID, parentID, title, body)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownErr, err
	}

	out := map[string]string{
		"pageId": result.ID,
		"title":  result.Title,
		"url":    client.PageURL(result.Links.WebUI),
	}
	outBytes, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintln(stdout, string(outBytes))
	return exitOK, nil
}

func runPageListChildren(args []string, stdout, stderr io.Writer) (int, error) {
	var pageID string

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
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page list-children: --page-id is required")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	children, err := client.GetPageChildren(pageID)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownErr, err
	}

	for _, c := range children {
		fmt.Fprintf(stdout, "%s\t%s\n", c.ID, c.Title)
	}
	return exitOK, nil
}

// runLint validates an ADF file and prints findings.
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

// ── index command ──────────────────────────────────────────────────────────

const (
	homePageID    = "164232"
	homeSpaceID   = "131352"
	defaultCloud  = "lybel"
	indentNone    = ""
	indentLevel1  = "↳ "
	indentLevel2  = "↳↳ "
)

// runIndex handles the `index` subcommand with verbs: add, remove, sync.
func runIndex(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "index: requires a verb: add, remove, sync")
		return exitInputErr, errInvalidUsage
	}
	switch args[0] {
	case "add":
		return runIndexAdd(args[1:], stdout, stderr)
	case "remove":
		return runIndexRemove(args[1:], stdout, stderr)
	case "sync":
		return runIndexSync(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "index: unknown verb:", args[0])
		fmt.Fprintln(stderr, "  valid verbs: add, remove, sync")
		return exitInputErr, errInvalidUsage
	}
}

// indexPageContext holds the ADF doc and metadata for index operations.
type indexPageContext struct {
	doc      adf.Node
	inputFile string // empty = fetched from API
	pageID   string // the page to write back to (if fetched from API)
}

// loadIndexPage loads the ADF doc from --input file or fetches the Home page.
func loadIndexPage(inputFile string, client *adf.ConfluenceClient) (*indexPageContext, error) {
	if inputFile != "" {
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", inputFile, err)
		}
		doc, err := adf.UnmarshalDoc(data)
		if err != nil {
			return nil, fmt.Errorf("invalid ADF in %s: %w", inputFile, err)
		}
		return &indexPageContext{doc: doc, inputFile: inputFile, pageID: ""}, nil
	}

	// Fetch from API
	meta, err := client.GetPage(homePageID, "atlas_doc_format")
	if err != nil {
		return nil, fmt.Errorf("fetching Home page (ID %s): %w", homePageID, err)
	}
	adfStr := meta.Body.AtlasDocFormat.Value
	if adfStr == "" {
		return nil, fmt.Errorf("Home page has no ADF body")
	}
	doc, err := adf.UnmarshalDoc([]byte(adfStr))
	if err != nil {
		return nil, fmt.Errorf("parsing Home page ADF: %w", err)
	}
	return &indexPageContext{doc: doc, inputFile: "", pageID: homePageID}, nil
}

// saveIndexPage writes the updated doc to file or uploads it via the API.
func saveIndexPage(ctx *indexPageContext, doc adf.Node, client *adf.ConfluenceClient, message string, dryRun bool, stderr io.Writer) error {
	if ctx.inputFile != "" {
		if dryRun {
			fmt.Fprintf(stderr, "[dry-run] Would write updated ADF to %s\n", ctx.inputFile)
			return nil
		}
		out, err := adf.Marshal(doc, true)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		return os.WriteFile(ctx.inputFile, out, 0644)
	}
	// Upload via API
	return client.UpdatePage(ctx.pageID, "", 0, doc, message, dryRun, stderr)
}

func runIndexAdd(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		pageID    string
		title     string
		under     string
		inputFile string
		indent    int
		ifMissing bool
		dryRun    bool
	)

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
		case "--title":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--title requires a value")
				return exitInputErr, errInvalidUsage
			}
			title = remaining[i+1]
			i++
		case "--under":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--under requires a value")
				return exitInputErr, errInvalidUsage
			}
			under = remaining[i+1]
			i++
		case "--input":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--input requires a file path")
				return exitInputErr, errInvalidUsage
			}
			inputFile = remaining[i+1]
			i++
		case "--indent":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--indent requires 0, 1, or 2")
				return exitInputErr, errInvalidUsage
			}
			n, nerr := strconv.Atoi(remaining[i+1])
			if nerr != nil || n < 0 || n > 2 {
				fmt.Fprintln(stderr, "--indent must be 0, 1, or 2")
				return exitInputErr, errInvalidUsage
			}
			indent = n
			i++
		case "--if-missing":
			ifMissing = true
		case "--dry-run":
			dryRun = true
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "index add: --page-id is required")
		return exitInputErr, errInvalidUsage
	}
	if title == "" {
		fmt.Fprintln(stderr, "index add: --title is required")
		return exitInputErr, errInvalidUsage
	}
	if under == "" {
		fmt.Fprintln(stderr, "index add: --under is required")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	ctx, err := loadIndexPage(inputFile, client)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitUnknownErr, err
	}

	// Build the cell values
	var prefix string
	switch indent {
	case 1:
		prefix = indentLevel1
	case 2:
		prefix = indentLevel2
	default:
		prefix = indentNone
	}
	displayTitle := prefix + title
	pageIDCell := "`" + pageID + "`"
	rowText := displayTitle + "|" + pageIDCell

	updated, existed, err := adf.TableAddRow(ctx.doc, under, 0, rowText, "", ifMissing)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, err
	}
	if existed {
		fmt.Fprintf(stderr, "notice: page %q already listed under %q — skipped (--if-missing)\n", title, under)
		fmt.Fprintln(stdout, `{"status":"skipped","reason":"already exists"}`)
		return exitOK, nil
	}

	if err := saveIndexPage(ctx, updated, client, "index add: "+title, dryRun, stderr); err != nil {
		fmt.Fprintln(stderr, "saving:", err)
		return exitUnknownErr, err
	}

	fmt.Fprintf(stdout, `{"status":"ok","pageId":%q,"title":%q}`+"\n", pageID, title)
	return exitOK, nil
}

func runIndexRemove(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		pageID    string
		inputFile string
		dryRun    bool
	)

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
		case "--input":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--input requires a file path")
				return exitInputErr, errInvalidUsage
			}
			inputFile = remaining[i+1]
			i++
		case "--dry-run":
			dryRun = true
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "index remove: --page-id is required")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	ctx, err := loadIndexPage(inputFile, client)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitUnknownErr, err
	}

	// Search all sections for a row whose second cell contains the pageID
	updated, removeErr := removeIndexRow(ctx.doc, pageID)
	if removeErr != nil {
		fmt.Fprintln(stderr, removeErr)
		return exitInputErr, removeErr
	}

	if err := saveIndexPage(ctx, updated, client, "index remove: "+pageID, dryRun, stderr); err != nil {
		fmt.Fprintln(stderr, "saving:", err)
		return exitUnknownErr, err
	}

	fmt.Fprintf(stdout, `{"status":"ok","removed":%q}`+"\n", pageID)
	return exitOK, nil
}

// removeIndexRow scans all tables in the doc and removes the first row whose
// any cell contains pageID.
func removeIndexRow(doc adf.Node, pageID string) (adf.Node, error) {
	newContent := make([]adf.Node, len(doc.Content))
	copy(newContent, doc.Content)

	removed := false
	for i, n := range newContent {
		if n.Type != "table" {
			continue
		}
		var newRows []adf.Node
		for _, row := range n.Content {
			rowStr := nodeText(row)
			if !removed && strings.Contains(rowStr, pageID) {
				removed = true
				continue
			}
			newRows = append(newRows, row)
		}
		if removed {
			updated := n
			updated.Content = newRows
			newContent[i] = updated
			break
		}
	}

	if !removed {
		return adf.Node{}, fmt.Errorf("page ID %q not found in any table", pageID)
	}

	out := doc
	out.Content = newContent
	return out, nil
}

// nodeText recursively extracts all text from a node.
func nodeText(n adf.Node) string {
	var sb strings.Builder
	collectNodeText(n, &sb)
	return sb.String()
}

func collectNodeText(n adf.Node, sb *strings.Builder) {
	if n.Text != "" {
		sb.WriteString(n.Text)
	}
	for _, c := range n.Content {
		collectNodeText(c, sb)
	}
}

func runIndexSync(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		parentPageID string
		under        string
		inputFile    string
		dryRun       bool
	)

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--parent-page-id":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--parent-page-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			parentPageID = remaining[i+1]
			i++
		case "--under":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--under requires a value")
				return exitInputErr, errInvalidUsage
			}
			under = remaining[i+1]
			i++
		case "--input":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--input requires a file path")
				return exitInputErr, errInvalidUsage
			}
			inputFile = remaining[i+1]
			i++
		case "--dry-run":
			dryRun = true
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if parentPageID == "" {
		fmt.Fprintln(stderr, "index sync: --parent-page-id is required")
		return exitInputErr, errInvalidUsage
	}
	if under == "" {
		fmt.Fprintln(stderr, "index sync: --under is required")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	// Fetch children of the parent page
	children, err := client.GetPageChildren(parentPageID)
	if err != nil {
		fmt.Fprintln(stderr, "fetching children:", err)
		return exitUnknownErr, err
	}

	ctx, err := loadIndexPage(inputFile, client)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitUnknownErr, err
	}

	// Add each child that's not already in the index
	doc := ctx.doc
	added := 0
	for _, child := range children {
		rowText := child.Title + "|`" + child.ID + "`"
		updated, existed, addErr := adf.TableAddRow(doc, under, 0, rowText, "", true)
		if addErr != nil {
			fmt.Fprintf(stderr, "warning: could not add %q (%s): %v\n", child.Title, child.ID, addErr)
			continue
		}
		if !existed {
			doc = updated
			added++
		}
	}

	fmt.Fprintf(stderr, "index sync: %d children found, %d added\n", len(children), added)

	if err := saveIndexPage(ctx, doc, client, "index sync", dryRun, stderr); err != nil {
		fmt.Fprintln(stderr, "saving:", err)
		return exitUnknownErr, err
	}

	fmt.Fprintf(stdout, `{"status":"ok","added":%d}`+"\n", added)
	return exitOK, nil
}

// writeJSON marshals n and writes it to stdout.
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
