package l2tp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

// newTestEngine constructs an engine with deterministic parameters for
// tests. RecvWindow=4 keeps reorder tests small; InitialPeerRWS=4 matches
// the default; rtimeout=1s, cap=16s, maxRetransmit=3 yields a
// schedule 1+2+4 = 7s (short enough to exercise retention without
// thousand-second tests).
func newTestEngine() *ReliableEngine {
	return NewReliableEngine(ReliableConfig{
		LocalTunnelID:  100,
		PeerTunnelID:   200,
		RTimeout:       time.Second,
		RTimeoutCap:    16 * time.Second,
		MaxRetransmit:  3,
		RecvWindow:     4,
		InitialPeerRWS: 4,
	})
}

// messageTypeAVP returns the encoded Message Type AVP bytes for a given
// message type, as used as the first AVP in every control message.
// Shape: M=1 H=0 rsvd=0 Length=8 VendorID=0 AttrType=0 Value=msgType.
func messageTypeAVP(msgType uint16) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint16(b[0:2], 0x8008) // M=1, Length=8
	// VendorID=0, AttrType=0 already zero.
	binary.BigEndian.PutUint16(b[6:8], msgType)
	return b
}

// parseSent parses a [12+]byte control message produced by the engine
// so tests can read the stamped Ns/Nr without depending on phase-1
// internals beyond ParseMessageHeader.
func parseSent(t *testing.T, sent []byte) MessageHeader {
	t.Helper()
	hdr, err := ParseMessageHeader(sent)
	if err != nil {
		t.Fatalf("ParseMessageHeader: %v", err)
	}
	return hdr
}

// mustEnqueue wraps Enqueue to make error-free invocations explicit,
// satisfying the errcheck linter while keeping the happy-path tests
// terse. All current unit tests use sid=0 (tunnel-scoped messages like
// SCCRQ/SCCRP/HELLO); integration tests in reliable_integration_test.go
// exercise session-scoped IDs.
//
//nolint:unparam // sid varies in integration tests; unit tests stay tunnel-scoped
func mustEnqueue(t *testing.T, e *ReliableEngine, sid uint16, body []byte, now time.Time) []byte {
	t.Helper()
	sent, err := e.Enqueue(sid, body, now)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	return sent
}

// VALIDATES: AC-2 happy path enqueue with open window assigns Ns=0 and
// carries the engine's current nextRecvSeq as Nr.
func TestEnqueueOpenWindow(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)
	body := messageTypeAVP(1) // SCCRQ

	sent, err := e.Enqueue(0, body, now)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if sent == nil {
		t.Fatalf("Enqueue returned nil; expected bytes (window open)")
	}
	hdr := parseSent(t, sent)
	if hdr.Ns != 0 {
		t.Errorf("Ns = %d, want 0", hdr.Ns)
	}
	if hdr.Nr != 0 {
		t.Errorf("Nr = %d, want 0 (nothing received yet)", hdr.Nr)
	}
	if !hdr.IsControl {
		t.Errorf("T bit not set")
	}
	if e.Outstanding() != 1 {
		t.Errorf("Outstanding = %d, want 1", e.Outstanding())
	}
}

// VALIDATES: AC-3 enqueue beyond the window queues the message rather
// than sending. The window starts at min(cwnd=1, peerRWS=4) = 1, so the
// second Enqueue must queue.
func TestEnqueueWindowFull(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)

	sent1, _ := e.Enqueue(0, messageTypeAVP(1), now)
	if sent1 == nil {
		t.Fatalf("first Enqueue returned nil, expected bytes")
	}
	sent2, err := e.Enqueue(0, messageTypeAVP(2), now)
	if err != nil {
		t.Fatalf("second Enqueue: %v", err)
	}
	if sent2 != nil {
		t.Errorf("second Enqueue returned %d bytes; expected nil (window closed)", len(sent2))
	}
	if e.Outstanding() != 1 {
		t.Errorf("Outstanding = %d, want 1", e.Outstanding())
	}
}

