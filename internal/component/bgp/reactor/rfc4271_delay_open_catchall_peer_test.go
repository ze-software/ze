// Design: docs/architecture/behavior/fsm-connect.md — Connect catch-all with a live DelayOpen timer.
// Related: delay_open_test.go — real Peer setup and wire helpers.
package reactor

import (
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/clock"
)

// delayOpenIdleObservation observes the first published Idle state without
// replacing the Peer's production FSM callback or changing time. Safe for
// concurrent use; session is armed only after the real dial entered DelayOpen.
//
// Peer.runOnce timestamps its FSM history row synchronously in the callback,
// before Session.Run retires the timers. FSM.runCallback releases the FSM lock
// around that callback, so Now may read State and the separate timer mutex.
// This observation depends on that history timestamp remaining before retire:
// otherwise final cleanup would mask a missing FSM-owned DelayOpen stop.
type delayOpenIdleObservation struct {
	clock.Clock
	session  atomic.Pointer[Session]
	recorded atomic.Bool
	running  chan bool
}

func (c *delayOpenIdleObservation) Now() time.Time {
	now := c.Clock.Now()
	session := c.session.Load()
	if session == nil {
		return now
	}
	if session.State() != fsm.StateIdle {
		return now
	}
	if c.recorded.CompareAndSwap(false, true) {
		c.running <- session.timers.IsDelayOpenTimerRunning()
	}
	return now
}

// TestRFC4271ConnectCatchAllStopsRunningDelayOpen sends a KEEPALIVE before any
// OPEN through a real delayed Peer's read loop. The published Idle transition
// must already have stopped DelayOpen, before Run's final cleanup can mask it.
// MUTATION: remove only StopDelayOpenTimer from FSM.change's Idle arm.
//
// RFC requirement: RFC4271-8.2.2-15 positive -- A KEEPALIVE (Event 26) received on a real Peer holding its connected socket in Connect stops its running DelayOpenTimer before the Idle history transition is published, increments ConnectRetryCounter once, and releases the session and TCP connection without sending a message; retry, hold and keepalive timers are stopped.
func TestRFC4271ConnectCatchAllStopsRunningDelayOpen(t *testing.T) {
	observed := &delayOpenIdleObservation{Clock: clock.RealClock{}, running: make(chan bool, 1)}
	peer, session, far, _ := startDelayOpenPeer(t, false, 240*time.Second, func(peer *Peer) {
		peer.SetClock(observed)
	})
	require.Equal(t, fsm.StateConnect, session.State())
	require.True(t, session.timers.IsDelayOpenTimerRunning())
	delayOpenNoBytes(t, far)
	before := peer.ConnectRetryCounter()
	observed.session.Store(session)

	// RFC 4271 Section 8.2.2, Connect: Event 26 belongs to "any other events".
	delayOpenWrite(t, far, &message.Keepalive{})

	select {
	case running := <-observed.running:
		assert.False(t, running, "Idle was published with DelayOpen still running")
	case <-time.After(5 * time.Second):
		t.Fatal("the Peer never published the catch-all Idle transition")
	}
	require.NoError(t, far.SetReadDeadline(time.Now().Add(5*time.Second)))
	var octet [1]byte
	read, err := far.Read(octet[:])
	assert.Zero(t, read, "Connect catch-all sends no message")
	assert.ErrorIs(t, err, io.EOF, "the held TCP connection is dropped")
	delayOpenAwait(t, func() bool { return peer.currentSession() != session })
	assert.Equal(t, fsm.StateIdle, session.State())
	assert.Equal(t, before+1, peer.ConnectRetryCounter(), "the failed attempt is counted once")
	assert.Nil(t, session.Conn(), "the old Session releases its socket")
	assert.False(t, session.timers.IsDelayOpenTimerRunning())
	assert.False(t, session.timers.IsConnectRetryTimerRunning())
	assert.False(t, session.timers.IsHoldTimerRunning())
	assert.False(t, session.timers.IsKeepaliveTimerRunning())
}

// TestRFC4271ConnectDuplicateStartPreservesRunningDelayOpen is the opposite
// polarity: Event 1 is outside Connect's catch-all list. A valid OPEN and
// KEEPALIVE must still establish the very same delayed session afterwards.
// MUTATION: make handleConnect's ignored start-event arm change to Idle.
//
// RFC requirement: RFC4271-8.2.2-15 negative -- ManualStart on a real Peer holding a connected socket in Connect preserves its running DelayOpenTimer, session, socket and retry counter; valid OPEN and KEEPALIVE messages then establish that same session.
func TestRFC4271ConnectDuplicateStartPreservesRunningDelayOpen(t *testing.T) {
	peer, session, far, _ := startDelayOpenPeer(t, false, 240*time.Second)
	before := peer.ConnectRetryCounter()
	conn := session.Conn()
	require.Equal(t, fsm.StateConnect, session.State())
	require.True(t, session.timers.IsDelayOpenTimerRunning())

	// RFC 4271 Section 8.2.2: "The start events (Events 1, 3-7) are ignored in the Connect state."
	require.NoError(t, session.Start())
	require.Equal(t, fsm.StateConnect, session.State())
	require.True(t, session.timers.IsDelayOpenTimerRunning())
	assert.Equal(t, before, peer.ConnectRetryCounter())
	assert.Same(t, session, peer.currentSession())
	assert.Same(t, conn, session.Conn())
	delayOpenNoBytes(t, far)

	delayOpenWrite(t, far, delayOpenRemote(0x02020202))
	delayOpenReadType(t, far, 1)
	delayOpenReadType(t, far, 4)
	delayOpenWrite(t, far, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return session.State() == fsm.StateEstablished })
	assert.Same(t, session, peer.currentSession())
	assert.Same(t, conn, session.Conn())
	assert.Equal(t, before, peer.ConnectRetryCounter())
}
