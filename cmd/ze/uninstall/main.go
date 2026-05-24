// Design: plan/spec-install-0-umbrella.md — ze uninstall: remove binary + systemd + config

package uninstall

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
)

const systemdUnitPath = "/etc/systemd/system/ze.service"

func Run(args []string) int {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)

	prefix := fs.String("prefix", "", "Installation prefix to uninstall from")
	purge := fs.Bool("purge", false, "Also remove config directory and database")
	dryRun := fs.Bool("dry-run", false, "Print what would be done without making changes")
	yes := fs.Bool("yes", false, "Skip confirmation prompt")

	fs.Usage = func() { usage() }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	binPath, err := resolveBinPath(*prefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	configDir := paths.ConfigDirFromBinary(binPath)
	if configDir == "" && *prefix == "" {
		fmt.Fprintf(os.Stderr, "warning: %s is not in a standard system prefix, use --prefix to target a specific installation\n", binPath)
	}

	hasUnit := false
	if _, statErr := os.Stat(systemdUnitPath); statErr == nil {
		hasUnit = true
	}

	if *dryRun {
		return dryRunUninstall(binPath, configDir, hasUnit, *purge)
	}

	if !*yes {
		if !confirm(binPath, configDir, hasUnit, *purge) {
			fmt.Fprintf(os.Stderr, "aborted\n")
			return 1
		}
	}

	if hasUnit {
		if err := removeSystemdUnit(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "removed systemd unit\n")
	}

	if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: removing %s: %v\n", binPath, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "removed %s\n", binPath)

	if *purge && configDir != "" {
		if err := os.RemoveAll(configDir); err != nil {
			fmt.Fprintf(os.Stderr, "error: removing %s: %v\n", configDir, err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "removed %s\n", configDir)
	}

	fmt.Fprintf(os.Stderr, "\nuninstall complete\n")
	return 0
}

func resolveBinPath(prefix string) (string, error) {
	if prefix != "" {
		return filepath.Join(prefix, "bin", "ze"), nil
	}

	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find own binary: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(self)
	if err != nil {
		return "", fmt.Errorf("resolving binary path: %w", err)
	}

	return resolved, nil
}

func confirm(binPath, configDir string, hasUnit, purge bool) bool {
	fmt.Fprintf(os.Stderr, "will remove:\n")
	fmt.Fprintf(os.Stderr, "  %s\n", binPath)
	if hasUnit {
		fmt.Fprintf(os.Stderr, "  %s\n", systemdUnitPath)
	}
	if purge && configDir != "" {
		fmt.Fprintf(os.Stderr, "  %s (purge)\n", configDir)
	}
	fmt.Fprintf(os.Stderr, "continue? [y/N]: ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "y" || answer == "yes"
}

func removeSystemdUnit() error {
	for _, args := range [][]string{
		{"stop", "ze"},
		{"disable", "ze"},
	} {
		if err := runSystemctl(args...); err != nil {
			fmt.Fprintf(os.Stderr, "warning: systemctl %s: %v\n", strings.Join(args, " "), err)
		}
	}

	if err := os.Remove(systemdUnitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", systemdUnitPath, err)
	}

	return runSystemctl("daemon-reload")
}

func runSystemctl(args ...string) error {
	cmd := exec.CommandContext(context.Background(), "systemctl", args...) // #nosec G204 - args are hardcoded string literals
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func dryRunUninstall(binPath, configDir string, hasUnit, purge bool) int {
	fmt.Fprintf(os.Stderr, "would remove %s\n", binPath)
	if hasUnit {
		fmt.Fprintf(os.Stderr, "would run: systemctl stop ze\n")
		fmt.Fprintf(os.Stderr, "would run: systemctl disable ze\n")
		fmt.Fprintf(os.Stderr, "would remove %s\n", systemdUnitPath)
		fmt.Fprintf(os.Stderr, "would run: systemctl daemon-reload\n")
	}
	if purge && configDir != "" {
		fmt.Fprintf(os.Stderr, "would remove %s\n", configDir)
	}
	return 0
}

func usage() {
	p := helpfmt.Page{
		Command: "ze uninstall",
		Summary: "Remove ze binary, systemd unit, and optionally config directory",
		Usage:   []string{"ze uninstall [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--prefix <path>", Desc: "Installation prefix (default: detect from running binary)"},
				{Name: "--purge", Desc: "Also remove config directory and database"},
				{Name: "--dry-run", Desc: "Print what would be done without making changes"},
				{Name: "--yes", Desc: "Skip confirmation prompt"},
			}},
		},
		Examples: []string{
			"ze uninstall                       Remove binary and systemd unit",
			"ze uninstall --purge               Also remove config and database",
			"ze uninstall --prefix /opt/ze",
			"ze uninstall --dry-run",
		},
	}
	p.Write()
}
