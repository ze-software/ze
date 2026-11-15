// Design: docs/architecture/behavior/fsm-established.md -- independent receive and send liveness
// Related: session_write.go -- sendHoldTimerExpired and the bounded notification attempt
// Related: peer_history.go -- immutable transitions from the production Peer callback
// RFC: rfc/short/rfc9687.md -- Sections 4.3, 4.4, and 5

package reactor

import (
	"context"
	"io"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestRFC9687ValidSendHoldExpiresBehindBlockedKeepalive establishes an actual
// Peer using parsed 90/480-second timers. Another userspace holder owns its
// write path while inbound KEEPALIVEs continue to renew the receive timer.
// Periodic KEEPALIVEs reach bounded write-path waits and leave the session up
// on contention, as production requires. No socket I/O budget is armed by the
// holder: a blocked socket write can legitimately close earlier on that budget.
// The live check at 479 seconds rules out unconditional closure; expiration
// must then close that socket and record Established-to-Idle as the first exit.
//
// RFC 9687 Section 4.4: "If SendHoldTime is non-zero, then it MUST be
// greater than the value of HoldTime; see Section 6 of [RFC9687] for
// suggested default values."
// RFC 9687 Section 5: "If the local system does not send any BGP messages
// within the period specified in SendHoldTime, then a NOTIFICATION message
// with the "Send Hold Timer Expired" Error Code MAY be sent and the BGP
// connection MUST be closed."
//
// MUTATION: disable startSendHoldTimer's AfterFunc, omit the expiry's close,
// or change the expiry's Established destination to Active: each must fail
// this same real-Peer timeline, not merely a final-state assertion.
// RFC requirement: RFC9687-4.3-7 positive -- with parsed SendHoldTime 480s exceeding negotiated HoldTime 90s, and inbound KEEPALIVEs keeping receive hold alive while the periodic outbound KEEPALIVE is blocked, the first recorded transition out of Established after Send Hold expiry enters Idle.
// RFC requirement: RFC9687-5-1 positive -- the valid 90s/480s live Peer closes its socket after Send Hold expiry despite a blocked outbound KEEPALIVE and fresh inbound KEEPALIVEs; the close cause is exactly ErrSendHoldTimerExpired, and the same socket remains open before expiry.
func TestRFC9687ValidSendHoldExpiresBehindBlockedKeepalive(t *testing.T) {
	peer, session, conn, transport, elapsed := rfc9687ValidBlockedPeer(t)
	observer := &Reactor{peers: map[netip.AddrPort]*Peer{peer.Settings().PeerKey(): peer}}
	deadline := session.sendHoldDeadline.Load()
	require.Equal(t, elapsed.Now().Add(480*time.Second).UnixNano(), deadline)

	release := holdWritePath(session)
	t.Cleanup(release)
	// Every step is shorter than HoldTime. The budget fence observes semantic
	// processing, rather than merely the socket accepting the input bytes.
	for range 31 {
		elapsed.Add(15 * time.Second)
		rfc9687ValidInboundKeepalive(t, session, conn, elapsed)
	}
	elapsed.Add(14 * time.Second)
	rfc9687ValidInboundKeepalive(t, session, conn, elapsed)
	require.Eventually(t, func() bool { return elapsed.waitedFor(session.controlWriteDeadline()) },
		runExitDeadline, time.Millisecond, "the periodic KEEPALIVE must reach its bounded write-path wait")
	require.Equal(t, fsm.StateEstablished, session.State(), "479 seconds is before Send Hold expiry")
	require.Equal(t, deadline, session.sendHoldDeadline.Load(), "inbound liveness must not renew send liveness")
	require.NoError(t, closeCause(session))
	for _, transition := range observer.PeerFSMHistory(peer.Settings().Address.String()) {
		require.NotEqual(t, fsm.StateEstablished.String(), transition.From,
			"a valid live session must not have left Established before the deadline")
	}
	select {
	case <-transport.closed:
		t.Fatal("the same socket closed before Send Hold expiry")
	default:
	}

	elapsed.Add(time.Second)
	// The expiry has one optional NOTIFICATION attempt, bounded to one second
	// behind the blocked writer. Wait for that exact acquisition before moving
	// time again, so scheduling cannot give the callback a later start time.
	require.Eventually(t, func() bool { return elapsed.waitedFor(holdExpiryNotifyDeadline) },
		runExitDeadline, time.Millisecond, "the actual Send Hold callback must reach its bounded attempt")
	elapsed.Add(holdExpiryNotifyDeadline)
	select {
	case <-transport.closed:
	case <-time.After(runExitDeadline):
		t.Fatal("Send Hold expiry left the socket open behind its blocked KEEPALIVE writer")
	}
	// The close was observed while the path remained held. Release it now so
	// the owner's final field clearing and down publication can finish too.
	release()
	var octet [1]byte
	_, err := conn.Read(octet[:])
	require.ErrorIs(t, err, io.EOF, "the remote socket must observe local closure")
	require.Equal(t, ErrSendHoldTimerExpired, closeCause(session))
	require.NotErrorIs(t, closeCause(session), ErrHoldTimerExpired,
		"fresh inbound KEEPALIVEs exclude receive-hold expiration")

	var firstDestination string
	require.Eventually(t, func() bool {
		for _, transition := range observer.PeerFSMHistory(peer.Settings().Address.String()) {
			if transition.From == fsm.StateEstablished.String() {
				firstDestination = transition.To
				return true
			}
		}
		return false
	}, runExitDeadline, time.Millisecond, "the production Peer callback must retain its first exit")
	require.Equal(t, fsm.StateIdle.String(), firstDestination,
		"a later transport cleanup cannot repair the first exit from Established")
}

// rfc9687ValidBlockedPeer owns its pipe, Peer, and timer callbacks. Cleanup
// closes the pipe and wakes all acquisition waits before joining those owners,
// including when a fatal assertion interrupts a callback at its write lock.
func rfc9687ValidBlockedPeer(t *testing.T) (*Peer, *Session, net.Conn, *rfc9687BlockedConn, *rfc9687ConcurrentClock) {
	t.Helper()
	settings, err := parsePeerFromTree("send-hold", map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "192.0.2.1"},
			"local":  map[string]any{"ip": "auto"},
		},
		"session": map[string]any{"asn": map[string]any{"remote": "65001"}},
		"timer": map[string]any{
			"receive-hold-time": "90", "send-hold-time": "480",
		},
	}, 65000, 0x01010101)
	require.NoError(t, err, "the production parser must accept the complete timer pairing")
	settings.Connection = ConnectionActive
	server, client := net.Pipe()
	transport := &rfc9687BlockedConn{Conn: server, closed: make(chan struct{})}
	elapsed := &rfc9687ConcurrentClock{FakeClock: sim.NewFakeClock(time.Now())}
	peer := NewPeer(settings)
	peer.SetClock(elapsed)
	peer.SetDialer(&rfc9687PipeDialer{conn: transport})
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(func() {
		elapsed.stop()
		transport.Close() //nolint:errcheck // cleanup also runs after the production close
		client.Close()    //nolint:errcheck // interrupt every owned socket operation
		cancel()
		waitCtx, waitCancel := context.WithTimeout(context.Background(), runExitDeadline)
		defer waitCancel()
		if err := peer.Wait(waitCtx); err != nil {
			t.Errorf("the owned Peer did not stop during cleanup: %v", err)
		}
		done := make(chan struct{})
		go func() { defer close(done); elapsed.callbacks.Wait() }()
		select {
		case <-done:
		case <-time.After(runExitDeadline):
			t.Error("owned timer callbacks did not return after socket closure and timer release")
		}
	})
	peer.StartWithContext(ctx)
	rfc9687TransitionRead(t, client, msgtype.TypeOPEN)
	require.NoError(t, client.SetWriteDeadline(time.Now().Add(runExitDeadline)))
	_, err = client.Write(wellFormedOpen())
	require.NoError(t, err)
	rfc9687TransitionRead(t, client, msgtype.TypeKEEPALIVE)
	_, err = client.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	body := rfc9687TransitionRead(t, client, msgtype.TypeUPDATE)
	require.Equal(t, []byte{0, 0, 0, 0}, body, "observe the initial EOR before blocking subsequent writes")
	var session *Session
	require.Eventually(t, func() bool {
		session = peer.currentSession()
		if session == nil {
			return false
		}
		if session.State() != fsm.StateEstablished {
			return false
		}
		return session.sendingInitialRoutes.Load() == 0
	}, runExitDeadline, time.Millisecond)
	require.Equal(t, 90*time.Second, session.timers.HoldTime())
	return peer, session, client, transport, elapsed
}

