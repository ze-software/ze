// Design: docs/architecture/provisioning/dhcp-server.md -- RFC 2131 client identity and reply options
// Related: rfc2131_test.go -- the buildMsg, tlv, ipOpt and replyOptionCodes helpers

package dhcpserver

import (
	"bytes"
	"net"
	"net/netip"
	"strings"
	"testing"
)

// TestRFC2131ChaddrContentsIdentifyTheClient proves a client that sends no
// client identifier is identified by the contents of its chaddr field, and by
// nothing else.
// VALIDATES: RFC 2131 Section 4.2 -- "If the client does not provide a 'client
// identifier' option, the server MUST use the contents of the 'chaddr' field to
// identify the client."
// PREVENTS: identity keyed on the transaction id or on part of the hardware
// address. Method: requests that share a chaddr and differ in every other field
// the client controls (xid, flags) map to one client; requests that share the
// xid and differ in one chaddr octet, first or last, map to two clients.
func TestRFC2131ChaddrContentsIdentifyTheClient(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x30}
	first := h.handle(buildMsg(msgDiscover, mac, 0x42201, netip.Addr{}, 0x0000, nil))
	second := h.handle(buildMsg(msgDiscover, mac, 0x42202, netip.Addr{}, 0x8000, nil))
	if first == nil || second == nil {
		t.Fatal("expected a DHCPOFFER for both requests, got nil")
	}
	firstIP := netip.AddrFrom4([4]byte(first[16:20]))
	secondIP := netip.AddrFrom4([4]byte(second[16:20]))
	// RFC requirement: RFC2131-4.2-2 positive -- two DISCOVERs with no client identifier that carry the same chaddr and a different xid and flags word are one client: the same address is offered to both.
	if firstIP != secondIP {
		t.Errorf("one chaddr offered %v then %v; the chaddr contents must identify the client", firstIP, secondIP)
	}

	// Two clients that share the xid, and whose chaddr differs in one octet only.
	lastOctet := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x31}
	firstOctet := net.HardwareAddr{0x12, 0x22, 0x33, 0x44, 0x55, 0x30}
	byLast := h.handle(buildMsg(msgDiscover, lastOctet, 0x42201, netip.Addr{}, 0x0000, nil))
	byFirst := h.handle(buildMsg(msgDiscover, firstOctet, 0x42201, netip.Addr{}, 0x0000, nil))
	if byLast == nil || byFirst == nil {
		t.Fatal("expected a DHCPOFFER for both requests, got nil")
	}
	lastIP := netip.AddrFrom4([4]byte(byLast[16:20]))
	firstOctetIP := netip.AddrFrom4([4]byte(byFirst[16:20]))
	// RFC requirement: RFC2131-4.2-2 negative -- a DISCOVER whose chaddr differs from a bound client's in its first or its last octet only, with the same xid, is a different client: it is not offered that client's address.
	if lastIP == firstIP {
		t.Errorf("a chaddr differing in its last octet was offered the other client's %v", firstIP)
	}
	if firstOctetIP == firstIP {
		t.Errorf("a chaddr differing in its first octet was offered the other client's %v", firstIP)
	}
	if lastIP == firstOctetIP {
		t.Errorf("two distinct chaddrs were both offered %v", lastIP)
	}
}

