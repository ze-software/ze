// Design: docs/architecture/config/syntax.md -- config import command
// Overview: main.go -- dispatch and exit codes

package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// cmdImportWithStorage imports config files from the filesystem into blob storage.
func cmdImportWithStorage(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config import", flag.ContinueOnError)
	name := fs.String("name", "", "store under this name instead of the filename")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config import [--name <name>] <file>...

Import config files from the filesystem into the database.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  ze config import router.conf
  ze config import --name production.conf /etc/ze/router.conf
  ze config import site-a.conf site-b.conf
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	files := fs.Args()
	if len(files) == 0 {
		fs.Usage()
		return 1
	}

	if !storage.IsBlobStorage(store) {
		fmt.Fprintf(os.Stderr, "error: import requires blob storage (run 'ze init' first)\n")
		return exitError
	}

	if *name != "" && len(files) > 1 {
		fmt.Fprintf(os.Stderr, "error: --name can only be used with a single file\n")
		return 1
	}

	imported := 0
	for _, path := range files {
		data, err := os.ReadFile(path) //nolint:gosec // user-provided config path
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: read %s: %v\n", path, err)
			continue
		}

		key := filepath.Base(path)
		if *name != "" {
			key = *name
		}

		if err := store.WriteFile(key, data, 0); err != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", key, err)
			continue
		}

		fmt.Printf("imported %s (%d bytes)\n", key, len(data))
		imported++
	}

	if imported == 0 {
		return exitError
	}

	fmt.Printf("%d file(s) imported\n", imported)
	return exitOK
}
