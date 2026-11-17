//go:build linux

// Design: docs/features/interfaces.md -- plugin-owned devices (bridges and links)
// Overview: owned_link_linux.go -- CreateOwnedLink under test
// Related: owned_bridge_linux.go -- CreateOwnedBridge, the bridge the host end joins
//
// These create a real owned bridge and a real link whose peer end is created
// inside a second network namespace, then read both ends back from the kernel.
// Each test runs userns.Enter first, which re-runs it as root over a user and
// network namespace of its own, so it needs no host privilege and a refused
// operation fails it. The reconcile-level proof (adoption after a restart) is
// in the iface component's owned_bridge_integration_linux_test.go, which can
// drive the unexported pass.

package ifacenetlink

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/test/userns"
)

const (
	ownedTestBridge  = "zol-net"
	ownedTestHostEnd = "zol-9-1"
	ownedTestAlias   = "ze:owned:test"
	ownedTestPeerMAC = "02:00:5e:00:53:01"
)

// newPeerNamespace creates an anonymous network namespace for a link's peer
// end and returns the calling thread to the namespace it was in. The file is
// the only reference that keeps the namespace alive: closing it destroys the
// namespace. The caller MUST be on a locked thread.
func newPeerNamespace(t *testing.T) *os.File {
	t.Helper()
	hostNS, err := netns.Get()
	if err != nil {
		t.Fatalf("read the host namespace: %v", err)
	}
	defer hostNS.Close() //nolint:errcheck // a read-only handle
	peerNS, err := netns.New()
	if err != nil {
		t.Fatalf("create the peer namespace: %v", err)
	}
	if err := netns.Set(hostNS); err != nil {
		t.Fatalf("return to the host namespace: %v", err)
	}
	return os.NewFile(uintptr(peerNS), "peer-netns")
}

// createOwnedTestLink makes the owned bridge and one link into namespace with
// both families' addresses and gateways, through the backend under test.
func createOwnedTestLink(t *testing.T, b *netlinkBackend, namespace *os.File) {
	t.Helper()
	bridge := iface.OwnedBridgeSpec{Name: ownedTestBridge, MTU: 1400, Alias: ownedTestAlias}
	if err := b.CreateOwnedBridge(bridge); err != nil {
		t.Fatalf("CreateOwnedBridge: %v", err)
	}
	for _, gateway := range []string{"192.0.2.1/24", "2001:db8::1/64"} {
		address, err := netlink.ParseAddr(gateway)
		if err != nil {
			t.Fatalf("parse %s: %v", gateway, err)
		}
		link, err := netlink.LinkByName(ownedTestBridge)
		if err != nil {
			t.Fatalf("read the bridge: %v", err)
		}
		address.Flags = unix.IFA_F_NODAD
		if err := netlink.AddrAdd(link, address); err != nil {
			t.Fatalf("address the bridge with %s: %v", gateway, err)
		}
	}
	mac, err := net.ParseMAC(ownedTestPeerMAC)
	if err != nil {
		t.Fatalf("parse mac: %v", err)
	}
	spec := iface.OwnedLinkSpec{
		HostEnd:       ownedTestHostEnd,
		Bridge:        ownedTestBridge,
		MTU:           1400,
		Alias:         ownedTestAlias,
		Namespace:     namespace,
		PeerName:      "eth0",
		PeerMAC:       mac,
		PeerAddresses: []netip.Prefix{netip.MustParsePrefix("192.0.2.2/24"), netip.MustParsePrefix("2001:db8::2/64")},
		PeerGateways:  []netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("2001:db8::1")},
	}
	if err := b.CreateOwnedLink(spec); err != nil {
		t.Fatalf("CreateOwnedLink: %v", err)
	}
}

