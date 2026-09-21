// RFC: rfc/short/rfc5036.md -- gated MUST coverage for the session parameters and
// the Hello transport address
// Design: docs/architecture/ldp/mpls-ldp.md -- LDP plugin
// Related: rfc5036_test.go -- the helpers these tests share (rfcTestSession,
// readLDPPDU, runSessionForTest, readNotificationStatus)
//
// VALIDATES: the Common Session Parameters ze proposes and negotiates (RFC 5036
// Section 3.5.3): the A bit is Downstream Unsolicited, PVLim is 0 beside D=0, the
// reserved bits are zero on transmission and ignored on receipt, and the Max PDU
// Length is the smaller of the two proposals with 255-or-less read as the default.
// The required-parameter order of Section 3.5, the KeepAlive pacing of Section
// 3.5.4.1, and the one transport address every Hello carries (Section 2.5.2).
// PREVENTS: the D bit read at 0x04 where the RFC puts it at 0x40, a Max PDU proposal
// of 100 taken as a literal 100-octet limit, and a Hello whose transport address
// follows the sending socket rather than the configured one.
package ldp

import (
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// encodeInitPDURaw builds a complete Initialization PDU whose Common Session
// Parameters carry the flags octet, the PVLim octet and the Max PDU Length exactly
// as given, so a test can set bits the encoder never sets.
func encodeInitPDURaw(flags, pvlim uint8, keepalive, maxPDU uint16) []byte {
	var value [14]byte
	binary.BigEndian.PutUint16(value[0:2], ldpVersion)
	binary.BigEndian.PutUint16(value[2:4], keepalive)
	value[4] = flags
	value[5] = pvlim
	binary.BigEndian.PutUint16(value[6:8], maxPDU)
	copy(value[8:12], []byte{10, 0, 0, 1})
	var buf [128]byte
	off := ldpHeaderLen
	off += encodeMessageHeader(buf[off:], MessageHeader{Type: MsgTypeInitialize, MessageID: 7})
	off += EncodeTLV(buf[off:], TLV{Type: TLVTypeCommonSession, Length: 14, Value: value[:]})
	bodyLen := off - ldpHeaderLen
	binary.BigEndian.PutUint16(buf[ldpHeaderLen+2:ldpHeaderLen+4], uint16(bodyLen-ldpTLVHdrLen))
	encodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  uint16(bodyLen + 6),
		LSRID:      [4]byte{10, 0, 0, 2},
		LabelSpace: 0,
	})
	return buf[:off]
}

// readSentInit runs SendInit on sess and returns the Common Session Parameters
// value the peer reads: the 14 octets after the TLV header.
func readSentInit(t *testing.T, sess *Session, remote net.Conn) []byte {
	t.Helper()
	errCh := make(chan error, 1)
	go func() { errCh <- sess.SendInit() }()
	_, msgHdr, body := readLDPPDU(t, remote)
	if err := <-errCh; err != nil {
		t.Fatalf("SendInit: %v", err)
	}
	if msgHdr.Type != MsgTypeInitialize {
		t.Fatalf("message type = %#x, want Initialization", msgHdr.Type)
	}
	tlv, _, err := DecodeTLV(body)
	if err != nil {
		t.Fatalf("DecodeTLV: %v", err)
	}
	if tlv.Type != TLVTypeCommonSession {
		t.Fatalf("first TLV = %#x, want Common Session Parameters", tlv.Type)
	}
	if len(tlv.Value) != 14 {
		t.Fatalf("Common Session Parameters length = %d, want 14", len(tlv.Value))
	}
	return tlv.Value
}

// expectNoPDU fails when anything arrives on conn inside a short window.
func expectNoPDU(t *testing.T, conn net.Conn, what string) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var probe [1]byte
	if _, err := conn.Read(probe[:]); err == nil {
		t.Errorf("%s", what)
	}
}

