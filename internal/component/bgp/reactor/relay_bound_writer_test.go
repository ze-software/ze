// Design: docs/architecture/api/process-protocol.md -- bound replay final socket ownership.
package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type boundRelayWireFixture struct {
	target  *selectedWriterFixture
	source  *Peer
	session *Session
	route   rpc.StoredRoute
	sender  plugin.Sender
}

// newBoundRelayWireFixture uses the real forwarding materializer and final
// request writer. The existing selected fixture supplies an established socket,
// exact process lifetime and normal grant, not a selected-output bypass.
func newBoundRelayWireFixture(t *testing.T, addPath bool) *boundRelayWireFixture {
	t.Helper()
	target := newSelectedWriterFixture(t, addPath)
	source := NewPeer(&PeerSettings{Address: netip.MustParseAddr("192.0.2.79"), Port: 179, LocalAS: 65000, PeerAS: 65003})
	acceptingPolicies(t, source.settings)
	boundSourceGrant(t, source, selectedWriterProcess)
	session := sourceSessionFixture(t, source)
	contextID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: addPath}))
	require.NoError(t, err)
	session.setRecvCtxID(contextID)
	require.NoError(t, session.captureSourceReceipt(source, &sourceReceipts))
	t.Cleanup(session.closeSourceReceipt)
	target.api.r.peers[source.settings.PeerKey()] = source
	source.reactor = target.api.r
	target.peer.establishedSession.Store(target.peer.currentSession())
	sender, err := target.owner.Sender().WithDestination(target.peer.sourceOwner, target.peer.currentSession().initialReplay.Load())
	require.NoError(t, err)
	route := rpc.StoredRoute{SourcePeer: source.addrString, SourceOwner: source.sourceOwner, SourceReceipt: session.sourceReceiptID(), SourceConnectionID: session.connectionID,
		Family: family.IPv4Unicast.String(), MsgID: 19, SourcePolicyEpoch: source.sourcePolicyEpoch.Load(), NLRIFraming: rpc.NLRIFramingPrefixOnly, PathID: 0,
		NLRIHex: "18c61207", AttrHex: hex.EncodeToString(synthesizedNextHopAttrs(source.settings.Address, 65003)), NextHopHex: hex.EncodeToString(source.settings.Address.AsSlice())}
	return &boundRelayWireFixture{target: target, source: source, session: session, route: route, sender: sender}
}
func (f *boundRelayWireFixture) relay(t *testing.T, route rpc.StoredRoute) error {
	t.Helper()
	return f.target.api.RelayStoredRouteBound(t.Context(), f.target.peer.settings.Address, []rpc.StoredRoute{route}, f.sender, f.target.owner)
}

// TestBoundStoredRelaySettlesActualOriginalSocket proves synchronous final wire
// completion for normal and legal source ADD-PATH zero. A same-address sibling
// cannot receive the original target's replay, and a wrong tuple refuses it.
func TestBoundStoredRelaySettlesActualOriginalSocket(t *testing.T) {
	for _, addPath := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "path-zero"}[addPath], func(t *testing.T) {
			f := newBoundRelayWireFixture(t, addPath)
			sibling := NewPeer(&PeerSettings{Address: f.target.peer.settings.Address, Port: 2179, LocalAS: 65000, PeerAS: 65199})
			sourceSessionFixture(t, sibling)
			f.target.api.r.peers[sibling.settings.PeerKey()] = sibling
			require.NoError(t, f.relay(t, f.route))
			bodies := updateBodies(t, f.target.conn.written())
			require.Len(t, bodies, 1, "return follows the actual socket flush")
			require.NotEmpty(t, bodies[0])
			require.Len(t, f.target.peer.adjOut.routes, 1)
			for _, entry := range f.target.peer.adjOut.routes {
				require.Same(t, f.source, entry.owner.source)
				require.Equal(t, f.session.connectionID, entry.owner.connectionID)
				require.Equal(t, uint32(0), entry.owner.received)
			}
			wrong, err := f.target.owner.Sender().WithDestination(sibling.sourceOwner, f.target.peer.currentSession().initialReplay.Load())
			require.NoError(t, err)
			err = f.target.api.RelayStoredRouteBound(t.Context(), f.target.peer.settings.Address, []rpc.StoredRoute{f.route}, wrong, f.target.owner)
			require.Error(t, err)
			require.Len(t, updateBodies(t, f.target.conn.written()), 1, "wrong tuple cannot touch the useful original socket")
		})
	}
}

