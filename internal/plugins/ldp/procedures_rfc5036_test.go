// Design: docs/architecture/ldp/mpls-ldp.md -- RFC 5036 session procedures
//
// RFC 5036 session procedures that ze proves at the boundary it owns.
// Each test drives processMessages over a net.Pipe and reads what ze writes
// back, so the assertion is the wire: the Notification's status code and the
// message it refers to, or the absence of any Notification.
//
// Covered here:
//   - Section 3.3, the U bit of an unknown TLV (RFC5036-3.3-1)
//   - Section 3.4.4.1, the Hop Count check (RFC5036-3.4.4.1-1, -2)
//   - Section 3.5.3, Session Rejected/No Hello (RFC5036-3.5.3-9)
//   - Section 2.5.3, the session setup backoff (RFC5036-2.5.3-3, -4)
//
// Related: rfc5036_test.go (the helpers), sessionparams_rfc5036_test.go.
package ldp

import (
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// tlvTypeUnassigned is a TLV type no RFC registers, so ze does not know it.
const tlvTypeUnassigned uint16 = 0x3F00

// mappingLabel is the label every Label Mapping these tests build carries.
const mappingLabel uint32 = 100

// encodeLabelMappingPDU builds one PDU carrying one Label Mapping message for
// prefix and mappingLabel, followed by extra TLVs, each written raw so a test
// can set the U bit or a type ze does not know. Message ID is 9.
func encodeLabelMappingPDU(prefix netip.Prefix, extra ...TLV) []byte {
	var buf [256]byte
	off := ldpHeaderLen
	msgStart := off
	off += encodeMessageHeader(buf[off:], MessageHeader{Type: MsgTypeLabelMapping, MessageID: 9})
	off += encodeFECTLV(buf[:], off, prefix)
	off += encodeLabelTLV(buf[:], off, mappingLabel)
	for _, t := range extra {
		off += EncodeTLV(buf[off:], t)
	}
	binary.BigEndian.PutUint16(buf[msgStart+2:msgStart+4], uint16(off-msgStart-ldpTLVHdrLen))
	encodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  uint16(off - ldpHeaderLen + 6),
		LSRID:      [4]byte{10, 0, 0, 2},
		LabelSpace: 0,
	})
	return buf[:off]
}

// hopCountTLV is a Hop Count TLV (RFC 5036 Section 3.4.4) carrying value.
func hopCountTLV(value uint8) TLV {
	return TLV{Type: TLVTypeHopCount, Length: 1, Value: []byte{value}}
}

// expectNoWrite fails the test when ze writes anything on remote inside
// ldpNoWriteWindow: the Notification the RFC forbids here would arrive well
// inside it, and processMessages has returned before the wait starts.
func expectNoWrite(t *testing.T, remote net.Conn) {
	t.Helper()
	if err := remote.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var one [1]byte
	n, err := remote.Read(one[:])
	if err == nil || n != 0 {
		t.Fatalf("ze wrote %d octets, want nothing on the wire", n)
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("read error = %v, want a deadline timeout", err)
	}
}

// operationalSession is a session that has sent its Initialization and got the
// peer's, so a Label Mapping is in order. hopCountMax is the configured maximum.
func operationalSession(conn net.Conn, hopCountMax uint8) *Session {
	sess := NewSession(conn, SessionConfig{
		LocalLSRID:    [4]byte{10, 0, 0, 1},
		PeerLSRID:     [4]byte{10, 0, 0, 2},
		PeerAddr:      netip.MustParseAddr("10.0.0.2"),
		KeepaliveTime: DefaultKeepaliveTime,
		HopCountMax:   hopCountMax,
	}, newLIB(), slogutil.DiscardLogger())
	sess.state = StateOperational
	return sess
}