// VALIDATES: AC-4 in-order receive advances nextRecvSeq and marks
// needsZLB. PREVENTS: receiving valid in-sequence messages without
// delivering them to the upper layer.
func TestOnReceiveInOrder(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)

	hdr := MessageHeader{
		IsControl: true, HasLength: true, HasSequence: true,
		Version: 2, Length: 12 + 8, TunnelID: 100, SessionID: 0,
		Ns: 0, Nr: 0,
	}
	result := e.OnReceive(hdr, messageTypeAVP(1), now)
	if result.Class != ClassDelivered {
		t.Errorf("Class = %v, want ClassDelivered", result.Class)
	}
	if len(result.Delivered) != 1 || result.Delivered[0].Ns != 0 {
		t.Errorf("Delivered = %+v, want 1 entry with Ns=0", result.Delivered)
	}
	if result.Delivered[0].MessageType != 1 {
		t.Errorf("MessageType = %d, want 1 (SCCRQ)", result.Delivered[0].MessageType)
	}
	if !e.NeedsZLB() {
		t.Errorf("NeedsZLB = false, want true after in-order receive")
	}
}

// VALIDATES: AC-5 duplicate is classified ClassDuplicate and sets
// needsZLB (MUST ACK per RFC 2661 S5.8 line 2550).
func TestOnReceiveDuplicate(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)
	hdr := MessageHeader{
		IsControl: true, HasLength: true, HasSequence: true,
		Version: 2, Length: 20, TunnelID: 100,
		Ns: 0, Nr: 0,
	}
	// First receive: in-order.
	e.OnReceive(hdr, messageTypeAVP(1), now)

	// Piggyback-ACK by enqueuing something (clears needsZLB).
	mustEnqueue(t, e, 0, messageTypeAVP(2), now)

	if e.NeedsZLB() {
		t.Fatalf("NeedsZLB not cleared after piggyback")
	}

	// Same Ns=0 arrives again: duplicate.
	result := e.OnReceive(hdr, messageTypeAVP(1), now)
	if result.Class != ClassDuplicate {
		t.Errorf("Class = %v, want ClassDuplicate", result.Class)
	}
	if len(result.Delivered) != 0 {
		t.Errorf("Delivered = %+v, want empty for duplicate", result.Delivered)
	}
	if !e.NeedsZLB() {
		t.Errorf("NeedsZLB = false, want true after duplicate (RFC MUST ACK)")
	}
}

// VALIDATES: AC-6 reorder-queued message is buffered and delivered
// in order when the gap fills.
func TestOnReceiveReorderGapFill(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)

	// Arrive Ns=1 out of order (nextRecvSeq=0 still).
	hdr1 := MessageHeader{IsControl: true, HasSequence: true, Version: 2, TunnelID: 100, Ns: 1, Nr: 0}
	r1 := e.OnReceive(hdr1, messageTypeAVP(2), now)
	if r1.Class != ClassReorderQueued {
		t.Errorf("Ns=1 classified as %v, want ClassReorderQueued", r1.Class)
	}
	if len(r1.Delivered) != 0 {
		t.Errorf("gap not filled yet: Delivered = %+v", r1.Delivered)
	}
	if e.NeedsZLB() {
		t.Errorf("NeedsZLB set while gap open -- RFC 2661 S5.8 forbids ACK before in-order")
	}

	// Now Ns=0 arrives, filling the gap. Both should deliver.
	hdr0 := MessageHeader{IsControl: true, HasSequence: true, Version: 2, TunnelID: 100, Ns: 0, Nr: 0}
	r0 := e.OnReceive(hdr0, messageTypeAVP(1), now)
	if r0.Class != ClassDelivered {
		t.Errorf("Ns=0 classified as %v, want ClassDelivered", r0.Class)
	}
	if len(r0.Delivered) != 2 {
		t.Fatalf("expected 2 delivered, got %d", len(r0.Delivered))
	}
	if r0.Delivered[0].Ns != 0 || r0.Delivered[1].Ns != 1 {
		t.Errorf("ordering wrong: %+v", r0.Delivered)
	}
	if !e.NeedsZLB() {
		t.Errorf("NeedsZLB = false, want true after gap fill")
	}
}

