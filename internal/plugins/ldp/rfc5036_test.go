// RFC: rfc/short/rfc5036.md -- gated MUST coverage for the LDP specification
// Design: plan/learned/920-mpls-ldp.md -- LDP plugin
//
// These tests bind RFC 5036 MUST-level obligations to the producing code. Each
// obligation is pinned from both sides: a positive test that the required behavior
// happens, and a negative test that the code distinguishes the required case from a
// case that must be treated differently.
//
// VALIDATES: the gated MUSTs of rfc/short/rfc5036.md that ze meets -- PDU version,
// Common Hello Parameters reserved bits, Common Session Parameters protocol version,
// Initialization exchange, KeepAlive negotiation and pacing, Hello Hold Time defaults.
// PREVENTS: a Hello Hold Time of 0 being read as "drop the adjacency" (RFC 5036
// Section 3.5.2 makes it "use the default": 15s Link, 45s Targeted), and a Targeted
// Hello silently inheriting the Link default.
package ldp

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

func rfcTestSession(conn net.Conn) *Session {
	return NewSession(
		conn,
		[4]byte{10, 0, 0, 1}, 0,
		[4]byte{10, 0, 0, 2}, 0,
		netip.MustParseAddr("10.0.0.2"),
		NewLIB(),
		slogutil.DiscardLogger(),
	)
}

// ldpReadTimeout bounds every wire read in this file: a message that is due arrives
// well inside it, and a message that is not due never does.
const ldpReadTimeout = 2 * time.Second

// readLDPPDU reads one LDP PDU from conn and returns its header, the header of the
// first message it carries, and that message's body (after the message header).
func readLDPPDU(t *testing.T, conn net.Conn) (PDUHeader, MessageHeader, []byte) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(ldpReadTimeout)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var hdr [ldpHeaderLen]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		t.Fatalf("read PDU header: %v", err)
	}
	pdu, err := DecodePDUHeader(hdr[:])
	if err != nil {
		t.Fatalf("DecodePDUHeader: %v", err)
	}
	body := make([]byte, int(pdu.PDULength)-6)
	if _, err := io.ReadFull(conn, body); err != nil {
		t.Fatalf("read PDU body: %v", err)
	}
	msgHdr, err := DecodeMessageHeader(body)
	if err != nil {
		t.Fatalf("DecodeMessageHeader: %v", err)
	}
	return pdu, msgHdr, body[ldpMsgHdrLen:]
}

// encodeInitPDU builds a complete Initialization PDU with the given Common Session
// Parameters protocol version and keepalive time.
func encodeInitPDU(version, keepalive uint16) []byte {
	var buf [256]byte
	bodyLen := EncodeInit(buf[ldpHeaderLen:], InitMessage{
		MessageID:       7,
		ProtocolVersion: version,
		KeepaliveTime:   keepalive,
		MaxPDULength:    4096,
	})
	EncodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  uint16(bodyLen + 6),
		LSRID:      [4]byte{10, 0, 0, 2},
		LabelSpace: 0,
	})
	return buf[:ldpHeaderLen+bodyLen]
}

// encodeHelloPDU builds a complete Hello PDU with an explicitly chosen Common Hello
// Parameters flags word, so a test can set the reserved bits the encoder never sets.
func encodeHelloPDU(lsrID [4]byte, holdTime, flags uint16) []byte {
	var value [4]byte
	binary.BigEndian.PutUint16(value[0:2], holdTime)
	binary.BigEndian.PutUint16(value[2:4], flags)

	var buf [128]byte
	off := ldpHeaderLen
	off += EncodeMessageHeader(buf[off:], MessageHeader{Type: MsgTypeHello, MessageID: 1})
	off += EncodeTLV(buf[off:], TLV{Type: TLVTypeCommonHello, Length: 4, Value: value[:]})
	bodyLen := off - ldpHeaderLen
	binary.BigEndian.PutUint16(buf[ldpHeaderLen+2:ldpHeaderLen+4], uint16(bodyLen-ldpTLVHdrLen))
	EncodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  uint16(bodyLen + 6),
		LSRID:      lsrID,
		LabelSpace: 0,
	})
	return buf[:off]
}

// --------------------------------------------------------------------------
// RFC5036-x-1 -- PDU header Version is 1
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-x-1 positive -- every PDU ze emits carries Version 1 and a
// received Version 1 PDU header decodes and is accepted (wire.go DecodePDUHeader).
func TestRFC5036PDUVersionOneAccepted(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	go func() { _ = sess.SendInit() }()

	pdu, _, _ := readLDPPDU(t, remote)
	if pdu.Version != ldpVersion {
		t.Fatalf("emitted PDU version = %d, want %d", pdu.Version, ldpVersion)
	}

	// And the receive side accepts that same header.
	var hdr [ldpHeaderLen]byte
	EncodePDUHeader(hdr[:], PDUHeader{Version: 1, PDULength: 14, LSRID: [4]byte{10, 0, 0, 2}})
	got, err := DecodePDUHeader(hdr[:])
	if err != nil {
		t.Fatalf("DecodePDUHeader(version 1): %v", err)
	}
	if got.Version != 1 {
		t.Errorf("decoded version = %d, want 1", got.Version)
	}
}