// TestBoundStoredRelayReleasedReceiptRetiresOnlyOriginalPhysical sends an exact
// old source, releases its actual Session receipt, and withdraws its acknowledged
// original SENT slot. A fresh physical source then survives the same old request.
func TestBoundStoredRelayReleasedReceiptRetiresOnlyOriginalPhysical(t *testing.T) {
	for _, addPath := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "path-zero"}[addPath], func(t *testing.T) {
			f := newBoundRelayWireFixture(t, addPath)
			require.NoError(t, f.relay(t, f.route))
			old := f.route
			old.Withdraw = true
			old.SourceReceipt = 0
			f.session.closeSourceReceipt()
			require.False(t, sourceReceipts.recoverySource(f.route.SourceReceipt, f.route.SourceOwner).ok)
			require.NoError(t, f.relay(t, old))
			bodies := updateBodies(t, f.target.conn.written())
			require.Len(t, bodies, 2)
			require.Empty(t, f.target.peer.adjOut.routes, "exact original current SENT ownership is retired")
			successor := sourceSessionFixture(t, f.source)
			successor.setRecvCtxID(f.session.recvCtxID)
			require.NoError(t, successor.captureSourceReceipt(f.source, &sourceReceipts))
			t.Cleanup(successor.closeSourceReceipt)
			fresh := f.route
			fresh.SourceConnectionID = successor.connectionID
			fresh.SourceReceipt = successor.sourceReceiptID()
			fresh.MsgID++
			require.NoError(t, f.relay(t, fresh))
			require.Len(t, updateBodies(t, f.target.conn.written()), 3)
			require.NoError(t, f.relay(t, old))
			require.Len(t, updateBodies(t, f.target.conn.written()), 3, "old physical removal never withdraws a successor")
			require.Len(t, f.target.peer.adjOut.routes, 1)
			for _, entry := range f.target.peer.adjOut.routes {
				require.Equal(t, successor.connectionID, entry.owner.connectionID)
			}
			denied := fresh
			denied.SourceReceipt = 0
			require.Error(t, f.relay(t, denied), "receipt-free owned announcements never become manual")
		})
	}
}

// TestBoundCachedRelayWaitsForExactDestination uses the actual bound cache
// entry, then changes the target's publication token. The old tuple cannot
// follow its address into that successor and never reports queued success.
func TestBoundCachedRelayWaitsForExactDestination(t *testing.T) {
	f := newBoundRelayWireFixture(t, false)
	source, err := f.target.api.originalRelayFacts(f.source.settings.Address, &f.route, f.sender)
	require.NoError(t, err)
	var spans []relayAttrSpan
	update, id, _, err := f.target.api.buildRelayUpdate([]rpc.StoredRoute{f.route}, &source, &spans)
	require.NoError(t, err)
	defer f.target.api.r.recentUpdates.Release(id)
	require.Equal(t, f.session.sourceReceiptID(), update.SourceReceipt)
	destinations := []rpc.ForwardTarget{{Destination: f.target.peer.settings.PeerKey().String(), DestinationOwner: f.target.peer.sourceOwner, DestinationSession: f.target.peer.currentSession().initialReplay.Load()}}
	require.NoError(t, f.target.api.ForwardUpdatesBound(t.Context(), []uint64{id}, destinations, "", f.target.owner.Sender(), f.target.owner))
	require.Len(t, updateBodies(t, f.target.conn.written()), 1)
	f.target.peer.currentSession().initialReplay.Add(1)
	require.Error(t, f.target.api.ForwardUpdatesBound(t.Context(), []uint64{id}, destinations, "", f.target.owner.Sender(), f.target.owner))
	require.Len(t, updateBodies(t, f.target.conn.written()), 1)
	// Raw diagnostic/manual bytes remain an explicitly separate send contract.
}
