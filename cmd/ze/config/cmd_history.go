// Design: docs/architecture/config/syntax.md — config history command
// Overview: main.go — dispatch and exit codes

package config

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

func cmdHistoryWithStorage(store storage.Storage, args []string) int {
	return cmdHistoryImpl(store, args)
}

func cmdHistory(args []string) int {
	return cmdHistoryImpl(storage.NewFilesystem(), args)
}

func cmdHistoryImpl(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config history", flag.ExitOnError)
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config history",
			Summary: "List rollback revisions for a configuration file",
			Usage:   []string{"ze config history <file>"},
			Sections: []helpfmt.HelpSection{
				{Title: "Description", Entries: []helpfmt.HelpEntry{
					{Name: "", Desc: "Revisions are stored in the rollback/ subdirectory alongside the config file."},
				}},
				{Title: "Exit codes", Entries: []helpfmt.HelpEntry{
					{Name: "0", Desc: "Success"},
					{Name: "2", Desc: "File not found or error"},
				}},
			},
		}
		p.Write()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires a config file\n")
		fs.Usage()
		return exitError
	}

	ed, err := cli.NewEditorWithStorage(store, fs.Arg(0))
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
