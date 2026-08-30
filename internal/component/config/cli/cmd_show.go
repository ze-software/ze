// Design: docs/architecture/config/syntax.md — one-shot config-path inspection
// Overview: main.go — dispatch and exit codes
// Related: cmd_dump.go — full-tree dump (this is the path-scoped sibling)

package cli

import (
	"flag"
	"io"
	"os"

	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// openShowEditor builds the read-only editor for `ze config show`, reading the
// config from stdin when configFile is "-" (via cliio) and otherwise from the
// store. The stdin form parses the piped bytes directly; no file is touched.
func openShowEditor(store storage.Storage, configFile string) (*editor.Editor, error) {
	if cliio.IsStdin(configFile) {
		data, err := cliio.ReadFile(configFile)
		if err != nil {
			return nil, err
		}
		return editor.NewEditorFromContent(data, configFile)
	}
	return editor.NewEditorWithStorage(store, configFile)
}

// cmdShow implements `ze config show <file> [path...]`.
//
// It is the one-shot, non-interactive way to inspect an on-disk configuration
// at a path: `ze config show ze.conf bgp peer edge1` prints the tree rooted at
// that path. With no path it prints the whole parsed tree. The path tokens are
// the same space-separated config path that `ze config set` and the
// `ze config completion` engine use; list entries are addressed by their key
// (`bgp peer edge1`).
//
// Every secret leaf reads as the display placeholder, in the text form and in
// the JSON form. The parser decodes a $9$ value into the tree, so an unmasked
// render published in cleartext what the file holds encoded.
//
// Like `ze config dump`/`validate`, it reads a config file directly from the
// filesystem (not the blob store), so a plain path works without `-f`.
func cmdShow(args []string) int {
	return showConfig(os.Stdout, storage.NewFilesystem(), args)
}

// showConfig is the io.Writer-parameterised core of `ze config show`, so tests
// can assert on the rendered tree without capturing os.Stdout.
func showConfig(out io.Writer, store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config show", flag.ExitOnError)
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config show",
			Summary: "Show the configuration tree at a path",
			Usage:   []string{"ze config show <file> [path...]"},
			Examples: []string{
				"ze config show ze.conf",
				"ze config show ze.conf bgp",
				"ze config show ze.conf bgp peer edge1",
				"ze config show ze.conf environment web",
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() < 1 {
		helpfmt.WriteError(os.Stderr, false, "missing config file")
		fs.Usage()
		return exitError
	}

	configFile := fs.Arg(0)
	path := fs.Args()[1:]

	ed, err := openShowEditor(store, configFile)
	if err != nil {
		helpfmt.WriteError(os.Stderr, false, "%v", err)
		return exitError
	}
	defer ed.Close() //nolint:errcheck // read-only inspection, nothing to flush

	// The text form of the whole configuration still answers the raw file when
	// the parse failed: the operator must read the broken line to repair it.
	// Every other shape reports the failure, which is what showTree does.
	if ed.DisplayTreeAtPath(nil) == nil && len(path) == 0 {
		return writeText(out, ed.DisplayContentAtPath(nil))
	}

	// Resolve the tree first so a parse failure and a path miss are two
	// answers, and so a bad path is an explicit error rather than the silent
	// fall-back to the whole tree that DisplayContentAtPath does.
	if _, code := showTree(ed, configFile, path); code != exitOK {
		return code
	}
	return writeText(out, ed.DisplayContentAtPath(path))
}

// showTree resolves the masked display tree of an open config at a path, and
// names on stderr which of the two failures it met: a configuration that parses
// nowhere, or a path that resolves to nothing.
//
// It is the one resolution both spellings of this command run: the text form
// above.
func showTree(ed *editor.Editor, configFile string, path []string) (*config.Tree, int) {
	whole := ed.DisplayTreeAtPath(nil)
	if whole == nil {
		helpfmt.WriteError(os.Stderr, false, "%s: the configuration does not parse", configFile)
		return nil, exitError
	}
	if len(path) == 0 {
		return whole, exitOK
	}
	subtree := ed.DisplayTreeAtPath(path)
	if subtree == nil {
		var b textbuf.Buffer
		helpfmt.WriteError(os.Stderr, false, "path not found: %s", b.Join(path, " ").String())
		return nil, exitError
	}
	return subtree, exitOK
}

// writeText writes s to w and maps a write error (e.g. a closed pipe) to a
// non-zero exit code rather than a silent partial-write success.
func writeText(w io.Writer, s string) int {
	if _, err := io.WriteString(w, s); err != nil {
		helpfmt.WriteError(os.Stderr, false, "write: %v", err)
		return exitError
	}
	return exitOK
}
