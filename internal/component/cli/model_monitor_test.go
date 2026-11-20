// Design: docs/architecture/api/commands.md -- a monitor's declared pipe operations reach it on every stream site.

package cli

import (
	"context"
	"io"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestModelMonitorHandsDeclaredOperations asks whether a monitor line with a
// chain, typed in the daemon-hosted model, reaches the stream lookup
// partitioned: the monitor's own declared operation arrives in the declared
// list, and no chain word arrives as an argument (AC-35).
//
// Method: a monitor prefix declares one operation, `only`, taking an argument.
// The line carries one argument before the pipe and that operation after it.
// extractMonitorCmdArgs is the one seam both arms of the model's key handler
// call before a monitor session starts, and it answers the value the stream
// lookup (pluginserver.GetStreamingHandlerForCommand) produced.
//
// Red before the partition: the lookup splits nothing, so the arguments are
// `alpha | only up` and the declared list is empty.
// MUTATION: drop the declared list GetStreamingHandlerForCommand attaches; the
// operations comparison fails.
func TestModelMonitorHandsDeclaredOperations(t *testing.T) {
	const prefix = "monitor test model-declared-operations"
	pluginserver.RegisterStreamingHandler(prefix, pluginserver.OneShotStreamingOnly,
		func(context.Context, *pluginserver.Server, io.Writer, string, command.ValidatedArgs) error {
			return nil
		})
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(prefix) })
	command.RegisterPipeFilters([]string{prefix},
		command.PipeFilter{Name: "only", Description: "Keep the events of one state", TakesArg: true})

	validated, err := extractMonitorCmdArgs(prefix + " alpha | only up")
	if err != nil {
		t.Fatalf("the monitor line was refused: %v", err)
	}
	if got := validated.Tokens(); !slices.Equal(got, []string{"alpha"}) {
		t.Errorf("monitor arguments = %q, want [alpha]: a chain word reached the monitor as an argument", got)
	}
	want := []command.PipeOperation{{Name: "only", Args: []string{"up"}}}
	got := validated.PipeOperations()
	if !slices.EqualFunc(want, got, func(a, b command.PipeOperation) bool {
		return a.Name == b.Name && slices.Equal(a.Args, b.Args)
	}) {
		t.Errorf("declared list = %+v, want %+v", got, want)
	}
}
