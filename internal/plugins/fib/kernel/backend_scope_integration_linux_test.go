//go:build integration && linux

// Design: docs/architecture/rib/unified-locrib.md -- device-only forwarding covers recursive gateways.
// Related: nexthop_linux.go -- the final netlink route determines its scope.

package fibkernel

import (
	"net"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/sysrib/events"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // Register the real interface-name resolver.
	"github.com/ze-software/ze/internal/test/userns"
)

// A cover learned through a device can prove a recursive gateway even when the
// gateway is outside the device's configured address subnet. The independent
// connected-gateway control proves that the namespace and backend are usable.
func TestKernelDeviceOnlyCoverResolvesGateway(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	for _, multipath := range []bool{false, true} {
		name := "single"
		if multipath {
			name = "multipath"
		}
		t.Run(name, func(t *testing.T) {
			withScopeBackend(t, func(backend *netlinkBackend, h *netlink.Handle, indices [2]int) {
				control := RichRoute{
					Prefix:  netip.MustParsePrefix("198.19.83.0/24"),
					NextHop: netip.MustParseAddr("198.18.83.1"), Interface: "ze-scope0",
					RouteType: events.RouteTypeUnicast,
				}
				if err := backend.addRichRoute(control); err != nil {
					t.Fatalf("install connected-gateway control: %v", err)
				}
				requireScopeForwarding(t, h, control, indices[0])

				cover := RichRoute{
					Prefix: netip.MustParsePrefix("10.248.83.0/24"), Interface: "ze-scope0",
					RouteType: events.RouteTypeUnicast, Metric: 11,
				}
				if multipath {
					cover.ECMPPaths = []events.ECMPPath{{Interface: "ze-scope1"}}
				}
				if err := backend.addRichRoute(cover); err != nil {
					t.Fatalf("install off-subnet device-only cover: %v", err)
				}
				dependent := RichRoute{
					Prefix:  netip.MustParsePrefix("198.19.84.0/24"),
					NextHop: netip.MustParseAddr("10.248.83.1"), Interface: "ze-scope0",
					RouteType: events.RouteTypeUnicast, Metric: 17,
				}
				if err := backend.addRichRoute(dependent); err != nil {
					t.Fatalf("device-only cover must admit its ordinary non-onlink gateway: %v", err)
				}
				requireScopeForwarding(t, h, dependent, indices[0])
				requireScopeForwarding(t, h, control, indices[0])
			})
		})
	}
}

// Scope follows the completed forwarding group. A gateway in an ECMP member
// requires universe scope even when the primary is device-only. A fast-reroute
// backup is not part of the group the kernel gets, so its gateway does not
// change the scope. IPv6
// retains universe scope, including routes that carry an encapsulation.
func TestBuildRichRouteScope(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withScopeBackend(t, func(_ *netlinkBackend, _ *netlink.Handle, _ [2]int) {
		v4 := netip.MustParsePrefix("198.19.85.0/24")
		v6 := netip.MustParsePrefix("2001:db8:85::/64")
		gateway4 := netip.MustParseAddr("198.18.83.1")
		gateway6 := netip.MustParseAddr("fe80::2")
		sid := netip.MustParseAddr("2001:db8:86::1")
		cases := []struct {
			name  string
			route RichRoute
			scope netlink.Scope
		}{
			{"IPv4 device", RichRoute{Prefix: v4, Interface: "ze-scope0"}, netlink.SCOPE_LINK},
			{"explicit unicast device", RichRoute{Prefix: v4, Interface: "ze-scope0", RouteType: events.RouteTypeUnicast}, netlink.SCOPE_LINK},
			{"IPv4 gateway", RichRoute{Prefix: v4, NextHop: gateway4, Interface: "ze-scope0"}, netlink.SCOPE_UNIVERSE},
			{"IPv6 device", RichRoute{Prefix: v6, Interface: "ze-scope0"}, netlink.SCOPE_UNIVERSE},
			{"IPv6 gateway", RichRoute{Prefix: v6, NextHop: gateway6, Interface: "ze-scope0"}, netlink.SCOPE_UNIVERSE},
			{"IPv4 through IPv6 Via", RichRoute{Prefix: v4, NextHop: gateway6, Interface: "ze-scope0"}, netlink.SCOPE_UNIVERSE},
			{"IPv4 device MPLS", RichRoute{Prefix: v4, Interface: "ze-scope0", Labels: []uint32{16000}}, netlink.SCOPE_LINK},
			{"IPv4 device SRv6", RichRoute{Prefix: v4, Interface: "ze-scope0", SRv6SID: sid}, netlink.SCOPE_LINK},
			{"IPv6 device MPLS", RichRoute{Prefix: v6, Interface: "ze-scope0", Labels: []uint32{16000}}, netlink.SCOPE_UNIVERSE},
			{"IPv6 device SRv6", RichRoute{Prefix: v6, Interface: "ze-scope0", SRv6SID: sid}, netlink.SCOPE_UNIVERSE},
			{"device ECMP", RichRoute{Prefix: v4, Interface: "ze-scope0", ECMPPaths: []events.ECMPPath{{Interface: "ze-scope1"}}}, netlink.SCOPE_LINK},
			{"device primary gateway ECMP", RichRoute{Prefix: v4, Interface: "ze-scope0", ECMPPaths: []events.ECMPPath{{NextHop: gateway4, Interface: "ze-scope1"}}}, netlink.SCOPE_UNIVERSE},
			{"gateway primary device ECMP", RichRoute{Prefix: v4, NextHop: gateway4, Interface: "ze-scope0", ECMPPaths: []events.ECMPPath{{Interface: "ze-scope1"}}}, netlink.SCOPE_UNIVERSE},
			{"device backup", RichRoute{Prefix: v4, Interface: "ze-scope0", Backup: []events.ECMPPath{{Interface: "ze-scope1"}}}, netlink.SCOPE_LINK},
			{"device primary, unprogrammed gateway backup", RichRoute{Prefix: v4, Interface: "ze-scope0", Backup: []events.ECMPPath{{NextHop: gateway4, Interface: "ze-scope1"}}}, netlink.SCOPE_LINK},
			{"gateway primary device backup", RichRoute{Prefix: v4, NextHop: gateway4, Interface: "ze-scope0", Backup: []events.ECMPPath{{Interface: "ze-scope1"}}}, netlink.SCOPE_UNIVERSE},
			{"Via primary device backup", RichRoute{Prefix: v4, NextHop: gateway6, Interface: "ze-scope0", Backup: []events.ECMPPath{{Interface: "ze-scope1"}}}, netlink.SCOPE_UNIVERSE},
			{"blackhole", RichRoute{Prefix: v4, NextHop: gateway4, Interface: "ze-scope0", RouteType: events.RouteTypeBlackhole}, netlink.SCOPE_UNIVERSE},
			{"unreachable", RichRoute{Prefix: v4, RouteType: events.RouteTypeUnreachable}, netlink.SCOPE_UNIVERSE},
			{"prohibit", RichRoute{Prefix: v4, RouteType: events.RouteTypeProhibit}, netlink.SCOPE_UNIVERSE},
		}
		for _, tc := range cases {
			route, err := buildRichRoute(tc.route)
			if err != nil {
				t.Errorf("%s: build route: %v", tc.name, err)
				continue
			}
			if route.Scope != tc.scope {
				t.Errorf("%s: scope = %v, want %v", tc.name, route.Scope, tc.scope)
			}
		}
	})
}

