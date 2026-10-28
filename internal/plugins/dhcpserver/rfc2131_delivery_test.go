// Design: docs/architecture/provisioning/dhcp-server.md -- RFC 2131 conformance coverage
//
// Proves the RFC 2131 Section 4.1 delivery of a server reply on the client's
// own subnet: a DHCPNAK is broadcast whenever giaddr is zero, and a DHCPOFFER
// to a client that left the BROADCAST bit clear reaches its hardware address,
// through the neighbor entry the server installs before the unicast.

package dhcpserver

import (
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestNakBroadcastWhenGiaddrZero drives a SELECTING REQUEST that names this
// server, carries no requested address and fills ciaddr, so the handler NAKs
// it, then reads the destination responseAddr chooses for that DHCPNAK. A
// RENEWING REQUEST with the same ciaddr is ACKed, and its ACK still goes to
// ciaddr, so the broadcast belongs to the NAK alone.
//
// RFC requirement: RFC2131-4.3.2-1 positive — a DHCPNAK answering a REQUEST whose giaddr is zero and whose ciaddr is non-zero is sent to 255.255.255.255:68, not to ciaddr.
// RFC requirement: RFC2131-4.3.2-1 negative — a DHCPACK answering a REQUEST with the same zero giaddr and non-zero ciaddr is not broadcast: it is unicast to ciaddr:68.
func TestNakBroadcastWhenGiaddrZero(t *testing.T) {
	t.Parallel()

	h := newTestServer(t)
	defer h.leases.stop()

	ciaddr := netip.MustParseAddr("192.168.1.77")
	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x41}

	selecting := buildMsg(msgRequest, mac, 0x4101, ciaddr, 0, ipOpt(optServerID, h.serverIP))
	nak := h.handle(selecting)
	if nak == nil {
		t.Fatal("expected DHCPNAK for the SELECTING REQUEST with no requested address, got nil")
	}
	if got := getResponseMsgType(nak); got != msgNak {
		t.Fatalf("message type = %d, want %d (NAK)", got, msgNak)
	}
	dst := responseAddr(selecting, nak)
	if !dst.IP.Equal(net.IPv4bcast) || dst.Port != 68 {
		t.Errorf("NAK destination = %s, want 255.255.255.255:68 (giaddr is zero, ciaddr is %s)", dst, ciaddr)
	}

	renewing := buildMsg(msgRequest, mac, 0x4102, ciaddr, 0, nil)
	ack := h.handle(renewing)
	if ack == nil {
		t.Fatal("expected DHCPACK for the RENEWING REQUEST, got nil")
	}
	if got := getResponseMsgType(ack); got != msgAck {
		t.Fatalf("message type = %d, want %d (ACK)", got, msgAck)
	}
	dst = responseAddr(renewing, ack)
	if dst.IP.String() != ciaddr.String() || dst.Port != 68 {
		t.Errorf("ACK destination = %s, want %s:68", dst, ciaddr)
	}
}

// neighborCall is one neighborInstall call a test observed.
type neighborCall struct {
	iface    string
	addr     netip.Addr
	hardware string
}

// recordNeighbors replaces neighborInstall for the test with a recorder that
// answers refusal, and neighborRead with a table holding no entry, and
// restores both at cleanup. Tests that call it MUST NOT run in parallel:
// neighborInstall and neighborRead are package state.
func recordNeighbors(t *testing.T, refusal error) *[]neighborCall {
	t.Helper()
	savedRead := neighborRead
	neighborRead = func(string, netip.Addr) (neighborEntry, error) { return neighborEntry{}, nil }
	t.Cleanup(func() { neighborRead = savedRead })
	calls := &[]neighborCall{}
	saved := neighborInstall
	neighborInstall = func(iface string, addr netip.Addr, hardware net.HardwareAddr) error {
		*calls = append(*calls, neighborCall{iface: iface, addr: addr, hardware: hardware.String()})
		return refusal
	}
	t.Cleanup(func() { neighborInstall = saved })
	return calls
}

