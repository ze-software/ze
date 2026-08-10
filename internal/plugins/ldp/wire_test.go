package ldp

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

// VALIDATES: AC-4 -- the LDP Address message (RFC 5036 Section 3.5.5) decodes to
// the peer's interface address list, used for next-hop resolution.
func TestDecodeAddressList(t *testing.T) {
	val := binary.BigEndian.AppendUint16(nil, AFIIPv4)
	val = append(val, 10, 0, 0, 1, 192, 168, 1, 1)
	body := make([]byte, ldpTLVHdrLen+len(val))
	EncodeTLV(body, TLV{Type: TLVTypeAddressList, Length: uint16(len(val)), Value: val})

	m, err := decodeAddressList(7, body)
	if err != nil {
		t.Fatalf("DecodeAddressList: %v", err)
	}
	if m.Family != AFIIPv4 {
		t.Errorf("family = %d, want %d", m.Family, AFIIPv4)
	}
	want := []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("192.168.1.1")}
	if len(m.Addresses) != len(want) {
		t.Fatalf("got %d addresses, want %d", len(m.Addresses), len(want))
	}
	for i, a := range want {
		if m.Addresses[i] != a {
			t.Errorf("address[%d] = %s, want %s", i, m.Addresses[i], a)
		}
	}
}

// VALIDATES: a malformed (too short) Address List TLV value is rejected, not
// silently misparsed.
func TestDecodeAddressListShort(t *testing.T) {
	body := make([]byte, ldpTLVHdrLen+1)
	EncodeTLV(body, TLV{Type: TLVTypeAddressList, Length: 1, Value: []byte{0x00}})
	if _, err := decodeAddressList(1, body); err == nil {
		t.Error("expected error for truncated address list TLV")
	}
}

func TestLDPHelloEncode(t *testing.T) {
	h := HelloMessage{
		MessageID:     1,
		HoldTime:      15,
		Targeted:      false,
		RequestTarget: false,
		TransportAddr: netip.MustParseAddr("10.0.0.1"),
	}
	var buf [128]byte
	n := EncodeHello(buf[:], h)
	if n == 0 {
		t.Fatal("EncodeHello returned 0 bytes")
	}

	hdr, err := decodeMessageHeader(buf[:])
	if err != nil {
		t.Fatalf("DecodeMessageHeader: %v", err)
	}
	if hdr.Type != MsgTypeHello {
		t.Fatalf("type = %#x, want %#x", hdr.Type, MsgTypeHello)
	}
	if hdr.MessageID != 1 {
		t.Fatalf("message ID = %d, want 1", hdr.MessageID)
	}
}

func TestLDPHelloDecode(t *testing.T) {
	orig := HelloMessage{
		MessageID:     42,
		HoldTime:      30,
		Targeted:      true,
		RequestTarget: true,
		TransportAddr: netip.MustParseAddr("192.168.1.1"),
	}
	var buf [128]byte
	n := EncodeHello(buf[:], orig)

	hdr, err := decodeMessageHeader(buf[:n])
	if err != nil {
		t.Fatalf("DecodeMessageHeader: %v", err)
	}
	body := buf[ldpMsgHdrLen:n]

	decoded, err := DecodeHello(hdr.MessageID, body)
	if err != nil {
		t.Fatalf("DecodeHello: %v", err)
	}
	if decoded.MessageID != orig.MessageID {
		t.Errorf("MessageID = %d, want %d", decoded.MessageID, orig.MessageID)
	}
	if decoded.HoldTime != orig.HoldTime {
		t.Errorf("HoldTime = %d, want %d", decoded.HoldTime, orig.HoldTime)
	}
	if decoded.Targeted != orig.Targeted {
		t.Errorf("Targeted = %v, want %v", decoded.Targeted, orig.Targeted)
	}
	if decoded.RequestTarget != orig.RequestTarget {
		t.Errorf("RequestTarget = %v, want %v", decoded.RequestTarget, orig.RequestTarget)
	}
	if decoded.TransportAddr != orig.TransportAddr {
		t.Errorf("TransportAddr = %v, want %v", decoded.TransportAddr, orig.TransportAddr)
	}
}

func TestLDPHelloRoundTripIPv6(t *testing.T) {
	orig := HelloMessage{
		MessageID:     99,
		HoldTime:      45,
		Targeted:      true,
		TransportAddr: netip.MustParseAddr("2001:db8::1"),
	}
	var buf [128]byte
	n := EncodeHello(buf[:], orig)

	hdr, err := decodeMessageHeader(buf[:n])
	if err != nil {
		t.Fatalf("DecodeMessageHeader: %v", err)
	}
	decoded, err := DecodeHello(hdr.MessageID, buf[ldpMsgHdrLen:n])
	if err != nil {
		t.Fatalf("DecodeHello: %v", err)
	}
	if decoded.TransportAddr != orig.TransportAddr {
		t.Errorf("TransportAddr = %v, want %v", decoded.TransportAddr, orig.TransportAddr)
	}
}

