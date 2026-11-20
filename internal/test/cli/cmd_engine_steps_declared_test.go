package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// TestPipedEngineDispatchSendsTheDeclaredOperations asks whether an engine
// step whose command declares a pipe operation sends that operation to the
// daemon as a pipe operation, never folded into the command words, and keeps
// the residual chain for its own rendering (package 3c of
// spec-pipe-chain-honored-on-every-command-surface).
//
// Method: a command path declaring `scope`, and a send that records the line
// and answers compact JSON. `<path> | scope | json` must be sent as
// `<path> | scope | raw` (raw keeps the daemon from adding the pipe metadata
// the step's formatter adds), and the step must read the answer as `| json`
// renders it (indented).
//
// MUTATION: send the command part alone; the declared `scope` then never
// reaches the daemon. Send the typed line; the daemon would apply `| json`
// and the step would render it a second time.
func TestPipedEngineDispatchSendsTheDeclaredOperations(t *testing.T) {
	const path = "show test engine-steps-declared"
	command.RegisterPipeFilters([]string{path},
		command.PipeFilter{Name: "scope", Description: "Pick a scope", Argument: command.ArgNone})

	var sent []string
	dispatch := pipedEngineDispatch(func(_ context.Context, line string) (string, []byte, error) {
		sent = append(sent, line)
		return "done", []byte(`{"ok":"yes"}`), nil
	})

	status, output, err := dispatch(context.Background(), path+" | scope | json")
	if err != nil {
		t.Fatalf("piped dispatch failed: %v", err)
	}
	if want := path + " | scope | raw"; len(sent) != 1 || sent[0] != want {
		t.Fatalf("daemon received %q, want %q", sent, want)
	}
	if status != "done" {
		t.Fatalf("status = %q, want the daemon's done", status)
	}
	if !strings.Contains(output, `"ok": "yes"`) {
		t.Fatalf("| json was not applied by the step: %q", output)
	}
}