// RFC requirement: RFC5036-x-1 negative -- a PDU header carrying any version other than
// 1 is rejected, and a discovery Hello wrapped in one creates no adjacency.
func TestRFC5036PDUVersionOtherRejected(t *testing.T) {
	for _, version := range []uint16{0, 2, 65535} {
		var hdr [ldpHeaderLen]byte
		EncodePDUHeader(hdr[:], PDUHeader{Version: version, PDULength: 14})
		if _, err := DecodePDUHeader(hdr[:]); !errors.Is(err, errBadVersion) {
			t.Errorf("DecodePDUHeader(version %d) error = %v, want errBadVersion", version, err)
		}
	}

	// Production path: processDiscoveryPacket must drop the Hello, not adjacency it.
	table := NewAdjacencyTable()
	pkt := encodeHelloPDU([4]byte{10, 0, 0, 2}, 15, 0)
	binary.BigEndian.PutUint16(pkt[0:2], 2) // corrupt the version
	processDiscoveryPacket(pkt, [4]byte{10, 0, 0, 1}, "eth0", table, nil, slogutil.DiscardLogger())
	if table.Len() != 0 {
		t.Errorf("adjacencies = %d, want 0 (version 2 Hello must be dropped)", table.Len())
	}
}

// --------------------------------------------------------------------------
// RFC5036-x-2 -- Common Hello Parameters reserved bits
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-x-2 positive -- EncodeHello leaves the 14 reserved bits of the
// Common Hello Parameters TLV zero, including when both defined flags are set.
func TestRFC5036HelloReservedBitsZeroOnTransmit(t *testing.T) {
	for _, tc := range []struct{ targeted, request bool }{
		{false, false}, {true, false}, {false, true}, {true, true},
	} {
		var buf [128]byte
		n := EncodeHello(buf[:], HelloMessage{MessageID: 1, HoldTime: 15, Targeted: tc.targeted, RequestTarget: tc.request})
		tlv, _, err := DecodeTLV(buf[ldpMsgHdrLen:n])
		if err != nil {
			t.Fatalf("DecodeTLV: %v", err)
		}
		if tlv.Type != TLVTypeCommonHello {
			t.Fatalf("first TLV = %#x, want Common Hello Parameters", tlv.Type)
		}
		flags := binary.BigEndian.Uint16(tlv.Value[2:4])
		if reserved := flags & 0x3FFF; reserved != 0 {
			t.Errorf("targeted=%v request=%v: reserved bits = %#x, want 0", tc.targeted, tc.request, reserved)
		}
	}
}

// RFC requirement: RFC5036-x-2 negative -- reserved bits set on the wire are ignored on
// receipt: they are not decoded as the T or R flags and do not corrupt the hold time.
func TestRFC5036HelloReservedBitsIgnoredOnReceipt(t *testing.T) {
	pkt := encodeHelloPDU([4]byte{10, 0, 0, 2}, 30, 0x3FFF) // every reserved bit set, T=R=0
	hello, err := DecodeHello(1, pkt[ldpHeaderLen+ldpMsgHdrLen:])
	if err != nil {
		t.Fatalf("DecodeHello: %v", err)
	}
	if hello.Targeted {
		t.Error("Targeted = true, want false (reserved bits must not be read as T)")
	}
	if hello.RequestTarget {
		t.Error("RequestTarget = true, want false (reserved bits must not be read as R)")
	}
	if hello.HoldTime != 30 {
		t.Errorf("HoldTime = %d, want 30", hello.HoldTime)
	}
}

// --------------------------------------------------------------------------
// RFC5036-x-3 -- Common Session Parameters Protocol Version is 1
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-x-3 positive -- ze sets Protocol Version 1 in the Common
// Session Parameters TLV it sends, and accepts an Initialization carrying version 1.
func TestRFC5036InitProtocolVersionOne(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	go func() { _ = sess.SendInit() }()

	_, msgHdr, msgBody := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeInitialize {
		t.Fatalf("message type = %#x, want Initialization", msgHdr.Type)
	}
	init, err := DecodeInit(msgHdr.MessageID, msgBody)
	if err != nil {
		t.Fatalf("DecodeInit: %v", err)
	}
	if init.ProtocolVersion != 1 {
		t.Errorf("sent Protocol Version = %d, want 1", init.ProtocolVersion)
	}

	// Receive side: version 1 is accepted and drives the FSM.
	rx := rfcTestSession(local)
	rx.state = StateOpenSent
	pdu := encodeInitPDU(1, 30)
	if err := rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, nil, nil, nil); err != nil {
		t.Fatalf("processMessages(version 1): %v", err)
	}
	if rx.State() != StateOperational {
		t.Errorf("state = %s, want operational", rx.State())
	}
}

