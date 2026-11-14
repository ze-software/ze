// Design: docs/architecture/testing/ci-format.md -- option=stop-reading
// Related: stop_reading.go -- the parser, the validation and the hold under test

package peer

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

// stopReadingTestWait bounds every wait of these tests on the far side.
const stopReadingTestWait = 20 * time.Second

// tcpPair returns the two ends of one loopback TCP connection: local is the
// end the peer under test holds, remote the end that plays the daemon. The
// stall is a property of kernel socket buffers, which an in-memory pipe lacks.
func tcpPair(t *testing.T) (local, remote net.Conn) {
	t.Helper()
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close() //nolint:errcheck // test listener
	var dialer net.Dialer
	remote, err = dialer.DialContext(t.Context(), "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	local, err = listener.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	t.Cleanup(func() { _ = local.Close() })
	return local, remote
}

// newStopReadingPeer builds a check peer that stops reading connection conn,
// holding the given expectations.
func newStopReadingPeer(t *testing.T, conn, connections int, expect ...string) *Peer {
	t.Helper()
	peer, err := New(&Config{
		Mode:           ModeCheck,
		TCPConnections: connections,
		StopReading:    &StopReadingDecl{Conn: conn},
		Expect:         expect,
		Output:         new(bytes.Buffer),
	})
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}
	return peer
}

// establishAndSendEOR plays the daemon up to its first UPDATE: OPEN, the read
// of the peer's OPEN and KEEPALIVE, then an End-of-RIB and the read of the
// KEEPALIVE the peer answers it with.
func establishAndSendEOR(conn net.Conn) error {
	if err := conn.SetDeadline(time.Now().Add(stopReadingTestWait)); err != nil {
		return err
	}
	if _, err := conn.Write(minimalOpenMsg(65001)); err != nil {
		return err
	}
	reader := newMessageReader(conn)
	for range 2 { // the peer's OPEN and KEEPALIVE
		if _, _, err := reader.readMessage(); err != nil {
			return err
		}
	}
	eor, err := hex.DecodeString(unmetEOR)
	if err != nil {
		return err
	}
	return sendAndReadReply(conn, eor)
}

// serveConnection is one lifecycle worker, joined by each test's result read.
func serveConnection(ctx context.Context, p *Peer, conn net.Conn, done chan<- Result) {
	done <- p.handleConnection(ctx, conn)
}

// holdConnectionUnread is one lifecycle worker, joined by each test's result read.
func holdConnectionUnread(ctx context.Context, p *Peer, conn net.Conn, interval time.Duration, done chan<- Result) {
	done <- p.holdUnread(ctx, conn, interval)
}

// awaitResult reads a worker's result, failing the test when none arrives.
func awaitResult(t *testing.T, done <-chan Result) Result {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(stopReadingTestWait):
		t.Fatal("the peer did not return")
		return Result{}
	}
}

// TestPeerOptionStopReading proves what option=stop-reading parses into, and
// what it refuses, through parseOptionConfig, the dispatcher every option line
// of a peer block goes through, so the arm in peerOptionArms is what is read.
//
// VALIDATES: the option is claimed by ze-peer, names connection 1 unless conn=
// says otherwise, and refuses an unknown key, a value that is not a boolean
// and a conn= that names no connection.
// PREVENTS: a mistyped line that parses into nothing, which would leave the
// peer reading and the test's stall absent with no word of why.
func TestPeerOptionStopReading(t *testing.T) {
	t.Run("value=true stops reading the first connection", func(t *testing.T) {
		var config Config
		claimed, err := parseOptionConfig(&config, "stop-reading", map[string]string{keyValue: "true"})
		if err != nil {
			t.Fatal(err)
		}
		if !claimed {
			t.Fatal("ze-peer did not claim option=stop-reading")
		}
		if config.StopReading == nil {
			t.Fatal("value=true left the option off")
		}
		if config.StopReading.Conn != 1 {
			t.Fatalf("connection = %d, want 1", config.StopReading.Conn)
		}
	})

	t.Run("conn names the connection", func(t *testing.T) {
		var config Config
		kv := map[string]string{keyValue: "true", keyStopReadingConn: "2"}
		if _, err := parseOptionConfig(&config, "stop-reading", kv); err != nil {
			t.Fatal(err)
		}
		if config.StopReading == nil {
			t.Fatal("value=true left the option off")
		}
		if config.StopReading.Conn != 2 {
			t.Fatalf("connection = %d, want 2", config.StopReading.Conn)
		}
	})

	t.Run("value=false leaves the option off", func(t *testing.T) {
		config := Config{StopReading: &StopReadingDecl{Conn: 1}}
		if _, err := parseOptionConfig(&config, "stop-reading", map[string]string{keyValue: "false"}); err != nil {
			t.Fatal(err)
		}
		if config.StopReading != nil {
			t.Fatalf("value=false left the option on: %+v", config.StopReading)
		}
	})

	t.Run("an unknown key is refused by name", func(t *testing.T) {
		var config Config
		kv := map[string]string{keyValue: "true", "after": "3"}
		_, err := parseOptionConfig(&config, "stop-reading", kv)
		if err == nil {
			t.Fatal("accepted the unknown key after=")
		}
		if !strings.Contains(err.Error(), `"after"`) {
			t.Fatalf("the refusal does not name the key: %v", err)
		}
		if config.StopReading != nil {
			t.Fatal("a refused line still set the option")
		}
	})

	refused := []struct {
		name string
		kv   map[string]string
	}{
		{"a missing value", map[string]string{}},
		{"a value that is not a boolean", map[string]string{keyValue: "yes"}},
		{"a conn that is not a number", map[string]string{keyValue: "true", keyStopReadingConn: "first"}},
		{"connection zero", map[string]string{keyValue: "true", keyStopReadingConn: "0"}},
		{"a negative connection", map[string]string{keyValue: "true", keyStopReadingConn: "-1"}},
	}
	for _, tt := range refused {
		t.Run(tt.name+" is refused", func(t *testing.T) {
			var config Config
			if _, err := parseOptionConfig(&config, "stop-reading", tt.kv); err == nil {
				t.Fatalf("accepted %v", tt.kv)
			}
			if config.StopReading != nil {
				t.Fatal("a refused line still set the option")
			}
		})
	}
}

