// Design: docs/architecture/behavior/peer-lifecycle.md -- retry ownership across session retirement.
// Related: session_read.go -- owed transport failure must precede timer retirement.
package reactor

import (
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/core/clock"
)

// TestDelayOpenRetirementDrainsLateOwedFailure exercises the retirement boundary
// after a holder has missed the read loop's close request. The actual Peer read
// loop publishes both owed flags while openMu is held; invoking retireTimers
// at that boundary must deliver the failure before it erases the running delay.
func TestDelayOpenRetirementDrainsLateOwedFailure(t *testing.T) {
	backoff := newBackoffClock()
	peer, session, remote, _ := startDelayOpenPeer(t, true, 240*time.Second, func(peer *Peer) {
		peer.SetClock(backoff)
	})
	session.openMu.Lock()
	unlock := sync.OnceFunc(session.unlockOpen)
	t.Cleanup(unlock)
	closeConnQuietly(remote)
	delayOpenAwait(t, func() bool {
		if !session.closeOwed.Load() {
			return false
		}
		return session.retireOwed.Load()
	})
	// This is the holder's existing retirement operation after its earlier
	// closeOwed swap, with the read loop's late request already published.
	session.retireTimers()
	unlock()
	wait := backoff.awaitWait(t)
	require.Nil(t, peer.currentSession())
	require.Nil(t, session.Conn())
	require.Equal(t, fsm.StateIdle, session.State())
	require.Equal(t, uint32(1), peer.ConnectRetryCounter())
	requireBackoffBase(t, wait, fsm.DefaultConnectRetryTime, "retirement must preserve the late Event 18 retry")
	backoff.release()
	delayOpenRetryEstablishReplacement(t, peer, session)
}

// retrySnapshotClock is a second view of the same clock the Session uses.
// After capturing an actual retry arm it injects a scheduling gap between the
// next timer-clock sample and a later Session-clock sample. Safe concurrently.
type retrySnapshotClock struct {
	*backoffClock
	mu       sync.Mutex
	capture  bool
	jump     bool
	deadline time.Time
}

func (c *retrySnapshotClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.backoffClock.Now()
	if c.jump {
		c.jump = false
		c.backoffClock.advance(30 * time.Second)
	}
	return now
}

func (c *retrySnapshotClock) AfterFunc(delay time.Duration, callback func()) clock.Timer {
	c.mu.Lock()
	captured := c.capture
	if c.capture {
		c.capture = false
		c.deadline = c.backoffClock.Now().Add(delay)
		c.jump = true
	}
	c.mu.Unlock()
	timer := c.backoffClock.AfterFunc(delay, callback)
	if captured {
		return &retrySnapshotTimer{Timer: timer, clock: c}
	}
	return timer
}

// retrySnapshotTimer ends the injected snapshot window when cleanup cancels
// the actual source timer. A later remaining-budget read at the Peer wait is
// outside that window and must retain the original deadline unchanged.
type retrySnapshotTimer struct {
	clock.Timer
	clock *retrySnapshotClock
}

func (t *retrySnapshotTimer) Stop() bool {
	t.clock.mu.Lock()
	t.clock.jump = false
	t.clock.mu.Unlock()
	return t.Timer.Stop()
}

// TestDelayOpenRetryPreservesOriginalClockDeadline detects a double-sample
// translation that adds a scheduling gap to an already sampled retry interval.
// Both views share one clock, so the transferred deadline must equal the actual
// arm even when time advances between the two possible samples.
func TestDelayOpenRetryPreservesOriginalClockDeadline(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Connection = ConnectionPassive
	settings.DelayOpen = 240 * time.Second
	backoff := newBackoffClock()
	view := &retrySnapshotClock{backoffClock: backoff}
	peer := NewPeer(settings)
	peer.SetClock(backoff)
	peer.setReconnectDelay(time.Hour, time.Hour)
	t.Cleanup(startAndStop(t, peer))
	var session *Session
	delayOpenAwait(t, func() bool {
		session = peer.currentSession()
		if session == nil {
			return false
		}
		return session.State() == fsm.StateActive
	})
	// The passive Session has armed no protocol timer yet. Both views share
	// one underlying clock before accepting the transport and starting delay.
	session.timers.SetClock(view)
	local, remote := net.Pipe()
	t.Cleanup(func() { closeConnQuietly(remote) })
	require.NoError(t, peer.acceptConnection(local))
	require.True(t, session.holdsOpen())
	view.mu.Lock()
	view.capture = true
	view.mu.Unlock()
	closeConnQuietly(remote)
	wait := backoff.awaitWait(t)
	view.mu.Lock()
	deadline := view.deadline
	view.mu.Unlock()
	require.False(t, deadline.IsZero(), "the fixture must observe the actual retry arm")
	require.Equal(t, deadline.Sub(backoff.Now()), wait, "deadline transfer must not add the scheduling gap")
	backoff.release()
	delayOpenRetryEstablishReplacement(t, peer, session)
}
