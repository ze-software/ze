// Design: docs/architecture/provisioning/dhcp-server.md -- RFC 2131 conformance coverage
//
// These tests bind the DHCP server's behavior to the MUST-level requirements of
// RFC 2131 (rfc/short/rfc2131.md). Each assertion that enforces a requirement
// carries an `RFC requirement: RFC2131-<section>-<n> <polarity>` tag, which is
// what scripts/dev/rfc_requirements.py reads.

package dhcpserver

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
)

// Option codes ze never emits. Declared here so the assertions that prove their
// absence from a reply read by name rather than by magic number.
const (
	optHostName         = 12 // RFC 2132 Section 3.14
	optMaxMessageSize   = 57 // RFC 2132 Section 9.10
	optClientIdentifier = 61 // RFC 2132 Section 9.14
)

// tlv encodes one DHCP option (code, length, value).
func tlv(code byte, data ...byte) []byte {
	return append([]byte{code, byte(len(data))}, data...)
}

// ipOpt encodes an IPv4-valued option (option 50, 54, ...).
func ipOpt(code byte, a netip.Addr) []byte {
	b := a.As4()
	return tlv(code, b[0], b[1], b[2], b[3])
}

// buildMsg encodes a BOOTREQUEST of the given DHCP message type carrying `extra`
// as raw option bytes after the message-type option.
func buildMsg(msgType byte, mac net.HardwareAddr, xid uint32, ciaddr netip.Addr, flags uint16, extra []byte) []byte {
	pkt := make([]byte, 244+len(extra)+8)
	pkt[0] = opRequest
	pkt[1] = htypeEthernet
	pkt[2] = hlenEthernet
	binary.BigEndian.PutUint32(pkt[4:8], xid)
	binary.BigEndian.PutUint16(pkt[10:12], flags)
	if ciaddr.IsValid() {
		ci := ciaddr.As4()
		copy(pkt[12:16], ci[:])
	}
	copy(pkt[28:34], mac)
	binary.BigEndian.PutUint32(pkt[236:240], magicCookie)
	off := 240
	copy(pkt[off:], []byte{optMessageType, 1, msgType})
	off += 3
	copy(pkt[off:], extra)
	off += len(extra)
	pkt[off] = optEnd
	return pkt
}

// replyOptionCodes walks a server reply's options field strictly by the declared
// length octets and returns the codes present plus whether the walk landed on an
// End (255) marker. It fails the test if any option overruns the field.
func replyOptionCodes(t *testing.T, pkt []byte) (map[byte]bool, bool) {
	t.Helper()
	codes := map[byte]bool{}
	opts := pkt[240:]
	for i := 0; i < len(opts); {
		if opts[i] == optEnd {
			return codes, true
		}
		if opts[i] == optPad {
			i++
			continue
		}
		if i+1 >= len(opts) {
			t.Fatalf("option code %d at offset %d has no length octet", opts[i], 240+i)
		}
		l := int(opts[i+1])
		if i+2+l > len(opts) {
			t.Fatalf("option code %d at offset %d declares length %d, overrunning the options field", opts[i], 240+i, l)
		}
		codes[opts[i]] = true
		i += 2 + l
	}
	return codes, false
}

// replyOptionCounts is replyOptionCodes with per-code occurrence counts.
func replyOptionCounts(t *testing.T, pkt []byte) map[byte]int {
	t.Helper()
	counts := map[byte]int{}
	opts := pkt[240:]
	for i := 0; i < len(opts); {
		if opts[i] == optEnd {
			break
		}
		if opts[i] == optPad {
			i++
			continue
		}
		if i+1 >= len(opts) {
			t.Fatalf("option code %d at offset %d has no length octet", opts[i], 240+i)
		}
		l := int(opts[i+1])
		if i+2+l > len(opts) {
			t.Fatalf("option code %d at offset %d declares length %d, overrunning the options field", opts[i], 240+i, l)
		}
		counts[opts[i]]++
		i += 2 + l
	}
	return counts
}

// exchange drives a DISCOVER/REQUEST pair and returns the OFFER and the ACK.
func exchange(t *testing.T, h *dhcpHandler, mac net.HardwareAddr, xid uint32) ([]byte, []byte) {
	t.Helper()
	offer := h.handle(buildMsg(msgDiscover, mac, xid, netip.Addr{}, 0, nil))
	if offer == nil {
		t.Fatal("expected DHCPOFFER, got nil")
	}
	offered := netip.AddrFrom4([4]byte(offer[16:20]))
	req := append(ipOpt(optRequestedIP, offered), ipOpt(optServerID, h.serverIP)...)
	ack := h.handle(buildMsg(msgRequest, mac, xid, netip.Addr{}, 0, req))
	if ack == nil {
		t.Fatal("expected DHCPACK, got nil")
	}
	return offer, ack
}