// rfc9687ValidInboundKeepalive observes the receive timer's renewed deadline;
// this proves the read goroutine processed the input while its writer stalls.
func rfc9687ValidInboundKeepalive(t *testing.T, session *Session, conn net.Conn, elapsed *rfc9687ConcurrentClock) {
	t.Helper()
	require.NoError(t, conn.SetWriteDeadline(time.Now().Add(runExitDeadline)))
	_, err := conn.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	want := elapsed.Now().Add(90 * time.Second)
	require.Eventually(t, func() bool {
		budget := session.timers.HoldBudget()
		if budget.State != fsm.HoldBudgetArmed {
			return false
		}
		return budget.Expires.Equal(want)
	}, runExitDeadline, time.Millisecond, "the inbound KEEPALIVE must renew the actual receive timer")
}

// rfc9687BlockedConn observes local socket closure without a remote read that
// could release a writer. Safe for concurrent use; zero is not usable.
type rfc9687BlockedConn struct {
	net.Conn
	closeOnce sync.Once
	closed    chan struct{}
}

func (c *rfc9687BlockedConn) Close() error {
	err := c.Conn.Close()
	c.closeOnce.Do(func() { close(c.closed) })
	return err
}

// rfc9687PipeDialer delivers its socket once. A reconnect cannot replace the
// transport whose expiration the test observes. Safe for concurrent use.
type rfc9687PipeDialer struct {
	conn net.Conn
	used atomic.Bool
}