// VALIDATES: AC-7 reorder beyond advertised window is discarded. With
// RecvWindow=4, an Ns 5 ahead is dropped.
func TestOnReceiveReorderBeyondWindow(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)

	// RecvWindow=4: valid reorder offsets are 1..3 (Ns 1, 2, 3).
	// Ns=5 with nextRecvSeq=0 is offset 5, beyond capacity=4.
	hdr := MessageHeader{IsControl: true, HasSequence: true, Version: 2, TunnelID: 100, Ns: 5, Nr: 0}
	r := e.OnReceive(hdr, messageTypeAVP(7), now)
	if r.Class != ClassDiscarded {
		t.Errorf("Class = %v, want ClassDiscarded", r.Class)
	}
	if e.NeedsZLB() {
		t.Errorf("NeedsZLB set for discarded message -- should not ACK")
	}
}

// VALIDATES: AC-8 Nr advance clears acknowledged entries from rtms_queue
// and opens the window for queued sends.
func TestOnReceiveAckAdvance(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)

	// Send Ns=0 (window=1 consumed).
	mustEnqueue(t, e, 0, messageTypeAVP(1), now)

	// Queue Ns=1 (window full, queued).
	mustEnqueue(t, e, 0, messageTypeAVP(2), now)
	if e.Outstanding() != 1 {
		t.Fatalf("Outstanding = %d before ACK, want 1", e.Outstanding())
	}

	// Peer sends a message with Nr=1, acking our Ns=0.
	hdr := MessageHeader{IsControl: true, HasSequence: true, Version: 2, TunnelID: 100, Ns: 0, Nr: 1}
	r := e.OnReceive(hdr, messageTypeAVP(1), now)
	if len(r.NewSends) != 1 {
		t.Errorf("NewSends = %d, want 1 (queued Ns=1 promoted)", len(r.NewSends))
	}
	// Rtms should now hold Ns=1 only.
	if e.Outstanding() != 1 {
		t.Errorf("Outstanding after ACK = %d, want 1 (Ns=0 acked, Ns=1 now in flight)", e.Outstanding())
	}
}

// VALIDATES: AC-9 data messages (T=0) are classified ClassDataMessage
// and do NOT update engine state. RFC 2661 trap 24.4.
func TestOnReceiveDataMessage(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)

	hdr := MessageHeader{IsControl: false, Version: 2, TunnelID: 100, SessionID: 50, Ns: 99, Nr: 99}
	r := e.OnReceive(hdr, []byte("ppp-frame"), now)
	if r.Class != ClassDataMessage {
		t.Errorf("Class = %v, want ClassDataMessage", r.Class)
	}
	// State must not have been mutated.
	if e.nextRecvSeq != 0 {
		t.Errorf("nextRecvSeq = %d, want 0 (data message must not advance)", e.nextRecvSeq)
	}
}

// VALIDATES: AC-10 ZLB (empty body) is classified ClassZLB and does
// NOT set needsZLB (no ACK-of-ACK).
func TestOnReceiveZLB(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)

	// Send something to have an outstanding message.
	mustEnqueue(t, e, 0, messageTypeAVP(1), now)
	if e.Outstanding() != 1 {
		t.Fatalf("setup: Outstanding = %d, want 1", e.Outstanding())
	}

	// Receive a ZLB acknowledging our Ns=0 via Nr=1.
	hdr := MessageHeader{IsControl: true, HasSequence: true, Version: 2, TunnelID: 100, Ns: 0, Nr: 1}
	r := e.OnReceive(hdr, []byte{}, now)
	if r.Class != ClassZLB {
		t.Errorf("Class = %v, want ClassZLB", r.Class)
	}
	if e.Outstanding() != 0 {
		t.Errorf("Outstanding after ZLB = %d, want 0", e.Outstanding())
	}
	if e.NeedsZLB() {
		t.Errorf("NeedsZLB set after ZLB -- must not ACK a ZLB")
	}
}

// VALIDATES: AC-11 Tick retransmits outstanding messages with Nr
// rewritten. PREVENTS: the stale-Nr trap (RFC 2661 S5.8 line 2589-2590
// and S24.9).
func TestTickRetransmit(t *testing.T) {
	e := newTestEngine()
	t0 := time.Unix(0, 0)

	mustEnqueue(t, e, 0, messageTypeAVP(1), t0)
	// Advance state so nextRecvSeq has changed (peer sent a separate
	// message we processed in-order but haven't acked yet).
	hdr := MessageHeader{IsControl: true, HasSequence: true, Version: 2, TunnelID: 100, Ns: 0, Nr: 0}
	e.OnReceive(hdr, messageTypeAVP(2), t0)
	// nextRecvSeq is now 1.

	// Tick at t0+1s: deadline expires, retransmit fires.
	result := e.Tick(t0.Add(time.Second))
	if len(result.Retransmits) != 1 {
		t.Fatalf("Retransmits = %d, want 1", len(result.Retransmits))
	}
	retHdr := parseSent(t, result.Retransmits[0])
	if retHdr.Ns != 0 {
		t.Errorf("retransmit Ns = %d, want 0 (unchanged)", retHdr.Ns)
	}
	if retHdr.Nr != 1 {
		t.Errorf("retransmit Nr = %d, want 1 (updated to nextRecvSeq)", retHdr.Nr)
	}
}

