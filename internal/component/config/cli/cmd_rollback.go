// Design: docs/architecture/api/commands.md -- indexed config rollback.
// Overview: main.go — dispatch and exit codes

package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/storage/daemonroute"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/resolve"
)

func cmdRollbackWithStorage(store storage.Storage, args []string) int {
	return cmdRollbackImpl(store, args)
}

func cmdRollback(args []string) int {
	return cmdRollbackImpl(nil, args)
}

func cmdRollbackImpl(store storage.Storage, args []string) int {
	return cmdRollbackThrough(store, args, daemonroute.OverSSH)
}

func cmdRollbackThrough(store storage.Storage, args []string, ask daemonroute.Ask) int {
	fs := flag.NewFlagSet("config rollback", flag.ExitOnError)
	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze config rollback",
			ShortHelp: "Restore a configuration file from rollback revision N",
			Usage:     []string{"ze config rollback <N> <file>"},
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

	if fs.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "error: requires exactly a revision number and config file\n")
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

	if store != nil {
		return rollbackStoredConfig(store, fs.Arg(1), n)
	}
	store, err = resolve.StorageFor(fs.Arg(1))
	if errors.Is(err, storage.ErrBusy) {
		selected, routeErr := rollbackConfigThroughDaemon(resolve.StoreDir(fs.Arg(1)), fs.Arg(1), n, err, ask)
		if routeErr != nil {
			fmt.Fprintln(os.Stderr, "error:", rollbackRouteRefusal(routeErr, fs.Arg(1)))
			return exitError
		}
		return printConfigRollback(n, selected)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: config rollback: %v\n", err)
		return exitError
	}
	defer store.Close() //nolint:errcheck // Offline ownership.
	ed, err := cli.NewLooseFileEditor(store, fs.Arg(1))
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

	// The owner lock taken above refuses a running daemon's store, so a
	// window record here was left by a daemon that stopped during a window.
	// Its next start reverts that window, which would wipe this write, so the
	// write is refused and the refusal says to start the daemon.
	err = confirm.WriteOutsideRecorded(store, fs.Arg(1), func() error {
		return ed.Rollback(backups[n-1].Path)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: rollback failed: %v\n", err)
		return exitError
	}

	return printConfigRollback(n, backups[n-1].Timestamp)
}

// rollbackStoredConfig leaves selection and publication inside one owner
// transaction. Selecting first could let concurrent history change the index.
func rollbackStoredConfig(store storage.Storage, name string, revision int) int {
	var selected storage.VersionInfo
	err := confirm.WriteOutsideRecorded(store, name, func() error {
		var rollbackErr error
		selected, _, rollbackErr = storage.RollbackConfig(store, name, revision)
		return rollbackErr
	})
	if err != nil {
		// Keep the recorded window's boot recovery command on the final line.
		if stopped, recorded := err.(*confirm.StoppedWindowError); recorded { //nolint:errorlint // WriteOutsideRecorded returns this refusal directly, before apply.
			var refusal *opererr.Error = stopped.OperatorError() //nolint:modernize // inferdecl: Pin the complete stopped-window refusal type at the stderr boundary.
			fmt.Fprintln(os.Stderr, "error:", refusal)
			return exitError
		}
		fmt.Fprintln(os.Stderr, "error:", opererr.Wrap(err,
			"ze config rollback: the stored rollback could not finish",
			"The requested publication was not confirmed. Inspect its history before retrying.",
			configRollbackHistoryLine(name)))
		return exitError
	}
	return printConfigRollback(revision, selected.Date)
}

func printConfigRollback(revision int, selected time.Time) int {
	fmt.Printf("Rolled back to revision %d (%s)\n", revision, selected.Format("2006-01-02 15:04:05"))
	return exitOK
}
