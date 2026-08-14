package peer

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"
)

// connectSignal is an io.Writer that closes seen once the peer announces an
// accepted connection, so a test can wait for the handshake goroutine to be
// running instead of sleeping.
type connectSignal struct {
	once sync.Once
	seen chan struct{}
}

func (w *connectSignal) Write(b []byte) (int, error) {
	if bytes.Contains(b, []byte("new connection from")) {
		w.once.Do(func() { close(w.seen) })
	}
	return len(b), nil
}

// TestSortConnBatchRemoteIP verifies remote-ip mapping remains stable when
// router IDs change across reload batches.
//
// VALIDATES: conn_map remote-ip orders connections by TCP source address.
// PREVENTS: Router-id rotations making reload tests depend on accept order.
func TestSortConnBatchRemoteIP(t *testing.T) {
	conns := []connWithID{
		{routerID: 0x090A0B0C, remoteIP: netip.MustParseAddr("127.0.0.3")},
		{routerID: 0x01020304, remoteIP: netip.MustParseAddr("127.0.0.1")},
		{routerID: 0x05060708, remoteIP: netip.MustParseAddr("127.0.0.2")},
	}

	sortConnBatch(conns, connMapRemoteIP)

	want := []netip.Addr{
		netip.MustParseAddr("127.0.0.1"),
		netip.MustParseAddr("127.0.0.2"),
		netip.MustParseAddr("127.0.0.3"),
	}
	for i := range want {
		if conns[i].remoteIP != want[i] {
			t.Fatalf("conns[%d].remoteIP = %s, want %s", i, conns[i].remoteIP, want[i])
		}
	}
}

// TestSortConnBatchRouterID verifies the existing router-id mapping remains
// available for concurrent sessions whose TCP source addresses are not stable.
//
// VALIDATES: conn_map router-id orders connections by OPEN router-id.
// PREVENTS: Remote-ip support changing existing router-id mapping semantics.
func TestSortConnBatchRouterID(t *testing.T) {
	conns := []connWithID{
		{routerID: 0x090A0B0C, remoteIP: netip.MustParseAddr("127.0.0.1")},
		{routerID: 0x01020304, remoteIP: netip.MustParseAddr("127.0.0.3")},
		{routerID: 0x05060708, remoteIP: netip.MustParseAddr("127.0.0.2")},
	}

	sortConnBatch(conns, connMapRouterID)

	want := []uint32{0x01020304, 0x05060708, 0x090A0B0C}
	for i := range want {
		if conns[i].routerID != want[i] {
			t.Fatalf("conns[%d].routerID = %#x, want %#x", i, conns[i].routerID, want[i])
		}
	}
}

// newBatchTestPeer builds the minimal Peer that acceptConnMapBatch needs.
//
// The checker is real and never nil. acceptConnMapBatch consults
// Completed() on the cancel path, so a nil checker here would reintroduce
// the fail-open shape these tests exist to pin. With no expectations the
// checker reports completed, which is the state a passing run ends in.
func newBatchTestPeer(t testing.TB, out io.Writer, expect ...string) *Peer {
	t.Helper()
	checker, err := newChecker(expect)
	if err != nil {
		t.Fatalf("NewChecker(%v): %v", expect, err)
	}
	checker.Init()
	return &Peer{config: &Config{}, checker: checker, output: out}
}

