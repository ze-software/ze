// Design: docs/architecture/config/syntax.md — config list/cat commands

package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/pkg/zefs"
)

// cmdListWithStorage lists stored configurations and explicit loose files.
//
// `ze config list --backup <artifact>` lists the configs inside a backup
// artifact instead, and only those: the loose files beside the operator have
// nothing to do with what the artifact holds.
func cmdListWithStorage(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config list", flag.ContinueOnError)
	backupPath := fs.String(flagBackup, "", "List the configs inside a backup artifact")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: ze config list [--backup <artifact>]")
		return exitError
	}
	if *backupPath != "" {
		backup, err := openBackup(*backupPath, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: config list: %v\n", err)
			return exitError
		}
		defer backup.Close() //nolint:errcheck // Read-only inspection.
		if !listStored(backup) {
			fmt.Fprintf(os.Stderr, "No config found in %s.\n", *backupPath)
		}
		return exitOK
	}
	if store == nil {
		var err error
		store, err = storage.OpenReadOnly(resolve.StoreDir(""))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: config list: %v\n", err)
			return exitError
		}
		defer store.Close() //nolint:errcheck // Read-only inspection.
	}
	found := listStored(store)

	// List .conf files from filesystem (XDG config home + cwd)
	for _, dir := range configSearchDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
				continue
			}
			fmt.Println("[fs] " + filepath.Join(dir, e.Name()))
			found = true
		}
	}

	if !found {
		fmt.Fprintln(os.Stderr, "No config files found. Use 'ze config edit' to create one.")
	}
	return exitOK
}

// listStored prints every active and draft config the store holds, one
// "[data] <key>" line each, and reports whether it printed any.
func listStored(store storage.Storage) bool {
	found := false
	for _, prefix := range []string{zefs.KeyFileActive.Dir(), zefs.KeyFileDraft.Dir()} {
		keys, err := store.List(prefix)
		if err != nil {
			continue // directory doesn't exist yet
		}
		for _, key := range keys {
			fmt.Println("[data] " + key)
			found = true
		}
	}
	return found
}

// configSearchDirs returns directories to scan for .conf files.
func configSearchDirs() []string {
	var dirs []string

	// XDG config home (~/.config/ze/)
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		if home := os.Getenv("HOME"); home != "" {
			configHome = filepath.Join(home, ".config")
		}
	}
	if configHome != "" {
		dirs = append(dirs, filepath.Join(configHome, "ze"))
	}

	// Current directory
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, cwd)
	}

	return dirs
}

// cmdCatWithStorage prints the content of a stored key.
func cmdCatWithStorage(store storage.Storage, args []string) int {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: ze config cat <key>\n")
		return 1
	}
	if store == nil {
		var err error
		store, err = storage.OpenReadOnly(resolve.StoreDir(""))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: config cat: %v\n", err)
			return exitError
		}
		defer store.Close() //nolint:errcheck // Read-only inspection.
	}

	data, err := store.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	os.Stdout.Write(data) //nolint:errcheck // stdout write
	return exitOK
}
