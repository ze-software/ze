//go:build integration && linux

// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- Linux backend resolver
// Related: rfc2328_transport_integration_linux_test.go -- veth lab and namespace helpers

package transport

import (
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// unnumberedDonor is the router address the unnumbered device borrows. The
// donor device keeps it, and iface installs a /32 copy on the borrower.
const unnumberedDonor = "10.255.0.1"

// RFC requirement: RFC2328-8.1-1 positive -- on an unnumbered point-to-point device that holds only the /32 copy of an address another device of the router owns (the form iface installs for `ipv4 unnumbered`), the production backend opens the device, sets that donor address as the socket's multicast source, and the peer namespace receives the OSPF datagram with the donor address as its IP source; the device itself, not the donor, holds the AllSPFRouters membership, so the peer's reply arrives on it.
//
// VALIDATES: the IP source of OSPF packets on an unnumbered link is the donor.
// Method: a dummy device holds 10.255.0.1/32, the veth end holds the same /32
// and nothing else, and the peer end in its own namespace holds 10.255.0.2/32.
// The backend opens the veth end and sends to AllSPFRouters; the peer's OSPF
// socket reads the IP header source, then the peer answers on AllSPFRouters
// and the unnumbered end must receive it.
func TestOSPFUnnumberedSourcesFromDonorAddress(t *testing.T) {
	if !rawSocketAvailable() {
		t.Skip("CAP_NET_RAW unavailable")
	}
	loadIfaceBackend(t)
	donor := netip.MustParseAddr(unnumberedDonor)
	withVethPair(t, unnumberedDonor+"/32", "10.255.0.2/32", func(lab ospfVethLab) {
		addDonorDevice(t, unnumberedDonor+"/32")
		backend := NewBackend()
		ha, err := backend.OpenInterface(lab.nameA, nil)
		if err != nil {
			t.Fatalf("OpenInterface(%s) on the unnumbered device: %v", lab.nameA, err)
		}
		defer ha.Close() //nolint:errcheck // best-effort cleanup
		tx, ok := ha.(*linuxInterface)
		if !ok {
			t.Fatalf("handle type = %T, want *linuxInterface", ha)
		}
		selected, err := unix.GetsockoptInet4Addr(tx.txFD, unix.IPPROTO_IP, unix.IP_MULTICAST_IF)
		if err != nil {
			t.Fatalf("getsockopt IP_MULTICAST_IF: %v", err)
		}
		if selected != donor.As4() {
			t.Fatalf("multicast source = %v, want the donor %v", selected, donor)
		}

		var hb InterfaceHandle
		runInNS(t, lab.peerNS, func() {
			hb, err = backend.OpenInterface(lab.nameB, nil)
		})
		if err != nil {
			t.Fatalf("OpenInterface B: %v", err)
		}
		defer hb.Close() //nolint:errcheck // best-effort cleanup

		payload := []byte{2, 1, 0, 24, 10, 255, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		if err := ha.Send(AllSPFRouters, payload); err != nil {
			t.Fatalf("Send multicast: %v", err)
		}
		expectOSPFPacket(t, hb, donor, payload)

		reply := []byte{2, 1, 0, 24, 10, 255, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		if err := hb.Send(AllSPFRouters, reply); err != nil {
			t.Fatalf("peer Send multicast: %v", err)
		}
		expectOSPFPacket(t, ha, netip.MustParseAddr("10.255.0.2"), reply)
	})
}

// RFC requirement: RFC2328-8.1-1 negative -- in a namespace whose devices hold no IPv4 address at all, where a multicast route lets the kernel itself join AllSPFRouters on the point-to-point device, the production backend refuses to open that device and names the missing address, so no OSPF packet leaves a router that has no IP address to source it from.
//
// VALIDATES: "there must be at least one IP address assigned to the router".
// Method: the peer namespace holds no IPv4 address; a 224.0.0.0/4 route names
// the veth end, and a control raw socket proves the kernel accepts the
// AllSPFRouters membership there. The backend must still refuse the device.
func TestOSPFRefusesDeviceWithoutRouterAddress(t *testing.T) {
	if !rawSocketAvailable() {
		t.Skip("CAP_NET_RAW unavailable")
	}
	loadIfaceBackend(t)
	withVethPair(t, unnumberedDonor+"/32", "", func(lab ospfVethLab) {
		var err error
		var hb InterfaceHandle
		runInNS(t, lab.peerNS, func() {
			addMulticastRoute(t, lab.nameB)
			expectKernelJoinsWithoutAddress(t, lab.nameB)
			hb, err = NewBackend().OpenInterface(lab.nameB, nil)
		})
		if err == nil {
			hb.Close() //nolint:errcheck // best-effort cleanup
			t.Fatalf("OpenInterface(%s) opened a device in a router with no IPv4 address", lab.nameB)
		}
		if !strings.Contains(err.Error(), "has no IPv4 address") {
			t.Fatalf("OpenInterface(%s) error = %v, want it to name the missing IPv4 address", lab.nameB, err)
		}
	})
}

// addDonorDevice creates the dummy device that owns the donor address, as the
// loopback does on a router, and removes it at cleanup.
func addDonorDevice(t *testing.T, cidr string) {
	t.Helper()
	name := "zeospd" + strconv.Itoa(osPidSuffix())
	if err := netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}}); err != nil {
		t.Skipf("add dummy (needs CAP_NET_ADMIN): %v", err)
	}
	t.Cleanup(func() {
		if link, err := netlink.LinkByName(name); err == nil {
			_ = netlink.LinkDel(link)
		}
	})
	setupVethAddr(t, name, cidr)
}

// addMulticastRoute routes 224.0.0.0/4 out the named device, so the kernel can
// join a group there without an address. The route goes with the device.
func addMulticastRoute(t *testing.T, name string) {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", name, err)
	}
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       &net.IPNet{IP: net.IPv4(224, 0, 0, 0).To4(), Mask: net.CIDRMask(4, 32)},
		Scope:     netlink.SCOPE_LINK,
	}
	if err := netlink.RouteAdd(route); err != nil {
		t.Fatalf("add 224.0.0.0/4 dev %s: %v", name, err)
	}
}

// expectKernelJoinsWithoutAddress is the control: a raw OSPF socket bound to
// the device joins AllSPFRouters with no interface address, so a refusal from
// the backend is Ze's, not the kernel's.
func expectKernelJoinsWithoutAddress(t *testing.T, name string) {
	t.Helper()
	fd, err := openInterfaceSocket(name)
	if err != nil {
		t.Fatalf("control socket on %s: %v", name, err)
	}
	defer closeFD(fd)
	mreq := unix.IPMreq{Multiaddr: AllSPFRouters.As4()}
	if err := unix.SetsockoptIPMreq(fd, unix.IPPROTO_IP, unix.IP_ADD_MEMBERSHIP, &mreq); err != nil {
		t.Fatalf("control: the kernel refused AllSPFRouters on %s with no address: %v", name, err)
	}
}
