package raw

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	// The top-level `peer` container's selector leaf is declared by
	// ze-cli-announce-cmd, not by this module, and inheritArgDefs anchors it to
	// the `peer` keyword for every command below it. Without that module the
	// tree these tests build gives `peer <selector> raw` no selector at all, so
	// the package would test a grammar the daemon does not run.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/announce/yang"
)

// newDispatchContext creates a CommandContext with all init()-registered RPCs,
// simulating the production dispatch chain, and states who issued the command.
func newDispatchContext(reactor plugin.ReactorLifecycle, sender plugin.Sender) *pluginserver.CommandContext {
	server, _ := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	return &pluginserver.CommandContext{Server: server, Sender: sender}
}

// TestDispatchBGPPeerRaw verifies "peer <addr> raw" dispatches correctly, and
// that the dispatch chain carries the issuer's identity to the rail.
//
// The identity half is not decoration. Raw writes bytes of the caller's choosing
// into one session, so the rail gates it on the peer attaching that process
// ((*reactorAPIAdapter).SendRawMessage, rawOrigin), and a chain that lost
// ctx.Sender on the way would arrive as the zero Sender: a command nobody
// claimed, which that rail refuses. This test used to dispatch with exactly that
// zero value and assert success, which documented the opposite of the rule.
//
// VALIDATES: the chain reaches handleRaw with the peer selector and the payload,
// an operator gets through, and an unnamed issuer is refused with nothing sent.
// PREVENTS: a dispatch path that forgets to set CommandContext.Sender looking
// like a working command here while the daemon refuses it, or worse, a handler
// that substitutes the operator's authority for the one it was given.
func TestDispatchBGPPeerRaw(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor, plugin.OperatorSender())

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "peer 192.0.2.1 raw type update hex DEADBEEF")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.rawMessages, 1)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, reactor.rawMessages[0].payload)
	assert.True(t, reactor.rawMessages[0].sender.IsOperator(),
		"the operator's authority must arrive as the operator's, not as a process")

	// The same command with nobody named. The refusal is the rail's, and the
	// mock states it where the reactor states it (mock_reactor_test.go).
	unnamed := newDispatchContext(reactor, plugin.Sender{})
	resp, err = unnamed.Server.Dispatcher().Dispatch(unnamed, "peer 192.0.2.1 raw type update hex DEADBEEF")
	require.Error(t, err, "a raw injection nobody claimed must be refused")
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Len(t, reactor.rawMessages, 1, "a refused injection must send nothing")
}

// TestDispatchBGPPeerRawCarriesAProcessIdentity pins the third state: a command
// issued by a named process arrives as that process.
//
// VALIDATES: the chain neither drops the process name nor upgrades it to the
// operator's exemption, which is what the rail's attach-block lookup reads.
// PREVENTS: the send permission being decided on an identity the entry point
// invented, which no config could then grant or refuse.
func TestDispatchBGPPeerRawCarriesAProcessIdentity(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor, plugin.ProcessSender("injector"))

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "peer 192.0.2.1 raw type update hex DEADBEEF")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.rawMessages, 1)
	got := reactor.rawMessages[0].sender
	assert.False(t, got.IsOperator(), "a process must not arrive carrying operator authority")
	process, ok := got.Process()
	require.True(t, ok)
	assert.Equal(t, "injector", process)
}
