package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/diegoclair/skills/pkg/atlassian/adf"
)

// ── index command ──────────────────────────────────────────────────────────

const (
	indentNone   = ""
	indentLevel1 = "↳ "
	indentLevel2 = "↳↳ "
)

// configNotSetErr is the error returned when a required config value is missing.
var errConfigNotSet = fmt.Errorf("no active space configured — run `confluence-docs setup` or `confluence-docs space use <key>`")

// currentHomePageID returns the home page ID from config.
func currentHomePageID() (string, error) {
	cfg := adf.ReadActiveConfig()
	if cfg.HomePageID == "" {
		return "", errConfigNotSet
	}
	return cfg.HomePageID, nil
}

// currentSpaceID returns the active space ID from config.
func currentSpaceID() (string, error) {
	cfg := adf.ReadActiveConfig()
	if cfg.SpaceID == "" {
		return "", errConfigNotSet
	}
	return cfg.SpaceID, nil
}

// currentSpaceKey returns the active space key from config.
func currentSpaceKey() (string, error) {
	cfg := adf.ReadActiveConfig()
	if cfg.SpaceKey == "" {
		return "", errConfigNotSet
	}
	return cfg.SpaceKey, nil
}

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
	doc       adf.Node
	inputFile string // empty = fetched from API
	pageID    string // the page to write back to (if fetched from API)
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

	// Fetch from API using configured home page ID.
	homeID, err := currentHomePageID()
	if err != nil {
		return nil, fmt.Errorf("home page not configured: %w", err)
	}
	meta, err := client.GetPage(homeID, "atlas_doc_format")
	if err != nil {
		return nil, fmt.Errorf("fetching Home page (ID %s): %w", homeID, err)
	}
	adfStr := meta.Body.AtlasDocFormat.Value
	if adfStr == "" {
		return nil, fmt.Errorf("Home page has no ADF body")
	}
	doc, err := adf.UnmarshalDoc([]byte(adfStr))
	if err != nil {
		return nil, fmt.Errorf("parsing Home page ADF: %w", err)
	}
	return &indexPageContext{doc: doc, inputFile: "", pageID: homeID}, nil
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

	// In --input file mode, no HTTP round-trip happens (read from file,
	// write to file or stdout); credentials aren't needed.
	var client *adf.ConfluenceClient
	if inputFile == "" {
		var ok bool
		client, ok = buildClient(cloud, email, token, stderr)
		if !ok {
			return exitUnknownErr, nil
		}
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

	updated, existed, err := adf.TableAddRow(ctx.doc, under, 0, rowText, "", ifMissing, adf.MatchSpec{})
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

	// In --input file mode, no HTTP round-trip happens (read from file,
	// write to file or stdout); credentials aren't needed.
	var client *adf.ConfluenceClient
	if inputFile == "" {
		var ok bool
		client, ok = buildClient(cloud, email, token, stderr)
		if !ok {
			return exitUnknownErr, nil
		}
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
		updated, existed, addErr := adf.TableAddRow(doc, under, 0, rowText, "", true, adf.MatchSpec{})
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

// runUpdate self-updates the confluence-docs install. With --check it only
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