// VALIDATES: AC-11 Tick backoff schedule doubles up to the cap.
func TestTickBackoffSchedule(t *testing.T) {
	e := NewReliableEngine(ReliableConfig{
		PeerTunnelID:   200,
		RTimeout:       time.Second,
		RTimeoutCap:    4 * time.Second,
		MaxRetransmit:  10,
		RecvWindow:     4,
		InitialPeerRWS: 4,
	})
	t0 := time.Unix(0, 0)
	mustEnqueue(t, e, 0, messageTypeAVP(1), t0)

	// Expected schedule: 1s, 2s, 4s, 4s (capped), 4s ...
	expected := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 4 * time.Second}
	now := t0
	for i, want := range expected {
		now = now.Add(want)
		result := e.Tick(now)
		if len(result.Retransmits) != 1 {
			t.Fatalf("tick %d: no retransmit", i)
		}
		if result.TeardownRequired {
			t.Fatalf("tick %d: teardown requested", i)
		}
	}
}

// VALIDATES: AC-12 Tick on an empty queue is a no-op.
func TestTickEmpty(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0).Add(time.Hour)
	r := e.Tick(now)
	if len(r.Retransmits) != 0 || r.TeardownRequired {
		t.Errorf("Tick on empty queue returned %+v, want empty", r)
	}
}

// VALIDATES: AC-13 max-retransmit exceeded signals teardown.
func TestTickMaxAttempts(t *testing.T) {
	e := newTestEngine() // maxRetransmit=3
	t0 := time.Unix(0, 0)
	mustEnqueue(t, e, 0, messageTypeAVP(1), t0)

	// Advance well past all deadlines. Attempts 1..3 should succeed;
	// the 4th call with queue still non-empty signals teardown.
	now := t0
	for i := 1; i <= 3; i++ {
		now = now.Add(time.Duration(1<<i) * time.Second)
		r := e.Tick(now)
		if r.TeardownRequired {
			t.Fatalf("tick attempt %d: teardown premature", i)
		}
	}
	// 4th attempt -- attempts counter reaches 4 > 3, teardown.
	now = now.Add(16 * time.Second)
	r := e.Tick(now)
	if !r.TeardownRequired {
		t.Errorf("after 3 retransmits + 1, teardown not signaled")
	}
}

// VALIDATES: AC-14 congestion reset on retransmit halves ssthresh,
// resets cwnd to 1.
func TestTickRetransmitCongestionReset(t *testing.T) {
	e := newTestEngine()
	t0 := time.Unix(0, 0)
	mustEnqueue(t, e, 0, messageTypeAVP(1), t0)

	// Force cwnd higher via acks (need another outstanding+ack cycle;
	// simplest: drive via hand-crafted received Nr advances).
	e.win.cwnd = 4
	e.win.ssthresh = 4

	e.Tick(t0.Add(time.Second))

	if e.CWND() != 1 {
		t.Errorf("cwnd after retransmit = %d, want 1", e.CWND())
	}
	if e.SSTHRESH() != 2 {
		t.Errorf("ssthresh after retransmit = %d, want 2 (=max(4/2,1))", e.SSTHRESH())
	}
}

// VALIDATES: AC-19, AC-20 NextDeadline returns the oldest deadline,
// zero when idle.
func TestNextDeadlineLifecycle(t *testing.T) {
	e := newTestEngine()
	if !e.NextDeadline().IsZero() {
		t.Errorf("NextDeadline before any send = %v, want zero", e.NextDeadline())
	}
	t0 := time.Unix(0, 0)
	mustEnqueue(t, e, 0, messageTypeAVP(1), t0)
	got := e.NextDeadline()
	want := t0.Add(time.Second)
	if !got.Equal(want) {
		t.Errorf("NextDeadline = %v, want %v", got, want)
	}
}