// nakForSelecting returns a DHCPNAK produced by the SELECTING path: a REQUEST that
// names this server but carries no requested address.
func nakForSelecting(t *testing.T, h *dhcpHandler, mac net.HardwareAddr, xid uint32) []byte {
	t.Helper()
	nak := h.handle(buildMsg(msgRequest, mac, xid, netip.Addr{}, 0, ipOpt(optServerID, h.serverIP)))
	if nak == nil {
		t.Fatal("expected DHCPNAK, got nil")
	}
	if got := getResponseMsgType(nak); got != msgNak {
		t.Fatalf("message type = %d, want %d (NAK)", got, msgNak)
	}
	return nak
}

// TestServerIdentifierInEveryReply proves every server-originated message carries
// option 54, set to an address on the client's own subnet.
// VALIDATES: RFC 2131 Table 3 -- server identifier in OFFER/ACK/NAK.
// PREVENTS: a reply path that omits option 54, which leaves a client unable to
// address its DHCPREQUEST or DHCPRELEASE at the server that answered it.
// The subnet-containment assertion below is deliberately NOT tagged for
// RFC2131-4.1-2: on-subnet is weaker than the Section 4.1 MUST (an address
// reachable from the client), and the gap annotation on RFC2131-4.1-2 in
// rfc/short/rfc2131.md records where the two diverge.
func TestServerIdentifierInEveryReply(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	offer, ack := exchange(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x01}, 0x2131)
	nak := nakForSelecting(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x02}, 0x2132)

	want := h.serverIP.As4()
	for _, tc := range []struct {
		name  string
		reply []byte
	}{{"OFFER", offer}, {"ACK", ack}, {"NAK", nak}} {
		got := getResponseOption(tc.reply, optServerID)
		// RFC requirement: RFC2131-4.3-1 positive -- OFFER, ACK and NAK each carry the server identifier option (54) holding this server's address.
		if len(got) != 4 || [4]byte(got) != want {
			t.Errorf("%s server identifier = %v, want %v", tc.name, got, want[:])
			continue
		}
		// The identifier lies inside the client's own subnet. That is necessary
		// for reachability but not sufficient for it; see the header comment.
		if sid := netip.AddrFrom4([4]byte(got)); !h.subnet.Prefix.Contains(sid) {
			t.Errorf("%s server identifier %v is outside the client subnet %v", tc.name, sid, h.subnet.Prefix)
		}
	}
}

// TestLeaseTimeInOfferAndAck proves the lease duration is returned to the client.
// VALIDATES: RFC 2131 Table 3 -- IP address lease time in DHCPOFFER and in the
// DHCPACK answering a DHCPREQUEST.
// PREVENTS: an OFFER/ACK without option 51, which leaves the client with no expiry.
func TestLeaseTimeInOfferAndAck(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	offer, ack := exchange(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x03}, 0x2133)

	// RFC requirement: RFC2131-4.3-2 positive -- the DHCPOFFER carries the IP address lease time option (51) with the configured lease duration.
	if lt := getResponseOption(offer, optLeaseTime); len(lt) != 4 || binary.BigEndian.Uint32(lt) != h.subnet.LeaseTimeSec {
		t.Errorf("OFFER lease time = %v, want %d seconds", lt, h.subnet.LeaseTimeSec)
	}
	// RFC requirement: RFC2131-4.3-3 positive -- the DHCPACK answering a DHCPREQUEST carries the IP address lease time option (51) with the configured lease duration.
	if lt := getResponseOption(ack, optLeaseTime); len(lt) != 4 || binary.BigEndian.Uint32(lt) != h.subnet.LeaseTimeSec {
		t.Errorf("ACK lease time = %v, want %d seconds", lt, h.subnet.LeaseTimeSec)
	}
}

