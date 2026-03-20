// Design: docs/architecture/config/syntax.md — config ls/cat commands

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// cmdLsWithStorage lists config files from both blob storage and filesystem.
func cmdLsWithStorage(store storage.Storage, _ []string) int {
	found := false

	// List from blob storage
	if storage.IsBlobStorage(store) {
		for _, prefix := range []string{"file/active", "file/draft"} {
			keys, err := store.List(prefix)
			if err != nil {
				continue // directory doesn't exist yet
			}
			for _, key := range keys {
				fmt.Println("[db] " + key)
				found = true
			}
		}
	}

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

// cmdCatWithStorage prints the content of a key from the blob store.
func cmdCatWithStorage(store storage.Storage, args []string) int {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: ze config cat <key>\n")
		return 1
	}

	data, err := store.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	os.Stdout.Write(data) //nolint:errcheck // stdout write
	return exitOK
}