// VALIDATES: AC-21 NeedsZLB lifecycle: set on in-order receive,
// cleared by outbound send or BuildZLB.
func TestNeedsZLBLifecycle(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)
	if e.NeedsZLB() {
		t.Errorf("fresh engine: NeedsZLB = true, want false")
	}

	// Receive in-order: should set.
	hdr := MessageHeader{IsControl: true, HasSequence: true, Version: 2, TunnelID: 100, Ns: 0, Nr: 0}
	e.OnReceive(hdr, messageTypeAVP(1), now)
	if !e.NeedsZLB() {
		t.Errorf("after in-order receive: NeedsZLB = false, want true")
	}

	// Cleared by BuildZLB.
	buf := make([]byte, 64)
	n := e.BuildZLB(buf, 0)
	if n != 12 {
		t.Errorf("BuildZLB returned %d, want 12", n)
	}
	if e.NeedsZLB() {
		t.Errorf("after BuildZLB: NeedsZLB = true, want false")
	}
}

// VALIDATES: AC-22 BuildZLB format. Ns = nextSendSeq unchanged, Nr =
// nextRecvSeq. Does not consume Ns.
func TestBuildZLBFormat(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)

	// Advance nextSendSeq and nextRecvSeq.
	mustEnqueue(t, e, 0, messageTypeAVP(1), now) // Ns=0 consumed, nextSendSeq=1
	hdr := MessageHeader{IsControl: true, HasSequence: true, Version: 2, TunnelID: 100, Ns: 0, Nr: 1}
	e.OnReceive(hdr, messageTypeAVP(2), now) // nextRecvSeq=1, peer acked Ns=0

	nsBefore := e.nextSendSeq
	buf := make([]byte, 64)
	n := e.BuildZLB(buf, 0)
	if n != 12 {
		t.Fatalf("BuildZLB returned %d, want 12", n)
	}
	if e.nextSendSeq != nsBefore {
		t.Errorf("BuildZLB consumed Ns: before=%d, after=%d", nsBefore, e.nextSendSeq)
	}
	hdrOut := parseSent(t, buf[:n])
	if hdrOut.Ns != nsBefore {
		t.Errorf("ZLB Ns = %d, want %d (nextSendSeq)", hdrOut.Ns, nsBefore)
	}
	if hdrOut.Nr != 1 {
		t.Errorf("ZLB Nr = %d, want 1 (nextRecvSeq)", hdrOut.Nr)
	}
	if hdrOut.Length != 12 {
		t.Errorf("ZLB Length = %d, want 12", hdrOut.Length)
	}
}

// VALIDATES: AC-23 Close transitions to closed state.
func TestCloseTransitions(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)
	e.Close(now)
	if !e.closed {
		t.Errorf("Close did not set closed flag")
	}
	if !e.closedAt.Equal(now) {
		t.Errorf("closedAt = %v, want %v", e.closedAt, now)
	}

	// Double Close is idempotent.
	e.Close(now.Add(time.Hour))
	if !e.closedAt.Equal(now) {
		t.Errorf("second Close updated closedAt; should be idempotent")
	}
}

// VALIDATES: AC-24, AC-25 Expired returns false before retention elapses
// and true after.
func TestExpired(t *testing.T) {
	e := newTestEngine() // rtimeout=1s, cap=16s, maxRetransmit=3 -> retention=7s
	t0 := time.Unix(0, 0)

	if e.Expired(t0.Add(time.Hour)) {
		t.Errorf("Expired before Close returned true")
	}

	e.Close(t0)
	if e.Expired(t0.Add(6 * time.Second)) {
		t.Errorf("Expired at t+6s, retention is 7s")
	}
	if !e.Expired(t0.Add(7 * time.Second)) {
		t.Errorf("not Expired at t+7s, retention should have elapsed")
	}
	if !e.Expired(t0.Add(100 * time.Second)) {
		t.Errorf("not Expired at t+100s")
	}
}

