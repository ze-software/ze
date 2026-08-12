package peer

import (
	"context"
	"encoding/hex"
	"net"
	"testing"
	"time"
)

// TestDialCheckPeerRunsTheScriptOverTheDialedConnection covers the ACTIVE role in
// check mode: the peer dials the daemon and then runs the ordinary .ci
// expect/action script over that connection.
//
// VALIDATES: the one topology a listening test peer cannot express -- a daemon
// that only ACCEPTS. A BGP dynamic peer group is exactly that (`ip dynamic` plus
// `connect false`, internal/component/bgp/reactor/reactor_dynamic.go), so no
// scripted functional test could reach one while --dial was inject-only.
// PREVENTS: the silent half of that gap. runDialCheck reads the remote's OPEN
// first, which is correct for an accepting speaker (RFC 4271 Section 8.2.2 sends
// OPEN on TcpConnectionConfirmed) and would deadlock against a remote that waits
// for ours.
//
// DISCRIMINATION: the assertion is the peer's own verdict over a message it could
// only have read off the dialed socket. Routing --dial back to runActive fails it
// with "--dial requires an inject spec"; leaving the handshake half-done fails it
// on the read.
func TestDialCheckPeerRunsTheScriptOverTheDialedConnection(t *testing.T) {
	const asn = 65001
	eor := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000"

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listen addr: %T", ln.Addr())
	}

	errCh := make(chan error, 1)
	go func() {
		conn, aerr := ln.Accept()
		if aerr != nil {
			errCh <- aerr
			return
		}
		defer func() { _ = conn.Close() }()

		// An accepting BGP speaker sends its OPEN without waiting for the
		// caller's. This is what makes the dialing check peer's read-first
		// handshake correct.
		if _, werr := conn.Write(minimalOpenMsg(asn, "127.0.0.1")); werr != nil {
			errCh <- werr
			return
		}
		if _, _, rerr := ReadMessage(conn); rerr != nil { // the peer's OPEN
			errCh <- rerr
			return
		}
		if _, _, rerr := ReadMessage(conn); rerr != nil { // the peer's KEEPALIVE
			errCh <- rerr
			return
		}
		frame, derr := hex.DecodeString(eor)
		if derr != nil {
			errCh <- derr
			return
		}
		if _, werr := conn.Write(frame); werr != nil {
			errCh <- werr
			return
		}
		errCh <- nil
	}()

	p, err := New(&Config{
		Dial:   addr.String(),
		Expect: []string{"expect=bgp:conn=1:seq=1:hex=" + eor},
		Output: newDiscardWriter(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// The peer's verdict is read FIRST, and the remote's error is drained with a
	// deadline. A peer that never dials leaves the accept goroutine parked, so
	// reading errCh first would turn every failure into a ten-minute hang and
	// hide which side broke.
	result := p.Run(ctx)
	if !result.Success {
		t.Fatalf("dialing check peer failed: %v", result.Error)
	}
	select {
	case serr := <-errCh:
		if serr != nil {
			t.Fatalf("remote: %v", serr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("remote did not finish its side of the exchange")
	}
}

// newDiscardWriter keeps the peer's transcript out of the test log while still
// exercising every printf path the real binary takes.
func newDiscardWriter() *discardWriter { return &discardWriter{} }

type discardWriter struct{}

func (*discardWriter) Write(p []byte) (int, error) { return len(p), nil }
