// Design: docs/architecture/update-building.md -- registry retirement across a peer's opQueue.
// Related: peer_initial_sync.go -- both actual operation-queue drains.
// Related: origin_runtime_direct_retirement_test.go -- genuine accepted-entry timer and SDK fixture.
package reactor

import (
	"bufio"
	"bytes"
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// replacementOriginSession supplies a fresh FSM-established recording Session.
// The real Peer publisher numbers and exposes it below. No claim of a TCP BGP
// handshake is made: the provider SDK registration/restart is a real connection.
func replacementOriginSession(t *testing.T, f *originRuntimeSDKFixture) *Session {
	t.Helper()
	peer := f.writer.peer
	old := peer.currentSession()
	session := NewSession(peer.settings)
	session.negotiated = old.negotiated
	session.sendCtxID = old.sendCtxID
	session.adjOut = &peer.adjOut
	session.egressFinalAccepts = old.egressFinalAccepts
	session.egressRouteFilter = peer.egressGate(session)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, session.fsm.State())
	connection := &recordingConn{}
	session.mu.Lock()
	session.conn = connection
	session.bufWriter = bufio.NewWriterSize(connection, 4096)
	session.mu.Unlock()
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	f.writer.conn = connection
	return session
}

// MUTATION: dropping registry retirement at the actual opQueue drain lets old queued A withdraw selected B.
func TestOriginRuntimeDirectRegistryOpQueueCannotWithdrawSelectedLocal(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			for _, closing := range []bool{false, true} {
				t.Run(map[bool]string{false: "initial-sync-drain", true: "closing-drain"}[closing], func(t *testing.T) {
					var intake atomic.Bool
					var destinationSession atomic.Uint64
					destinationSession.Store(71)
					f := newOriginRuntimeSDKFixtureWithIntakeSession(t, typed, intake.Load, destinationSession.Load)
					peer := f.writer.peer
					oldSession := peer.currentSession()
					// Give this fixture's original Session its actual production
					// receipt before A; the next publisher must number above it.
					peer.enterEstablished(oldSession)
					require.False(t, peer.drainAndCloseQueueGate(oldSession, peer.addrString, 4096))
					destinationSession.Store(oldSession.initialReplay.Load())
					peer.refreshForwardFacts()
					originalProvider, supported, err := f.host.OriginIntakeOwner(f.name)
					require.NoError(t, err)
					require.False(t, supported)
					require.NoError(t, originalProvider.Current())
					stale := beginDirectRegistryExpiry(t, f, f.batch)
					oldWire := bytes.Clone(f.writer.conn.written())
					bodies := updateBodies(t, oldWire)
					require.Len(t, bodies, 1)
					require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[0], false))
					// The queued command is produced while down, never inserted
					// into opQueue or converted into a forwarded item by the test.
					oldSession.seal()
					oldSession.closeConnWithin(0)
					peer.setState(PeerStateConnecting)
					stale.finish(t, f.ctx)
					direct := f.batch
					direct.NLRIs = []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.18.9.0/24"), 0, false)}
					useful := beginDirectRegistryExpiry(t, f, direct)
					useful.finish(t, f.ctx)
					peer.mu.RLock()
					queued := len(peer.opQueue)
					kinds := make([]PeerOpType, 0, queued)
					for _, op := range peer.opQueue {
						kinds = append(kinds, op.Type)
					}
					peer.mu.RUnlock()
					require.Equal(t, 3, queued)
					require.Equal(t, []PeerOpType{PeerOpWithdraw, PeerOpAnnounce, PeerOpWithdraw}, kinds)
					require.Equal(t, oldWire, f.writer.conn.written(), "down-peer API queueing must emit nothing")
					originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 0, 0, 0, 0)
					session := replacementOriginSession(t, f)
					peer.enterEstablished(session)
					destinationSession.Store(session.initialReplay.Load())
					require.Greater(t, session.initialReplay.Load(), oldSession.initialReplay.Load())
					peer.refreshForwardFacts()
					peer.announceEstablished(f.writer.api.r, session)
					require.True(t, peer.shouldQueue(), "Established is published before initial drain opens the gate")
					intake.Store(true)
					response, err := f.host.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: f.host}, "request plugin restart "+f.name)
					require.NoError(t, err)
					require.Equal(t, plugin.StatusDone, response.Status, response)
					require.Error(t, originalProvider.Current())
					provider, supported, err := f.host.OriginIntakeOwner(f.name)
					require.NoError(t, err)
					require.True(t, supported)
					require.NoError(t, provider.Current())
					selected := f.batch
					selected.NextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.253"))
					result, _ := originRegistryAnnounce(t, f, selected, plugin.SelectedOwner{})
					require.Equal(t, rpc.RouteAdmissionSelected, result.Admissions[0].Disposition)
					store := f.writer.api.r.originReceipts
					store.mu.Lock()
					original := store.receipts[result.Admissions[0].Generation]
					store.mu.Unlock()
					require.NotNil(t, original)
					before := bytes.Clone(f.writer.conn.written())
					bodies = updateBodies(t, before)
					require.Len(t, bodies, 1, "B must already be on the new wire before either actual opQueue drain")
					hop, present := bodyPathAttr(t, bodies[0], 3)
					require.True(t, present)
					require.Equal(t, []byte{192, 0, 2, 253}, hop)
					if closing {
						require.False(t, peer.drainAndCloseQueueGate(session, peer.addrString, 4096))
					} else {
						peer.sendInitialRoutes(session)
					}
					bodies = updateBodies(t, f.writer.conn.written())
					expected := 4
					if closing {
						expected = 3
					}
					require.Len(t, bodies, expected, "only B, useful direct C announce/withdraw, and the main drain's EOR may leave")
					for _, body := range bodies {
						require.NotContains(t, updatePrefixes(t, body, true), netip.MustParsePrefix("198.18.7.0/24"), "stale A must never withdraw selected B, even before a replay")
					}
					require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, bodies[1], false))
					require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, bodies[2], true))
					require.Same(t, original, originRuntimeCurrent(t, store, original.key))
					require.NoError(t, original.ctx.Err())
					originRuntimeExpectCounts(t, store, 1, 1, 0, 0)
					peer.mu.RLock()
					queued = len(peer.opQueue)
					peer.mu.RUnlock()
					require.Zero(t, queued)
					require.False(t, peer.shouldQueue())
					peer.adjOut.mu.Lock()
					slots, routes := len(peer.adjOut.selectedPaths), len(peer.adjOut.routes)
					peer.adjOut.mu.Unlock()
					require.Equal(t, 1, slots)
					require.Equal(t, 1, routes)
					require.NoError(t, f.writer.api.WithdrawNLRIBatch(f.ctx, selector.ParseDefault(peer.addrString), selected, f.sender))
					after := updateBodies(t, f.writer.conn.written())
					require.Len(t, after, len(bodies)+1)
					require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, after[len(bodies)], true), "explicit manual withdrawal must remain effective")
					originRuntimeExpectCounts(t, store, 0, 0, 0, 0)
					peer.adjOut.mu.Lock()
					slots, routes = len(peer.adjOut.selectedPaths), len(peer.adjOut.routes)
					peer.adjOut.mu.Unlock()
					require.Zero(t, slots)
					require.Zero(t, routes)
				})
			}
		})
	}
}
