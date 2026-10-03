package l2tp

// Design: docs/architecture/wire/l2tp.md -- RFC 2661 obligation proofs
//
// Tagged proofs of RFC 2661 obligations over the header codec, the AVP
// writers and parsers, the tunnel and session FSMs, the reliable engine and
// the reactor's retention reap. Each tag names exactly what its body asserts.
//
// VALIDATES: RFC 2661 sender and receiver obligations through their producers.
// PREVENTS: a tag claiming a clause no assertion checks.
// Related: rfc2661_message_test.go, rfc2661_session_fsm_test.go, rfc2661_reliable_test.go.

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"net/netip"
	"testing"
	"time"
)

// headerReservedMask covers the bits of the leading header word that RFC 2661
// Section 3.1 leaves reserved: every bit that is not T, L, S, O, P or Ver.
const headerReservedMask = 0xFFFF &^ (flagT | flagL | flagS | flagO | flagP | verMask)

// RFC requirement: RFC2661-x-1 positive -- every control header
// WriteControlHeader emits, and every data header WriteDataHeader emits under
// each of the 16 flag combinations, carries 0 in every reserved bit of the
// leading header word.
//
// TestRFC2661ReservedHeaderBitsSentZero reads the leading word of every header
// shape the two writers produce and masks out the defined fields.
func TestRFC2661ReservedHeaderBitsSentZero(t *testing.T) {
	buf := make([]byte, 32)
	WriteControlHeader(buf, 0, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF)
	if word := binary.BigEndian.Uint16(buf); word&headerReservedMask != 0 {
		t.Fatalf("control header word 0x%04x sets reserved bits 0x%04x", word, word&headerReservedMask)
	}
	for combo := range 16 {
		clear(buf)
		h := MessageHeader{
			HasLength:   combo&1 != 0,
			HasSequence: combo&2 != 0,
			HasOffset:   combo&4 != 0,
			Priority:    combo&8 != 0,
			TunnelID:    0xFFFF,
			SessionID:   0xFFFF,
			Ns:          0xFFFF,
			Nr:          0xFFFF,
		}
		WriteDataHeader(buf, 0, h)
		if word := binary.BigEndian.Uint16(buf); word&headerReservedMask != 0 {
			t.Fatalf("data header combo %d word 0x%04x sets reserved bits", combo, word)
		}
	}
}

// RFC requirement: RFC2661-x-1 negative -- a control header and a data header
// arriving with every reserved bit set to 1 are accepted by ParseMessageHeader
// with the same parsed fields as the header with the reserved bits clear.
//
// TestRFC2661ReservedHeaderBitsIgnoredOnReceive parses each header twice, once
// clean and once with the reserved bits set, and compares the results.
func TestRFC2661ReservedHeaderBitsIgnoredOnReceive(t *testing.T) {
	control := []byte{0xC8, 0x02, 0x00, 0x0C, 0, 7, 0, 9, 0, 3, 0, 4}
	data := []byte{0x40, 0x02, 0x00, 0x08, 0, 7, 0, 9}
	for _, clean := range [][]byte{control, data} {
		want, err := ParseMessageHeader(clean)
		if err != nil {
			t.Fatalf("clean header % x refused: %v", clean, err)
		}
		dirty := append([]byte(nil), clean...)
		word := binary.BigEndian.Uint16(dirty) | headerReservedMask
		binary.BigEndian.PutUint16(dirty, word)
		got, err := ParseMessageHeader(dirty)
		if err != nil {
			t.Fatalf("header with reserved bits set (% x) refused: %v", dirty, err)
		}
		if got != want {
			t.Fatalf("reserved bits changed the parse: got %+v, want %+v", got, want)
		}
	}
}

// icrqWithReservedBitAVP builds an ICRQ carrying Assigned Session ID 9 and,
// after it, a second Assigned Session ID AVP of value 77 whose AVP header sets
// a reserved bit, with the given M bit.
func icrqWithReservedBitAVP(mandatory bool) []byte {
	buf := make([]byte, 128)
	off := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgICRQ))
	off += WriteAVPUint16(buf, off, true, AVPAssignedSessionID, 9)
	off += WriteAVPUint32(buf, off, true, AVPCallSerialNumber, 1)
	reserved := off
	off += WriteAVPUint16(buf, off, mandatory, AVPAssignedSessionID, 77)
	// The first AVP header octet is M, H, then four reserved bits.
	buf[reserved] |= 0x20
	return buf[:off]
}

