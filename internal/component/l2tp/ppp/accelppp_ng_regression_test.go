// Design: docs/architecture/wire/l2tp.md -- PPP control-protocol parsing under hostile input
// Related: lcp.go -- ParseLCPPacket, the Length check these cases exercise
// Related: session_run.go -- handleFrame and handleLCPPacket, the receive dispatch
// Related: dhcpv6.go -- parseDHCPv6, the DHCPv6-PD server parser

package ppp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"
)

// The tests in this file replay the malformed inputs behind accel-ppp-ng's
// security fixes against Ze's PPP and DHCPv6 code. Each test names the
// accel-ppp-ng commit its input comes from. Every negative case is paired
// with a positive control on the same session, so a handler that drops
// every packet fails the test as surely as one that answers the malformed
// packet.

// ngLCPFrame builds one PPP frame carrying an LCP-format packet whose Length
// field is declaredLen, followed by data. The Length field is written as
// given, so it may lie about the octets that follow.
func ngLCPFrame(proto uint16, code, id uint8, declaredLen uint16, data []byte) []byte {
	buf := make([]byte, 64+len(data))
	off := WriteFrame(buf, 0, proto, nil)
	buf[off] = code
	buf[off+1] = id
	binary.BigEndian.PutUint16(buf[off+2:], declaredLen)
	off += lcpHeaderLen
	off += copy(buf[off:], data)
	return buf[:off]
}

// VALIDATES: an LCP packet whose Length field exceeds the received octets is
// dropped without a reply, and the same packet with an honest Length is
// answered.
//
// Source: accel-ppp-ng 1a42ab4 (T8830), "Fix heap data leak due to large,
// invalid packet length in LCP/IPCP/IPv6CP/CCP headers". accel-ppp trusted
// hdr->len and echoed that many octets from its receive buffer.
//
// RFC 1661 Section 5: "Octets outside the range of the Length field should
// be treated as Data Link Layer padding and MUST be ignored on reception."
// A Length past the end leaves no packet to read, so ParseLCPPacket refuses
// it with errLCPLengthMismatch.
func TestNGLCPLengthBeyondBufferDropped(t *testing.T) {
	magic := []byte{0, 0, 0, 0} // newRFC1661Session's peerMagic is zero.

	if _, err := ParseLCPPacket([]byte{LCPEchoRequest, 1, 0xFF, 0xFF, 0, 0, 0, 0}); !errors.Is(err, errLCPLengthMismatch) {
		t.Fatalf("ParseLCPPacket(Length 0xFFFF over 8 octets) error = %v, want errLCPLengthMismatch", err)
	}

	s, rec, _ := newRFC1661Session(LCPStateOpened)
	if s.handleFrame(ngLCPFrame(ProtoLCP, LCPEchoRequest, 1, 0x0400, magic)) {
		t.Fatal("an LCP Echo-Request with Length 1024 over 8 octets ended the session")
	}
	if c := rec.count(); c != 0 {
		t.Fatalf("frames sent for an Echo-Request whose Length exceeds the frame = %d, want 0", c)
	}

	// Positive control: the honest Length draws exactly one Echo-Reply.
	if s.handleFrame(ngLCPFrame(ProtoLCP, LCPEchoRequest, 2, lcpHeaderLen+4, magic)) {
		t.Fatal("a valid Echo-Request ended the session")
	}
	frames := decodeFrames(t, rec)
	if len(frames) != 1 || frames[0].Pkt.Code != LCPEchoReply || frames[0].Pkt.Identifier != 2 {
		t.Fatalf("reply to a valid Echo-Request = %+v, want one Echo-Reply with Identifier 2", frames)
	}
}

