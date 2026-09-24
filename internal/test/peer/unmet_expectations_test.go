// Design: docs/architecture/testing/ci-format.md -- the check peer's success contract
// Related: peer.go -- Run, the guard under test

package peer

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// unmetEOR is the IPv4 unicast End-of-RIB both connections of the scripts below
// expect. Each test's remote sends it once, so conn=1 completes and conn=2 never does.
const unmetEOR = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000"

// twoConnectionScript owes one End-of-RIB on each of two connections.
var twoConnectionScript = []string{
	"expect=bgp:conn=1:seq=1:hex=" + unmetEOR,
	"expect=bgp:conn=2:seq=1:hex=" + unmetEOR,
}

// serveOneConnection plays the daemon for ONE connection: it sends its OPEN, reads
// the peer's OPEN and KEEPALIVE, sends the End-of-RIB, then waits for the peer to
// close the connection, which is what a peer does when a later connection is owed.
func serveOneConnection(conn net.Conn) error {
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	if _, err := conn.Write(minimalOpenMsg(65001)); err != nil {
		return err
	}
	if _, _, err := ReadMessage(conn); err != nil { // the peer's OPEN
		return err
	}
	if _, _, err := ReadMessage(conn); err != nil { // the peer's KEEPALIVE
		return err
	}
	frame, err := hex.DecodeString(unmetEOR)
	if err != nil {
		return err
	}
	if _, err := conn.Write(frame); err != nil {
		return err
	}
	// Only an orderly close ends conn=1. A deadline or a reset is the peer
	// failing to close, which the caller MUST see rather than read as success.
	for {
		_, _, err := ReadMessage(conn)
		if errors.Is(err, io.EOF) {
			return nil // the peer closed its side: conn=1 is over
		}
		if err != nil {
			return err
		}
	}
}

// signalWriter reports on seen the first time the peer's output carries needle.
// The peer serializes its writes under Peer.mu, so no lock is needed here.
type signalWriter struct {
	needle string
	text   strings.Builder
	seen   chan struct{}
	fired  bool
}

func (w *signalWriter) Write(b []byte) (int, error) {
	w.text.Write(b)
	if !w.fired && strings.Contains(w.text.String(), w.needle) {
		w.fired = true
		close(w.seen)
	}
	return len(b), nil
}

// requireUnmet asserts the peer failed and that its error names the connection
// whose expectation was never met.
func requireUnmet(t *testing.T, result Result) {
	t.Helper()
	if result.Success {
		t.Fatal("a check peer whose conn=2 expectation was never met reported success")
	}
	if !errors.Is(result.Error, ErrExpectationsUnmet) {
		t.Fatalf("error must be ErrExpectationsUnmet, got %v", result.Error)
	}
	if !strings.Contains(result.Error.Error(), "conn=2") {
		t.Fatalf("error must name the connection still owed, got %v", result.Error)
	}
}

// TestListeningCheckPeerStoppedWhileWaitingForNextConnectionFails covers the peer
// the runner stops with SIGTERM while it waits for a connection the script still
// owes.
//
// VALIDATES: a check peer reports success only when every expectation it holds
// was met. Stopped earlier, it fails and names the first unmet expectation.
// PREVENTS: the observed false pass (a peer printed "waiting for next connection
// (1/2)" and then "successful" after SIGTERM). The cancel reached the Accept loop,
// whose cancel branch returned Success without asking the checker.
//
// DISCRIMINATION: the cancel is sent only after the peer printed that it waits
// for connection 2, so it lands in the Accept loop. Removing the check-mode
// guard in Run makes this test report success.
func TestListeningCheckPeerStoppedWhileWaitingForNextConnectionFails(t *testing.T) {
	port := reservePort(t)
	out := &signalWriter{needle: "waiting for next connection (1/2)", seen: make(chan struct{})}
	p, err := New(&Config{
		Port:           port,
		TCPConnections: 2,
		Expect:         twoConnectionScript,
		Output:         out,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	done := make(chan Result, 1)
	go func() { done <- p.Run(ctx) }()

	select {
	case <-p.Ready():
	case res := <-done:
		t.Fatalf("peer ended before it listened: %v", res.Error)
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck // test socket
	if err := serveOneConnection(conn); err != nil {
		t.Fatalf("remote side: %v", err)
	}

	select {
	case <-out.seen:
	case res := <-done:
		t.Fatalf("peer ended before waiting for conn=2: success=%v err=%v", res.Success, res.Error)
	}
	cancel() // what the runner's SIGTERM does to ze-test peer
	requireUnmet(t, <-done)
}

// TestDialingCheckPeerOwedASecondConnectionFails covers the active role: a dialing
// peer serves exactly one connection, so a script that owes a second one can
// never be met.
//
// VALIDATES: runDialCheck's early return after connection 1 is not a pass.
// PREVENTS: endSequence answering Success for the end of conn=1, which Run
// relayed as the peer's verdict with conn=2 never opened.
//
// DISCRIMINATION: removing the check-mode guard in Run makes this test report
// success.
func TestDialingCheckPeerOwedASecondConnectionFails(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close() //nolint:errcheck // test listener

	errCh := make(chan error, 1)
	go func() {
		conn, aerr := ln.Accept()
		if aerr != nil {
			errCh <- aerr
			return
		}
		defer conn.Close() //nolint:errcheck // test socket
		errCh <- serveOneConnection(conn)
	}()

	p, err := New(&Config{
		Dial:   ln.Addr().String(),
		Expect: twoConnectionScript,
		Output: newDiscardWriter(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	requireUnmet(t, p.Run(ctx))
	if serr := <-errCh; serr != nil {
		t.Fatalf("remote side: %v", serr)
	}
}

// TestCheckerUnmetNamesTheFirstOwedExpectation pins the text the failure carries.
//
// VALIDATES: the error names the connection, the expectation, and how many remain.
// PREVENTS: a failure that says only "failed", which sends the reader to diff the
// whole script by hand.
func TestCheckerUnmetNamesTheFirstOwedExpectation(t *testing.T) {
	c, err := newChecker(twoConnectionScript)
	if err != nil {
		t.Fatalf("newChecker: %v", err)
	}
	got := c.unmet()
	want := "conn=1 still owes " + strings.ToLower(unmetEOR) + " (2 expectations unmet)"
	if !strings.EqualFold(got, want) {
		t.Fatalf("unmet() = %q, want %q", got, want)
	}
}
