// RSVP-TE wire codec round-trips and RFC 2205 common-header/object obligations.
//
// VALIDATES: the wire.go/build.go encoders and DecodeMessage against RFC 2205 --
// the common header carries Version 1 (Section 3.1), the reserved octet is zero
// on send (Section 3.1), and every emitted object length is a multiple of 4
// (Section 3.1.2) -- plus per-object encode/decode round-trips and the ERO/RRO
// bounds guards.
// PREVENTS: encoding a header with the wrong version or a nonzero reserved octet,
// accepting a non-version-1 header on decode, and emitting an object whose length
// is not a multiple of 4 (which would desynchronise a conformant peer's object walk).
//
// Producers exercised: encodeHeader / DecodeHeader (wire.go), the per-object
// encoders (wire.go), and buildPath / buildResv / buildPathErr (build.go).

package rsvpte

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

func TestRSVPPathEncode(t *testing.T) {
	buf := make([]byte, 512)
	off := 0

	session := sessionIPv4{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.2"),
		TunnelID:       100,
		ExtTunnelID:    0x0a000001,
	}
	sender := senderTemplateIPv4{
		SenderAddr: netip.MustParseAddr("10.0.0.1"),
		LSPID:      1,
	}
	hop := rsvpHop{
		NextHop: netip.MustParseAddr("10.0.0.1"),
		LIH:     0,
	}
	tv := timeValues{RefreshPeriod: 30000}
	lr := labelRequest{L3PID: 0x0800}
	ero := []eroHop{
		{Loose: false, Address: netip.MustParsePrefix("10.0.0.2/32")},
		{Loose: true, Address: netip.MustParsePrefix("10.0.0.3/32")},
	}
	tspec := FlowSpec{
		TokenRate:      1e9,
		TokenBucket:    1e9,
		PeakRate:       1e9,
		MinPolicedUnit: 64,
		MaxPacketSize:  1500,
	}

	off += rsvpHdrLen
	off += encodeSessionIPv4(buf[off:], session)
	off += encodeSenderTemplate(buf[off:], sender)
	off += encodeRSVPHop(buf[off:], hop)
	off += encodeTimeValues(buf[off:], tv)
	off += encodeLabelRequest(buf[off:], lr)
	off += encodeERO(buf[off:], ero)
	off += encodeFlowSpec(buf[off:], ClassSenderTSpec, tspec)

	hdr := Header{
		Version: rsvpVersion,
		MsgType: MsgTypePath,
		TTL:     64,
		Length:  uint16(off),
	}
	encodeHeader(buf, hdr)

	if off < rsvpHdrLen+16 {
		t.Fatalf("PATH message too short: %d bytes", off)
	}
	if hdr.Length != uint16(off) {
		t.Fatalf("header length mismatch: %d vs %d", hdr.Length, off)
	}
}

