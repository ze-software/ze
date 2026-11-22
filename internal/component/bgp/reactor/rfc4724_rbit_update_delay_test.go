// Design: docs/architecture/behavior/peer-lifecycle.md — the startup convergence hold
// RFC: rfc/short/rfc4724.md — Section 3, a peer with the Restart State bit set is not waited for
// Related: update_delay.go — updateDelayHold.trackLocked, the one place Ze waits for a peer's End-of-RIB

package reactor

import (
	"context"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
)

// rbitHoldMaxDelay is the update-delay outer bound both tests arm. The fake
// clock is never advanced, so the hold can only release on convergence: a
// release seen by either test is the convergence test's answer, never the timer.
const rbitHoldMaxDelay = 60 * time.Second

// rbitHeldPeer builds an Established peer attached to r, writing into the
// returned recordingConn, whose negotiated capabilities come from the
// production converter (NewNegotiatedCapabilities over capability.Negotiate):
// IPv4 unicast, and Graceful Restart with the peer's Restart State bit as
// restartState. default-originate for IPv4 unicast gives its initial routing
// update one route, 0.0.0.0/0, ahead of the End-of-RIB. The peer has received
// no End-of-RIB.
func rbitHeldPeer(t *testing.T, r *Reactor, restartState bool) (*Peer, *recordingConn) {
	t.Helper()
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	peer.reactor = r
	peer.settings.LocalAddress = netip.MustParseAddr("10.0.0.1")
	peer.settings.DefaultOriginate = map[string]bool{"ipv4/unicast": true}

	localCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: 65000},
		&capability.GracefulRestart{RestartTime: 120},
	}
	remoteCaps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: 65001},
		&capability.GracefulRestart{
			RestartState: restartState,
			RestartTime:  120,
			Families: []capability.GracefulRestartFamily{{
				AFI:  capability.AFIIPv4,
				SAFI: capability.SAFIUnicast,
			}},
		},
	}
	neg := capability.Negotiate(localCaps, remoteCaps, capability.PeerIdentity{
		LocalASN: 65000,
		PeerASN:  65001,
	})
	require.NotNil(t, neg.GracefulRestart, "the peer must have negotiated Graceful Restart")
	require.Equal(t, restartState, neg.GracefulRestart.RestartState)

	session := peer.testInitialSync()
	session.mu.Lock()
	session.negotiated = neg
	session.mu.Unlock()
	peer.negotiated.Store(NewNegotiatedCapabilities(neg))
	require.Zero(t, peer.Stats().EORReceived, "the peer must have sent no End-of-RIB")
	return peer, conn
}

// rbitUpdateNLRI returns the IPv4 NLRI field of one UPDATE message: what
// follows the Withdrawn Routes and Path Attributes fields (RFC 4271 Section 4.3).
func rbitUpdateNLRI(t *testing.T, msg []byte) []byte {
	t.Helper()
	require.Greater(t, len(msg), message.HeaderLen)
	require.Equal(t, byte(msgtype.TypeUPDATE), msg[message.HeaderLen-1], "the message must be an UPDATE")
	body := msg[message.HeaderLen:]
	require.GreaterOrEqual(t, len(body), 4)
	withdrawn := int(binary.BigEndian.Uint16(body[0:2]))
	require.GreaterOrEqual(t, len(body), 4+withdrawn)
	attributes := int(binary.BigEndian.Uint16(body[2+withdrawn : 4+withdrawn]))
	require.GreaterOrEqual(t, len(body), 4+withdrawn+attributes)
	return body[4+withdrawn+attributes:]
}

// requireRBitInitialUpdate asserts the peer's wire holds exactly its initial
// routing update: the 0.0.0.0/0 UPDATE, then the IPv4 unicast End-of-RIB.
func requireRBitInitialUpdate(t *testing.T, peer *Peer, conn *recordingConn) {
	t.Helper()
	msgs := splitWireMessages(t, conn.written())
	require.Len(t, msgs, 2, "the initial routing update must be one route UPDATE then the End-of-RIB")
	require.Equal(t, []byte{0x00}, rbitUpdateNLRI(t, msgs[0]), "the first message must advertise 0.0.0.0/0")
	require.Equal(t, eorWire(family.IPv4Unicast), msgs[1], "the second message must be the End-of-RIB")
	require.Equal(t, uint32(1), peer.Stats().EORSent)
}