// VALIDATES: an IPCP packet whose Length exceeds the received octets is
// dropped without a reply and without a state change, and the same code with
// an honest Length is answered.
//
// Source: accel-ppp-ng 1a42ab4 (T8830), the ipcp_recv and ipv6cp_recv half of
// the same fix.
func TestNGIPCPLengthBeyondBufferDropped(t *testing.T) {
	s, rec, _ := newIPCPSession(t)
	s.ncpStarted = true
	s.disableIPCP = false

	if s.handleFrame(ngLCPFrame(ProtoIPCP, LCPTerminateRequest, 9, 0xFFFF, nil)) {
		t.Fatal("an IPCP Terminate-Request with Length 0xFFFF ended the session")
	}
	if c := rec.count(); c != 1 {
		t.Fatalf("frame count after a lying IPCP Length = %d, want only the initial Configure-Request", c)
	}
	if got := s.ncpState(AddressFamilyIPv4); got != LCPStateReqSent {
		t.Fatalf("IPCP state = %s, want Req-Sent unchanged", got)
	}

	// Positive control: the honest Length draws a Terminate-Ack.
	s.handleFrame(ngLCPFrame(ProtoIPCP, LCPTerminateRequest, 10, lcpHeaderLen, nil))
	frames := decodeFrames(t, rec)
	if len(frames) != 2 || frames[1].Proto != ProtoIPCP || frames[1].Pkt.Code != LCPTerminateAck {
		t.Fatalf("frames after a valid IPCP Terminate-Request = %+v, want a Terminate-Ack second", frames)
	}
}

// VALIDATES: a CCP packet whose Length exceeds the received octets draws a
// Protocol-Reject (Ze implements no CCP) whose Rejected-Information is exactly
// the octets received, never the octets the Length claims.
//
// Source: accel-ppp-ng 1a42ab4 (T8830), the ccp_recv half of the same fix.
//
// RFC 1661 Section 5.7: "Upon reception of a packet with an unknown
// Protocol field, an implementation MUST send a Protocol-Reject".
func TestNGCCPLengthBeyondBufferRejectsReceivedOctetsOnly(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	frame := ngLCPFrame(ProtoCCP, LCPConfigureRequest, 1, 0x0800, []byte{0x11, 0x22})

	if s.handleFrame(frame) {
		t.Fatal("a CCP packet ended the session")
	}
	frames := decodeFrames(t, rec)
	if len(frames) != 1 || frames[0].Pkt.Code != LCPProtocolReject {
		t.Fatalf("reply to CCP = %+v, want one LCP Protocol-Reject", frames)
	}
	data := frames[0].Pkt.Data
	if len(data) < 2 || binary.BigEndian.Uint16(data) != ProtoCCP {
		t.Fatalf("Protocol-Reject Rejected-Protocol = %x, want 80fd", data)
	}
	want := []byte{LCPConfigureRequest, 1, 0x08, 0x00, 0x11, 0x22}
	if got := data[2:]; !bytes.Equal(got, want) {
		t.Fatalf("Rejected-Information = %x, want the %d octets received %x", got, len(want), want)
	}
}

// VALIDATES: LCP Echo-Request, Echo-Reply and Discard-Request shorter than
// the 4-octet Magic-Number are dropped: no Echo-Reply goes out and the
// outstanding-echo counter does not reset. The full-length packets work.
//
// Source: accel-ppp-ng 1957e7d (T8830), "Enforce packet-type specific minimum
// LCP packet lengths".
//
// RFC 1661 Section 5.8 places a Magic-Number field in each of the three
// codes, so a packet without one is malformed.
func TestNGLCPEchoShorterThanMagicDropped(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	s.echoOutstanding = 2

	for _, code := range []uint8{LCPEchoRequest, LCPEchoReply, LCPDiscardRequest} {
		for _, short := range [][]byte{nil, {0}, {0, 0, 0}} {
			if s.handleLCPPacket(LCPPacket{Code: code, Identifier: 7, Data: short}) {
				t.Fatalf("%s with %d-octet Data ended the session", LCPCodeName(code), len(short))
			}
		}
	}
	if c := rec.count(); c != 0 {
		t.Fatalf("frames sent for short echo packets = %d, want 0", c)
	}
	if s.echoOutstanding != 2 {
		t.Fatalf("echoOutstanding = %d after short Echo-Replies, want 2 unchanged", s.echoOutstanding)
	}

	// Positive controls: a full Echo-Reply clears the counter and a full
	// Echo-Request is answered.
	s.handleLCPPacket(LCPPacket{Code: LCPEchoReply, Identifier: 8, Data: []byte{0, 0, 0, 0}})
	if s.echoOutstanding != 0 {
		t.Fatalf("echoOutstanding = %d after a valid Echo-Reply, want 0", s.echoOutstanding)
	}
	s.handleLCPPacket(LCPPacket{Code: LCPEchoRequest, Identifier: 9, Data: []byte{0, 0, 0, 0}})
	if c := rec.count(); c != 1 {
		t.Fatalf("frames after a valid Echo-Request = %d, want 1 Echo-Reply", c)
	}
}