func TestRSVPPathDecode(t *testing.T) {
	buf := make([]byte, 512)
	off := 0

	session := sessionIPv4{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.2"),
		TunnelID:       100,
		ExtTunnelID:    0x0a000001,
	}
	sender := senderTemplateIPv4{
		SenderAddr: netip.MustParseAddr("10.0.0.1"),
		LSPID:      1,
	}
	hop := rsvpHop{
		NextHop: netip.MustParseAddr("10.0.0.1"),
		LIH:     42,
	}
	tv := timeValues{RefreshPeriod: 30000}
	lr := labelRequest{L3PID: 0x0800}

	off += rsvpHdrLen
	off += encodeSessionIPv4(buf[off:], session)
	off += encodeSenderTemplate(buf[off:], sender)
	off += encodeRSVPHop(buf[off:], hop)
	off += encodeTimeValues(buf[off:], tv)
	off += encodeLabelRequest(buf[off:], lr)

	hdr := Header{
		Version: rsvpVersion,
		MsgType: MsgTypePath,
		TTL:     64,
		Length:  uint16(off),
	}
	encodeHeader(buf, hdr)

	msg, err := DecodeMessage(buf[:off])
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}

	if msg.Header.MsgType != MsgTypePath {
		t.Errorf("MsgType = %d, want %d", msg.Header.MsgType, MsgTypePath)
	}
	if !msg.HasSession {
		t.Fatal("missing SESSION object")
	}
	if msg.Session.TunnelEndpoint != session.TunnelEndpoint {
		t.Errorf("TunnelEndpoint = %s, want %s", msg.Session.TunnelEndpoint, session.TunnelEndpoint)
	}
	if msg.Session.TunnelID != session.TunnelID {
		t.Errorf("TunnelID = %d, want %d", msg.Session.TunnelID, session.TunnelID)
	}
	if !msg.HasSenderTemplate {
		t.Fatal("missing SENDER_TEMPLATE object")
	}
	if msg.SenderTemplate.SenderAddr != sender.SenderAddr {
		t.Errorf("SenderAddr = %s, want %s", msg.SenderTemplate.SenderAddr, sender.SenderAddr)
	}
	if msg.SenderTemplate.LSPID != sender.LSPID {
		t.Errorf("LSPID = %d, want %d", msg.SenderTemplate.LSPID, sender.LSPID)
	}
	if !msg.HasHop {
		t.Fatal("missing RSVP_HOP object")
	}
	if msg.Hop.NextHop != hop.NextHop {
		t.Errorf("NextHop = %s, want %s", msg.Hop.NextHop, hop.NextHop)
	}
	if msg.Hop.LIH != hop.LIH {
		t.Errorf("LIH = %d, want %d", msg.Hop.LIH, hop.LIH)
	}
	if !msg.HasTimeValues {
		t.Fatal("missing TIME_VALUES object")
	}
	if msg.TimeValues.RefreshPeriod != tv.RefreshPeriod {
		t.Errorf("RefreshPeriod = %d, want %d", msg.TimeValues.RefreshPeriod, tv.RefreshPeriod)
	}
	if !msg.HasLabelRequest {
		t.Fatal("missing LABEL_REQUEST object")
	}
	if msg.LabelRequest.L3PID != lr.L3PID {
		t.Errorf("L3PID = 0x%04x, want 0x%04x", msg.LabelRequest.L3PID, lr.L3PID)
	}
}

func TestRSVPResvEncode(t *testing.T) {
	buf := make([]byte, 512)
	off := 0

	session := sessionIPv4{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.2"),
		TunnelID:       100,
		ExtTunnelID:    0x0a000001,
	}
	hop := rsvpHop{
		NextHop: netip.MustParseAddr("10.0.0.2"),
		LIH:     0,
	}
	tv := timeValues{RefreshPeriod: 30000}
	label := labelObject{Label: 1000}
	fs := FlowSpec{
		TokenRate:      1e9,
		TokenBucket:    1e9,
		PeakRate:       1e9,
		MinPolicedUnit: 64,
		MaxPacketSize:  1500,
	}

	off += rsvpHdrLen
	off += encodeSessionIPv4(buf[off:], session)
	off += encodeRSVPHop(buf[off:], hop)
	off += encodeTimeValues(buf[off:], tv)
	off += encodeStyle(buf[off:], StyleSharedExplicit)
	off += encodeFlowSpec(buf[off:], ClassFlowSpec, fs)
	off += encodeLabelObject(buf[off:], label)

	hdr := Header{
		Version: rsvpVersion,
		MsgType: MsgTypeResv,
		TTL:     64,
		Length:  uint16(off),
	}
	encodeHeader(buf, hdr)

	msg, err := DecodeMessage(buf[:off])
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}

	if msg.Header.MsgType != MsgTypeResv {
		t.Errorf("MsgType = %d, want %d", msg.Header.MsgType, MsgTypeResv)
	}
	if !msg.HasLabel {
		t.Fatal("missing LABEL object")
	}
	if msg.Label.Label != 1000 {
		t.Errorf("Label = %d, want 1000", msg.Label.Label)
	}
	if !msg.HasStyle {
		t.Fatal("missing STYLE object")
	}
	if msg.Style != StyleSharedExplicit {
		t.Errorf("Style = %d, want %d", msg.Style, StyleSharedExplicit)
	}
	if !msg.HasFlowSpec {
		t.Fatal("missing FLOWSPEC object")
	}
}

