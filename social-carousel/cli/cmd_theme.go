package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// runTheme handles: theme list | theme show <name> | theme create --from FILE
//
// Themes are the customisation surface of the skill. Five presets ship
// embedded; users can author their own and drop them in
// ~/.config/social-carousel/themes/.
func runTheme(args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "theme — usage: theme list | theme show <name> | theme create --from <yaml> [--name N]")
		return exitInputErr, errInvalidUsage
	}
	switch args[0] {
	case "list":
		return themeList(stdout, stderr)
	case "show":
		return themeShow(args[1:], stdout, stderr)
	case "create":
		return themeCreate(args[1:], stdout, stderr)
	case "-h", "--help":
		fmt.Fprintln(stdout, "theme — manage carousel themes.")
		fmt.Fprintln(stdout, "  theme list                              List presets + custom themes.")
		fmt.Fprintln(stdout, "  theme show <name>                       Print the theme YAML.")
		fmt.Fprintln(stdout, "  theme create --from <yaml> [--name N]   Save the theme block from a carousel YAML as a new custom theme.")
		return exitOK, nil
	}
	fmt.Fprintln(stderr, "unknown verb:", args[0])
	return exitInputErr, errInvalidUsage
}

func themeList(stdout, stderr io.Writer) (int, error) {
	presets, custom, err := listThemes()
	if err != nil {
		return exitUnknownErr, err
	}
	fmt.Fprintln(stdout, "PRESETS:")
	for _, n := range presets {
		fmt.Fprintln(stdout, "  ", n)
	}
	if len(custom) == 0 {
		fmt.Fprintln(stdout, "\nCUSTOM:  (none — author your own with `theme create`)")
	} else {
		fmt.Fprintln(stdout, "\nCUSTOM:")
		for _, n := range custom {
			fmt.Fprintln(stdout, "  ", n)
		}
	}
	return exitOK, nil
}

func themeShow(args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "theme show: missing name")
		return exitInputErr, errInvalidUsage
	}
	name := args[0]
	t, err := loadTheme(name)
	if err != nil {
		fmt.Fprintln(stderr, "load theme:", err)
		return exitUnknownErr, err
	}
	out, err := yaml.Marshal(t)
	if err != nil {
		return exitUnknownErr, err
	}
	fmt.Fprint(stdout, string(out))
	return exitOK, nil
}

// themeCreate extracts a theme block from a carousel YAML the user
// already iterated and approved, then saves it as a reusable preset.
// The theme is identified by the `theme` field of the source YAML if
// that field is an inline mapping (not a string reference). If the
// source uses a preset name, --name is required and the preset is
// copied to ~/.config/social-carousel/themes/<name>.yaml.
func themeCreate(args []string, stdout, stderr io.Writer) (int, error) {
	var fromPath, name string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--from requires a path")
				return exitInputErr, errInvalidUsage
			}
			fromPath = args[i+1]
			i++
		case "--name":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--name requires a string")
				return exitInputErr, errInvalidUsage
			}
			name = args[i+1]
			i++
		default:
			fmt.Fprintln(stderr, "unknown flag:", args[i])
			return exitInputErr, errInvalidUsage
		}
	}
	if fromPath == "" {
		fmt.Fprintln(stderr, "theme create: --from is required")
		return exitInputErr, errInvalidUsage
	}
	data, err := os.ReadFile(fromPath)
	if err != nil {
		fmt.Fprintln(stderr, "read --from:", err)
		return exitUnknownErr, err
	}
	// Parse just enough to recover the theme.
	var raw struct {
		Theme any `yaml:"theme"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		fmt.Fprintln(stderr, "parse YAML:", err)
		return exitInputErr, err
	}
	var theme Theme
	switch t := raw.Theme.(type) {
	case string:
		// Reference to a preset. Materialize it.
		loaded, err := loadTheme(t)
		if err != nil {
			return exitUnknownErr, err
		}
		theme = *loaded
		if name == "" {
			name = "copy-of-" + t
		}
	case map[string]any:
		// Inline theme: re-marshal into the typed struct.
		raw2, _ := yaml.Marshal(t)
		if err := yaml.Unmarshal(raw2, &theme); err != nil {
			return exitInputErr, err
		}
		if name == "" {
			if theme.Name != "" {
				name = theme.Name
			} else {
				name = "custom-theme"
			}
		}
	default:
		fmt.Fprintln(stderr, "could not extract theme block from", fromPath)
		return exitInputErr, errInvalidUsage
	}
	theme.Name = name

	dir, err := userThemesDir()
	if err != nil {
		return exitUnknownErr, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return exitUnknownErr, err
	}
	dst := filepath.Join(dir, name+".yaml")
	out, err := yaml.Marshal(theme)
	if err != nil {
		return exitUnknownErr, err
	}
	if err := os.WriteFile(dst, out, 0o644); err != nil {
		fmt.Fprintln(stderr, "write theme:", err)
		return exitUnknownErr, err
	}
	fmt.Fprintln(stdout, "Saved:", dst)
	fmt.Fprintln(stdout, "Use it with: theme:", name)
	return exitOK, nil
}

// userThemesDir returns ~/.config/social-carousel/themes/, honouring XDG.
func userThemesDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "social-carousel", "themes"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "social-carousel", "themes"), nil
}