// --------------------------------------------------------------------------
// RFC5036-3.3-1 -- the U bit of an unknown TLV
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-3.3-1 positive -- a Label Mapping carrying a TLV type
// ze does not know with the U bit set is processed as if the TLV were absent:
// the mapping reaches the label callback with its FEC and label, and ze writes
// no Notification.
func TestRFC5036UnknownTLVWithUBitSetIsSkipped(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	rx := operationalSession(local, DefaultHopCountMax)
	prefix := netip.MustParsePrefix("10.1.0.0/16")
	pdu := encodeLabelMappingPDU(prefix,
		TLV{Type: tlvUnknownBit | tlvTypeUnassigned, Length: 2, Value: []byte{0xAA, 0xBB}})

	var got []labelMappingMessage
	err := rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, 0,
		func(lm labelMappingMessage, _ [4]byte) { got = append(got, lm) }, nil, nil)
	if err != nil {
		t.Fatalf("processMessages: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("label callback fired %d times, want 1: the unknown TLV with U=1 was not skipped", len(got))
	}
	if got[0].FEC.Prefix != prefix || got[0].Label != mappingLabel {
		t.Errorf("mapping = %s label %d, want %s label 100", got[0].FEC.Prefix, got[0].Label, prefix)
	}
	expectNoWrite(t, remote)
}

// RFC requirement: RFC5036-3.3-1 negative -- a Label Mapping carrying a TLV type
// ze does not know with the U bit clear is ignored whole: the label callback
// never fires, ze answers an Unknown TLV Notification (0x00000006, E bit clear)
// referring to that Label Mapping, and the session stays up because
// processMessages returns nil.
func TestRFC5036UnknownTLVWithUBitClearIsRefused(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	rx := operationalSession(local, DefaultHopCountMax)
	pdu := encodeLabelMappingPDU(netip.MustParsePrefix("10.1.0.0/16"),
		TLV{Type: tlvTypeUnassigned, Length: 2, Value: []byte{0xAA, 0xBB}})

	fired := 0
	done := make(chan error, 1)
	go func() {
		done <- rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, 0,
			func(labelMappingMessage, [4]byte) { fired++ }, nil, nil)
	}()

	status, referID, referType := readNotificationStatus(t, remote)
	if status != statusUnknownTLV {
		t.Errorf("status code = %#08x, want %#08x (Unknown TLV, E bit clear)", status, statusUnknownTLV)
	}
	if referID != 9 {
		t.Errorf("status refers to message %d, want the Label Mapping message id 9", referID)
	}
	if referType != MsgTypeLabelMapping {
		t.Errorf("status refers to message type %#x, want Label Mapping (%#x)", referType, MsgTypeLabelMapping)
	}
	if err := <-done; err != nil {
		t.Errorf("processMessages error = %v, want nil: an unknown TLV is not a fatal error", err)
	}
	if fired != 0 {
		t.Errorf("label callback fired %d times, want 0: the message was not ignored", fired)
	}
	if rx.State() != StateOperational {
		t.Errorf("state = %s, want Operational: the session must stay up", rx.State())
	}
}

// --------------------------------------------------------------------------
// RFC5036-3.4.4.1-1, -2 -- the Hop Count check
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-3.4.4.1-1 positive -- a Label Mapping whose Hop Count
// TLV is at the configured maximum is checked and applied: the callback receives
// the decoded hop count and ze writes no Notification.
// RFC requirement: RFC5036-3.4.4.1-2 negative -- a hop count that does not exceed
// the maximum draws no Loop Detected Notification.
func TestRFC5036HopCountWithinMaximumIsApplied(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	rx := operationalSession(local, 4)
	pdu := encodeLabelMappingPDU(netip.MustParsePrefix("10.1.0.0/16"), hopCountTLV(4))

	var got []labelMappingMessage
	err := rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, 0,
		func(lm labelMappingMessage, _ [4]byte) { got = append(got, lm) }, nil, nil)
	if err != nil {
		t.Fatalf("processMessages: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("label callback fired %d times, want 1", len(got))
	}
	if !got[0].HasHopCount || got[0].HopCount != 4 {
		t.Errorf("hop count = %d (present %v), want 4 present: the Hop Count TLV was not read",
			got[0].HopCount, got[0].HasHopCount)
	}
	expectNoWrite(t, remote)
}