func TestRSVPEROEncode(t *testing.T) {
	hops := []eroHop{
		{Loose: false, Address: netip.MustParsePrefix("10.0.0.1/32")},
		{Loose: true, Address: netip.MustParsePrefix("10.0.0.2/32")},
		{Loose: false, Address: netip.MustParsePrefix("10.0.0.3/32")},
	}

	buf := make([]byte, 256)
	n := encodeERO(buf, hops)

	decoded, err := decodeERO(buf[objHdrLen:n])
	if err != nil {
		t.Fatalf("DecodeERO: %v", err)
	}
	if len(decoded) != len(hops) {
		t.Fatalf("got %d hops, want %d", len(decoded), len(hops))
	}
	for i, h := range decoded {
		if h.Loose != hops[i].Loose {
			t.Errorf("hop[%d] Loose = %v, want %v", i, h.Loose, hops[i].Loose)
		}
		if h.Address != hops[i].Address {
			t.Errorf("hop[%d] Address = %s, want %s", i, h.Address, hops[i].Address)
		}
	}
}

func TestRSVPRRODecode(t *testing.T) {
	entries := []rroEntry{
		{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.1"), Flags: 0x01},
		{Type: RROSubLabel, Label: 1000, Flags: 0x01},
		{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.2"), Flags: 0x00},
	}

	buf := make([]byte, 256)
	n := encodeRRO(buf, entries)

	decoded, err := decodeRRO(buf[objHdrLen:n])
	if err != nil {
		t.Fatalf("DecodeRRO: %v", err)
	}
	if len(decoded) != len(entries) {
		t.Fatalf("got %d entries, want %d", len(decoded), len(entries))
	}

	if decoded[0].Type != RROSubIPv4 {
		t.Errorf("entry[0] Type = %d, want %d", decoded[0].Type, RROSubIPv4)
	}
	if decoded[0].Address != entries[0].Address {
		t.Errorf("entry[0] Address = %s, want %s", decoded[0].Address, entries[0].Address)
	}
	if decoded[0].Flags != 0x01 {
		t.Errorf("entry[0] Flags = 0x%02x, want 0x01", decoded[0].Flags)
	}

	if decoded[1].Type != RROSubLabel {
		t.Errorf("entry[1] Type = %d, want %d", decoded[1].Type, RROSubLabel)
	}
	if decoded[1].Label != 1000 {
		t.Errorf("entry[1] Label = %d, want 1000", decoded[1].Label)
	}

	if decoded[2].Address != entries[2].Address {
		t.Errorf("entry[2] Address = %s, want %s", decoded[2].Address, entries[2].Address)
	}
}

func TestRSVPLabelObject(t *testing.T) {
	// RFC requirement: RFC3209-4.1-2 positive -- a valid 20-bit label (<= MaxLabel 0xFFFFF, upper 12 bits zero) round-trips through encodeLabelObject/decodeLabelObject unchanged (the "max valid" case).
	// RFC requirement: RFC3209-4.1-2 negative -- a label above 0xFFFFF (upper 12 bits nonzero) is rejected by decodeLabelObject with errLabelRange (wire.go:386), not decoded (the "above max" case).
	tests := []struct {
		name    string
		label   uint32
		wantErr bool
	}{
		{"zero", 0, false},
		{"typical", 1000, false},
		{"max valid", MaxLabel, false},
		{"above max", MaxLabel + 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 16)
			n := encodeLabelObject(buf, labelObject{Label: tt.label})
			if n != 8 {
				t.Fatalf("EncodeLabelObject returned %d bytes, want 8", n)
			}

			decoded, err := decodeLabelObject(buf[objHdrLen:n])
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for out-of-range label")
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeLabelObject: %v", err)
			}
			if decoded.Label != tt.label {
				t.Errorf("Label = %d, want %d", decoded.Label, tt.label)
			}
		})
	}
}