// RFC requirement: RFC2661-4.1-1 negative -- an AVP received with a reserved
// bit set is treated as an unrecognized AVP: with M=0 its value is ignored (the
// session keeps the Assigned Session ID of the clean AVP), and with M=1 the
// session is terminated: handleICRQ answers with exactly one CDN, creates no
// session, and leaves the tunnel established.
//
// TestRFC2661ReservedAVPBitTreatedAsUnrecognized drives handleICRQ with an ICRQ
// whose trailing Assigned Session ID AVP sets a reserved header bit.
func TestRFC2661ReservedAVPBitTreatedAsUnrecognized(t *testing.T) {
	now := time.Now()
	logger := slog.Default()

	tun := newEstablishedTunnel(t, 0)
	tun.handleICRQ(icrqWithReservedBitAVP(false), now, logger)
	if tun.sessionCount() != 1 {
		t.Fatalf("M=0 reserved-bit AVP: %d sessions, want 1", tun.sessionCount())
	}
	for _, s := range tun.sessions {
		if s.remoteSID != 9 {
			t.Fatalf("M=0 reserved-bit AVP was honored: remote SID %d, want 9", s.remoteSID)
		}
	}

	tun = newEstablishedTunnel(t, 0)
	out := tun.handleICRQ(icrqWithReservedBitAVP(true), now, logger)
	if len(out) != 1 {
		t.Fatalf("M=1 reserved-bit AVP: %d datagrams, want one CDN", len(out))
	}
	if mt := sentMessageType(t, out[0].bytes); mt != MsgCDN {
		t.Fatalf("M=1 reserved-bit AVP: answered with message type %d, want CDN", mt)
	}
	if tun.sessionCount() != 0 {
		t.Fatalf("M=1 reserved-bit AVP: %d sessions, want 0", tun.sessionCount())
	}
	if tun.state != L2TPTunnelEstablished {
		t.Fatalf("M=1 reserved-bit AVP in a session message closed the tunnel: %s", tun.state)
	}
}

// sentMessageType parses a datagram a tunnel emitted and returns the value of
// the Message Type AVP that opens its body.
func sentMessageType(t *testing.T, pkt []byte) MessageType {
	t.Helper()
	hdr, err := ParseMessageHeader(pkt)
	if err != nil {
		t.Fatalf("emitted datagram: %v", err)
	}
	it := NewAVPIterator(pkt[hdr.PayloadOff:hdr.Length])
	_, attrType, _, value, ok := it.Next()
	if !ok {
		t.Fatalf("emitted datagram carries no AVP: %v", it.Err())
	}
	if attrType != AVPMessageType {
		t.Fatalf("emitted datagram opens with AVP %d, want Message Type", attrType)
	}
	mt, err := readAVPUint16(value)
	if err != nil {
		t.Fatalf("emitted Message Type: %v", err)
	}
	return MessageType(mt)
}

// helloBody fires the HELLO timer on an established tunnel and returns the
// AVP body of the one datagram it emits.
func helloBody(t *testing.T) []byte {
	t.Helper()
	out := newEstablishedTunnel(t, 1).handleHelloTimer(time.Now())
	if len(out) != 1 {
		t.Fatalf("HELLO timer produced %d datagrams, want 1", len(out))
	}
	hdr, err := ParseMessageHeader(out[0].bytes)
	if err != nil {
		t.Fatalf("HELLO header: %v", err)
	}
	return out[0].bytes[hdr.PayloadOff:hdr.Length]
}