// TestRFC2131EveryReplyOptionContainedInItsField proves that every option the
// server sends lies entirely inside the field that holds it.
// VALIDATES: RFC 2131 Section 4.1 -- "Any individual option in the 'options',
// 'sname' and 'file' fields MUST be entirely contained in that field."
// PREVENTS: a reply whose options field a client cannot walk. Method: the OFFER,
// the ACK and the DHCPNAK (a separate producer) are walked by their length
// octets to the End option. The sname and file fields must hold no option at
// all, since ze sends no option overload (52). The negative forces the encoder
// toward the violation: a bootfile name longer than one option can hold (255
// octets) must not be written as an option whose length octet disagrees with
// its data.
func TestRFC2131EveryReplyOptionContainedInItsField(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x32}
	offer, ack := exchange(t, h, mac, 0x42203)
	nak := nakForSelecting(t, h, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x33}, 0x42204)
	for _, reply := range []struct {
		name string
		pkt  []byte
	}{{"OFFER", offer}, {"ACK", ack}, {"NAK", nak}} {
		codes, reachedEnd := replyOptionCodes(t, reply.pkt)
		// RFC requirement: RFC2131-4.1-6 positive -- every option of the OFFER, the ACK and the DHCPNAK is walked by its length octet to the End option inside the options field, and the sname and file fields hold no option: they are all zero and no option overload (52) is sent.
		if !reachedEnd {
			t.Errorf("%s: the options field does not reach End inside the field", reply.name)
		}
		if codes[optOverload] {
			t.Errorf("%s carries option overload (52)", reply.name)
		}
		if !bytes.Equal(reply.pkt[snameStart:fileEnd], make([]byte, fileEnd-snameStart)) {
			t.Errorf("%s: the sname and file fields are not empty", reply.name)
		}
	}

	pxe := newTestPXEServer(t)
	defer pxe.leases.stop()
	pxe.pxe.BootfileBIOS = strings.Repeat("b", 300)
	long := pxe.handle(buildPXEDiscover(net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x34}, 0x42205, 0))
	if long == nil {
		t.Fatal("expected a DHCPOFFER, got nil")
	}
	longCodes, longEnd := replyOptionCodes(t, long)
	// RFC requirement: RFC2131-4.1-6 negative -- a bootfile name of 300 octets, which no option can hold, is not written as an option 67 whose length octet disagrees with its data: the reply walks to End through exactly the options the server sends, with no fragment of the name read as an option.
	if !longEnd {
		t.Error("the reply with an oversized bootfile does not reach End inside the options field")
	}
	want := map[byte]bool{
		optMessageType: true, optServerID: true, optSubnetMask: true, optRouter: true, optDNS: true,
		optLeaseTime: true, optT1: true, optT2: true, optVendorClassID: true, optTFTPServerName: true,
		optVendorSpecific: true,
	}
	for code := range longCodes {
		if !want[code] {
			t.Errorf("the reply carries option %d, which the server does not send", code)
		}
	}
	if got := getResponseOption(long, optBootfileName); got != nil {
		t.Errorf("option 67 carries %d octets of a 300-octet name", len(got))
	}
}

// TestRFC2131ConfiguredParameterValuesReturned proves that a parameter the client
// asks for is returned carrying the value the server is configured with.
// VALIDATES: RFC 2131 Section 4.3.1 -- "IF the server has been explicitly
// configured with a default value for the parameter, the server MUST include
// that value in an appropriate option in the 'option' field".
// PREVENTS: a wrong value of the right size (a DNS list checked only by length),
// and a server that returns the value the client suggested. Method: every
// requested parameter is compared byte for byte with the configured value, first
// for a plain request, then for a request that suggests other values for all four.
func TestRFC2131ConfiguredParameterValuesReturned(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	wantMask := []byte{255, 255, 255, 0}
	wantRouter := []byte{192, 168, 1, 1}
	wantDNS := []byte{8, 8, 8, 8, 8, 8, 4, 4}
	wantDomain := []byte("home.lan")
	prl := tlv(optParamReqList, optSubnetMask, optRouter, optDNS, optDomainName)
	check := func(reply []byte) []string {
		var wrong []string
		if !bytes.Equal(getResponseOption(reply, optSubnetMask), wantMask) {
			wrong = append(wrong, "subnet mask")
		}
		if !bytes.Equal(getResponseOption(reply, optRouter), wantRouter) {
			wrong = append(wrong, "router")
		}
		if !bytes.Equal(getResponseOption(reply, optDNS), wantDNS) {
			wrong = append(wrong, "DNS")
		}
		if !bytes.Equal(getResponseOption(reply, optDomainName), wantDomain) {
			wrong = append(wrong, "domain name")
		}
		return wrong
	}

	plain := h.handle(buildMsg(msgDiscover, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x35}, 0x42206, netip.Addr{}, 0, prl))
	if plain == nil {
		t.Fatal("expected a DHCPOFFER, got nil")
	}
	// RFC requirement: RFC2131-4.3.1-2 positive -- the requested subnet mask, router, DNS and domain-name parameters, each explicitly configured, are returned carrying exactly the configured values.
	if wrong := check(plain); len(wrong) > 0 {
		t.Errorf("OFFER does not carry the configured value for %v", wrong)
	}

	var suggested []byte
	suggested = append(suggested, prl...)
	suggested = append(suggested, tlv(optSubnetMask, 255, 255, 0, 0)...)
	suggested = append(suggested, tlv(optRouter, 10, 0, 0, 1)...)
	suggested = append(suggested, tlv(optDNS, 1, 1, 1, 1)...)
	suggested = append(suggested, tlv(optDomainName, []byte("client.example")...)...)
	hinted := h.handle(buildMsg(msgDiscover, net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x36}, 0x42207, netip.Addr{}, 0, suggested))
	if hinted == nil {
		t.Fatal("expected a DHCPOFFER, got nil")
	}
	// RFC requirement: RFC2131-4.3.1-2 negative -- a request that suggests its own subnet mask, router, DNS and domain-name values still receives the configured values, not the suggested ones.
	if wrong := check(hinted); len(wrong) > 0 {
		t.Errorf("OFFER to a client suggesting values does not carry the configured value for %v", wrong)
	}
}

