// Design: docs/architecture/behavior/peer-lifecycle.md -- live DelayOpen retry scheduling.
// Related: delay_open_test.go -- startDelayOpenPeer drives the real passive Peer.
// Related: peer_run_backoff_test.go -- backoffClock records and releases actual Peer waits.
// RFC 4271 Section 8.2.2 -- see rfc/short/rfc4271.md.
package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestRFC4271ActiveDelayOpenFailureRetriesAtInitialValue closes a real held
// socket, reads the Peer's actual requested wait, then releases that wait and
// establishes a replacement. A timer-start count alone cannot pass this test.
// The nonzero counter distinguishes increment from assigning a fixed value.
// MUTATION: discard the Event 18 retry deadline before the old session retires.
//
// RFC requirement: RFC4271-8.2.2-13 positive -- A transport failure on a real passive Peer holding its socket in Active with DelayOpen running moves the old FSM to Idle, clears DelayOpen, releases its socket and published Session, increments a nonzero ConnectRetryCounter once, and schedules the actual Peer retry within the jittered initial ConnectRetryTime interval; releasing that wait creates a distinct session that completes OPEN and KEEPALIVE exchange and reaches Established without resetting the counter.
func TestRFC4271ActiveDelayOpenFailureRetriesAtInitialValue(t *testing.T) {
	backoff := newBackoffClock()
	peer, old, far, _ := startDelayOpenPeer(t, true, 240*time.Second, func(peer *Peer) {
		peer.SetClock(backoff)
	})
	require.Equal(t, fsm.StateActive, old.State(), "held socket begins in Active")
	require.True(t, old.timers.IsDelayOpenTimerRunning())
	peer.connectRetryCounter.Increment()
	peer.connectRetryCounter.Increment()
	before := peer.ConnectRetryCounter()
	closeConnQuietly(far)
	wait := backoff.awaitWait(t)
	require.Nil(t, peer.currentSession(), "the failed connection is released before retry")
	require.Nil(t, old.Conn())
	require.False(t, old.timers.IsDelayOpenTimerRunning())
	require.Equal(t, fsm.StateIdle, old.State())
	require.Equal(t, before+1, peer.ConnectRetryCounter())
	// RFC 4271 Section 8.2.2 (Active): "restarts the ConnectRetryTimer (with the initial value),"
	// Section 10 permits jitter between 0.75 and 1.0 of that initial value.
	initial := fsm.DefaultConnectRetryTime
	require.GreaterOrEqual(t, wait, initial-initial/4, "retry must not run before the initial-value interval")
	require.LessOrEqual(t, wait, initial, "Active failure must use ConnectRetryTime, not independent backoff")
	backoff.release()
	delayOpenRetryEstablishReplacement(t, peer, old)
	require.Equal(t, before+1, peer.ConnectRetryCounter())
}

// TestRFC4271OpenSentFailureKeepsOrdinaryBackoff disables DelayOpen and closes
// the socket only after TCP success has sent OPEN. The failure then belongs
// to OpenSent, outside Active's counter and initial-value retry clauses.
// MUTATION: apply the initial-value retry to the ordinary backoff branch.
//
// RFC requirement: RFC4271-8.2.2-13 negative -- With DelayOpen disabled a real passive Peer sends OPEN and reaches OpenSent before its transport fails; that failure leaves the old FSM in Active, clears its socket and published Session, preserves a nonzero ConnectRetryCounter, and retains the configured ordinary jittered backoff instead of Active Event 18's initial retry; releasing that wait permits a distinct replacement to establish without resetting the counter.
func TestRFC4271OpenSentFailureKeepsOrdinaryBackoff(t *testing.T) {
	backoff := newBackoffClock()
	peer, old, far, _ := startDelayOpenPeer(t, true, 0, func(peer *Peer) {
		peer.SetClock(backoff)
	})
	require.Equal(t, fsm.StateOpenSent, old.State())
	peer.connectRetryCounter.Increment()
	peer.connectRetryCounter.Increment()
	before := peer.ConnectRetryCounter()
	closeConnQuietly(far)
	wait := backoff.awaitWait(t)
	require.Nil(t, peer.currentSession(), "the failed connection is released before retry")
	require.Nil(t, old.Conn())
	require.False(t, old.timers.IsDelayOpenTimerRunning())
	require.Equal(t, fsm.StateActive, old.State())
	require.Equal(t, before, peer.ConnectRetryCounter())
	// startDelayOpenPeer fixes ordinary backoff at one hour.
	initial := time.Hour
	require.GreaterOrEqual(t, wait, initial-initial/4, "ordinary backoff must retain its configured interval")
	require.LessOrEqual(t, wait, initial, "ordinary backoff remains within its jittered interval")
	backoff.release()
	delayOpenRetryEstablishReplacement(t, peer, old)
	require.Equal(t, before, peer.ConnectRetryCounter())
}