// --------------------------------------------------------------------------
// RFC5036-3.5.3-3 -- Downstream Unsolicited is the discipline off ATM and FR
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-3.5.3-3 positive -- the Initialization ze sends proposes
// Downstream Unsolicited: the A bit of its Common Session Parameters is 0.
func TestRFC5036InitProposesDownstreamUnsolicited(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	value := readSentInit(t, rfcTestSession(local), remote)
	if value[4]&initFlagOnDemand != 0 {
		t.Errorf("flags octet = %#02x: the A bit is set, ze proposed Downstream On Demand", value[4])
	}
	msg, err := DecodeInit(7, append([]byte{0x05, 0x00, 0x00, 0x0e}, value...))
	if err != nil {
		t.Fatalf("DecodeInit: %v", err)
	}
	if msg.OnDemand {
		t.Error("DecodeInit reads ze's own Initialization as Downstream On Demand")
	}
}

// RFC requirement: RFC5036-3.5.3-3 negative -- a peer proposing Downstream On Demand
// (A=1) on a session that is not on an ATM or Frame Relay link does not move ze off
// Downstream Unsolicited: the session goes operational with no Notification, and the
// Label Mapping ze then sends goes out unsolicited.
func TestRFC5036InitOnDemandProposalKeepsDownstreamUnsolicited(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	rx := rfcTestSession(local)
	rx.state = StateOpenSent

	pdu := encodeInitPDURaw(initFlagOnDemand, 0, 30, 4096)
	msg, err := DecodeInit(7, pdu[ldpHeaderLen+ldpMsgHdrLen:])
	if err != nil {
		t.Fatalf("DecodeInit: %v", err)
	}
	if !msg.OnDemand {
		t.Fatal("the test PDU does not carry A=1: DecodeInit did not read the A bit")
	}
	if err := rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, nil, nil, nil); err != nil {
		t.Fatalf("processMessages: %v", err)
	}
	if rx.State() != StateOperational {
		t.Fatalf("state = %s, want operational: Downstream Unsolicited MUST be used, not refused", rx.State())
	}
	expectNoPDU(t, remote, "a Notification was sent for a Downstream On Demand proposal")

	errCh := make(chan error, 1)
	go func() { errCh <- rx.SendLabelMapping(netip.MustParsePrefix("10.1.0.0/24"), 100) }()
	_, msgHdr, _ := readLDPPDU(t, remote)
	if err := <-errCh; err != nil {
		t.Fatalf("SendLabelMapping: %v", err)
	}
	if msgHdr.Type != MsgTypeLabelMapping {
		t.Errorf("message after the exchange = %#x, want an unsolicited Label Mapping", msgHdr.Type)
	}
}

// --------------------------------------------------------------------------
// RFC5036-3.5.3-5 -- PVLim is 0 when Loop Detection is disabled
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-3.5.3-5 positive -- the Initialization ze sends carries
// D=0 and a Path Vector Limit of 0.
func TestRFC5036InitPathVectorLimitZeroWithoutLoopDetection(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	value := readSentInit(t, rfcTestSession(local), remote)
	if value[4]&initFlagLoopDetection != 0 {
		t.Errorf("flags octet = %#02x: D is set, ze does not run Loop Detection", value[4])
	}
	if value[5] != 0 {
		t.Errorf("PVLim = %d beside D=0, want 0", value[5])
	}
}

// RFC requirement: RFC5036-3.5.3-5 negative -- EncodeInit never writes a non-zero
// Path Vector Limit beside D=0: given a limit and Loop Detection disabled it writes
// 0, and the same limit beside D=1 is what reaches the wire.
func TestRFC5036EncodeInitDropsPathVectorLimitWithoutLoopDetection(t *testing.T) {
	encode := func(loop bool) []byte {
		var buf [64]byte
		n := EncodeInit(buf[:], initMessage{
			MessageID:       1,
			ProtocolVersion: ldpVersion,
			KeepaliveTime:   60,
			MaxPDULength:    4096,
			LoopDetection:   loop,
			PathVectorLimit: 7,
		})
		return buf[ldpMsgHdrLen+ldpTLVHdrLen : n]
	}
	off := encode(false)
	if off[4]&initFlagLoopDetection != 0 {
		t.Errorf("D=1 written with Loop Detection disabled")
	}
	if off[5] != 0 {
		t.Errorf("PVLim = %d beside D=0, want 0: the limit MUST be 0 when Loop Detection is disabled", off[5])
	}
	on := encode(true)
	if on[4]&initFlagLoopDetection == 0 {
		t.Errorf("D=0 written with Loop Detection enabled")
	}
	if on[5] != 7 {
		t.Errorf("PVLim = %d beside D=1, want 7: the guard dropped a limit it must keep", on[5])
	}
}

