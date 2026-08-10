// Tests for the dedicated Basic Discovery reader goroutine (spec
// plan/spec-fixit-ldp-hello-read-loop): inbound Hellos must be drained
// continuously and decoupled from the Hello send tick, so a shared segment with
// N neighbors does not drop N-1 Hellos per interval and flap adjacencies.
//
// The reader (readDiscoveryLoop) drains any *net.UDPConn, so these tests drive it
// over plain loopback unicast UDP rather than multicast: the drain contract is
// identical and unicast keeps the tests deterministic across environments. Before
// the fix ReadFromUDP was gated behind the helloTicker select in discoverOnInterface,
// so the socket was drained one datagram per HelloInterval (5s); these tests assert
// the post-fix behavior where every datagram is consumed promptly.
package ldp

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// buildHelloDatagram encodes a full LDP PDU carrying one Basic Discovery Hello,
// identical to sendHello's on-wire framing, for a given neighbor LSR-ID.
func buildHelloDatagram(lsrID [4]byte, transport netip.Addr) []byte {
	var buf [128]byte
	bodyLen := EncodeHello(buf[ldpHeaderLen:], HelloMessage{
		MessageID:     1,
		HoldTime:      15,
		TransportAddr: transport,
	})
	pduLen := uint16(bodyLen + 6)
	encodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  pduLen,
		LSRID:      lsrID,
		LabelSpace: 0,
	})
	out := make([]byte, ldpHeaderLen+bodyLen)
	copy(out, buf[:ldpHeaderLen+bodyLen])
	return out
}

// newLoopbackUDP binds a UDP socket on loopback and returns it with its concrete
// address for peers to send to.
func newLoopbackUDP(t *testing.T) (*net.UDPConn, *net.UDPAddr) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("LocalAddr is not *net.UDPAddr: %T", conn.LocalAddr())
	}
	return conn, addr
}

