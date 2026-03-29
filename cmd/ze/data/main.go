// Design: docs/architecture/zefs-format.md -- ZeFS blob store CLI
//
// Package data provides the ze data subcommand for managing ZeFS blob stores.
package data

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

const defaultBlobName = "database.zefs"

// subcommandHandlers maps subcommand names to their handler functions.
// Each handler receives the blob path and remaining args.
var subcommandHandlers = map[string]func(string, []string) int{
	"import":     cmdImport,
	"rm":         cmdRm,
	"ls":         cmdLs,
	"cat":        cmdCat,
	"registered": cmdRegistered,
}

// Run executes the data subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	storePath, remaining := extractPathFlag(args)

	if len(remaining) == 0 {
		usage()
		return 1
	}

	subcmd := remaining[0]
	subArgs := remaining[1:]

	if subcmd == "help" || subcmd == "-h" || subcmd == "--help" {
		usage()
		return 0
	}

	if handler, ok := subcommandHandlers[subcmd]; ok {
		return handler(storePath, subArgs)
	}

	fmt.Fprintf(os.Stderr, "unknown data subcommand: %s\n", subcmd)
	candidates := make([]string, 0, len(subcommandHandlers))
	for k := range subcommandHandlers {
		candidates = append(candidates, k)
	}
	if s := suggest.Command(subcmd, candidates); s != "" {
		fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
	}
	usage()
	return 1
}

// extractPathFlag parses --path <path> from args and returns the blob path and remaining args.
func extractPathFlag(args []string) (string, []string) {
	storePath := ""
	remaining := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--path" && i+1 < len(args):
			storePath = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--path="):
			storePath = args[i][len("--path="):]
		default:
			remaining = append(remaining, args[i])
		}
	}

	if storePath == "" {
		configDir := paths.DefaultConfigDir()
		if configDir == "" {
			storePath = defaultBlobName
		} else {
			storePath = filepath.Join(configDir, defaultBlobName)
		}
	}

	return storePath, remaining
}

// openStore opens an existing blob store or returns an error.
func openStore(storePath string) (*zefs.BlobStore, error) {
	s, err := zefs.Open(storePath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", storePath, err)
	}
	return s, nil
}

// openOrCreateStore opens an existing store or creates a new one.
func openOrCreateStore(storePath string) (*zefs.BlobStore, error) {
	if _, err := os.Stat(storePath); err == nil {
		return zefs.Open(storePath)
	}
	return zefs.Create(storePath)
}

// filePathToKey converts a filesystem path to a blob key under the file/active/ namespace.
// Only the base filename is used as the key (not the full path).
func filePathToKey(path string) string {
	return zefs.KeyFileActive.Key(filepath.Base(path))
}

func cmdImport(storePath string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: ze data import <file>...\n")
		return 1
	}

	s, err := openOrCreateStore(storePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	wl, err := s.Lock()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: lock %s: %v\n", storePath, err)
		return 2
	}

	imported := 0
	for _, path := range args {
		data, readErr := os.ReadFile(path) //nolint:gosec // user-provided path
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "error: read %s: %v\n", path, readErr)
			continue
		}

		key := filePathToKey(path)

		if writeErr := wl.WriteFile(key, data, 0); writeErr != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", key, writeErr)
			continue
		}

		fmt.Printf("imported %s (%d bytes)\n", key, len(data))
		imported++
	}

	if err := wl.Release(); err != nil {
		fmt.Fprintf(os.Stderr, "error: flush: %v\n", err)
		return 2
	}

	fmt.Printf("%d files imported into %s\n", imported, storePath)
	if imported == 0 {
		return 2
	}
	return 0
}

func cmdRm(storePath string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: ze data rm <key>...\n")
		return 1
	}

	s, err := openStore(storePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	wl, err := s.Lock()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: lock %s: %v\n", storePath, err)
		return 2
	}

	removed := 0
	for _, key := range args {
		if rmErr := wl.Remove(key); rmErr != nil {
			fmt.Fprintf(os.Stderr, "error: remove %s: %v\n", key, rmErr)
			continue
		}
		fmt.Printf("removed %s\n", key)
		removed++
	}

	if err := wl.Release(); err != nil {
		fmt.Fprintf(os.Stderr, "error: flush: %v\n", err)
		return 2
	}

	fmt.Printf("%d entries removed from %s\n", removed, storePath)
	if removed == 0 {
		return 2
	}
	return 0
}

func cmdLs(storePath string, args []string) int {
	s, err := openStore(storePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	prefix := ""
	if len(args) > 0 {
		prefix = args[0]
	}

	keys := s.List(prefix)
	for _, key := range keys {
		fmt.Println(key)
	}
	return 0
}

func cmdCat(storePath string, args []string) int {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: ze data cat <key>\n")
		return 1
	}

	s, err := openStore(storePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	data, readErr := s.ReadFile(args[0])
	if readErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", readErr)
		return 2
	}

	os.Stdout.Write(data) //nolint:errcheck // stdout write
	return 0
}

func cmdRegistered(_ string, args []string) int {
	if len(args) > 0 {
		return showRegisteredKey(args[0])
	}
	return listRegisteredKeys()
}

func listRegisteredKeys() int {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	printKeyRow(w, "PATTERN", "DESCRIPTION")
	printKeyRow(w, "-------", "-----------")
	for _, e := range zefs.Entries() {
		printKeyRow(w, e.Pattern, e.Description)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func showRegisteredKey(key string) int {
	for _, e := range zefs.Entries() {
		if e.Pattern != key {
			continue
		}
		fmt.Printf("Pattern:     %s\n", e.Pattern)
		fmt.Printf("Description: %s\n", e.Description)
		return 0
	}
	fmt.Fprintf(os.Stderr, "error: unknown key %q\n", key)
	return 1
}

// printKeyRow writes a tab-separated row to w.
func printKeyRow(w *tabwriter.Writer, cols ...string) {
	if _, err := fmt.Fprintln(w, strings.Join(cols, "\t")); err != nil {
		return
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze data [--path <store>] <command> [args...]

Manage ZeFS blob stores.

Commands:
  import <file>...    Import files into the blob store
  rm <key>...         Remove entries from the blob store
  ls [prefix]         List entries in the blob store
  cat <key>           Print entry content to stdout
  registered          List all registered key patterns

Flags:
  --path <store>      Path to the blob store (default: {configDir}/database.zefs)

Examples:
  ze data import /etc/ze/router.conf /etc/ze/site-b.conf
  ze data ls
  ze data ls file/active/
  ze data ls meta/
  ze data cat file/active/etc/ze/router.conf
  ze data rm file/active/etc/ze/old-router.conf
  ze data registered
  ze data --path /tmp/test.zefs import router.conf
`)
}
