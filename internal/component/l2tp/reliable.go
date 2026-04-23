// Design: docs/architecture/wire/l2tp.md — L2TP reliable delivery engine
// RFC: rfc/short/rfc2661.md — RFC 2661 Section 5.8 (reliable delivery) + Appendix A (congestion)
// Related: reliable_seq.go — seqBefore, constants, retentionDuration
// Related: reliable_window.go — CWND/SSTHRESH state and advancement
// Related: reliable_reorder.go — out-of-order ring buffer

package l2tp

import (
	"encoding/binary"
	"errors"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/bufpool"
)

// Pool sizing for the reliable engine's send/rtms queues. A TACACS+-style
// max-size slab (1023 bytes = 10-bit Length field ceiling) is borrowed on
// Enqueue and Put back on ACK in processNr. 16 slabs cover two generous
// windows of in-flight messages per tunnel; overflow falls through to
// the pool's `New` path, which is a plain `make` -- acceptable because
// L2TP control traffic is slow-paced (tunnel setup + periodic HELLO).
const (
	reliableSlabSize = 0x03FF // 1023 octets; matches 10-bit Length field cap
	reliableSlabSeed = 16
)

// Classification tags the outcome of processing one inbound control
// message. The caller (phase 3's tunnel state machine) uses it to decide
// whether to deliver the payload to upper layers, skip it, or treat it
// as a data-plane message.
type Classification int

const (
	// ClassDelivered means the message arrived in order; its payload is
	// delivered to the upper layer. The engine advances nextRecvSeq and
	// may flush any subsequent buffered reorder entries in the same call.
	ClassDelivered Classification = iota

	// ClassDuplicate means the message was already seen and processed.
	// The engine does NOT deliver it again but MUST acknowledge it
	// (RFC 2661 S5.8 line 2550) -- needsZLB is set.
	ClassDuplicate

	// ClassReorderQueued means the message is out of order within the
	// advertised receive window; the engine buffered it for later
	// in-order delivery. No ACK is emitted yet because Nr has not
	// advanced (RFC 2661 S5.8 B.2 example).
	ClassReorderQueued

	// ClassDiscarded means the message is out of order beyond the
	// advertised window; the peer violated our RWS or reordering is
	// worse than one window. Dropped; peer will retransmit.
	ClassDiscarded

	// ClassZLB means the message is a Zero-Length Body ACK. Its Nr is
	// processed (clears rtms_queue) but the message itself is not
	// delivered anywhere. No counter-ACK is needed.
	ClassZLB

	// ClassDataMessage means the header carries T=0 (data plane).
	// Phase 3 should pass it to the kernel data path, not to the
	// reliable engine. The engine refuses to update state (RFC 2661
	// trap 24.4: Nr in data messages is reserved).
	ClassDataMessage
)

// Errors returned by the engine.
var (
	// ErrEngineClosed is returned by Enqueue after Close has been called.
	ErrEngineClosed = errors.New("l2tp: reliable engine is closed")

	// ErrBodyTooLarge is returned by Enqueue when header+body would
	// exceed the 10-bit L2TP Length field (1023 octets).
	ErrBodyTooLarge = errors.New("l2tp: message body exceeds 10-bit Length field")

	// ErrSendQueueFull is returned by Enqueue when the gated send queue
	// has reached MaxSendQueueDepth (256). Caller (phase 3) should
	// apply application-layer flow control before this fires; the
	// engine's cap prevents an unbounded allocation DoS if the peer
	// stops acknowledging entirely.
	ErrSendQueueFull = errors.New("l2tp: send queue full")

	// ErrBodyEmpty is returned by Enqueue when the supplied body would
	// produce a message indistinguishable from a ZLB on the wire (RFC
	// 2661 S5.8: a control message with no AVPs). Callers wanting to
	// emit a Zero-Length Body ACK MUST use BuildZLB instead; it does
	// not consume a sequence number. The minimum legal body is a
	// Message Type AVP (8 octets: 6-byte header + 2-byte value), per
	// RFC 2661 S4.1 which requires every control message to carry a
	// Message Type AVP as its first attribute.
	ErrBodyEmpty = errors.New("l2tp: body missing Message Type AVP (use BuildZLB for ZLB)")
)