// TestRSVPSessionObjectEncoding pins the SESSION object wire format RFC 3209 requires:
// Class-Num SESSION, C-Type 7 (LSP_TUNNEL_IPv4), and a zeroed reserved field on send.
func TestRSVPSessionObjectEncoding(t *testing.T) {
	// RFC requirement: RFC3209-4.6.1-2 positive -- encodeSessionIPv4 stamps SESSION C-Type 7 (LSP_TUNNEL_IPv4) for RSVP-TE LSP tunnels (wire.go:240).
	// RFC requirement: RFC3209-4.6.1-1 positive -- encodeSessionIPv4 writes the SESSION reserved field (body bytes 4-5) as zero on send (wire.go:243-244).
	buf := make([]byte, 32)
	// Nonzero endpoint/tunnel-id/ext-id so a stray write into the reserved field would show.
	n := encodeSessionIPv4(buf, sessionIPv4{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"),
		TunnelID:       0xffff,
		ExtTunnelID:    0xffffffff,
	})
	if n != 16 {
		t.Fatalf("encodeSessionIPv4 returned %d bytes, want 16", n)
	}
	if buf[2] != ClassSession {
		t.Errorf("SESSION Class-Num = %d, want %d", buf[2], ClassSession)
	}
	if buf[3] != CTypeLSPTunnelIPv4 {
		t.Errorf("SESSION C-Type = %d, want %d (LSP_TUNNEL_IPv4)", buf[3], CTypeLSPTunnelIPv4)
	}
	if buf[8] != 0 || buf[9] != 0 {
		t.Errorf("SESSION reserved field = 0x%02x%02x, want 0x0000", buf[8], buf[9])
	}
}

// TestRSVPSenderTemplateReservedZeroOnSend pins the SENDER_TEMPLATE reserved field to
// zero on send.
func TestRSVPSenderTemplateReservedZeroOnSend(t *testing.T) {
	// RFC requirement: RFC3209-4.6.2-1 positive -- encodeSenderTemplate zeroes the SENDER_TEMPLATE reserved field (body bytes 4-5) on send (wire.go:275-276).
	buf := make([]byte, 32)
	n := encodeSenderTemplate(buf, senderTemplateIPv4{
		SenderAddr: netip.MustParseAddr("10.0.0.1"),
		LSPID:      0xffff,
	})
	if n != 12 {
		t.Fatalf("encodeSenderTemplate returned %d bytes, want 12", n)
	}
	if buf[8] != 0 || buf[9] != 0 {
		t.Errorf("SENDER_TEMPLATE reserved field = 0x%02x%02x, want 0x0000", buf[8], buf[9])
	}
}

// RFC requirement: RFC2205-3.1-1 positive -- the common header encodes Version
// rsvpVersion (1) via encodeHeader (wire.go:172) and DecodeHeader accepts it, so a
// Version-1 header round-trips with Version == 1 (the version guard is wire.go:194).
func TestRSVPHeaderRoundTrip(t *testing.T) {
	hdr := Header{
		Version:  rsvpVersion,
		Flags:    0,
		MsgType:  MsgTypePath,
		Checksum: 0,
		TTL:      64,
		Length:   100,
	}

	buf := make([]byte, rsvpHdrLen)
	encodeHeader(buf, hdr)

	decoded, err := DecodeHeader(buf)
	if err != nil {
		t.Fatalf("DecodeHeader: %v", err)
	}
	if decoded.Version != hdr.Version {
		t.Errorf("Version = %d, want %d", decoded.Version, hdr.Version)
	}
	if decoded.MsgType != hdr.MsgType {
		t.Errorf("MsgType = %d, want %d", decoded.MsgType, hdr.MsgType)
	}
	if decoded.TTL != hdr.TTL {
		t.Errorf("TTL = %d, want %d", decoded.TTL, hdr.TTL)
	}
	if decoded.Length != hdr.Length {
		t.Errorf("Length = %d, want %d", decoded.Length, hdr.Length)
	}
}