func TestLDPInitEncode(t *testing.T) {
	m := initMessage{
		MessageID:          1,
		ProtocolVersion:    1,
		KeepaliveTime:      180,
		MaxPDULength:       4096,
		ReceiverLSRID:      [4]byte{10, 0, 0, 2},
		ReceiverLabelSpace: 0,
		LoopDetection:      false,
		PathVectorLimit:    0,
	}
	var buf [128]byte
	n := EncodeInit(buf[:], m)
	if n == 0 {
		t.Fatal("EncodeInit returned 0 bytes")
	}

	hdr, err := decodeMessageHeader(buf[:n])
	if err != nil {
		t.Fatalf("DecodeMessageHeader: %v", err)
	}
	if hdr.Type != MsgTypeInitialize {
		t.Fatalf("type = %#x, want %#x", hdr.Type, MsgTypeInitialize)
	}

	decoded, err := DecodeInit(hdr.MessageID, buf[ldpMsgHdrLen:n])
	if err != nil {
		t.Fatalf("DecodeInit: %v", err)
	}
	if decoded.ProtocolVersion != 1 {
		t.Errorf("ProtocolVersion = %d, want 1", decoded.ProtocolVersion)
	}
	if decoded.KeepaliveTime != 180 {
		t.Errorf("KeepaliveTime = %d, want 180", decoded.KeepaliveTime)
	}
	if decoded.MaxPDULength != 4096 {
		t.Errorf("MaxPDULength = %d, want 4096", decoded.MaxPDULength)
	}
	if decoded.ReceiverLSRID != m.ReceiverLSRID {
		t.Errorf("ReceiverLSRID = %v, want %v", decoded.ReceiverLSRID, m.ReceiverLSRID)
	}
}

func TestLDPInitRoundTripLoopDetection(t *testing.T) {
	m := initMessage{
		MessageID:       2,
		ProtocolVersion: 1,
		KeepaliveTime:   90,
		MaxPDULength:    256,
		ReceiverLSRID:   [4]byte{172, 16, 0, 1},
		LoopDetection:   true,
		PathVectorLimit: 10,
	}
	var buf [128]byte
	n := EncodeInit(buf[:], m)

	hdr, err := decodeMessageHeader(buf[:n])
	if err != nil {
		t.Fatalf("DecodeMessageHeader: %v", err)
	}
	decoded, err := DecodeInit(hdr.MessageID, buf[ldpMsgHdrLen:n])
	if err != nil {
		t.Fatalf("DecodeInit: %v", err)
	}
	if !decoded.LoopDetection {
		t.Error("LoopDetection = false, want true")
	}
	if decoded.PathVectorLimit != 10 {
		t.Errorf("PathVectorLimit = %d, want 10", decoded.PathVectorLimit)
	}
}

func TestLDPLabelMapEncode(t *testing.T) {
	m := labelMappingMessage{
		MessageID: 5,
		FEC: FECElement{
			Type:   FECPrefix,
			Family: AFIIPv4,
			Prefix: netip.MustParsePrefix("10.0.0.0/24"),
		},
		Label: 1000,
	}
	var buf [128]byte
	n := encodeLabelMapping(buf[:], m)
	if n == 0 {
		t.Fatal("EncodeLabelMapping returned 0 bytes")
	}

	hdr, err := decodeMessageHeader(buf[:n])
	if err != nil {
		t.Fatalf("DecodeMessageHeader: %v", err)
	}
	if hdr.Type != MsgTypeLabelMapping {
		t.Fatalf("type = %#x, want %#x", hdr.Type, MsgTypeLabelMapping)
	}

	decoded, err := decodeLabelMapping(hdr.MessageID, buf[ldpMsgHdrLen:n])
	if err != nil {
		t.Fatalf("DecodeLabelMapping: %v", err)
	}
	if decoded.Label != 1000 {
		t.Errorf("Label = %d, want 1000", decoded.Label)
	}
	if decoded.FEC.Prefix != m.FEC.Prefix {
		t.Errorf("FEC.Prefix = %v, want %v", decoded.FEC.Prefix, m.FEC.Prefix)
	}
}