// VALIDATES: a Protocol-Reject too short to hold its Rejected-Protocol field
// neither ends the session nor sends anything nor leaves Opened.
//
// Source: accel-ppp-ng 1957e7d (T8830), the ProtoRej minimum length.
// Ze reads no Rejected-Protocol field on receive: every Protocol-Reject maps
// to the RXJ+ event (codeToEvent), which RFC 1661 Section 4.1 makes a no-op
// in Opened. This test pins that the short form stays that no-op.
func TestNGLCPShortProtocolRejectIsNoOp(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateOpened)
	for _, short := range [][]byte{nil, {0x80}} {
		if s.handleLCPPacket(LCPPacket{Code: LCPProtocolReject, Identifier: 3, Data: short}) {
			t.Fatalf("Protocol-Reject with %d-octet Data ended the session", len(short))
		}
	}
	if c := rec.count(); c != 0 {
		t.Fatalf("frames sent for short Protocol-Rejects = %d, want 0", c)
	}
	if got := s.currentState(); got != LCPStateOpened {
		t.Fatalf("LCP state = %s, want Opened", got)
	}
}

// VALIDATES: every Configuration Option walker stops on an option Length of 0
// or 1 instead of looping on an offset that never advances.
//
// Source: accel-ppp-ng 2fa6475 (T8550), "Zero length packet leading to
// infinite CPU loop" in SSTP. Ze has no SSTP; the same shape (a length field
// that drives the loop offset) lives in the LCP and NCP option walkers.
//
// RFC 1661 Section 6: "The Length field is one octet, and indicates the
// length of this Configuration Option including the Type, Length and Data
// fields." A Length below 2 cannot advance past the option's own header.
func TestNGOptionWalkersStopOnZeroLength(t *testing.T) {
	for _, l := range []byte{0, 1} {
		opts := []byte{LCPOptMRU, l, LCPOptMRU, l}
		if w := WalkLCPOptions(opts); w.Fault != LCPOptionsBadLength {
			t.Errorf("WalkLCPOptions(Length %d) fault = %v, want LCPOptionsBadLength", l, w.Fault)
		}
		if got := scanNCPOptions([]byte{IPCPOptIPAddress, l}, isKnownIPCPOptionForNG); got != ncpOptionsBadLength {
			t.Errorf("scanNCPOptions(Length %d) = %v, want ncpOptionsBadLength", l, got)
		}
	}
	if _, err := ParseLCPPacket([]byte{LCPConfigureRequest, 1, 0, 0}); !errors.Is(err, errLCPLengthMismatch) {
		t.Errorf("ParseLCPPacket(Length 0) error = %v, want errLCPLengthMismatch", err)
	}
}

// isKnownIPCPOptionForNG recognizes only IP-Address, enough for the walker
// test above to tell a bad Length from an unknown Type.
func isKnownIPCPOptionForNG(t uint8) bool { return t == IPCPOptIPAddress }

