// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Related: dhcp_shortframe_integration_linux_test.go -- the veth lab and frame helpers
//
// VALIDATES: the real DHCPv4 client installs the leased address with the
// lifetime the ACK granted (a finite lease its seconds, the RFC 2131 Section
// 3.3 infinite value as the kernel's infinite lifetime), registers it as
// leased with the iface address registry while it holds it, and removes and
// unregisters it on Stop.
// PREVENTS: an infinite lease the reconcile cannot tell from a static address
// because nobody claimed it, and a registration that outlives the lease.

//go:build integration && linux

package ifacedhcp

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/mdlayher/packet"
	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
)

func TestDHCPv4LeaseLifetimeAndOwnership(t *testing.T) {
	cases := []struct {
		name  string
		lease time.Duration
		check func(t *testing.T, valid, preferred int)
	}{
		{"finite", 10 * time.Minute, func(t *testing.T, valid, preferred int) {
			t.Helper()
			if valid <= 0 || valid > 600 || preferred <= 0 || preferred > 600 {
				t.Errorf("finite 600s lease installed valid=%d preferred=%d, want both in (0, 600]", valid, preferred)
			}
		}},
		{"infinite", 0xffffffff * time.Second, func(t *testing.T, valid, preferred int) {
			t.Helper()
			if valid != 0xffffffff || preferred != 0xffffffff {
				t.Errorf("infinite lease installed valid=%#x preferred=%#x, want 0xffffffff for both", valid, preferred)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			zeSide, peerConn := sfSetupLab(t)
			client, err := newDHCPClient(zeSide, "0", &recordingBus{}, true, false, dHCPConfig{})
			if err != nil {
				t.Fatalf("newDHCPClient(%s): %v", zeSide, err)
			}
			if err := client.Start(); err != nil {
				t.Fatalf("Start: %v", err)
			}
			t.Cleanup(client.Stop)

			bindV4Lease(t, peerConn, tc.lease)
			const cidr = "192.0.2.50/24"
			addr := waitForV4Addr(t, zeSide, true)
			tc.check(t, addr.ValidLft, addr.PreferedLft)

			// The registry refuses a second owner and names the holder: the
			// lease is registered under the client's owner name.
			err = iface.RegisterOwnedAddresses(zeSide, "lease-probe", []string{cidr})
			if err == nil {
				iface.UnregisterOwnedAddresses("lease-probe")
				t.Fatalf("%s on %s is not registered as leased while the client holds it", cidr, zeSide)
			}
			if !strings.Contains(err.Error(), client.v4Owner) {
				t.Errorf("conflict names %v, want the client's owner %q", err, client.v4Owner)
			}

			client.Stop()
			waitForV4Addr(t, zeSide, false)
			if err := iface.RegisterOwnedAddresses(zeSide, "lease-probe", []string{cidr}); err != nil {
				t.Errorf("after Stop the lease is still registered: %v", err)
			}
			iface.UnregisterOwnedAddresses("lease-probe")
		})
	}
}

// bindV4Lease answers the client's DISCOVER and REQUEST with a lease of the
// given length for sfClientIP/24.
func bindV4Lease(t *testing.T, peerConn *packet.Conn, lease time.Duration) {
	t.Helper()
	reply := func(req *dhcpv4.DHCPv4, kind dhcpv4.MessageType) {
		t.Helper()
		msg, err := dhcpv4.NewReplyFromRequest(req,
			dhcpv4.WithMessageType(kind),
			dhcpv4.WithServerIP(sfServerIP),
			dhcpv4.WithYourIP(sfClientIP),
			dhcpv4.WithOption(dhcpv4.OptServerIdentifier(sfServerIP)),
			dhcpv4.WithOption(dhcpv4.OptSubnetMask(net.CIDRMask(24, 32))),
			dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(lease)),
		)
		if err != nil {
			t.Fatalf("build %v: %v", kind, err)
		}
		if err := sfWrite(peerConn, sfUDPFrame(msg.ToBytes())); err != nil {
			t.Fatalf("write %v: %v", kind, err)
		}
	}
	reply(sfReadDHCP(t, peerConn, dhcpv4.MessageTypeDiscover, 10*time.Second), dhcpv4.MessageTypeOffer)
	reply(sfReadDHCP(t, peerConn, dhcpv4.MessageTypeRequest, 10*time.Second), dhcpv4.MessageTypeAck)
}

// waitForV4Addr polls dev for sfClientIP and returns it once present (want)
// or once gone (!want), failing after five seconds.
func waitForV4Addr(t *testing.T, dev string, want bool) netlink.Addr {
	t.Helper()
	link, err := netlink.LinkByName(dev)
	if err != nil {
		t.Fatalf("LinkByName %s: %v", dev, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			t.Fatalf("AddrList %s: %v", dev, err)
		}
		found := -1
		for i := range addrs {
			if addrs[i].IP.Equal(sfClientIP) {
				found = i
			}
		}
		if want && found >= 0 {
			return addrs[found]
		}
		if !want && found < 0 {
			return netlink.Addr{}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%v on %s: present=%v not reached within 5s", sfClientIP, dev, want)
	return netlink.Addr{}
}
