//go:build integration && linux

package iface

import (
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/vishvananda/netlink"
)

// startDHCPv4Server starts an in-process DHCPv4 server on the named interface
// that offers a fixed IP from the 10.77.0.0/24 range. The server responds to
// DISCOVER with OFFER and REQUEST with ACK.
//
// Returns the server (caller must close) and the offered IP.
func startDHCPv4Server(t *testing.T, ifaceName string) (*server4.Server, net.IP) {
	t.Helper()

	offeredIP := net.ParseIP("10.77.0.100")
	serverIP := net.ParseIP("10.77.0.1")
	subnetMask := net.IPMask(net.ParseIP("255.255.255.0").To4())

	handler := func(conn net.PacketConn, peer net.Addr, req *dhcpv4.DHCPv4) {
		var resp *dhcpv4.DHCPv4
		var err error

		switch req.MessageType() {
		case dhcpv4.MessageTypeDiscover:
			resp, err = dhcpv4.NewReplyFromRequest(req,
				dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
				dhcpv4.WithYourIP(offeredIP),
				dhcpv4.WithServerIP(serverIP),
				dhcpv4.WithOption(dhcpv4.OptSubnetMask(subnetMask)),
				dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(60*time.Second)),
				dhcpv4.WithOption(dhcpv4.OptRouter(serverIP)),
			)
		case dhcpv4.MessageTypeRequest:
			resp, err = dhcpv4.NewReplyFromRequest(req,
				dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
				dhcpv4.WithYourIP(offeredIP),
				dhcpv4.WithServerIP(serverIP),
				dhcpv4.WithOption(dhcpv4.OptSubnetMask(subnetMask)),
				dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(60*time.Second)),
				dhcpv4.WithOption(dhcpv4.OptRouter(serverIP)),
			)
		default:
			return // ignore other message types
		}

		if err != nil {
			t.Logf("dhcp server: error building response: %v", err)
			return
		}
		if _, err := conn.WriteTo(resp.ToBytes(), peer); err != nil {
			t.Logf("dhcp server: error sending response: %v", err)
		}
	}

	laddr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: dhcpv4.ServerPort,
	}
	srv, err := server4.NewServer(ifaceName, laddr, handler)
	if err != nil {
		t.Fatalf("NewServer on %q: %v", ifaceName, err)
	}

	go func() {
		if srvErr := srv.Serve(); srvErr != nil {
			// Server returns error when closed -- this is expected.
			t.Logf("dhcp server: serve returned: %v", srvErr)
		}
	}()

	return srv, offeredIP
}

func TestIntegrationDHCPv4Lease(t *testing.T) {
	// VALIDATES: DHCPClient obtains an IPv4 lease and installs it on the interface.
	// PREVENTS: DHCP negotiation fails against a real server or address not applied.
	withNetNS(t, func() {
		// Create a veth pair: srv0 (server side) and cli0 (client side).
		createVethForTest(t, "cli0", "srv0")

		// Assign the server IP to the server-side interface.
		if err := AddAddress("srv0", "10.77.0.1/24"); err != nil {
			t.Fatalf("AddAddress server: %v", err)
		}

		// Bring both sides up (CreateVeth already does this, but be explicit).
		link, err := netlink.LinkByName("srv0")
		if err != nil {
			t.Fatalf("LinkByName srv0: %v", err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatalf("LinkSetUp srv0: %v", err)
		}

		srv, offeredIP := startDHCPv4Server(t, "srv0")
		t.Cleanup(func() { srv.Close() })

		// Give server a moment to start.
		time.Sleep(200 * time.Millisecond)

		// Create the DHCP client on the client side.
		bus := &collectingBus{}
		client, err := NewDHCPClient("cli0", 0, bus, true, false)
		if err != nil {
			t.Fatalf("NewDHCPClient: %v", err)
		}
		if err := client.Start(); err != nil {
			t.Fatalf("DHCPClient.Start: %v", err)
		}
		t.Cleanup(func() { client.Stop() })

		// Wait for the lease-acquired event.
		ev := waitForEvent(t, bus, TopicDHCPLeaseAcquired, 30*time.Second)
		if ev.Topic != TopicDHCPLeaseAcquired {
			t.Errorf("event topic = %q, want %q", ev.Topic, TopicDHCPLeaseAcquired)
		}

		// Verify the offered address was installed on cli0.
		cidr := offeredIP.String() + "/24"
		if !hasAddress("cli0", cidr) {
			t.Errorf("address %s not found on cli0 after lease", cidr)
			// Dump actual addresses for diagnostics.
			cliLink, _ := netlink.LinkByName("cli0")
			if cliLink != nil {
				addrs, _ := netlink.AddrList(cliLink, netlink.FAMILY_V4)
				for _, a := range addrs {
					t.Logf("  cli0 addr: %s", a.IPNet.String())
				}
			}
		}
	})
}

func TestIntegrationDHCPv4Stop(t *testing.T) {
	// VALIDATES: Stopping DHCPClient removes the leased address.
	// PREVENTS: Leaked addresses after DHCP client shutdown.
	withNetNS(t, func() {
		createVethForTest(t, "cli0", "srv0")

		if err := AddAddress("srv0", "10.77.0.1/24"); err != nil {
			t.Fatalf("AddAddress server: %v", err)
		}

		srv, offeredIP := startDHCPv4Server(t, "srv0")
		t.Cleanup(func() { srv.Close() })

		time.Sleep(200 * time.Millisecond)

		bus := &collectingBus{}
		client, err := NewDHCPClient("cli0", 0, bus, true, false)
		if err != nil {
			t.Fatalf("NewDHCPClient: %v", err)
		}
		if err := client.Start(); err != nil {
			t.Fatalf("DHCPClient.Start: %v", err)
		}

		// Wait for lease.
		waitForEvent(t, bus, TopicDHCPLeaseAcquired, 30*time.Second)

		cidr := offeredIP.String() + "/24"
		if !hasAddress("cli0", cidr) {
			t.Fatalf("address %s not found on cli0 before stop", cidr)
		}

		// Stop the client -- it should remove the leased address.
		client.Stop()

		// Give a moment for cleanup.
		time.Sleep(500 * time.Millisecond)

		if hasAddress("cli0", cidr) {
			t.Errorf("address %s still present on cli0 after Stop", cidr)
		}
	})
}

// Note: TestIntegrationDHCPv6Lease is not included. The DHCPv6 server6 API
// requires link-local address setup, multicast group joins, and DUID
// configuration that make a reliable in-process test prohibitively complex
// within a network namespace. DHCPv6 is tested at the unit level via mocked
// nclient6 interactions. A proper integration test would require an external
// DHCPv6 server (e.g., dnsmasq in a container).
