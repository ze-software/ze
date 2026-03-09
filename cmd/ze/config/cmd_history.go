// Design: docs/architecture/config/syntax.md — config history command
// Overview: main.go — dispatch and exit codes

package config

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/config/editor"
)

func cmdHistory(args []string) int {
	fs := flag.NewFlagSet("config history", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config history <file>

List rollback revisions for a configuration file.
Revisions are stored in the rollback/ subdirectory alongside the config file.

Exit codes:
  0  Success
  2  File not found or error
`)
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires a config file\n")
		fs.Usage()
		return exitError
	}

	ed, err := editor.NewEditor(fs.Arg(0))
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

	if len(backups) == 0 {
		fmt.Println("No rollback revisions found")
		return exitOK
	}

	for i, b := range backups {
		fmt.Printf("%d  %s  %s\n", i+1, b.Timestamp.Format("2006-01-02 15:04:05"), b.Path)
	}

	return exitOK
}
