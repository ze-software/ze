// Design: docs/architecture/zefs-format.md -- ZeFS blob store CLI
//
// Package db provides the ze db subcommand for managing ZeFS blob stores.
package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

const defaultBlobName = "database.zefs"

// subcommandHandlers maps subcommand names to their handler functions.
// Each handler receives the blob path and remaining args.
var subcommandHandlers = map[string]func(string, []string) int{
	"import": cmdImport,
	"rm":     cmdRm,
	"ls":     cmdLs,
	"cat":    cmdCat,
}

// Run executes the db subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	dbPath, remaining := extractDBFlag(args)

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
		return handler(dbPath, subArgs)
	}

	fmt.Fprintf(os.Stderr, "unknown db subcommand: %s\n", subcmd)
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

// extractDBFlag parses --db <path> from args and returns the blob path and remaining args.
func extractDBFlag(args []string) (string, []string) {
	dbPath := ""
	remaining := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--db" && i+1 < len(args):
			dbPath = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--db="):
			dbPath = args[i][len("--db="):]
		default:
			remaining = append(remaining, args[i])
		}
	}

	if dbPath == "" {
		configDir := paths.DefaultConfigDir()
		if configDir == "" {
			dbPath = defaultBlobName
		} else {
			dbPath = filepath.Join(configDir, defaultBlobName)
		}
	}

	return dbPath, remaining
}

// openStore opens an existing blob store or returns an error.
func openStore(dbPath string) (*zefs.BlobStore, error) {
	s, err := zefs.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dbPath, err)
	}
	return s, nil
}

// openOrCreateStore opens an existing store or creates a new one.
func openOrCreateStore(dbPath string) (*zefs.BlobStore, error) {
	if _, err := os.Stat(dbPath); err == nil {
		return zefs.Open(dbPath)
	}
	return zefs.Create(dbPath)
}

// filePathToKey converts a filesystem path to a blob key.
// Resolves to absolute path, then strips the leading slash.
func filePathToKey(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", path, err)
	}
	// Strip leading / to make it a valid fs.ValidPath key
	return strings.TrimPrefix(abs, "/"), nil
}

func cmdImport(dbPath string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: ze db import <file>...\n")
		return 1
	}

	s, err := openOrCreateStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	wl, err := s.Lock()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: lock %s: %v\n", dbPath, err)
		return 2
	}

	imported := 0
	for _, path := range args {
		data, readErr := os.ReadFile(path) //nolint:gosec // user-provided path
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "error: read %s: %v\n", path, readErr)
			continue
		}

		key, keyErr := filePathToKey(path)
		if keyErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", keyErr)
			continue
		}

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

	fmt.Printf("%d files imported into %s\n", imported, dbPath)
	if imported == 0 {
		return 2
	}
	return 0
}

func cmdRm(dbPath string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: ze db rm <key>...\n")
		return 1
	}

	s, err := openStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer s.Close() //nolint:errcheck // best-effort close

	wl, err := s.Lock()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: lock %s: %v\n", dbPath, err)
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

	fmt.Printf("%d entries removed from %s\n", removed, dbPath)
	if removed == 0 {
		return 2
	}
	return 0
}

func cmdLs(dbPath string, args []string) int {
	s, err := openStore(dbPath)
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

func cmdCat(dbPath string, args []string) int {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: ze db cat <key>\n")
		return 1
	}

	s, err := openStore(dbPath)
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

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: ze db [--db path] <command> [args...]

Manage ZeFS blob stores.

Commands:
  import <file>...    Import files into the blob store
  rm <key>...         Remove entries from the blob store
  ls [prefix]         List entries in the blob store
  cat <key>           Print entry content to stdout

Flags:
  --db <path>         Path to the blob store (default: {configDir}/database.zefs)

Examples:
  ze db import /etc/ze/router.conf /etc/ze/site-b.conf
  ze db ls
  ze db ls etc/ze
  ze db cat etc/ze/router.conf
  ze db rm etc/ze/old-router.conf
  ze db --db /tmp/test.zefs import router.conf
`)
}
