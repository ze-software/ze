// Design: docs/architecture/config/syntax.md — one-shot config-path inspection
// Overview: main.go — dispatch and exit codes
// Related: cmd_dump.go — full-tree dump (this is the path-scoped sibling)

package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// openShowEditor builds the read-only editor for `ze config show`, reading the
// config from stdin when configFile is "-" (via cliio) and otherwise from the
// file. `ze config show` opens no live store (AC-24): the file is the loose
// config. With --backup, configFile names a config inside that artifact and
// the bytes are its file/active entry, read under the artifact's shared lock.
func openShowEditor(configFile, backupPath string) (*editor.Editor, error) {
	if backupPath != "" {
		return openBackupShowEditor(configFile, backupPath)
	}
	data, err := cliio.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	return editor.NewEditorFromContent(data, configFile)
}

// openBackupShowEditor reads one config out of a backup artifact and releases
// the artifact before the editor is built: the render needs the bytes, never
// the handle.
func openBackupShowEditor(configName, backupPath string) (*editor.Editor, error) {
	store, err := openBackup(backupPath, false)
	if err != nil {
		return nil, err
	}
	data, err := store.ReadFile(configName)
	if closeErr := store.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, fmt.Errorf("backup %s: %w", backupPath, err)
	}
	return editor.NewEditorFromContent(data, configName)
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
	return showConfig(os.Stdout, args)
}

// showConfig is the io.Writer-parameterised core of `ze config show`, so tests
// can assert on the rendered tree without capturing os.Stdout.
func showConfig(out io.Writer, args []string) int {
	fs := flag.NewFlagSet("config show", flag.ExitOnError)
	backupPath := fs.String(flagBackup, "", "Read the config from a backup artifact")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze config show",
			ShortHelp: "Show the configuration tree at a path",
			Usage:     []string{"ze config show <file> [path...]", "ze config show --backup <artifact> <config-name> [path...]"},
			Examples: []string{
				"ze config show ze.conf",
				"ze config show ze.conf bgp",
				"ze config show ze.conf bgp peer edge1",
				"ze config show ze.conf environment web",
				"ze config show --backup router.zefs router.conf bgp",
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

	ed, err := openShowEditor(configFile, *backupPath)
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