// delayOpenRetryEstablishReplacement completes the original retry proof's
// useful handshake after the test releases its recorded Peer wait.
func delayOpenRetryEstablishReplacement(t *testing.T, peer *Peer, old *Session) {
	t.Helper()
	delayOpenAwait(t, func() bool {
		next := peer.currentSession()
		if next == nil {
			return false
		}
		if next == old {
			return false
		}
		return next.State() == fsm.StateActive
	})
	next := peer.currentSession()
	require.NotNil(t, next)
	remote, local := collisionProofTCPPair(t)
	require.NoError(t, peer.acceptConnection(local))
	delayOpenWrite(t, remote, delayOpenRemote(0x02020202))
	delayOpenReadType(t, remote, 1)
	delayOpenReadType(t, remote, 4)
	delayOpenWrite(t, remote, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return next.State() == fsm.StateEstablished })
}

// TestDelayOpenConnectFailureRetriesAtInitialValue drops a real outbound socket
// while OPEN is held. The replacement must use the restarted initial-value
// retry and complete its own handshake; the failed Connect attempt is not
// counted as an Active failure.
func TestDelayOpenConnectFailureRetriesAtInitialValue(t *testing.T) {
	n, backoff := startBackoffNeighbor(t, time.Hour, func(settings *PeerSettings) {
		settings.DelayOpen = 240 * time.Second
	})
	require.NoError(t, n.listener.SetDeadline(time.Now().Add(5*time.Second)))
	far, err := n.listener.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { closeConnQuietly(far) })
	var old *Session
	delayOpenAwait(t, func() bool {
		old = n.peer.currentSession()
		if old == nil {
			return false
		}
		return old.timers.IsDelayOpenTimerRunning()
	})
	require.Equal(t, fsm.StateConnect, old.State())
	closeConnQuietly(far)
	wait := backoff.awaitWait(t)
	require.Nil(t, n.peer.currentSession(), "failed connection released before retry")
	require.Nil(t, old.Conn())
	require.Equal(t, fsm.StateActive, old.State())
	require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())
	require.False(t, old.timers.IsDelayOpenTimerRunning())
	requireBackoffBase(t, wait, fsm.DefaultConnectRetryTime, "held Connect failure uses initial-value retry")
	backoff.release()

	// Actual dial and remote OPEN release the replacement's DelayOpen wait.
	require.NoError(t, n.listener.SetDeadline(time.Now().Add(5*time.Second)))
	replacement, err := n.listener.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { closeConnQuietly(replacement) })
	delayOpenWrite(t, replacement, delayOpenRemote(0x02020202))
	delayOpenReadType(t, replacement, 1)
	delayOpenReadType(t, replacement, 4)
	delayOpenWrite(t, replacement, &message.Keepalive{})
	delayOpenAwait(t, func() bool {
		next := n.peer.currentSession()
		if next == nil {
			return false
		}
		if next == old {
			return false
		}
		return next.State() == fsm.StateEstablished
	})
}
