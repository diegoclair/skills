package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// themeDir is the subdirectory under XDG_CONFIG_HOME / ~/.config
	// where user-authored custom themes are stored.
	themeDir = "social-carousel/themes"

	// embeddedThemePrefix is the path inside templatesFS where preset
	// theme YAML files live.
	embeddedThemePrefix = "templates/themes"
)

// loadTheme resolves a theme name or path and returns a populated Theme.
//
// Resolution order:
//  1. If name looks like a path (contains "/" or ends with ".yaml"/".yml"),
//     read directly from disk.
//  2. ~/.config/social-carousel/themes/<name>.yaml  (user custom)
//  3. Embedded preset at templates/themes/<name>.yaml
//
// Returns an error if the theme cannot be found in any location.
// The returned Theme has safe defaults applied for required fields.
func loadTheme(name string) (*Theme, error) {
	if name == "" {
		name = "dark-tech"
	}

	var (
		data []byte
		err  error
	)

	// --- 1. Explicit file path ---
	ext := filepath.Ext(name)
	if filepath.IsAbs(name) || ext == ".yaml" || ext == ".yml" || strings.Contains(name, "/") {
		abs, absErr := filepath.Abs(name)
		if absErr != nil {
			return nil, fmt.Errorf("resolve theme path %q: %w", name, absErr)
		}
		data, err = os.ReadFile(abs)
		if err != nil {
			return nil, fmt.Errorf("read theme %q: %w", abs, err)
		}
		return parseTheme(data, filepath.Base(name))
	}

	// --- 2. User custom theme ---
	customPath, cdErr := customThemePath(name)
	if cdErr == nil {
		if _, statErr := os.Stat(customPath); statErr == nil {
			data, err = os.ReadFile(customPath)
			if err != nil {
				return nil, fmt.Errorf("read custom theme %q: %w", customPath, err)
			}
			return parseTheme(data, name)
		}
	}

	// --- 3. Embedded preset ---
	embPath := embeddedThemePrefix + "/" + name + ".yaml"
	data, err = templatesFS.ReadFile(embPath)
	if err != nil {
		return nil, fmt.Errorf("theme %q not found (checked custom dir and embedded presets): %w", name, err)
	}
	return parseTheme(data, name)
}

// listThemes returns the names of (presets, custom) themes available.
// Either slice may be empty if no themes are found in that category.
// The function never returns an error for a missing custom dir — it just
// returns an empty custom slice in that case.
func listThemes() (presets []string, custom []string, err error) {
	// --- presets from embed ---
	entries, readErr := templatesFS.ReadDir(embeddedThemePrefix)
	if readErr != nil {
		// If the directory isn't embedded yet (e.g. build without templates),
		// return empty rather than crashing.
		presets = []string{}
	} else {
		for _, e := range entries {
			n := e.Name()
			if !e.IsDir() && strings.HasSuffix(n, ".yaml") {
				presets = append(presets, strings.TrimSuffix(n, ".yaml"))
			}
		}
	}

	// --- custom from ~/.config/social-carousel/themes/ ---
	customDir, cdErr := customThemeDir()
	if cdErr != nil {
		return presets, nil, nil //nolint:nilerr // home dir unavailable — soft fail
	}
	dirEntries, rdErr := os.ReadDir(customDir)
	if rdErr != nil {
		// Directory simply doesn't exist yet — that's fine.
		return presets, nil, nil
	}
	for _, e := range dirEntries {
		n := e.Name()
		if !e.IsDir() && strings.HasSuffix(n, ".yaml") {
			custom = append(custom, strings.TrimSuffix(n, ".yaml"))
		}
	}
	return presets, custom, nil
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

// customThemeDir returns the directory where user themes are stored.
func customThemeDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "social-carousel", "themes"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", themeDir), nil
}

// customThemePath returns the full path for a named custom theme YAML.
func customThemePath(name string) (string, error) {
	dir, err := customThemeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".yaml"), nil
}

// parseTheme unmarshals raw YAML bytes into a Theme, applying safe
// defaults for required fields that were left blank.
//
// name is used as a fallback when the YAML's `name` field is empty.
func parseTheme(data []byte, name string) (*Theme, error) {
	var t Theme
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse theme %q: %w", name, err)
	}
	if t.Name == "" {
		t.Name = strings.TrimSuffix(name, ".yaml")
	}
	// Apply safe defaults so templates never receive empty required CSS vars.
	if t.BgPrimary == "" {
		t.BgPrimary = "#0D1117"
	}
	if t.FgPrimary == "" {
		t.FgPrimary = "#F0F6FC"
	}
	if t.Accent == "" {
		t.Accent = "#11C47E"
	}
	if t.FontHeading == "" {
		t.FontHeading = "Outfit"
	}
	if t.FontBody == "" {
		t.FontBody = "DM Sans"
	}
	return &t, nil
}
