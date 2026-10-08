// Design: docs/guide/config-editor.md — importing into an explicit destination
package cli

import (
	"bufio"
	"flag"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/term"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// cmdImportWithStorage imports loose inputs into one independently selected store.
// A destination that already exists is replaced only once confirmed, by --yes or
// by an answer on the terminal; without either, nothing is written.
func cmdImportWithStorage(store storage.Storage, args []string) int {
	var out textbuf.Buffer
	fs := flag.NewFlagSet("config import", flag.ContinueOnError)
	name := fs.String("name", "", "destination config name (one input only)")
	dir := fs.String("dir", "", "destination store folder (default: ze.config.dir or default folder)")
	yes := fs.Bool("yes", false, "replace existing destination configs without asking")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	files := fs.Args()
	if len(files) == 0 {
		fs.Usage()
		return exitError
	}
	if *name != "" && len(files) != 1 {
		out.Str("error: --name requires exactly one input\n").StdErr() //nolint:errcheck // terminal output
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
			out.Str("error: config import destination: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
			return exitError
		}
		defer store.Close() //nolint:errcheck // Offline ownership.
	}
	names := make([]string, len(files))
	seen := make(map[string]string, len(files))
	var existing []string
	readsStdin := false
	for i, path := range files {
		key := filepath.Base(path)
		if *name != "" {
			key = *name
		}
		if key == "." || key == ".." || strings.ContainsAny(key, "/\\") || cliio.IsStdin(key) {
			out.Str("error: invalid destination name ").Quoted(key).Str("; use --name for stdin\n").StdErr() //nolint:errcheck // terminal output
			return exitError
		}
		if previous, ok := seen[key]; ok {
			out.Str("error: import name ").Str(key).Str(" collides between ").Str(previous).Str(" and ").Str(path).
				Str("; import separately with --name\n").StdErr() //nolint:errcheck // terminal output
			return exitError
		}
		if store.Exists(key) {
			existing = append(existing, key)
		}
		if cliio.IsStdin(path) {
			readsStdin = true
		}
		seen[key] = path
		names[i] = key
	}
	if len(existing) > 0 && !*yes {
		// A config read from stdin leaves no terminal to answer on, so that
		// import is as non-interactive as a script's.
		interactive := !readsStdin && term.IsTerminal(int(os.Stdin.Fd()))
		if !confirmImportReplace(existing, interactive, os.Stdin, os.Stderr) {
			return exitError
		}
	}
	contents := make([][]byte, len(files))
	for i, path := range files {
		data, err := cliio.ReadFile(path)
		if err != nil {
			out.Str("error: read ").Str(path).Str(": ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
			return exitError
		}
		contents[i] = data
	}
	for i, key := range names {
		if err := importOne(store, key, contents[i], slices.Contains(existing, key)); err != nil {
			out.Reset()
			out.Str("error: import ").Str(key).Str(": ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
			return exitError
		}
	}
	out.Reset()
	out.Int(int64(len(files))).Str(" file(s) imported\n").StdOut() //nolint:errcheck // terminal output
	return exitOK
}

// importOne writes one confirmed input. A replaced config is committed as a new
// active version, the previous one kept as rollback, because writing only the
// file/active mirror is shadowed by an existing active pointer: the daemon
// would go on reading the old config while the import reported success.
func importOne(store storage.Storage, key string, data []byte, replace bool) error {
	var out textbuf.Buffer
	if replace {
		if _, err := storage.RestoreConfig(store, key, data); err != nil {
			return err
		}
		out.Str("replaced ")
	} else {
		if err := store.WriteFile(key, data, 0o600); err != nil {
			return err
		}
		out.Str("imported ")
	}
	out.Str(key).Str(" (").Int(int64(len(data))).Str(" bytes)\n").StdOut() //nolint:errcheck // terminal output
	return nil
}

// confirmImportReplace asks whether the existing configs in names may be
// replaced. Without a terminal it asks nothing and refuses, naming --yes, so a
// script never waits on a prompt. On a terminal, y or yes consents; any other
// answer, and a failed or empty read, is a refusal.
func confirmImportReplace(names []string, interactive bool, in io.Reader, errw io.Writer) bool {
	var out textbuf.Buffer
	list := strings.Join(names, ", ")
	if !interactive {
		out.Str("error: destination config ").Str(list).Str(" already exists; rerun with --yes to replace it\n")
		errw.Write(out.Bytes()) //nolint:errcheck,gosec // terminal output
		return false
	}
	out.Str("destination config ").Str(list).Str(" already exists\nreplace it? [y/N] ")
	errw.Write(out.Bytes()) //nolint:errcheck,gosec // terminal output
	scanner := bufio.NewScanner(in)
	answer := ""
	if scanner.Scan() {
		answer = strings.ToLower(strings.TrimSpace(scanner.Text()))
	}
	if answer == "y" || answer == "yes" {
		return true
	}
	out.Reset()
	out.Str("error: destination config ").Str(list).Str(" not replaced\n")
	errw.Write(out.Bytes()) //nolint:errcheck,gosec // terminal output
	return false
}
