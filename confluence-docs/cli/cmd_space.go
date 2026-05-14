// cmd_space.go — `confluence-docs space` subcommand family.
//
// Subcommands:
//
//	confluence-docs space list            List accessible spaces (cached 1h)
//	confluence-docs space use <key>       Switch the active space
//	confluence-docs space current         Print the currently active space
//
// The cache lives at ~/.cache/confluence-docs/spaces.json (JSON array).
// TTL is 1h; use --refresh on `space list` to force a fresh fetch.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lybel-app/skills/pkg/atlassian/adf"
	"github.com/lybel-app/skills/pkg/atlassian/setup"
)

// spacesCacheTTL is how long the spaces list is considered fresh.
const spacesCacheTTL = 1 * time.Hour

// spacesCacheEntry is what we store in the JSON cache file.
type spacesCacheEntry struct {
	FetchedAt time.Time         `json:"fetchedAt"`
	Spaces    []adf.SpaceResult `json:"spaces"`
}

// spacesCachePath returns the path to the spaces cache file.
func spacesCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve cache dir: %w", err)
	}
	return filepath.Join(dir, "confluence-docs", "spaces.json"), nil
}

// loadSpacesCache reads the cache file. Returns nil if missing, expired, or unreadable.
func loadSpacesCache() *spacesCacheEntry {
	path, err := spacesCachePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entry spacesCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}
	if time.Since(entry.FetchedAt) > spacesCacheTTL {
		return nil // expired
	}
	return &entry
}

// saveSpacesCache writes the spaces list to disk. Errors are ignored (best-effort).
func saveSpacesCache(spaces []adf.SpaceResult) {
	path, err := spacesCachePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	entry := spacesCacheEntry{FetchedAt: time.Now().UTC(), Spaces: spaces}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}

// fetchAndCacheSpaces calls the API and saves the result.
func fetchAndCacheSpaces(client *adf.ConfluenceClient) ([]adf.SpaceResult, error) {
	spaces, err := client.ListSpaces()
	if err != nil {
		return nil, err
	}
	saveSpacesCache(spaces)
	return spaces, nil
}

// getSpaces returns the spaces list, using cache when fresh and API when stale/missing.
func getSpaces(client *adf.ConfluenceClient, forceRefresh bool) ([]adf.SpaceResult, bool, error) {
	if !forceRefresh {
		if cached := loadSpacesCache(); cached != nil {
			return cached.Spaces, true, nil
		}
	}
	spaces, err := fetchAndCacheSpaces(client)
	return spaces, false, err
}

// runSpace dispatches space subcommands.
func runSpace(args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printSpaceHelp(stdout)
		return exitOK, nil
	}

	switch args[0] {
	case "list":
		return runSpaceList(args[1:], stdout, stderr)
	case "use":
		return runSpaceUse(args[1:], stdout, stderr)
	case "current":
		return runSpaceCurrent(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "space: unknown subcommand:", args[0])
		fmt.Fprintln(stderr, "  valid subcommands: list, use, current")
		return exitInputErr, errInvalidUsage
	}
}

func printSpaceHelp(w io.Writer) {
	fmt.Fprintln(w, "space — manage Confluence space selection")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  confluence-docs space list             List accessible spaces")
	fmt.Fprintln(w, "  confluence-docs space use <key>        Switch active space by key")
	fmt.Fprintln(w, "  confluence-docs space current          Print active space info")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "FLAGS for `space list`:")
	fmt.Fprintln(w, "  --refresh     Force refresh from API (ignore cache)")
	fmt.Fprintln(w, "  --json        JSON output: [{id, key, name, homepageId, active}]")
	fmt.Fprintln(w, "  --text        Human-readable table (default)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "FLAGS for `space current`:")
	fmt.Fprintln(w, "  --json        JSON output: {id, key, name, homepageId}")
}