// TestReplyOmitsClientOnlyOptions proves the server never sends back the options
// that belong only in a client message.
// VALIDATES: RFC 2131 Table 3 -- requested IP address, client identifier, parameter
// request list and maximum message size are prohibited in OFFER/ACK/NAK, the lease
// time is prohibited in a DHCPNAK, and a DHCPNAK carries no other options.
// PREVENTS: a reply builder that echoes the request's options back to the client.
func TestReplyOmitsClientOnlyOptions(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	offer, ack := exchange(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x04}, 0x2134)
	nak := nakForSelecting(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x05}, 0x2135)

	offerCodes, _ := replyOptionCodes(t, offer)
	ackCodes, _ := replyOptionCodes(t, ack)
	nakCodes, _ := replyOptionCodes(t, nak)

	for name, codes := range map[string]map[byte]bool{"OFFER": offerCodes, "ACK": ackCodes, "NAK": nakCodes} {
		// RFC requirement: RFC2131-4.3-5 positive -- no server reply (OFFER, ACK or NAK) carries the requested IP address option (50).
		if codes[optRequestedIP] {
			t.Errorf("%s carries the requested IP address option (50)", name)
		}
		// RFC requirement: RFC2131-4.3-7 positive -- no server reply (OFFER, ACK or NAK) carries the parameter request list option (55).
		if codes[optParamReqList] {
			t.Errorf("%s carries the parameter request list option (55)", name)
		}
		// RFC requirement: RFC2131-4.3-8 positive -- no server reply (OFFER, ACK or NAK) carries the maximum DHCP message size option (57).
		if codes[optMaxMessageSize] {
			t.Errorf("%s carries the maximum message size option (57)", name)
		}
	}

	for name, codes := range map[string]map[byte]bool{"OFFER": offerCodes, "ACK": ackCodes} {
		// RFC requirement: RFC2131-4.3-6 positive -- neither the DHCPOFFER nor the DHCPACK carries the client identifier option (61).
		if codes[optClientIdentifier] {
			t.Errorf("%s carries the client identifier option (61)", name)
		}
	}

	// RFC requirement: RFC2131-4.3-4 positive -- the DHCPNAK carries no IP address lease time option (51).
	if nakCodes[optLeaseTime] {
		t.Error("NAK carries the IP address lease time option (51)")
	}
	// RFC requirement: RFC2131-4.3-9 positive -- the DHCPNAK carries only the message type (53) and server identifier (54) options, no others.
	for code := range nakCodes {
		if code != optMessageType && code != optServerID {
			t.Errorf("NAK carries option %d; a DHCPNAK carries no options beyond the message type and server identifier", code)
		}
	}
}

// TestReplyDoesNotEchoClientOptions proves a request loaded with the very options a
// reply may not carry does not make the server emit them.
// VALIDATES: RFC 2131 Table 3, exercised adversarially -- the prohibition holds for
// input that invites the violation, not merely for a bare request.
// PREVENTS: an "echo unknown options back" reply builder, the natural way this
// prohibition gets broken.
func TestReplyDoesNotEchoClientOptions(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	// Every option Table 3 forbids in a reply, sent by the client in its request.
	loaded := func(reqIP netip.Addr) []byte {
		var b []byte
		b = append(b, ipOpt(optRequestedIP, reqIP)...)
		b = append(b, tlv(optLeaseTime, 0x00, 0x00, 0x1c, 0x20)...)
		b = append(b, tlv(optParamReqList, optSubnetMask, optRouter, optDNS)...)
		b = append(b, tlv(optMaxMessageSize, 0x02, 0x40)...)
		b = append(b, tlv(optClientIdentifier, 0x01, 0xde, 0xad, 0xbe, 0xef, 0x00, 0x01)...)
		b = append(b, tlv(optHostName, 'z', 'e')...)
		return b
	}

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x06}
	offer := h.handle(buildMsg(msgDiscover, mac, 0x2136, netip.Addr{}, 0,
		loaded(netip.MustParseAddr("192.168.1.150"))))
	if offer == nil {
		t.Fatal("expected DHCPOFFER, got nil")
	}
	offered := netip.AddrFrom4([4]byte(offer[16:20]))

	ackReq := append(ipOpt(optServerID, h.serverIP), loaded(offered)...)
	ack := h.handle(buildMsg(msgRequest, mac, 0x2136, netip.Addr{}, 0, ackReq))
	if ack == nil {
		t.Fatal("expected DHCPACK, got nil")
	}

	// A NAK carrying a client-supplied option 50: ciaddr places the client on our
	// subnet while the requested address does not belong to it, so the server NAKs.
	nakMAC := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x07}
	nak := h.handle(buildMsg(msgRequest, nakMAC, 0x2137, netip.MustParseAddr("192.168.1.50"), 0,
		loaded(netip.MustParseAddr("10.9.9.9"))))
	if nak == nil {
		t.Fatal("expected DHCPNAK, got nil")
	}
	if got := getResponseMsgType(nak); got != msgNak {
		t.Fatalf("message type = %d, want %d (NAK)", got, msgNak)
	}

	offerCodes, _ := replyOptionCodes(t, offer)
	ackCodes, _ := replyOptionCodes(t, ack)
	nakCodes, _ := replyOptionCodes(t, nak)

	for name, codes := range map[string]map[byte]bool{"OFFER": offerCodes, "ACK": ackCodes, "NAK": nakCodes} {
		// RFC requirement: RFC2131-4.3-5 negative -- a request that carries the requested IP address option (50) still yields a reply without it; the prohibited option is not echoed.
		if codes[optRequestedIP] {
			t.Errorf("%s echoed the client's requested IP address option (50)", name)
		}
		// RFC requirement: RFC2131-4.3-7 negative -- a request that carries a parameter request list (55) still yields a reply without it; the prohibited option is not echoed.
		if codes[optParamReqList] {
			t.Errorf("%s echoed the client's parameter request list option (55)", name)
		}
		// RFC requirement: RFC2131-4.3-8 negative -- a request that carries a maximum message size option (57) still yields a reply without it; the prohibited option is not echoed.
		if codes[optMaxMessageSize] {
			t.Errorf("%s echoed the client's maximum message size option (57)", name)
		}
	}

	for name, codes := range map[string]map[byte]bool{"OFFER": offerCodes, "ACK": ackCodes} {
		// RFC requirement: RFC2131-4.3-6 negative -- a request that carries a client identifier option (61) still yields an OFFER/ACK without it; the prohibited option is not echoed.
		if codes[optClientIdentifier] {
			t.Errorf("%s echoed the client's client identifier option (61)", name)
		}
	}

	// RFC requirement: RFC2131-4.3-4 negative -- a request asking for a specific lease time (51) that is answered with a DHCPNAK still yields a NAK carrying no lease time.
	if nakCodes[optLeaseTime] {
		t.Error("NAK echoed a lease time option (51) in answer to a request that asked for one")
	}
	// RFC requirement: RFC2131-4.3-9 negative -- a request loaded with options 50/51/55/57/61/12 answered by a DHCPNAK still yields a NAK holding only the message type and server identifier.
	for code := range nakCodes {
		if code != optMessageType && code != optServerID {
			t.Errorf("NAK carries option %d after a request loaded with client options", code)
		}
	}
}

