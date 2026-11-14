// Design: docs/guide/config-editor.md — importing into an explicit destination
// Related: ../storage/daemonroute/daemonroute.go -- the route a busy store's import takes to its daemon
package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/term"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/storage/daemonroute"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// commandConfigImport is the local command the refusals of the daemon route
// name.
const commandConfigImport = "ze config import"

// cmdImportWithStorage imports loose inputs into one independently selected store.
// A destination that already exists is replaced only once confirmed, by --yes or
// by an answer on the terminal; without either, nothing is written. While a
// daemon owns the destination store, the inputs go to that daemon as
// `update data config` (cmdImportThroughDaemon), after the same name checks and
// the same confirmation, and the same lines are printed.
func cmdImportWithStorage(store storage.Storage, args []string) int {
	return cmdImportThrough(store, args, daemonroute.OverSSH)
}

// cmdImportThrough is cmdImportWithStorage over one daemon ask, so a test stubs
// the daemon a busy store's import is sent to.
func cmdImportThrough(store storage.Storage, args []string, ask daemonroute.Ask) int {
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
		owned, err := storage.Open(folder)
		if errors.Is(err, storage.ErrBusy) {
			return cmdImportThroughDaemon(folder, files, *name, *yes, err, ask)
		}
		if err != nil {
			out.Str("error: config import destination: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
			return exitError
		}
		defer owned.Close() //nolint:errcheck // Offline ownership.
		store = owned
	}
	targets, err := cmdImportNames(store, files, *name)
	if err != nil {
		out.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
		return exitError
	}
	if !cmdImportConfirmed(targets, files, *yes) {
		return exitError
	}
	if err := cmdImportRead(targets); err != nil {
		out.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
		return exitError
	}
	for i := range targets {
		target := &targets[i]
		// store is the owner, so no daemon runs and none can open a confirmed
		// commit window during the write. A window record found here was left
		// by a daemon that stopped during that window; its next start reverts
		// the window, which would undo the import, so the import is refused
		// with the window's own refusal, as `ze config rollback` is.
		err = confirm.WriteOutsideRecorded(store, target.name, func() error {
			return cmdImportWrite(store, target)
		})
		if err != nil {
			out.Reset()
			out.Str("error: import ").Str(target.name).Str(": ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
			return exitError
		}
		cmdImportPrintDone(target)
	}
	cmdImportPrintCount(len(files))
	return exitOK
}

// cmdImportWrite lands one target in the store it owns. A replaced config is
// committed as a new active version, the previous one kept as rollback,
// because writing only the file/active mirror is shadowed by an existing
// active pointer: the daemon would go on reading the old config while the
// import reported success.
func cmdImportWrite(store storage.Storage, target *cmdImportTarget) error {
	if target.exists {
		_, err := storage.RestoreConfig(store, target.name, target.data)
		return err
	}
	return store.WriteFile(target.name, target.data, 0o600)
}

// cmdImportThroughDaemon is the import into a store a daemon owns: busy is the
// store's refusal of the owner open. It names and confirms the targets as the
// offline route does, over a reader of the same store, then reads every input
// and builds every `update data config` line before anything is sent, so an
// unreadable input or one over the bound one SSH packet carries stops the
// import with nothing sent. Each line then goes to the daemon in input order;
// the first refusal, or a daemon that does not answer, ends the import, as
// the first failed write ends the offline one. The lines printed are the
// offline route's.
func cmdImportThroughDaemon(folder string, files []string, name string, yes bool, busy error, ask daemonroute.Ask) int {
	var out textbuf.Buffer
	targets, err := cmdImportBusyNames(folder, files, name)
	if err != nil {
		out.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
		return exitError
	}
	if !cmdImportConfirmed(targets, files, yes) {
		return exitError
	}
	if err := cmdImportRead(targets); err != nil {
		out.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
		return exitError
	}
	lines, err := cmdImportLines(targets)
	if err != nil {
		out.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
		return exitError
	}
	storePath := filepath.Join(folder, storage.TreeName)
	for i := range targets {
		target := &targets[i]
		var answer daemonroute.ConfigAnswer
		err := daemonroute.Send(ask, storePath, lines[i], &answer)
		out.Reset()
		if unanswered, ok := errors.AsType[*daemonroute.Unanswered](err); ok {
			out.Str("error: ").Err(daemonroute.UnansweredRefusal(commandConfigImport, busy, unanswered)).Byte('\n').StdErr() //nolint:errcheck // terminal output
			return exitError
		}
		if err != nil {
			out.Str("error: import ").Str(target.name).Str(": ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
			return exitError
		}
		if mismatch := answer.Mismatch(commandConfigImport, target.name, len(target.data)); mismatch != nil {
			out.Str("error: ").Err(mismatch).Byte('\n').StdErr() //nolint:errcheck // terminal output
			return exitError
		}
		cmdImportPrintDone(target)
	}
	cmdImportPrintCount(len(files))
	return exitOK
}

// cmdImportBusyNames is cmdImportNames over a reader of the store a daemon
// owns, which reads beside the live owner and is closed before anything is
// sent.
func cmdImportBusyNames(folder string, files []string, name string) ([]cmdImportTarget, error) {
	reader, err := storage.OpenReadOnly(folder, folder)
	if err != nil {
		return nil, fmt.Errorf("config import destination: %w", err)
	}
	targets, namesErr := cmdImportNames(reader, files, name)
	closeErr := reader.Close()
	if namesErr != nil {
		return nil, namesErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf("config import destination: %w", closeErr)
	}
	return targets, nil
}

// cmdImportLines builds the daemon command of every target, each judged by
// the model as the daemon's validator judges it.
func cmdImportLines(targets []cmdImportTarget) ([]string, error) {
	lines := make([]string, len(targets))
	for i := range targets {
		line, err := daemonroute.ConfigLine(targets[i].name, targets[i].data)
		if err != nil {
			return nil, fmt.Errorf("import %s: %w", targets[i].name, err)
		}
		lines[i] = line
	}
	return lines, nil
}

// cmdImportPrintDone prints the line one landed target gets, on both routes.
func cmdImportPrintDone(target *cmdImportTarget) {
	verb := "imported "
	if target.exists {
		verb = "replaced "
	}
	var out textbuf.Buffer
	out.Str(verb).Str(target.name).Str(" (").Int(int64(len(target.data))).Str(" bytes)\n").StdOut() //nolint:errcheck // terminal output
}

// cmdImportPrintCount prints the count line both routes end with.
func cmdImportPrintCount(files int) {
	var out textbuf.Buffer
	out.Int(int64(files)).Str(" file(s) imported\n").StdOut() //nolint:errcheck // terminal output
}

// cmdImportTarget is one input of cmdImportWithStorage and the config it lands
// in. exists records, once, whether the store held that config before the
// import began, which decides between a replace and a fresh write.
type cmdImportTarget struct {
	path   string
	name   string
	exists bool
	data   []byte
}

// cmdImportNames names the destination of every input: --name when given,
// else the input's base name. It refuses a name that is not a plain config
// name (storage.PlainConfigName), and two inputs that would land on the same
// config.
func cmdImportNames(store storage.Storage, files []string, name string) ([]cmdImportTarget, error) {
	targets := make([]cmdImportTarget, len(files))
	seen := make(map[string]string, len(files))
	for i, path := range files {
		key := filepath.Base(path)
		if name != "" {
			key = name
		}
		if !storage.PlainConfigName(key) {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("invalid destination name ").Quoted(key).Str("; use --name for stdin").String())
		}
		if previous, ok := seen[key]; ok {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("import name ").Str(key).Str(" collides between ").Str(previous).Str(" and ").Str(path).
				Str("; import separately with --name").String())
		}
		seen[key] = path
		exists, err := cmdImportExists(store, key)
		if err != nil {
			return nil, err
		}
		targets[i] = cmdImportTarget{path: path, name: key, exists: exists}
	}
	return targets, nil
}

