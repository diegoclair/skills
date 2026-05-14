package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/diegoclair/skills/pkg/atlassian/jira"
	"github.com/diegoclair/skills/pkg/atlassian/setup"
)

// parseCommonFlags consumes the cross-command flags (--cloud / --email /
// --token) before the per-command flag loop, mirroring the same pattern
// confluence-docs uses. Returns the remaining args and the resolved values.
func parseCommonFlags(args []string) (remaining []string, cloud, email, token string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--cloud":
			if i+1 >= len(args) {
				return nil, "", "", "", fmt.Errorf("flag --cloud requires a value")
			}
			cloud = args[i+1]
			i++
		case "--email":
			if i+1 >= len(args) {
				return nil, "", "", "", fmt.Errorf("flag --email requires a value")
			}
			email = args[i+1]
			i++
		case "--token":
			if i+1 >= len(args) {
				return nil, "", "", "", fmt.Errorf("flag --token requires a value")
			}
			token = args[i+1]
			i++
		default:
			remaining = append(remaining, a)
		}
	}
	return remaining, cloud, email, token, nil
}

// buildClient resolves cloud + credentials from the flags-then-env-then-file
// chain and returns a Jira client. Returns (nil, false) on failure, after
// printing a helpful error to stderr.
//
// Resolution order for each value:
//   1. Explicit flag (--cloud / --email / --token)
//   2. Environment variable (ATLASSIAN_CLOUD / ATLASSIAN_EMAIL / ATLASSIAN_API_TOKEN)
//   3. On-disk credentials/config (~/.config/atlassian/credentials + per-skill config)
func buildClient(cloud, email, token string, stderr io.Writer) (*jira.Client, bool) {
	resolvedCloud := resolveCloud(cloud)
	if resolvedCloud == "" {
		fmt.Fprintln(stderr, "no Atlassian cloud subdomain configured — run `jira-tickets setup`, pass --cloud, or set ATLASSIAN_CLOUD.")
		return nil, false
	}

	resolvedEmail, resolvedToken, err := resolveCreds(email, token, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, false
	}

	client, err := jira.NewClient(resolvedCloud, resolvedEmail, resolvedToken)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, false
	}
	return client, true
}

// resolveCloud picks the cloud subdomain from (in order) explicit flag,
// ATLASSIAN_CLOUD env var, or the per-skill config file written by `setup`.
func resolveCloud(flag string) string {
	if flag != "" {
		return flag
	}
	if env := os.Getenv("ATLASSIAN_CLOUD"); env != "" {
		return env
	}
	cfg := setup.ReadConfigFile()
	return cfg.Cloud
}

// resolveCreds picks (email, token) from (in order) explicit flags, env vars,
// or the shared ~/.config/atlassian/credentials file (with legacy fallback
// chain handled inside pkg/atlassian/setup).
func resolveCreds(email, token string, stderr io.Writer) (string, string, error) {
	if email != "" && token != "" {
		return email, token, nil
	}
	envEmail := os.Getenv("ATLASSIAN_EMAIL")
	envToken := os.Getenv("ATLASSIAN_API_TOKEN")
	if envEmail != "" && envToken != "" {
		return envEmail, envToken, nil
	}

	fileEmail, fileToken, err := setup.ReadCredsFile(stderr)
	if err != nil {
		return "", "", fmt.Errorf("no Atlassian credentials found — run `jira-tickets setup`, pass --email + --token, or set ATLASSIAN_EMAIL + ATLASSIAN_API_TOKEN. (read error: %w)", err)
	}
	if email == "" {
		email = fileEmail
	}
	if token == "" {
		token = fileToken
	}
	if email == "" || token == "" {
		return "", "", fmt.Errorf("Atlassian email or token still missing after consulting flags, env, and file")
	}
	return email, token, nil
}

// parseStringList splits a comma-separated list, trimming whitespace and
// dropping empty entries. Used for --fields / --labels / etc.
func parseStringList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// issueWebURL builds the Jira UI URL for an issue, e.g.
// https://mycompany.atlassian.net/browse/PROJ-123. Falls back to the API
// base URL when something is missing.
func issueWebURL(client *jira.Client, key string) string {
	if client == nil || key == "" {
		return ""
	}
	return fmt.Sprintf("https://%s.atlassian.net/browse/%s", client.Cloud, key)
}