// RFC requirement: RFC5036-3.4.4.1-1 negative -- a Label Mapping whose Hop Count
// TLV exceeds the configured maximum is not applied: the label callback never
// fires.
// RFC requirement: RFC5036-3.4.4.1-2 positive -- that mapping is answered with a
// Loop Detected Notification (0x0000000B, E bit clear) referring to it, and the
// session stays up.
func TestRFC5036HopCountAboveMaximumDrawsLoopDetected(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	rx := operationalSession(local, 4)
	pdu := encodeLabelMappingPDU(netip.MustParsePrefix("10.1.0.0/16"), hopCountTLV(5))

	fired := 0
	done := make(chan error, 1)
	go func() {
		done <- rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, 0,
			func(labelMappingMessage, [4]byte) { fired++ }, nil, nil)
	}()

	status, referID, referType := readNotificationStatus(t, remote)
	if status != statusLoopDetected {
		t.Errorf("status code = %#08x, want %#08x (Loop Detected, E bit clear)", status, statusLoopDetected)
	}
	if referID != 9 {
		t.Errorf("status refers to message %d, want the Label Mapping message id 9", referID)
	}
	if referType != MsgTypeLabelMapping {
		t.Errorf("status refers to message type %#x, want Label Mapping (%#x)", referType, MsgTypeLabelMapping)
	}
	if err := <-done; err != nil {
		t.Errorf("processMessages error = %v, want nil: Loop Detected is advisory", err)
	}
	if fired != 0 {
		t.Errorf("label callback fired %d times, want 0: a looped mapping was applied", fired)
	}
	if rx.State() != StateOperational {
		t.Errorf("state = %s, want Operational", rx.State())
	}
}

// --------------------------------------------------------------------------
// RFC5036-3.5.3-9 -- Session Rejected/No Hello
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-3.5.3-9 positive -- an Initialization whose PDU header
// carries the LDP Identifier of the Hello adjacency the session was opened for
// is accepted: the session goes operational and ze writes no Notification.
func TestRFC5036InitFromHelloAdjacencyAccepted(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	rx := rfcTestSession(local)
	rx.state = StateOpenSent

	pdu := encodeInitPDU(ldpVersion, 30)
	if err := rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, 0, nil, nil, nil); err != nil {
		t.Fatalf("processMessages: %v", err)
	}
	if rx.State() != StateOperational {
		t.Errorf("state = %s, want Operational", rx.State())
	}
	expectNoWrite(t, remote)
}

// RFC requirement: RFC5036-3.5.3-9 negative -- an Initialization whose PDU header
// carries an LDP Identifier that matches no Hello adjacency of the session is
// refused with a Session Rejected/No Hello Notification (0x00000010, E bit set)
// referring to it, and the session is not established.
func TestRFC5036InitWithoutHelloAdjacencyRejected(t *testing.T) {
	for _, tc := range []struct {
		name       string
		lsrID      [4]byte
		labelSpace uint16
	}{
		{"other lsr-id", [4]byte{10, 0, 0, 9}, 0},
		{"other label space", [4]byte{10, 0, 0, 2}, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local, remote := net.Pipe()
			defer func() { _ = local.Close() }()
			defer func() { _ = remote.Close() }()

			rx := rfcTestSession(local)
			rx.state = StateOpenSent

			done := make(chan error, 1)
			go func() {
				pdu := encodeInitPDU(ldpVersion, 30)
				done <- rx.processMessages(pdu[ldpHeaderLen:], tc.lsrID, tc.labelSpace, nil, nil, nil)
			}()

			status, referID, referType := readNotificationStatus(t, remote)
			if status != statusSessionRejectedNoHello {
				t.Errorf("status code = %#08x, want %#08x (Session Rejected/No Hello, E bit set)",
					status, statusSessionRejectedNoHello)
			}
			if referID != 7 {
				t.Errorf("status refers to message %d, want the Initialization message id 7", referID)
			}
			if referType != MsgTypeInitialize {
				t.Errorf("status refers to message type %#x, want Initialization (%#x)", referType, MsgTypeInitialize)
			}
			if err := <-done; !errors.Is(err, errNoHelloAdjacency) {
				t.Errorf("processMessages error = %v, want errNoHelloAdjacency", err)
			}
			if rx.State() == StateOperational {
				t.Error("session went operational on an Initialization matching no Hello adjacency")
			}
		})
	}
}

