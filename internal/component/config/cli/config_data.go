// Design: docs/architecture/api/commands.md — where a command is served
// Related: plan/spec-cli-pipe-operator-coverage.md — AC-10
//
// config_data.go answers `show config dump`, `history` and `ls` with structured
// data, so they reach the pipe layer. They printed text and returned an exit
// code, while YANG declared a wire method for each that no daemon handler
// implements.
//
// THREE of the six config commands are deliberately NOT here, and that is an
// answer rather than a gap:
//
//   - `show config cat` returns the configuration TEXT of one snapshot. The
//     text is the answer.
//   - `show config fmt` returns the config pretty-printed. The FORMATTING is
//     the answer, and a record of a formatting is a record of nothing.
//   - `show config diff` returns a rendered diff. A structured diff, one record
//     per change, would genuinely serve a tool, but nothing in the tree emits
//     one, so it would be designed here rather than lifted. That makes it a
//     feature with its own spec rather than a conversion.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/pkg/zefs"
)

// withRuntimeStore runs fn against the process's config storage.
//
// It is storageShortcut's half that acquires the store, without the half that
// dispatches a subcommand by name, so a data handler reaches the same storage
// the printing handler does.
func withRuntimeStore(fn func(storage.Storage) (any, int)) (any, int) {
	store, ok := registry.RuntimeStorage().(storage.Storage)
	if !ok {
		fmt.Fprintln(os.Stderr, "error: config storage unavailable")
		return nil, 1
	}
	defer func() {
		if err := store.Close(); err != nil {
			_ = err // best-effort cleanup before exit
		}
	}()
	return fn(store)
}

// dataDump answers `show config dump <file>`: the fully resolved configuration.
//
// It is ONE nested document rather than rows, and it declares that shape, so a
// row operator is refused over it by name instead of answering something
// plausible. The payload is the map `--json` emits, from the same
// resolveDump call, so the two spellings cannot disagree.
func dataDump(args []string) (any, int) {
	stripPrivate := false
	var configPath string
	for _, arg := range args {
		if arg == "--strip-private" {
			stripPrivate = true
			continue
		}
		if !strings.HasPrefix(arg, "-") && configPath == "" {
			configPath = arg
		}
	}
	if configPath == "" {
		fmt.Fprintln(os.Stderr, "error: missing config file (use - for stdin)")
		return nil, 1
	}

	res, code := resolveDump(configPath, stripPrivate)
	if code != 0 {
		return nil, code
	}
	return res.dumpMap, 0
}

// dataLs answers `show config ls`: every configuration the daemon can see, from
// both places it looks.
//
// The printer wrote `[data] <key>` and `[fs] <path>` lines. The bracket prefix
// becomes a FIELD, which is what a row operator can select on: `| match data`
// used to match the prefix by accident of it being in the line.
func dataLs(_ []string) (any, int) {
	return withRuntimeStore(func(store storage.Storage) (any, int) {
		rows := make([]map[string]any, 0)

		if storage.IsBlobStorage(store) {
			for _, prefix := range []string{zefs.KeyFileActive.Dir(), zefs.KeyFileDraft.Dir()} {
				keys, err := store.List(prefix)
				if err != nil {
					continue // the directory does not exist yet
				}
				for _, key := range keys {
					rows = append(rows, map[string]any{"source": "data", "path": key})
				}
			}
		}

		for _, dir := range configSearchDirs() {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
					continue
				}
				rows = append(rows, map[string]any{
					"source": "fs", "path": filepath.Join(dir, e.Name()),
				})
			}
		}
		return map[string]any{"configs": rows}, 0
	})
}

// dataHistory answers `show config history <file>`: the rollback revisions
// stored beside a configuration, and the draft if one is open.
//
// The draft is a ROW like any other rather than a line printed above the table,
// so a caller sees one list and can select on it. The printer wrote
// `draft  (editing in progress)` before the numbered revisions, which no row
// operator could reach.
func dataHistory(args []string) (any, int) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: requires a config file")
		return nil, exitError
	}
	if cliio.IsStdin(args[0]) {
		fmt.Fprintln(os.Stderr,
			`error: history needs on-disk revision history; a config read from stdin ("-") has none`)
		return nil, exitError
	}

	return withRuntimeStore(func(store storage.Storage) (any, int) {
		ed, err := cli.NewEditorWithStorage(store, args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return nil, exitError
		}
		defer ed.Close() //nolint:errcheck // best effort cleanup

		backups, err := ed.ListBackups()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return nil, exitError
		}

		rows := make([]map[string]any, 0, len(backups)+1)
		if ed.HasDraft() {
			rows = append(rows, map[string]any{"revision": "draft", "state": "editing in progress"})
		}
		for i, b := range backups {
			rows = append(rows, map[string]any{
				"revision":  i + 1,
				"timestamp": b.Timestamp.Format("2006-01-02 15:04:05"),
				"path":      b.Path,
			})
		}
		return map[string]any{"revisions": rows}, 0
	})
}
