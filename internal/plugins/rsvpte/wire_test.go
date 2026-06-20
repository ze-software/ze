package rsvpte

import (
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