func withScopeBackend(t *testing.T, fn func(*netlinkBackend, *netlink.Handle, [2]int)) {
	t.Helper()
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatalf("open namespace netlink handle: %v", err)
		}
		defer h.Close()
		addLoopback(t, h)
		var indices [2]int
		for i, name := range []string{"ze-scope0", "ze-scope1"} {
			link := &netlink.Dummy{Name: name}
			if err := h.LinkAdd(link); err != nil {
				t.Fatalf("create %s: %v", name, err)
			}
			actual, err := h.LinkByName(name)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			if err := h.LinkSetUp(actual); err != nil {
				t.Fatalf("bring %s up: %v", name, err)
			}
			indices[i] = actual.Attrs().Index
			if i == 0 {
				address, err := netlink.ParseAddr("198.18.83.254/24")
				if err != nil {
					t.Fatal(err)
				}
				if err := h.AddrAdd(actual, address); err != nil {
					t.Fatalf("address %s: %v", name, err)
				}
			}
		}
		if err := iface.LoadBackend("netlink"); err != nil {
			t.Fatalf("load interface-name resolver: %v", err)
		}
		defer func() {
			if err := iface.CloseBackend(); err != nil {
				t.Errorf("close interface-name resolver: %v", err)
			}
		}()
		fn(&netlinkBackend{handle: h}, h, indices)
	})
}

func requireScopeForwarding(t *testing.T, h *netlink.Handle, want RichRoute, index int) {
	t.Helper()
	decision, err := forwardingDecision(t, h, want.Prefix.Addr().Next().String())
	if err != nil {
		t.Fatalf("forwarding decision for %s: %v", want.Prefix, err)
	}
	if decision.Dst == nil {
		t.Fatalf("forwarding decision for %s has no destination: %+v", want.Prefix, decision)
	}
	if decision.Dst.String() != want.Prefix.String() {
		t.Errorf("selected prefix = %s, want %s", decision.Dst, want.Prefix)
	}
	if !decision.Gw.Equal(net.IP(want.NextHop.AsSlice())) {
		t.Errorf("gateway for %s = %s, want %s", want.Prefix, decision.Gw, want.NextHop)
	}
	if decision.LinkIndex != index {
		t.Errorf("device for %s = %d, want %d", want.Prefix, decision.LinkIndex, index)
	}
	if decision.Protocol != rtprotZE {
		t.Errorf("protocol for %s = %d, want %d", want.Prefix, decision.Protocol, rtprotZE)
	}
	if decision.Flags&unix.RTNH_F_ONLINK != 0 {
		t.Errorf("ordinary gateway for %s acquired an on-link bypass", want.Prefix)
	}
}
