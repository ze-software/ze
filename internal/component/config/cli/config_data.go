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
// THREE config answers are deliberately NOT here, and that is an answer rather
// than a gap:
//
//   - `show config cat` returns the configuration TEXT of one snapshot. The
//     text is the answer.
//   - `show config fmt` returns the config pretty-printed. The FORMATTING is
//     the answer, and a record of a formatting is a record of nothing.
//   - `ze config migrate` returns a configuration in the set or the
//     hierarchical form. Both are configuration TEXT, for the same reason, and
//     the form is grammar the operator types (parseOutputForm, cmd_migrate.go).

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/resolve"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// Row keys of the answers below. register.go declares the column order with the
// same names, so a rename here that misses that file drops a column.
const (
	// flagStripPrivate and leafStripPrivate are the two spellings of the
	// secret mask on `show config dump`: the local flag, and the flag leaf the
	// model declares so the daemon route can be asked for it.
	flagStripPrivate = "--strip-private"
	leafStripPrivate = "strip-private"

	keyPath     = "path"
	keyRevision = "revision"
	keySource   = "source"
)

// withRuntimeStore runs fn against the process's config storage.
//
// It opens the store the `ze config` subcommands open when no store is handed
// to RunWithStorage, so a data handler reaches the same storage the printing
// handler does.
func withRuntimeStore(configPath string, fn func(storage.Storage) command.LocalAnswer) command.LocalAnswer {
	store, err := storage.OpenReadOnly(resolve.StoreDir(configPath))
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(fmt.Errorf("config storage: %w", err)))
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
//
// Masking the secrets SELECTS what is answered, so the model declares it as
// the flag leaf `strip-private`, which every surface can send; the local
// `--strip-private` spelling reads the same.
func dataDump(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	stripPrivate := false
	var configPath string
	for _, arg := range args {
		if arg == flagStripPrivate || arg == leafStripPrivate {
			stripPrivate = true
			continue
		}
		if configPath != "" {
			continue
		}
		// `-` is the PATH that means stdin, not an option, so it is taken
		// before the leading-dash test below can drop it with the flags
		// (ai/rules/cli.md: "- means stdin or stdout"). Reading it as an
		// option left `show config dump -` reporting the path missing, and
		// the command had no way to be piped into at all.
		if cliio.IsStdin(arg) {
			configPath = arg
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		configPath = arg
	}
	// No file is no answer: this command parses the file it is given, in this
	// process, so it cannot say what a running daemon applied. The refusal
	// names the daemon-side command that can.
	if configPath == "" {
		return command.LocalRefusal(1,
			"error: show config dump needs a config file to read (use - for stdin).\n"+
				"To see the configuration the running daemon applied, run:\n"+
				"  show config running\n")
	}

	readPath, err := command.CallerPath(validated.Route(), "show config dump", args, configPath)
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}

	var diag textbuf.Buffer
	res, code := resolveDump(readPath, stripPrivate, &diag)
	if code != 0 {
		return command.LocalRefusal(code, diag.String())
	}
	// A config that resolved can still carry parse warnings: they travel
	// beside the payload, and the local route prints them before it.
	return command.LocalAnswer{Payload: res.dumpMap, Diagnostic: diag.String()}
}

