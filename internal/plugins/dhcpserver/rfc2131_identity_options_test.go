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

// BOOTP fixed fields that a server reply leaves unused (RFC 2131 Section 2).
const (
	snameStart = 44
	fileStart  = 108
	fileEnd    = 236
	// optOverload is the RFC 2132 Section 9.3 option that moves options into
	// the sname and file fields.
	optOverload = 52
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