// --------------------------------------------------------------------------
// RFC5036-3.5.3-6 -- the reserved bits are zero on transmission, ignored on receipt
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-3.5.3-6 positive -- the six reserved bits of the Common
// Session Parameters flags octet are zero in the Initialization ze sends.
func TestRFC5036InitReservedBitsZeroOnTransmission(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	value := readSentInit(t, rfcTestSession(local), remote)
	const reservedMask = ^(initFlagOnDemand | initFlagLoopDetection)
	if value[4]&reservedMask != 0 {
		t.Errorf("flags octet = %#02x: reserved bits set on transmission", value[4])
	}
}

// RFC requirement: RFC5036-3.5.3-6 negative -- an Initialization whose six reserved
// bits are all set is not refused and not misread: the session goes operational with
// no Notification, the KeepAlive Time it carries is negotiated, and neither A nor D is
// read from the reserved bits.
func TestRFC5036InitReservedBitsIgnoredOnReceipt(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	const reservedBits = ^(initFlagOnDemand | initFlagLoopDetection)
	pdu := encodeInitPDURaw(reservedBits, 0, 30, 4096)
	msg, err := DecodeInit(7, pdu[ldpHeaderLen+ldpMsgHdrLen:])
	if err != nil {
		t.Fatalf("DecodeInit: %v", err)
	}
	if msg.OnDemand || msg.LoopDetection {
		t.Errorf("reserved bits read as A=%v D=%v, want both false", msg.OnDemand, msg.LoopDetection)
	}

	rx := rfcTestSession(local)
	rx.state = StateOpenSent
	if err := rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, nil, nil, nil); err != nil {
		t.Fatalf("processMessages: %v", err)
	}
	if rx.State() != StateOperational {
		t.Errorf("state = %s, want operational: reserved bits MUST be ignored on receipt", rx.State())
	}
	if got := rx.currentKeepalive(); got != 30*time.Second {
		t.Errorf("keepalive = %v, want 30s: the reserved bits disturbed the negotiation", got)
	}
	expectNoPDU(t, remote, "a Notification was sent for an Initialization with reserved bits set")
}

// --------------------------------------------------------------------------
// RFC5036-3.5.3-7 -- Max PDU Length is the smaller of the two proposals
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-3.5.3-7 positive -- the session's maximum PDU length is
// the smaller of ze's 4096 and the peer's proposal: 1000 wins over 4096, and a
// proposal of 255 or less means the 4096 default, so it does not win over 4096.
func TestRFC5036InitMaxPDULengthTakesTheSmallerProposal(t *testing.T) {
	cases := []struct {
		name string
		peer uint16
		want uint16
	}{
		{"peer proposes 1000", 1000, 1000},
		{"peer proposes 255, meaning the 4096 default", 255, DefaultMaxPDULength},
		{"peer proposes 100, meaning the 4096 default", 100, DefaultMaxPDULength},
		{"peer proposes 0, meaning the 4096 default", 0, DefaultMaxPDULength},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			local, remote := net.Pipe()
			defer func() { _ = local.Close() }()
			defer func() { _ = remote.Close() }()

			rx := rfcTestSession(local)
			rx.state = StateOpenSent
			pdu := encodeInitPDURaw(0, 0, 30, tc.peer)
			if err := rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, nil, nil, nil); err != nil {
				t.Fatalf("processMessages: %v", err)
			}
			rx.mu.Lock()
			got := rx.maxPDU
			rx.mu.Unlock()
			if got != tc.want {
				t.Errorf("max PDU length = %d, want %d", got, tc.want)
			}
		})
	}
}

// RFC requirement: RFC5036-3.5.3-7 negative -- a peer proposal LARGER than ze's own
// never raises the session's maximum PDU length: 8000 against 4096 leaves 4096.
func TestRFC5036InitMaxPDULengthNeverRaisedAboveOwnProposal(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	rx := rfcTestSession(local)
	rx.state = StateOpenSent
	pdu := encodeInitPDURaw(0, 0, 30, 8000)
	if err := rx.processMessages(pdu[ldpHeaderLen:], [4]byte{10, 0, 0, 2}, nil, nil, nil); err != nil {
		t.Fatalf("processMessages: %v", err)
	}
	rx.mu.Lock()
	got := rx.maxPDU
	rx.mu.Unlock()
	if got != DefaultMaxPDULength {
		t.Errorf("max PDU length = %d, want %d: the larger proposal won", got, DefaultMaxPDULength)
	}
}

