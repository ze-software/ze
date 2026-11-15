// Design: docs/architecture/behavior/fsm-established.md -- outbound UPDATE timing.
// Related: delay_open_test.go -- real Peer.run, socket and timer-clock fixture.
// Related: session_write.go -- SendUpdate and the destination writer.
// RFC naming: untagged -- draft nonzero-default MRAI probes; no adopted default or passing discrimination proof exists.
//
// These are policy candidates, not conformance claims. RFC 4271 Section 10
// suggests a 30-second EBGP MRAI and jitter in [0.75, 1.0]. It does not require
// that default, and zero is a valid configured interval. These probes propose
// that default and an immediately eligible first advertisement. They must not
// turn the RFC4271-9.2.1.1-2/-3 gap rows into proven requirements. Before they
// become requirement tests, bind them to an explicitly selected nonzero
// interval through the adopted configuration and prove the producer mutations.

package reactor

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/test/sim"
)

const (
	mraiCandidateBase = 30 * time.Second
	mraiCandidateMin  = 3 * mraiCandidateBase / 4
)

// mraiCandidatePeer uses an actual Peer lifecycle and TCP handshake. Zero hold
// time isolates UPDATE pacing from keepalive, hold and SendHold expirations.
// The Peer owns and joins the session through startDelayOpenPeer's cleanup.
func mraiCandidatePeer(t *testing.T) (*Peer, net.Conn, *sim.FakeClock) {
	t.Helper()
	clock := sim.NewFakeClock(time.Now())
	peer, session, far, _ := startDelayOpenPeer(t, false, 0, func(p *Peer) {
		p.SetClock(clock)
		p.settings.ReceiveHoldTime = 0
		acceptingPolicies(t, p.settings)
	})
	// startDelayOpenPeer replaces the protocol-timer clock before the dial.
	// Reunite it with the session's clock before negotiating or sending routes.
	session.timers.SetClock(clock)
	remote := delayOpenRemote(0x02020202)
	remote.HoldTime = 0
	// RFC 4271 Sections 4.2 and 8.2.2: establish through real wire messages.
	delayOpenWrite(t, far, remote)
	delayOpenReadType(t, far, 4)
	delayOpenWrite(t, far, message.NewKeepalive())
	delayOpenAwait(t, func() bool {
		return session.State() == fsm.StateEstablished &&
			peer.State() == PeerStateEstablished && !peer.pendingSync()
	})
	return peer, far, clock
}

func mraiCandidateAnnouncement(nextHop byte) *message.Update {
	return &message.Update{
		PathAttributes: []byte{
			0x40, 1, 1, 0, // ORIGIN IGP.
			0x40, 2, 4, 2, 1, 0xfd, 0xe8, // AS_SEQUENCE 65000 (two-octet session).
			0x40, 3, 4, 192, 0, 2, nextHop,
		},
		NLRI: []byte{24, 203, 0, 113},
	}
}

// mraiCandidateReadRoute skips only the initial empty IPv4 End-of-RIB. Every
// other packet is an assertion: a NOTIFICATION cannot masquerade as no route.
// within bounds observation, not simulated protocol time.
func mraiCandidateReadRoute(t *testing.T, far net.Conn, within time.Duration) ([]byte, bool) {
	t.Helper()
	require.NoError(t, far.SetReadDeadline(time.Now().Add(within)))
	defer func() { require.NoError(t, far.SetReadDeadline(time.Time{})) }()
	for range 4 {
		packet, err := core4271ReadMessage(far)
		if err != nil {
			if timeout, ok := errors.AsType[net.Error](err); ok {
				if timeout.Timeout() {
					return nil, false
				}
			}
			t.Fatalf("MRAI wire observation failed: %v", err)
		}
		require.Equal(t, byte(2), packet[18], "MRAI observation must stay on an Established UPDATE stream")
		if len(packet) == 23 {
			require.Equal(t, []byte{0, 0, 0, 0}, packet[19:], "only an empty End-of-RIB may be skipped")
			continue
		}
		return packet, true
	}
	t.Fatal("unexpected repeated End-of-RIB packets")
	return nil, false
}

func mraiCandidateRequireRoute(t *testing.T, far net.Conn, update *message.Update) {
	t.Helper()
	packet, ok := mraiCandidateReadRoute(t, far, 5*time.Second)
	require.True(t, ok, "the selected route must reach the real destination socket")
	require.Equal(t, message.PackTo(update, nil), packet)
}

