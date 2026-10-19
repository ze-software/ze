// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Related: dhcp_v6_selector_renew_integration_linux_test.go -- the veth lab and the Go DHCPv6 server this test reuses
// RFC: rfc/short/rfc9915.md

//go:build integration && linux

package iface_test

import (
	"net"
	"sync"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

// TestRFC9915DuplicateAddressDeclinedOnTheLink leases the client an address
// the peer end already holds, so the kernel's duplicate address detection on
// the client's device fails for real.
//
// VALIDATES: AC-26 through the factory the interface plugin calls -- the
// client sees the kernel delete the tentative address (the kernel's verdict on
// a failed DAD for an address of finite lifetime), keeps it off the device and
// the lease, and sends
// a Decline to ff02::1:2 from its link-local address, carrying the server's
// DUID and one IA_NA with the address and no IA_PD.
// PREVENTS: a client that keeps a duplicate address installed and never tells
// the server.
func TestRFC9915DuplicateAddressDeclinedOnTheLink(t *testing.T) {
	// RFC requirement: RFC9915-18.2.10.1-1 positive -- a leased address another node on the link holds fails the kernel's DAD, and the client declines it to the server and removes it from the device.
	// RFC requirement: RFC9915-18.2.8-6 positive -- the Decline leaves from the device's link-local address, never from the declined address, which is no longer on the device.
	device, peer, peerNS, server := v6sbSetupLabNS(t)
	v6dadHoldOnPeer(t, peerNS, peer)
	iface.SetResolverMappingForTest(map[string]string{v6sbLogical: device})
	t.Cleanup(func() { iface.SetResolverMappingForTest(nil) })

	factory := iface.DHCPClientFactoryForTest()
	if factory == nil {
		t.Fatal("no DHCP client factory is installed: the ifacedhcp blank import did not run its init")
	}
	client, err := factory(v6sbLogical, "0", &v6sbBus{}, false, true, "", "", false, 0, "", 0)
	if err != nil {
		t.Fatalf("start the DHCPv6 client on %s: %v", v6sbLogical, err)
	}
	var stopOnce sync.Once
	t.Cleanup(func() { stopOnce.Do(client.Stop) })

	solicit := server.await(t, "the Solicit", dhcpv6.MessageTypeSolicit)
	server.replyLease(t, solicit)

	decline := server.await(t, "the Decline", dhcpv6.MessageTypeDecline, dhcpv6.MessageTypeRenew)
	if !decline.from.IP.IsLinkLocalUnicast() {
		t.Errorf("the Decline left from %v, want the device's link-local address", decline.from.IP)
	}
	if decline.from.IP.Equal(v6sbLeased) {
		t.Errorf("the Decline left from the declined address %v", v6sbLeased)
	}
	m := decline.msg
	if sid := m.Options.ServerID(); sid == nil || !sid.Equal(v6sbServerDUID) {
		t.Errorf("Decline Server ID %v, want %v", sid, v6sbServerDUID)
	}
	if !m.Options.ClientID().Equal(solicit.msg.Options.ClientID()) {
		t.Errorf("Decline Client ID %v, want the Solicit's %v", m.Options.ClientID(), solicit.msg.Options.ClientID())
	}
	if pds := m.Options.IAPD(); len(pds) != 0 {
		t.Errorf("Decline carries %d IA_PD, want none", len(pds))
	}
	na := m.Options.OneIANA()
	if na == nil {
		t.Fatal("the Decline carries no IA_NA")
	}
	if addrs := na.Options.Addresses(); len(addrs) != 1 || !addrs[0].IPv6Addr.Equal(v6sbLeased) {
		t.Errorf("Decline IA Addresses %v, want only %v", addrs, v6sbLeased)
	}
	if a := v6sbFindAddress(t, device); a != nil {
		t.Errorf("the declined address is still on %s (flags %#x)", device, a.Flags)
	}
	// The library builds no Reply to a Decline, so the test writes it.
	reply := &dhcpv6.Message{MessageType: dhcpv6.MessageTypeReply, TransactionID: m.TransactionID}
	reply.AddOption(dhcpv6.OptClientID(m.Options.ClientID()))
	reply.AddOption(dhcpv6.OptServerID(v6sbServerDUID))
	server.send(t, reply, decline.from)
}

// v6dadHoldOnPeer gives the peer end the address the server leases, exempt
// from DAD, so the peer answers the client's DAD probe for it.
func v6dadHoldOnPeer(t *testing.T, peerNS netns.NsHandle, peer string) {
	t.Helper()
	h, err := netlink.NewHandleAt(peerNS)
	if err != nil {
		t.Fatalf("netlink handle in the peer namespace: %v", err)
	}
	defer h.Close()

	link, err := h.LinkByName(peer)
	if err != nil {
		t.Fatalf("LinkByName(%s) in the peer namespace: %v", peer, err)
	}
	addr := &netlink.Addr{IPNet: &net.IPNet{IP: v6sbLeased, Mask: net.CIDRMask(128, 128)}, Flags: unix.IFA_F_NODAD}
	if err := h.AddrAdd(link, addr); err != nil {
		t.Fatalf("hold %v on %s: %v", v6sbLeased, peer, err)
	}
}