// serverIDOf answers the server identifier (option 54) a reply carries,
// failing the test when it carries none or one of the wrong length.
func serverIDOf(t *testing.T, what string, resp []byte) netip.Addr {
	t.Helper()
	b := getResponseOption(resp, optServerID)
	if len(b) != 4 {
		t.Fatalf("%s: option 54 is %v, want four octets", what, b)
	}
	return netip.AddrFrom4([4]byte(b))
}

// withGiaddr sets the relay address of pkt, as a relay agent forwarding it
// does.
func withGiaddr(pkt []byte, giaddr netip.Addr) []byte {
	a := giaddr.As4()
	copy(pkt[24:28], a[:])
	return pkt
}

// noRouterLAN is sharedLANConfig with no default-router in either LAN subnet,
// so nothing in the configuration names an address of the server's.
func noRouterLAN(t *testing.T) []*dhcpHandler {
	t.Helper()
	cfg := sharedLANConfig(householdPool)
	for i := range cfg.SharedNetworks[1].Subnets {
		cfg.SharedNetworks[1].Subnets[i].DefaultRouter = netip.Addr{}
	}
	handlers := buildHandlers(cfg, nil)
	t.Cleanup(func() {
		for _, h := range handlers {
			h.leases.stop()
		}
	})
	return handlers
}

// relayedBranch is the configuration of a branch subnet 172.16.5.0/24 that
// the server reaches only through a relay at 172.16.5.1, beside the
// household subnet the listen device is addressed in.
func relayedBranch(t *testing.T) []*dhcpHandler {
	t.Helper()
	cfg := serverConfig{Enabled: true, SharedNetworks: []sharedNetwork{
		{Name: "branch", Subnets: []subnetConfig{{
			Prefix:       netip.MustParsePrefix("172.16.5.0/24"),
			Ranges:       []addressRange{{Name: "branch", Start: netip.MustParseAddr("172.16.5.100"), Stop: netip.MustParseAddr("172.16.5.199")}},
			LeaseTimeSec: 3600,
		}}},
		{Name: "lan", Subnets: []subnetConfig{{
			Prefix:       netip.MustParsePrefix("192.168.50.0/24"),
			Ranges:       []addressRange{householdPool},
			LeaseTimeSec: 3600,
		}}},
	}}
	handlers := buildHandlers(cfg, nil)
	t.Cleanup(func() {
		for _, h := range handlers {
			h.leases.stop()
		}
	})
	return handlers
}