// TestStopReadingRefusesIncompatiblePeer proves New refuses a peer the option
// cannot run under, and accepts the one it is written for.
//
// VALIDATES: stop-reading needs a check peer that serves the named connection,
// without conn_map, silent or remote_close.
// PREVENTS: a combination that runs with one of the two options quietly not
// in force.
func TestStopReadingRefusesIncompatiblePeer(t *testing.T) {
	decl := &StopReadingDecl{Conn: 1}
	refused := []struct {
		name   string
		config Config
	}{
		{"sink mode", Config{StopReading: decl, Mode: ModeSink}},
		{"echo mode", Config{StopReading: decl, Mode: ModeEcho}},
		{"conn_map", Config{StopReading: decl, ConnMap: connMapRouterID}},
		{"silent", Config{StopReading: decl, Silent: true}},
		{"remote_close", Config{StopReading: decl, RemoteClose: true}},
		{"a connection past tcp_connections", Config{StopReading: &StopReadingDecl{Conn: 3}, TCPConnections: 2}},
		{"connection 2 of a one-connection peer", Config{StopReading: &StopReadingDecl{Conn: 2}}},
	}
	for i := range refused {
		tt := &refused[i] // a Config is too large to copy per case
		t.Run(tt.name+" is refused", func(t *testing.T) {
			tt.config.Output = new(bytes.Buffer)
			if _, err := New(&tt.config); err == nil {
				t.Fatal("accepted an incompatible peer")
			}
		})
	}
	t.Run("a check peer serving the connection is accepted", func(t *testing.T) {
		config := Config{StopReading: &StopReadingDecl{Conn: 2}, TCPConnections: 2, Output: new(bytes.Buffer)}
		if _, err := New(&config); err != nil {
			t.Fatal(err)
		}
	})
}

// TestStopReadingBlocksTheRemoteWriter proves the stall itself: once the
// daemon's first UPDATE has arrived, nothing more is read, so a writer on the
// other end runs out of room, while KEEPALIVEs keep arriving from the peer.
//
// The method is a real loopback TCP connection on which the test plays the
// daemon. After the End-of-RIB it writes filler until one write makes no
// progress for a second. A peer that still read would take the whole bound
// below without ever blocking, so reaching the bound fails the test. It then
// reads two KEEPALIVEs: the reply to the End-of-RIB and one from the timer.
//
// VALIDATES: option=stop-reading stops the read loop, keeps the connection
// open and keeps the KEEPALIVEs coming.
// PREVENTS: a peer that only stops answering (option=silent) being taken for
// one that stopped reading.
func TestStopReadingBlocksTheRemoteWriter(t *testing.T) {
	local, remote := tcpPair(t)
	peer := newStopReadingPeer(t, 1, 1, "expect=bgp:conn=1:seq=1:hex="+unmetEOR)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan Result, 1)
	go serveConnection(ctx, peer, local, done)

	// The peer's reply to the End-of-RIB is read here, so the first KEEPALIVE
	// read after the stall can only come from the timer.
	if err := establishAndSendEOR(remote); err != nil {
		t.Fatalf("establish: %v", err)
	}

	// 64 MiB is far above any socket buffer pair a host autotunes to, and a
	// reading peer drains it in well under the per-write second.
	const (
		chunkOctets = 64 << 10
		boundOctets = 64 << 20
	)
	filler := make([]byte, chunkOctets)
	stalled := false
	for written := 0; written < boundOctets; {
		if err := remote.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		n, err := remote.Write(filler)
		written += n
		if err == nil {
			continue
		}
		if !isTimeout(err) {
			t.Fatalf("write after %d octets: %v", written, err)
		}
		stalled = true
		break
	}
	if !stalled {
		t.Fatalf("the peer took %d octets after the first UPDATE without the writer ever blocking: it is still reading", boundOctets)
	}

	if err := remote.SetReadDeadline(time.Now().Add(stopReadingTestWait)); err != nil {
		t.Fatal(err)
	}
	header, _, err := newMessageReader(remote).readMessage()
	if err != nil {
		t.Fatalf("no KEEPALIVE arrived from the peer that stopped reading: %v", err)
	}
	if header[18] != MsgKEEPALIVE {
		t.Fatalf("the peer wrote message type %d, want a KEEPALIVE", header[18])
	}

	select {
	case result := <-done:
		t.Fatalf("the peer let go of the connection while the remote held it open: %+v", result)
	default:
	}
}

