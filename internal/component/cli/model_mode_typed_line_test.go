// Design: docs/architecture/api/commands.md -- a rendering surface sends the typed line and renders the residual chain.
package cli

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// TestOperationalCommandSendsTheTypedLine asks whether the model hands its
// executor the line as typed, pipe chain included, and renders the residual
// chain over the answer itself (package 3c of
// spec-pipe-chain-honored-on-every-command-surface), on both filesystem
// authorities.
//
// Method: a command path that declares one operation (`scope`), and an
// executor that records its input and answers compact JSON. The line
// `<path> | scope | json` must reach the executor whole, and the result must
// be the document as `| json` renders it (indented), which the executor never
// does.
//
// MUTATION: hand the executor the command part the formatter returns; it then
// records `<path>` alone and the declared `scope` never reaches the daemon.
func TestOperationalCommandSendsTheTypedLine(t *testing.T) {
	const path = "show test model-typed-line"
	command.RegisterPipeFilters([]string{path},
		command.PipeFilter{Name: "scope", Description: "Pick a scope", Argument: command.ArgNone})
	const line = path + " | scope | json"

	for _, authority := range []FilesystemAuthority{FilesystemAuthorityUnknown, FilesystemAuthorityOperatorLocal} {
		var received []string
		m := NewCommandModel(authority)
		m.SetCommandExecutor(func(input string) (CommandOutput, error) {
			received = append(received, input)
			return CommandOutput{Text: `{"ok":"yes"}`}, nil
		})

		raw := m.executeOperationalCommand(line)()
		msg, ok := raw.(commandResultMsg)
		if !ok {
			t.Fatalf("command result type = %T, want commandResultMsg", raw)
		}
		if msg.err != nil {
			t.Fatalf("the typed line was refused: %v", msg.err)
		}
		if len(received) != 1 || received[0] != line {
			t.Fatalf("executor received %q, want the typed line %q", received, line)
		}
		if !strings.Contains(msg.result.output, `"ok": "yes"`) {
			t.Errorf("the model did not render | json over the answer: %q", msg.result.output)
		}
	}
}