// waitFor polls cond until true or the deadline elapses.
func waitFor(deadline time.Duration, cond func() bool) bool {
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

// VALIDATES: AC-1 wiring -- discoverOnInterface itself (not just the extracted
// readDiscoveryLoop) spawns the reader goroutine, drains an inbound burst, and
// returns on ctx cancel. Substitutes the multicast listen with a loopback socket
// via the listenDiscovery seam.
// PREVENTS: a regression that deletes the `go readDiscoveryLoop(...)` launch in
// discoverOnInterface (register.go) -- which the direct-call tests below would miss,
// since only the integration-tagged FRR test drives discoverOnInterface on the wire.
// Leak-freedom of the reader/sender teardown is covered precisely by -race plus
// TestDiscoveryReaderExitsOnCancel*; this test locks the launch + drain + return.
func TestDiscoverOnInterfaceSpawnsReaderAndDrains(t *testing.T) {
	recvConn, recvAddr := newLoopbackUDP(t)
	t.Cleanup(func() { _ = recvConn.Close() }) // idempotent: discoverOnInterface closes it on exit
	origListen := listenDiscovery
	listenDiscovery = func(_ *net.Interface, _ *net.UDPAddr) (*net.UDPConn, error) {
		return recvConn, nil
	}
	t.Cleanup(func() { listenDiscovery = origListen })

	adjTable := newAdjacencyTable()
	lsrID := [4]byte{1, 1, 1, 1}
	cfg := ldpConfig{
		HelloInterval: 50 * time.Millisecond,
		HelloHoldTime: 15 * time.Second,
		TransportAddr: netip.MustParseAddr("1.1.1.1"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	interfaceDone := make(chan struct{})
	go func() {
		defer close(interfaceDone)
		// ifName "" -> ifi nil -> no multicast egress pinning; discoverOnInterface
		// reads from the substituted loopback socket and closes it on ctx cancel.
		discoverOnInterface(ctx, slogutil.DiscardLogger(), cfg, lsrID, "", adjTable, nil)
	}()

	const n = 4
	senderConn, _ := newLoopbackUDP(t)
	t.Cleanup(func() { _ = senderConn.Close() })
	for i := 1; i <= n; i++ {
		lsr := [4]byte{10, 0, 0, byte(i)}
		dg := buildHelloDatagram(lsr, netip.AddrFrom4(lsr))
		if _, err := senderConn.WriteToUDP(dg, recvAddr); err != nil {
			t.Fatalf("write hello %d: %v", i, err)
		}
	}

	if !waitFor(3*time.Second, func() bool { return adjTable.Len() == n }) {
		t.Fatalf("discoverOnInterface did not drain burst: adjTable.Len() = %d, want %d", adjTable.Len(), n)
	}

	// ctx cancel must tear down BOTH the send loop and the reader goroutine, and
	// discoverOnInterface must return (join the reader) with no leak.
	cancel()
	select {
	case <-interfaceDone:
	case <-time.After(2 * time.Second):
		t.Fatal("discoverOnInterface did not return within 2s of ctx cancel (reader/sender leak)")
	}
}

// VALIDATES: AC-1 -- the dedicated reader drains a back-to-back burst of N Hellos
// arriving within one HelloInterval; all N adjacencies appear well before the hold
// time.
// PREVENTS: a regression to the pre-fix loop that gated ReadFromUDP behind the
// helloTicker select and drained only one datagram per 5s interval, dropping the
// other N-1 Hellos and flapping adjacencies on a shared segment.
func TestDiscoveryDrainsBurst(t *testing.T) {
	const n = 5
	adjTable := newAdjacencyTable()
	recvConn, recvAddr := newLoopbackUDP(t)
	localLSRID := [4]byte{1, 1, 1, 1}

	senderConn, _ := newLoopbackUDP(t)
	t.Cleanup(func() { _ = senderConn.Close() })

	// Burst N distinct-neighbor Hellos back-to-back, all inside one HelloInterval.
	for i := 1; i <= n; i++ {
		lsr := [4]byte{10, 0, 0, byte(i)}
		dg := buildHelloDatagram(lsr, netip.AddrFrom4(lsr))
		if _, err := senderConn.WriteToUDP(dg, recvAddr); err != nil {
			t.Fatalf("write hello %d: %v", i, err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go readDiscoveryLoop(ctx, recvConn, localLSRID, "eth0", adjTable, nil, slogutil.DiscardLogger(), done)

	// All N must be drained far inside DefaultHelloHoldTime (15s).
	if !waitFor(3*time.Second, func() bool { return adjTable.Len() == n }) {
		t.Fatalf("adjTable.Len() = %d, want %d (burst not fully drained)", adjTable.Len(), n)
	}

	cancel()
	if err := recvConn.Close(); err != nil {
		t.Fatalf("recvConn close: %v", err)
	}
	<-done
}

// VALIDATES: AC-2 -- a single Hello arriving at an arbitrary instant is consumed
// promptly by the continuous reader.
// PREVENTS: a regression to the timing-fragile pre-fix loop where a lone neighbor's
// Hello was consumed only if it landed inside the 1s read window that opened once
// per 5s, otherwise its hold timer flapped.
func TestDiscoverySingleHelloPrompt(t *testing.T) {
	adjTable := newAdjacencyTable()
	recvConn, recvAddr := newLoopbackUDP(t)
	localLSRID := [4]byte{1, 1, 1, 1}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go readDiscoveryLoop(ctx, recvConn, localLSRID, "eth0", adjTable, nil, slogutil.DiscardLogger(), done)

	// Let the reader block on Read first, then deliver one Hello.
	senderConn, _ := newLoopbackUDP(t)
	t.Cleanup(func() { _ = senderConn.Close() })
	lsr := [4]byte{10, 0, 0, 9}
	dg := buildHelloDatagram(lsr, netip.AddrFrom4(lsr))
	if _, err := senderConn.WriteToUDP(dg, recvAddr); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	// Must appear well under the pre-fix worst case (~5s tick + read window).
	if !waitFor(500*time.Millisecond, func() bool { return adjTable.Len() == 1 }) {
		t.Fatalf("single Hello not consumed promptly: adjTable.Len() = %d", adjTable.Len())
	}

	cancel()
	if err := recvConn.Close(); err != nil {
		t.Fatalf("recvConn close: %v", err)
	}
	<-done
}

// VALIDATES: AC-3 -- on ctx cancel + socket close the reader goroutine returns and
// closes done; no goroutine or socket leak (asserted -race clean by CI).
// PREVENTS: a reader that outlives its interface's ctx and leaks across config reloads.
func TestDiscoveryReaderExitsOnCancel(t *testing.T) {
	adjTable := newAdjacencyTable()
	recvConn, _ := newLoopbackUDP(t)
	localLSRID := [4]byte{1, 1, 1, 1}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go readDiscoveryLoop(ctx, recvConn, localLSRID, "eth0", adjTable, nil, slogutil.DiscardLogger(), done)

	// Reader is blocked on Read with no inbound traffic. Cancel + close, as
	// discoverOnInterface does on reload/shutdown, and require a prompt exit.
	cancel()
	if err := recvConn.Close(); err != nil {
		t.Fatalf("recvConn close: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("reader did not exit within 2s of ctx cancel + socket close")
	}
}

// VALIDATES: AC-3 / spec risk A-3 -- even if the socket close is missed, the 1s
// read-deadline backstop wakes the reader so it re-checks ctx and returns. Cancels
// ctx WITHOUT closing the socket.
// PREVENTS: a reader wedged forever in a blocked ReadFromUDP when a close is lost.
func TestDiscoveryReaderExitsOnCancelDeadlineBackstop(t *testing.T) {
	adjTable := newAdjacencyTable()
	recvConn, _ := newLoopbackUDP(t)
	t.Cleanup(func() { _ = recvConn.Close() })
	localLSRID := [4]byte{1, 1, 1, 1}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go readDiscoveryLoop(ctx, recvConn, localLSRID, "eth0", adjTable, nil, slogutil.DiscardLogger(), done)

	cancel() // no socket close: rely on the deadline backstop to unblock the Read

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("reader did not exit via deadline backstop within 3s")
	}
}

// VALIDATES: AC-4 / spec risk A-1 -- the send path keeps its ticker cadence under a
// continuous inbound Hello flood on the same socket (net.UDPConn tolerates one
// concurrent Read and Write); concurrent reads still populate the adjacency table.
// PREVENTS: a regression where reads gate sends (or vice versa), starving Hello
// egress or the drain.
func TestDiscoverySendUnaffectedByReads(t *testing.T) {
	adjTable := newAdjacencyTable()
	nodeConn, nodeAddr := newLoopbackUDP(t) // our discovery socket
	peerConn, peerAddr := newLoopbackUDP(t) // neighbor: floods us and receives our Hellos
	localLSRID := [4]byte{1, 1, 1, 1}
	cfg := ldpConfig{
		HelloInterval: 50 * time.Millisecond,
		HelloHoldTime: 15 * time.Second,
		TransportAddr: netip.MustParseAddr("1.1.1.1"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Reader drains inbound Hellos from the neighbor.
	readerDone := make(chan struct{})
	go readDiscoveryLoop(ctx, nodeConn, localLSRID, "eth0", adjTable, nil, slogutil.DiscardLogger(), readerDone)

	// Neighbor floods us with Hellos as fast as it can until ctx cancel.
	floodDone := make(chan struct{})
	go func() {
		defer close(floodDone)
		lsr := [4]byte{10, 0, 0, 2}
		dg := buildHelloDatagram(lsr, netip.AddrFrom4(lsr))
		for ctx.Err() == nil {
			if _, err := peerConn.WriteToUDP(dg, nodeAddr); err != nil {
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	// Neighbor counts the Hellos we send it.
	var recvCount int
	peerRecvDone := make(chan struct{})
	go func() {
		defer close(peerRecvDone)
		buf := make([]byte, 512)
		for ctx.Err() == nil {
			if err := peerConn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
				return
			}
			if _, _, err := peerConn.ReadFromUDP(buf); err != nil {
				continue
			}
			recvCount++
		}
	}()

	// Send loop mirroring discoverOnInterface's post-fix select: send-only ticker.
	helloTicker := time.NewTicker(cfg.HelloInterval)
	defer helloTicker.Stop()
	sendHello(nodeConn, peerAddr, localLSRID, cfg, slogutil.DiscardLogger())
	deadline := time.After(400 * time.Millisecond)
sendLoop:
	for {
		select {
		case <-deadline:
			break sendLoop
		case <-helloTicker.C:
			sendHello(nodeConn, peerAddr, localLSRID, cfg, slogutil.DiscardLogger())
		}
	}

	cancel()
	if err := nodeConn.Close(); err != nil {
		t.Fatalf("nodeConn close: %v", err)
	}
	if err := peerConn.Close(); err != nil {
		t.Fatalf("peerConn close: %v", err)
	}
	<-readerDone
	<-floodDone
	<-peerRecvDone

	// ~400ms / 50ms plus the initial Hello ≈ 8 sends; require a clear majority to
	// survive scheduling jitter. If reads gated writes this would be near zero.
	if recvCount < 4 {
		t.Fatalf("neighbor received %d Hellos, want >= 4 (send cadence starved by reads)", recvCount)
	}
	// Reads worked concurrently: the neighbor's flood produced an adjacency.
	if adjTable.Len() != 1 {
		t.Fatalf("adjTable.Len() = %d, want 1 (inbound Hellos not drained during sends)", adjTable.Len())
	}
}