// TestServerIdentifierIsSegmentAddress proves the server identifier is an
// address the server holds and the client reaches, never one derived from the
// configuration.
// VALIDATES: AC-8, AC-19: RFC 2131 Section 4.1 -- "To accommodate potentially
// incomplete network connectivity, a server MUST choose an address as a
// 'server identifier' that, to the best of the server's knowledge, is
// reachable from the client."
// PREVENTS: option 54 holding the first pool address, which the server leases
// to a client, or the subnet's network address, which is no host.
// Method: subnets with no default-router; the OFFER, ACK and NAK of each
// subnet of the segment name the device's address in that subnet, and a
// relayed OFFER for a subnet the device holds no address in names the device's
// primary address.
func TestServerIdentifierIsSegmentAddress(t *testing.T) {
	t.Parallel()

	seg := lanSegment(t, noRouterLAN(t))
	// wrong is an address of the client's subnet that is not the client's, so
	// its INIT-REBOOT REQUEST is refused by that subnet.
	cases := []struct {
		mac   net.HardwareAddr
		want  netip.Addr
		wrong netip.Addr
	}{
		{macPlain, netip.MustParseAddr("192.168.50.1"), netip.MustParseAddr("192.168.50.250")},     // household pool
		{macReservedB, netip.MustParseAddr("10.99.0.1"), netip.MustParseAddr("10.99.0.250")},       // device subnet reservation
		{macReservedA, netip.MustParseAddr("192.168.50.1"), netip.MustParseAddr("192.168.50.250")}, // household reservation
	}
	for i, c := range cases {
		xid := uint32(0x6100 + i)
		offer := seg.serve(buildMsg(msgDiscover, c.mac, xid, netip.Addr{}, 0, nil))
		expectReply(t, "DISCOVER from "+c.mac.String(), offer, msgOffer)
		offered := yiaddrOf(offer)
		ack := seg.serve(buildMsg(msgRequest, c.mac, xid, netip.Addr{}, 0,
			append(ipOpt(optRequestedIP, offered), ipOpt(optServerID, c.want)...)))
		expectReply(t, "SELECTING REQUEST from "+c.mac.String(), ack, msgAck)
		nak := seg.serve(buildMsg(msgRequest, c.mac, xid, netip.Addr{}, 0, ipOpt(optRequestedIP, c.wrong)))
		expectReply(t, "INIT-REBOOT REQUEST for another address from "+c.mac.String(), nak, msgNak)
		// RFC requirement: RFC2131-4.1-2 positive -- with no default-router configured, the DHCPOFFER, DHCPACK and DHCPNAK the server sends a client no relay forwarded carry as server identifier (option 54) the listen device's own address inside the subnet the client is served from: 192.168.50.1 for the household subnet, 10.99.0.1 for the device subnet.
		for _, r := range []struct {
			what string
			resp []byte
		}{{"OFFER", offer}, {"ACK", ack}, {"NAK", nak}} {
			if got := serverIDOf(t, r.what, r.resp); got != c.want {
				t.Errorf("%s to %s: option 54 = %v, want the device's address %v", r.what, c.mac, got, c.want)
			}
		}
	}

	branch := relayedBranch(t)
	device := []netip.Addr{netip.MustParseAddr("192.168.50.1")}
	giaddr := netip.MustParseAddr("172.16.5.1")
	relayedSeg, ok := resolveSegment(branch, giaddr, device)
	if !ok {
		t.Fatal("the relay's address resolved no segment")
	}
	offer := relayedSeg.serve(withGiaddr(buildMsg(msgDiscover, macPlain, 0x6110, netip.Addr{}, 0, nil), giaddr))
	expectReply(t, "relayed DISCOVER", offer, msgOffer)
	if got := yiaddrOf(offer); !netip.MustParsePrefix("172.16.5.0/24").Contains(got) {
		t.Fatalf("relayed OFFER of %v, want an address of 172.16.5.0/24", got)
	}
	// RFC requirement: RFC2131-4.1-2 negative -- a relayed DHCPOFFER for a subnet with no default-router that the listen device holds no address in carries as server identifier the device's primary address (192.168.50.1), never the subnet's network address, its first pool address or any other address inside the relayed subnet, none of which the server holds.
	got := serverIDOf(t, "relayed OFFER", offer)
	if got != device[0] {
		t.Errorf("relayed OFFER: option 54 = %v, want the device's primary address %v", got, device[0])
	}
	if netip.MustParsePrefix("172.16.5.0/24").Contains(got) {
		t.Errorf("relayed OFFER: option 54 %v is inside the relayed subnet, which the server holds no address in", got)
	}
}

