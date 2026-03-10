package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/transaction"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// newDispatchContext creates a CommandContext with all init()-registered RPCs,
// simulating the production dispatch chain.
func newDispatchContext() *pluginserver.CommandContext {
	server := pluginserver.NewServer(&pluginserver.ServerConfig{
		CommitManager: transaction.NewCommitManager(),
	}, nil)
	return &pluginserver.CommandContext{Server: server}
}

// TestDispatchBGPLogShow verifies "bgp log show" dispatches through init() registration.
//
// VALIDATES: AC-6 — log show registered and dispatchable.
// PREVENTS: Log show handler not registered in dispatcher.
func TestDispatchBGPLogShow(t *testing.T) {
	slogutil.ResetLevelRegistry()
	defer slogutil.ResetLevelRegistry()

	ctx := newDispatchContext()
	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp log show")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

// TestDispatchBGPLogSet verifies "bgp log set" dispatches through init() registration.
//
// VALIDATES: AC-7 — log set registered and dispatchable.
// PREVENTS: Log set handler not registered in dispatcher.
func TestDispatchBGPLogSet(t *testing.T) {
	slogutil.ResetLevelRegistry()
	defer slogutil.ResetLevelRegistry()

	t.Setenv("ze.log.dispatchsettest", "warn")
	_ = slogutil.Logger("dispatchsettest")

	ctx := newDispatchContext()
	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "bgp log set dispatchsettest debug")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}