// TestRFC4724RestartStateBitPeerIsNotWaitedForUnderUpdateDelay drives the one
// place Ze waits for a peer's End-of-RIB before advertising to it: the
// update-delay hold, through Peer.startInitialRoutes, the door the FSM
// Established branch calls.
//
// RFC 4724 Section 3: "When set (value 1), this bit indicates that the BGP
// speaker has restarted, and its peer MUST NOT wait for the End-of-RIB marker
// from the speaker before advertising routing information to the speaker."
//
// VALIDATES: with the hold armed (max-delay 60s, the clock never advanced) and
// one expected peer that negotiated Graceful Restart with the Restart State bit
// set and has sent no End-of-RIB, startInitialRoutes releases the hold as
// converged and the peer's wire receives the 0.0.0.0/0 UPDATE and the IPv4
// unicast End-of-RIB.
// PREVENTS: the hold counting an R=1 peer's End-of-RIB as owed, which holds
// both restarting speakers until max-delay, the deadlock the bit exists to
// avoid.
// MUTATION: dropping `|| nc.GracefulRestart.RestartState` from
// updateDelayHold.trackLocked keeps the peer held, nothing reaches the wire,
// and awaitInitialSyncDone fails.
//
// RFC requirement: RFC4724-3-5 positive -- with update-delay armed and the only expected peer having negotiated Graceful Restart with the Restart State bit set and sent no End-of-RIB, Peer.startInitialRoutes releases the hold as converged without the max-delay timer and the 0.0.0.0/0 UPDATE then the IPv4 unicast End-of-RIB are written to that peer.
func TestRFC4724RestartStateBitPeerIsNotWaitedForUnderUpdateDelay(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	peer, conn := rbitHeldPeer(t, r, true)
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: rbitHoldMaxDelay}, []*Peer{peer}))

	session := peer.testInitialSync()
	peer.startInitialRoutes(session)

	require.False(t, updateDelayHolding(r),
		"a peer with the Restart State bit set must not be waited for: the hold must release at once")
	require.Equal(t, updateDelayConverged.String(), updateDelayReason(r))

	awaitInitialSyncDone(t, session)
	requireRBitInitialUpdate(t, peer, conn)
	require.Zero(t, peer.Stats().EORReceived, "the routes must have left with no End-of-RIB from the peer")
}

// TestRFC4724RestartStateBitClearPeerIsWaitedForUnderUpdateDelay is the case
// just outside the clause: the same hold and the same peer, with the Restart
// State bit clear. RFC 4724 Section 4.1 then defers until the peer's End-of-RIB
// arrives, so Ze advertises nothing to it until then.
//
// VALIDATES: startInitialRoutes leaves the hold holding, starts no initial sync
// for the session (its start latch is still not started, and the claim is made
// on the calling goroutine, so nothing can be written behind the check), and
// writes nothing to the peer. The peer's IPv4 unicast End-of-RIB
// (updateDelayEndOfRIB, the inbound decode's entry point) then releases the hold
// as converged, and the 0.0.0.0/0 UPDATE and the End-of-RIB are written.
// PREVENTS: a hold that settles every Graceful Restart peer on Established,
// which the positive test alone would pass.
// MUTATION: settling every peer in updateDelayHold.trackLocked (an early
// `record.settled = true; return`) releases the hold on Established and the
// wire is no longer empty before the End-of-RIB.
//
// RFC requirement: RFC4724-3-5 negative -- with update-delay armed and the only expected peer having negotiated Graceful Restart with the Restart State bit clear and sent no End-of-RIB, Peer.startInitialRoutes keeps the hold, starts no initial sync and writes nothing to that peer; its IPv4 unicast End-of-RIB then releases the hold as converged and the 0.0.0.0/0 UPDATE then the End-of-RIB are written.
func TestRFC4724RestartStateBitClearPeerIsWaitedForUnderUpdateDelay(t *testing.T) {
	r := &Reactor{}
	clk := updateDelayClock()
	peer, conn := rbitHeldPeer(t, r, false)
	require.True(t, r.updateDelay.arm(context.Background(), clk,
		UpdateDelay{MaxDelay: rbitHoldMaxDelay}, []*Peer{peer}))

	session := peer.testInitialSync()
	peer.startInitialRoutes(session)

	require.True(t, updateDelayHolding(r),
		"a peer with the Restart State bit clear owes its End-of-RIB: the hold must keep its initial update")
	require.Equal(t, syncLatchNotStarted, session.initialSyncStart.Load(),
		"no initial sync may start while the peer's End-of-RIB is owed")
	require.Empty(t, conn.written(), "nothing may be advertised before the peer's End-of-RIB")
	require.Zero(t, peer.Stats().EORSent)

	peer.updateDelayEndOfRIB(family.IPv4Unicast)

	require.False(t, updateDelayHolding(r), "the peer's End-of-RIB must release the hold")
	require.Equal(t, updateDelayConverged.String(), updateDelayReason(r))

	awaitInitialSyncDone(t, session)
	requireRBitInitialUpdate(t, peer, conn)
}
