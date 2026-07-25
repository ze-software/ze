// Design: docs/architecture/config/syntax.md — one-shot config-path inspection
// Overview: main.go — dispatch and exit codes
// Related: cmd_dump.go — full-tree dump (this is the path-scoped sibling)

package cli

import (
	"encoding/json"
	"flag"
	"io"
	"os"

	editor "github.com/ze-software/ze/internal/component/cli"
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
// (`bgp peer edge1`). `--json` emits the subtree as a JSON object.
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
	jsonOutput := fs.Bool("json", false, "output the subtree as JSON")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config show",
			Summary: "Show the configuration tree at a path",
			Usage:   []string{"ze config show <file> [path...]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--json", Desc: "Output the subtree as JSON"},
				}},
			},
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

	// No path: whole tree.
	if len(path) == 0 {
		if *jsonOutput {
			return encodeJSONTo(out, ed.Tree().ToMap())
		}
		return writeText(out, ed.ContentAtPath(nil))
	}

	// Resolve the path first so a miss is an explicit error, not a silent
	// fall-back to the whole tree (which ContentAtPath does on a bad path).
	subtree := ed.WalkPath(path)
	if subtree == nil {
		var b textbuf.Buffer
		helpfmt.WriteError(os.Stderr, false, "path not found: %s", b.Join(path, " ").String())
		return exitError
	}

	if *jsonOutput {
		return encodeJSONTo(out, subtree.ToMap())
	}
	return writeText(out, ed.ContentAtPath(path))
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

// encodeJSONTo writes v as indented JSON to w, returning an exit code.
func encodeJSONTo(w io.Writer, v any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		helpfmt.WriteError(os.Stderr, false, "encoding JSON: %v", err)
		return exitError
	}
	return exitOK
}
