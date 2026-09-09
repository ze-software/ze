// Design: docs/research/l2tpv2-ze-integration.md -- receiver goroutines and a failing socket
//
// Goal: prove rsReaderLoop paces a read error it cannot classify, the same
// way TestListenerReadLoopPacesAFailingSocket
// (internal/component/l2tp/listener_test.go) proves it for readLoop.
// Method: force a real, persistent, non-context-done error by setting the
// socket's own read deadline into the past, then measure how long a
// legitimate packet takes to be picked up once the deadline lifts.

//go:build linux

package ppp

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"golang.org/x/net/ipv6"
)

// TestRSReaderLoopPacesAFailingSocket
// VALIDATES: AC-1 for rsReaderLoop: a socket that fails on every read does
// not spin the goroutine at full speed.
// PREVENTS: rsReaderLoop retrying an unclassified read error at once,
// forever.
//
// Wraps a plain UDP6 socket in ipv6.NewPacketConn rather than opening a raw
// ICMPv6 socket: rsReaderLoop only calls pc.ReadFrom, so the protocol and
// payload are irrelevant to the error path under test, and a UDP socket
// needs no elevated privilege the sandbox this runs in may lack.
func TestRSReaderLoopPacesAFailingSocket(t *testing.T) {
	udpConn, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.ParseIP("::1"), Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer udpConn.Close() //nolint:errcheck // test cleanup

	pc := ipv6.NewPacketConn(udpConn)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	rsCh := make(chan struct{}, 1)
	go rsReaderLoop(ctx, pc, rsCh, "test0", slog.Default())

	// Force every read already in flight, and every one after it, to fail
	// at once and persistently: a deadline already in the past. Not
	// context-done, and not recovered by the next attempt.
	if err := udpConn.SetReadDeadline(time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	// 400ms lands inside the pacer's seventh wait: the growth curve
	// 0, 10, 20, 40, 80, 160ms then the 250ms ceiling crosses 310ms
	// before that wait starts, the same timeline
	// TestListenerReadLoopPacesAFailingSocket documents.
	time.Sleep(400 * time.Millisecond)

	// Lift the deadline and send a legitimate packet. A spinning
	// (unpaced) rsReaderLoop would already be blocked in pc.ReadFrom
	// waiting for it and would signal rsCh in well under a millisecond.
	if err := udpConn.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear read deadline: %v", err)
	}

	client, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.ParseIP("::1"), Port: 0})
	if err != nil {
		t.Fatalf("client listen: %v", err)
	}
	defer client.Close() //nolint:errcheck // test cleanup

	udpLocalAddr, ok := udpConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("udpConn.LocalAddr() = %T, want *net.UDPAddr", udpConn.LocalAddr())
	}
	dst := &net.UDPAddr{IP: net.ParseIP("::1"), Port: udpLocalAddr.Port}
	sent := time.Now()
	if _, err := client.WriteToUDP([]byte{0x01}, dst); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case <-rsCh:
		elapsed := time.Since(sent)
		if elapsed < 30*time.Millisecond {
			t.Fatalf("rsCh signaled in %s: rsReaderLoop is not pacing its retries", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for rsReaderLoop to signal rsCh after the failing socket recovered")
	}
}

// TestRSReaderLoopStopsPromptlyWhilePacing
// VALIDATES: AC-5 for rsReaderLoop: canceling ctx while the pacer is
// waiting at the ceiling must not sit out that wait. Also the "at least
// one of the three context-driven loops" prompt-exit assertion the phase
// brief requires, alongside TestListenerReadLoopStopsPromptlyWhilePacing
// for the listener and TestDiscoveryReaderStopsPromptlyWhilePacing for
// discoveryReader.
// PREVENTS: a goroutine that outlives its owner (ai/rules/goroutine-lifecycle.md).
func TestRSReaderLoopStopsPromptlyWhilePacing(t *testing.T) {
	udpConn, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.ParseIP("::1"), Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer udpConn.Close() //nolint:errcheck // test cleanup

	pc := ipv6.NewPacketConn(udpConn)
	ctx, cancel := context.WithCancel(t.Context())

	rsCh := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		rsReaderLoop(ctx, pc, rsCh, "test0", slog.Default())
		close(done)
	}()

	if err := udpConn.SetReadDeadline(time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	// Long enough that the pacer has reached its ceiling wait (see the
	// timeline note in TestRSReaderLoopPacesAFailingSocket).
	time.Sleep(400 * time.Millisecond)

	start := time.Now()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("rsReaderLoop did not return after ctx was canceled")
	}
	elapsed := time.Since(start)

	// Generous on purpose, and still far below one ceiling-length wait:
	// a return that sat out the delay would take at least 100+ms longer.
	if elapsed > 150*time.Millisecond {
		t.Fatalf("rsReaderLoop took %s to return while pacing at the ceiling: it sat out the delay", elapsed)
	}
}
