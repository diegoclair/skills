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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/lybel-app/skills/cli/lybel-docs/adf"
	"github.com/lybel-app/skills/cli/lybel-docs/setup"
)

// version is injected at build time via -ldflags "-X main.version=..."
var version = "dev"

const helpText = `lybel-docs — Confluence ADF toolkit: convert, edit, lint, and publish pages.

USAGE:
  lybel-docs setup        [--email X --token Y | --check | --print-config-path]
  lybel-docs update       [--check]
  lybel-docs adf          [--file PATH] [--pretty]
  lybel-docs edit         [--input PATH] OPERATION [--at-level N] [--pretty]
  lybel-docs page         VERB [flags]
  lybel-docs search       "term" [--limit N] [--space lybel] [--cql RAW] [--json]
  lybel-docs home         [--refresh | --status | --show | --query "X" | --digest] [--max-age 24h]
  lybel-docs lint         FILE.json
  lybel-docs extract-body [< mcp-response.json]
  lybel-docs index        VERB [flags]
  lybel-docs --version
  lybel-docs --help

COMMANDS:
  setup         Interactive wizard to configure Atlassian API credentials.
  update        Self-update: fetch the latest release and re-run the installer.
  adf           Convert markdown (stdin or --file) to an ADF JSON document.
  edit          Apply a section-level or table edit to an existing ADF doc.
  page          Fetch, upload, create, digest, or apply edits to Confluence pages.
  search        CQL search via the v1 API. TSV output (id\ttitle\turl\texcerpt).
  home          Local cache of the Confluence Home page (refresh, status, show, query).
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
  --replace-intro FRAGMENT.md             Replace pre-heading content (intro callout).
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
  page get     --page-id ID [--cloud SUBDOMAIN] [--output FILE] [--quiet]
               [--format adf|text|markdown|export_view|html|view|storage]
               [--section "Heading" [--at-level N]]
               Default --format is "adf", or "text" when --section is set.
               "text"/"markdown" render ADF locally (reuses adf.RenderText).
               "html" is an alias of "view".
               --section slices the response to a single heading + its body
               (heading + nodes until the next heading of equal-or-higher level).
               Only --format adf|text|markdown work with --section.
  page upload  --page-id ID --adf FILE [--title TITLE] [--message MSG] [--dry-run]
               [--cloud SUBDOMAIN] [--email EMAIL] [--token TOKEN]
  page create  --space-id ID --parent-id ID --title TITLE
               [--markdown FILE | --adf FILE] [--cloud SUBDOMAIN]
               [--email EMAIL] [--token TOKEN]
  page digest  --page-id ID [--json]
               Print a slim summary of the page (title, version, headings,
               word counts, macros). Replaces a 10-40 KB ADF read with a
               <1 KB digest — answers most "what's in this page?" questions
               without round-tripping the full doc.
  page apply   --page-id ID OPERATION [--fragment FILE] [--at-level N]
               [--message MSG] [--dry-run]
               Atomic update: GET ADF → apply op → PUT. On 409 conflict
               (someone else updated mid-flight), refetches and retries once.
               The full ADF never leaves the binary.
               OPERATION is one of:
                 --append                                          (needs --fragment)
                 --replace-intro                                   (needs --fragment)
                 --insert-after  "Heading"                         (needs --fragment)
                 --insert-before "Heading"                         (needs --fragment)
                 --replace-section "Heading"                       (needs --fragment)
                 --delete-section  "Heading"
                 --table-add-row    "Heading" --row "a|b|c"        [--after-row "x"] [--if-missing]
                 --table-remove-row "Heading" --match-cell "text"
                 --multi OPS.json    Apply many ops atomically in 1 GET+PUT.
               In --row, '|' is the cell separator. To include a literal pipe
               character inside a cell, escape it with a backslash, e.g.:
               --row "Foo (A\|B)|123"  → cells: ["Foo (A|B)", "123"].
  page rewrite --page-id ID --markdown FILE [--strategy headings]
               [--message MSG] [--dry-run] [--allow-add] [--allow-remove]
               High-level wrapper: split markdown by headings, match against
               the page, replace each matched section. Pre-heading content
               becomes a replace-intro op. Mismatches are warnings unless
               --allow-add / --allow-remove flip them into ops.
  page children  --page-id ID [--cloud SUBDOMAIN]
               List direct children of a page. Output: TSV (id, title) per line.
               (Old name 'list-children' still works as alias.)

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

SEARCH:
  search "term" [--limit N] [--space KEY] [--json]
    Default CQL: space="<KEY>" AND type="page" AND (title ~ "term" OR text ~ "term")
    Default space: lybel. Default limit: 10. Output: TSV (id, title, url, excerpt).
  search --cql 'space=lybel AND label="adr"'
    Pass raw CQL — caller is responsible for escaping.

HOME CACHE:
  home --refresh                 Force GET + cache (~/.cache/lybel-docs/home.json).
  home --status                  Print cache metadata (default if no flag).
  home --show                    Print cached page text (markdown rendering of ADF).
  home --query "term"            Grep cached content; auto-refresh if cache missing.
  home --digest                  Print cached digest.
  home --max-age DURATION        Stale threshold for warning (default 24h).
  WRITES: the cache is read-only for navigation. 'page apply' always GETs
  fresh ADF before PUT — the cache is NEVER used as the source for updates.

EXAMPLES:
  # Convert markdown to ADF
  lybel-docs adf < page.md > page.adf.json

  # Slim summary of a page (cheap; LLM-friendly)
  lybel-docs page digest --page-id 164232

  # Atomic update without ever loading the full ADF into the caller
  lybel-docs page apply --page-id 164232 \
    --replace-section "Roadmap" --fragment new.md --message "rewrite roadmap"

  # Search the lybel space
  lybel-docs search "advisor" --limit 5

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
	case "update":
		return runUpdate(args[1:], stdout, stderr)
	case "adf":
		return runADF(args[1:], stdin, stdout, stderr)
	case "edit":
		return runEdit(args[1:], stdin, stdout, stderr)
	case "page":
		return runPage(args[1:], stdin, stdout, stderr)
	case "search":
		return runSearch(args[1:], stdout, stderr)
	case "home":
		return runHome(args[1:], stdout, stderr)
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
	opReplaceIntro
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
	case "children", "list-children":
		// `children` is the canonical (single-word) verb, parallel to
		// `get`/`digest`/`apply`. `list-children` kept as a back-compat alias
		// from the original kebab-case name.
		return runPageListChildren(args[1:], stdout, stderr)
	case "digest":
		return runPageDigest(args[1:], stdout, stderr)
	case "apply":
		return runPageApply(args[1:], stdout, stderr)
	case "rewrite":
		return runPageRewrite(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "page: unknown verb:", args[0])
		fmt.Fprintln(stderr, "  valid verbs: get, upload, create, children, digest, apply, rewrite")
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
	var (
		pageID, format, outputFile string
		section                    string
		atLevel                    int
		quiet                      bool
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
		case "--format":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--format requires a value (adf|text|markdown|export_view|html|view|storage)")
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
		case "--section":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--section requires a heading")
				return exitInputErr, errInvalidUsage
			}
			section = remaining[i+1]
			i++
		case "--at-level":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--at-level requires a value (1-6)")
				return exitInputErr, errInvalidUsage
			}
			n, lvErr := strconv.Atoi(remaining[i+1])
			if lvErr != nil || n < 1 || n > 6 {
				fmt.Fprintln(stderr, "--at-level must be an integer between 1 and 6")
				return exitInputErr, errInvalidUsage
			}
			atLevel = n
			i++
		case "--quiet":
			quiet = true
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
		// Default: when slicing a section, the caller almost always wants
		// readable prose (text). Otherwise, keep the historical ADF default.
		if section != "" {
			format = "text"
		} else {
			format = "adf"
		}
	}

	// "text" and "markdown" are local renderings of ADF — they require
	// fetching atlas_doc_format and running adf.RenderText. They are NOT
	// API-side body formats.
	wantLocalRender := format == "text" || format == "markdown"

	// --section requires local-side slicing on the parsed ADF. The only
	// formats supported with --section are adf, text, and markdown — the
	// server-rendered formats (view/html/storage/export_view) describe the
	// whole page and can't be sliced reliably by heading.
	if section != "" {
		switch format {
		case "adf", "text", "markdown":
			// ok
		default:
			fmt.Fprintf(stderr, "--section is only supported with --format adf|text|markdown (got %q)\n", format)
			return exitInputErr, errInvalidUsage
		}
	}

	// Validate --format and resolve it to a Confluence API body-format.
	// This MUST happen before buildClient so a bad --format fails fast with
	// a clear message even when credentials are missing (matters for tests
	// and CI where no creds are configured).
	var bodyFormat, fieldName string
	switch format {
	case "adf", "text", "markdown":
		// All three need ADF from the API; "text"/"markdown" are rendered
		// locally after fetch.
		bodyFormat, fieldName = "atlas_doc_format", "atlas_doc_format"
	case "storage":
		bodyFormat, fieldName = "storage", "storage"
	case "view", "html":
		bodyFormat, fieldName = "view", "view"
	case "export_view":
		bodyFormat, fieldName = "export_view", "export_view"
	default:
		fmt.Fprintf(stderr, "unknown format %q — use adf, text, markdown, storage, view, html, or export_view\n", format)
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	meta, err := client.GetPage(pageID, bodyFormat)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownErr, err
	}

	var raw []byte
	switch fieldName {
	case "atlas_doc_format":
		raw = []byte(meta.Body.AtlasDocFormat.Value)
	case "storage":
		raw = []byte(meta.Body.Storage.Value)
	case "view":
		raw = []byte(meta.Body.View.Value)
	case "export_view":
		raw = []byte(meta.Body.ExportView.Value)
	}
	if len(raw) == 0 {
		fmt.Fprintf(stderr, "page has no %s body — try a different --format\n", fieldName)
		return exitUnknownErr, fmt.Errorf("empty %s body", fieldName)
	}

	// Compute the final output bytes based on (--section, --format).
	var out []byte
	if section != "" || wantLocalRender {
		// Both branches need the parsed ADF doc.
		doc, dErr := adf.UnmarshalDoc(raw)
		if dErr != nil {
			fmt.Fprintln(stderr, "parse ADF:", dErr)
			return exitParseErr, dErr
		}

		// Slice to a section if requested.
		target := doc
		if section != "" {
			sub, sErr := adf.SectionContent(doc, section, atLevel)
			if sErr != nil {
				fmt.Fprintln(stderr, "operation failed:", sErr)
				fmt.Fprintln(stderr, "current top-level sections:")
				for _, n := range doc.Content {
					if n.Type == "heading" {
						fmt.Fprintf(stderr, "  - h%d %q\n", headingLevelFromNode(n), strings.TrimSpace(allText(n)))
					}
				}
				return exitInputErr, errInvalidUsage
			}
			target = sub
		}

		// Render into the requested format.
		switch format {
		case "adf":
			marshalled, mErr := adf.Marshal(target, false)
			if mErr != nil {
				fmt.Fprintln(stderr, "marshal:", mErr)
				return exitUnknownErr, mErr
			}
			out = marshalled
		case "text", "markdown":
			out = []byte(adf.RenderText(target))
		}
	} else {
		// No slicing, no local rendering — emit the API body as-is.
		out = raw
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, out, 0644); err != nil {
			fmt.Fprintln(stderr, "writing output:", err)
			return exitUnknownErr, err
		}
		if !quiet {
			fmt.Fprintf(stderr, "wrote %d bytes to %s\n", len(out), outputFile)
		}
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
		// Auto-refresh the home cache if this write touched the Home page.
		refreshHomeCacheAfterWrite(pageID, client, stderr)
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
		fmt.Fprintln(stderr, "page children: --page-id is required")
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

// runPageDigest fetches a page, parses its ADF, and emits a slim summary
// (title, version, headings + word counts, macros). Designed to answer
// "what's in this page?" without round-tripping the full ADF.
func runPageDigest(args []string, stdout, stderr io.Writer) (int, error) {
	var pageID string
	var asJSON bool

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
		case "--json":
			asJSON = true
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page digest: --page-id is required")
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	meta, err := client.GetPage(pageID, "atlas_doc_format")
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownErr, err
	}
	if meta.Body.AtlasDocFormat.Value == "" {
		fmt.Fprintln(stderr, "page has no ADF body")
		return exitUnknownErr, fmt.Errorf("empty ADF body")
	}

	doc, err := adf.UnmarshalDoc([]byte(meta.Body.AtlasDocFormat.Value))
	if err != nil {
		fmt.Fprintln(stderr, "parse ADF:", err)
		return exitParseErr, err
	}

	digest := adf.BuildDigest(doc, meta.ID, meta.Title, client.PageURL(meta.Links.WebUI), meta.Version.Number)

	if asJSON {
		out, _ := json.MarshalIndent(digest, "", "  ")
		fmt.Fprintln(stdout, string(out))
	} else {
		fmt.Fprint(stdout, digest.FormatText())
	}
	return exitOK, nil
}

// multiOp is a single batch operation read from --multi JSON or built
// internally by `page rewrite`. The shape mirrors the single-op CLI flags so
// users who already know `page apply` can compose multi files easily.
type multiOp struct {
	Kind      string `json:"kind"`
	Heading   string `json:"heading,omitempty"`
	AtLevel   int    `json:"atLevel,omitempty"`
	Fragment  string `json:"fragment,omitempty"`
	Row       string `json:"row,omitempty"`
	AfterRow  string `json:"afterRow,omitempty"`
	MatchCell string `json:"matchCell,omitempty"`
	IfMissing bool   `json:"ifMissing,omitempty"`
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
		out, existed, err := adf.TableAddRow(doc, op.Heading, op.AtLevel, op.Row, op.AfterRow, op.IfMissing)
		if existed {
			return doc, true, nil
		}
		return out, false, err
	case "table-remove-row":
		out, err := adf.TableRemoveRow(doc, op.Heading, op.AtLevel, op.MatchCell)
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
		if op.MatchCell == "" {
			return fmt.Errorf("table-remove-row requires matchCell")
		}
	default:
		return fmt.Errorf("unknown op kind %q", op.Kind)
	}
	return nil
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
				// Abort: report which op (1-indexed) failed.
				fmt.Fprintf(stderr, "op %d (%s) failed: %v\n", i+1, op.Kind, opErr)
				if op.Kind != "table-add-row" && op.Kind != "table-remove-row" {
					fmt.Fprintln(stderr, "current top-level sections:")
					for _, n := range current.Content {
						if n.Type == "heading" {
							fmt.Fprintf(stderr, "  - h%d %q\n", headingLevelFromNode(n), strings.TrimSpace(allText(n)))
						}
					}
				}
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
		pageID       string
		op           editOp
		heading      string
		fragmentPath string
		atLevel      int
		message      string
		dryRun       bool
		rowText      string
		afterRow     string
		matchCell    string
		ifMissing    bool
		multiPath    string
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
	case opTableRemoveRow:
		if matchCell == "" {
			fmt.Fprintln(stderr, "page apply: --table-remove-row requires --match-cell \"text\"")
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
			result, rowExisted, opErr = adf.TableAddRow(doc, heading, atLevel, rowText, afterRow, ifMissing)
			if rowExisted {
				fmt.Fprintf(stderr, "notice: row with first cell %q already exists in %q — skipped (--if-missing)\n",
					strings.SplitN(rowText, "|", 2)[0], heading)
				fmt.Fprintf(stdout, `{"status":"skipped","reason":"row already exists","pageId":%q}`+"\n", pageID)
				return exitOK, nil
			}
		case opTableRemoveRow:
			result, opErr = adf.TableRemoveRow(doc, heading, atLevel, matchCell)
		}
		if opErr != nil {
			// For section ops, list the current top-level headings to help
			// the caller. Table ops embed their own heading list in the
			// error message — printing it once is enough.
			fmt.Fprintln(stderr, "operation failed:", opErr)
			if op != opTableAddRow && op != opTableRemoveRow {
				fmt.Fprintln(stderr, "current top-level sections:")
				for _, n := range doc.Content {
					if n.Type == "heading" {
						fmt.Fprintf(stderr, "  - h%d %q\n", headingLevelFromNode(n), strings.TrimSpace(allText(n)))
					}
				}
			}
			// Return errInvalidUsage so main() prints a terse "lybel-docs:
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

	url := fmt.Sprintf("%s/spaces/%s/pages/%s", client.BaseURL(), defaultCloud, pageID)
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
	url := fmt.Sprintf("%s/spaces/%s/pages/%s", client.BaseURL(), defaultCloud, pageID)
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
func runPageRewrite(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		pageID       string
		markdownFile string
		strategy     = "headings"
		message      string
		dryRun       bool
		allowAdd     bool
		allowRemove  bool
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
		case "--markdown":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--markdown requires a file path")
				return exitInputErr, errInvalidUsage
			}
			markdownFile = remaining[i+1]
			i++
		case "--strategy":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--strategy requires a value")
				return exitInputErr, errInvalidUsage
			}
			strategy = remaining[i+1]
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
		case "--allow-add":
			allowAdd = true
		case "--allow-remove":
			allowRemove = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "page rewrite — replace matched sections of a page from a markdown file.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  --page-id ID           target page (required)")
			fmt.Fprintln(stdout, "  --markdown FILE        new content (required)")
			fmt.Fprintln(stdout, "  --strategy headings    (default; only value)")
			fmt.Fprintln(stdout, "  --message MSG          version comment")
			fmt.Fprintln(stdout, "  --dry-run              print proposed ops without writing")
			fmt.Fprintln(stdout, "  --allow-add            also append headings present in markdown but not in page")
			fmt.Fprintln(stdout, "  --allow-remove         also delete headings present in page but not in markdown")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	if pageID == "" {
		fmt.Fprintln(stderr, "page rewrite: --page-id is required")
		return exitInputErr, errInvalidUsage
	}
	if markdownFile == "" {
		fmt.Fprintln(stderr, "page rewrite: --markdown FILE is required")
		return exitInputErr, errInvalidUsage
	}
	if strategy != "headings" {
		fmt.Fprintf(stderr, "page rewrite: unknown strategy %q (only 'headings' supported)\n", strategy)
		return exitInputErr, errInvalidUsage
	}

	mdBytes, err := os.ReadFile(markdownFile)
	if err != nil {
		fmt.Fprintln(stderr, "reading markdown:", err)
		return exitInputErr, err
	}

	// Split markdown by headings; produce a list of (level, title, body) plus
	// a pre-heading intro slice. Body excludes the heading line itself.
	mdIntro, mdSections, err := splitMarkdownByHeadings(mdBytes)
	if err != nil {
		fmt.Fprintln(stderr, "split markdown:", err)
		return exitParseErr, err
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	// Fetch current page & its top-level headings.
	meta, err := client.GetPage(pageID, "atlas_doc_format")
	if err != nil {
		fmt.Fprintln(stderr, "fetching page:", err)
		return exitUnknownErr, err
	}
	if meta.Body.AtlasDocFormat.Value == "" {
		fmt.Fprintln(stderr, "page has no ADF body")
		return exitUnknownErr, fmt.Errorf("empty ADF body")
	}
	doc, err := adf.UnmarshalDoc([]byte(meta.Body.AtlasDocFormat.Value))
	if err != nil {
		fmt.Fprintln(stderr, "parse ADF:", err)
		return exitParseErr, err
	}

	// Collect (level, title) for each top-level heading in the page.
	type pageHeading struct {
		level int
		title string
	}
	var pageHeadings []pageHeading
	for _, n := range doc.Content {
		if n.Type == "heading" {
			pageHeadings = append(pageHeadings, pageHeading{
				level: headingLevelFromNode(n),
				title: strings.TrimSpace(allText(n)),
			})
		}
	}
	pageHeadingKey := func(h pageHeading) string {
		return fmt.Sprintf("%d:%s", h.level, h.title)
	}
	mdHeadingKey := func(level int, title string) string {
		return fmt.Sprintf("%d:%s", level, title)
	}
	pageHeadingSet := make(map[string]bool)
	for _, h := range pageHeadings {
		pageHeadingSet[pageHeadingKey(h)] = true
	}
	mdHeadingSet := make(map[string]bool)
	for _, s := range mdSections {
		mdHeadingSet[mdHeadingKey(s.level, s.title)] = true
	}

	// Build temp dir for fragment files.
	tmpDir, err := os.MkdirTemp("", "lybel-rewrite-")
	if err != nil {
		fmt.Fprintln(stderr, "tempdir:", err)
		return exitUnknownErr, err
	}
	defer os.RemoveAll(tmpDir)

	writeFrag := func(name, content string) (string, error) {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return "", err
		}
		return path, nil
	}

	// Build ops + reporting lines.
	var ops []multiOp
	var report []rewriteReportLine
	wouldAdd, wouldRemove := 0, 0

	// 1. Intro (markdown pre-heading content)
	if strings.TrimSpace(mdIntro) != "" {
		path, err := writeFrag("intro.md", mdIntro)
		if err != nil {
			fmt.Fprintln(stderr, "writing intro frag:", err)
			return exitUnknownErr, err
		}
		ops = append(ops, multiOp{Kind: "replace-intro", Fragment: path})
		report = append(report, rewriteReportLine{"✓", "replaced intro"})
	}

	// 2. Walk markdown sections in order. For each:
	//    - if matched in page (level+title): emit replace-section op
	//    - if not: would-add → optionally append op
	for i, s := range mdSections {
		key := mdHeadingKey(s.level, s.title)
		body := s.fullText() // includes heading line + body
		fragName := fmt.Sprintf("section-%d.md", i)
		path, err := writeFrag(fragName, body)
		if err != nil {
			fmt.Fprintln(stderr, "writing section frag:", err)
			return exitUnknownErr, err
		}
		if pageHeadingSet[key] {
			ops = append(ops, multiOp{
				Kind:     "replace-section",
				Heading:  s.title,
				AtLevel:  s.level,
				Fragment: path,
			})
			report = append(report, rewriteReportLine{"✓",
				fmt.Sprintf("replaced section %q (h%d)", s.title, s.level)})
		} else {
			if allowAdd {
				ops = append(ops, multiOp{Kind: "append", Fragment: path})
				report = append(report, rewriteReportLine{"+",
					fmt.Sprintf("added section %q (h%d) at end", s.title, s.level)})
			} else {
				report = append(report, rewriteReportLine{"⚠",
					fmt.Sprintf("would add: %q (h%d) [pass --allow-add to apply]", s.title, s.level)})
				wouldAdd++
			}
		}
	}

	// 3. Headings in page but NOT in markdown → would-remove (or remove if flag).
	for _, h := range pageHeadings {
		key := pageHeadingKey(h)
		if mdHeadingSet[key] {
			continue
		}
		if allowRemove {
			ops = append(ops, multiOp{
				Kind:    "delete-section",
				Heading: h.title,
				AtLevel: h.level,
			})
			report = append(report, rewriteReportLine{"-",
				fmt.Sprintf("removed section %q (h%d)", h.title, h.level)})
		} else {
			report = append(report, rewriteReportLine{"⚠",
				fmt.Sprintf("would remove: %q (h%d) [pass --allow-remove to apply]", h.title, h.level)})
			wouldRemove++
		}
	}

	if len(ops) == 0 {
		fmt.Fprintln(stderr, "page rewrite: no matching headings — nothing to do")
		for _, r := range report {
			fmt.Fprintf(stderr, "  %s %s\n", r.mark, r.text)
		}
		return exitOK, nil
	}

	// Pre-load fragments for the multi-apply.
	fragments := make([][]adf.Node, len(ops))
	for i, op := range ops {
		if err := validateMultiOp(op); err != nil {
			fmt.Fprintf(stderr, "internal: built invalid op %d (%s): %v\n", i+1, op.Kind, err)
			return exitUnknownErr, err
		}
		frag, err := loadMultiFragment(op)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitInputErr, err
		}
		fragments[i] = frag
	}

	if dryRun {
		fmt.Fprintf(stdout, "Rewriting page %s (v%d → v%d) [dry-run]\n",
			pageID, meta.Version.Number, meta.Version.Number+1)
		for _, r := range report {
			fmt.Fprintf(stdout, "  %s %s\n", r.mark, r.text)
		}
		fmt.Fprintf(stdout, "%d sections replaced, %d add skipped, %d remove skipped\n",
			countReports(report, "✓"), wouldAdd, wouldRemove)
		return exitOK, nil
	}

	fromV, toV, title, applied, err := runMultiApply(client, pageID, message, ops, fragments, false, stdout, stderr)
	if err != nil {
		return exitInputErr, err
	}
	refreshHomeCacheAfterWrite(pageID, client, stderr)

	fmt.Fprintf(stdout, "Rewriting page %s (v%d → v%d)\n", pageID, fromV, toV)
	for _, r := range report {
		fmt.Fprintf(stdout, "  %s %s\n", r.mark, r.text)
	}
	fmt.Fprintf(stdout, "%d sections replaced, %d add skipped, %d remove skipped\n",
		countReports(report, "✓"), wouldAdd, wouldRemove)
	url := fmt.Sprintf("%s/spaces/%s/pages/%s", client.BaseURL(), defaultCloud, pageID)
	fmt.Fprintf(stdout, "URL: %s\n", url)
	_ = title
	_ = applied
	return exitOK, nil
}

// rewriteReportLine is a single line of human output for `page rewrite`.
type rewriteReportLine struct {
	mark string // "✓", "+", "-", "⚠"
	text string
}

// countReports counts report lines whose mark equals m.
func countReports(rs []rewriteReportLine, m string) int {
	n := 0
	for _, r := range rs {
		if r.mark == m {
			n++
		}
	}
	return n
}

// mdSection is a markdown section parsed by splitMarkdownByHeadings.
type mdSection struct {
	level    int    // 1..6
	title    string // heading text (trimmed)
	headLine string // the original heading line, e.g. "## Foo"
	body     string // body text after the heading, may contain blank lines
}

// fullText returns the section serialized back to markdown including its
// heading line, suitable for writing as a fragment file.
func (s mdSection) fullText() string {
	if s.body == "" {
		return s.headLine + "\n"
	}
	return s.headLine + "\n" + s.body
}

// splitMarkdownByHeadings parses raw markdown bytes and returns:
//   - intro: text before the first heading line (may be empty)
//   - sections: one entry per heading, in document order
//
// Headings are detected as ATX-style lines beginning with 1-6 '#' chars
// followed by a space. Setext headings (=== / ---) are NOT supported.
func splitMarkdownByHeadings(src []byte) (intro string, sections []mdSection, err error) {
	lines := strings.Split(string(src), "\n")
	type pending struct {
		level    int
		title    string
		headLine string
		bodyBuf  []string
	}
	var introBuf []string
	var cur *pending
	flush := func() {
		if cur == nil {
			return
		}
		body := strings.Join(cur.bodyBuf, "\n")
		// Trim trailing blank lines from body so fragments are tidy, but
		// preserve a trailing newline.
		body = strings.TrimRight(body, "\n") + "\n"
		if strings.TrimSpace(body) == "" {
			body = ""
		}
		sections = append(sections, mdSection{
			level:    cur.level,
			title:    cur.title,
			headLine: cur.headLine,
			body:     body,
		})
		cur = nil
	}
	for _, line := range lines {
		level, title, ok := parseATXHeading(line)
		if ok {
			flush()
			cur = &pending{
				level:    level,
				title:    title,
				headLine: line,
			}
			continue
		}
		if cur == nil {
			introBuf = append(introBuf, line)
		} else {
			cur.bodyBuf = append(cur.bodyBuf, line)
		}
	}
	flush()
	intro = strings.Join(introBuf, "\n")
	intro = strings.TrimRight(intro, "\n")
	if strings.TrimSpace(intro) != "" {
		intro += "\n"
	} else {
		intro = ""
	}
	return intro, sections, nil
}

// parseATXHeading returns (level, trimmedTitle, true) if line is an ATX
// heading like "## Foo". Otherwise returns (0, "", false).
func parseATXHeading(line string) (int, string, bool) {
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i == 0 || i > 6 {
		return 0, "", false
	}
	if i >= len(line) || line[i] != ' ' {
		return 0, "", false
	}
	title := strings.TrimSpace(line[i+1:])
	// Strip optional trailing # tokens (e.g. "## Foo ##").
	title = strings.TrimRight(title, "#")
	title = strings.TrimSpace(title)
	if title == "" {
		return 0, "", false
	}
	return i, title, true
}

// headingLevelFromNode is a thin wrapper to expose heading level for the
// section-list error message in runPageApply. Mirrors adf.headingLevel.
func headingLevelFromNode(n adf.Node) int {
	if n.Attrs == nil {
		return 1
	}
	switch v := n.Attrs["level"].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 1
}

// allText collects the inline text of a node tree (for printing heading text).
func allText(n adf.Node) string {
	var sb strings.Builder
	collectAllText(n, &sb)
	return sb.String()
}

func collectAllText(n adf.Node, sb *strings.Builder) {
	if n.Text != "" {
		sb.WriteString(n.Text)
	}
	for _, c := range n.Content {
		collectAllText(c, sb)
	}
}

// runSearch runs a CQL query against Confluence and prints results as TSV
// (pageId\ttitle\turl\texcerpt). Defaults the space filter to `lybel`.
func runSearch(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		query   string
		rawCQL  string
		space   string
		limit   int
		asJSON  bool
	)

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--cql":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--cql requires a value")
				return exitInputErr, errInvalidUsage
			}
			rawCQL = remaining[i+1]
			i++
		case "--space":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--space requires a value")
				return exitInputErr, errInvalidUsage
			}
			space = remaining[i+1]
			i++
		case "--limit":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--limit requires a value")
				return exitInputErr, errInvalidUsage
			}
			n, sErr := strconv.Atoi(remaining[i+1])
			if sErr != nil || n < 1 || n > 250 {
				fmt.Fprintln(stderr, "--limit must be an integer between 1 and 250")
				return exitInputErr, errInvalidUsage
			}
			limit = n
			i++
		case "--json":
			asJSON = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "search — CQL search via the Confluence v1 search API. TSV output by default.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  lybel-docs search \"term\"                  # title or text match in default space")
			fmt.Fprintln(stdout, "  lybel-docs search --cql 'space=lybel AND label=\"adr\"'")
			fmt.Fprintln(stdout, "  lybel-docs search \"term\" --limit 5 --json")
			return exitOK, nil
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintln(stderr, "unknown flag:", a)
				return exitInputErr, errInvalidUsage
			}
			query = a
		}
	}

	if rawCQL == "" && query == "" {
		fmt.Fprintln(stderr, "search: provide a query term or --cql RAW")
		return exitInputErr, errInvalidUsage
	}
	if space == "" {
		space = defaultCloud // "lybel"
	}
	if limit == 0 {
		limit = 10
	}

	cql := rawCQL
	if cql == "" {
		// CQL string-literals use double quotes; escape any in the query.
		safe := strings.ReplaceAll(query, `"`, `\"`)
		cql = fmt.Sprintf(`space = "%s" AND type = "page" AND (title ~ "%s" OR text ~ "%s")`,
			space, safe, safe)
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	results, err := client.SearchCQL(cql, limit)
	if err != nil {
		fmt.Fprintln(stderr, "search error:", err)
		return exitUnknownErr, err
	}

	if asJSON {
		out, _ := json.MarshalIndent(results, "", "  ")
		fmt.Fprintln(stdout, string(out))
		return exitOK, nil
	}

	if len(results) == 0 {
		fmt.Fprintln(stderr, "no results")
		return exitOK, nil
	}
	for _, r := range results {
		// TSV: id\ttitle\turl\texcerpt — newlines in excerpt already collapsed
		excerpt := r.Excerpt
		if len(excerpt) > 200 {
			excerpt = excerpt[:200] + "…"
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", r.PageID, r.Title, r.URL, excerpt)
	}
	return exitOK, nil
}