// RFC requirement: RFC2661-4.1-2 positive -- the first AVP of every control
// message body Ze emits (SCCRQ, SCCRP, SCCCN, StopCCN, ICRQ, ICRP, ICCN, OCRQ,
// OCRP, CDN and the HELLO handleHelloTimer sends) is the Message Type AVP.
//
// TestRFC2661EveryEmittedMessageOpensWithMessageType reads the first AVP of
// each body the writers produce.
func TestRFC2661EveryEmittedMessageOpensWithMessageType(t *testing.T) {
	bodies := append(writtenBodies(), writtenBody{name: "HELLO", body: helloBody(t)})
	for _, wb := range bodies {
		it := NewAVPIterator(wb.body)
		vendorID, attrType, _, _, ok := it.Next()
		if !ok {
			t.Fatalf("%s: body carries no AVP: %v", wb.name, it.Err())
		}
		if vendorID != 0 || attrType != AVPMessageType {
			t.Fatalf("%s: first AVP is vendor %d type %d, want Message Type", wb.name, vendorID, attrType)
		}
	}
}

// RFC requirement: RFC2661-4.4-1 positive -- every AVP of the HELLO
// handleHelloTimer emits whose Section 4.4 definition fixes M at 1, the
// Message Type AVP, carries M=1.
//
// TestRFC2661HelloAVPMandatoryBits covers the one emitted body the
// writtenBodies walk in TestRFC2661WrittenAVPFlags does not reach.
func TestRFC2661HelloAVPMandatoryBits(t *testing.T) {
	it := NewAVPIterator(helloBody(t))
	seen := 0
	for {
		_, attrType, flags, _, ok := it.Next()
		if !ok {
			break
		}
		seen++
		want, known := rfc2661MandatoryBit[attrType]
		if !known {
			t.Fatalf("HELLO: AVP %d has no expected M-bit in the test table", attrType)
		}
		if got := flags&FlagMandatory != 0; got != want {
			t.Fatalf("HELLO: AVP %d M=%v, RFC 2661 Section 4.4 wants %v", attrType, got, want)
		}
	}
	if err := it.Err(); err != nil {
		t.Fatalf("HELLO: iterate: %v", err)
	}
	if seen == 0 {
		t.Fatalf("HELLO: empty body")
	}
}

// requireAVPs fails when body lacks one of the wanted attribute types.
func requireAVPs(t *testing.T, name string, body []byte, want []AVPType) {
	t.Helper()
	got := avpTypesOf(t, body)
	for _, w := range want {
		if !got[w] {
			t.Fatalf("%s: AVP %d absent from the emitted body", name, w)
		}
	}
}

// RFC requirement: RFC2661-6.1-1 positive -- writeSCCRQBody emits Message
// Type, Protocol Version, Host Name, Framing Capabilities and Assigned Tunnel
// ID, with and without the optional Challenge and Tie Breaker.
//
// TestRFC2661SCCRQCarriesSection61AVPs checks the sender side of Section 6.1.
func TestRFC2661SCCRQCarriesSection61AVPs(t *testing.T) {
	defaults := TunnelDefaults{HostName: "ze", FramingCapabilities: 3, RecvWindow: 4}
	want := []AVPType{AVPMessageType, AVPProtocolVersion, AVPHostName, AVPFramingCapabilities, AVPAssignedTunnelID}
	buf := make([]byte, 512)
	n := writeSCCRQBody(buf, 5, defaults, nil, nil)
	requireAVPs(t, "SCCRQ bare", buf[:n], want)
	n = writeSCCRQBody(buf, 5, defaults, []byte{1, 2, 3, 4}, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	requireAVPs(t, "SCCRQ with Challenge and Tie Breaker", buf[:n], want)
}

// RFC requirement: RFC2661-6.2-1 positive -- writeSCCRPBody emits Message
// Type, Protocol Version, Framing Capabilities, Host Name and Assigned Tunnel
// ID, with and without the optional Challenge and Challenge Response.
//
// TestRFC2661SCCRPCarriesSection62AVPs checks the sender side of Section 6.2.
func TestRFC2661SCCRPCarriesSection62AVPs(t *testing.T) {
	defaults := TunnelDefaults{HostName: "ze", FramingCapabilities: 3, RecvWindow: 4}
	want := []AVPType{AVPMessageType, AVPProtocolVersion, AVPFramingCapabilities, AVPHostName, AVPAssignedTunnelID}
	buf := make([]byte, 512)
	n := writeSCCRPBody(buf, 5, defaults, nil, nil)
	requireAVPs(t, "SCCRP bare", buf[:n], want)
	n = writeSCCRPBody(buf, 5, defaults, []byte{1, 2, 3, 4}, make([]byte, 16))
	requireAVPs(t, "SCCRP with Challenge and Response", buf[:n], want)
}

// RFC requirement: RFC2661-6.2-1 negative -- parseSCCRP refuses an SCCRP that
// carries Protocol Version, Framing Capabilities, Host Name and Assigned Tunnel
// ID but no Message Type AVP, and accepts the same body with it.
//
// TestRFC2661SCCRPWithoutMessageTypeRefused covers the one Section 6.2 member
// the per-AVP SCCRP tests leave to an untagged test.
func TestRFC2661SCCRPWithoutMessageTypeRefused(t *testing.T) {
	buf := make([]byte, 256)
	off := WriteAVPBytes(buf, 0, true, 0, AVPProtocolVersion, protocolVersionValue[:])
	off += WriteAVPUint32(buf, off, true, AVPFramingCapabilities, 3)
	off += WriteAVPString(buf, off, true, AVPHostName, "peer")
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, 5)
	if _, err := parseSCCRP(buf[:off]); err == nil {
		t.Fatalf("SCCRP without Message Type accepted")
	}
	if _, err := parseSCCRP(sccrpBodyWithHostName("peer")); err != nil {
		t.Fatalf("SCCRP with Message Type refused: %v", err)
	}
}

