// Design: docs/architecture/api/commands.md -- a live view hands its command's declared operations to the handler it starts.

package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// TestLiveViewsHandDeclaredOperationsToTheirHandler asks whether a command
// reached through ProcessStreamPipes (the model's piped ping and traceroute
// views) that declares a pipe operation has it handed to the handler the view
// starts, rather than dropped, and whether a view whose command declares none
// starts its handler as before.
//
// Method: both monitors declare `only` for this test; each recording factory
// reads the declared list from the context it is started with
// (command.PipeOperationsFromContext). Not parallel: it changes the pipe
// filter registry, which the cleanup empties again.
//
// MUTATION: drop the factory wrap in startPingMonitorPiped or
// startTraceroutePiped, or stop setting PipeFlags.Declared; that list is empty.
func TestLiveViewsHandDeclaredOperationsToTheirHandler(t *testing.T) {
	t.Cleanup(command.ResetPipeFiltersForTest)
	command.RegisterPipeFilters([]string{"monitor ping", "monitor traceroute"},
		command.PipeFilter{Name: "only", Description: "Keep the replies of one state", Argument: command.ArgText})
	want := []command.PipeOperation{{Name: "only", Args: []string{"up"}}}

	var pinged [][]command.PipeOperation
	m := NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.SetPingFactory(func(ctx context.Context, _ string, _, _ time.Duration, _, _ int) (<-chan map[string]any, context.CancelFunc, error) {
		pinged = append(pinged, command.PipeOperationsFromContext(ctx))
		ch := make(chan map[string]any)
		close(ch)
		return ch, func() {}, nil
	})
	require.NotNil(t, m.startPingMonitorPiped("monitor ping 192.0.2.1 | only up | log"), m.statusMessage)
	require.Len(t, pinged, 1)
	assert.Equal(t, want, pinged[0], "the ping handler receives the declared list")

	var traced [][]command.PipeOperation
	m = NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.setTracerouteFactory(func(ctx context.Context, _ string, _ int) (<-chan map[string]any, context.CancelFunc, error) {
		traced = append(traced, command.PipeOperationsFromContext(ctx))
		ch := make(chan map[string]any)
		close(ch)
		return ch, func() {}, nil
	})
	require.NotNil(t, m.startTraceroutePiped("monitor traceroute 192.0.2.1 | only up | log"), m.statusMessage)
	require.NotEmpty(t, traced, "the traceroute view started a round")
	assert.Equal(t, want, traced[0], "the traceroute handler receives the declared list")

	pinged = nil
	m = NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.SetPingFactory(func(ctx context.Context, _ string, _, _ time.Duration, _, _ int) (<-chan map[string]any, context.CancelFunc, error) {
		pinged = append(pinged, command.PipeOperationsFromContext(ctx))
		ch := make(chan map[string]any)
		close(ch)
		return ch, func() {}, nil
	})
	require.NotNil(t, m.startPingMonitorPiped("monitor ping 192.0.2.1 | log"), m.statusMessage)
	require.Len(t, pinged, 1)
	assert.Nil(t, pinged[0], "a chain with no declared operation hands none")
}