// VALIDATES: a Configure-Nak suggesting the all-zero Interface-Identifier is
// not adopted, while a valid suggestion is; and every identifier Ze suggests
// has the "u" bit clear and is neither zero nor Ze's own.
//
// Source: accel-ppp-ng bbacd99 (T8598), "IPv6CP: Fix for stack memory leakage
// via ipv6-peer-interface-id calling-sid config", which also normalized the
// generated identifier (u bit cleared, all-zero avoided).
//
// RFC 5072 Section 4.1: "The 'u' (universal/local) bit of the suggested
// identifier MUST be set to zero (0) regardless of its source unless the
// globally unique EUI-48/EUI-64 derived identifier is provided for the
// exclusive use by the remote peer."
func TestNGIPv6CPInterfaceIDNormalized(t *testing.T) {
	s, _, _ := newRFC1661Session(LCPStateOpened)
	local := [ipv6cpInterfaceIDLen]byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80}
	s.localInterfaceID = local

	var zero [ipv6cpInterfaceIDLen]byte
	if v := s.absorbIPv6CPNak(LCPPacket{Code: LCPConfigureNak, Data: ipv6cpInterfaceIDOption(zero)}); v != ncpReplyAbsorbed {
		t.Fatalf("absorbIPv6CPNak(zero) verdict = %v, want ncpReplyAbsorbed", v)
	}
	if s.localInterfaceID != local {
		t.Fatalf("local Interface-Identifier = %x after a zero Nak suggestion, want %x unchanged", s.localInterfaceID, local)
	}

	suggested := [ipv6cpInterfaceIDLen]byte{0x00, 0, 0, 0, 0, 0, 0, 0x01}
	s.absorbIPv6CPNak(LCPPacket{Code: LCPConfigureNak, Data: ipv6cpInterfaceIDOption(suggested)})
	if s.localInterfaceID != suggested {
		t.Fatalf("local Interface-Identifier = %x after a valid Nak suggestion, want %x", s.localInterfaceID, suggested)
	}

	for range 1000 {
		id, err := suggestIPv6CPInterfaceID(local)
		if err != nil {
			t.Fatalf("suggestIPv6CPInterfaceID: %v", err)
		}
		if id[0]&ipv6cpUniversalLocalBitMask != 0 {
			t.Fatalf("suggested Interface-Identifier %x has the u bit set", id)
		}
		if id == zero || id == local {
			t.Fatalf("suggested Interface-Identifier %x is zero or equals the local one", id)
		}
	}
}

// ngDHCPv6Option appends one DHCPv6 option to b.
func ngDHCPv6Option(b []byte, code uint16, data []byte) []byte {
	var hdr [4]byte
	binary.BigEndian.PutUint16(hdr[:], code)
	binary.BigEndian.PutUint16(hdr[2:], uint16(len(data)))
	return append(append(b, hdr[:]...), data...)
}

// ngDHCPv6OptionCodes lists the option codes of a DHCPv6 client/server
// message, failing the test on a malformed option list.
func ngDHCPv6OptionCodes(t *testing.T, msg []byte) []uint16 {
	t.Helper()
	var codes []uint16
	for off := 4; off < len(msg); {
		if len(msg)-off < 4 {
			t.Fatalf("reply option header truncated at %d", off)
		}
		codes = append(codes, binary.BigEndian.Uint16(msg[off:]))
		off += 4 + int(binary.BigEndian.Uint16(msg[off+2:]))
		if off > len(msg) {
			t.Fatalf("reply option overruns the message")
		}
	}
	return codes
}

// ngDHCPv6Handle runs one received message through the server path the
// DHCPv6 reader runs (parseDHCPv6, then handleDHCPv6), with a pool that
// always has a prefix.
func ngDHCPv6Handle(t *testing.T, wire []byte) ([]byte, error) {
	t.Helper()
	msg, err := parseDHCPv6(wire)
	if err != nil {
		return nil, err
	}
	svc, _ := newTestIPv6Service()
	pool := func() (netip.Prefix, bool) { return netip.MustParsePrefix("2001:db8:1::/56"), true }
	return svc.handleDHCPv6(msg, testServerID(), pool)
}

// ngSolicit is a valid Solicit with Client Identifier and IA_PD, followed by
// the extra options.
func ngSolicit(extra ...[]byte) []byte {
	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}
	b := buildSolicit([3]byte{1, 2, 3}, clientDUID, 1)
	for _, e := range extra {
		b = append(b, e...)
	}
	return b
}