// cmdImportExists reports whether the store already holds the config name.
// The active pointer decides first, because the daemon reads a config through
// it: a config with versions and no file key is still replaced, never
// written beside its pointer, where the pointer would shadow it.
func cmdImportExists(store storage.Storage, name string) (bool, error) {
	pointed, err := storage.HasActivePointer(store, name)
	if err != nil {
		return false, err
	}
	if pointed {
		return true, nil
	}
	return store.Exists(name), nil
}

// cmdImportConfirmed answers whether every target that replaces an existing
// config may: --yes, no existing config, or a yes on the terminal.
func cmdImportConfirmed(targets []cmdImportTarget, files []string, yes bool) bool {
	if yes {
		return true
	}
	existing := cmdImportExisting(targets)
	if len(existing) == 0 {
		return true
	}
	// A config read from stdin leaves no terminal to answer on, so that
	// import is as non-interactive as a script's.
	interactive := !slices.ContainsFunc(files, cliio.IsStdin) && term.IsTerminal(int(os.Stdin.Fd()))
	return cmdImportConfirm(existing, interactive, os.Stdin, os.Stderr)
}

// cmdImportExisting lists the targets whose config already exists, in input
// order: the configs an import would replace.
func cmdImportExisting(targets []cmdImportTarget) []string {
	var existing []string
	for i := range targets {
		if targets[i].exists {
			existing = append(existing, targets[i].name)
		}
	}
	return existing
}