// runSpaceList implements `confluence-docs space list`.
func runSpaceList(args []string, stdout, stderr io.Writer) (int, error) {
	var (
		asJSON       bool
		forceRefresh bool
	)

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for _, a := range remaining {
		switch a {
		case "--json":
			asJSON = true
		case "--text":
			asJSON = false
		case "--refresh":
			forceRefresh = true
		case "-h", "--help":
			printSpaceHelp(stdout)
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "space list: unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	spaces, fromCache, err := getSpaces(client, forceRefresh)
	if err != nil {
		fmt.Fprintln(stderr, "space list:", err)
		return exitUnknownErr, err
	}

	activeCfg := adf.ReadActiveConfig()

	if fromCache {
		fmt.Fprintln(stderr, "(spaces loaded from cache — use --refresh to update)")
	}

	if asJSON {
		type jsonSpace struct {
			ID         string `json:"id"`
			Key        string `json:"key"`
			Name       string `json:"name"`
			HomepageID string `json:"homepageId"`
			Active     bool   `json:"active"`
		}
		out := make([]jsonSpace, 0, len(spaces))
		for _, s := range spaces {
			out = append(out, jsonSpace{
				ID:         s.ID,
				Key:        s.Key,
				Name:       s.Name,
				HomepageID: s.HomepageID,
				Active:     s.Key == activeCfg.SpaceKey,
			})
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return exitOK, nil
	}

	// TSV / text output.
	for _, s := range spaces {
		marker := " "
		if s.Key == activeCfg.SpaceKey {
			marker = "✓"
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", marker, s.ID, s.Key, s.Name)
	}
	return exitOK, nil
}

// runSpaceUse implements `confluence-docs space use <key>`.
func runSpaceUse(args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "space use: requires a space key (e.g. `space use myspace`)")
		return exitInputErr, errInvalidUsage
	}

	targetKey := args[0]
	rest := args[1:]

	remaining, cloud, email, token, err := parseCommonPageFlags(rest)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}
	if len(remaining) > 0 {
		fmt.Fprintln(stderr, "space use: unexpected arguments:", strings.Join(remaining, " "))
		return exitInputErr, errInvalidUsage
	}

	client, ok := buildClient(cloud, email, token, stderr)
	if !ok {
		return exitUnknownErr, nil
	}

	// Try cache first; if key not found, refresh.
	spaces, _, err := getSpaces(client, false)
	if err != nil {
		fmt.Fprintln(stderr, "space use: fetching spaces:", err)
		return exitUnknownErr, err
	}

	var found *adf.SpaceResult
	for i := range spaces {
		if strings.EqualFold(spaces[i].Key, targetKey) {
			found = &spaces[i]
			break
		}
	}

	if found == nil {
		// Cache miss — force refresh and try again.
		spaces, _, err = getSpaces(client, true)
		if err != nil {
			fmt.Fprintln(stderr, "space use: refreshing spaces:", err)
			return exitUnknownErr, err
		}
		for i := range spaces {
			if strings.EqualFold(spaces[i].Key, targetKey) {
				found = &spaces[i]
				break
			}
		}
	}

	if found == nil {
		fmt.Fprintf(stderr, "space use: key %q not found\n", targetKey)
		fmt.Fprintln(stderr, "  available keys:")
		for _, s := range spaces {
			fmt.Fprintf(stderr, "    %s (%s)\n", s.Key, s.Name)
		}
		return exitInputErr, errInvalidUsage
	}

	// Read current config and update space fields.
	cfg := setup.ReadConfigFile()
	cfg.SpaceID = found.ID
	cfg.SpaceKey = found.Key
	cfg.SpaceName = found.Name
	cfg.HomePageID = found.HomepageID

	if err := setup.WriteConfig(cfg); err != nil {
		fmt.Fprintln(stderr, "space use: writing config:", err)
		return exitUnknownErr, err
	}

	// Fetch home page title for confirmation message.
	homeTitle := ""
	if found.HomepageID != "" {
		if meta, metaErr := client.GetPage(found.HomepageID, ""); metaErr == nil {
			homeTitle = meta.Title
		}
	}

	fmt.Fprintf(stdout, "Switched to %q (key: %s, id: %s).", found.Name, found.Key, found.ID)
	if homeTitle != "" {
		fmt.Fprintf(stdout, " Home: %q (id: %s).", homeTitle, found.HomepageID)
	} else if found.HomepageID != "" {
		fmt.Fprintf(stdout, " Home: id %s.", found.HomepageID)
	}
	fmt.Fprintln(stdout)
	return exitOK, nil
}

// runSpaceCurrent implements `confluence-docs space current`.
func runSpaceCurrent(args []string, stdout, stderr io.Writer) (int, error) {
	var asJSON bool

	remaining, cloud, email, token, err := parseCommonPageFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInputErr, errInvalidUsage
	}

	for _, a := range remaining {
		switch a {
		case "--json":
			asJSON = true
		case "-h", "--help":
			printSpaceHelp(stdout)
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "space current: unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	cfg := adf.ReadActiveConfig()
	if cfg.SpaceID == "" || cfg.SpaceKey == "" {
		fmt.Fprintln(stderr, "no active space configured — run `confluence-docs setup` or `space use <key>`")
		return exitInputErr, errInvalidUsage
	}

	// Optionally fetch the home page title.
	homeTitle := ""
	if cfg.HomePageID != "" {
		// Only fetch if we have a client available.
		if client, ok := buildClient(cloud, email, token, stderr); ok {
			if meta, metaErr := client.GetPage(cfg.HomePageID, ""); metaErr == nil {
				homeTitle = meta.Title
			}
		}
	}

	if asJSON {
		out := map[string]string{
			"id":         cfg.SpaceID,
			"key":        cfg.SpaceKey,
			"name":       cfg.SpaceName,
			"homepageId": cfg.HomePageID,
		}
		if homeTitle != "" {
			out["homepageTitle"] = homeTitle
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return exitOK, nil
	}

	fmt.Fprintf(stdout, "Active space: %s (key: %s, id: %s)\n", cfg.SpaceName, cfg.SpaceKey, cfg.SpaceID)
	if cfg.HomePageID != "" {
		if homeTitle != "" {
			fmt.Fprintf(stdout, "Home page:    %q (id: %s)\n", homeTitle, cfg.HomePageID)
		} else {
			fmt.Fprintf(stdout, "Home page ID: %s\n", cfg.HomePageID)
		}
	}
	return exitOK, nil
}
