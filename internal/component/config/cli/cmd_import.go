// Design: docs/guide/config-editor.md — importing into an explicit destination
package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/resolve"
)

// cmdImportWithStorage imports loose inputs into one independently selected store.
// All destination collisions are refused before any input is published.
func cmdImportWithStorage(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config import", flag.ContinueOnError)
	name := fs.String("name", "", "destination config name (one input only)")
	dir := fs.String("dir", "", "destination store folder (default: ze.config.dir or default folder)")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	files := fs.Args()
	if len(files) == 0 {
		fs.Usage()
		return exitError
	}
	if *name != "" && len(files) != 1 {
		fmt.Fprintln(os.Stderr, "error: --name requires exactly one input")
		return exitError
	}
	if store == nil {
		folder := *dir
		if folder == "" {
			folder = resolve.StoreDir("")
		}
		var err error
		store, err = storage.Open(folder)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: config import destination: %v\n", err)
			return exitError
		}
		defer store.Close() //nolint:errcheck // Offline ownership.
	}
	names := make([]string, len(files))
	seen := make(map[string]string, len(files))
	for i, path := range files {
		key := filepath.Base(path)
		if *name != "" {
			key = *name
		}
		if key == "." || key == ".." || strings.ContainsAny(key, "/\\") || cliio.IsStdin(key) {
			fmt.Fprintf(os.Stderr, "error: invalid destination name %q; use --name for stdin\n", key)
			return exitError
		}
		if previous, ok := seen[key]; ok {
			fmt.Fprintf(os.Stderr, "error: import name %s collides between %s and %s; import separately with --name\n", key, previous, path)
			return exitError
		}
		if store.Exists(key) {
			fmt.Fprintf(os.Stderr, "error: destination config %s already exists\n", key)
			return exitError
		}
		seen[key] = path
		names[i] = key
	}
	contents := make([][]byte, len(files))
	for i, path := range files {
		data, err := cliio.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: read %s: %v\n", path, err)
			return exitError
		}
		contents[i] = data
	}
	for i, key := range names {
		if err := store.WriteFile(key, contents[i], 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "error: import %s: %v\n", key, err)
			return exitError
		}
		fmt.Printf("imported %s (%d bytes)\n", key, len(contents[i]))
	}
	fmt.Printf("%d file(s) imported\n", len(files))
	return exitOK
}