// MaxSendQueueDepth caps the number of messages waiting on the gated
// send queue. L2TP control traffic is slow-paced (tunnel + session
// setup, periodic HELLO); 256 is generous for every realistic scenario
// and prevents an unbounded allocation if the peer stops acknowledging
// while the application keeps Enqueueing.
const MaxSendQueueDepth = 256

// ReliableConfig carries the engine's per-tunnel parameters.
// LocalTunnelID and PeerTunnelID are required; other fields default
// when zero.
type ReliableConfig struct {
	// LocalTunnelID is the tunnel ID we assigned to the peer for this
	// tunnel. It appears in the TunnelID header field of inbound
	// messages (the peer addresses us with it) but is NOT written into
	// outbound headers -- the outbound field carries the peer's ID.
	LocalTunnelID uint16

	// PeerTunnelID is the tunnel ID the peer assigned for this tunnel,
	// learned from the Assigned Tunnel ID AVP in SCCRQ (LAC->LNS) or
	// SCCRP (LNS->LAC). Every outbound header carries this value in its
	// TunnelID field. Zero until the peer has assigned one.
	PeerTunnelID uint16

	// RTimeout is the initial retransmission timeout. Zero means
	// DefaultRTimeout (1s).
	RTimeout time.Duration

	// RTimeoutCap caps exponential backoff. Zero means DefaultRTimeoutCap
	// (16s). RFC 2661 S5.8 requires this to be no less than 8 seconds.
	RTimeoutCap time.Duration

	// MaxRetransmit is the maximum retransmit count before the engine
	// signals teardown. Zero means DefaultMaxRetransmit (5).
	MaxRetransmit int

	// RecvWindow is the value we advertise to the peer. It also sizes
	// our reorder ring buffer. Zero means DefaultRecvWindow (16).
	RecvWindow uint16

	// InitialPeerRWS is the initial assumption about the peer's window
	// before the peer's Receive Window Size AVP arrives. Zero means
	// DefaultPeerRcvWindow (4).
	InitialPeerRWS uint16
}

// RecvEntry is one message delivered to the upper layer by OnReceive.
// The payload is the AVP bytes after the 12-byte header; the engine has
// already processed Ns/Nr. The MessageType is extracted from the first
// AVP (Message Type AVP MUST be first per RFC 2661 S4.1) as a
// convenience for phase 3.
//
// Payload ownership is asymmetric and callers MUST handle it correctly:
//
//   - For in-order delivery (the first entry in ReceiveResult.Delivered
//     when Class == ClassDelivered), Payload aliases the caller's
//     OnReceive `payload` argument. Caller MUST process it before the
//     caller's read buffer is reused.
//   - For gap-fill delivery (entries after the first when a reorder
//     queue flush happens), Payload references the engine's internal
//     reorder-queue copy. That copy remains valid until the next engine
//     method call that mutates the queue.
//
// The safe rule for phase 3: process every delivered Payload before
// invoking another engine method, or copy with bytes.Clone if
// retention is required.
type RecvEntry struct {
	Ns          uint16
	SessionID   uint16
	MessageType uint16
	Payload     []byte
}

// ReceiveResult bundles the outputs of OnReceive. A single inbound
// message can deliver multiple messages (gap fill) and release
// previously queued sends (window open).
type ReceiveResult struct {
	Class     Classification
	Delivered []RecvEntry
	NewSends  [][]byte
}

// TickResult bundles the outputs of Tick. Retransmits are fully
// encoded message bytes (with Nr rewritten to the current expected
// value per RFC 2661 S5.8 line 2589-2590).
type TickResult struct {
	Retransmits      [][]byte
	TeardownRequired bool
}

