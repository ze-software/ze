// Design: docs/architecture/api/commands.md -- clear bgp rib advertised origin authority.
// Related: advertised.go -- handleRibClearAdvertised preserves ctx.OriginContext().
package peer

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/selector"
)

type advertisedOriginReactor struct {
	*mockReactor
	gotSender      plugin.Sender
	gotHasDeadline bool
	gotCtxErr      error
	gotOwner       plugin.SelectedOwner
	gotOwnerErr    error
}

func (r *advertisedOriginReactor) WithdrawAdvertised(ctx context.Context, sel *selector.Selector, afi uint16, safi uint8, sender plugin.Sender) (int, error) {
	r.gotSender = sender
	_, r.gotHasDeadline = ctx.Deadline()
	r.gotCtxErr = ctx.Err()
	r.gotOwner, r.gotOwnerErr = plugin.OriginProducerFromContext(ctx)
	return r.mockReactor.WithdrawAdvertised(ctx, sel, afi, safi, sender)
}

func startClearAdvertisedProcess(t *testing.T, name string) *process.Process {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:       name,
		RunEngine:  func(net.Conn) int { <-ctx.Done(); return 0 },
		CLIHandler: func([]string) int { return 0 },
	}))
	proc := process.NewProcess(plugin.PluginConfig{Name: name, Internal: true})
	require.NoError(t, proc.StartWithContext(ctx))
	t.Cleanup(func() {
		cancel()
		proc.Stop()
		join, stopJoin := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopJoin()
		require.NoError(t, proc.Wait(join))
	})
	require.NoError(t, proc.InitConns())
	return proc
}

// TestRibClearAdvertisedPreservesOriginProducerContext proves that
// handleRibClearAdvertised derives its timeout context from ctx.OriginContext()
// rather than context.Background(), preserving the authenticated process
// incarnation, fail-closed missing-process refusal, and request cancellation.
// Method: drives handleRibClearAdvertised with a real internal process and a
// recording reactor adapter, inspecting OriginProducerFromContext, deadline,
// and cancellation state inside WithdrawAdvertised while the call is active.
func TestRibClearAdvertisedPreservesOriginProducerContext(t *testing.T) {
	const pluginName = "test-clear-advertised-origin"
	proc := startClearAdvertisedProcess(t, pluginName)
	expectedOwner, err := proc.SelectedOwner()
	require.NoError(t, err)

	reactor := &advertisedOriginReactor{mockReactor: &mockReactor{advertisedCount: 1}}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)

	sender := plugin.ProcessSender(pluginName)
	cmdCtx := &pluginserver.CommandContext{
		Server:         server,
		Process:        proc,
		RequestContext: t.Context(),
		Sender:         sender,
		Selectors:      map[string]string{advertisedSelectorLeaf: "127.0.0.1"},
	}

	resp, err := handleRibClearAdvertised(cmdCtx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status, resp.Error)
	require.Equal(t, sender, reactor.gotSender)
	require.True(t, reactor.gotHasDeadline, "withdraw context must retain the bounded timeout")
	require.NoError(t, reactor.gotCtxErr, "active request context must not be canceled during WithdrawAdvertised")
	require.NoError(t, reactor.gotOwnerErr)
	require.True(t, reactor.gotOwner.SameIncarnation(expectedOwner), "clear advertised must carry the authenticated process incarnation")

	// Missing Process with a process Sender must stay an explicit refusal
	// rather than falling through without a producer token.
	cmdCtx.Process = nil
	resp, err = handleRibClearAdvertised(cmdCtx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status, resp.Error)
	require.ErrorContains(t, reactor.gotOwnerErr, "origin producer process is missing")

	// Request cancellation must propagate into WithdrawAdvertised.
	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()
	cmdCtx.Process = proc
	cmdCtx.RequestContext = canceledCtx
	resp, err = handleRibClearAdvertised(cmdCtx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status, resp.Error)
	require.ErrorIs(t, reactor.gotCtxErr, context.Canceled)
}
