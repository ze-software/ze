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
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// openShowEditor builds the read-only editor for `ze config show`, reading the
// config from stdin when configFile is "-" (via cliio) and otherwise from the
// file. `ze config show` opens no live store (AC-24): the file is the loose
// config. With --backup, configFile names a config inside that artifact and
// the bytes are its verified active version, read under the artifact's shared lock.
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
	name, err := storage.StoredConfigName(store, configName)
	var data []byte
	if err == nil {
		data, err = storage.ReadActiveConfig(store, name)
	}
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
	return showConfigTo(os.Stdout, os.Stderr, args)
}

// showConfig is the io.Writer-parameterised core of `ze config show`, so tests
// can assert on the rendered tree without capturing os.Stdout.
func showConfig(out io.Writer, args []string) int {
	return showConfigTo(out, os.Stderr, args)
}

// showConfigTo runs `ze config show` writing byte-pure written configuration to
// out and diagnostics or plain-display implied notices to errOut.
func showConfigTo(out, errOut io.Writer, args []string) int {
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
		helpfmt.WriteError(errOut, false, "missing config file")
		fs.Usage()
		return exitError
	}

	configFile := fs.Arg(0)
	path := fs.Args()[1:]

	ed, err := openShowEditor(configFile, *backupPath)
	if err != nil {
		helpfmt.WriteError(errOut, false, "%v", err)
		return exitError
	}
	defer ed.Close() //nolint:errcheck // read-only inspection, nothing to flush

	answer := sourceEditorAnswer(ed, configFile, path)
	if answer.Diagnostic != "" {
		_, _ = io.WriteString(errOut, answer.Diagnostic) //nolint:errcheck // CLI diagnostic
	}
	if answer.Payload == nil {
		return answer.Code
	}
	blob, ok := answer.Payload.(command.Blob)
	if !ok {
		panic("BUG: source display answered something other than its bytes")
	}
	if rc := writeText(out, string(blob)); rc != exitOK {
		return rc
	}
	if whole := ed.DisplayTreeAtPath(nil); whole != nil {
		report, reportErr := config.BuildImpliedReport(whole, config.ImpliedOptions{
			Offline:    false,
			ConfigFile: configFile,
		})
		if reportErr == nil {
			if notice := config.ImpliedPlainNotice(report); notice != "" {
				var tb textbuf.Buffer
				_, _ = io.WriteString(errOut, tb.Str(notice).Byte('\n').String()) //nolint:errcheck // CLI plain-display notice
			}
		}
	}
	return answer.Code
}

// sourceEditorAnswer is the source display shared by local files and supplied
// bytes. A whole source that cannot parse is returned verbatim for repair; a
// parsed source is masked, and a missing subtree is never the whole source.
func sourceEditorAnswer(ed *editor.Editor, name string, path []string) command.LocalAnswer {
	whole := ed.DisplayTreeAtPath(nil)
	if whole == nil {
		if len(path) == 0 {
			return command.LocalAnswer{Payload: command.Blob(ed.DisplayContentAtPath(nil))}
		}
		return command.LocalRefusal(exitError, command.LocalErrorLine(opererr.New(
			name+": the configuration does not parse",
			"A subtree cannot be selected until the source can be parsed.",
			"ze show config supplied validation help")))
	}
	if len(path) == 0 {
		return command.LocalAnswer{Payload: command.Blob(ed.DisplayContentAtPath(nil))}
	}
	subtree := ed.DisplayTreeAtPath(path)
	if subtree == nil {
		var b textbuf.Buffer
		return command.LocalRefusal(exitError, command.LocalErrorLine(opererr.New(
			b.Str("path not found: ").Join(path, " ").String(),
			"No subtree at that path exists in the supplied configuration.",
			"ze show config supplied source help")))
	}
	return command.LocalAnswer{Payload: command.Blob(ed.DisplayContentAtPath(path))}
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