func mraiCandidateRequireQuiet(t *testing.T, far net.Conn) {
	t.Helper()
	packet, ok := mraiCandidateReadRoute(t, far, 100*time.Millisecond)
	require.False(t, ok, "nonzero-default MRAI candidate emitted an UPDATE before eligibility: %x", packet)
}

// VALIDATES: candidate coverage for RFC 4271 Section 9.2.1.1 spacing, given
// Section 10's suggested EBGP default. Announcements and withdrawals share the
// same destination bound. No assertion requires another prefix to send early.
// Pending proof: with an adopted nonzero interval, bypass its eligibility guard
// and observe this unit go red after its unmodified baseline passes.
func TestRFC4271MRAIDefaultCandidateCommonDestinationSpacing(t *testing.T) {
	for _, withdrawal := range []bool{false, true} {
		name := "announcement"
		if withdrawal {
			name = "withdrawal"
		}
		t.Run(name, func(t *testing.T) {
			peer, far, clock := mraiCandidatePeer(t)
			first := mraiCandidateAnnouncement(11)
			require.NoError(t, peer.SendUpdate(first))
			mraiCandidateRequireRoute(t, far, first)
			next := mraiCandidateAnnouncement(12)
			if withdrawal {
				next = &message.Update{WithdrawnRoutes: []byte{24, 203, 0, 113}}
			}
			require.NoError(t, peer.SendUpdate(next))
			clock.Add(mraiCandidateMin - time.Nanosecond)
			mraiCandidateRequireQuiet(t, far)
			clock.Add(mraiCandidateBase - mraiCandidateMin + time.Nanosecond)
			mraiCandidateRequireRoute(t, far, next)
		})
	}
}

// VALIDATES: candidate coverage for RFC4271-9.2.1.1-3: "If new routes are
// selected multiple times while awaiting the expiration of
// MinRouteAdvertisementIntervalTimer, the last route selected SHALL be
// advertised at the end of MinRouteAdvertisementIntervalTimer."
// The writer receives already-selected replacements; this does not test the
// RIB's selection algorithm. Pending proof: retain the first pending update in
// place of replacing it, and observe the latest-route wire assertion go red.
func TestRFC4271MRAIDefaultCandidateCoalescesLatestSelected(t *testing.T) {
	peer, far, clock := mraiCandidatePeer(t)
	first := mraiCandidateAnnouncement(11)
	require.NoError(t, peer.SendUpdate(first))
	mraiCandidateRequireRoute(t, far, first)
	require.NoError(t, peer.SendUpdate(mraiCandidateAnnouncement(12)))
	clock.Add(time.Second)
	latest := mraiCandidateAnnouncement(13)
	require.NoError(t, peer.SendUpdate(latest))
	clock.Add(mraiCandidateMin - time.Second - time.Nanosecond)
	mraiCandidateRequireQuiet(t, far)
	clock.Add(mraiCandidateBase - mraiCandidateMin + time.Nanosecond)
	mraiCandidateRequireRoute(t, far, latest)
	mraiCandidateRequireQuiet(t, far)
}

// VALIDATES: candidate coverage for RFC4271-9.2.1.1-2/-3: a withdrawal replaces
// the pending announcement for the same destination, reaches the socket, and
// cannot be followed by resurrection of the canceled selected route.
// Pending proof: leave the pending announcement armed after withdrawing it;
// the exact withdrawal and later silence assertions must detect the stale send.
func TestRFC4271MRAIDefaultCandidateWithdrawCancelsPendingAnnouncement(t *testing.T) {
	peer, far, clock := mraiCandidatePeer(t)
	first := mraiCandidateAnnouncement(11)
	require.NoError(t, peer.SendUpdate(first))
	mraiCandidateRequireRoute(t, far, first)
	require.NoError(t, peer.SendUpdate(mraiCandidateAnnouncement(12)))
	withdrawal := &message.Update{WithdrawnRoutes: []byte{24, 203, 0, 113}}
	require.NoError(t, peer.SendUpdate(withdrawal))
	clock.Add(mraiCandidateMin - time.Nanosecond)
	mraiCandidateRequireQuiet(t, far)
	clock.Add(mraiCandidateBase - mraiCandidateMin + time.Nanosecond)
	mraiCandidateRequireRoute(t, far, withdrawal)
	clock.Add(mraiCandidateBase)
	mraiCandidateRequireQuiet(t, far)
}