// runHome manages the local Home cache. Verbs: refresh (force GET), status
// (print metadata), show (print rendered text), query (search content),
// digest (print cached digest).
//
// The cache is read-only for navigation. Writes to the Home (or any page)
// always go through `page apply`, which always GETs fresh ADF before PUT —
// the cache is never the source of truth for an update.
func runHome(args []string, stdout, stderr io.Writer) (int, error) {
	// Default TTL: how long the cache is "fresh enough" without re-fetching.
	// Read paths (--query/--show/--digest) auto-refresh if the cache is older
	// than this; --refresh always fetches regardless. Cross-session staleness
	// is bounded by this value.
	const defaultMaxAge = 1 * time.Hour

	var (
		refresh    bool
		status     bool
		show       bool
		showDigest bool
		query      string
		maxAge     time.Duration = defaultMaxAge
		pageID     string        = "164232" // Home is locked to this ID for now
	)

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for i := 0; i < len(remaining); i++ {
		a := remaining[i]
		switch a {
		case "--refresh":
			refresh = true
		case "--status":
			status = true
		case "--show":
			show = true
		case "--digest":
			showDigest = true
		case "--query":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--query requires a value")
				return exitInputErr, errInvalidUsage
			}
			query = remaining[i+1]
			i++
		case "--max-age":
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--max-age requires a duration (e.g. 1h, 30m)")
				return exitInputErr, errInvalidUsage
			}
			d, dErr := time.ParseDuration(remaining[i+1])
			if dErr != nil {
				fmt.Fprintln(stderr, "--max-age:", dErr)
				return exitInputErr, errInvalidUsage
			}
			maxAge = d
			i++
		case "--page-id":
			// Allow override for advanced users / testing
			if i+1 >= len(remaining) {
				fmt.Fprintln(stderr, "--page-id requires a value")
				return exitInputErr, errInvalidUsage
			}
			pageID = remaining[i+1]
			i++
		case "-h", "--help":
			fmt.Fprintln(stdout, "home — Lybel Confluence Home page cache.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  lybel-docs home --refresh             # always fetch + overwrite cache")
			fmt.Fprintln(stdout, "  lybel-docs home --status              # show cache metadata (read-only)")
			fmt.Fprintln(stdout, "  lybel-docs home --show                # print cached text")
			fmt.Fprintln(stdout, "  lybel-docs home --query \"advisor\"     # grep cached content")
			fmt.Fprintln(stdout, "  lybel-docs home --digest              # print cached page digest")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Auto-refresh rules:")
			fmt.Fprintln(stdout, "  --query/--show/--digest auto-refresh when the cache is missing OR")
			fmt.Fprintln(stdout, "  older than --max-age (default 1h). Callers don't need to think about it.")
			fmt.Fprintln(stdout, "  --refresh ALWAYS fetches, ignoring TTL — use it after another machine")
			fmt.Fprintln(stdout, "  edited the Home and you want immediate sync.")
			fmt.Fprintln(stdout, "  Writes to the Home (page apply, index add/remove/sync) auto-refresh")
			fmt.Fprintln(stdout, "  the cache after the PUT succeeds.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "The cache is read-only for navigation: writes always GET fresh ADF")
			fmt.Fprintln(stdout, "before PUT (atomic), so the cache is never used as the source for an update.")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	// Default action: if no verb given, --status
	if !refresh && !status && !show && !showDigest && query == "" {
		status = true
	}

	// --refresh: always fetch, never short-circuit on cache freshness.
	// The whole point of an explicit --refresh is "I know I want fresh data".
	if refresh {
		client, ok := buildClient(cloud, email, token, stderr)
		if !ok {
			return exitUnknownErr, nil
		}
		c, fErr := fetchHomeCache(client, pageID)
		if fErr != nil {
			fmt.Fprintln(stderr, "refresh failed:", fErr)
			return exitUnknownErr, fErr
		}
		if sErr := adf.SaveHomeCache(c); sErr != nil {
			fmt.Fprintln(stderr, "saving cache:", sErr)
			return exitUnknownErr, sErr
		}
		path, _ := adf.HomeCachePath()
		fmt.Fprintf(stdout, "ok — %s (cached at %s)\n", c.FormatStatus(), path)
		return exitOK, nil
	}

	// Read paths: load cache; auto-refresh if missing or stale (TTL).
	// --status is a read-only metadata check and should NOT auto-refresh.
	cache, loadErr := adf.LoadHomeCache()
	needRefresh := false
	if loadErr != nil {
		if os.IsNotExist(loadErr) {
			needRefresh = (show || showDigest || query != "")
			if !needRefresh {
				fmt.Fprintln(stderr, "no home cache. Run: lybel-docs home --refresh")
				return exitUnknownErr, loadErr
			}
		} else {
			fmt.Fprintln(stderr, "loading cache:", loadErr)
			return exitUnknownErr, loadErr
		}
	} else if !status && cache.IsStale(maxAge) {
		needRefresh = true
	}

	if needRefresh {
		why := "missing"
		if cache != nil {
			why = fmt.Sprintf("stale by %s", formatDurationCompact(cache.Age()))
		}
		fmt.Fprintf(stderr, "(home cache auto-refreshed: %s)\n", why)
		client, ok := buildClient(cloud, email, token, stderr)
		if !ok {
			return exitUnknownErr, nil
		}
		c, fErr := fetchHomeCache(client, pageID)
		if fErr != nil {
			fmt.Fprintln(stderr, "auto-refresh failed:", fErr)
			return exitUnknownErr, fErr
		}
		if sErr := adf.SaveHomeCache(c); sErr != nil {
			fmt.Fprintln(stderr, "saving cache:", sErr)
			return exitUnknownErr, sErr
		}
		cache = c
	}

	if status {
		fmt.Fprintln(stdout, cache.FormatStatus())
		path, _ := adf.HomeCachePath()
		fmt.Fprintf(stdout, "  path: %s\n", path)
		fmt.Fprintf(stdout, "  url:  %s\n", cache.URL)
		fmt.Fprintf(stdout, "  size: %d bytes (text content)\n", len(cache.TextContent))
		return exitOK, nil
	}
	if showDigest {
		fmt.Fprint(stdout, cache.Digest.FormatText())
		return exitOK, nil
	}
	if show {
		fmt.Fprint(stdout, cache.TextContent)
		return exitOK, nil
	}
	if query != "" {
		matches := grepHome(cache.TextContent, query)
		if len(matches) == 0 {
			fmt.Fprintf(stderr, "no matches for %q in cached home (v%d)\n", query, cache.Version)
			return exitOK, nil
		}
		for _, m := range matches {
			if m.heading != "" {
				fmt.Fprintf(stdout, "## %s\n", m.heading)
			}
			fmt.Fprintf(stdout, "  %s\n", m.line)
		}
		return exitOK, nil
	}

	return exitOK, nil
}

// fetchHomeCache fetches the Home page and builds a HomeCache (digest + text
// rendering) without writing to disk. Caller is responsible for SaveHomeCache.
func fetchHomeCache(client *adf.ConfluenceClient, pageID string) (*adf.HomeCache, error) {
	meta, err := client.GetPage(pageID, "atlas_doc_format")
	if err != nil {
		return nil, fmt.Errorf("get home page: %w", err)
	}
	if meta.Body.AtlasDocFormat.Value == "" {
		return nil, fmt.Errorf("home page has no ADF body")
	}
	doc, err := adf.UnmarshalDoc([]byte(meta.Body.AtlasDocFormat.Value))
	if err != nil {
		return nil, fmt.Errorf("parse home ADF: %w", err)
	}
	url := client.PageURL(meta.Links.WebUI)
	digest := adf.BuildDigest(doc, meta.ID, meta.Title, url, meta.Version.Number)
	textContent := adf.RenderText(doc)
	return &adf.HomeCache{
		FetchedAt:   time.Now().UTC(),
		PageID:      meta.ID,
		Title:       meta.Title,
		Version:     meta.Version.Number,
		URL:         url,
		TextContent: textContent,
		Digest:      digest,
	}, nil
}

// refreshHomeCacheAfterWrite re-fetches and saves the Home cache when a write
// operation has just modified the Home page. This is the auto-refresh-on-write
// path: the caller's session sees the new state immediately, without needing
// an explicit `home --refresh`.
//
// No-op when pageID isn't the Home (the only page we cache today). Errors
// are reported to stderr but don't fail the calling write — the write itself
// already succeeded; cache freshness is best-effort.
func refreshHomeCacheAfterWrite(pageID string, client *adf.ConfluenceClient, stderr io.Writer) {
	if pageID != homePageID {
		return
	}
	c, err := fetchHomeCache(client, homePageID)
	if err != nil {
		fmt.Fprintf(stderr, "(warning: home cache refresh after write failed: %v)\n", err)
		return
	}
	if err := adf.SaveHomeCache(c); err != nil {
		fmt.Fprintf(stderr, "(warning: saving refreshed home cache failed: %v)\n", err)
		return
	}
	fmt.Fprintf(stderr, "(home cache auto-refreshed: v%d)\n", c.Version)
}

// homeMatch is a single hit from grepHome.
type homeMatch struct {
	heading string // closest preceding heading, "" if none
	line    string // the matched line, trimmed
}

// grepHome does a case-insensitive substring search over the cached Home text
// content. Each match carries the closest preceding heading as section context,
// so the LLM caller can see where it lives.
//
// Headings are detected by the markdown-ish output of adf.RenderText (lines
// starting with `# `, `## `, etc.). A 30-line break between matches with the
// same heading collapses them into a single bullet group.
func grepHome(content, query string) []homeMatch {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var matches []homeMatch
	currentHeading := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isMarkdownHeading(trimmed) {
			currentHeading = trimHashPrefix(trimmed)
			continue
		}
		if strings.Contains(strings.ToLower(trimmed), q) {
			matches = append(matches, homeMatch{heading: currentHeading, line: trimmed})
		}
	}
	// Dedupe: collapse consecutive matches under the same heading.
	if len(matches) <= 1 {
		return matches
	}
	out := make([]homeMatch, 0, len(matches))
	for i, m := range matches {
		if i > 0 && m.heading == matches[i-1].heading {
			out = append(out, homeMatch{heading: "", line: m.line})
			continue
		}
		out = append(out, m)
	}
	return out
}

