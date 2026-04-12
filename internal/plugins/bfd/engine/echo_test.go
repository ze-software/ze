package engine

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/transport"
)

// echoReqFor builds a SessionRequest with echo negotiated: both
// DesiredMinEchoTxInterval non-zero locally and the test primes
// the RemoteMinEchoRxInterval by calling Machine.PokeRemoteMinEcho
// through the Loop's byDiscr lookup so EchoEnabled returns true
// without running a full three-way handshake.
func echoReqFor(peer, local string) api.SessionRequest {
	return api.SessionRequest{
		Peer:                     netip.MustParseAddr(peer),
		Local:                    netip.MustParseAddr(local),
		Interface:                "echo0",
		Mode:                     api.SingleHop,
		DesiredMinTxInterval:     10_000,
		RequiredMinRxInterval:    10_000,
		DetectMult:               3,
		DesiredMinEchoTxInterval: 10_000,
	}
}

// echoHook is an in-memory MetricsHook that counts every echo event
// so tests can assert the scheduler fired and the RX path observed
// the histogram.
type echoHook struct {
	mu     sync.Mutex
	txs    atomic.Int64
	rxs    atomic.Int64
	rtts   []time.Duration
	others map[string]int
}

func newEchoHook() *echoHook {
	return &echoHook{others: map[string]int{}}
}

func (h *echoHook) OnStateChange(_, _ packet.State, _ packet.Diag, _, _ string) {}
func (h *echoHook) OnTxPacket(_ string)                                         {}
func (h *echoHook) OnRxPacket(_ string)                                         {}
func (h *echoHook) OnAuthFailure(_ string)                                      {}
func (h *echoHook) OnEchoTx(_ string)                                           { h.txs.Add(1) }
func (h *echoHook) OnEchoRx(_ string)                                           { h.rxs.Add(1) }
func (h *echoHook) OnEchoRTT(_ string, rtt time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.rtts = append(h.rtts, rtt)
}

