// Design: docs/architecture/cli/plugin-modes.md — ze local uninstall: remove binary + config

package local

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/paths"
)

func cmdUninstall(args []string) int {
	fs := flag.NewFlagSet("local uninstall", flag.ContinueOnError)

	prefix := fs.String("prefix", "", "Installation prefix to uninstall from")
	purge := fs.Bool("purge", false, "Also remove config directory and database")
	dryRun := fs.Bool("dry-run", false, "Print what would be done without making changes")
	yes := fs.Bool("yes", false, "Skip confirmation prompt")

	fs.Usage = func() { uninstallUsage() }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}

	binPath, err := resolveBinPath(*prefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	configDir := paths.ConfigDirFromBinary(binPath)
	if configDir == "" && *prefix == "" {
		fmt.Fprintf(os.Stderr, "warning: %s is not in a standard system prefix, use --prefix to target a specific installation\n", binPath)
	}

	if *dryRun {
		return dryRunUninstall(binPath, configDir, *purge)
	}

	if !*yes {
		if !confirmUninstall(binPath, configDir, *purge) {
			fmt.Fprintf(os.Stderr, "aborted\n")
			return exitError
		}
	}

	if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: removing %s: %v\n", binPath, err)
		return exitError
	}
	fmt.Fprintf(os.Stderr, "removed %s\n", binPath)

	if *purge && configDir != "" {
		if err := os.RemoveAll(configDir); err != nil {
			fmt.Fprintf(os.Stderr, "error: removing %s: %v\n", configDir, err)
			return exitError
		}
		fmt.Fprintf(os.Stderr, "removed %s\n", configDir)
	}

	fmt.Fprintf(os.Stderr, "\nuninstall complete\n")
	return exitOK
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

func confirmUninstall(binPath, configDir string, purge bool) bool {
	fmt.Fprintf(os.Stderr, "will remove:\n")
	fmt.Fprintf(os.Stderr, "  %s\n", binPath)
	if purge && configDir != "" {
		fmt.Fprintf(os.Stderr, "  %s (purge)\n", configDir)
	}
	fmt.Fprintf(os.Stderr, "continue? [y/N]: ")

	// A failed read is not consent. Scan returns false on EOF, on a read
	// error, and on an over-long line alike, and all three mean no uninstall.
	// A truncated answer is not "y" either.
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "y" || answer == "yes"
}

func dryRunUninstall(binPath, configDir string, purge bool) int {
	fmt.Fprintf(os.Stderr, "would remove %s\n", binPath)
	if purge && configDir != "" {
		fmt.Fprintf(os.Stderr, "would remove %s\n", configDir)
	}
	return exitOK
}

func uninstallUsage() {
	p := helpfmt.Page{
		Command: "ze local uninstall",
		Summary: "Remove ze binary and optionally config directory",
		Usage:   []string{"ze local uninstall [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--prefix <path>", Desc: "Installation prefix (default: detect from running binary)"},
				{Name: "--purge", Desc: "Also remove config directory and database"},
				{Name: "--dry-run", Desc: "Print what would be done without making changes"},
				{Name: "--yes", Desc: "Skip confirmation prompt"},
			}},
		},
		Examples: []string{
			"ze local uninstall                 Remove binary only",
			"ze local uninstall --purge         Also remove config and database",
			"ze local uninstall --prefix /opt/ze",
			"ze local uninstall --dry-run",
		},
	}
	p.WriteErr()
}
