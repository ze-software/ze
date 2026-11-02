// Design: docs/architecture/config/syntax.md — config list/cat commands

package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
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
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "usage: ze config list [--backup <artifact>]")
		return exitError
	}
	if fs.NArg() != 0 {
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
		store, err = storage.OpenReadOnly(resolve.StoreDir(""), resolve.StoreDir(""))
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

// configSearchDirs returns directories to scan for .conf files: the XDG
// config directory and the working directory.
func configSearchDirs() []string {
	var dirs []string
	if dir := xdgConfigDir(); dir != "" {
		dirs = append(dirs, dir)
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, cwd)
	}
	return dirs
}

// xdgConfigDir answers ze's directory under the XDG config home
// (~/.config/ze/), or empty when neither XDG_CONFIG_HOME nor HOME is set.
func xdgConfigDir() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		if home := os.Getenv("HOME"); home != "" {
			configHome = filepath.Join(home, ".config")
		}
	}
	if configHome == "" {
		return ""
	}
	return filepath.Join(configHome, "ze")
}

// cmdCatWithStorage prints the content of a stored key.
func cmdCatWithStorage(store storage.Storage, args []string) int {
	return command.PrintLocalAnswer("show config cat", catAnswer(store, args))
}

// catAnswer answers `show config cat <key>`: the stored file's bytes as a
// Blob, or the refusal in Diagnostic. A nil store opens the default store
// read-only. It writes nothing itself (command.LocalDataHandler).
func catAnswer(store storage.Storage, args []string) command.LocalAnswer {
	if len(args) != 1 {
		return command.LocalRefusal(1, "usage: ze config cat <key>\n")
	}
	if store == nil {
		var err error
		store, err = storage.OpenReadOnly(resolve.StoreDir(""), resolve.StoreDir(""))
		if err != nil {
			var tb textbuf.Buffer
			return command.LocalRefusal(exitError, tb.Str("error: config cat: ").Err(err).Byte('\n').String())
		}
		defer store.Close() //nolint:errcheck // Read-only inspection.
	}

	data, err := store.ReadFile(args[0])
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}
	return command.LocalAnswer{Payload: command.Blob(data), Code: exitOK}
}