// TestHoldUnreadReturnsWhenTheRemoteCloses proves how the stall ends: the
// peer sees the remote's close through its own KEEPALIVE writes, with no read,
// and reports success so the next connection can be accepted.
//
// VALIDATES: the stalled connection ends when the daemon closes it.
// PREVENTS: a peer that needs a read to notice the close, which would drain
// the buffer the stall exists to keep full.
func TestHoldUnreadReturnsWhenTheRemoteCloses(t *testing.T) {
	local, remote := tcpPair(t)
	peer := newStopReadingPeer(t, 1, 1)

	done := make(chan Result, 1)
	go holdConnectionUnread(t.Context(), peer, local, 10*time.Millisecond, done)

	if err := remote.SetReadDeadline(time.Now().Add(stopReadingTestWait)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := newMessageReader(remote).readMessage(); err != nil {
		t.Fatalf("no KEEPALIVE before the close: %v", err)
	}
	if err := remote.Close(); err != nil {
		t.Fatal(err)
	}

	result := awaitResult(t, done)
	if !result.Success {
		t.Fatalf("the remote's close was not read as the end of the stall: %v", result.Error)
	}
}

// TestHoldUnreadFailsWhenHeldToTeardown proves the verdict when the daemon
// never closes the connection: the test's teardown is a failure that says so.
//
// VALIDATES: a daemon that keeps a session whose peer stopped reading fails
// the peer with errUnreadHeldToTeardown.
// PREVENTS: teardown being read as the close, which would pass a daemon that
// never dropped the session.
func TestHoldUnreadFailsWhenHeldToTeardown(t *testing.T) {
	local, _ := tcpPair(t)
	peer := newStopReadingPeer(t, 1, 1)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan Result, 1)
	go holdConnectionUnread(ctx, peer, local, 10*time.Millisecond, done)
	cancel()

	result := awaitResult(t, done)
	if result.Success {
		t.Fatal("teardown was read as the remote closing the connection")
	}
	if !errors.Is(result.Error, errUnreadHeldToTeardown) {
		t.Fatalf("error = %v, want errUnreadHeldToTeardown", result.Error)
	}
}

// TestStopReadingRefusesExpectationsLeftBehind proves the peer refuses a
// script that expects a frame after the first UPDATE on the connection it
// stops reading, and names the expectation.
//
// VALIDATES: the first UPDATE must meet the connection's last expectation.
// PREVENTS: an expectation that can never be met being carried over to the
// next connection, where another frame would be judged against it.
func TestStopReadingRefusesExpectationsLeftBehind(t *testing.T) {
	local, remote := tcpPair(t)
	peer := newStopReadingPeer(t, 1, 1,
		"expect=bgp:conn=1:seq=1:hex="+unmetEOR,
		"expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0015030400")

	done := make(chan Result, 1)
	go serveConnection(t.Context(), peer, local, done)
	if err := establishAndSendEOR(remote); err != nil {
		t.Fatalf("establish: %v", err)
	}

	result := awaitResult(t, done)
	if result.Success {
		t.Fatal("accepted an expectation the peer can never read a frame for")
	}
	if !strings.Contains(result.Error.Error(), "must meet the last expectation") {
		t.Fatalf("the refusal does not say what the script owes: %v", result.Error)
	}
}

// TestStopReadingLeavesOtherConnectionsRead proves the option stalls only the
// connection it names: a peer told to stop reading connection 2 ends
// connection 1 the usual way, when that connection's expectations are met.
//
// VALIDATES: conn= scopes the stall to one connection.
// PREVENTS: every connection stalling, which would leave the reconnect a test
// waits for unread too.
func TestStopReadingLeavesOtherConnectionsRead(t *testing.T) {
	local, remote := tcpPair(t)
	peer := newStopReadingPeer(t, 2, 2,
		"expect=bgp:conn=1:seq=1:hex="+unmetEOR,
		"expect=bgp:conn=2:seq=1:hex="+unmetEOR)

	done := make(chan Result, 1)
	go serveConnection(t.Context(), peer, local, done)
	if err := establishAndSendEOR(remote); err != nil {
		t.Fatalf("establish: %v", err)
	}

	// The remote end stays open: a stalled peer would hold the connection until
	// it closed, and never return here.
	result := awaitResult(t, done)
	if !result.Success {
		t.Fatalf("connection 1 failed: %v", result.Error)
	}
}
