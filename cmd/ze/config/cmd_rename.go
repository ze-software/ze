// Design: docs/architecture/config/syntax.md -- config rename command
// Overview: main.go -- dispatch and exit codes

package config

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// cmdRenameWithStorage renames a config entry in blob storage.
func cmdRenameWithStorage(store storage.Storage, args []string) int {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: ze config rename <old-name> <new-name>\n")
		return 1
	}

	old, new := args[0], args[1]

	if !store.Exists(old) {
		fmt.Fprintf(os.Stderr, "error: %s not found\n", old)
		return exitError
	}

	if store.Exists(new) {
		fmt.Fprintf(os.Stderr, "error: %s already exists\n", new)
		return exitError
	}

	if err := store.Rename(old, new); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	fmt.Printf("renamed %s -> %s\n", old, new)
	return exitOK
}
