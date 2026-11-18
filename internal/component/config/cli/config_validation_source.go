// Design: docs/architecture/api/commands.md -- validation source selection.
// Related: cmd_validate.go -- the local pending adapter.
// Related: config_data.go -- the modeled file validation adapter.

package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// readValidationSource judges the actual source selected by pending before it
// reads bytes. A confined committed neighbor cannot authorize an escaping draft.
func readValidationSource(route command.Route, tokens []string, file string, pending bool) ([]byte, string, error) {
	selected := file
	if pending {
		if cliio.IsStdin(file) {
			return nil, "", opererr.New("pending validation cannot read standard input",
				"A saved draft needs a configuration filename.", "ze config validate --pending <file>")
		}
		selected += ".draft"
	}
	readPath, err := command.CallerPath(route, "show config validation", tokens, selected)
	if err != nil {
		return nil, "", err
	}
	data, err := cliio.ReadFile(readPath)
	if err != nil {
		var fix textbuf.Buffer
		fix.Str("ze config validate ")
		if pending {
			fix.Str("--pending ")
		}
		command.WriteInvocationValue(&fix, file)
		return nil, "", opererr.Wrap(err, "configuration source "+selected+" could not be read",
			"No validation result is available; make the selected source readable and retry.", fix.String())
	}
	return data, selected, nil
}