// RFC requirement: RFC2661-10-2 positive -- allocateLocalTID never returns 0
// as Ze's Assigned Tunnel ID, including when its counter wraps past 0xFFFF,
// and allocateSessionID never returns 0 as Ze's Assigned Session ID over 2^20
// allocations on an empty tunnel.
//
// TestRFC2661LocalIDsNeverZero drives both allocators. A random 16-bit draw is
// 0 once in 65536, so 2^20 draws reach 0 with near certainty and an allocator
// that stopped skipping it would return it.
func TestRFC2661LocalIDsNeverZero(t *testing.T) {
	r := &l2tpReactor{
		logger:           slog.Default(),
		tunnelsByLocalID: make(map[uint16]*L2TPTunnel),
		tunnelsByPeer:    make(map[peerKey]*L2TPTunnel),
	}
	r.nextLocalTID = 0xFFFF
	tid, err := r.allocateLocalTID()
	if err != nil {
		t.Fatalf("allocateLocalTID: %v", err)
	}
	if tid == 0 {
		t.Fatalf("allocateLocalTID returned 0 after the counter wrapped")
	}

	tun := newEstablishedTunnel(t, 0)
	for i := range 1 << 20 {
		if sid := tun.allocateSessionID(); sid == 0 {
			t.Fatalf("allocateSessionID returned 0 on draw %d with an empty session map", i)
		}
	}
}

// RFC requirement: RFC2661-10-2 negative -- parseSCCRQ refuses an SCCRQ whose
// Assigned Tunnel ID is 0 and accepts the same SCCRQ with Assigned Tunnel ID 1.
//
// TestRFC2661PeerZeroTunnelIDRefused covers the tunnel half of the receive side.
func TestRFC2661PeerZeroTunnelIDRefused(t *testing.T) {
	sccrq := func(tid uint16) []byte {
		return bodyOf(MsgSCCRQ,
			func(buf []byte, off int) int {
				return WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, protocolVersionValue[:])
			},
			strAVP(AVPHostName, "peer"),
			u32AVP(AVPFramingCapabilities, 3),
			u16AVP(AVPAssignedTunnelID, tid),
		)
	}
	if _, err := parseSCCRQ(sccrq(0)); err == nil {
		t.Fatalf("SCCRQ with Assigned Tunnel ID 0 accepted")
	}
	if _, err := parseSCCRQ(sccrq(1)); err != nil {
		t.Fatalf("SCCRQ with Assigned Tunnel ID 1 refused: %v", err)
	}
}

// stopCCNPayload builds a well-formed StopCCN body.
func stopCCNPayload() []byte {
	buf := make([]byte, 64)
	off := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgStopCCN))
	off += WriteAVPUint16(buf, off, true, AVPAssignedTunnelID, 200)
	off += writeAVPResultCode(buf, off, ResultCodeValue{Result: 1})
	return buf[:off]
}

