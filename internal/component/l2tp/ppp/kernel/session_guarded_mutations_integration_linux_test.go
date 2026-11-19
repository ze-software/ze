// Design: docs/research/l2tpv2-ze-integration.md -- actual SessionHandle writer

//go:build integration && linux

package kernel_test

import (
	"net"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/test/userns"
)

// The actual public session, actual creation backend and actual original unit
// are used here. No injected descriptor, feature receipt, ACK or mutation is
// supplied. Renaming and a live old-name survivor distinguish owned authority
// from the previous by-name backend path. Existing replacement-at-send tests
// remain mandatory alongside this useful full consumer control.
func TestSessionGuardedMutationsUseRenamedOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unit, backend, original := createdUnitNative(t, "zsg-original")
	provider, ok := backend.(iface.NativePPPBackend)
	if !ok {
		t.Fatal("actual original backend has no native PPP writer")
	}
	if _, err := unit.BindNativeOperations(t.Context(), backend, func(owner *pppkernel.CreatedUnit, index int, number int32) (pppkernel.NativeUnitOperations, error) {
		writer, err := provider.BindNativePPPUnit(owner, index, number)
		return writer, err
	}); err != nil {
		t.Fatalf("positive original native binding: %v", err)
	}
	channel := createdSessionChannel(t, "zsg-access", 37)
	cfg, err := unit.SessionConfig(channel)
	if err != nil {
		t.Fatal(err)
	}
	owner := createdSessionOpen(t, cfg)
	release := owner.(ppp.NativeReleaseHandle)
	if err := release.BindNativeReleaseSeal(guardedSessionNativeSeal{}); err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetName(original, "zsg-renamed"); err != nil {
		t.Fatal(err)
	}
	survivor := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "zsg-original", MTU: 1701}}
	if err := netlink.LinkAdd(survivor); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(survivor); err != nil {
			t.Error(err)
		}
	})
	if err := owner.SetMTU(1431); err != nil {
		t.Fatal(err)
	}
	if err := owner.SetAdminUp(); err != nil {
		t.Fatal(err)
	}
	actual, err := netlink.LinkByIndex(original.Attrs().Index)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Attrs().MTU != 1431 || actual.Attrs().Flags&net.FlagUp == 0 || actual.Attrs().Alias != "guarded-session-native" {
		t.Fatalf("original native state %+v", actual.Attrs())
	}
	// The device destination is what a RADIUS Framed-Route naming a gateway
	// other than the peer installs: a route through pppN with no gateway,
	// which the owner guard accepts (RTA_OIF, no RTA_GATEWAY, no rtm flag).
	for _, entry := range []struct{ local, peer, destination, deviceDestination string }{
		{"198.51.100.1/24", "198.51.100.2/24", "192.0.2.0/24", "203.0.113.128/25"},
		{"fe80::1/64", "fe80::2/64", "2001:db8:1234::/48", "2001:db8:5678::/48"},
	} {
		local, peer := netip.MustParsePrefix(entry.local), netip.MustParsePrefix(entry.peer)
		destination := netip.MustParsePrefix(entry.destination)
		if err := owner.AddAddressP2P(local, peer); err != nil {
			t.Fatal(err)
		}
		requireGuardedSessionPair(t, actual, local, peer, true)
		if err := owner.AddRoute(destination, peer.Addr(), 19, rtproto.Iface); err != nil {
			t.Fatal(err)
		}
		requireGuardedSessionRoute(t, actual, destination, peer.Addr(), true)
		if err := owner.RemoveRoute(destination, peer.Addr(), 19, rtproto.Iface); err != nil {
			t.Fatal(err)
		}
		requireGuardedSessionRoute(t, actual, destination, peer.Addr(), false)
		deviceDestination := netip.MustParsePrefix(entry.deviceDestination)
		if err := owner.AddRoute(deviceDestination, netip.Addr{}, 19, rtproto.Iface); err != nil {
			t.Fatalf("device route %s through the session interface was refused: %v", deviceDestination, err)
		}
		requireGuardedSessionRoute(t, actual, deviceDestination, netip.Addr{}, true)
		if err := owner.RemoveRoute(deviceDestination, netip.Addr{}, 19, rtproto.Iface); err != nil {
			t.Fatal(err)
		}
		requireGuardedSessionRoute(t, actual, deviceDestination, netip.Addr{}, false)
		if err := owner.RemoveAddress(local); err != nil {
			t.Fatal(err)
		}
		requireGuardedSessionPair(t, actual, local, peer, false)
	}
	kept, err := netlink.LinkByIndex(survivor.Attrs().Index)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Attrs().Name != "zsg-original" || kept.Attrs().MTU != 1701 || kept.Attrs().Alias != "" || kept.Attrs().Flags&net.FlagUp != 0 {
		t.Fatalf("old-name survivor was mutated: %+v", kept.Attrs())
	}
	addresses, err := netlink.AddrList(kept, netlink.FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	for _, address := range addresses {
		if address.IP.IsLinkLocalUnicast() {
			continue
		}
		t.Fatalf("old-name survivor acquired subscriber address: %+v", address)
	}
	if _, err := owner.Counters(); err != nil {
		t.Fatalf("final original accounting before Close: %v", err)
	}
	if err := owner.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if release.NativeReleasePending() {
		t.Fatal("confirmed original native close retained local custody")
	}
}

func requireGuardedSessionPair(t *testing.T, link netlink.Link, local, peer netip.Prefix, wanted bool) {
	t.Helper()
	addresses, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, address := range addresses {
		gotLocal, valid := netip.AddrFromSlice(address.IP)
		if !valid || gotLocal.Unmap() != local.Addr() || address.Peer == nil {
			continue
		}
		gotPeer, valid := netip.AddrFromSlice(address.Peer.IP)
		bits, _ := address.Peer.Mask.Size()
		if valid && gotPeer.Unmap() == peer.Addr() && bits == peer.Bits() {
			found = true
		}
	}
	if found != wanted {
		t.Fatalf("exact original pair %s %s present=%v wanted=%v", local, peer, found, wanted)
	}
}

type guardedSessionNativeSeal struct{}

func (guardedSessionNativeSeal) NativeBirthTag() string   { return "guarded-session-native" }
func (guardedSessionNativeSeal) SealNativeRelease() error { return nil }

func requireGuardedSessionRoute(t *testing.T, link netlink.Link, destination netip.Prefix, peer netip.Addr, wanted bool) {
	t.Helper()
	family := netlink.FAMILY_V6
	if destination.Addr().Is4() {
		family = netlink.FAMILY_V4
	}
	routes, err := netlink.RouteList(link, family)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, route := range routes {
		if route.Dst == nil || route.Dst.String() != destination.String() || route.LinkIndex != link.Attrs().Index {
			continue
		}
		// An invalid peer asks for a device route: no gateway at all.
		if peer.IsValid() {
			gateway, valid := netip.AddrFromSlice(route.Gw)
			if !valid || gateway.Unmap() != peer {
				continue
			}
		} else if len(route.Gw) != 0 {
			continue
		}
		if route.Priority == 19 && route.Protocol == netlink.RouteProtocol(rtproto.Iface) {
			found = true
		}
	}
	if found != wanted {
		t.Fatalf("exact original route %s via %s present=%v wanted=%v", destination, peer, found, wanted)
	}
}