// TestOfferReachesClientHardware drives a DISCOVER through the handler and
// replyDestination, the step serveRequests takes before it writes the reply.
// With the BROADCAST bit clear and giaddr and ciaddr zero, the OFFER goes to
// yiaddr:68 after the yiaddr -> chaddr neighbor entry is installed on the
// interface the request arrived on, because the client does not hold yiaddr
// and would never answer the ARP request for it. With the bit set, the OFFER
// is broadcast and no entry is written.
//
// RFC requirement: RFC2131-4.1-12 positive — with the BROADCAST bit set the OFFER is sent to 255.255.255.255:68 and no neighbor entry is installed.
// RFC requirement: RFC2131-4.1-12 negative — with the BROADCAST bit clear the OFFER is not broadcast: the yiaddr -> chaddr neighbor entry is installed on the receiving interface and the OFFER goes to yiaddr:68.
func TestOfferReachesClientHardware(t *testing.T) {
	calls := recordNeighbors(t, nil)

	h := newTestServer(t)
	defer h.leases.stop()
	log := slogutil.DiscardLogger()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x42}
	discover := buildMsg(msgDiscover, mac, 0x4201, netip.Addr{}, 0, nil)
	offer := h.handle(discover)
	if offer == nil {
		t.Fatal("expected DHCPOFFER, got nil")
	}
	yiaddr := netip.AddrFrom4([4]byte(offer[16:20]))

	dst := replyDestination("eth7", discover, offer, log)
	if dst.IP.String() != yiaddr.String() || dst.Port != 68 {
		t.Errorf("OFFER destination = %s, want %s:68", dst, yiaddr)
	}
	want := neighborCall{iface: "eth7", addr: yiaddr, hardware: mac.String()}
	if len(*calls) != 1 || (*calls)[0] != want {
		t.Errorf("neighbor entries installed = %+v, want exactly %+v", *calls, want)
	}

	*calls = (*calls)[:0]
	bcast := buildMsg(msgDiscover, mac, 0x4202, netip.Addr{}, 0x8000, nil)
	bcastOffer := h.handle(bcast)
	if bcastOffer == nil {
		t.Fatal("expected DHCPOFFER for the broadcast DISCOVER, got nil")
	}
	dst = replyDestination("eth7", bcast, bcastOffer, log)
	if !dst.IP.Equal(net.IPv4bcast) || dst.Port != 68 {
		t.Errorf("broadcast OFFER destination = %s, want 255.255.255.255:68", dst)
	}
	if len(*calls) != 0 {
		t.Errorf("broadcast OFFER installed neighbor entries %+v, want none", *calls)
	}
}

// TestOfferBroadcastWhenUnicastImpossible proves the fallback: when the
// neighbor entry cannot be installed (no CAP_NET_ADMIN, an interface gone, a
// chaddr the interface's link layer cannot hold), the OFFER is broadcast
// rather than unicast into an ARP request nobody answers.
func TestOfferBroadcastWhenUnicastImpossible(t *testing.T) {
	calls := recordNeighbors(t, errors.New("operation not permitted"))

	h := newTestServer(t)
	defer h.leases.stop()
	log := slogutil.DiscardLogger()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x43}
	discover := buildMsg(msgDiscover, mac, 0x4301, netip.Addr{}, 0, nil)
	offer := h.handle(discover)
	if offer == nil {
		t.Fatal("expected DHCPOFFER, got nil")
	}
	dst := replyDestination("eth7", discover, offer, log)
	if !dst.IP.Equal(net.IPv4bcast) || dst.Port != 68 {
		t.Errorf("refused install: destination = %s, want 255.255.255.255:68", dst)
	}
	if len(*calls) != 1 {
		t.Errorf("neighbor install attempts = %d, want 1", len(*calls))
	}
}
