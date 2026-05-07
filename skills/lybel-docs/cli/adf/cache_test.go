package adf

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHomeCache_RoundTrip(t *testing.T) {
	// Redirect cache to a tempdir so the test doesn't touch the user's real cache.
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	// On macOS os.UserCacheDir checks $HOME/Library/Caches; override too.
	t.Setenv("HOME", tmp)

	original := &HomeCache{
		FetchedAt:   time.Now().UTC().Truncate(time.Second),
		PageID:      "164232",
		Title:       "Home",
		Version:     33,
		URL:         "https://example.com/p/164232",
		TextContent: "## Hello\n\nworld\n",
		Digest: Digest{
			PageID:      "164232",
			Title:       "Home",
			URL:         "https://example.com/p/164232",
			Version:     33,
			TotalWords:  1,
			MacroCounts: map[string]int{},
			Sections: []SectionSummary{
				{Level: 2, Heading: "Hello", Words: 1},
			},
			LinksCount: 0,
		},
	}

	if err := SaveHomeCache(original); err != nil {
		t.Fatalf("save: %v", err)
	}

	path, err := HomeCachePath()
	if err != nil {
		t.Fatalf("HomeCachePath: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cache file not found at %s: %v", path, err)
	}

	loaded, err := LoadHomeCache()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.PageID != "164232" {
		t.Errorf("PageID mismatch: %q", loaded.PageID)
	}
	if loaded.Version != 33 {
		t.Errorf("Version mismatch: %d", loaded.Version)
	}
	if loaded.TextContent != original.TextContent {
		t.Errorf("TextContent mismatch")
	}
	if !loaded.FetchedAt.Equal(original.FetchedAt) {
		t.Errorf("FetchedAt mismatch: %v vs %v", loaded.FetchedAt, original.FetchedAt)
	}
}

func TestHomeCache_IsStale(t *testing.T) {
	c := &HomeCache{FetchedAt: time.Now().UTC().Add(-2 * time.Hour)}
	if c.IsStale(time.Hour) != true {
		t.Errorf("expected stale at maxAge=1h")
	}
	if c.IsStale(3*time.Hour) != false {
		t.Errorf("expected fresh at maxAge=3h")
	}
}

func TestHomeCache_LoadMissingReturnsNotExist(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("HOME", tmp)

	_, err := LoadHomeCache()
	if err == nil {
		t.Fatal("expected error on missing cache")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist, got %v", err)
	}
}