// TestSelectingAcceptsAnyOwnAddress proves a SELECTING REQUEST is accepted
// whichever of the server's addresses on the segment it names, and only then.
// VALIDATES: AC-7: RFC 2131 Section 4.1 -- "A server with multiple network
// addresses MUST be prepared to to accept any of its network addresses as
// identifying that server in a DHCP message."
// PREVENTS: a REQUEST naming another of the server's own addresses silently
// dropped as if it selected another server.
// Method: the LAN device holds 192.168.50.1, 10.99.0.1 and 10.99.0.2, and the
// device subnet names 10.99.0.2 as its server-identifier. A 10.99.0.0/24 offer
// is requested naming each of the three in turn, then naming 192.0.2.9.
func TestSelectingAcceptsAnyOwnAddress(t *testing.T) {
	t.Parallel()

	cfg := sharedLANConfig(householdPool)
	cfg.SharedNetworks[1].Subnets[1].ServerIdentifier = netip.MustParseAddr("10.99.0.2")
	handlers := buildHandlers(cfg, nil)
	t.Cleanup(func() {
		for _, h := range handlers {
			h.leases.stop()
		}
	})
	device := []netip.Addr{netip.MustParseAddr("192.168.50.1"), netip.MustParseAddr("10.99.0.1"), netip.MustParseAddr("10.99.0.2")}
	seg, ok := resolveSegment(handlers, netip.IPv4Unspecified(), device)
	if !ok {
		t.Fatal("the LAN device's addresses resolved no segment")
	}

	reserved := netip.MustParseAddr("10.99.0.20")
	// RFC requirement: RFC2131-4.1-1 positive -- a SELECTING DHCPREQUEST for a 10.99.0.0/24 offer is acknowledged whichever of the server's addresses on the segment its server identifier names: the subnet's configured server-identifier 10.99.0.2, the device's other address in that subnet 10.99.0.1, and the device's address 192.168.50.1 in the other subnet of the segment.
	for i, own := range device {
		req := buildMsg(msgRequest, macReservedB, uint32(0x6200+i), netip.Addr{}, 0,
			append(ipOpt(optRequestedIP, reserved), ipOpt(optServerID, own)...))
		ack := seg.serve(req)
		expectReply(t, "SELECTING REQUEST naming "+own.String(), ack, msgAck)
		if got := yiaddrOf(ack); got != reserved {
			t.Errorf("ACK naming %v: yiaddr %v, want %v", own, got, reserved)
		}
	}

	// RFC requirement: RFC2131-4.1-1 negative -- a SELECTING DHCPREQUEST whose server identifier (192.0.2.9) is none of the server's addresses on the segment draws no reply and binds nothing, because the client selected another server.
	foreign := netip.MustParseAddr("192.0.2.9")
	asked := netip.MustParseAddr("10.99.0.77")
	req := buildMsg(msgRequest, macPlain, 0x6210, netip.Addr{}, 0,
		append(ipOpt(optRequestedIP, asked), ipOpt(optServerID, foreign)...))
	if resp := seg.serve(req); resp != nil {
		t.Errorf("REQUEST naming %v drew %s, want no reply", foreign, msgTypeName(getResponseMsgType(resp)))
	}
	if _, addr, bound := seg.binding(macPlain); bound {
		t.Errorf("REQUEST naming another server bound %v", addr)
	}
}

// TestServerIdentifierRelayOnlySubnet proves a relayed client of a subnet the
// listen device holds no address in is served and identified by the device's
// primary address, and that its REQUEST naming that address is accepted.
// VALIDATES: AC-19, AC-12.
// PREVENTS: a relay-only subnet left unanswered for want of a server address
// inside it, or its REQUEST dropped as addressed to another server.
func TestServerIdentifierRelayOnlySubnet(t *testing.T) {
	t.Parallel()

	device := []netip.Addr{netip.MustParseAddr("192.168.50.1")}
	giaddr := netip.MustParseAddr("172.16.5.1")
	seg, ok := resolveSegment(relayedBranch(t), giaddr, device)
	if !ok {
		t.Fatal("the relay's address resolved no segment")
	}
	offer := seg.serve(withGiaddr(buildMsg(msgDiscover, macPlain, 0x6300, netip.Addr{}, 0, nil), giaddr))
	expectReply(t, "relayed DISCOVER", offer, msgOffer)
	offered := yiaddrOf(offer)
	ack := seg.serve(withGiaddr(buildMsg(msgRequest, macPlain, 0x6300, netip.Addr{}, 0,
		append(ipOpt(optRequestedIP, offered), ipOpt(optServerID, device[0])...)), giaddr))
	expectReply(t, "relayed SELECTING REQUEST naming the primary address", ack, msgAck)
	if got := serverIDOf(t, "relayed ACK", ack); got != device[0] {
		t.Errorf("relayed ACK: option 54 = %v, want %v", got, device[0])
	}
	if got := netip.AddrFrom4([4]byte(ack[24:28])); got != giaddr {
		t.Errorf("relayed ACK: giaddr %v, want the relay %v", got, giaddr)
	}

	// AC-12: a relay inside a subnet the device is addressed in is answered
	// from that subnet, with the device's address in it.
	lan := sharedLAN(t, householdPool)
	relay := netip.MustParseAddr("10.99.0.254")
	lanSeg, ok := resolveSegment(lan, relay, lanAddrs)
	if !ok {
		t.Fatal("the relay 10.99.0.254 resolved no segment")
	}
	offer = lanSeg.serve(withGiaddr(buildMsg(msgDiscover, macPlain, 0x6301, netip.Addr{}, 0, nil), relay))
	expectReply(t, "DISCOVER relayed from 10.99.0.254", offer, msgOffer)
	if got := yiaddrOf(offer); !netip.MustParsePrefix("10.99.0.0/24").Contains(got) {
		t.Errorf("OFFER of %v, want an address of 10.99.0.0/24", got)
	}
	if got := serverIDOf(t, "OFFER relayed from 10.99.0.254", offer); got != netip.MustParseAddr("10.99.0.1") {
		t.Errorf("option 54 = %v, want the device's address 10.99.0.1", got)
	}
}