// --------------------------------------------------------------------------
// RFC5036-2.5.3-3, -4 -- the session setup backoff
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-2.5.3-3 positive -- each failed setup attempt doubles
// the delay before the next one, up to the maximum.
// RFC requirement: RFC5036-2.5.3-4 positive -- the first delay is 15 seconds and
// the delays grow to a maximum of 2 minutes, where they stay.
func TestRFC5036SetupRetryBackoffGrows(t *testing.T) {
	retries := map[string]*setupRetry{}
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	want := []time.Duration{15 * time.Second, 30 * time.Second, time.Minute, 2 * time.Minute, 2 * time.Minute}
	for i, delay := range want {
		recordSetupFailure(slogutil.DiscardLogger(), retries, "10.0.0.2:0", now)
		r := retries["10.0.0.2:0"]
		if r == nil {
			t.Fatal("no backoff entry recorded for the adjacency")
		}
		if r.delay != delay {
			t.Errorf("failure %d: delay = %s, want %s", i+1, r.delay, delay)
		}
		if got := r.notBefore.Sub(now); got != delay {
			t.Errorf("failure %d: next attempt allowed after %s, want %s", i+1, got, delay)
		}
		now = r.notBefore
	}
}

// RFC requirement: RFC5036-2.5.3-3 negative -- no setup attempt is allowed inside
// the delay a failure imposed.
// RFC requirement: RFC5036-2.5.3-4 negative -- an attempt 14 seconds after the
// first failure is refused, and one at 15 seconds is allowed.
func TestRFC5036SetupRetryRefusedInsideDelay(t *testing.T) {
	retries := map[string]*setupRetry{}
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	recordSetupFailure(slogutil.DiscardLogger(), retries, "10.0.0.2:0", now)
	r := retries["10.0.0.2:0"]
	if r == nil {
		t.Fatal("no backoff entry recorded for the adjacency")
	}
	if r.allows(now) {
		t.Error("an attempt at the moment of the failure is allowed")
	}
	if r.allows(now.Add(14 * time.Second)) {
		t.Error("an attempt 14s after the failure is allowed, want refused until 15s")
	}
	if !r.allows(now.Add(15 * time.Second)) {
		t.Error("an attempt 15s after the failure is refused")
	}
}

// --------------------------------------------------------------------------
// RFC5036-3.1-1 -- the LSR identifier is globally unique
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-3.1-1 positive -- a Hello whose LDP Identifier names
// another LSR forms an adjacency, and the identifier ze puts in its own PDU
// header is the configured lsr-id, unchanged.
func TestRFC5036DistinctLSRIDFormsAdjacency(t *testing.T) {
	table := newAdjacencyTable()
	pkt := encodeHelloPDU([4]byte{10, 0, 0, 2}, 15, 0)
	processDiscoveryPacket(pkt, [4]byte{10, 0, 0, 1}, "eth0", table, nil, slogutil.DiscardLogger())
	if table.Len() != 1 {
		t.Fatalf("adjacencies = %d, want 1 for a Hello from another LSR", table.Len())
	}

	var buf [ldpHeaderLen]byte
	encodePDUHeader(buf[:], PDUHeader{Version: ldpVersion, PDULength: 6, LSRID: [4]byte{10, 0, 0, 1}})
	if got := [4]byte(buf[4:8]); got != [4]byte{10, 0, 0, 1} {
		t.Errorf("PDU header LSR ID = %v, want the configured 10.0.0.1", got)
	}
}

