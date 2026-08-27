// Design: docs/architecture/api/commands.md — where a command is served
// Related: docs/architecture/api/commands.md — the operator model this serves
//
// yang_data.go answers `show yang tree` and `show yang completion` with
// structured data, so they reach the pipe layer. Both printed text and returned
// an exit code, while YANG declared a wire method for each that no daemon
// handler implements.
//
// The payload is produced by the SAME writer the `--json` flag uses, then
// decoded. That costs a marshal and a parse on two offline introspection
// commands, and it buys the guarantee that the piped answer and the `--json`
// answer are the same shape by construction rather than by two definitions
// somebody has to keep in step.
//
// `show yang doc` is NOT converted. It renders documentation PROSE for a
// reader, it has no --json path to lift, and the same facts already reach a
// machine as structured data through `ze help command --json`. Inventing a
// second record for them would be a second surface to keep true.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// The two filter flags `show yang tree` accepts, spelled as an operator types
// them.
const (
	flagCommands = "--commands"
	flagConfig   = "--config"
)

// decodeWritten runs a JSON writer and answers what it wrote, decoded.
func decodeWritten(write func(w *bytes.Buffer) error) (any, int) {
	var buf bytes.Buffer
	if err := write(&buf); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return nil, 1
	}
	var payload any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return nil, 1
	}
	return payload, 0
}

// dataTree answers `show yang tree [--commands|--config]`: the unified config
// and command tree.
//
// Its ROWS are the top-level nodes, each carrying its own children, because
// that is what formatTreeJSON emits. `| first 1` therefore answers one subtree
// whole, and `| match` keeps the roots holding the text.
func dataTree(args []string) (any, int) {
	options, err := parseTreeOptions(args)
	if err != nil {
		return nil, writeOptionError(err)
	}
	if options.jsonOutput {
		writeLocalJSONFlagError()
		return nil, 1
	}

	root, err := buildUnifiedTree()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return nil, 1
	}
	return decodeWritten(
		func(w *bytes.Buffer) error { return formatTreeJSON(w, root, options.filter) })
}

// dataCompletion answers `show yang completion`: the prefix collisions in the
// config and command trees, as rows.
func dataCompletion(args []string) (any, int) {
	options, err := parseCompletionOptions(args)
	if err != nil {
		return nil, writeOptionError(err)
	}
	if options.jsonOutput {
		writeLocalJSONFlagError()
		return nil, 1
	}

	root, err := buildUnifiedTree()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return nil, 1
	}
	groups := collectCollisions(root, options.minPrefix)
	return decodeWritten(func(w *bytes.Buffer) error { return formatCollisionsJSON(w, groups) })
}

func writeLocalJSONFlagError() {
	fmt.Fprintln(os.Stderr, "error: --json is a ze yang rendering option; use | json")
}
