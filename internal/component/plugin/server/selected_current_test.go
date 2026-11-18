// Design: docs/architecture/api/process-protocol.md -- exact selecting connection.
// Related: selected_current.go -- authenticated read-only selection lookup.
package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func selectedCurrentInput() rpc.CurrentSelectionInput {
	return rpc.CurrentSelectionInput{Destination: "192.0.2.7", DestinationSession: 3,
		DeadlineMS: time.Now().Add(time.Second).UnixMilli(),
		Keys:       []rpc.CurrentSelectionKey{{Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 0, 0}, Revision: 7}}}
}

func selectedCurrentAnswer(input rpc.CurrentSelectionInput) *rpc.CurrentSelectionOutput {
	return &rpc.CurrentSelectionOutput{Destination: input.Destination, DestinationSession: input.DestinationSession,
		Cut: 7, Results: []rpc.CurrentSelectionResult{{Family: input.Keys[0].Family,
			KeyNLRI: input.Keys[0].KeyNLRI, Revision: input.Keys[0].Revision,
			Presence: rpc.CurrentSelectionPresent, CurrentRevision: 7}}}
}

func selectedCurrentProcess(t *testing.T, host *Server) *process.Process {
	t.Helper()
	proc := nativeOwnerProcess(t, host)
	require.NoError(t, proc.InitConns())
	proc.Conn().SetBridge(proc.Bridge())
	return proc
}

// TestSelectedCurrentUsesExactConnection proves a live owner reaches its own
// independent callback and a same-name replacement never inherits that authority.
func TestSelectedCurrentUsesExactConnection(t *testing.T) {
	host := &Server{ctx: t.Context()}
	first := selectedCurrentProcess(t, host)
	manager := process.NewProcessManager(nil)
	require.NoError(t, manager.AddProcess(first.Name(), first))
	host.procManager.Store(manager)
	owner, err := plugin.NewSelectedOwner(plugin.ProcessSender(first.Name()), first.Done(), first.EngineDone())
	require.NoError(t, err)
	calls := 0
	first.Bridge().SetCurrentSelection(func(_ context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		calls++
		return selectedCurrentAnswer(input), nil
	})
	first.Bridge().SetReady()
	output, err := host.CurrentSelection(t.Context(), owner, selectedCurrentInput())
	require.NoError(t, err)
	require.Equal(t, uint64(7), output.Cut)
	require.Equal(t, 1, calls)

	replacement := selectedCurrentProcess(t, host)
	require.NoError(t, manager.AddProcess(first.Name(), replacement))
	replacementCalls := 0
	replacement.Bridge().SetCurrentSelection(func(_ context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		replacementCalls++
		return selectedCurrentAnswer(input), nil
	})
	replacement.Bridge().SetReady()
	_, err = host.CurrentSelection(t.Context(), owner, selectedCurrentInput())
	require.ErrorContains(t, err, "incarnation")
	require.Zero(t, replacementCalls)
	require.Equal(t, 1, calls)
}

// TestSelectedCurrentRechecksIncarnationAfterAnswer replaces the configured
// process inside its callback; a late reply must not authorize output.
func TestSelectedCurrentRechecksIncarnationAfterAnswer(t *testing.T) {
	host := &Server{ctx: t.Context()}
	proc := selectedCurrentProcess(t, host)
	manager := process.NewProcessManager(nil)
	require.NoError(t, manager.AddProcess(proc.Name(), proc))
	host.procManager.Store(manager)
	owner, err := plugin.NewSelectedOwner(plugin.ProcessSender(proc.Name()), proc.Done(), proc.EngineDone())
	require.NoError(t, err)
	proc.Bridge().SetCurrentSelection(func(_ context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		if err := manager.AddProcess(proc.Name(), process.NewProcess(proc.Config())); err != nil {
			return nil, err
		}
		return selectedCurrentAnswer(input), nil
	})
	proc.Bridge().SetReady()
	_, err = host.CurrentSelection(t.Context(), owner, selectedCurrentInput())
	require.ErrorContains(t, err, "incarnation")
}
