// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Related: dhcp_shortframe_integration_linux_test.go -- the veth lab and frame helpers
//
// VALIDATES: RFC 2131 Section 3.6 -- "A client with multiple network interfaces
// must use DHCP through each interface independently to obtain configuration
// information parameters for those separate interfaces."
// PREVENTS: a client that runs one exchange for the host, or applies the lease
// one interface obtained to another interface.

//go:build integration && linux

package ifacedhcp

import (
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/mdlayher/packet"
	"github.com/vishvananda/netlink"
)

// TestRFC2131ClientRunsDHCPOnEachInterfaceIndependently drives two real clients
// on two veth pairs. Method: each lab answers its own interface's DISCOVER and
// REQUEST with a different address; each interface must then carry the address
// its own exchange leased, and never the other interface's.
func TestRFC2131ClientRunsDHCPOnEachInterfaceIndependently(t *testing.T) {
	labs := []struct {
		name   string
		leased net.IP
		zeSide string
		peer   *packet.Conn
	}{
		{name: "first", leased: net.IP{192, 0, 2, 50}},
		{name: "second", leased: net.IP{192, 0, 2, 60}},
	}

	for i := range labs {
		labs[i].zeSide, labs[i].peer = sfSetupLab(t)
		client, err := newDHCPClient(labs[i].zeSide, "0", &recordingBus{}, true, false, dHCPConfig{})
		if err != nil {
			t.Fatalf("newDHCPClient(%s): %v", labs[i].zeSide, err)
		}
		if err := client.Start(); err != nil {
			t.Fatalf("Start(%s): %v", labs[i].zeSide, err)
		}
		t.Cleanup(client.Stop)
	}

	for i := range labs {
		mimServeLease(t, labs[i].peer, labs[i].leased)
	}

	for i := range labs {
		other := labs[1-i]
		// RFC requirement: RFC2131-3.6-1 positive -- two clients on two interfaces each complete their own DORA exchange, and each interface carries the address its own exchange leased and not the address leased on the other interface.
		mimWaitAddress(t, labs[i].zeSide, labs[i].leased)
		if mimHasAddress(t, labs[i].zeSide, other.leased) {
			t.Errorf("%s interface %s carries %v, leased on %s", labs[i].name, labs[i].zeSide, other.leased, other.zeSide)
		}
	}
}

// mimServeLease answers one client's DISCOVER with an OFFER of leased, then its
// REQUEST with an ACK of the same address.
func mimServeLease(t *testing.T, peer *packet.Conn, leased net.IP) {
	t.Helper()

	discover := sfReadDHCP(t, peer, dhcpv4.MessageTypeDiscover, 10*time.Second)
	mimReply(t, peer, discover, dhcpv4.MessageTypeOffer, leased)
	request := sfReadDHCP(t, peer, dhcpv4.MessageTypeRequest, 10*time.Second)
	mimReply(t, peer, request, dhcpv4.MessageTypeAck, leased)
}

// mimReply writes one server reply of type kind, leasing leased, to the client
// that sent request.
func mimReply(t *testing.T, peer *packet.Conn, request *dhcpv4.DHCPv4, kind dhcpv4.MessageType, leased net.IP) {
	t.Helper()

	reply, err := dhcpv4.NewReplyFromRequest(request,
		dhcpv4.WithMessageType(kind),
		dhcpv4.WithServerIP(sfServerIP),
		dhcpv4.WithYourIP(leased),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(sfServerIP)),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(net.CIDRMask(24, 32))),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(10*time.Minute)),
	)
	if err != nil {
		t.Fatalf("build %s: %v", kind, err)
	}
	if err := sfWrite(peer, sfUDPFrame(reply.ToBytes())); err != nil {
		t.Fatalf("write %s: %v", kind, err)
	}
}

// mimWaitAddress fails the test unless name carries want within ten seconds.
// The lease is installed by the client's own goroutine after the ACK, so the
// address appears some time after the last frame is written.
func mimWaitAddress(t *testing.T, name string, want net.IP) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if mimHasAddress(t, name, want) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("interface %s never carried the leased address %v", name, want)
}

// mimHasAddress reports whether the kernel holds want on the interface name.
func mimHasAddress(t *testing.T, name string, want net.IP) bool {
	t.Helper()

	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", name, err)
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("AddrList(%s): %v", name, err)
	}
	for _, addr := range addrs {
		if addr.IP.Equal(want) {
			return true
		}
	}
	return false
}