// cmdImportRead reads every input before anything is written, so an unreadable
// input stops the import with the store untouched.
func cmdImportRead(targets []cmdImportTarget) error {
	for i := range targets {
		data, err := cliio.ReadFile(targets[i].path)
		if err != nil {
			var tb textbuf.Buffer
			return errors.New(tb.Str("read ").Str(targets[i].path).Str(": ").Err(err).String())
		}
		targets[i].data = data
	}
	return nil
}

// cmdImportConfirm asks on prompts whether the existing configs in names may be
// replaced, and reads the reply from answers. Without a terminal it asks
// nothing and refuses, naming --yes, so a script never waits on a prompt. On a
// terminal, y or yes consents; any other answer, an empty one, and a failed
// read are a refusal, the failed read reported as such.
func cmdImportConfirm(names []string, interactive bool, answers io.Reader, prompts io.Writer) bool {
	var out textbuf.Buffer
	subject, exist, pronoun := "destination config ", " already exists", "it"
	if len(names) > 1 {
		subject, exist, pronoun = "destination configs ", " already exist", "them"
	}
	list := strings.Join(names, ", ")
	if !interactive {
		out.Str("error: ").Str(subject).Str(list).Str(exist).Str("; rerun with --yes to replace ").Str(pronoun).Byte('\n')
		prompts.Write(out.Bytes()) //nolint:errcheck,gosec // terminal output
		return false
	}
	out.Str(subject).Str(list).Str(exist).Str("\nreplace ").Str(pronoun).Str("? [y/N] ")
	prompts.Write(out.Bytes()) //nolint:errcheck,gosec // terminal output
	out.Reset()

	scanner := bufio.NewScanner(answers)
	// Scan is false both at the end of input and on a read failure. Err tells
	// them apart: the end of input leaves Text empty, which is a refusal, and a
	// failure is reported, so it cannot pass for an answer of no.
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		out.Str("error: read answer: ").Err(err).Byte('\n')
		prompts.Write(out.Bytes()) //nolint:errcheck,gosec // terminal output
		return false
	}
	switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
	case "y", "yes":
		return true
	}
	out.Str("error: ").Str(subject).Str(list).Str(" not replaced\n")
	prompts.Write(out.Bytes()) //nolint:errcheck,gosec // terminal output
	return false
}