// RFC requirement: RFC2205-3.1-1 negative -- a common header carrying Version 2
// (buf[0] high nibble 0x2) is rejected by DecodeHeader with errBadVersion
// (wire.go:194); only Version 1 is accepted.
func TestRSVPDecodeHeaderBadVersion(t *testing.T) {
	buf := make([]byte, rsvpHdrLen)
	buf[0] = 0x20
	_, err := DecodeHeader(buf)
	if err == nil {
		t.Fatal("expected error for bad RSVP version")
	}
}

func TestRSVPDecodeHeaderTooShort(t *testing.T) {
	_, err := DecodeHeader(make([]byte, 4))
	if err == nil {
		t.Fatal("expected error for short buffer")
	}
}

func TestRSVPFlowSpecRoundTrip(t *testing.T) {
	fs := FlowSpec{
		TokenRate:      1e9,
		TokenBucket:    1e6,
		PeakRate:       2e9,
		MinPolicedUnit: 64,
		MaxPacketSize:  9000,
	}

	buf := make([]byte, 64)
	n := encodeFlowSpec(buf, ClassSenderTSpec, fs)

	decoded, err := decodeFlowSpec(buf[objHdrLen:n])
	if err != nil {
		t.Fatalf("DecodeFlowSpec: %v", err)
	}
	if decoded.TokenRate != fs.TokenRate {
		t.Errorf("TokenRate = %g, want %g", decoded.TokenRate, fs.TokenRate)
	}
	if decoded.TokenBucket != fs.TokenBucket {
		t.Errorf("TokenBucket = %g, want %g", decoded.TokenBucket, fs.TokenBucket)
	}
	if decoded.PeakRate != fs.PeakRate {
		t.Errorf("PeakRate = %g, want %g", decoded.PeakRate, fs.PeakRate)
	}
	if decoded.MinPolicedUnit != fs.MinPolicedUnit {
		t.Errorf("MinPolicedUnit = %d, want %d", decoded.MinPolicedUnit, fs.MinPolicedUnit)
	}
	if decoded.MaxPacketSize != fs.MaxPacketSize {
		t.Errorf("MaxPacketSize = %d, want %d", decoded.MaxPacketSize, fs.MaxPacketSize)
	}
}

func TestRSVPErrorSpecRoundTrip(t *testing.T) {
	es := errorSpec{
		ErrorNode:  netip.MustParseAddr("10.0.0.5"),
		Flags:      0x01,
		ErrorCode:  24,
		ErrorValue: 1,
	}

	buf := make([]byte, 16)
	n := encodeErrorSpec(buf, es)

	decoded, err := decodeErrorSpec(buf[objHdrLen:n])
	if err != nil {
		t.Fatalf("DecodeErrorSpec: %v", err)
	}
	if decoded.ErrorNode != es.ErrorNode {
		t.Errorf("ErrorNode = %s, want %s", decoded.ErrorNode, es.ErrorNode)
	}
	if decoded.ErrorCode != es.ErrorCode {
		t.Errorf("ErrorCode = %d, want %d", decoded.ErrorCode, es.ErrorCode)
	}
	if decoded.ErrorValue != es.ErrorValue {
		t.Errorf("ErrorValue = %d, want %d", decoded.ErrorValue, es.ErrorValue)
	}
}

