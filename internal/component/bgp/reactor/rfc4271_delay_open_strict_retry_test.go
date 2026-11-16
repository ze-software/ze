// Design: docs/architecture/behavior/peer-lifecycle.md -- Active retry after strict DelayOpen.
// Related: rfc4271_delay_open_retry_test.go -- actual Peer retry interval and opposite control.
// Related: bfd_delay_open_test.go -- strict OPEN negotiation and Peer BFD release.
// RFC 4271 Section 8.2.2 -- see rfc/short/rfc4271.md.
package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestRFC4271ActiveStrictPendingFailureRetriesAtInitialValue closes a real
// passive Peer's socket after strict Event 20 has cleared DelayOpen but retained
// Active. The actual retry wait must retain Active Event 18's initial interval;
// releasing it must permit a distinct strict session to establish after BFD Up.
// MUTATION: transfer retry only while DelayOpenTimer is still running.
//
// RFC requirement: RFC4271-8.2.2-13 positive -- A TCP EOF on a real passive Peer in ActiveDelayOpenBfdUpPending, after its OPEN exchange cleared DelayOpenTimer, moves the old FSM to Idle, releases its socket and published Session, stops the DelayOpen, ConnectRetry, Hold, Keepalive and BFD hold timers, increments a nonzero ConnectRetryCounter once, and schedules the actual Peer retry within the jittered initial ConnectRetryTime interval; releasing the wait permits a distinct strict session to exchange OPENs, receive Peer BFD Up, exchange KEEPALIVEs and reach Established without resetting the counter.
func TestRFC4271ActiveStrictPendingFailureRetriesAtInitialValue(t *testing.T) {
	backoff := newBackoffClock()
	peer, old, far, _ := startDelayOpenPeer(t, true, 240*time.Second, func(peer *Peer) {
		peer.SetClock(backoff)
		peer.settings.BFD = &BFDSettings{Enabled: true, Strict: true, HoldTime: 5}
		peer.settings.Capabilities = []capability.Capability{&capability.BFDStrictMode{}}
	})
	old.setBFDStateReader(func() (api.State, time.Time, bool) {
		return api.StateDown, time.Now(), true
	})
	delayOpenWrite(t, far, delayOpenStrictRetryRemote(t))
	delayOpenReadType(t, far, 1)
	delayOpenAwait(t, func() bool {
		return old.fsm.BfdSubState() == fsm.SubStateActiveDelayOpenBfdUpPending
	})
	require.Equal(t, fsm.StateActive, old.State())
	require.False(t, old.timers.IsDelayOpenTimerRunning(), "strict Event 20 has consumed DelayOpen")
	require.True(t, old.timers.IsHoldTimerRunning(), "the negotiated hold bounds the BFD wait")
	delayOpenNoBytes(t, far)
	peer.connectRetryCounter.Increment()
	peer.connectRetryCounter.Increment()
	before := peer.ConnectRetryCounter()
	closeConnQuietly(far)
	wait := backoff.awaitWait(t)
	require.Nil(t, peer.currentSession(), "the failed strict session is released before retry")
	require.Nil(t, old.Conn())
	require.Equal(t, fsm.StateIdle, old.State())
	require.Equal(t, fsm.SubStateNone, old.fsm.BfdSubState())
	require.False(t, old.timers.IsDelayOpenTimerRunning())
	require.False(t, old.timers.IsConnectRetryTimerRunning())
	require.False(t, old.timers.IsHoldTimerRunning())
	require.False(t, old.timers.IsKeepaliveTimerRunning())
	require.False(t, old.timers.IsBfdHoldTimerRunning())
	require.Equal(t, before+1, peer.ConnectRetryCounter())
	// RFC 4271 Section 8.2.2 (Active): "restarts the ConnectRetryTimer (with the initial value),"
	// The clause is unconditional on DelayOpenTimer; Section 10 permits 0.75..1.0 jitter.
	initial := fsm.DefaultConnectRetryTime
	require.GreaterOrEqual(t, wait, initial-initial/4, "strict pending retry must retain its initial-value lower bound")
	require.LessOrEqual(t, wait, initial, "Active strict pending failure must use ConnectRetryTime, not independent backoff")
	backoff.release()
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
	next.setBFDStateReader(func() (api.State, time.Time, bool) {
		return api.StateDown, time.Now(), true
	})
	remote, local := collisionProofTCPPair(t)
	require.NoError(t, peer.acceptConnection(local))
	delayOpenWrite(t, remote, delayOpenStrictRetryRemote(t))
	delayOpenReadType(t, remote, 1)
	delayOpenAwait(t, func() bool {
		return next.fsm.BfdSubState() == fsm.SubStateActiveDelayOpenBfdUpPending
	})
	delayOpenNoBytes(t, remote)
	peer.deliverBFDEvent(fsm.EventBfdUp)
	delayOpenReadType(t, remote, 4)
	require.Equal(t, fsm.StateOpenConfirm, next.State())
	delayOpenWrite(t, remote, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return next.State() == fsm.StateEstablished })
	require.Same(t, next, peer.currentSession())
	require.Same(t, local, next.Conn())
	require.Equal(t, before+1, peer.ConnectRetryCounter())
}

// delayOpenStrictRetryRemote supplies the fixture's real AS with capability 74.
// Both actual connection epochs MUST send this same conforming strict proposal.
func delayOpenStrictRetryRemote(t *testing.T) *message.Open {
	t.Helper()
	open, err := message.UnpackOpen(openBodyWithBFDStrict(9))
	require.NoError(t, err)
	open.MyAS = 65001
	return open
}
