package l2tp

// Design: docs/architecture/wire/l2tp.md -- reliable delivery and teardown
//
// RFC 2661 Section 5.8: "If no peer response is detected after several
// retransmissions (a recommended default is 5, but SHOULD be configurable),
// the tunnel and all sessions within MUST be cleared."
// Related: reactor.go (handleTick), reliable.go (Tick).

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC2661-5.8-3 positive -- when the HELLO sent on an
// established tunnel with one established session goes unanswered through
// every retransmission, the tick that exhausts the budget sends a StopCCN,
// removes the session and leaves the tunnel closed.
// RFC requirement: RFC2661-5.8-3 negative -- on every tick before the budget
// is exhausted the tunnel stays established and keeps its session.
//
// VALIDATES: retransmit exhaustion clears the tunnel, not only its routes.
// PREVENTS: a reactor that withdraws the session route but keeps the tunnel.
//
// TestRFC2661RetransmitExhaustionClearsTunnel disables dead-peer detection so
// the retransmit budget is the only teardown trigger, then ticks the reactor
// once past each retransmission deadline.
func TestRFC2661RetransmitExhaustionClearsTunnel(t *testing.T) {
	clk := newTestClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	spy := &recordingRouteObserver{}
	ln, r, logs, stop := buildLogReactorWithClockObserver(t, clk, 60*time.Second, "", spy)
	defer stop()
	r.setHelloRetries(0)

	client, localTID := driveToEstablished(t, ln, "")
	defer client.Close()
	waitForLog(t, logs, "tunnel now established")

	const sid uint16 = 0x4242
	r.tunnelsMu.Lock()
	tunnel := r.tunnelsByLocalID[localTID]
	tunnel.sessions = map[uint16]*L2TPSession{
		sid: {localSID: sid, remoteSID: 0x4243, state: L2TPSessionEstablished, createdAt: clk.now()},
	}
	tunnel.lastActivity = clk.now().Add(-70 * time.Second)
	r.tunnelsMu.Unlock()

	tunnelState := func() (L2TPTunnelState, int) {
		r.tunnelsMu.Lock()
		defer r.tunnelsMu.Unlock()
		return tunnel.state, tunnel.sessionCount()
	}

	// The first tick sends the HELLO; the peer never acknowledges it.
	clk.add(65 * time.Second)
	r.handleTick(tickReq{tunnelID: localTID})

	retransmitTimeout := DefaultRTimeout
	for attempt := range DefaultMaxRetransmit {
		clk.add(retransmitTimeout + time.Second)
		retransmitTimeout = min(retransmitTimeout*2, DefaultRTimeoutCap)
		r.handleTick(tickReq{tunnelID: localTID})
		state, sessions := tunnelState()
		require.Equal(t, L2TPTunnelEstablished, state, "tunnel cleared at retransmission %d, before the budget ran out", attempt+1)
		require.Equal(t, 1, sessions, "session cleared at retransmission %d, before the budget ran out", attempt+1)
	}

	clk.add(retransmitTimeout + time.Second)
	r.handleTick(tickReq{tunnelID: localTID})
	state, sessions := tunnelState()
	require.Equal(t, L2TPTunnelClosed, state, "tunnel not cleared after the retransmit budget ran out")
	require.Equal(t, 0, sessions, "session survived the retransmit exhaustion")
	waitForLog(t, logs, "StopCCN sent; tunnel closed")
}