// TestDecodeEROCapsHops: decodeERO bounds the hop count so a malicious/looping
// ERO cannot grow an unbounded slice or (after a transit relay) overflow the
// fixed encode buffer. Regression for the unbounded-ERO panic.
func TestDecodeEROCapsHops(t *testing.T) {
	body := make([]byte, 200*8) // 200 IPv4 subobjects (8 bytes each)
	for i := range 200 {
		off := i * 8
		body[off] = EROSubIPv4Prefix
		body[off+1] = 8
		body[off+6] = 32
	}
	hops, err := decodeERO(body)
	if err != nil {
		t.Fatalf("decodeERO: %v", err)
	}
	if len(hops) > maxExplicitRouteHops {
		t.Errorf("decodeERO returned %d hops, want <= %d", len(hops), maxExplicitRouteHops)
	}
}

// TestEncodeEROBounded: encodeERO must not write past the buffer even when given
// more hops than fit (the off+20 guard stops early). Regression for the
// transit-relay buffer overflow from an oversized ERO.
func TestEncodeEROBounded(t *testing.T) {
	hops := make([]eroHop, 300)
	for i := range hops {
		hops[i] = eroHop{Address: netip.MustParsePrefix("2001:db8::1/128")} // 20 bytes each
	}
	buf := make([]byte, maxRSVPMessage)
	n := encodeERO(buf, hops) // must not panic / overflow
	if n > maxRSVPMessage {
		t.Errorf("encodeERO wrote %d bytes, exceeds buffer %d", n, maxRSVPMessage)
	}
}

// TestTransitRelayLargeERONoPanic: the worst-case PATH a transit PLR ever
// re-encodes -- a capped (64-hop) IPv6 ERO plus the protection objects
// (SESSION_ATTRIBUTE with a max-length name + FAST_REROUTE) -- must fit the fixed
// 1500-byte buffer, not panic, and decode back cleanly. This pins the ERO cap to a
// value that leaves room for every trailing object (the buffer is tightest here).
func TestTransitRelayLargeERONoPanic(t *testing.T) {
	hops := make([]eroHop, maxExplicitRouteHops)
	for i := range hops {
		hops[i] = eroHop{Address: netip.MustParsePrefix("2001:db8::1/128")} // 20 bytes each (worst case)
	}
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		ERO:            hops,
		SenderTSpec:    FlowSpec{TokenRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		// Protection appends SESSION_ATTRIBUTE (max-length name) + FAST_REROUTE after
		// the max ERO -- the tightest the PATH buffer ever gets.
		Protection: &protectionRequest{Facility: true, HopLimit: 16, Name: string(make([]byte, maxSessionName))},
	}
	raw := buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64)
	if len(raw) > maxRSVPMessage {
		t.Fatalf("worst-case PATH is %d bytes, exceeds the %d-byte buffer", len(raw), maxRSVPMessage)
	}
	if _, err := DecodeMessage(raw); err != nil {
		t.Fatalf("DecodeMessage of a worst-case PATH: %v", err)
	}
}

// RFC requirement: RFC2205-3.1-2 positive -- the reserved octet of the common
// header (byte 5) is zero on every message ze sends. buildPath composes a real
// PATH through encodeMessage -> encodeHeader (wire.go:177 writes buf[5] = 0), and
// the encoded byte must read back as 0. The receive path does not reject a nonzero
// reserved octet (the RFC does not require it), so this obligation is send-only.
func TestRSVPReservedByteZeroOnSend(t *testing.T) {
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.2"), TunnelID: 1},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
	}
	raw := buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64)
	if len(raw) < rsvpHdrLen {
		t.Fatalf("PATH shorter than the common header: %d bytes", len(raw))
	}
	if raw[5] != 0 {
		t.Fatalf("common-header reserved byte = 0x%02x, want 0x00", raw[5])
	}
}

