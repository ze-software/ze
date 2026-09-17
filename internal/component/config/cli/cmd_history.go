// Design: docs/architecture/config/syntax.md — config history command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/resolve"
)

func cmdHistoryWithStorage(store storage.Storage, args []string) int {
	return cmdHistoryImpl(store, args)
}

func cmdHistory(args []string) int {
	return cmdHistoryImpl(nil, args)
}

func cmdHistoryImpl(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config history", flag.ExitOnError)
	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze config history",
			ShortHelp: "List rollback revisions for a configuration file",
			Usage:     []string{"ze config history <file>"},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionDescription, Entries: []helpfmt.HelpEntry{
					{Name: "", Desc: "Revisions are stored in the rollback/ subdirectory alongside the config file."},
				}},
				{Title: helpSectionExitCodes, Entries: []helpfmt.HelpEntry{
					{Name: "0", Desc: helpDescSuccess},
					{Name: "2", Desc: "File not found or error"},
				}},
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "error: requires exactly one config file\n")
		fs.Usage()
		return exitError
	}

	// History lists the rollback/ revisions stored alongside the config file; a
	// config read from stdin ("-") has no such history.
	if cliio.IsStdin(fs.Arg(0)) {
		fmt.Fprintf(os.Stderr, "error: history needs on-disk revision history; a config read from stdin (\"-\") has none\n")
		return exitError
	}

	if store == nil {
		var err error
		store, err = storage.OpenReadOnly(resolve.StoreDir(fs.Arg(0)))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: config history: %v\n", err)
			return exitError
		}
		defer store.Close() //nolint:errcheck // Read-only inspection.
	}

	backups, err := store.ListVersions(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if len(backups) == 0 && !store.Exists(cli.DraftPath(fs.Arg(0))) {
		fmt.Println("No rollback revisions found")
		return exitOK
	}

	if store.Exists(cli.DraftPath(fs.Arg(0))) {
		fmt.Println("draft  (editing in progress)")
	}
	for i, b := range backups {
		fmt.Printf("%d  %s  %s\n", i+1, b.Date.Format("2006-01-02 15:04:05"), b.Path)
	}

	return exitOK
}