// VALIDATES: spec-bfd-6b AC-1 -- when two Loops share paired echo
// transports and both sessions have echo negotiated, the express-loop
// tick fires periodic echo TX and the RX path matches returning
// echoes to compute a round-trip observation.
// PREVENTS: regression where echoTickLocked never runs (wiring miss),
// handleEchoInbound drops valid ZEEC envelopes, or OnEchoTx/OnEchoRx
// stop firing.
func TestEchoRoundTrip(t *testing.T) {
	addrAA := netip.MustParseAddr(addrA)
	addrBB := netip.MustParseAddr(addrB)

	controlA, controlB := transport.Pair(api.SingleHop, addrAA, addrBB)
	echoLbA, echoLbB := transport.Pair(api.SingleHop, addrAA, addrBB)

	loopA := NewLoopWithEcho(controlA, echoLbA, clock.RealClock{})
	loopB := NewLoopWithEcho(controlB, echoLbB, clock.RealClock{})

	hookA := newEchoHook()
	hookB := newEchoHook()
	loopA.SetMetricsHook(hookA)
	loopB.SetMetricsHook(hookB)

	if err := loopA.Start(); err != nil {
		t.Fatalf("loopA.Start: %v", err)
	}
	defer func() {
		if err := loopA.Stop(); err != nil {
			t.Errorf("loopA.Stop: %v", err)
		}
	}()
	if err := loopB.Start(); err != nil {
		t.Fatalf("loopB.Start: %v", err)
	}
	defer func() {
		if err := loopB.Stop(); err != nil {
			t.Errorf("loopB.Stop: %v", err)
		}
	}()

	hA, err := loopA.EnsureSession(echoReqFor(addrB, addrA))
	if err != nil {
		t.Fatalf("loopA.EnsureSession: %v", err)
	}
	hB, err := loopB.EnsureSession(echoReqFor(addrA, addrB))
	if err != nil {
		t.Fatalf("loopB.EnsureSession: %v", err)
	}
	subA := hA.Subscribe()
	subB := hB.Subscribe()
	defer hA.Unsubscribe(subA)
	defer hB.Unsubscribe(subB)

	// The three-way handshake is required for both machines to reach Up
	// so echoTickLocked stops clearing the echo schedule.
	deadline := time.Now().Add(6 * time.Second)
	var upA, upB bool
	for !upA || !upB {
		if time.Now().After(deadline) {
			t.Fatalf("handshake not Up (upA=%v upB=%v)", upA, upB)
		}
		select {
		case ev := <-subA:
			if ev.State == packet.StateUp {
				upA = true
			}
		case ev := <-subB:
			if ev.State == packet.StateUp {
				upB = true
			}
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Wait for the echo scheduler to fire at least a few times on both
	// sides, then verify the RX path ran and recorded RTT samples.
	deadline = time.Now().Add(3 * time.Second)
	for hookA.txs.Load() < 3 || hookB.txs.Load() < 3 ||
		hookA.rxs.Load() < 1 || hookB.rxs.Load() < 1 {
		if time.Now().After(deadline) {
			t.Fatalf("echo counters too low: txA=%d txB=%d rxA=%d rxB=%d",
				hookA.txs.Load(), hookB.txs.Load(),
				hookA.rxs.Load(), hookB.rxs.Load())
		}
		time.Sleep(20 * time.Millisecond)
	}

	hookA.mu.Lock()
	rttSamples := len(hookA.rtts)
	hookA.mu.Unlock()
	if rttSamples == 0 {
		t.Fatalf("no RTT samples recorded on hookA")
	}
}

// dropEchoTransport is a Transport that accepts Sends silently and
// never delivers any inbound packets. Used by
// TestEchoDetectionSwitchover to simulate a peer that is dropping
// every echo packet so the engine's echo detection path fires.
//
// Start / Stop are idempotent. The RX channel is never closed mid-test
// so the express loop keeps selecting on a live channel; Stop closes
// it so Loop.run's range exits cleanly.
type dropEchoTransport struct {
	rx chan transport.Inbound
}

func newDropEchoTransport() *dropEchoTransport {
	return &dropEchoTransport{rx: make(chan transport.Inbound)}
}

func (*dropEchoTransport) Start() error                    { return nil }
func (*dropEchoTransport) Send(_ transport.Outbound) error { return nil }
func (t *dropEchoTransport) RX() <-chan transport.Inbound  { return t.rx }
func (t *dropEchoTransport) Stop() error {
	select {
	case <-t.rx:
	default:
		close(t.rx)
	}
	return nil
}

// VALIDATES: spec-bfd-6c Phase B.2 -- when echo is negotiated and
// Up but no reflected echoes return within DetectMult * EchoInterval,
// the engine declares the session Down with DiagEchoFailed.
// PREVENTS: regression where EchoFail never fires, fires with the
// wrong diagnostic, or fails to clear the outstanding ring on
// teardown.
func TestEchoDetectionSwitchover(t *testing.T) {
	addrAA := netip.MustParseAddr(addrA)
	addrBB := netip.MustParseAddr(addrB)

	controlA, controlB := transport.Pair(api.SingleHop, addrAA, addrBB)
	loopA := NewLoopWithEcho(controlA, newDropEchoTransport(), clock.RealClock{})
	loopB := NewLoopWithEcho(controlB, newDropEchoTransport(), clock.RealClock{})

	if err := loopA.Start(); err != nil {
		t.Fatalf("loopA.Start: %v", err)
	}
	defer func() {
		if err := loopA.Stop(); err != nil {
			t.Errorf("loopA.Stop: %v", err)
		}
	}()
	if err := loopB.Start(); err != nil {
		t.Fatalf("loopB.Start: %v", err)
	}
	defer func() {
		if err := loopB.Stop(); err != nil {
			t.Errorf("loopB.Stop: %v", err)
		}
	}()

	hA, err := loopA.EnsureSession(echoReqFor(addrB, addrA))
	if err != nil {
		t.Fatalf("loopA.EnsureSession: %v", err)
	}
	if _, err := loopB.EnsureSession(echoReqFor(addrA, addrB)); err != nil {
		t.Fatalf("loopB.EnsureSession: %v", err)
	}

	// Wait for the handshake to reach Up. Subsequent transitions
	// (Up -> Down with DiagEchoFailed) are observed via subA.
	subA := hA.Subscribe()
	deadline := time.Now().Add(6 * time.Second)
	upA := false
	for !upA {
		if time.Now().After(deadline) {
			t.Fatalf("handshake not Up before deadline")
		}
		select {
		case ev := <-subA:
			if ev.State == packet.StateUp {
				upA = true
			}
		case <-time.After(50 * time.Millisecond):
		}
	}

	// After Up the drop transport swallows every outbound echo so
	// the per-session outstanding ring fills up without any matching
	// reflections. The engine tick then observes
	// EchoDetectionExpired and calls EchoFail which publishes a
	// Down event with DiagEchoFailed.
	//
	// Detection time = DetectMult (3) * EchoInterval (10ms) = 30ms;
	// the poll interval is 5ms so the first expiry observation
	// happens ~35ms after the first echo TX. A 2-second deadline is
	// generously above that floor to absorb scheduler jitter under
	// load.
	deadline = time.Now().Add(2 * time.Second)
	gotEchoFailed := false
	for !gotEchoFailed {
		if time.Now().After(deadline) {
			t.Fatalf("no echo-failed transition within 2s")
		}
		select {
		case ev := <-subA:
			if ev.State == packet.StateDown && ev.Diag == packet.DiagEchoFailed {
				gotEchoFailed = true
			}
		case <-time.After(20 * time.Millisecond):
		}
	}
}