// RFC requirement: RFC2661-9-1 positive -- a StopCCN received on a tunnel with
// one established and one wait-connect session clears both sessions and sends
// no explicit call control message: handleStopCCN returns no datagram, the
// reliable engine's next send sequence number does not advance, and its send
// queue does not grow.
//
// TestRFC2661StopCCNClearsSessionsSilently checks both halves of the Section
// 6.4 sentence on one tunnel.
func TestRFC2661StopCCNClearsSessionsSilently(t *testing.T) {
	now := time.Now()
	logger := slog.Default()
	tun := newEstablishedTunnel(t, 0)
	tun.handleICRQ(buildICRQ(500, 1001), now, logger)
	for _, s := range tun.sessions {
		tun.handleICCN(s, buildICCN(10000000, 2), now, logger)
	}
	tun.handleICRQ(buildICRQ(501, 1002), now, logger)
	if tun.sessionCount() != 2 {
		t.Fatalf("setup: %d sessions, want 2", tun.sessionCount())
	}
	sendSeq := tun.engine.nextSendSeq
	// The first ICRP is unacknowledged, so the window is full and a message
	// handleStopCCN enqueued would wait in sendQueue without taking an Ns.
	queued := len(tun.engine.sendQueue)

	out := tun.handleStopCCN(now, stopCCNPayload())
	if len(out) != 0 {
		t.Fatalf("StopCCN produced %d datagrams, want none", len(out))
	}
	if tun.engine.nextSendSeq != sendSeq {
		t.Fatalf("StopCCN enqueued %d control messages, want none", tun.engine.nextSendSeq-sendSeq)
	}
	if len(tun.engine.sendQueue) != queued {
		t.Fatalf("StopCCN queued %d control messages, want none", len(tun.engine.sendQueue)-queued)
	}
	if tun.sessionCount() != 0 {
		t.Fatalf("StopCCN left %d sessions", tun.sessionCount())
	}
}

// RFC requirement: RFC2661-10-1 positive -- a CDN for an established session
// cleans up every resource the session held: the session leaves the tunnel,
// one kernel teardown for its local session ID and one session-down are
// queued, and a CDN for a wait-connect session removes it and queues its
// session-down with no kernel teardown, since it holds no kernel session. In
// both cases Ze sends back nothing: handleCDN returns no datagram, the
// reliable engine's next send sequence number does not advance, and its send
// queue does not grow.
//
// TestRFC2661CDNCleansUpAndSendsNothing drives handleCDN in the two session
// states and reads the queues the reactor drains into the kernel worker, the
// PPP driver and the address pool.
func TestRFC2661CDNCleansUpAndSendsNothing(t *testing.T) {
	now := time.Now()
	logger := slog.Default()
	for _, established := range []bool{true, false} {
		tun := newEstablishedTunnel(t, 0)
		tun.handleICRQ(buildICRQ(500, 1001), now, logger)
		var sess *L2TPSession
		for _, s := range tun.sessions {
			sess = s
		}
		if established {
			tun.handleICCN(sess, buildICCN(10000000, 2), now, logger)
		}
		sid := sess.localSID
		sendSeq := tun.engine.nextSendSeq
		// The ICRP is unacknowledged, so the window is full and a message
		// handleCDN enqueued would wait in sendQueue without taking an Ns.
		queued := len(tun.engine.sendQueue)

		out := tun.handleCDN(sid, buildCDN(1, 500), logger)
		if len(out) != 0 {
			t.Fatalf("established=%v: CDN produced %d datagrams, want none", established, len(out))
		}
		if tun.engine.nextSendSeq != sendSeq {
			t.Fatalf("established=%v: CDN enqueued a control message", established)
		}
		if len(tun.engine.sendQueue) != queued {
			t.Fatalf("established=%v: CDN queued a control message", established)
		}
		if tun.sessionCount() != 0 {
			t.Fatalf("established=%v: session survived the CDN", established)
		}
		teardowns, downs := tun.drainPendingTeardowns()
		if len(downs) != 1 || downs[0].localSID != sid {
			t.Fatalf("established=%v: session-downs %+v, want one for SID %d", established, downs, sid)
		}
		wantTeardowns := 0
		if established {
			wantTeardowns = 1
		}
		if len(teardowns) != wantTeardowns {
			t.Fatalf("established=%v: %d kernel teardowns, want %d", established, len(teardowns), wantTeardowns)
		}
		if established && teardowns[0].localSID != sid {
			t.Fatalf("kernel teardown for SID %d, want %d", teardowns[0].localSID, sid)
		}
	}
}

