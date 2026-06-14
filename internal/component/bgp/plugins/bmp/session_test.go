package bmp

import (
	"errors"
	"net"
	"testing"
	"time"
)

// isTimeout reports whether err is an i/o deadline (timeout) error. A read that
// fails with a timeout means the server did NOT close the connection in time,
// which the session tests treat as a failure rather than a successful close.
func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func TestBMPSessionAccepts(t *testing.T) {
	// VALIDATES: AC-12 -- Plugin accepts TCP connection, reads BMP Common Header, validates version==3
	// PREVENTS: session goroutine crash on connect

	bp := &BMPPlugin{
		state:  newBMPState(),
		stopCh: make(chan struct{}),
	}

	// Start listener on ephemeral port.
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	bp.listeners = append(bp.listeners, ln)

	bp.sessions.Go(func() {
		bp.acceptLoop(ln, 10)
	})

	// Connect and send Initiation message.
	conn, err := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	buf := make([]byte, 256)
	init := &Initiation{
		TLVs: []TLV{MakeStringTLV(InitTLVSysName, "test-router")},
	}
	n := WriteInitiation(buf, 0, init)
	if _, err := conn.Write(buf[:n]); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Send Termination to cleanly end session.
	term := &Termination{
		TLVs: []TLV{MakeStringTLV(TermTLVString, "done")},
	}
	n = WriteTermination(buf, 0, term)
	if _, err := conn.Write(buf[:n]); err != nil {
		t.Fatalf("write termination: %v", err)
	}

	// Close our end and stop the plugin.
	if err := conn.Close(); err != nil {
		t.Logf("close: %v", err)
	}

	close(bp.stopCh)
	bp.stopListeners()
	bp.sessions.Wait()
}

func TestBMPMalformedHeaderDrops(t *testing.T) {
	// VALIDATES: AC-19 -- Malformed header closes session without panic
	// PREVENTS: panic on garbage input

	bp := &BMPPlugin{
		state:  newBMPState(),
		stopCh: make(chan struct{}),
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	bp.listeners = append(bp.listeners, ln)

	bp.sessions.Go(func() {
		bp.acceptLoop(ln, 10)
	})

	// Connect and send invalid BMP (version 2).
	conn, err := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	badHeader := []byte{2, 0, 0, 0, 6, MsgInitiation} // version 2 = invalid
	if _, err := conn.Write(badHeader); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Verify the server closed the connection after the bad version. The read
	// blocks until the server closes its end (deterministic: EOF/reset arrives
	// immediately on close), or the generous deadline fires if it never does.
	readBuf := make([]byte, 1)
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	_, readErr := conn.Read(readBuf)
	switch {
	case readErr == nil:
		t.Error("expected connection closed by server after bad version, but read succeeded")
	case isTimeout(readErr):
		t.Errorf("server did not close connection after bad version within deadline: %v", readErr)
	}

	if err := conn.Close(); err != nil {
		t.Logf("close: %v", err)
	}

	close(bp.stopCh)
	bp.stopListeners()
	bp.sessions.Wait()
}

func TestBMPMaxSessionsRejects(t *testing.T) {
	// VALIDATES: security -- max-sessions cap enforced
	// PREVENTS: unbounded connection count

	bp := &BMPPlugin{
		state:  newBMPState(),
		stopCh: make(chan struct{}),
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	bp.listeners = append(bp.listeners, ln)

	bp.sessions.Go(func() {
		bp.acceptLoop(ln, 1) // max 1 session
	})

	// First connection: should be accepted.
	conn1, err := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial 1: %v", err)
	}

	// Send valid init to keep session alive.
	buf := make([]byte, 256)
	init := &Initiation{TLVs: []TLV{MakeStringTLV(InitTLVSysName, "r1")}}
	n := WriteInitiation(buf, 0, init)
	if _, err := conn1.Write(buf[:n]); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Second connection: should be rejected (max sessions = 1). The accept loop
	// is single-goroutine and serial, so conn1 is accepted and counted
	// (active.Add(1) in acceptLoop) before conn2 can be accepted; no pause is
	// needed to let the first session "start".
	conn2, err := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial 2: %v", err)
	}

	// Server should close conn2 immediately. The read blocks until the server
	// closes its end (deterministic), or the generous deadline fires.
	if err := conn2.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	readBuf := make([]byte, 1)
	_, readErr := conn2.Read(readBuf)
	switch {
	case readErr == nil:
		t.Error("expected second connection to be rejected, but read succeeded")
	case isTimeout(readErr):
		t.Errorf("server did not reject second connection within deadline: %v", readErr)
	}

	if err := conn1.Close(); err != nil {
		t.Logf("close conn1: %v", err)
	}
	if err := conn2.Close(); err != nil {
		t.Logf("close conn2: %v", err)
	}

	close(bp.stopCh)
	bp.stopListeners()
	bp.sessions.Wait()
}
