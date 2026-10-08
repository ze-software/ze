// Design: docs/guide/config-editor.md — importing into an explicit destination
package cli

import (
	"bufio"
	"errors"
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
	targets, err := cmdImportNames(store, files, *name)
	if err != nil {
		out.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
		return exitError
	}
	if !*yes {
		if existing := cmdImportExisting(targets); len(existing) > 0 {
			// A config read from stdin leaves no terminal to answer on, so that
			// import is as non-interactive as a script's.
			interactive := !slices.ContainsFunc(files, cliio.IsStdin) && term.IsTerminal(int(os.Stdin.Fd()))
			if !cmdImportConfirm(existing, interactive, os.Stdin, os.Stderr) {
				return exitError
			}
		}
	}
	if err := cmdImportRead(targets); err != nil {
		out.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
		return exitError
	}
	for i := range targets {
		target := &targets[i]
		// A replaced config is committed as a new active version, the previous
		// one kept as rollback, because writing only the file/active mirror is
		// shadowed by an existing active pointer: the daemon would go on reading
		// the old config while the import reported success.
		verb := "imported "
		if target.exists {
			verb = "replaced "
			_, err = storage.RestoreConfig(store, target.name, target.data)
		} else {
			err = store.WriteFile(target.name, target.data, 0o600)
		}
		out.Reset()
		if err != nil {
			out.Str("error: import ").Str(target.name).Str(": ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output
			return exitError
		}
		out.Str(verb).Str(target.name).Str(" (").Int(int64(len(target.data))).Str(" bytes)\n").StdOut() //nolint:errcheck // terminal output
	}
	out.Reset()
	out.Int(int64(len(files))).Str(" file(s) imported\n").StdOut() //nolint:errcheck // terminal output
	return exitOK
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
// name, and two inputs that would land on the same config.
func cmdImportNames(store storage.Storage, files []string, name string) ([]cmdImportTarget, error) {
	targets := make([]cmdImportTarget, len(files))
	seen := make(map[string]string, len(files))
	for i, path := range files {
		key := filepath.Base(path)
		if name != "" {
			key = name
		}
		if !cmdImportNameValid(key) {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("invalid destination name ").Quoted(key).Str("; use --name for stdin").String())
		}
		if previous, ok := seen[key]; ok {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("import name ").Str(key).Str(" collides between ").Str(previous).Str(" and ").Str(path).
				Str("; import separately with --name").String())
		}
		seen[key] = path
		targets[i] = cmdImportTarget{path: path, name: key, exists: store.Exists(key)}
	}
	return targets, nil
}

// cmdImportNameValid reports whether key names a config in the store folder
// itself: not a directory step, not a path, and not the stdin marker.
func cmdImportNameValid(key string) bool {
	switch key {
	case ".", "..":
		return false
	}
	if strings.ContainsAny(key, "/\\") {
		return false
	}
	return !cliio.IsStdin(key)
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