// TestServerIdentifierLeafNotHeld proves a configured server-identifier is
// sent only while the listen device holds it.
// VALIDATES: AC-9 runtime half: the leaf set and held fills option 54; set and
// not held, the request is not answered, no pool address is taken, and one
// warning names the subnet, the address and the fix.
// PREVENTS: a silent fallback to another address, or option 54 naming an
// address no client reaches.
func TestServerIdentifierLeafNotHeld(t *testing.T) {
	t.Parallel()

	cfg := sharedLANConfig(householdPool)
	configured := netip.MustParseAddr("10.99.0.9")
	cfg.SharedNetworks[1].Subnets[1].ServerIdentifier = configured
	handlers := buildHandlers(cfg, nil)
	t.Cleanup(func() {
		for _, h := range handlers {
			h.leases.stop()
		}
	})

	seg := lanSegment(t, handlers)
	var warnings []string
	seg.warn = func(msg string, attrs ...any) {
		line := msg
		for _, a := range attrs {
			if s, ok := a.(string); ok {
				line += " " + s
			}
		}
		warnings = append(warnings, line)
	}
	if resp := seg.serve(buildMsg(msgDiscover, macReservedB, 0x6400, netip.Addr{}, 0, nil)); resp != nil {
		t.Fatalf("DISCOVER with server-identifier %v not held drew %s, want no reply", configured, msgTypeName(getResponseMsgType(resp)))
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %q, want one", warnings)
	}
	for _, want := range []string{"10.99.0.0/24", "10.99.0.9", "server-identifier", "Fix:"} {
		if !strings.Contains(warnings[0], want) {
			t.Errorf("warning %q does not name %q", warnings[0], want)
		}
	}
	asked := netip.MustParseAddr("10.99.0.60")
	if resp := seg.serve(buildMsg(msgDiscover, macPlain, 0x6401, netip.Addr{}, 0, ipOpt(optRequestedIP, asked))); resp != nil {
		if got := yiaddrOf(resp); got == asked {
			t.Errorf("OFFER of %v from the subnet whose server-identifier is not held", got)
		}
	}
	if _, addr, ok := seg.held(macPlain); ok && addr == asked {
		t.Errorf("the subnet whose server-identifier is not held took %v for the client", addr)
	}

	held, ok := resolveSegment(handlers, netip.IPv4Unspecified(), append(append([]netip.Addr(nil), lanAddrs...), configured))
	if !ok {
		t.Fatal("the LAN device's addresses resolved no segment")
	}
	offer := held.serve(buildMsg(msgDiscover, macReservedB, 0x6402, netip.Addr{}, 0, nil))
	expectReply(t, "DISCOVER with server-identifier held", offer, msgOffer)
	if got := serverIDOf(t, "OFFER", offer); got != configured {
		t.Errorf("option 54 = %v, want the configured server-identifier %v", got, configured)
	}
}