// rtmsEntry is one unacknowledged outbound message. The engine owns
// bytes and mutates byte positions 10-11 (Nr) on retransmit. `bytes`
// is a sub-slice of a `bufpool.Pool` slab (cap == reliableSlabSize) so
// processNr can return it via `e.bufs.Put` on ACK.
type rtmsEntry struct {
	ns    uint16
	bytes []byte
}

// pendingSend is a message waiting for the window to open. The slab is
// a pool buffer with the AVP body already copied into
// slab[ControlHeaderLen:ControlHeaderLen+bodyLen]; send() stamps the
// header into slab[:ControlHeaderLen] when the window reopens.
type pendingSend struct {
	sessionID uint16
	slab      []byte // full pool slab; cap == reliableSlabSize
	bodyLen   int
}

// ReliableEngine implements RFC 2661 Section 5.8 reliable delivery for
// one L2TP control connection. NOT safe for concurrent use; the caller
// (phase 3's reactor) must serialize all method invocations.
type ReliableEngine struct {
	cfg ReliableConfig

	// Sequencing state.
	nextSendSeq uint16 // Ns we will assign to the next outbound non-ZLB
	nextRecvSeq uint16 // Ns we expect from the peer next
	peerNr      uint16 // highest Nr the peer has acknowledged

	// Outbound queues.
	sendQueue []pendingSend // gated by window
	rtmsQueue []rtmsEntry   // in flight, awaiting ACK

	// Inbound buffering.
	reorder *reorderQueue

	// Congestion / flow control.
	win *window

	// Retransmission.
	rtimeout      time.Duration // current timeout (doubles on expiry)
	attempts      int           // consecutive retransmit count
	deadline      time.Time     // earliest retransmit deadline (zero if idle)
	maxRetransmit int
	rtimeoutCap   time.Duration

	// Acknowledgment.
	needsZLB bool

	// Pre-allocated 1023-byte slabs for control-message wire buffers.
	// Every pendingSend and rtmsEntry borrows from here on Enqueue and
	// returns on processNr ACK -- eliminates the per-message `make` on
	// the send path per the "No make where pools exist" design rule.
	bufs *bufpool.Pool

	// Teardown.
	closed    bool
	closedAt  time.Time
	retention time.Duration
}

// ReliableStats is a point-in-time snapshot of the reliable engine state.
type ReliableStats struct {
	NextSendSeq     uint16
	NextRecvSeq     uint16
	PeerNr          uint16
	Outstanding     int
	RetransmitCount int
	CWND            uint16
	SSThresh        uint16
	PeerRWS         uint16
}

// Stats returns a snapshot of the engine's sequencing and congestion state.
func (e *ReliableEngine) Stats() ReliableStats {
	s := ReliableStats{
		NextSendSeq:     e.nextSendSeq,
		NextRecvSeq:     e.nextRecvSeq,
		PeerNr:          e.peerNr,
		Outstanding:     len(e.rtmsQueue),
		RetransmitCount: e.attempts,
	}
	if e.win != nil {
		s.CWND = e.win.cwnd
		s.SSThresh = e.win.ssthresh
		s.PeerRWS = e.win.peerRWS
	}
	return s
}

// NewReliableEngine constructs an engine with the supplied config. Zero
// fields are populated with defaults. RecvWindow and InitialPeerRWS are
// clamped to RecvWindowMax (32768) because advertising or assuming a
// larger peer window breaks the 32767-half sequence-number
// classification (RFC 2661 S5.8 "preceding 32767 values, inclusive").
func NewReliableEngine(cfg ReliableConfig) *ReliableEngine {
	if cfg.RTimeout == 0 {
		cfg.RTimeout = DefaultRTimeout
	}
	if cfg.RTimeoutCap == 0 {
		cfg.RTimeoutCap = DefaultRTimeoutCap
	}
	if cfg.MaxRetransmit == 0 {
		cfg.MaxRetransmit = DefaultMaxRetransmit
	}
	if cfg.RecvWindow == 0 {
		cfg.RecvWindow = DefaultRecvWindow
	}
	if cfg.RecvWindow > RecvWindowMax {
		cfg.RecvWindow = RecvWindowMax
	}
	if cfg.InitialPeerRWS == 0 {
		cfg.InitialPeerRWS = DefaultPeerRcvWindow
	}
	// newWindow clamps InitialPeerRWS via clampPeerRWS; no duplicate
	// clamp needed here.
	return &ReliableEngine{
		cfg:           cfg,
		reorder:       newReorderQueue(cfg.RecvWindow),
		win:           newWindow(cfg.InitialPeerRWS),
		rtimeout:      cfg.RTimeout,
		rtimeoutCap:   cfg.RTimeoutCap,
		maxRetransmit: cfg.MaxRetransmit,
		retention:     retentionDuration(cfg.RTimeout, cfg.RTimeoutCap, cfg.MaxRetransmit),
		bufs:          bufpool.New(reliableSlabSeed, reliableSlabSize, "l2tp-reliable"),
	}
}