// dataList answers `show config list`: every configuration the daemon can see,
// from both places it looks.
//
// The printer wrote `[data] <key>` and `[fs] <path>` lines. The bracket prefix
// becomes a FIELD, which is what a row operator can select on: `| match data`
// used to match the prefix by accident of it being in the line.
func dataList(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if len(args) != 0 {
		return command.LocalRefusal(exitError, "usage: show config list\n")
	}
	return withRuntimeStore("", func(store storage.Storage) command.LocalAnswer {
		rows := make([]map[string]any, 0)

		for _, prefix := range []string{zefs.KeyFileActive.Dir(), zefs.KeyFileDraft.Dir()} {
			keys, err := store.List(prefix)
			if err != nil {
				continue // the directory does not exist yet
			}
			for _, key := range keys {
				rows = append(rows, map[string]any{keySource: "data", keyPath: key})
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
		return command.LocalAnswer{Payload: map[string]any{"configs": rows}}
	})
}

// dataHistory answers `show config history <file>`: the rollback revisions
// stored beside a configuration, and the draft if one is open.
//
// The draft is a ROW like any other rather than a line printed above the table,
// so a caller sees one list and can select on it. The printer wrote
// `draft  (editing in progress)` before the numbered revisions, which no row
// operator could reach.
func dataHistory(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if len(args) != 1 {
		return command.LocalRefusal(exitError, "error: requires exactly one config file\n")
	}
	if cliio.IsStdin(args[0]) {
		return command.LocalRefusal(exitError,
			"error: history needs on-disk revision history; a config read from stdin (\"-\") has none\n")
	}

	// The store opened is the one beside the named file, so on the daemon
	// route the file must lie where the daemon reads configuration from.
	if _, err := command.CallerPath(validated.Route(), "show config history", args, args[0]); err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}

	return withRuntimeStore(args[0], func(store storage.Storage) command.LocalAnswer {
		backups, err := store.ListVersions(args[0])
		if err != nil {
			return command.LocalRefusal(exitError, command.LocalErrorLine(err))
		}

		rows := make([]map[string]any, 0, len(backups)+1)
		if store.Exists(cli.DraftPath(args[0])) {
			rows = append(rows, map[string]any{keyRevision: "draft", "state": "editing in progress"})
		}
		for i, b := range backups {
			rows = append(rows, map[string]any{
				keyRevision: i + 1,
				"timestamp": b.Date.Format("2006-01-02 15:04:05"),
				keyPath:     b.Path,
			})
		}
		return command.LocalAnswer{Payload: map[string]any{"revisions": rows}}
	})
}

// dataValidate answers `show config validation <file>`: whether the configuration is
// usable, and every diagnostic that says why not.
//
// The exit code is the verdict and the payload is the evidence, so a rejected
// configuration answers its diagnostics AND exits 1
// (command.LocalDataHandler).
func dataValidate(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if len(args) == 0 {
		return command.LocalRefusal(exitError, "error: missing config file (use - for stdin)\n")
	}
	if len(args) > 1 {
		var reason textbuf.Buffer
		reason.Str("error: show config validation takes one config file, got ").Int(int64(len(args))).Byte('\n')
		return command.LocalRefusal(exitError, reason.String())
	}

	readPath, err := command.CallerPath(validated.Route(), "show config validation", args, args[0])
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}
	data, err := cliio.ReadFile(readPath)
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}

	result := runValidation(string(data), args[0])
	payload := diagnostic.NewValidateResult(result.Path, result.Valid, result.Diagnostics, result.Config)
	if result.Valid {
		return command.LocalAnswer{Payload: payload, Code: exitOK}
	}
	// The exit code is invisible on the daemon route (SSH session, web, MCP),
	// so the verdict also travels as one line beside the evidence: without it
	// the daemon reported a failure "with no reason" while the reasons sat in
	// the payload.
	var verdict textbuf.Buffer
	verdict.Str("error: ").Str(args[0]).Str(" is not a valid configuration: ").
		Int(int64(len(result.Diagnostics))).Str(" diagnostic(s) in the answer say why\n")
	return command.LocalAnswer{Payload: payload, Code: exitInvalid, Diagnostic: verdict.String()}
}

// dataDiff answers `show config diff <file1> <file2>` and
// `show config diff <N> <file>`: what the two resolved configurations do not
// agree about, with every secret value masked.
//
// It is ONE document holding three keyed sets rather than rows.
func dataDiff(validated command.ValidatedArgs) command.LocalAnswer {
	args, err := diffCallerPaths(validated)
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}
	var diag textbuf.Buffer
	diff, code := resolveDiff(nil, args, &diag)
	if diff == nil {
		return command.LocalRefusal(code, diag.String())
	}
	return command.LocalAnswer{Payload: map[string]any{
		"added":   diff.Added,
		"removed": diff.Removed,
		"changed": diff.Changed,
	}, Code: exitOK}
}

// diffCallerPaths answers the words of `show config diff` with every file
// path confined to where the route may read (command.CallerPath). A leading
// revision number names no file, and resolveDiff reads it as one; the file
// after it still names the store opened beside it, so it is confined too.
//
// A call with other than two words is answered unchanged: resolveDiff refuses
// the count with its usage line, which says more than a path refusal would.
func diffCallerPaths(validated command.ValidatedArgs) ([]string, error) {
	tokens := validated.Tokens()
	if len(tokens) != 2 {
		return tokens, nil
	}
	words := make([]string, len(tokens))
	for i, word := range tokens {
		if i == 0 && isRevisionNumber(word) {
			words[i] = word
			continue
		}
		path, err := command.CallerPath(validated.Route(), "show config diff", tokens, word)
		if err != nil {
			return nil, err
		}
		words[i] = path
	}
	return words, nil
}

// isRevisionNumber reports whether word is the revision resolveDiff reads as a
// rollback point rather than a file: it uses strconv.Atoi, and so does this.
func isRevisionNumber(word string) bool {
	_, err := strconv.Atoi(word)
	return err == nil
}