func isMarkdownHeading(line string) bool {
	if len(line) < 2 || line[0] != '#' {
		return false
	}
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	return i >= 1 && i <= 6 && i < len(line) && line[i] == ' '
}

func trimHashPrefix(line string) string {
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	return strings.TrimSpace(line[i:])
}

// formatDurationCompact: 5s, 12m, 3h, 2d
func formatDurationCompact(d time.Duration) string {
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	if m < 60 {
		return fmt.Sprintf("%dm", m)
	}
	h := m / 60
	if h < 24 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dd", h/24)
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

	// Auto-refresh home cache only if we wrote to the live API (not --input file
	// and not dry-run). ctx.pageID is set when the doc was loaded from the API.
	if !dryRun && ctx.pageID != "" {
		refreshHomeCacheAfterWrite(ctx.pageID, client, stderr)
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

	if !dryRun && ctx.pageID != "" {
		refreshHomeCacheAfterWrite(ctx.pageID, client, stderr)
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

	if !dryRun && ctx.pageID != "" && added > 0 {
		refreshHomeCacheAfterWrite(ctx.pageID, client, stderr)
	}

	fmt.Fprintf(stdout, `{"status":"ok","added":%d}`+"\n", added)
	return exitOK, nil
}

// runUpdate self-updates the lybel-docs install. With --check it only
// reports whether a newer release is available (exit 0 = up to date,
// exit 10 = update available). Without --check it shells out to the
// public installer (install.sh on Unix, install.ps1 on Windows) which
// downloads the latest release zip, replaces the binary + skill files,
// and refreshes the symlink — the same flow a user would run via the
// curl one-liner, but invokable as a single command Claude can run when
// a user says "atualiza a skill".
//
// Why exec the installer instead of doing the download in-process:
// the installer already handles platform detection, archive layout,
// symlink+PATH registration, credential preservation, and Windows User
// PATH registration. Re-implementing that in Go would duplicate logic
// that we'd then have to keep in sync with two separate scripts.
func runUpdate(args []string, stdout, stderr io.Writer) (int, error) {
	const (
		repoOwnerRepo = "lybel-app/skills"
		installShURL  = "https://raw.githubusercontent.com/lybel-app/skills/main/cli/lybel-docs/install/install.sh"
		installPS1URL = "https://raw.githubusercontent.com/lybel-app/skills/main/cli/lybel-docs/install/install.ps1"
		// exit 10 is reserved for "update available" so scripts/CI can
		// distinguish "all good" (0) from "needs upgrade" without parsing.
		exitUpdateAvailable = 10
	)

	var checkOnly bool
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--check":
			checkOnly = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "update — fetch the latest release of lybel-docs.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  lybel-docs update            # download + install latest release")
			fmt.Fprintln(stdout, "  lybel-docs update --check    # only report whether an update is available")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Behavior: resolves the latest release tag from GitHub, compares with the")
			fmt.Fprintln(stdout, "currently-installed version, and (unless --check) shells out to install.sh")
			fmt.Fprintln(stdout, "(or install.ps1 on Windows) to perform the upgrade. Credentials and the")
			fmt.Fprintln(stdout, "home cache are preserved across the update.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "Exit codes:")
			fmt.Fprintln(stdout, "  0   up to date (or upgrade succeeded)")
			fmt.Fprintln(stdout, "  10  --check: an update is available")
			fmt.Fprintln(stdout, "  3   network error / installer failure")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	latest, err := resolveLatestVersion(repoOwnerRepo)
	if err != nil {
		fmt.Fprintln(stderr, "could not resolve latest version:", err)
		return exitUnknownErr, err
	}

	current := version
	if normalizeVersion(current) == normalizeVersion(latest) {
		fmt.Fprintf(stdout, "lybel-docs is up to date (%s).\n", current)
		return exitOK, nil
	}

	if checkOnly {
		fmt.Fprintf(stdout, "current: %s\nlatest:  %s\nrun: lybel-docs update\n", current, latest)
		return exitUpdateAvailable, nil
	}

	fmt.Fprintf(stdout, "Updating lybel-docs: %s → %s ...\n", current, latest)

	// Shell out to the public installer. This works for Linux/macOS via
	// `curl | bash`. On Windows the equivalent is `iwr | iex` in PowerShell.
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// Try to move the running binary out of the way so install.ps1 can
		// overwrite the destination cleanly. Best-effort — on failure the
		// installer will likely fail with a clear error from Windows.
		if exe, eerr := os.Executable(); eerr == nil {
			_ = os.Rename(exe, exe+".old")
		}
		cmd = exec.Command("powershell", "-NoProfile", "-Command",
			fmt.Sprintf("iwr -useb %s | iex", installPS1URL))
	default:
		cmd = exec.Command("sh", "-c",
			fmt.Sprintf("curl -fsSL %s | bash", installShURL))
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(stderr, "installer failed:", err)
		return exitUnknownErr, err
	}
	return exitOK, nil
}

// resolveLatestVersion returns the latest release tag for repo ("owner/repo")
// by following GitHub's /releases/latest redirect. Same approach as install.sh.
func resolveLatestVersion(repo string) (string, error) {
	url := fmt.Sprintf("https://github.com/%s/releases/latest", repo)
	client := &http.Client{
		Timeout: 10 * time.Second,
		// Don't follow the redirect — we want to read the Location header.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("no Location header (status %d) — repo may be empty or unreachable", resp.StatusCode)
	}
	// Location looks like https://github.com/owner/repo/releases/tag/v0.3.3
	idx := strings.LastIndex(loc, "/tag/")
	if idx < 0 {
		return "", fmt.Errorf("unexpected Location format: %s", loc)
	}
	tag := strings.TrimSpace(loc[idx+len("/tag/"):])
	if tag == "" {
		return "", fmt.Errorf("empty tag in Location: %s", loc)
	}
	return tag, nil
}

// normalizeVersion strips a leading "v" so "v0.3.3" and "0.3.3" compare equal.
// Also handles the build-time "dev" / "v0.3.0-3-g734f5ea-dirty" variants — for
// those, equality with a clean tag is impossible, which is the intent (a dev
// build should always show as "behind").
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	return v
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