// TestChaddrIdentifiesClientWithoutClientIdentifier proves a client that sends no
// client identifier is identified by its hardware address.
// VALIDATES: RFC 2131 Section 4.2 -- absent a client identifier, the server uses
// the contents of the chaddr field to identify the client.
// PREVENTS: identification by transaction id, which would hand a second address to
// a client that merely retransmitted with a fresh xid.
func TestChaddrIdentifiesClientWithoutClientIdentifier(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x08}
	first := h.handle(buildMsg(msgDiscover, mac, 0x2138, netip.Addr{}, 0, nil))
	// A different xid, so only the hardware address can tie the two requests together.
	second := h.handle(buildMsg(msgDiscover, mac, 0x9999, netip.Addr{}, 0, nil))
	if first == nil || second == nil {
		t.Fatal("expected a DHCPOFFER for both requests, got nil")
	}
	firstIP := netip.AddrFrom4([4]byte(first[16:20]))
	secondIP := netip.AddrFrom4([4]byte(second[16:20]))
	// RFC requirement: RFC2131-4.2-2 positive -- two DISCOVERs from the same chaddr and no client identifier map to one client, so the same address is offered.
	if firstIP != secondIP {
		t.Errorf("same chaddr offered %v then %v; chaddr must identify the client", firstIP, secondIP)
	}
	// The chaddr the reply carries is the one that identified the client.
	if !bytes.Equal(first[28:34], mac) {
		t.Errorf("reply chaddr = %v, want %v", first[28:34], mac)
	}
}

// TestUnidentifiableClientRejected proves a request that carries no usable hardware
// address is not answered, and that two distinct chaddrs are two distinct clients.
// VALIDATES: RFC 2131 Section 4.2 -- with no client identifier and no chaddr there
// is nothing to identify the client by, so the server cannot bind an address.
// PREVENTS: keying a binding on the empty identifier, which would collapse every
// such client onto one lease.
func TestUnidentifiableClientRejected(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	pkt := buildMsg(msgDiscover, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x09}, 0x2139, netip.Addr{}, 0, nil)
	pkt[2] = 0 // hlen = 0: no hardware address to identify the client by
	// RFC requirement: RFC2131-4.2-2 negative -- a request declaring a zero-length chaddr carries no identity the server can use, and is dropped rather than bound to an empty identifier.
	if resp := h.handle(pkt); resp != nil {
		t.Errorf("expected no reply to a request with hlen = 0, got message type %d", getResponseMsgType(resp))
	}

	other := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x0a}
	another := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x0b}
	a := h.handle(buildMsg(msgDiscover, other, 0x213a, netip.Addr{}, 0, nil))
	b := h.handle(buildMsg(msgDiscover, another, 0x213a, netip.Addr{}, 0, nil))
	if a == nil || b == nil {
		t.Fatal("expected a DHCPOFFER for both requests, got nil")
	}
	if netip.AddrFrom4([4]byte(a[16:20])) == netip.AddrFrom4([4]byte(b[16:20])) {
		t.Error("two distinct chaddrs sharing one xid received the same address; identity must come from chaddr")
	}
}

