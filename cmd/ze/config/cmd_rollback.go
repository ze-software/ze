// Design: docs/architecture/config/syntax.md — config rollback command
// Overview: main.go — dispatch and exit codes

package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
)

func cmdRollback(args []string) int {
	fs := flag.NewFlagSet("config rollback", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config rollback <N> <file>

Restore a configuration file from rollback revision N.
Use 'ze config history <file>' to list available revisions.

Exit codes:
  0  Success
  2  Error (file not found, invalid revision, etc.)
`)
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "error: requires revision number and config file\n")
		fs.Usage()
		return exitError
	}

	n, err := strconv.Atoi(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid revision number: %s\n", fs.Arg(0))
		return exitError
	}

	ed, err := cli.NewEditor(fs.Arg(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	defer ed.Close() //nolint:errcheck // best effort cleanup

	backups, err := ed.ListBackups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if n < 1 || n > len(backups) {
		fmt.Fprintf(os.Stderr, "error: revision %d not found (have %d revisions)\n", n, len(backups))
		return exitError
	}

	if err := ed.Rollback(backups[n-1].Path); err != nil {
		fmt.Fprintf(os.Stderr, "error: rollback failed: %v\n", err)
		return exitError
	}

	fmt.Printf("Rolled back to revision %d (%s)\n", n, backups[n-1].Timestamp.Format("2006-01-02 15:04:05"))
	return exitOK
}
