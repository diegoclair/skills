// confluence-docs converts extended markdown (with Confluence macros) into ADF
// JSON for use with the Atlassian Confluence REST API, edits existing
// ADF documents by section without losing macros, and talks directly to
// the Confluence Cloud REST API v2 to get/upload/create pages.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/diegoclair/skills/pkg/atlassian/setup"
)

func init() {
	// Scope the shared setup package's per-skill config (active space, home
	// page ID, etc.) to this skill. Credentials themselves live atlassian-wide
	// at ~/.config/atlassian/credentials — see pkg/atlassian/setup.ConfigPath.
	// Explicit even though "confluence-docs" is the package default: pair with
	// the equivalent call in jira-tickets/cli/main.go so neither skill relies
	// on a hidden default.
	setup.SetSkillName("confluence-docs")
}

// version is injected at build time via -ldflags "-X main.version=..."
// Falls back to the source-tree version when not set via ldflags (dev builds).
var version = "v0.14.1"

const helpText = `confluence-docs — Confluence ADF toolkit: convert, edit, lint, and publish pages.

USAGE:
  confluence-docs setup        [--email X --token Y | --check | --print-config-path]
  confluence-docs update       [--check]
  confluence-docs adf          [--file PATH] [--pretty]
  confluence-docs edit         [--input PATH] OPERATION [--at-level N] [--pretty]
  confluence-docs page         VERB [flags]
  confluence-docs search       "term" [--limit N] [--space lybel] [--cql RAW] [--json]
  confluence-docs home         [--refresh | --status | --show | --query "X" | --digest] [--max-age 24h]
  confluence-docs lint         FILE.json
  confluence-docs extract-body [< mcp-response.json]
  confluence-docs index        VERB [flags]
  confluence-docs check        --title "..." [--type TYPE] [--tags t1,t2] [--threshold 0.7]
  confluence-docs new          TYPE --title "..." [--parent-id ID] [--full-width] [--output FILE]
  confluence-docs km           SUBCMD [flags]
  confluence-docs --version
  confluence-docs --help

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
  check         Fuzzy title search before creating a page. Returns JSON with exists/similar/suggestion.
  new           Generate a markdown template for a new page by doc type (reference/decision/explanation/how-to/capture).
  km            Knowledge Map: generate and upload the Lybel KNOWLEDGE_MAP page (subcommands: generate, classify).

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
  --table-update-row "Heading" --match-cell "text" --row "a|b|c"  Replace a row.
  --table-update-cell "Heading" --match-cell "text" --col-name "Header" --value "v"
                                                   Update a single cell by column name.
  --table-move-row "Heading" --match-cell "text" --position N
                                                   Move a row to position N (1-indexed
                                                   across data rows; header at row 0 is
                                                   never moved; out-of-range clamps to
                                                   first/last).
  Any --match-cell flag also accepts --match-col COL --match-value V to match
  against an arbitrary column (located by header name) instead of the first
  column. --match-cell and --match-col are mutually exclusive.

EDIT FLAGS:
  -i, --input PATH   Read ADF from PATH instead of stdin. Use - for stdin.
      --at-level N   Match the heading only at level N (1-6). Default: first match.
      --after-row "text"  (--table-add-row) Insert after row whose first cell contains text.
      --if-missing   (--table-add-row) Skip silently if row with same first cell exists.
      --col-name "Header"  (--table-update-cell) Column header text to identify target cell.
      --value "text"       (--table-update-cell) New cell content.
      --position N         (--table-move-row) New row position (1-indexed across data rows).
      --match-col "Header" (table ops) Column header used for row match.
      --match-value "text" (table ops) Value to find in --match-col.
                          --match-col + --match-value together replace --match-cell
                          when the first column is not unique enough (rank, ID, etc.).
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
  page upload  --page-id ID (--adf FILE | --markdown FILE) [--title TITLE]
               [--message MSG] [--dry-run] [--cloud SUBDOMAIN] [--email EMAIL] [--token TOKEN]
               Replace the entire body of an existing page. --markdown is the
               recommended path for full-page rewrites: write the new content
               as markdown locally, then upload — single GET (for version) +
               single PUT, no per-section diffing. For surgical edits, prefer
               'page apply --replace-section' etc.
  page create  --space-id ID --parent-id ID --title TITLE
               [--markdown FILE | --adf FILE] [--cloud SUBDOMAIN]
               [--full-width | --fixed-width]
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
                 --table-update-row "Heading" --match-cell "text" --row "a|b|c"
                 --table-update-cell "Heading" --match-cell "text" --col-name "Header" --value "v"
                 --table-move-row   "Heading" --match-cell "text" --position N
                 Each --match-cell may be replaced by --match-col COL --match-value V
                 to match an arbitrary column (e.g. when col 1 holds a rank/ID).
                 --multi OPS.json    Apply many ops atomically in 1 GET+PUT.
               In --row, '|' is the cell separator. To include a literal pipe
               character inside a cell, escape it with a backslash, e.g.:
               --row "Foo (A\|B)|123"  → cells: ["Foo (A|B)", "123"].
  page move    --page-id ID [--parent-id NEW_PARENT_ID] [--title NEW_TITLE]
               [--message MSG] [--dry-run]
               Rename a page (--title) and/or move it under a new parent
               (--parent-id). At least one of the two is required. The body
               is preserved (refetched and re-PUT, since v2 PUT requires it).
  page reorder --page-id ID (--before TARGET_ID | --after TARGET_ID
               | --append-to NEW_PARENT_ID) [--dry-run]
               Reposition a page among its siblings (--before / --after) or
               append it as the last child of a new parent (--append-to).
               Wraps the v1 endpoint
               PUT /wiki/rest/api/content/{id}/move/{position}/{targetId}
               since v2 doesn't expose sibling-order control. Body and
               title are not touched. --dry-run prints the intended JSON
               action without making the API call (no creds required).
  page delete  --page-id ID [--yes]
               Trash a page (soft delete; restorable from Confluence trash).
               --yes is required to confirm.
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
  confluence-docs lint page.json
    Exits 0 if clean, 1 if errors found. Diagnostics on stderr.

EXTRACT-BODY:
  confluence-docs extract-body < mcp-response.json > adf-body.json
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
  :::properties                      Page Properties macro (key: value pairs); close with :::
    tipo: reference
    status: ativo
    relacionados: [[Page Title]], [[id:12345]]
    criado: 2026-05-01
  :::
  ![embed](URL)                      EmbedCard (YouTube, Figma, etc.) — standalone line.
  [text](URL)                        BlockCard when on a line by itself (smart link).
  https://...                        BlockCard when bare URL on its own line.

SEARCH:
  search "term" [--limit N] [--space KEY] [--json]
    Default CQL: space="<KEY>" AND type="page" AND (title ~ "term" OR text ~ "term")
    Default space: lybel. Default limit: 10. Output: TSV (id, title, url, excerpt).
  search --cql 'space=lybel AND label="adr"'
    Pass raw CQL — caller is responsible for escaping.

HOME CACHE:
  home --refresh                 Force GET + cache (~/.cache/confluence-docs/home.json).
  home --status                  Print cache metadata (default if no flag).
  home --show                    Print cached page text (markdown rendering of ADF).
  home --query "term"            Grep cached content; auto-refresh if cache missing.
  home --digest                  Print cached digest.
  home --max-age DURATION        Stale threshold for warning (default 24h).
  WRITES: the cache is read-only for navigation. 'page apply' always GETs
  fresh ADF before PUT — the cache is NEVER used as the source for updates.

EXAMPLES:
  # Convert markdown to ADF
  confluence-docs adf < page.md > page.adf.json

  # Slim summary of a page (cheap; LLM-friendly)
  confluence-docs page digest --page-id 164232

  # Atomic update without ever loading the full ADF into the caller
  confluence-docs page apply --page-id 164232 \
    --replace-section "Roadmap" --fragment new.md --message "rewrite roadmap"

  # Search the lybel space
  confluence-docs search "advisor" --limit 5

  # Append a new section (preserves all macros)
  confluence-docs edit --input page.json --append new-section.md > updated.json

  # Replace only the h3 "Ops" (not the h2 "Ops")
  confluence-docs edit --input page.json --replace-section "Ops" --at-level 3 fragment.md > out.json

  # Add a row to a table inside a section
  confluence-docs edit --input page.json --table-add-row "Page ID Index" \
    --row "My Page|987654" --if-missing > updated.json

  # Fetch a page as ADF
  confluence-docs page get --page-id 164232 --format adf --output current.json

  # Upload edited ADF back to Confluence (preview first, then commit)
  confluence-docs page upload --page-id 164232 --adf updated.json --dry-run
  confluence-docs page upload --page-id 164232 --adf updated.json --message "add row"

  # Create a new page
  confluence-docs page create --space-id 131352 --parent-id 164232 \
    --title "New Page" --markdown content.md

  # Unwrap MCP response body
  confluence-docs extract-body < mcp-response.json > body.json

  # Validate ADF structure
  confluence-docs lint page.json

  # Add entry to Home page index
  confluence-docs index add --page-id 999 --title "My Page" --under "Sócios"

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
		fmt.Fprintln(os.Stderr, "confluence-docs:", err)
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
		fmt.Fprintln(stdout, "confluence-docs", version)
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
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "new":
		return runNew(args[1:], stdin, stdout, stderr)
	case "km":
		return runKM(args[1:], stdin, stdout, stderr)
	case "space":
		return runSpace(args[1:], stdout, stderr)
	}

	fmt.Fprintln(stderr, "unknown command:", args[0])
	fmt.Fprint(stderr, helpText)
	return exitInputErr, errInvalidUsage
}