// VALIDATES: Relay-Forward messages, nested to any depth or carrying an
// invalid inner Relay-Message, draw no reply, while a Solicit does.
//
// Source: accel-ppp-ng bd1806e (T8475), "crash due to too many Rapid-Forward
// msg layers in a packet", and cc42de1 (T9325), "validate inner
// Relay-Message in RELAY-FORW". Ze's PPP DHCPv6 server is on a point-to-point
// link with no relay agent, parses no relay header, and does not recurse:
// handleDHCPv6 answers only Solicit, Request, Renew and Release.
func TestNGDHCPv6RelayForwardIgnored(t *testing.T) {
	const relayForw = 12
	relayHdr := func(inner []byte) []byte {
		b := make([]byte, 34) // msg-type, hop-count, link-address, peer-address
		b[0] = relayForw
		return ngDHCPv6Option(b, 9, inner) // OPTION_RELAY_MSG
	}
	nested := ngSolicit()
	for range 40 {
		nested = relayHdr(nested)
	}
	cases := map[string][]byte{
		"nested 40 deep":          nested,
		"inner shorter than hdr":  relayHdr([]byte{1, 2}),
		"no inner relay message":  make([]byte, 34),
		"two inner relay message": ngDHCPv6Option(relayHdr(ngSolicit()), 9, ngSolicit()),
	}
	for name, wire := range cases {
		wire[0] = relayForw
		resp, err := ngDHCPv6Handle(t, wire)
		if resp != nil {
			t.Errorf("%s: Relay-Forward drew a %d-octet reply, want none", name, len(resp))
		}
		_ = err // A parse error and a nil reply are both "no reply".
	}

	resp, err := ngDHCPv6Handle(t, ngSolicit())
	if err != nil || len(resp) == 0 || resp[0] != DHCPv6Advertise {
		t.Fatalf("Solicit control: reply %x, err %v; want an Advertise", resp, err)
	}
}

// VALIDATES: a Status-Code option shorter than its 2-octet code is not read,
// and a code outside the RFC's named set is carried as a number.
//
// Source: accel-ppp-ng ef78432 (T9325), "validate Status-Code option payload
// minimum length", and 24afb85 (T8549), "OOB read in print_status() for
// Status code option".
//
// RFC 9915 Section 21.13: "option-len:  2 + length of status-message field."
func TestNGDHCPv6StatusCodeShortOrUnknown(t *testing.T) {
	for _, data := range [][]byte{nil, {0x00}} {
		msg, err := parseDHCPv6(ngSolicit(ngDHCPv6Option(nil, D6OptStatusCode, data)))
		if err != nil {
			t.Fatalf("Status-Code len %d: parse error %v, want the option skipped", len(data), err)
		}
		if msg.StatusCode != nil {
			t.Fatalf("Status-Code len %d: StatusCode = %d, want nil (no code to read)", len(data), *msg.StatusCode)
		}
	}
	msg, err := parseDHCPv6(ngSolicit(ngDHCPv6Option(nil, D6OptStatusCode, []byte{0xFF, 0xFF})))
	if err != nil || msg.StatusCode == nil || *msg.StatusCode != 0xFFFF {
		t.Fatalf("Status-Code 0xFFFF: msg %+v err %v, want StatusCode 65535", msg, err)
	}
}

// VALIDATES: a Solicit carrying two Rapid-Commit options, or a malformed
// AFTR-Name option, is answered with an ordinary Advertise that echoes
// neither option.
//
// Source: accel-ppp-ng 371e7f0 (T8473), "crash due to multiple Rapid-commit
// options present in SOLICIT pkt", and 10a3e72 (T8474), "crash due to bad
// AFTR-Name option in SOLICIT msg". Ze implements no Rapid Commit and no
// AFTR-Name, so both are unknown options skipped by length.
func TestNGDHCPv6RapidCommitAndAFTRNameIgnored(t *testing.T) {
	const (
		optRapidCommit = 14
		optAFTRName    = 64
	)
	wires := map[string][]byte{
		"duplicate Rapid-Commit":  ngSolicit(ngDHCPv6Option(nil, optRapidCommit, nil), ngDHCPv6Option(nil, optRapidCommit, nil)),
		"AFTR-Name no terminator": ngSolicit(ngDHCPv6Option(nil, optAFTRName, []byte{3, 'a', 'b', 'c'})),
		"AFTR-Name compression":   ngSolicit(ngDHCPv6Option(nil, optAFTRName, []byte{0xC0, 0x0C, 0, 0})),
		"AFTR-Name label overrun": ngSolicit(ngDHCPv6Option(nil, optAFTRName, []byte{0x3F, 'a', 0})),
	}
	for name, wire := range wires {
		resp, err := ngDHCPv6Handle(t, wire)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(resp) == 0 || resp[0] != DHCPv6Advertise {
			t.Fatalf("%s: reply %x, want an Advertise", name, resp)
		}
		for _, code := range ngDHCPv6OptionCodes(t, resp) {
			if code == optRapidCommit || code == optAFTRName {
				t.Fatalf("%s: Advertise carries option %d", name, code)
			}
		}
	}
}