// Enqueue requests transmission of a control message with the given
// session ID and AVP body. The engine constructs the 12-byte header,
// assigns Ns (incrementing nextSendSeq), and fills Nr from current
// state. If the window allows immediate send, the fully encoded bytes
// are returned. Otherwise the message is queued and sent via NewSends
// in a later OnReceive or UpdatePeerRWS return.
//
// The returned byte slice is owned by the engine. It remains valid
// until the next engine method call that mutates rtms_queue
// (subsequent Enqueue, Tick, OnReceive with Nr advance). Specifically,
// Tick will rewrite the Nr field at offset 10-11 in place on
// retransmission. Callers MUST write the bytes to the transport
// before invoking another engine method, or copy if retention is
// required.
//
// Returns ErrEngineClosed after Close has been called.
// Returns ErrBodyEmpty if len(body) < AVPHeaderLen+2; an empty body
// would be indistinguishable from a ZLB on the wire but would still
// consume a sequence number. For ZLB ACKs, use BuildZLB.
// Returns ErrBodyTooLarge if len(body) > 1023 - ControlHeaderLen.
// Returns ErrSendQueueFull if the gated send queue is full
// (MaxSendQueueDepth).
func (e *ReliableEngine) Enqueue(sessionID uint16, body []byte, now time.Time) ([]byte, error) {
	if e.closed {
		return nil, ErrEngineClosed
	}
	if len(body) < AVPHeaderLen+2 {
		return nil, ErrBodyEmpty
	}
	if len(body)+ControlHeaderLen > 0x03FF {
		return nil, ErrBodyTooLarge
	}

	// Borrow one slab for this message's lifetime. Body is copied into
	// slab[ControlHeaderLen:ControlHeaderLen+bodyLen] so both the queued
	// path and the direct-send path can hand the same slab to send(),
	// which only needs to stamp the header at slab[:ControlHeaderLen].
	slab := e.bufs.Get()
	bodyLen := len(body)
	copy(slab[ControlHeaderLen:ControlHeaderLen+bodyLen], body)

	// If outstanding count is at the window limit, queue.
	outstanding := uint16(len(e.rtmsQueue)) //nolint:gosec // rtmsQueue is bounded by window <= 32768
	if e.win.available(outstanding) == 0 {
		if len(e.sendQueue) >= MaxSendQueueDepth {
			e.bufs.Put(slab)
			return nil, ErrSendQueueFull
		}
		e.sendQueue = append(e.sendQueue, pendingSend{sessionID: sessionID, slab: slab, bodyLen: bodyLen})
		return nil, nil
	}

	return e.send(sessionID, slab, bodyLen, now), nil
}