// TestMagicCookieRequiredAndEmitted proves the options field is framed by the magic
// cookie in both directions.
// VALIDATES: RFC 2131 Section 3 -- the first four octets of the options field are
// 99.130.83.99.
// PREVENTS: parsing a plain BOOTP message as DHCP, and emitting a reply whose
// options a client cannot find.
func TestMagicCookieRequiredAndEmitted(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	offer, ack := exchange(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x0c}, 0x213b)
	nak := nakForSelecting(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x0d}, 0x213c)

	for name, reply := range map[string][]byte{"OFFER": offer, "ACK": ack, "NAK": nak} {
		// RFC requirement: RFC2131-3-1 positive -- every server reply starts its options field with the magic cookie 0x63825363.
		if got := binary.BigEndian.Uint32(reply[236:240]); got != uint32(magicCookie) {
			t.Errorf("%s options field starts with %#08x, want %#08x", name, got, uint32(magicCookie))
		}
	}

	bad := buildMsg(msgDiscover, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x0e}, 0x213d, netip.Addr{}, 0, nil)
	binary.BigEndian.PutUint32(bad[236:240], 0xDEADBEEF)
	// RFC requirement: RFC2131-3-1 negative -- a message whose options field does not start with the magic cookie is not treated as DHCP and draws no reply.
	if resp := h.handle(bad); resp != nil {
		t.Errorf("expected no reply to a message without the magic cookie, got message type %d", getResponseMsgType(resp))
	}
}

// TestReplyOptionsTerminatedByEnd proves every reply's options field is closed by
// the End option.
// VALIDATES: RFC 2131 Section 4.1 -- the options field ends with the end option (255).
// PREVENTS: a reply whose last option runs into the trailing pad, which a client
// parses as an unterminated (and therefore untrusted) option stream.
func TestReplyOptionsTerminatedByEnd(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	offer, ack := exchange(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x0f}, 0x213e)
	nak := nakForSelecting(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x10}, 0x213f)

	for name, reply := range map[string][]byte{"OFFER": offer, "ACK": ack, "NAK": nak} {
		_, reachedEnd := replyOptionCodes(t, reply)
		// RFC requirement: RFC2131-4.1-3 positive -- walking each reply's options by their length octets lands on the End option (255).
		if !reachedEnd {
			t.Errorf("%s options field is not terminated by the End option (255)", name)
		}
	}

	// A request whose own options field is never terminated: the trailing bytes are
	// neither Pad nor End, so a server that mirrored its input framing would emit an
	// unterminated reply.
	unterminated := buildMsg(msgDiscover, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x11}, 0x2140, netip.Addr{}, 0, nil)
	for i := 243; i < len(unterminated); i++ {
		unterminated[i] = 0x01
	}
	resp := h.handle(unterminated)
	if resp == nil {
		t.Fatal("expected a DHCPOFFER for the unterminated request, got nil")
	}
	_, reachedEnd := replyOptionCodes(t, resp)
	// RFC requirement: RFC2131-4.1-3 negative -- a request whose options field carries no End option still yields a reply whose options field is terminated by one.
	if !reachedEnd {
		t.Error("reply to an unterminated request is itself unterminated; the End option (255) must always close the options field")
	}
}

