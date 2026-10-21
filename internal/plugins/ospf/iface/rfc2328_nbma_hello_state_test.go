// Design: docs/architecture/ospf/ospf-5-interface-ism.md -- NBMA Hello sending.
// Related: iface.go -- Start and Stop own the Hello timer; nbma.go -- helloTargetsLocked.
//
// VALIDATES: RFC 2328 Section 9.5.1, "The interface state must be at least Waiting for any
// Hello Packets to be sent out the NBMA interface."
// PREVENTS: an NBMA interface that sends Hellos while Down, before it is started or after it
// is stopped.
package iface

import (
	"net/netip"
	"sync"
	"testing"
	"time"
)

// lockedDstSender records each destination; the Hello timer goroutine writes it.
type lockedDstSender struct {
	mu   sync.Mutex
	dsts []netip.Addr
}

func (s *lockedDstSender) SendPacket(_ string, dst netip.Addr, _ []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dsts = append(s.dsts, dst)
	return nil
}
func (s *lockedDstSender) JoinAllDRouters(string) error  { return nil }
func (s *lockedDstSender) LeaveAllDRouters(string) error { return nil }

func (s *lockedDstSender) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.dsts)
}

// nbmaTimerConfig is an eligible NBMA interface whose Hello timer and poll interval are one
// second, with one configured neighbor that has not been heard.
func nbmaTimerConfig(t *testing.T, neighbor netip.Addr) Config {
	t.Helper()
	cfg := nbmaConfig(t)
	cfg.HelloInterval = 1
	cfg.PollInterval = 1
	cfg.NBMANeighbors = []NBMANeighbor{{Address: neighbor, Priority: 1}}
	return cfg
}

// RFC requirement: RFC2328-9.5.1-2 positive -- once Start brings an eligible NBMA interface to state Waiting, its Hello timer sends a unicast Hello to the configured neighbor 10.0.0.2 within 1.5 seconds (HelloInterval 1), and the interface is still in state Waiting when it does (Start, helloLoop, sendHelloAt).
//
// Goal: Hellos leave an NBMA interface once it reaches Waiting. Method: start the interface
// with a one-second Hello timer and wait for the first send to the configured neighbor.
func TestRFC2328NBMAHelloSentOnceWaiting(t *testing.T) {
	neighbor := netip.MustParseAddr("10.0.0.2")
	sender := &lockedDstSender{}
	ifc := New(nbmaTimerConfig(t, neighbor), sender, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	if got := ifc.State(); got != StateWaiting {
		t.Fatalf("state after Start = %s, want %s", got, StateWaiting)
	}
	deadline := time.Now().Add(1500 * time.Millisecond)
	for sender.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if sender.count() == 0 {
		t.Fatal("no Hello was sent within 1.5 s of the NBMA interface reaching Waiting")
	}
	sender.mu.Lock()
	first := sender.dsts[0]
	sender.mu.Unlock()
	if first != neighbor {
		t.Fatalf("first NBMA Hello went to %s, want the configured neighbor %s", first, neighbor)
	}
	if got := ifc.State(); got != StateWaiting {
		t.Fatalf("state when the Hello was sent = %s, want %s", got, StateWaiting)
	}
}

// RFC requirement: RFC2328-9.5.1-2 negative -- an NBMA interface in state Down sends no Hello: over 2.5 seconds (HelloInterval 1) an interface that was never started sends nothing, and an interface that was started and then stopped (state Down) sends nothing after Stop returns (Start, Stop).
//
// Goal: no Hello leaves an NBMA interface below Waiting. Method: watch a never-started
// interface and a stopped one for more than two Hello intervals each.
func TestRFC2328NBMANoHelloWhileDown(t *testing.T) {
	neighbor := netip.MustParseAddr("10.0.0.2")

	idle := &lockedDstSender{}
	never := New(nbmaTimerConfig(t, neighbor), idle, NopMetrics())
	if got := never.State(); got != StateDown {
		t.Fatalf("state before Start = %s, want %s", got, StateDown)
	}

	stopped := &lockedDstSender{}
	ifc := New(nbmaTimerConfig(t, neighbor), stopped, NopMetrics())
	ifc.Start()
	ifc.Stop()
	if got := ifc.State(); got != StateDown {
		t.Fatalf("state after Stop = %s, want %s", got, StateDown)
	}
	atStop := stopped.count()

	time.Sleep(2500 * time.Millisecond)
	if n := idle.count(); n != 0 {
		t.Fatalf("a never-started NBMA interface (state Down) sent %d Hellos", n)
	}
	if n := stopped.count() - atStop; n != 0 {
		t.Fatalf("a stopped NBMA interface (state Down) sent %d Hellos after Stop", n)
	}
}