// send stamps the header into slab[:ControlHeaderLen] (body is already
// at slab[ControlHeaderLen:ControlHeaderLen+bodyLen]), records the
// resulting wire bytes in the rtms_queue, schedules the retransmit
// deadline if needed, and returns a sub-slice of the slab for
// transmission. Caller owns the transmission; the returned slice remains
// valid only until the next engine call. The slab's capacity is
// reliableSlabSize so processNr can return it to `e.bufs` on ACK.
func (e *ReliableEngine) send(sessionID uint16, slab []byte, bodyLen int, now time.Time) []byte {
	length := uint16(ControlHeaderLen + bodyLen) //nolint:gosec // bounded by Enqueue's 1023-octet check
	// RFC 2661 Section 5.8: sender assigns Ns = next_send_seq, Nr =
	// next expected from peer. WriteControlHeader stamps flags 0xC802.
	WriteControlHeader(slab[:ControlHeaderLen], 0, length, e.cfg.PeerTunnelID, sessionID, e.nextSendSeq, e.nextRecvSeq)
	bytes := slab[:length]

	e.rtmsQueue = append(e.rtmsQueue, rtmsEntry{ns: e.nextSendSeq, bytes: bytes})
	e.nextSendSeq++

	// Needs-ZLB is cleared whenever we piggyback an outbound message:
	// the message itself carries our latest Nr.
	e.needsZLB = false

	// Start or extend the retransmit deadline if this is the first
	// in-flight message.
	if e.deadline.IsZero() {
		e.deadline = now.Add(e.rtimeout)
		e.attempts = 0
	}
	return bytes
}

// drainSendQueue promotes queued messages to the wire as the window
// allows. Returns the newly-sent bytes. Called after events that grow
// the window: OnReceive advancing peerNr, UpdatePeerRWS enlarging the
// cap.
//
// After each dequeue, the freed slot in the backing array is cleared
// before the slice header advances, so the old slab reference is not
// retained via the backing-array root even though the slice view no
// longer covers that index.
func (e *ReliableEngine) drainSendQueue(now time.Time) [][]byte {
	var sent [][]byte
	for len(e.sendQueue) > 0 {
		outstanding := uint16(len(e.rtmsQueue)) //nolint:gosec // rtmsQueue bounded by window
		if e.win.available(outstanding) == 0 {
			break
		}
		head := e.sendQueue[0]
		e.sendQueue[0] = pendingSend{}
		e.sendQueue = e.sendQueue[1:]
		sent = append(sent, e.send(head.sessionID, head.slab, head.bodyLen, now))
	}
	return sent
}

// OnReceive classifies an inbound control message and updates engine
// state. hdr must already be parsed (phase 1's ParseMessageHeader);
// payload is the raw bytes AFTER the 12-byte header.
//
// Side effects depend on classification (see Classification docs). The
// returned ReceiveResult.NewSends is populated when this receive
// advanced peerNr enough to open the send window.
func (e *ReliableEngine) OnReceive(hdr MessageHeader, payload []byte, now time.Time) ReceiveResult {
	// RFC 2661 trap 24.4: Nr in data messages is reserved. The engine
	// cannot trust it. Phase 3 should not even route data messages
	// here, but defend in depth.
	if !hdr.IsControl {
		return ReceiveResult{Class: ClassDataMessage}
	}

	// Process Nr: clear acknowledged entries from the rtms queue. This
	// applies to EVERY control message, including duplicates and ZLBs,
	// because Nr is advisory across the whole control channel.
	acked := e.processNr(hdr.Nr)

	// If any entries were cleared, the window may have grown. Drain
	// queued sends.
	var newSends [][]byte
	if acked > 0 {
		if len(e.rtmsQueue) == 0 {
			// All outstanding acked -- stop the retransmit timer.
			e.deadline = time.Time{}
			e.attempts = 0
			e.rtimeout = e.cfg.RTimeout
		} else {
			// Partial ack -- reset timer to initial for remaining entries.
			e.deadline = now.Add(e.cfg.RTimeout)
			e.attempts = 0
			e.rtimeout = e.cfg.RTimeout
		}
		newSends = e.drainSendQueue(now)
	}

	// Is this a ZLB? A control message with payload length 0 is a ZLB
	// acknowledgment. ZLBs do not consume Ns (RFC 2661 S5.8 line 2556-2557
	// "Ns is not incremented after a ZLB message is sent"), so we do
	// not compare against nextRecvSeq.
	if len(payload) == 0 {
		return ReceiveResult{Class: ClassZLB, NewSends: newSends}
	}

	// Non-ZLB control message: classify by Ns vs nextRecvSeq.
	if hdr.Ns == e.nextRecvSeq {
		// In order.
		entry := e.makeRecvEntry(hdr.Ns, hdr.SessionID, payload)
		e.nextRecvSeq++
		e.needsZLB = true
		delivered := []RecvEntry{entry}
		// Drain any consecutive reorder-queued messages.
		for _, r := range e.reorder.popInOrder(e.nextRecvSeq) {
			delivered = append(delivered, e.makeRecvEntry(r.ns, 0, r.payload))
			e.nextRecvSeq++
		}
		return ReceiveResult{Class: ClassDelivered, Delivered: delivered, NewSends: newSends}
	}
	if seqBefore(hdr.Ns, e.nextRecvSeq) {
		// Duplicate. MUST ACK per RFC 2661 S5.8 line 2550.
		e.needsZLB = true
		return ReceiveResult{Class: ClassDuplicate, NewSends: newSends}
	}
	// Out of order: hdr.Ns > nextRecvSeq in sequence terms.
	// Try to queue; if beyond window, discard.
	if e.reorder.store(hdr.Ns, e.nextRecvSeq, payload) {
		return ReceiveResult{Class: ClassReorderQueued, NewSends: newSends}
	}
	return ReceiveResult{Class: ClassDiscarded, NewSends: newSends}
}

