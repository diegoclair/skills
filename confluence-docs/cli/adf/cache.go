// Package adf - Local cache for the Confluence Home page.
//
// Design rules (set by user, see git history):
//   - The cache is read-only for navigation/queries — it MUST NEVER be the
//     source of truth for an update. Any write to a Confluence page goes
//     through `page apply`, which always GETs fresh ADF before PUT.
//   - "git pull"-style refresh: --refresh re-fetches and overwrites cache.
//     --query reads cache; if missing, auto-refreshes; if stale, warns but
//     still serves.
//   - One file per page (right now only Home, but the layout supports more).
package adf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HomeCache is the on-disk shape of the cached Home page.
type HomeCache struct {
	FetchedAt   time.Time `json:"fetchedAt"`
	PageID      string    `json:"pageId"`
	Title       string    `json:"title"`
	Version     int       `json:"version"`
	URL         string    `json:"url"`
	TextContent string    `json:"textContent"` // ADF rendered to plain text
	Digest      Digest    `json:"digest"`
}

// HomeCachePath returns the platform-appropriate path to the Home cache file:
//   - Linux:   $XDG_CACHE_HOME/confluence-docs/home.json (or ~/.cache/confluence-docs/home.json)
//   - macOS:   ~/Library/Caches/confluence-docs/home.json
//   - Windows: %LocalAppData%/confluence-docs/home.json
func HomeCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve cache dir: %w", err)
	}
	return filepath.Join(dir, "confluence-docs", "home.json"), nil
}

// LoadHomeCache reads and parses the cache file. Returns os.ErrNotExist (wrapped)
// if the file does not exist.
func LoadHomeCache() (*HomeCache, error) {
	path, err := HomeCachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c HomeCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse cache file %s: %w", path, err)
	}
	return &c, nil
}

// SaveHomeCache writes the cache to disk, creating parent dirs as needed.
func SaveHomeCache(c *HomeCache) error {
	path, err := HomeCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write cache file: %w", err)
	}
	return nil
}

// Age returns how long ago the cache was fetched.
func (c *HomeCache) Age() time.Duration {
	return time.Since(c.FetchedAt)
}

// IsStale reports whether the cache is older than maxAge.
func (c *HomeCache) IsStale(maxAge time.Duration) bool {
	return c.Age() > maxAge
}

// FormatStatus returns a one-line human-readable status of the cache.
func (c *HomeCache) FormatStatus() string {
	age := c.Age().Round(time.Second)
	return fmt.Sprintf("home v%d cached %s ago (fetched %s, %s)",
		c.Version,
		formatDuration(age),
		c.FetchedAt.Format("2006-01-02 15:04 MST"),
		c.Title,
	)
}

// formatDuration returns a compact human-readable duration: "5s", "12m", "3h", "2d".
func formatDuration(d time.Duration) string {
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