// TestOptionsContainedWithinTheirField proves options are read and written inside
// the bounds of the field that carries them.
// VALIDATES: RFC 2131 Section 4.1 -- each option must be entirely contained in the
// field it appears in.
// PREVENTS: an emitted option truncated by the buffer edge, and a read that walks
// past the end of a received packet on an over-long declared length.
func TestOptionsContainedWithinTheirField(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	offer, ack := exchange(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x12}, 0x2141)
	// replyOptionCodes fails the test if any option's declared length overruns the field.
	for _, reply := range [][]byte{offer, ack} {
		// RFC requirement: RFC2131-4.1-6 positive -- every option in the OFFER and the ACK is entirely contained in the options field, with no declared length running past its end.
		if _, reachedEnd := replyOptionCodes(t, reply); !reachedEnd {
			t.Error("reply options do not terminate inside the options field")
		}
	}

	// Option 50 declares four octets but only two remain before the field ends.
	// The malformed option is planted BEFORE any End marker and the packet stops
	// right after it, so the option walk reaches it: the bounds check inside
	// parseOptionAddr is then the only thing that can refuse the read. Planting it
	// after the End marker instead would prove nothing, because the walk breaks on
	// End before it ever looks at the option.
	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x13}
	serverIDOpt := ipOpt(optServerID, h.serverIP)
	truncated := buildMsg(msgRequest, mac, 0x2142, netip.Addr{}, 0, serverIDOpt)
	// buildMsg lays out: magic cookie at 236, message-type option at 240..242,
	// `extra` next, then the End marker. Fail loudly if that layout ever changes,
	// so this test cannot silently go back to asserting nothing.
	endIdx := 243 + len(serverIDOpt)
	if truncated[endIdx] != optEnd {
		t.Fatalf("buildMsg layout changed: expected the End option at offset %d, found %d", endIdx, truncated[endIdx])
	}
	truncated = append(truncated[:endIdx:endIdx], optRequestedIP, 4, 0xc0, 0xa8)
	// RFC requirement: RFC2131-4.1-6 negative -- a received option whose declared length runs past the end of its field is not read beyond the field boundary, so no address is taken from it.
	if got := parseOptionAddr(truncated, optRequestedIP); got.IsValid() {
		t.Errorf("read %v from an option truncated by the end of the field; an option not entirely contained must not be read", got)
	}
	// The request is still answered as a SELECTING DHCPREQUEST without a usable
	// requested address: a DHCPNAK, not a crash and not a bogus binding.
	resp := h.handle(truncated)
	if resp == nil {
		t.Fatal("expected a DHCPNAK for the truncated request, got nil")
	}
	if got := getResponseMsgType(resp); got != msgNak {
		t.Errorf("message type = %d, want %d (NAK)", got, msgNak)
	}
}

// TestFlagsReservedBitsIgnored proves the server keys delivery on the BROADCAST bit
// alone.
// VALIDATES: RFC 2131 Section 2 -- flags bits 1 to 15 are MUST BE ZERO and are
// ignored by servers.
// PREVENTS: a delivery decision that tests the whole flags word, which would
// broadcast (or unicast) at the whim of a client that set a reserved bit.
func TestFlagsReservedBitsIgnored(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x14}
	clean := buildMsg(msgDiscover, mac, 0x2143, netip.Addr{}, 0x0000, nil)
	offer := h.handle(clean)
	if offer == nil {
		t.Fatal("expected DHCPOFFER, got nil")
	}
	offered := netip.AddrFrom4([4]byte(offer[16:20]))
	// RFC requirement: RFC2131-2-3 positive -- with every flags bit zero the reply is unicast to the offered address, the BROADCAST-bit-clear behavior.
	if dst := responseAddr(clean, offer); dst.IP.String() != offered.String() || dst.Port != 68 {
		t.Errorf("destination = %s, want %s:68", dst, offered)
	}

	// A client that violates the MUST-BE-ZERO rule: every reserved bit set, BROADCAST clear.
	mbz := buildMsg(msgDiscover, mac, 0x2144, netip.Addr{}, 0x7fff, nil)
	mbzOffer := h.handle(mbz)
	if mbzOffer == nil {
		t.Fatal("expected DHCPOFFER for the reserved-bits request, got nil")
	}
	// RFC requirement: RFC2131-2-3 negative -- a client that sets the reserved flags bits is not rejected and delivery does not change: the reply is still unicast, because bits 1 to 15 are ignored.
	if dst := responseAddr(mbz, mbzOffer); dst.IP.String() != offered.String() || dst.Port != 68 {
		t.Errorf("reserved flags bits changed delivery: destination = %s, want %s:68", dst, offered)
	}

	// The BROADCAST bit alone still decides, even alongside the reserved bits.
	bcast := buildMsg(msgDiscover, mac, 0x2145, netip.Addr{}, 0xffff, nil)
	bcastOffer := h.handle(bcast)
	if bcastOffer == nil {
		t.Fatal("expected DHCPOFFER for the broadcast request, got nil")
	}
	if dst := responseAddr(bcast, bcastOffer); !dst.IP.Equal(net.IPv4bcast) {
		t.Errorf("destination = %s, want the broadcast address", dst)
	}
}