// TestAcceptConnMapBatchUnblocksOnCancel pins the cancel path of a mapped
// batch whose peer never hands over an OPEN.
//
// VALIDATES: canceling the context releases a mapped batch that is blocked in
// the OPEN handshake, and it reports done rather than hanging.
// PREVENTS: a connection that is accepted but never sends an OPEN wedging the
// whole batch until the runner's outer timeout, which reports a bare timeout
// and names neither the stuck connection nor the handshake as the cause.
func TestAcceptConnMapBatchUnblocksOnCancel(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close() //nolint:errcheck // test cleanup

	sig := &connectSignal{seen: make(chan struct{})}
	p := newBatchTestPeer(t, sig)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type outcome struct {
		result Result
		done   bool
	}
	res := make(chan outcome, 1)
	go func() {
		_, r, d := p.acceptConnMapBatch(ctx, ln, 1)
		res <- outcome{result: r, done: d}
	}()

	// Connect and stay silent: doOpenHandshake blocks in ReadMessage, which
	// sets no deadline of its own.
	var dialer net.Dialer
	conn, err := dialer.DialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck // test cleanup

	// Wait for the accept to be registered before canceling, so the test
	// exercises the stuck handshake rather than racing ln.Accept.
	select {
	case <-sig.seen:
	case <-time.After(10 * time.Second):
		t.Fatal("peer never announced the accepted connection")
	}
	cancel()

	select {
	case got := <-res:
		if !got.done {
			t.Fatalf("done = false, want true: a canceled batch must stop accepting")
		}
		if !got.result.Success {
			t.Fatalf("result.Success = false (%v), want true: cancel is teardown, not a peer failure", got.result.Error)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("acceptConnMapBatch did not return after context cancel: a stuck OPEN handshake still wedges the batch")
	}
}

// TestCanceledAcceptReportsFailureUntilCheckerCompletes pins guard 1 of
// spec-fixit-test-harness-fail-open-guards (AC-1, AC-2, AC-6).
//
// VALIDATES: an accept that fails with the context already done reports
// success only when the checker finished its expectations.
// PREVENTS: a run ended by the runner timeout or an operator interrupt
// reporting a pass it never earned. Cancellation is exactly the path where a
// false green costs most, and it was unconditionally green before
// (`ai/rules/evidence.md`: a zero value must never read as a valid answer).
func TestCanceledAcceptReportsFailureUntilCheckerCompletes(t *testing.T) {
	pending := newBatchTestPeer(t, io.Discard, "expect=bgp:conn=1:seq=1:ordered=180A0000")
	if pending.checker.Completed() {
		t.Fatal("checker reports completed with an expectation outstanding: this test would prove nothing")
	}
	settled := newBatchTestPeer(t, io.Discard)
	if !settled.checker.Completed() {
		t.Fatal("checker reports incomplete with no expectations: this test would prove nothing")
	}

	cases := []struct {
		name        string
		peer        *Peer
		wantSuccess bool
	}{
		{name: "expectation outstanding", peer: pending, wantSuccess: false},
		{name: "expectations satisfied", peer: settled, wantSuccess: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var lc net.ListenConfig
			ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("listen: %v", err)
			}
			// A closed listener fails Accept at once, and the context is already
			// done, which is precisely the branch guard 1 owns.
			ln.Close() //nolint:errcheck // the failing Accept is the point of the test
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, result, done := tc.peer.acceptConnMapBatch(ctx, ln, 1)
			if !done {
				t.Fatal("done = false, want true: a failed accept ends the batch")
			}
			if result.Success != tc.wantSuccess {
				t.Fatalf("result.Success = %v, want %v (error: %v)", result.Success, tc.wantSuccess, result.Error)
			}
			if !tc.wantSuccess && result.Error == nil {
				t.Fatal("result.Error = nil: a refused batch must name its reason")
			}
		})
	}
}

// TestAcceptConnMapBatchNamesFailedHandshake pins the diagnosis a failed
// handshake produces.
//
// VALIDATES: a handshake failure inside a mapped batch names the batch slot
// and the remote address alongside the underlying error.
// PREVENTS: an N-connection batch reporting a bare "read OPEN" that does not
// say which of the N connections failed.
func TestAcceptConnMapBatchNamesFailedHandshake(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close() //nolint:errcheck // test cleanup

	p := newBatchTestPeer(t, io.Discard)

	type outcome struct {
		result Result
		done   bool
	}
	res := make(chan outcome, 1)
	go func() {
		_, r, d := p.acceptConnMapBatch(context.Background(), ln, 1)
		res <- outcome{result: r, done: d}
	}()

	var dialer net.Dialer
	conn, err := dialer.DialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck // test cleanup

	// A well-formed message that is not an OPEN: the handshake rejects it
	// deterministically, with no reliance on timing.
	if _, err := conn.Write(KeepaliveMsg()); err != nil {
		t.Fatalf("write KEEPALIVE: %v", err)
	}

	select {
	case got := <-res:
		if got.result.Success {
			t.Fatal("result.Success = true, want false: a non-OPEN first message must fail the batch")
		}
		msg := got.result.Error.Error()
		for _, want := range []string{"batch slot 1", "remote 127.0.0.1:", "expected OPEN"} {
			if !strings.Contains(msg, want) {
				t.Errorf("error %q does not contain %q", msg, want)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("acceptConnMapBatch did not return after a rejected handshake")
	}
}