// --------------------------------------------------------------------------
// RFC5036-3.5-1 -- required parameters come first, in the specified order
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-3.5-1 positive -- in every message ze encodes the required
// parameters lead in the order their section gives: the Common Session Parameters
// TLV first in an Initialization (Section 3.5.3), the Common Hello Parameters TLV
// first in a Hello (Section 3.5.2), and the FEC TLV then the Label TLV in a Label
// Mapping (Section 3.5.7).
func TestRFC5036RequiredParametersInSpecifiedOrder(t *testing.T) {
	tlvTypes := func(body []byte) []uint16 {
		var types []uint16
		for off := 0; off < len(body); {
			tlv, n, err := DecodeTLV(body[off:])
			if err != nil {
				t.Fatalf("DecodeTLV at %d: %v", off, err)
			}
			types = append(types, tlv.Type)
			off += n
		}
		return types
	}
	var buf [256]byte

	n := EncodeInit(buf[:], initMessage{MessageID: 1, ProtocolVersion: ldpVersion, KeepaliveTime: 60, MaxPDULength: 4096})
	if got := tlvTypes(buf[ldpMsgHdrLen:n]); len(got) == 0 || got[0] != TLVTypeCommonSession {
		t.Errorf("Initialization TLV order = %#x, want Common Session Parameters first", got)
	}

	n = EncodeHello(buf[:], HelloMessage{MessageID: 1, HoldTime: 15, TransportAddr: netip.MustParseAddr("10.0.0.1")})
	if got := tlvTypes(buf[ldpMsgHdrLen:n]); len(got) == 0 || got[0] != TLVTypeCommonHello {
		t.Errorf("Hello TLV order = %#x, want Common Hello Parameters first", got)
	}

	n = encodeLabelMapping(buf[:], labelMappingMessage{
		MessageID: 1,
		FEC:       FECElement{Prefix: netip.MustParsePrefix("10.1.0.0/24")},
		Label:     100,
	})
	got := tlvTypes(buf[ldpMsgHdrLen:n])
	if len(got) < 2 || got[0] != TLVTypeFEC || got[1] != TLVTypeGenericLabel {
		t.Errorf("Label Mapping TLV order = %#x, want FEC then Label", got)
	}
}

// --------------------------------------------------------------------------
// RFC5036-3.5.4.1-1 -- the peer hears from ze at least every KeepAlive Time
// --------------------------------------------------------------------------

// RFC requirement: RFC5036-3.5.4.1-1 positive -- on an established session the gap
// between two consecutive PDUs ze sends never exceeds the negotiated KeepAlive Time.
func TestRFC5036PeerHearsFromZeWithinKeepaliveTime(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	const keepalive = 300 * time.Millisecond
	sess := rfcTestSession(local)
	sess.keepaliveTime = keepalive
	stop := runSessionForTest(t, sess)
	defer stop()

	_, msgHdr, _ := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeInitialize {
		t.Fatalf("first message = %#x, want Initialization", msgHdr.Type)
	}
	last := time.Now()
	for i := range 4 {
		readLDPPDU(t, remote)
		now := time.Now()
		if gap := now.Sub(last); gap > keepalive {
			t.Errorf("gap before PDU %d = %v, want at most the KeepAlive Time %v", i, gap, keepalive)
		}
		last = now
	}
}