// processNr removes rtms_queue entries acknowledged by the peer's Nr,
// and drives CWND growth. Returns the number of entries removed.
//
// Each freed rtmsEntry's bytes reference is returned to the engine's
// slab pool (cap matches `reliableSlabSize`, so `bufs.Put` accepts it)
// and then the backing-array slot is cleared before the slice header
// advances so the pool-owned memory is not also retained via the
// backing-array root.
func (e *ReliableEngine) processNr(nr uint16) int {
	// Nr is "next expected from peer", i.e., every Ns < nr is acked.
	// Only advance peerNr if the new nr is newer.
	if !seqBefore(e.peerNr, nr) {
		return 0
	}
	e.peerNr = nr

	n := 0
	for len(e.rtmsQueue) > 0 && seqBefore(e.rtmsQueue[0].ns, nr) {
		e.bufs.Put(e.rtmsQueue[0].bytes)
		e.rtmsQueue[0] = rtmsEntry{}
		e.rtmsQueue = e.rtmsQueue[1:]
		e.win.onAck()
		n++
	}
	return n
}

// makeRecvEntry extracts the Message Type from the first AVP (RFC 2661
// S4.1: "Message Type AVP MUST be first") and returns a RecvEntry. The
// payload reference is NOT copied -- the caller (phase 3) must process
// it before calling another engine method that might reuse the buffer.
func (e *ReliableEngine) makeRecvEntry(ns, sessionID uint16, payload []byte) RecvEntry {
	entry := RecvEntry{Ns: ns, SessionID: sessionID, Payload: payload}
	// Message Type AVP: first AVP, value is a uint16 after the 6-byte
	// AVP header (flags+length, vendor-id=0, attr-type=0).
	if len(payload) >= AVPHeaderLen+2 {
		entry.MessageType = binary.BigEndian.Uint16(payload[AVPHeaderLen:])
	}
	return entry
}

// UpdatePeerRWS notifies the engine that the peer's Receive Window Size
// AVP has been parsed from an inbound message. Subsequent send gating
// uses the new cap. Returns any newly-sendable queued messages.
func (e *ReliableEngine) UpdatePeerRWS(size uint16, now time.Time) [][]byte {
	e.win.updatePeerRWS(size)
	return e.drainSendQueue(now)
}