// RFC requirement: RFC2205-3.1.2-1 positive -- every RSVP object ze encodes carries
// a Length that is a multiple of 4. The whole-message builders compose the wire.go
// object encoders (SESSION, RSVP_HOP, TIME_VALUES, ERO, LABEL_REQUEST,
// SENDER_TEMPLATE, SENDER_TSPEC, STYLE, FLOWSPEC, LABEL, RRO, ERROR_SPEC); walking
// every object header in the built PATH/RESV/PathErr and checking Length %4 == 0
// pins the invariant across the full object set. Decode does not enforce %4, so this
// obligation is send-only.
func TestRSVPObjectLengthMultipleOfFour(t *testing.T) {
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.2"), TunnelID: 1},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		ERO: []eroHop{
			{Address: netip.MustParsePrefix("10.0.0.2/32")},
			{Loose: true, Address: netip.MustParsePrefix("2001:db8::1/128")},
		},
		SenderTSpec:  FlowSpec{TokenRate: 1e8},
		LabelRequest: labelRequest{L3PID: 0x0800},
	}
	rsb := &resvStateBlock{
		Session:  sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.2"), TunnelID: 1},
		FlowSpec: FlowSpec{TokenRate: 1e8},
		Label:    labelObject{Label: 1000},
		Style:    StyleSharedExplicit,
		RRO: []rroEntry{
			{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.2")},
			{Type: RROSubLabel, Label: 1000},
			{Type: RROSubIPv6, Address: netip.MustParseAddr("2001:db8::1")},
		},
	}
	filter := senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1}
	es := errorSpec{ErrorNode: netip.MustParseAddr("10.0.0.5"), ErrorCode: 24, ErrorValue: 1}

	msgs := map[string][]byte{
		"PATH":    buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64),
		"RESV":    buildResv(rsb, filter, DefaultRefreshPeriod, netip.MustParseAddr("10.0.0.2")),
		"PathErr": buildPathErr(psb.Session, filter, psb.SenderTSpec, es, netip.MustParseAddr("10.0.0.1")),
	}

	for name, raw := range msgs {
		hdr, err := DecodeHeader(raw)
		if err != nil {
			t.Fatalf("%s: DecodeHeader: %v", name, err)
		}
		objs := 0
		for off := rsvpHdrLen; off < int(hdr.Length); {
			if off+objHdrLen > len(raw) {
				t.Fatalf("%s: object header at offset %d runs past the %d-byte message", name, off, len(raw))
			}
			objLen := int(binary.BigEndian.Uint16(raw[off : off+2]))
			if objLen < objHdrLen {
				t.Fatalf("%s: object at offset %d has length %d, below the %d-byte header", name, off, objLen, objHdrLen)
			}
			if objLen%4 != 0 {
				t.Errorf("%s: object at offset %d (class %d) length %d is not a multiple of 4", name, off, raw[off+2], objLen)
			}
			off += objLen
			objs++
		}
		if objs == 0 {
			t.Fatalf("%s: walked zero objects", name)
		}
	}
}

// pathWithObject encodes a PATH for psb with one extra object appended, so a
// decoder meets an object class the DecodeMessage switch has no case for. The
// common header Length and checksum are patched, so the only unusual thing about
// the message is the extra object.
func pathWithObject(psb *pathStateBlock, hop netip.Addr, classNum, cType uint8, body []byte) []byte {
	base := buildPath(psb, hop, 64)
	objLen := objHdrLen + len(body)
	out := make([]byte, len(base)+objLen)
	copy(out, base)
	encodeObjectHeader(out[len(base):], objectHeader{Length: uint16(objLen), ClassNum: classNum, CType: cType})
	copy(out[len(base)+objHdrLen:], body)
	binary.BigEndian.PutUint16(out[6:8], uint16(len(out)))
	binary.BigEndian.PutUint16(out[2:4], 0)
	binary.BigEndian.PutUint16(out[2:4], internetChecksum(out))
	return out
}

