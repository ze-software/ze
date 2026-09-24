//go:build integration && linux

// Design: docs/architecture/rib/unified-locrib.md -- distinct per-member MPLS forwarding metadata.
package fibkernel

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/vishvananda/netlink"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/routewatch"
)

// The real FIB writer programs different stacks on two next hops. Varying the
// destination exercises both kernel hash buckets, and each received frame must
// use the label stack owned by the neighbor it addresses. A relabel repeats the
// same traffic without changing the first member.
func TestFIBECMPMemberLabelsOnWire(t *testing.T) {
	loadMPLSModules(t)
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		enableNetnsMPLS(t)
		bed := newMPLSTestbed(t, h)
		link, err := h.LinkByName(mplsZeLink)
		if err != nil {
			t.Fatal(err)
		}
		mplsMTUReturnRoutes(t, h, link)
		writer := newFIBKernel(newTestBackend(h))
		events, stopMonitor := startRouteMonitor(t, writer, h)
		defer stopMonitor()
		entry := sysribevents.BestChangeEntry{
			Action: routeaction.Add, Prefix: netip.MustParsePrefix("192.0.2.0/24"),
			NextHop: mplsNextHop, Labels: []uint32{200},
			ECMPPaths: []sysribevents.ECMPPath{{NextHop: mplsBypassHop, Weight: 1, Labels: []uint32{300, 400}}},
		}
		for _, memberLabels := range [][]uint32{{300, 400}, {500, 600}} {
			entry.ECMPPaths[0].Labels = memberLabels
			writer.processEvent(&sysribevents.BestChangeBatch{Family: family.IPv4Unicast, Changes: []sysribevents.BestChangeEntry{entry}})
			ownedEvent := routewatch.RouteEvent{
				Prefix: entry.Prefix, Protocol: rtprotZE, TableID: 254,
				Action: routewatch.ActionAdd,
			}
			awaitMonitorEvent(t, events, ownedEvent)
			// Recover through the live watcher before checking label bytes.
			routes, err := h.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Protocol: rtprotZE}, netlink.RT_FILTER_PROTOCOL)
			if err != nil {
				t.Fatal(err)
			}
			removed := false
			for i := range routes {
				if routes[i].Dst != nil && routes[i].Dst.String() == entry.Prefix.String() {
					if err := h.RouteDel(&routes[i]); err != nil {
						t.Fatal(err)
					}
					removed = true
				}
			}
			if !removed {
				t.Fatal("ECMP route never reached the kernel")
			}
			ownedEvent.Action = routewatch.ActionRemove
			awaitMonitorEvent(t, events, ownedEvent)
			seen := [2]bool{}
			for destination := byte(1); destination <= 128; destination++ {
				packet := ipv4UDP(netip.MustParseAddr("192.0.2.10"), netip.AddrFrom4([4]byte{192, 0, 2, destination}), 4000, []byte("ecmp-labels"))
				mplsMTUInject(bed, nil, packet, false)
				frame := bed.awaitForwarded("ECMP labeled packet", isMPLS)
				want := []uint32{200}
				switch {
				case slices.Equal(frame[:6], []byte(mplsNextMAC)):
					seen[0] = true
				case slices.Equal(frame[:6], []byte(mplsBypassMAC)):
					seen[1] = true
					want = memberLabels
				default:
					t.Fatalf("unknown ECMP destination MAC: %x", frame[:6])
				}
				if labels := labelStack(frame); !slices.Equal(labels, want) {
					t.Fatalf("neighbor %x received labels %v, want %v", frame[:6], labels, want)
				}
			}
			if !seen[0] || !seen[1] {
				t.Fatalf("kernel did not forward over both ECMP members: %v", seen)
			}
			entry.Action = routeaction.Update
		}
		entry.Action = routeaction.Withdraw
		writer.processEvent(&sysribevents.BestChangeBatch{Family: family.IPv4Unicast, Changes: []sysribevents.BestChangeEntry{entry}})
		routes, err := h.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Protocol: rtprotZE}, netlink.RT_FILTER_PROTOCOL)
		if err != nil {
			t.Fatal(err)
		}
		for _, route := range routes {
			if route.Dst != nil && route.Dst.String() == entry.Prefix.String() {
				t.Fatalf("withdrawal retained ECMP forwarding: %+v", route)
			}
		}
	})
}
