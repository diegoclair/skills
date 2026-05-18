package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/diegoclair/skills/pkg/release"
)

// runUpdate self-updates the binary by re-running the install script.
// Mirrors confluence-docs and jira-tickets behavior, but filtered by
// the "carousel-v*" tag prefix so sibling-skill releases are ignored.
func runUpdate(args []string, stdout, stderr io.Writer) (int, error) {
	const (
		repoOwnerRepo = "diegoclair/skills"
		tagPrefix     = "carousel-v"
		installShURL  = "https://raw.githubusercontent.com/diegoclair/skills/main/social-carousel/install/install.sh"
		installPS1URL = "https://raw.githubusercontent.com/diegoclair/skills/main/social-carousel/install/install.ps1"
	)

	var checkOnly bool
	for _, a := range args {
		switch a {
		case "--check":
			checkOnly = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "update — fetch the latest release of social-carousel.")
			fmt.Fprintln(stdout, "")
			fmt.Fprintln(stdout, "  social-carousel update          # install latest release")
			fmt.Fprintln(stdout, "  social-carousel update --check  # report if newer version exists")
			return exitOK, nil
		default:
			fmt.Fprintln(stderr, "unknown flag:", a)
			return exitInputErr, errInvalidUsage
		}
	}

	latest, err := release.FindLatestByPrefix(repoOwnerRepo, tagPrefix, nil)
	if err != nil {
		fmt.Fprintln(stderr, "could not resolve latest version:", err)
		return exitUnknownErr, err
	}
	current := version
	if release.NormalizeVersion(current) == release.NormalizeVersion(latest) {
		fmt.Fprintf(stdout, "social-carousel is up to date (%s).\n", current)
		return exitOK, nil
	}
	if checkOnly {
		fmt.Fprintf(stdout, "current: %s\nlatest:  %s\nrun: social-carousel update\n", current, latest)
		return exitUpdateAvailable, nil
	}
	fmt.Fprintf(stdout, "Updating social-carousel: %s → %s ...\n", current, latest)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
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