// VALIDATES: send_queue bounded at MaxSendQueueDepth. Prevents
// unbounded memory growth if the peer never acknowledges.
func TestEnqueueSendQueueCap(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)
	// First Enqueue goes on the wire (window=1). Every subsequent one
	// queues. The (1 + MaxSendQueueDepth)-th Enqueue must fail.
	mustEnqueue(t, e, 0, messageTypeAVP(1), now)
	for range MaxSendQueueDepth {
		if _, err := e.Enqueue(0, messageTypeAVP(1), now); err != nil {
			t.Fatalf("Enqueue within cap failed: %v", err)
		}
	}
	_, err := e.Enqueue(0, messageTypeAVP(1), now)
	if !errors.Is(err, ErrSendQueueFull) {
		t.Errorf("Enqueue past cap: err = %v, want ErrSendQueueFull", err)
	}
}

// VALIDATES: ErrBodyTooLarge when body exceeds 10-bit Length field.
func TestEnqueueBodyTooLarge(t *testing.T) {
	e := newTestEngine()
	big := make([]byte, 1024) // 1024 + 12 > 0x03FF
	_, err := e.Enqueue(0, big, time.Unix(0, 0))
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Errorf("err = %v, want ErrBodyTooLarge", err)
	}
}

// VALIDATES: ErrBodyEmpty rejects empty or under-sized body. An empty
// body would produce a message indistinguishable from a ZLB on the
// wire while still consuming a sequence number (nextSendSeq++), which
// desynchronizes the Ns space. ZLB ACKs must go through BuildZLB.
func TestEnqueueBodyEmpty(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)
	cases := [][]byte{nil, {}, make([]byte, AVPHeaderLen+1)}
	for _, body := range cases {
		_, err := e.Enqueue(0, body, now)
		if !errors.Is(err, ErrBodyEmpty) {
			t.Errorf("Enqueue(len=%d): err = %v, want ErrBodyEmpty", len(body), err)
		}
	}
	// Minimum legal body (Message Type AVP = 8 octets) succeeds.
	if _, err := e.Enqueue(0, messageTypeAVP(1), now); err != nil {
		t.Errorf("minimum body (Message Type AVP): err = %v, want nil", err)
	}
}

// VALIDATES: RecvWindow and InitialPeerRWS are clamped to RecvWindowMax
// in NewReliableEngine. PREVENTS: advertising or assuming a window
// larger than half the Ns space, which breaks seqBefore classification.
func TestNewReliableEngineClampsRecvWindow(t *testing.T) {
	e := NewReliableEngine(ReliableConfig{
		RecvWindow:     65535,
		InitialPeerRWS: 65535,
	})
	if e.cfg.RecvWindow != RecvWindowMax {
		t.Errorf("cfg.RecvWindow = %d, want %d", e.cfg.RecvWindow, RecvWindowMax)
	}
	if e.win.peerRWS != RecvWindowMax {
		t.Errorf("win.peerRWS = %d, want %d", e.win.peerRWS, RecvWindowMax)
	}
}

// VALIDATES: UpdatePeerRWS at the engine level drains the send queue
// when the window grows. PREVENTS: queued messages staying queued
// forever after the peer advertises a larger window mid-tunnel.
//
// Setup: the gating bottleneck must be peerRWS, not CWND, so we
// pre-set CWND to 4 and peerRWS to 1. available() = min(cwnd=4,
// peerRWS=1) = 1. First Enqueue consumes it; second queues. When
// UpdatePeerRWS lifts peerRWS to 4, available becomes 3 and the queued
// message promotes.
func TestUpdatePeerRWSDrainsSendQueue(t *testing.T) {
	e := NewReliableEngine(ReliableConfig{
		PeerTunnelID:   200,
		RTimeout:       time.Second,
		RTimeoutCap:    16 * time.Second,
		MaxRetransmit:  3,
		RecvWindow:     4,
		InitialPeerRWS: 1,
	})
	// Simulate that slow start already grew CWND to 4 via acks of
	// earlier messages; the present bottleneck is the peer's window.
	e.win.cwnd = 4
	e.win.ssthresh = 4

	now := time.Unix(0, 0)

	// First Enqueue hits the wire (available = min(4, 1) = 1).
	if sent, err := e.Enqueue(0, messageTypeAVP(1), now); err != nil || sent == nil {
		t.Fatalf("first Enqueue: sent=%v err=%v", sent != nil, err)
	}
	// Second Enqueue queues (peerRWS-bottlenecked).
	if sent, err := e.Enqueue(0, messageTypeAVP(2), now); err != nil || sent != nil {
		t.Fatalf("second Enqueue: expected queued, got sent=%v err=%v", sent != nil, err)
	}
	if len(e.sendQueue) != 1 {
		t.Fatalf("sendQueue length = %d, want 1", len(e.sendQueue))
	}

	// Peer advertises a larger window (SCCRP carried RWS=4).
	// UpdatePeerRWS must drain the queued message.
	newSends := e.UpdatePeerRWS(4, now)
	if len(newSends) != 1 {
		t.Errorf("NewSends after RWS growth = %d, want 1", len(newSends))
	}
	if len(e.sendQueue) != 0 {
		t.Errorf("sendQueue after drain = %d, want 0", len(e.sendQueue))
	}
}

