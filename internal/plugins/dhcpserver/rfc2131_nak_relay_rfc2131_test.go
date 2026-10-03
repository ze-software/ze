// Design: docs/architecture/provisioning/dhcp-server.md -- RFC 2131 conformance coverage
//
// Proves RFC 2131 Section 3.2 DHCPNAK delivery through a relay: a NAK answering
// a relayed REQUEST goes to the relay agent's address in giaddr, on the server
// port, and a NAK answering a directly connected client does not.

package dhcpserver

import (
	"net"
	"net/netip"
	"testing"
)

// TestNakDeliveredToRelayAgent drives a SELECTING REQUEST that names this
// server, carries no requested address, and was relayed (giaddr non-zero),
// then reads the destination responseAddr chooses for the resulting DHCPNAK.
//
// RFC requirement: RFC2131-3.2-4 positive — a DHCPNAK answering a REQUEST whose giaddr is non-zero is addressed to that giaddr on port 67
// RFC requirement: RFC2131-3.2-4 negative — a DHCPNAK answering a REQUEST whose giaddr is zero is not addressed to any relay agent: it goes to the broadcast address on port 68.
func TestNakDeliveredToRelayAgent(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	relay := netip.MustParseAddr("192.168.1.254")
	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x31}

	relayed := buildMsg(msgRequest, mac, 0x3201, netip.Addr{}, 0, ipOpt(optServerID, h.serverIP))
	ga := relay.As4()
	copy(relayed[24:28], ga[:])
	nak := h.handle(relayed)
	if nak == nil {
		t.Fatal("expected DHCPNAK for the relayed REQUEST, got nil")
	}
	if got := getResponseMsgType(nak); got != msgNak {
		t.Fatalf("message type = %d, want %d (NAK)", got, msgNak)
	}
	if got := netip.AddrFrom4([4]byte(nak[24:28])); got != relay {
		t.Fatalf("NAK giaddr = %s, want %s copied from the request", got, relay)
	}
	dst := responseAddr(relayed, nak)
	if dst.IP.String() != relay.String() || dst.Port != 67 {
		t.Errorf("relayed NAK destination = %s, want %s:67", dst, relay)
	}

	direct := nakForSelecting(t, h, mac, 0x3202)
	directReq := buildMsg(msgRequest, mac, 0x3202, netip.Addr{}, 0, ipOpt(optServerID, h.serverIP))
	dst = responseAddr(directReq, direct)
	if dst.IP.String() == relay.String() || dst.Port == 67 {
		t.Errorf("direct NAK destination = %s, must not be a relay agent on port 67", dst)
	}
	if !dst.IP.Equal(net.IPv4bcast) || dst.Port != 68 {
		t.Errorf("direct NAK destination = %s, want 255.255.255.255:68", dst)
	}
}