// RFC requirement: RFC5036-3.5.4.1-1 negative -- the pacing follows the KeepAlive
// Time the exchange NEGOTIATED, not the one ze proposed: after the peer lowers a
// 60s proposal to 1s, no gap between ze's PDUs exceeds 1s.
func TestRFC5036KeepalivePacingFollowsNegotiatedTime(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	sess.keepaliveTime = 60 * time.Second
	stop := runSessionForTest(t, sess)
	defer stop()

	_, msgHdr, _ := readLDPPDU(t, remote)
	if msgHdr.Type != MsgTypeInitialize {
		t.Fatalf("first message = %#x, want Initialization", msgHdr.Type)
	}
	// The session-establishment KeepAlive that follows the Initialization.
	readLDPPDU(t, remote)

	// The peer's Initialization proposes 1s; the negotiation takes the smaller.
	pdu := encodeInitPDURaw(0, 0, 1, 4096)
	if _, err := remote.Write(pdu); err != nil {
		t.Fatalf("write Initialization: %v", err)
	}
	last := time.Now()

	const negotiated = time.Second
	for i := range 3 {
		readLDPPDU(t, remote)
		now := time.Now()
		if gap := now.Sub(last); gap > negotiated {
			t.Errorf("gap before PDU %d = %v, want at most the negotiated KeepAlive Time %v", i, gap, negotiated)
		}
		last = now
	}
}

// --------------------------------------------------------------------------
// RFC5036-2.5.2-1 -- one transport address in every Hello for a label space
// --------------------------------------------------------------------------

// helloTransportAddr sends one Hello from src to dst through sendHello and returns
// the transport address the Hello carries.
func helloTransportAddr(t *testing.T, src, dst *net.UDPConn, cfg ldpConfig) netip.Addr {
	t.Helper()
	sendHello(src, dst.LocalAddr().(*net.UDPAddr), [4]byte{10, 0, 0, 1}, cfg, slogutil.DiscardLogger())
	if err := dst.SetReadDeadline(time.Now().Add(ldpReadTimeout)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var buf [256]byte
	n, _, err := dst.ReadFromUDP(buf[:])
	if err != nil {
		t.Fatalf("read Hello: %v", err)
	}
	if n < ldpHeaderLen+ldpMsgHdrLen {
		t.Fatalf("Hello is %d octets, shorter than its headers", n)
	}
	hello, err := DecodeHello(1, buf[ldpHeaderLen+ldpMsgHdrLen:n])
	if err != nil {
		t.Fatalf("DecodeHello: %v", err)
	}
	return hello.TransportAddr
}

// listenLoopbackUDP binds a UDP socket on the given loopback address.
func listenLoopbackUDP(t *testing.T, ip string) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(ip)})
	if err != nil {
		t.Skipf("cannot bind %s: %v", ip, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// RFC requirement: RFC5036-2.5.2-1 positive -- every Hello ze sends for label space
// 0 carries the one configured transport address, whichever socket sends it.
func TestRFC5036HellosCarryOneTransportAddress(t *testing.T) {
	cfg := ldpConfig{HelloHoldTime: DefaultHelloHoldTime, TransportAddr: netip.MustParseAddr("10.0.0.1")}
	dst := listenLoopbackUDP(t, "127.0.0.1")
	first := listenLoopbackUDP(t, "127.0.0.1")
	second := listenLoopbackUDP(t, "127.0.0.2")

	a := helloTransportAddr(t, first, dst, cfg)
	b := helloTransportAddr(t, second, dst, cfg)
	if a != cfg.TransportAddr {
		t.Errorf("first Hello transport address = %v, want %v", a, cfg.TransportAddr)
	}
	if b != cfg.TransportAddr {
		t.Errorf("second Hello transport address = %v, want %v", b, cfg.TransportAddr)
	}
}

// RFC requirement: RFC5036-2.5.2-1 negative -- a Hello never advertises the address
// of the socket that sends it: two Hellos sent from 127.0.0.1 and 127.0.0.2 for the
// same label space carry neither source, only the configured transport address.
func TestRFC5036HelloTransportAddressNeverFollowsTheSocket(t *testing.T) {
	cfg := ldpConfig{HelloHoldTime: DefaultHelloHoldTime, TransportAddr: netip.MustParseAddr("10.0.0.1")}
	dst := listenLoopbackUDP(t, "127.0.0.1")
	first := listenLoopbackUDP(t, "127.0.0.1")
	second := listenLoopbackUDP(t, "127.0.0.2")

	a := helloTransportAddr(t, first, dst, cfg)
	b := helloTransportAddr(t, second, dst, cfg)
	if a != b {
		t.Errorf("Hellos for one label space advertise two transport addresses: %v and %v", a, b)
	}
	for _, src := range []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("127.0.0.2")} {
		if a == src || b == src {
			t.Errorf("a Hello advertised its socket's address %v as the transport address", src)
		}
	}
}
