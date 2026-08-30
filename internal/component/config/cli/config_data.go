// Design: docs/architecture/api/commands.md — where a command is served
// Related: docs/architecture/api/commands.md — the local-data path this serves
//
// config_data.go answers the config commands that HAVE an answer with
// structured data, so they reach the pipe layer. They printed text or JSON and
// returned an exit code, while YANG declared a wire method for some of them
// that no daemon handler implements.
//
// Each handler here is the payload half of the `ze config <sub>` command that
// prints for a reader, so one resolution feeds both spellings and they cannot
// drift. The rendering flag each of those commands carried is DELETED: `| json`,
// `| yaml` and `| table` render one payload (ai/rules/cli.md).
//
// FOUR config answers are deliberately NOT here, and that is an answer rather
// than a gap:
//
//   - `show config cat` returns the configuration TEXT of one snapshot. The
//     text is the answer.
//   - `show config fmt` returns the config pretty-printed. The FORMATTING is
//     the answer, and a record of a formatting is a record of nothing.
//   - `ze config migrate` returns a configuration in the set or the
//     hierarchical form. Both are configuration TEXT, for the same reason, and
//     the form is grammar the operator types (parseOutputForm, cmd_migrate.go).
//   - `ze config completion --ghost` returns one line of ghost text. One string
//     is not a record, and the completion CANDIDATES, which are, answer through
//     `show config completion`.

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
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/pkg/zefs"
)

// Row keys of the answers below. register.go declares the column order with the
// same names, so a rename here that misses that file drops a column.
const (
	keyPath        = "path"
	keyRevision    = "revision"
	keySource      = "source"
	keyType        = "type"
	keyText        = "text"
	keyDescription = "description"
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
					rows = append(rows, map[string]any{keySource: "data", keyPath: key})
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
					keySource: "fs", keyPath: filepath.Join(dir, e.Name()),
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
			rows = append(rows, map[string]any{keyRevision: "draft", "state": "editing in progress"})
		}
		for i, b := range backups {
			rows = append(rows, map[string]any{
				keyRevision: i + 1,
				"timestamp": b.Timestamp.Format("2006-01-02 15:04:05"),
				keyPath:     b.Path,
			})
		}
		return map[string]any{"revisions": rows}, 0
	})
}

// dataValidate answers `validate config <file>`: whether the configuration is
// usable, and every diagnostic that says why not.
//
// The exit code is the verdict and the payload is the evidence, so a rejected
// configuration answers its diagnostics AND exits 1
// (registry.LocalDataHandler).
func dataValidate(args []string) (any, int) {
	if len(args) == 0 {
		helpfmt.WriteError(os.Stderr, false, "missing config file (use - for stdin)")
		return nil, exitError
	}
	if len(args) > 1 {
		helpfmt.WriteError(os.Stderr, false,
			"validate config takes one config file, got %d", len(args))
		return nil, exitError
	}

	data, err := cliio.ReadFile(args[0])
	if err != nil {
		helpfmt.WriteError(os.Stderr, false, "%v", err)
		return nil, exitError
	}

	result := runValidation(string(data), args[0])
	payload := diagnostic.NewValidateResult(result.Path, result.Valid, result.Diagnostics, result.Config)
	if result.Valid {
		return payload, exitOK
	}
	return payload, exitInvalid
}

// dataDiff answers `show config diff <file1> <file2>` and
// `show config diff <N> <file>`: what the two resolved configurations do not
// agree about, with every secret value masked.
//
// It is ONE document holding three keyed sets rather than rows.
func dataDiff(args []string) (any, int) {
	diff, code := resolveDiff(storage.NewFilesystem(), args)
	if diff == nil {
		return nil, code
	}
	return map[string]any{
		"added":   diff.Added,
		"removed": diff.Removed,
		"changed": diff.Changed,
	}, exitOK
}

// dataFix answers `show config fix <file>`: the repair plan a configuration's
// diagnostics imply. It never edits the file.
func dataFix(args []string) (any, int) {
	if len(args) == 0 {
		helpfmt.WriteError(os.Stderr, false, "missing config file (use - for stdin)")
		return nil, exitError
	}
	if len(args) > 1 {
		helpfmt.WriteError(os.Stderr, false, "expected one config file, got %d", len(args))
		return nil, exitError
	}
	return resolveFixPlan(args[0])
}

// dataCompletion answers `show config completion --input <text> [--context
// <path>] <file>`: what the editor would offer next, as ROWS.
//
// The text form pads three columns into a line (printCompletions,
// cmd_completion.go); the row carries the same three as FIELDS, which is what a
// row operator can select on, and under the kebab-case names the rest of the
// CLI answers with rather than the Go field names.
func dataCompletion(args []string) (any, int) {
	request, code := parseCompletionRequest(args, nil)
	if code != exitOK {
		return nil, code
	}
	if request.ghost {
		helpfmt.WriteError(os.Stderr, false,
			"--ghost is not part of `show config completion`: ghost text is one line, "+
				"so ask `ze config completion --ghost` for it")
		return nil, exitError
	}

	completer, code := completerFor(request.configPath)
	if code != exitOK {
		return nil, code
	}

	completions := completer.Complete(request.input, request.context)
	rows := make([]map[string]any, 0, len(completions))
	for _, comp := range completions {
		rows = append(rows, map[string]any{
			keyType: comp.Type, keyText: comp.Text, keyDescription: comp.Description,
		})
	}
	return map[string]any{"completions": rows}, exitOK
}