// tunnelWithSessionLogging returns an established tunnel carrying one session
// whose logger writes into the returned buffer.
func tunnelWithSessionLogging(t *testing.T) (*L2TPTunnel, *bytes.Buffer) {
	t.Helper()
	tun := newEstablishedTunnel(t, 0)
	tun.handleICRQ(buildICRQ(500, 1001), time.Now(), slog.Default())
	if tun.sessionCount() != 1 {
		t.Fatalf("setup: %d sessions, want 1", tun.sessionCount())
	}
	// The ICRP fills the one-message congestion window; acknowledge it so a
	// StopCCN leaves at once rather than queueing behind it.
	ackAll(t, tun, time.Now())
	var logs bytes.Buffer
	tun.logger = slog.New(slog.NewTextHandler(&logs, nil))
	return tun, &logs
}

// RFC requirement: RFC2661-7.1-1 positive -- a malformed control message (a
// body with no well-formed Message Type AVP) and an invalid one (an unknown
// Message Type marked mandatory) are each logged at warning level and clear the
// control connection: handleMessage sends one StopCCN, removes the session and
// leaves the tunnel closed.
// RFC requirement: RFC2661-7.1-1 negative -- an unknown Message Type with the
// M bit clear is neither invalid nor malformed: handleMessage sends nothing,
// logs no warning, and the tunnel and its session stay up.
//
// TestRFC2661InvalidOrMalformedMessageClearsControlConnection drives the tunnel
// dispatcher with the three entries the reliable engine can deliver.
func TestRFC2661InvalidOrMalformedMessageClearsControlConnection(t *testing.T) {
	now := time.Now()
	for _, entry := range []RecvEntry{
		{Malformed: true},
		{MessageType: 0x7777, MessageTypeMandatory: true},
	} {
		tun, logs := tunnelWithSessionLogging(t)
		out := tun.handleMessage(entry, now, TunnelDefaults{}, nil)
		if len(out) != 1 {
			t.Fatalf("%+v: %d datagrams, want one StopCCN", entry, len(out))
		}
		if mt := sentMessageType(t, out[0].bytes); mt != MsgStopCCN {
			t.Fatalf("%+v: answered with message type %d, want StopCCN", entry, mt)
		}
		if tun.sessionCount() != 0 || tun.state != L2TPTunnelClosed {
			t.Fatalf("%+v: tunnel %s with %d sessions, want closed with none", entry, tun.state, tun.sessionCount())
		}
		if !bytes.Contains(logs.Bytes(), []byte("level=WARN")) {
			t.Fatalf("%+v: nothing logged at warning level: %q", entry, logs.String())
		}
	}

	tun, logs := tunnelWithSessionLogging(t)
	out := tun.handleMessage(RecvEntry{MessageType: 0x7777}, now, TunnelDefaults{}, nil)
	if len(out) != 0 {
		t.Fatalf("optional unknown Message Type: %d datagrams, want none", len(out))
	}
	if tun.sessionCount() != 1 || tun.state != L2TPTunnelEstablished {
		t.Fatalf("optional unknown Message Type: tunnel %s with %d sessions", tun.state, tun.sessionCount())
	}
	if bytes.Contains(logs.Bytes(), []byte("level=WARN")) {
		t.Fatalf("optional unknown Message Type logged a warning: %q", logs.String())
	}
}

