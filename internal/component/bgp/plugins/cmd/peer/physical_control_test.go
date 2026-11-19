// Design: docs/architecture/api/process-protocol.md -- physical flow-control command boundary.
package peer

import (
	"context"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"net/netip"
	"testing"
)

type physicalPeerCapture struct {
	mockReactor
	seen []physicalPeerCall
}
type physicalPeerCall struct {
	ctx               context.Context
	address           netip.Addr
	owner, connection uint64
	pause             bool
}

func (r *physicalPeerCapture) PausePeer(ctx context.Context, address netip.Addr, owner, connection uint64) error {
	r.seen = append(r.seen, physicalPeerCall{ctx, address, owner, connection, true})
	return nil
}
func (r *physicalPeerCapture) ResumePeer(ctx context.Context, address netip.Addr, owner, connection uint64) error {
	r.seen = append(r.seen, physicalPeerCall{ctx, address, owner, connection, false})
	return nil
}

// Actual registered peer handlers must opt in and pass the supplied tuple and
// request context to the existing lifecycle controller. Unbound operator calls
// deliberately preserve their zero tuple. No address-to-owner lookup occurs.
func TestPeerFlowControlCarriesOriginalPhysicalConstraint(t *testing.T) {
	for _, wire := range []string{"ze-bgp:peer-pause", "ze-bgp:peer-resume"} {
		found := false
		for _, registration := range pluginserver.AllBuiltinRPCs() {
			if registration.WireMethod != wire {
				continue
			}
			found = true
			require.True(t, registration.AcceptsPeerConnection)
		}
		require.True(t, found, "actual flow-control command registration disappeared")
	}
	reactor := &physicalPeerCapture{}
	ctx := newTestContext(reactor)
	ctx.Peer = "192.0.2.249"
	ctx.RequestContext = t.Context()
	ctx.PeerOwner = 901
	ctx.ConnectionID = 902
	response, err := handleBgpPeerPause(ctx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
	response, err = handleBgpPeerResume(ctx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
	require.Len(t, reactor.seen, 2)
	for _, call := range reactor.seen {
		require.Equal(t, ctx.RequestContext, call.ctx)
		require.Equal(t, netip.MustParseAddr(ctx.Peer), call.address)
		require.Equal(t, uint64(901), call.owner)
		require.Equal(t, uint64(902), call.connection)
	}
	require.True(t, reactor.seen[0].pause)
	require.False(t, reactor.seen[1].pause)
	ctx.PeerOwner = 0
	ctx.ConnectionID = 0
	response, err = handleBgpPeerPause(ctx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
	require.Len(t, reactor.seen, 3)
	require.Zero(t, reactor.seen[2].owner)
	require.Zero(t, reactor.seen[2].connection)
}
