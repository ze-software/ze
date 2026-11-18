// Design: docs/architecture/api/commands.md -- stored configuration rename.
// Overview: main.go -- dispatch and exit codes

package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/storage/daemonroute"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/resolve"
)

// cmdRenameWithStorage renames the complete stored configuration identity.
func cmdRenameWithStorage(store storage.Storage, args []string) int {
	return cmdRenameThrough(store, args, daemonroute.OverSSH)
}

func cmdRenameThrough(store storage.Storage, args []string, ask daemonroute.Ask) int {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: ze config rename <old-name> <new-name>\n")
		return 1
	}
	if store == nil {
		var err error
		store, err = resolve.StorageFor("")
		if errors.Is(err, storage.ErrBusy) {
			if err := renameConfigThroughDaemon(resolve.StoreDir(""), args[0], args[1], err, ask); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return exitError
			}
			fmt.Printf("renamed %s -> %s\n", args[0], args[1])
			return exitOK
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: config rename: %v\n", err)
			return exitError
		}
		defer store.Close() //nolint:errcheck // Offline ownership.
	}

	old, new := args[0], args[1]

	if err := storage.RenameConfig(store, old, new); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", opererr.Wrap(err,
			"ze config rename: "+old+" could not be renamed to "+new,
			"Inspect the stored identities before retrying; no complete rename was confirmed.", "ze show config list"))
		return exitError
	}

	fmt.Printf("renamed %s -> %s\n", old, new)
	return exitOK
}