func TestLDPLabelMapIPv6(t *testing.T) {
	m := labelMappingMessage{
		MessageID: 6,
		FEC: FECElement{
			Type:   FECPrefix,
			Family: AFIIPv6,
			Prefix: netip.MustParsePrefix("2001:db8::/32"),
		},
		Label: 2000,
	}
	var buf [256]byte
	n := encodeLabelMapping(buf[:], m)

	hdr, err := decodeMessageHeader(buf[:n])
	if err != nil {
		t.Fatalf("DecodeMessageHeader: %v", err)
	}
	decoded, err := decodeLabelMapping(hdr.MessageID, buf[ldpMsgHdrLen:n])
	if err != nil {
		t.Fatalf("DecodeLabelMapping: %v", err)
	}
	if decoded.Label != 2000 {
		t.Errorf("Label = %d, want 2000", decoded.Label)
	}
	if decoded.FEC.Prefix != m.FEC.Prefix {
		t.Errorf("FEC.Prefix = %v, want %v", decoded.FEC.Prefix, m.FEC.Prefix)
	}
}

func TestLDPPDUHeaderRoundTrip(t *testing.T) {
	orig := PDUHeader{
		Version:    1,
		PDULength:  100,
		LSRID:      [4]byte{10, 0, 0, 1},
		LabelSpace: 0,
	}
	var buf [16]byte
	n := encodePDUHeader(buf[:], orig)
	if n != ldpHeaderLen {
		t.Fatalf("EncodePDUHeader returned %d, want %d", n, ldpHeaderLen)
	}

	decoded, err := decodePDUHeader(buf[:n])
	if err != nil {
		t.Fatalf("DecodePDUHeader: %v", err)
	}
	if decoded != orig {
		t.Errorf("decoded = %+v, want %+v", decoded, orig)
	}
}

func TestLDPPDUHeaderBadVersion(t *testing.T) {
	var buf [16]byte
	encodePDUHeader(buf[:], PDUHeader{Version: 2, PDULength: 10, LSRID: [4]byte{1, 2, 3, 4}})
	_, err := decodePDUHeader(buf[:])
	if err == nil {
		t.Fatal("expected error for version 2, got nil")
	}
}

func TestLDPPDUHeaderShort(t *testing.T) {
	_, err := decodePDUHeader([]byte{0, 1, 0})
	if err == nil {
		t.Fatal("expected error for short buffer, got nil")
	}
}

func TestLDPLabelValidation(t *testing.T) {
	if err := ValidateLabel(0); err != nil {
		t.Errorf("label 0: %v", err)
	}
	if err := ValidateLabel(MaxLabel); err != nil {
		t.Errorf("label %d: %v", MaxLabel, err)
	}
	if err := ValidateLabel(MaxLabel + 1); err == nil {
		t.Errorf("label %d: expected error", MaxLabel+1)
	}
}

func TestLDPLabelWithdrawEncode(t *testing.T) {
	m := labelWithdrawMessage{
		MessageID: 10,
		FEC: FECElement{
			Type:   FECPrefix,
			Family: AFIIPv4,
			Prefix: netip.MustParsePrefix("10.1.0.0/16"),
		},
		Label:    500,
		HasLabel: true,
	}
	var buf [128]byte
	n := EncodeLabelWithdraw(buf[:], m)
	if n == 0 {
		t.Fatal("EncodeLabelWithdraw returned 0 bytes")
	}

	hdr, err := decodeMessageHeader(buf[:n])
	if err != nil {
		t.Fatalf("DecodeMessageHeader: %v", err)
	}
	if hdr.Type != MsgTypeLabelWithdraw {
		t.Fatalf("type = %#x, want %#x", hdr.Type, MsgTypeLabelWithdraw)
	}
}

func TestLDPTLVShort(t *testing.T) {
	_, _, err := DecodeTLV([]byte{0, 1})
	if err == nil {
		t.Fatal("expected error for short TLV, got nil")
	}
}

func TestLDPKeepaliveEncode(t *testing.T) {
	m := keepaliveMessage{MessageID: 7}
	var buf [16]byte
	n := encodeKeepalive(buf[:], m)
	if n != ldpMsgHdrLen {
		t.Fatalf("EncodeKeepalive returned %d, want %d", n, ldpMsgHdrLen)
	}

	hdr, err := decodeMessageHeader(buf[:n])
	if err != nil {
		t.Fatalf("DecodeMessageHeader: %v", err)
	}
	if hdr.Type != MsgTypeKeepAlive {
		t.Fatalf("type = %#x, want %#x", hdr.Type, MsgTypeKeepAlive)
	}
	if hdr.MessageID != 7 {
		t.Fatalf("MessageID = %d, want 7", hdr.MessageID)
	}
}