// VALIDATES: AC-26 Enqueue after Close returns ErrEngineClosed.
func TestEnqueueAfterClose(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)
	e.Close(now)

	_, err := e.Enqueue(0, messageTypeAVP(1), now)
	if !errors.Is(err, ErrEngineClosed) {
		t.Errorf("err = %v, want ErrEngineClosed", err)
	}
}

// VALIDATES: AC-27 wraparound. Ns values 65534, 65535, 0 all processed
// in order, Nr 0 acks through wraparound.
func TestWraparoundAck(t *testing.T) {
	e := newTestEngine()
	// Manually advance state to just before wrap: mid-tunnel after ~65k
	// exchanges. peerNr=65534 reflects the peer having acked our prior
	// Ns=65533.
	e.nextSendSeq = 65534
	e.nextRecvSeq = 65534
	e.peerNr = 65534

	now := time.Unix(0, 0)

	// Send Ns=65534.
	mustEnqueue(t, e, 0, messageTypeAVP(1), now)

	// Receive Ns=65534, Nr=65535 (acks our send).
	hdr := MessageHeader{IsControl: true, HasSequence: true, Version: 2, TunnelID: 100, Ns: 65534, Nr: 65535}
	r := e.OnReceive(hdr, messageTypeAVP(2), now)
	if r.Class != ClassDelivered {
		t.Errorf("Class = %v, want ClassDelivered at wrap", r.Class)
	}
	if e.Outstanding() != 0 {
		t.Errorf("Outstanding = %d, want 0 after wraparound ack", e.Outstanding())
	}
	if e.nextRecvSeq != 65535 {
		t.Errorf("nextRecvSeq = %d, want 65535", e.nextRecvSeq)
	}

	// Receive Ns=65535, Nr=65535.
	hdr.Ns = 65535
	r = e.OnReceive(hdr, messageTypeAVP(3), now)
	if r.Class != ClassDelivered {
		t.Errorf("Ns=65535: Class = %v, want ClassDelivered", r.Class)
	}
	if e.nextRecvSeq != 0 {
		t.Errorf("nextRecvSeq after Ns=65535 = %d, want 0 (wrapped)", e.nextRecvSeq)
	}

	// Receive Ns=0 across wrap.
	hdr.Ns = 0
	hdr.Nr = 65535
	r = e.OnReceive(hdr, messageTypeAVP(4), now)
	if r.Class != ClassDelivered {
		t.Errorf("Ns=0 post-wrap: Class = %v, want ClassDelivered", r.Class)
	}
	if e.nextRecvSeq != 1 {
		t.Errorf("nextRecvSeq after full wrap = %d, want 1", e.nextRecvSeq)
	}
}

// VALIDATES: Enqueue body bytes are deeply copied into the rtms_queue,
// so caller mutation after Enqueue does not corrupt the stored message.
func TestEnqueueDefensiveCopy(t *testing.T) {
	e := newTestEngine()
	now := time.Unix(0, 0)

	body := messageTypeAVP(1)
	sent, _ := e.Enqueue(0, body, now)
	bodyCopy := bytes.Clone(body)
	body[6] = 0xff // mutate caller buffer

	// rtms_queue entry's bytes should still have the original message
	// type AVP value, because the engine built a fresh buffer.
	if sent[12+6] != bodyCopy[6] {
		t.Errorf("engine bytes aliased caller buffer: got %x, want %x", sent[12+6], bodyCopy[6])
	}
}
