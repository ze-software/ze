package peer

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestHandlerPeerDisableCarriesMessage checks that disable hands the joined
// tokens to the reactor as the RFC 9003 communication and echoes it.
func TestHandlerPeerDisableCarriesMessage(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handlePeerDisable(ctx, commandtest.Args("back", "soon"))
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	require.Len(t, reactor.adminCalls, 1)
	assert.Equal(t, adminCall{addr: netip.MustParseAddr("192.0.2.1"), disable: true, message: "back soon"}, reactor.adminCalls[0])
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, "back soon", data["shutdown-message"])
	assert.Equal(t, "down", data["admin-state"])
}

// TestHandlerPeerEnable checks that enable releases the named peer.
func TestHandlerPeerEnable(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handlePeerEnable(ctx, commandtest.Args())
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	require.Len(t, reactor.adminCalls, 1)
	assert.Equal(t, adminCall{addr: netip.MustParseAddr("192.0.2.1")}, reactor.adminCalls[0])
}

// TestHandlerPeerEnableRefusesArgument checks that enable refuses a stray token
// and reaches no peer, as ExaBGP's enable does.
func TestHandlerPeerEnableRefusesArgument(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.1"

	resp, err := handlePeerEnable(ctx, commandtest.Args("now"))
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "enable takes no argument")
	assert.Empty(t, reactor.adminCalls)
}
