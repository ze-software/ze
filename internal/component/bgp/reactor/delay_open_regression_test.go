// Design: docs/architecture/behavior/peer-lifecycle.md — DelayOpen failure ownership and inheritance.
package reactor

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/network"
)

// TestDelayOpenExpiryWriteFailureDeliversOneEvent18 uses an actual outbound
// TCP connection. Only its write half is shut; the Session reader stays live
// until the failed delayed OPEN retires the transport and wakes it. The writer
// then waits until cancellation enters closeConn, before delivering Event18.
func TestDelayOpenExpiryWriteFailureDeliversOneEvent18(t *testing.T) {
	entered := make(chan struct{})
	writeReset := make(chan struct{})
	resumeWrite := make(chan struct{})
	releaseWriter := sync.OnceFunc(func() { close(resumeWrite) })
	peer, session, _, clock := startDelayOpenPeer(t, false, 240*time.Second, func(peer *Peer) {
		peer.SetDialer(&delayOpenReadDialer{
			Dialer: peer.dialer, entered: entered,
			writeReset: writeReset, resumeWrite: resumeWrite,
		})
	})
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Session reader did not enter TCP read")
	}
	readConn, isReadConn := session.Conn().(*delayOpenReadConn)
	if !isReadConn {
		t.Fatal("session connection is not the delayOpenReadConn the fixture installed")
	}
	conn, isTCP := readConn.Conn.(*net.TCPConn)
	if !isTCP {
		t.Fatal("delayOpenReadConn does not wrap a TCP connection")
	}
	if err := conn.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	peer.connectRetryCounter.Increment()
	expired := make(chan struct{})
	go func() {
		clock.Add(240 * time.Second)
		close(expired)
	}()
	t.Cleanup(func() {
		// Release even on a failed barrier, before the fixture joins the Peer.
		releaseWriter()
		select {
		case <-expired:
		case <-time.After(5 * time.Second):
			t.Error("delayed OPEN writer did not finish after release")
		}
	})
	select {
	case <-writeReset:
	case <-time.After(5 * time.Second):
		t.Fatal("delayed OPEN writer did not reach deadline cleanup")
	}
	// writeMessageWithin resets the deadline after retireWrite closes TCP and
	// signals errChan, but before releasing writeMu. delayOpenExpired also
	// still owns openMu, so the awakened reader cannot deliver Event18.
	// Only cancellation's closeConn can now take s.mu: it passes the early
	// timer-cleanup position, then waits for writeMu. Observing that lock,
	// rather than sleeping or winning a race, orders that position before
	// the writer's Event18. closeConn need not finish until writeMu is free.
	delayOpenAwait(t, func() bool {
		if session.mu.TryRLock() {
			session.mu.RUnlock()
			return false
		}
		return true
	})
	releaseWriter()
	select {
	case <-expired:
	case <-time.After(5 * time.Second):
		t.Fatal("delayed OPEN writer did not deliver its failure")
	}
	delayOpenAwait(t, func() bool { return peer.currentSession() != session })
	if session.State() != fsm.StateActive || peer.ConnectRetryCounter() != 1 {
		t.Fatalf("write/read failure delivered extra Event18: state=%s counter=%d", session.State(), peer.ConnectRetryCounter())
	}
	failures := 0
	for _, transition := range peer.fSMHistory() {
		if transition.From == fsm.StateConnect.String() && transition.To == fsm.StateActive.String() {
			failures++
		}
		if transition.To == fsm.StateIdle.String() {
			t.Fatal("reader delivered a second failure after Connect became Active")
		}
	}
	if failures != 1 {
		t.Fatalf("failure transitions=%d, want 1", failures)
	}
	if session.Conn() != nil || session.timers.IsDelayOpenTimerRunning() ||
		session.timers.IsConnectRetryTimerRunning() || session.timers.IsHoldTimerRunning() {
		t.Fatal("write-failed Session retained transport or timers")
	}
}

type delayOpenReadDialer struct {
	network.Dialer
	entered     chan struct{}
	writeReset  chan struct{}
	resumeWrite <-chan struct{}
}

func (d *delayOpenReadDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := d.Dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return &delayOpenReadConn{
		Conn: conn, entered: d.entered,
		writeReset: d.writeReset, resumeWrite: d.resumeWrite,
	}, nil
}

type delayOpenReadConn struct {
	net.Conn
	entered     chan struct{}
	writeReset  chan struct{}
	resumeWrite <-chan struct{}
	once        sync.Once
}

func (c *delayOpenReadConn) Read(buf []byte) (int, error) {
	c.once.Do(func() { close(c.entered) })
	return c.Conn.Read(buf)
}

func (c *delayOpenReadConn) SetWriteDeadline(deadline time.Time) error {
	err := c.Conn.SetWriteDeadline(deadline)
	if deadline.IsZero() {
		close(c.writeReset)
		<-c.resumeWrite
	}
	return err
}

func delayOpenReadCollisionClose(t *testing.T, conn net.Conn) {
	t.Helper()
	packet := delayOpenReadType(t, conn, 3)
	if len(packet) != message.HeaderLen+2 || packet[message.HeaderLen] != byte(message.NotifyCease) ||
		packet[message.HeaderLen+1] != message.NotifyCeaseConnectionCollision {
		t.Fatalf("collision rejection is not Cease/ConnectionCollision: %x", packet)
	}
	var octet [1]byte
	if n, err := conn.Read(octet[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("collision loser did not close after Cease: bytes=%d error=%v", n, err)
	}
}