// Tick processes retransmit deadlines. If now is past the current
// deadline and the rtms_queue is non-empty, every outstanding message
// is retransmitted (Nr rewritten to current nextRecvSeq, deadline
// doubled up to rtimeoutCap). If the attempt counter reaches
// maxRetransmit, TeardownRequired is returned.
func (e *ReliableEngine) Tick(now time.Time) TickResult {
	if e.deadline.IsZero() || now.Before(e.deadline) || len(e.rtmsQueue) == 0 {
		return TickResult{}
	}

	e.attempts++
	if e.attempts > e.maxRetransmit {
		return TickResult{TeardownRequired: true}
	}

	// Rewrite Nr in each outstanding message's bytes (offset 10-11 of
	// the 12-byte control header). RFC 2661 S5.8 line 2589-2590: "Nr
	// value MUST be updated with the sequence number of the next
	// expected message".
	retrans := make([][]byte, 0, len(e.rtmsQueue))
	for i := range e.rtmsQueue {
		binary.BigEndian.PutUint16(e.rtmsQueue[i].bytes[10:12], e.nextRecvSeq)
		retrans = append(retrans, e.rtmsQueue[i].bytes)
	}

	// Congestion signal: halve SSTHRESH, reset CWND per Appendix A.
	e.win.onRetransmit()

	// Double the timeout for the next expiry, capped.
	e.rtimeout *= 2
	if e.rtimeout > e.rtimeoutCap {
		e.rtimeout = e.rtimeoutCap
	}
	e.deadline = now.Add(e.rtimeout)

	return TickResult{Retransmits: retrans}
}

// NextDeadline returns the earliest retransmit deadline, or a zero Time
// if the rtms_queue is empty. Phase 3 aggregates this across all
// tunnels in its global min-heap.
func (e *ReliableEngine) NextDeadline() time.Time {
	return e.deadline
}

// NeedsZLB reports whether there is a pending acknowledgment that has
// not yet been piggybacked on an outbound message. Phase 3 should call
// BuildZLB when no outbound control message is due.
func (e *ReliableEngine) NeedsZLB() bool {
	return e.needsZLB
}

// BuildZLB writes a 12-byte Zero-Length Body ACK into buf at off. The
// ZLB carries the current nextSendSeq (NOT incremented -- RFC 2661
// S5.8 line 2556-2557) and nextRecvSeq. Returns ControlHeaderLen.
// Clears the needsZLB flag.
//
// Callers MUST ensure buf has at least ControlHeaderLen bytes available
// at off.
func (e *ReliableEngine) BuildZLB(buf []byte, off int) int {
	WriteControlHeader(buf, off, ControlHeaderLen, e.cfg.PeerTunnelID, 0, e.nextSendSeq, e.nextRecvSeq)
	e.needsZLB = false
	return ControlHeaderLen
}

// Close transitions the engine to the closed state. Subsequent Enqueue
// calls return ErrEngineClosed. The engine continues to handle
// OnReceive for duplicate StopCCN retransmits and BuildZLB for their
// acknowledgments until Expired(now) returns true.
//
// RFC 2661 S5.8 line 2602-2605: "the state and reliable delivery
// mechanisms MUST be maintained and operated for the full retransmission
// interval after the final message exchange has occurred".
func (e *ReliableEngine) Close(now time.Time) {
	if e.closed {
		return
	}
	e.closed = true
	e.closedAt = now
}

// Expired reports whether the post-teardown retention period has
// elapsed. Before Close returns false; after Close, returns true once
// now is at least retentionDuration past closedAt.
func (e *ReliableEngine) Expired(now time.Time) bool {
	if !e.closed {
		return false
	}
	return now.Sub(e.closedAt) >= e.retention
}

// Outstanding reports the number of unacknowledged messages in flight.
// Exposed for observability (phase 3 logging / metrics).
func (e *ReliableEngine) Outstanding() int { return len(e.rtmsQueue) }

// CWND reports the current congestion window. Exposed for observability.
func (e *ReliableEngine) CWND() uint16 { return e.win.cwnd }

// SSTHRESH reports the current slow-start threshold. Exposed for
// observability.
func (e *ReliableEngine) SSTHRESH() uint16 { return e.win.ssthresh }