// RFC requirement: RFC5036-x-3 negative -- an Initialization whose Common Session
// Parameters Protocol Version is not 1 is rejected and the session never goes operational.
func TestRFC5036InitProtocolVersionOtherRejected(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	for _, version := range []uint16{0, 2, 65535} {
		rx := rfcTestSession(local)
		rx.state = StateOpenSent
		pdu := encodeInitPDU(version, 30)
		err := rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, nil, nil, nil)
		if !errors.Is(err, errBadVersion) {
			t.Errorf("version %d: error = %v, want errBadVersion", version, err)
		}
		if rx.State() == StateOperational {
			t.Errorf("version %d: session went operational on a rejected Initialization", version)
		}
	}
}

// --------------------------------------------------------------------------
// RFC5036-2.5.1-1 -- an LSR sends the Initialization message to start a session
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-2.5.1-1 positive -- the first message ze puts on a new LDP
// session is an Initialization, and sending it moves the FSM to open-sent.
func TestRFC5036SessionSendsInitializationFirst(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	sent := make(chan error, 1)
	go func() { sent <- sess.SendInit() }()

	_, msgHdr, _ := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeInitialize {
		t.Fatalf("first message type = %#x, want Initialization (%#x)", msgHdr.Type, MsgTypeInitialize)
	}
	if err := <-sent; err != nil {
		t.Fatalf("SendInit: %v", err)
	}
	if sess.State() != StateOpenSent {
		t.Errorf("state = %s, want open-sent", sess.State())
	}
}

// RFC requirement: RFC5036-2.5.1-1 negative -- a session that has NOT sent its own
// Initialization does not reach the operational state when the peer's arrives; it only
// advances to open-received, so the session cannot come up without ze's Initialization.
func TestRFC5036SessionNotOperationalWithoutOwnInit(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local) // NewSession starts in StateInitialized: no Init sent
	if sess.State() != StateInitialized {
		t.Fatalf("initial state = %s, want initialized", sess.State())
	}

	fired := 0
	pdu := encodeInitPDU(1, 30)
	if err := sess.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, nil, nil, func() { fired++ }); err != nil {
		t.Fatalf("processMessages: %v", err)
	}
	if sess.State() == StateOperational {
		t.Error("session went operational without having sent its own Initialization")
	}
	if sess.State() != StateOpenReceived {
		t.Errorf("state = %s, want open-received", sess.State())
	}
	if fired != 0 {
		t.Errorf("onOperational fired %d times, want 0", fired)
	}
}

// --------------------------------------------------------------------------
// RFC5036-2.5.1-2 -- accept the lower of the two proposed KeepAlive Timer values
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-2.5.1-2 positive -- when the peer proposes a KeepAlive time
// lower than ours, the session adopts the peer's value.
func TestRFC5036KeepaliveNegotiationAdoptsLower(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	sess.keepaliveTime = 60 * time.Second
	sess.handleInit(InitMessage{ProtocolVersion: 1, KeepaliveTime: 20}, [4]byte{10, 0, 0, 2})

	if got := sess.currentKeepalive(); got != 20*time.Second {
		t.Errorf("keepalive = %v, want 20s (the lower of 60 and 20)", got)
	}
}

// RFC requirement: RFC5036-2.5.1-2 negative -- when the peer proposes a KeepAlive time
// HIGHER than ours, the peer's value is refused and ours is kept; the negotiated value is
// never raised.
func TestRFC5036KeepaliveNegotiationRefusesHigher(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	sess.keepaliveTime = 30 * time.Second
	sess.handleInit(InitMessage{ProtocolVersion: 1, KeepaliveTime: 180}, [4]byte{10, 0, 0, 2})

	if got := sess.currentKeepalive(); got != 30*time.Second {
		t.Errorf("keepalive = %v, want 30s (the peer's higher 180s must not be adopted)", got)
	}
}

// --------------------------------------------------------------------------
// RFC5036-2.5.3-1 -- periodically send KeepAlive messages
// --------------------------------------------------------------------------

// runSessionForTest starts runSession against a pipe end and returns a stop function.
func runSessionForTest(t *testing.T, sess *Session) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go runSession(ctx, slogutil.DiscardLogger(), sess, sess.lib, "10.0.0.2:0",
		newLDPFIB(nil, slogutil.DiscardLogger()), func() { close(done) })
	return func() {
		cancel()
		sess.Stop()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("runSession did not exit")
		}
	}
}

