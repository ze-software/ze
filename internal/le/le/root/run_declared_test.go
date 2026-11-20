// Design: docs/architecture/api/commands.md -- an le tool's declared pipe operations reach it as a list.

package leroot

import (
	"bytes"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// TestRunHandsDeclaredOperationsNotArgv asks whether an le tool whose path
// declares a pipe operation receives it in the declared list, from the line
// read once by the partition, with its own argv untouched, and whether the
// residual chain still renders its answer.
//
// Method: the tool's path declares `only`; the line holds one argument, that
// operation and a generic `| count`. The handler records its argv and list.
//
// MUTATION: drop WithPipeOperations in runWithStartup; the list is empty. Put
// the fold back (append the declared words to argv); the argv comparison fails.
func TestRunHandsDeclaredOperationsNotArgv(t *testing.T) {
	const name = "declared-operations-probe"
	command.RegisterPipeFilters([]string{CommandPath(name)},
		command.PipeFilter{Name: "only", Description: "Keep the rows of one state", Argument: command.ArgText})
	var argv []string
	var operations []command.PipeOperation
	handler := func(args command.ValidatedArgs) command.LocalAnswer {
		argv = args.Tokens()
		operations = args.PipeOperations()
		return command.LocalAnswer{Payload: map[string]any{"rows": []any{
			map[string]any{"state": "up"}, map[string]any{"state": "up"},
		}}}
	}
	var stdout, stderr bytes.Buffer
	code := Run(name, handler, []string{"alpha", "|", "only", "up", "|", "count"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run answered %d, want 0: %s", code, stderr.String())
	}
	if !slices.Equal(argv, []string{"alpha"}) {
		t.Errorf("tool argv = %q, want [alpha]: a declared operation reached it as words", argv)
	}
	want := []command.PipeOperation{{Name: "only", Args: []string{"up"}}}
	if !slices.EqualFunc(want, operations, func(a, b command.PipeOperation) bool {
		return a.Name == b.Name && slices.Equal(a.Args, b.Args)
	}) {
		t.Errorf("declared list = %+v, want %+v", operations, want)
	}
	if stdout.String() != "2\n" {
		t.Errorf("stdout = %q, want the residual count of two rows", stdout.String())
	}
}
