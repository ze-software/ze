// Design: docs/architecture/config/syntax.md — config rollback command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
)

func cmdRollbackWithStorage(store storage.Storage, args []string) int {
	return cmdRollbackImpl(store, args)
}

func cmdRollback(args []string) int {
	return cmdRollbackImpl(storage.NewFilesystem(), args)
}

func cmdRollbackImpl(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config rollback", flag.ExitOnError)
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config rollback",
			Summary: "Restore a configuration file from rollback revision N",
			Usage:   []string{"ze config rollback <N> <file>"},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionDescription, Entries: []helpfmt.HelpEntry{
					{Name: "", Desc: "Use 'ze config history <file>' to list available revisions."},
				}},
				{Title: helpSectionExitCodes, Entries: []helpfmt.HelpEntry{
					{Name: "0", Desc: helpDescSuccess},
					{Name: "2", Desc: "Error (file not found, invalid revision, etc.)"},
				}},
			},
			SeeAlso: []string{"ze config history"},
		}
		p.WriteErr()
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

	// Rollback restores from the rollback/ revision history keyed by the config
	// file's path; a config read from stdin ("-") has no such history.
	if cliio.IsStdin(fs.Arg(1)) {
		fmt.Fprintf(os.Stderr, "error: rollback needs on-disk revision history; a config read from stdin (\"-\") has none\n")
		return exitError
	}

	ed, err := cli.NewEditorWithStorage(store, fs.Arg(1))
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