// RFC requirement: RFC5036-2.5.3-1 positive -- an established session emits KeepAlive
// messages repeatedly, paced at a third of the negotiated KeepAlive time.
func TestRFC5036KeepalivesSentPeriodically(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	sess.keepaliveTime = 150 * time.Millisecond // period = 50ms
	stop := runSessionForTest(t, sess)
	defer stop()

	// runSession sends Initialization first, then keepalives.
	_, msgHdr, _ := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeInitialize {
		t.Fatalf("first message = %#x, want Initialization", msgHdr.Type)
	}

	for i := range 4 {
		_, msgHdr, _ := readLDPPDU(t, remote)
		if msgHdr.Type != MsgTypeKeepAlive {
			t.Fatalf("message %d = %#x, want KeepAlive (%#x)", i, msgHdr.Type, MsgTypeKeepAlive)
		}
	}
}

// RFC requirement: RFC5036-2.5.3-1 negative -- KeepAlives are PACED by the negotiated
// interval, not emitted continuously: with a long keepalive time no second KeepAlive
// follows the initial one inside a short window.
func TestRFC5036KeepalivesNotSentContinuously(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	sess.keepaliveTime = 60 * time.Second // period = 20s: nothing more is due for 20s
	stop := runSessionForTest(t, sess)
	defer stop()

	_, msgHdr, _ := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeInitialize {
		t.Fatalf("first message = %#x, want Initialization", msgHdr.Type)
	}
	// The session-establishment KeepAlive that follows the Initialization.
	_, msgHdr, _ = readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeKeepAlive {
		t.Fatalf("second message = %#x, want KeepAlive", msgHdr.Type)
	}

	// No further KeepAlive is due for another 20s.
	if err := remote.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var buf [ldpHeaderLen]byte
	_, err := io.ReadFull(remote, buf[:])
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Errorf("read error = %v, want a timeout (KeepAlives must be paced, not continuous)", err)
	}
}

// --------------------------------------------------------------------------
// RFC5036-3.5.2-1 -- a Hello Hold Time of 0 means "use the default"
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-3.5.2-1 positive -- a Hello carrying Hold Time 0 keeps the
// adjacency and times it with the per-kind default: 15s for a Link Hello, 45s for a
// Targeted Hello.
func TestRFC5036HelloHoldTimeZeroUsesDefault(t *testing.T) {
	tests := []struct {
		name     string
		targeted bool
		want     time.Duration
	}{
		{"link", false, 15 * time.Second},
		{"targeted", true, 45 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := NewAdjacencyTable()
			pdu := PDUHeader{Version: ldpVersion, LSRID: [4]byte{10, 0, 0, 2}}
			hello := HelloMessage{
				HoldTime:      0,
				Targeted:      tt.targeted,
				TransportAddr: netip.MustParseAddr("10.0.0.2"),
			}

			adj, isNew := table.Update(pdu, hello, "eth0")
			if !isNew {
				t.Fatal("adjacency should be new")
			}
			// The obligation this pins: Hold Time 0 does NOT remove the adjacency.
			if table.Len() != 1 {
				t.Fatalf("adjacencies = %d, want 1 (Hold Time 0 must not drop the adjacency)", table.Len())
			}
			if adj.HoldTime != tt.want {
				t.Errorf("HoldTime = %v, want %v", adj.HoldTime, tt.want)
			}
			if adj.Expired(time.Now()) {
				t.Error("adjacency is immediately expired; Hold Time 0 must use the default, not 0")
			}
		})
	}
}

// RFC requirement: RFC5036-3.5.2-1 negative -- a Hello carrying a NON-zero Hold Time uses
// that value verbatim; the default is applied only for 0, and never overrides a peer's
// proposal in either direction.
func TestRFC5036HelloHoldTimeNonZeroNotDefaulted(t *testing.T) {
	tests := []struct {
		name     string
		targeted bool
		holdTime uint16
	}{
		{"link below default", false, 5},
		{"link above default", false, 90},
		{"targeted below default", true, 20},
		{"targeted above default", true, 120},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := NewAdjacencyTable()
			pdu := PDUHeader{Version: ldpVersion, LSRID: [4]byte{10, 0, 0, 3}}
			hello := HelloMessage{
				HoldTime:      tt.holdTime,
				Targeted:      tt.targeted,
				TransportAddr: netip.MustParseAddr("10.0.0.3"),
			}

			adj, _ := table.Update(pdu, hello, "eth0")
			want := time.Duration(tt.holdTime) * time.Second
			if adj.HoldTime != want {
				t.Errorf("HoldTime = %v, want %v (a non-zero Hold Time is used as sent)", adj.HoldTime, want)
			}
			if adj.HoldTime == defaultHoldTime(tt.targeted) {
				t.Errorf("HoldTime was replaced by the default %v", defaultHoldTime(tt.targeted))
			}
		})
	}
}