func (d *rfc9687PipeDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if d.used.Swap(true) {
		return nil, net.ErrClosed
	}
	return d.conn, nil
}

// rfc9687ConcurrentClock keeps one simulated time for every timer, but gives
// AfterFunc callbacks the concurrency real timers have: one blocked KEEPALIVE
// cannot prevent SendHoldTimer from firing. The owner MUST stop and join its
// callbacks after closing the socket. Safe for concurrent use; zero is invalid.
type rfc9687ConcurrentClock struct {
	*sim.FakeClock
	mu        sync.Mutex
	stopped   bool
	callbacks sync.WaitGroup
	waits     []time.Duration
	wake      []chan time.Time
}

func (c *rfc9687ConcurrentClock) AfterFunc(d time.Duration, callback func()) clock.Timer {
	return c.FakeClock.AfterFunc(d, func() {
		c.mu.Lock()
		if c.stopped {
			c.mu.Unlock()
			return
		}
		c.callbacks.Add(1)
		c.mu.Unlock()
		// Each is a test-owned timer lifecycle, joined after stop seals admission.
		go func() { defer c.callbacks.Done(); callback() }()
	})
}

// NewTimer schedules channel timers too: sim.FakeClock's own channel timers
// are inert, which would make LockWithin's bounded notification wait infinite.
func (c *rfc9687ConcurrentClock) NewTimer(d time.Duration) clock.Timer {
	ch := make(chan time.Time, 1)
	timer := c.FakeClock.AfterFunc(d, func() {
		select {
		case ch <- c.Now():
		default: // cleanup already released this wait
		}
	})
	c.mu.Lock()
	c.waits = append(c.waits, d)
	c.wake = append(c.wake, ch)
	if c.stopped {
		select {
		case ch <- c.Now():
		default: // an already-fired timer needs no cleanup wake
		}
	}
	c.mu.Unlock()
	return &rfc9687ChannelTimer{Timer: timer, ticks: ch}
}

func (c *rfc9687ConcurrentClock) waitedFor(wait time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Contains(c.waits, wait)
}

func (c *rfc9687ConcurrentClock) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = true
	for _, ch := range c.wake {
		select {
		case ch <- c.Now():
		default: // this bounded wait already fired
		}
	}
}

type rfc9687ChannelTimer struct {
	clock.Timer
	ticks <-chan time.Time
}

func (t *rfc9687ChannelTimer) C() <-chan time.Time { return t.ticks }