// TestRenewalTimersOrderedWithinLease proves the emitted T1/T2 timers are ordered.
// VALIDATES: RFC 2131 Section 4.4.5 -- T1 is earlier than T2 and T2 is earlier than
// the lease expiry.
// PREVENTS: a lease duration short enough for the integer T1/T2 arithmetic to
// collapse, which would put a client into REBINDING at or before RENEWING.
func TestRenewalTimersOrderedWithinLease(t *testing.T) {
	t.Parallel()

	for _, leaseSec := range []uint32{60, 3600, 86400, 604800} {
		sub := subnetConfig{
			Prefix:       netip.MustParsePrefix("192.168.1.0/24"),
			Ranges:       []addressRange{{Name: "pool", Start: netip.MustParseAddr("192.168.1.100"), Stop: netip.MustParseAddr("192.168.1.200")}},
			LeaseTimeSec: leaseSec,
		}
		h := newDHCPHandler(sub, netip.MustParseAddr("192.168.1.1"), pxeConfig{})
		offer := h.handle(buildMsg(msgDiscover, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x15}, 0x2146, netip.Addr{}, 0, nil))
		h.leases.stop()
		if offer == nil {
			t.Fatalf("lease %ds: expected DHCPOFFER, got nil", leaseSec)
		}
		t1b := getResponseOption(offer, optT1)
		t2b := getResponseOption(offer, optT2)
		if len(t1b) != 4 || len(t2b) != 4 {
			t.Fatalf("lease %ds: T1 = %v, T2 = %v, want four octets each", leaseSec, t1b, t2b)
		}
		t1 := binary.BigEndian.Uint32(t1b)
		t2 := binary.BigEndian.Uint32(t2b)
		// RFC requirement: RFC2131-4.4.5-2 positive -- the renewal timer T1 the server returns is strictly earlier than the rebinding timer T2.
		if t1 >= t2 {
			t.Errorf("lease %ds: T1 = %d, T2 = %d; T1 must be earlier than T2", leaseSec, t1, t2)
		}
		// RFC requirement: RFC2131-4.4.5-3 positive -- the rebinding timer T2 the server returns is strictly earlier than the lease expiry.
		if t2 >= leaseSec {
			t.Errorf("lease %ds: T2 = %d; T2 must be earlier than the lease expiry of %d", leaseSec, t2, leaseSec)
		}
	}
}

// TestShortLeaseTimeRejected proves a lease duration that would collapse T1 and T2
// cannot be configured.
// VALIDATES: RFC 2131 Section 4.4.5 -- the ordering of T1, T2 and the lease expiry
// is guarded at the configuration boundary, since T1 = lease/2 and T2 = 7*lease/8
// are equal for a lease of one or two seconds and both are zero for a lease of zero.
// PREVENTS: relaxing the lease-time bound and silently emitting T1 == T2.
func TestShortLeaseTimeRejected(t *testing.T) {
	t.Parallel()

	cfg := func(sec string) string {
		return `{"service":{"dhcp-server":{"enabled":"true","listen-interface":["br0"],"shared-network":{"L":{"subnet":{"192.168.1.0/24":{"range":{"start":"192.168.1.100","stop":"192.168.1.200"},"lease-time":"` + sec + `"}}}}}}}`
	}

	for _, sec := range []string{"1", "2", "59"} {
		// A lease of 1 or 2 seconds gives T1 == T2 in integer arithmetic, so accepting
		// it would emit timers that are not ordered.
		// RFC requirement: RFC2131-4.4.5-2 negative -- a lease duration short enough to make T1 equal T2 is rejected by configuration validation, so no such reply can be built.
		if _, err := parseConfig(cfg(sec)); err == nil {
			t.Errorf("lease-time %q accepted; a lease this short collapses T1 and T2", sec)
		}
	}
	// RFC requirement: RFC2131-4.4.5-3 negative -- a lease duration of zero, for which T2 could not be earlier than the expiry, is rejected by configuration validation.
	if _, err := parseConfig(cfg("0")); err == nil {
		t.Error("lease-time 0 accepted; T2 cannot be earlier than a zero lease expiry")
	}
	if _, err := parseConfig(cfg("60")); err != nil {
		t.Errorf("lease-time 60 rejected (%v); the shortest supported lease must remain configurable", err)
	}
}