// TestOwnedLinkIsCreatedInsideNamespace proves the host end is a marked veth
// member of the bridge, up, with the MTU, and that the peer end exists only
// inside the other namespace, under its name, with its hardware address, MTU,
// both addresses (the IPv6 one not tentative), both default routes and the up
// flag. Method: create through the backend, then read each namespace with a
// netlink handle of its own.
func TestOwnedLinkIsCreatedInsideNamespace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withMacvlanNetNS(t, func() {
		b := &netlinkBackend{}
		namespace := newPeerNamespace(t)
		t.Cleanup(func() { namespace.Close() }) //nolint:errcheck // closing destroys the test namespace
		createOwnedTestLink(t, b, namespace)

		bridge, err := netlink.LinkByName(ownedTestBridge)
		if err != nil {
			t.Fatalf("read the bridge: %v", err)
		}
		if bridge.Attrs().Alias != ownedTestAlias || bridge.Attrs().MTU != 1400 {
			t.Errorf("bridge alias %q mtu %d, want %q 1400", bridge.Attrs().Alias, bridge.Attrs().MTU, ownedTestAlias)
		}
		host, err := netlink.LinkByName(ownedTestHostEnd)
		if err != nil {
			t.Fatalf("read the host end: %v", err)
		}
		if host.Type() != "veth" {
			t.Errorf("host end type %q, want veth", host.Type())
		}
		if host.Attrs().Alias != ownedTestAlias {
			t.Errorf("host end alias %q, want %q", host.Attrs().Alias, ownedTestAlias)
		}
		if host.Attrs().MasterIndex != bridge.Attrs().Index {
			t.Errorf("host end master %d, want the bridge %d", host.Attrs().MasterIndex, bridge.Attrs().Index)
		}
		if host.Attrs().MTU != 1400 || host.Attrs().Flags&net.FlagUp == 0 {
			t.Errorf("host end mtu %d flags %v, want 1400 and up", host.Attrs().MTU, host.Attrs().Flags)
		}
		if _, err := netlink.LinkByName("eth0"); err == nil {
			t.Errorf("a device eth0 is visible in the host namespace")
		}

		handle, err := netlink.NewHandleAt(netns.NsHandle(int(namespace.Fd())), unix.NETLINK_ROUTE)
		if err != nil {
			t.Fatalf("open the peer namespace: %v", err)
		}
		defer handle.Close()
		peer, err := handle.LinkByName("eth0")
		if err != nil {
			t.Fatalf("no eth0 in the peer namespace: %v", err)
		}
		attrs := peer.Attrs()
		if attrs.HardwareAddr.String() != ownedTestPeerMAC || attrs.MTU != 1400 || attrs.Flags&net.FlagUp == 0 {
			t.Errorf("peer mac %s mtu %d flags %v, want %s 1400 up", attrs.HardwareAddr, attrs.MTU, attrs.Flags, ownedTestPeerMAC)
		}
		addresses, err := handle.AddrList(peer, netlink.FAMILY_ALL)
		if err != nil {
			t.Fatalf("list the peer's addresses: %v", err)
		}
		found := map[string]bool{}
		for _, address := range addresses {
			found[address.IPNet.String()] = true
			if address.IP.To4() == nil && address.Flags&unix.IFA_F_TENTATIVE != 0 && !address.IP.IsLinkLocalUnicast() {
				t.Errorf("peer address %s is tentative, want duplicate address detection off", address.IPNet)
			}
		}
		for _, want := range []string{"192.0.2.2/24", "2001:db8::2/64"} {
			if !found[want] {
				t.Errorf("peer lacks %s: %v", want, found)
			}
		}
		routes, err := handle.RouteList(peer, netlink.FAMILY_ALL)
		if err != nil {
			t.Fatalf("list the peer's routes: %v", err)
		}
		gateways := map[string]bool{}
		for _, route := range routes {
			if route.Dst == nil || route.Dst.IP.IsUnspecified() {
				if route.Gw != nil {
					gateways[route.Gw.String()] = true
				}
			}
		}
		for _, want := range []string{"192.0.2.1", "2001:db8::1"} {
			if !gateways[want] {
				t.Errorf("peer has no default route via %s: %v", want, gateways)
			}
		}
	})
}

// TestHostEndVanishesWithNamespace proves the kernel deletes the host end when
// the namespace holding the peer end is destroyed, so a link needs no call at
// container exit. Method: create the link, close the only reference to the
// peer namespace, and wait for the host end to leave the host namespace. The
// kernel tears a namespace down on a work queue, so the wait reads the state
// with a bound rather than once.
func TestHostEndVanishesWithNamespace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withMacvlanNetNS(t, func() {
		b := &netlinkBackend{}
		namespace := newPeerNamespace(t)
		createOwnedTestLink(t, b, namespace)
		if _, err := netlink.LinkByName(ownedTestHostEnd); err != nil {
			t.Fatalf("the host end is not there before the namespace ends: %v", err)
		}

		if err := namespace.Close(); err != nil {
			t.Fatalf("release the peer namespace: %v", err)
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			_, err := netlink.LinkByName(ownedTestHostEnd)
			var notFound netlink.LinkNotFoundError
			if err != nil && errors.As(err, &notFound) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("the host end %s outlived its peer's namespace by 10s (last read: %v)", ownedTestHostEnd, err)
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
}
