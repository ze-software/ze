// Design: docs/architecture/api/commands.md — where a command is served
// Related: docs/architecture/api/commands.md — the operator model this serves
//
// yang_data.go answers `show yang tree` and `show yang completion` with
// structured data, so they reach the pipe layer. Both printed text and returned
// an exit code, while YANG declared a wire method for each that no daemon
// handler implements.
//
// The payload is produced by the JSON writer that sits beside each text
// formatter in format.go, then decoded. That costs a marshal and a parse on
// two offline introspection commands. It buys one definition of the answer's
// shape, rather than two somebody has to keep in step.
//
// `show yang doc` is NOT converted. It renders documentation PROSE for a
// reader, and the same facts already reach a machine as structured data
// through `ze help command --json`. Inventing a second record for them would
// be a second surface to keep true.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The two filter flags `show yang tree` accepts, spelled as an operator types
// them. They SELECT which half of the tree is fetched, so they are the
// command's own grammar rather than a rendering option.
const (
	flagCommands = "--commands"
	flagConfig   = "--config"
)

// yangRefusal is the answer of a handler that could not build its payload: the
// same "error: <reason>" line the printers write. The handler writes nothing
// itself (command.LocalDataHandler).
func yangRefusal(err error) command.LocalAnswer {
	return command.LocalRefusal(1, command.LocalErrorLine(err))
}

// optionRefusal is the answer to an option the parser refused. A help request
// is not a failure: it answers the command's help page with exit code 0, as
// writeOptionError does for the printing form.
//
// The page is colored when the local route's stderr is a terminal, so the
// operator reads what the printing form shows. A route that serves the command
// for a remote caller reads the same text.
func optionRefusal(err error, page *helpfmt.Page) command.LocalAnswer {
	if !errors.Is(err, flag.ErrHelp) {
		return yangRefusal(err)
	}
	var help bytes.Buffer
	page.WriteTo(&help, slogutil.UseColor(os.Stderr))
	return command.LocalRefusal(0, help.String())
}

// decodeWritten runs a JSON writer and answers what it wrote, decoded.
func decodeWritten(write func(w *bytes.Buffer) error) command.LocalAnswer {
	var buf bytes.Buffer
	if err := write(&buf); err != nil {
		return yangRefusal(err)
	}
	var payload any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		return yangRefusal(err)
	}
	return command.LocalAnswer{Payload: payload}
}

// dataTree answers `show yang tree [--commands|--config]`: the unified config
// and command tree.
//
// Its ROWS are the top-level nodes, each carrying its own children, because
// that is what formatTreeJSON emits. `| first 1` therefore answers one subtree
// whole, and `| match` keeps the roots holding the text.
func dataTree(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	options, err := parseTreeOptions(args)
	if err != nil {
		return optionRefusal(err, treeHelpPage())
	}

	root, err := buildUnifiedTree()
	if err != nil {
		return yangRefusal(err)
	}
	return decodeWritten(
		func(w *bytes.Buffer) error { return formatTreeJSON(w, root, options.filter) })
}

// dataCompletion answers `show yang completion`: the prefix collisions in the
// config and command trees, as rows.
func dataCompletion(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	options, err := parseCompletionOptions(args)
	if err != nil {
		return optionRefusal(err, completionHelpPage())
	}

	root, err := buildUnifiedTree()
	if err != nil {
		return yangRefusal(err)
	}
	groups := collectCollisions(root, options.minPrefix)
	return decodeWritten(func(w *bytes.Buffer) error { return formatCollisionsJSON(w, groups) })
}
