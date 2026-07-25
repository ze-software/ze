// Design: docs/architecture/config/syntax.md — stdin ("-") wiring for config editor commands
// Overview: main.go — dispatch and exit codes
package cli

import (
	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
)

// openEditableConfig builds the editor for a read-modify command (set,
// deactivate, activate). For "-" it reads the config from stdin and routes the
// save to stdout, turning the command into a pipeline stage; a real path is
// edited in place, byte-identically to before (AC-10, AC-13). The stdin read
// claims the process's single stdin via cliio.
func openEditableConfig(store storage.Storage, configPath string) (*editor.Editor, error) {
	if !cliio.IsStdin(configPath) {
		return editor.NewEditorWithStorage(store, configPath)
	}
	data, err := cliio.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	ed, err := editor.NewEditorFromContent(data, configPath)
	if err != nil {
		return nil, err
	}
	sink, err := cliio.Create(configPath) // "-" -> stdout
	if err != nil {
		return nil, err
	}
	ed.SetStdoutSink(sink)
	return ed, nil
}