// RFC requirement: RFC5036-3.1-1 negative -- a Hello carrying ze's own LDP
// Identifier is the one collision ze can see, and it forms no adjacency: two
// LSRs sharing an identifier cannot tell each other's label spaces apart.
func TestRFC5036OwnLSRIDFormsNoAdjacency(t *testing.T) {
	table := newAdjacencyTable()
	pkt := encodeHelloPDU([4]byte{10, 0, 0, 1}, 15, 0)
	fired := 0
	processDiscoveryPacket(pkt, [4]byte{10, 0, 0, 1}, "eth0", table,
		func(*Adjacency) { fired++ }, slogutil.DiscardLogger())
	if table.Len() != 0 {
		t.Errorf("adjacencies = %d, want 0 for a Hello carrying our own LSR ID", table.Len())
	}
	if fired != 0 {
		t.Errorf("adjacency callback fired %d times, want 0", fired)
	}
}

// --------------------------------------------------------------------------
// RFC5036-2.6.1.2-2 -- ordered control waits for the downstream label
// --------------------------------------------------------------------------

// readSessionUp drives sess through runSession to operational over remote: it
// reads ze's Initialization and KeepAlive, then answers with the peer's
// Initialization. It returns after ze has taken the session operational.
func readSessionUp(t *testing.T, sess *Session, remote net.Conn) func() {
	t.Helper()
	stop := runSessionForTest(t, sess)
	_, msgHdr, _ := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeInitialize {
		t.Fatalf("first message = %#x, want Initialization", msgHdr.Type)
	}
	_, msgHdr, _ = readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeKeepAlive {
		t.Fatalf("second message = %#x, want KeepAlive", msgHdr.Type)
	}
	if _, err := remote.Write(encodeInitPDU(ldpVersion, 30)); err != nil {
		t.Fatalf("write peer Initialization: %v", err)
	}
	return stop
}

// RFC requirement: RFC5036-2.6.1.2-2 positive -- a FEC ze is the egress for is
// mapped as soon as the session is operational: the wait binds only a FEC the
// LSR is not the egress for, and the Label Mapping for the egress FEC goes out
// with no downstream label received.
func TestRFC5036EgressFECMappedWithoutWaiting(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	egress := netip.MustParsePrefix("10.0.0.1/32")
	want := sess.lib.EnsureLocal(egress)

	stop := readSessionUp(t, sess, remote)
	defer stop()

	_, msgHdr, body := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeLabelMapping {
		t.Fatalf("message after operational = %#x, want Label Mapping (%#x)", msgHdr.Type, MsgTypeLabelMapping)
	}
	lm, err := decodeLabelMapping(msgHdr.MessageID, body)
	if err != nil {
		t.Fatalf("decodeLabelMapping: %v", err)
	}
	if lm.FEC.Prefix != egress || lm.Label != want.Label {
		t.Errorf("mapping = %s label %d, want %s label %d", lm.FEC.Prefix, lm.Label, egress, want.Label)
	}
}

// RFC requirement: RFC5036-2.6.1.2-2 negative -- a FEC ze is not the egress for
// and holds no downstream label for is not mapped: with only a remote binding
// for it from another peer, the operational session receives no Label Mapping
// for that FEC.
func TestRFC5036NonEgressFECNotMappedBeforeDownstreamLabel(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	transit := netip.MustParsePrefix("10.9.0.0/16")
	// A binding learned from a third LSR is not a label from a downstream LSR
	// for this FEC on this session's path, and it creates no local binding.
	sess.lib.AddRemote(transit, 300, "10.0.0.3:0", netip.MustParseAddr("10.0.0.3"), netip.Addr{})

	stop := readSessionUp(t, sess, remote)
	defer stop()

	if err := remote.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var hdr [ldpHeaderLen + ldpMsgHdrLen]byte
	n, err := remote.Read(hdr[:])
	if err == nil {
		t.Fatalf("ze wrote %d octets after operational, want no Label Mapping for a FEC it is not the egress for", n)
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("read error = %v, want a deadline timeout", err)
	}
	if bindings := sess.lib.localBindings(); len(bindings) != 0 {
		t.Errorf("local bindings = %v, want none: a remote binding must not create a local one", bindings)
	}
}