// detourBodyIPv4 builds the body of an RFC 4090 Section 4.2.1 DETOUR object: one
// (PLR_ID, Avoid_Node_ID) pair of IPv4 addresses.
func detourBodyIPv4(plrID, avoidNodeID netip.Addr) []byte {
	body := make([]byte, 8)
	copy(body[0:4], plrID.AsSlice())
	copy(body[4:8], avoidNodeID.AsSlice())
	return body
}

// TestDecodeUnknownObjectClass drives DecodeMessage with an object class that has
// no case in the switch, and checks the RFC 2205 Section 3.10 classification.
//
// VALIDATES: the whole message is marked unacceptable when the Class-Num
// high-order bit is zero, and is left alone when that bit is set.
// PREVENTS: silently ignoring an object the sender needs this node to understand
// (RFC 4090 Section 4.2: a PLR that gets no PathErr for its DETOUR believes the
// detour LSP is established), and the opposite defect of rejecting an object
// RFC 2205 says to ignore or an optional object a conformant peer may send.
func TestDecodeUnknownObjectClass(t *testing.T) {
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
	}
	hop := netip.MustParseAddr("10.0.0.1")
	detour := detourBodyIPv4(netip.MustParseAddr("10.0.0.5"), netip.MustParseAddr("10.0.0.6"))

	cases := []struct {
		name     string
		classNum uint8
		cType    uint8
		body     []byte
		reject   bool
	}{
		// RFC requirement: RFC2205-3.10-1 positive -- an object of a class ze does
		// not implement whose Class-Num has the form 0bbbbbbb makes the whole
		// message unacceptable. DETOUR (63) is the RFC 4090 Section 4.2 case.
		{name: "detour-63", classNum: ClassDetour, cType: CTypeDetourIPv4, body: detour, reject: true},
		// RFC requirement: RFC2205-3.10-1 positive -- the rule is over the Class-Num
		// form, not over one object, so an unassigned 0bbbbbbb class rejects too.
		{name: "unassigned-0bbbbbbb", classNum: 0x40, cType: 1, body: []byte{0, 0, 0, 0}, reject: true},
		// RFC requirement: RFC2205-3.10-1 negative -- 10bbbbbb is ignored, so the
		// message survives and no error is owed.
		{name: "unassigned-10bbbbbb", classNum: 0xa0, cType: 1, body: []byte{0, 0, 0, 0}, reject: false},
		// RFC requirement: RFC2205-3.10-1 negative -- 11bbbbbb is ignored as well.
		{name: "unassigned-11bbbbbb", classNum: 0xc8, cType: 1, body: []byte{0, 0, 0, 0}, reject: false},
		// RFC requirement: RFC2205-3.10-1 negative -- ADSPEC is a class ze knows and
		// reads no body for, not an unknown one, and RFC 2205 Section 3.1.3 makes it
		// optional in a PATH. Rejecting it would refuse a legal message.
		{name: "adspec-known-unprocessed", classNum: ClassAdspec, cType: 2, body: []byte{0, 0, 0, 0}, reject: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := DecodeMessage(pathWithObject(psb, hop, tc.classNum, tc.cType, tc.body))
			if err != nil {
				t.Fatalf("DecodeMessage: %v", err)
			}
			if msg.HasUnknownObject != tc.reject {
				t.Fatalf("class %#x: HasUnknownObject = %v, want %v", tc.classNum, msg.HasUnknownObject, tc.reject)
			}
			if !tc.reject {
				return
			}
			if msg.UnknownObject.ClassNum != tc.classNum || msg.UnknownObject.CType != tc.cType {
				t.Fatalf("rejected object = (class %d, c-type %d), want (%d, %d)",
					msg.UnknownObject.ClassNum, msg.UnknownObject.CType, tc.classNum, tc.cType)
			}
			if !msg.HasSession || !msg.HasSenderTemplate {
				t.Fatal("decode must keep SESSION and SENDER_TEMPLATE: the PathErr is addressed with them")
			}
		})
	}
}