// TestUnconfiguredParametersOmitted proves the server returns only the parameters
// it holds a value for.
// VALIDATES: RFC 2131 Section 4.3.1 -- a server with no value for a parameter must
// not return one, and a parameter the client asked for that the server has an
// explicitly configured value for is returned carrying that value.
// PREVENTS: emitting a zero-valued router or DNS option, which a client installs as
// a default route to 0.0.0.0.
func TestUnconfiguredParametersOmitted(t *testing.T) {
	t.Parallel()

	bare := subnetConfig{
		Prefix:       netip.MustParsePrefix("192.168.1.0/24"),
		Ranges:       []addressRange{{Name: "pool", Start: netip.MustParseAddr("192.168.1.100"), Stop: netip.MustParseAddr("192.168.1.200")}},
		LeaseTimeSec: 3600,
	}
	h := newDHCPHandler(bare, netip.MustParseAddr("192.168.1.1"), pxeConfig{})
	defer h.leases.stop()

	offer := h.handle(buildMsg(msgDiscover, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x16}, 0x2147, netip.Addr{}, 0, nil))
	if offer == nil {
		t.Fatal("expected DHCPOFFER, got nil")
	}
	codes, _ := replyOptionCodes(t, offer)
	for _, code := range []byte{optRouter, optDNS, optDomainName} {
		// RFC requirement: RFC2131-4.3.1-4 positive -- a server holding no value for the router, DNS or domain-name parameter returns no option for it.
		if codes[code] {
			t.Errorf("OFFER carries option %d for a parameter the server has no value for", code)
		}
	}

	// The same server, asked explicitly for those parameters.
	asked := h.handle(buildMsg(msgDiscover, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x17}, 0x2148, netip.Addr{}, 0,
		tlv(optParamReqList, optRouter, optDNS, optDomainName)))
	if asked == nil {
		t.Fatal("expected DHCPOFFER, got nil")
	}
	askedCodes, _ := replyOptionCodes(t, asked)
	for _, code := range []byte{optRouter, optDNS, optDomainName} {
		// RFC requirement: RFC2131-4.3.1-4 negative -- a client that explicitly requests the router, DNS and domain-name parameters from a server holding no value for them still receives no option for them, rather than an empty or zero-valued one.
		if askedCodes[code] {
			t.Errorf("OFFER carries option %d because the client asked for it, though the server has no value", code)
		}
	}

	// A fully configured server returns its configured values for those parameters.
	full := newTestServer(t)
	defer full.leases.stop()
	configured := full.handle(buildMsg(msgDiscover, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x18}, 0x2149, netip.Addr{}, 0,
		tlv(optParamReqList, optSubnetMask, optRouter, optDNS, optDomainName)))
	if configured == nil {
		t.Fatal("expected DHCPOFFER, got nil")
	}
	wantRouter := full.subnet.DefaultRouter.As4()
	// RFC requirement: RFC2131-4.3.1-2 positive -- a parameter the client requested for which the server holds an explicitly configured value is returned carrying that configured value.
	if got := getResponseOption(configured, optRouter); len(got) != 4 || [4]byte(got) != wantRouter {
		t.Errorf("router option = %v, want the configured %v", got, wantRouter[:])
	}
	if got := string(getResponseOption(configured, optDomainName)); got != full.subnet.DomainName {
		t.Errorf("domain name option = %q, want the configured %q", got, full.subnet.DomainName)
	}
	if got := getResponseOption(configured, optDNS); len(got) != 4*len(full.subnet.DNSServers) {
		t.Errorf("DNS option = %v, want the %d configured servers", got, len(full.subnet.DNSServers))
	}
}

// TestEachParameterEmittedOnce proves no parameter is returned twice in one reply.
// VALIDATES: RFC 2131 Section 4.3.1 -- each requested parameter appears once in the
// reply unless the option's definition allows repetition.
// PREVENTS: a duplicated option, which leaves the client's choice of instance
// undefined (and, for a router or DNS option, its configuration undefined).
func TestEachParameterEmittedOnce(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	offer, ack := exchange(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x19}, 0x214a)
	for name, reply := range map[string][]byte{"OFFER": offer, "ACK": ack} {
		for code, n := range replyOptionCounts(t, reply) {
			// RFC requirement: RFC2131-4.3.1-6 positive -- every option in the OFFER and the ACK appears exactly once.
			if n != 1 {
				t.Errorf("%s carries option %d %d times, want once", name, code, n)
			}
		}
	}

	// A client that names the same parameters repeatedly, and echoes an option the
	// server also emits, must not multiply them in the reply.
	repeat := tlv(optParamReqList, optRouter, optRouter, optDNS, optDNS, optSubnetMask, optSubnetMask)
	repeat = append(repeat, ipOpt(optRouter, netip.MustParseAddr("192.168.1.254"))...)
	repeat = append(repeat, tlv(optParamReqList, optDomainName)...)

	pxe := newTestPXEServer(t)
	defer pxe.leases.stop()
	for name, server := range map[string]*dhcpHandler{"plain": h, "pxe": pxe} {
		reply := server.handle(buildMsg(msgDiscover, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x1a}, 0x214b, netip.Addr{}, 0, repeat))
		if reply == nil {
			t.Fatalf("%s: expected DHCPOFFER, got nil", name)
		}
		for code, n := range replyOptionCounts(t, reply) {
			// RFC requirement: RFC2131-4.3.1-6 negative -- a request naming the same parameters several times, and carrying a copy of an option the server emits, still yields a reply with each option exactly once.
			if n != 1 {
				t.Errorf("%s reply carries option %d %d times after a request repeating its parameters, want once", name, code, n)
			}
		}
	}
}