// RFC requirement: RFC2661-5.8-1 positive -- each retransmission of an
// unacknowledged message waits twice the previous interval: with RTimeout 1s
// and a cap of 8s the retransmits fire at 1s, 3s and 7s after the send, and no
// retransmit fires 1ns before any of those deadlines.
//
// TestRFC2661RetransmitBackoffDoubles ticks the engine on each side of each
// deadline, which a constant-interval engine fails at the second one.
func TestRFC2661RetransmitBackoffDoubles(t *testing.T) {
	e := NewReliableEngine(ReliableConfig{
		PeerTunnelID:   200,
		RTimeout:       time.Second,
		RTimeoutCap:    8 * time.Second,
		MaxRetransmit:  10,
		RecvWindow:     4,
		InitialPeerRWS: 4,
	})
	t0 := time.Unix(0, 0)
	mustEnqueue(t, e, 0, messageTypeAVP(1), t0)
	for i, deadline := range []time.Duration{time.Second, 3 * time.Second, 7 * time.Second} {
		if early := e.Tick(t0.Add(deadline - time.Nanosecond)); len(early.Retransmits) != 0 {
			t.Fatalf("retransmit %d fired before its %v deadline", i+1, deadline)
		}
		if due := e.Tick(t0.Add(deadline)); len(due.Retransmits) != 1 {
			t.Fatalf("retransmit %d did not fire at %v", i+1, deadline)
		}
	}
}

// RFC requirement: RFC2661-5.8-6 positive -- with a peer Receive Window Size
// of 4 and the congestion window grown, the send window admits 4 outstanding
// messages: available() is 4 with none outstanding and 1 with 3 outstanding.
// RFC requirement: RFC2661-5.8-6 negative -- with 4 messages outstanding
// against a peer window of 4, available() is 0, so a fifth message backs off.
//
// TestRFC2661PeerWindowOfFourAccepted grows the congestion window to the peer
// window and reads the admission count on each side of the fourth message.
func TestRFC2661PeerWindowOfFourAccepted(t *testing.T) {
	w := newWindow(4)
	for range 3 {
		w.onAck()
	}
	if got := w.available(0); got != 4 {
		t.Fatalf("available(0) = %d, want 4", got)
	}
	if got := w.available(3); got != 1 {
		t.Fatalf("available(3) = %d, want 1", got)
	}
	if got := w.available(4); got != 0 {
		t.Fatalf("available(4) = %d, want 0", got)
	}
}

// RFC requirement: RFC2661-5.8-7 positive -- a tunnel closed by a StopCCN
// stays in the reactor's tunnel map, with its reliable engine, for the full
// default retransmission interval of 1+2+4+8+16 = 31s: reapExpiredLocked 1ns
// before that returns nothing and leaves the tunnel in place.
// RFC requirement: RFC2661-5.8-7 negative -- at 31s after the close
// reapExpiredLocked removes the tunnel, so the state is not kept beyond the
// interval.
//
// TestRFC2661ClosedTunnelKeptForRetransmissionInterval builds the tunnel with
// the ReliableConfig the reactor passes and drives the reactor's reap.
func TestRFC2661ClosedTunnelKeptForRetransmissionInterval(t *testing.T) {
	r := &l2tpReactor{
		logger:           slog.Default(),
		tunnelsByLocalID: make(map[uint16]*L2TPTunnel),
		tunnelsByPeer:    make(map[peerKey]*L2TPTunnel),
	}
	t0 := time.Unix(0, 0)
	tun := newTunnel(100, 200, netip.MustParseAddrPort("10.0.0.1:1701"),
		ReliableConfig{RecvWindow: 4}, slog.Default(), t0)
	tun.state = L2TPTunnelEstablished
	r.tunnelsByLocalID[tun.localTID] = tun
	tun.handleStopCCN(t0, stopCCNPayload())
	if tun.state != L2TPTunnelClosed {
		t.Fatalf("setup: tunnel %s after StopCCN, want closed", tun.state)
	}

	interval := (1 + 2 + 4 + 8 + 16) * time.Second
	if reaped, _, _ := r.reapExpiredLocked(t0.Add(interval - time.Nanosecond)); len(reaped) != 0 {
		t.Fatalf("tunnel reaped %v before the full retransmission interval", reaped)
	}
	if r.tunnelsByLocalID[tun.localTID] != tun {
		t.Fatalf("tunnel left the map before the full retransmission interval")
	}
	if reaped, _, _ := r.reapExpiredLocked(t0.Add(interval)); len(reaped) != 1 {
		t.Fatalf("tunnel not reaped at the full retransmission interval: %v", reaped)
	}
	if _, kept := r.tunnelsByLocalID[tun.localTID]; kept {
		t.Fatalf("tunnel still in the map after the retransmission interval")
	}
}
