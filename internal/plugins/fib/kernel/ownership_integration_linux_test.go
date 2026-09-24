//go:build integration && linux

// Design: docs/architecture/rib/unified-locrib.md -- a refused install never grants ownership.
package fibkernel

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/routewatch"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // real interface-name resolver
)

// startRouteMonitor uses an isolated watcher in the caller's network namespace.
// The recorder follows the FIB callback, so receiving an event also joins its
// recovery attempt. The caller MUST stop the monitor before closing its backend.
func startRouteMonitor(t *testing.T, writer *fibKernel, h *netlink.Handle) (<-chan routewatch.RouteEvent, func()) {
	t.Helper()
	w := routewatch.New()
	// Start on the namespace-pinned test thread before runMonitor's goroutine.
	w.Start(func(err error) { t.Logf("routewatch: %v", err) })
	ctx, cancel := context.WithCancel(t.Context())
	ready, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		writer.runMonitor(ctx, w, ready)
	}()
	<-ready
	events := make(chan routewatch.RouteEvent, 128)
	unreg := w.Register(func(ev routewatch.RouteEvent) {
		select {
		case events <- ev:
		case <-ctx.Done():
		}
	})
	stop := func() {
		cancel()
		<-done
		unreg()
		w.Stop()
		w.Wait()
	}
	t.Cleanup(stop)
	// Registration and socket subscription finish independently. A retained
	// marker reaches us either as a live update or through ListExisting.
	marker := routewatch.RouteEvent{
		Prefix: netip.MustParsePrefix("198.18.255.254/32"), Protocol: 100,
		TableID: 1001, Action: routewatch.ActionAdd,
	}
	markerRoute := &netlink.Route{
		Dst:      &net.IPNet{IP: marker.Prefix.Addr().AsSlice(), Mask: net.CIDRMask(32, 32)},
		Protocol: 100, Table: 1001, Type: unix.RTN_BLACKHOLE,
	}
	require.NoError(t, h.RouteAdd(markerRoute))
	awaitMonitorEvent(t, events, marker)
	require.NoError(t, h.RouteDel(markerRoute))
	marker.Action = routewatch.ActionRemove
	awaitMonitorEvent(t, events, marker)
	return events, stop
}

func awaitMonitorEvent(t *testing.T, events <-chan routewatch.RouteEvent, want routewatch.RouteEvent) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case ev := <-events:
			if ev.Prefix == want.Prefix && ev.Action == want.Action && ev.Protocol == want.Protocol &&
				ev.TableID == want.TableID && ev.Metric == want.Metric {
				return
			}
		case <-timer.C:
			t.Fatalf("monitor did not handle route event %+v", want)
		}
	}
}

// Exercise both backend entry points and both IP families: IPv6's omitted
// priority has different kernel semantics, but neither family may replace a
// foreign route after its initial exclusive add was refused.
func TestFIBIPRouteOwnership(t *testing.T) {
	for _, version := range []string{"ipv4", "ipv6"} {
		for _, mode := range []string{"plain", "rich"} {
			t.Run(version+"/"+mode, func(t *testing.T) {
				withNetNS(t, func() {
					h, err := netlink.NewHandle()
					require.NoError(t, err)
					defer h.Close()
					setupDummyLink(t, h)
					require.NoError(t, iface.LoadBackend("netlink"))
					t.Cleanup(func() { require.NoError(t, iface.CloseBackend()) })
					link, err := h.LinkByName("ze-mpls0")
					require.NoError(t, err)
					prefix, nextHop, otherHop := "198.51.100.0/24", "10.0.0.2", "10.0.0.3"
					family := netlink.FAMILY_V4
					if version == "ipv6" {
						prefix, nextHop, otherHop = "2001:db8:20::/64", "2001:db8:1::2", "2001:db8:1::3"
						family = netlink.FAMILY_V6
						address, err := netlink.ParseAddr("2001:db8:1::1/64")
						require.NoError(t, err)
						address.Flags = unix.IFA_F_NODAD
						require.NoError(t, h.AddrAdd(link, address))
					}
					_, cidr, err := net.ParseCIDR(prefix)
					require.NoError(t, err)
					entry := incomingChange{Action: routeaction.Add, Prefix: netip.MustParsePrefix(prefix), NextHop: netip.MustParseAddr(nextHop)}
					foreign := &netlink.Route{Dst: cidr, Gw: net.ParseIP(nextHop), LinkIndex: link.Attrs().Index, Protocol: 100, Table: unix.RT_TABLE_MAIN}
					if mode == "rich" {
						entry.Interface, entry.TableID, entry.Metric = link.Attrs().Name, 100, 25
						foreign.Table, foreign.Priority = 100, 25
					}
					require.NoError(t, h.RouteAdd(foreign))
					unrelated := *foreign
					unrelated.Priority = 5000
					require.NoError(t, h.RouteAdd(&unrelated))
					writer := newFIBKernel(newTestBackend(h))
					events, stopMonitor := startRouteMonitor(t, writer, h)
					defer stopMonitor()
					assertRoute := func(protocol netlink.RouteProtocol, gateway string) {
						t.Helper()
						routes, err := h.RouteListFiltered(family, &netlink.Route{Dst: cidr, Table: foreign.Table}, netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
						require.NoError(t, err)
						require.Len(t, routes, 2, "keep the foreign alternative without leaving a superseded Ze route")
						found := false
						for i := range routes {
							if routes[i].Priority == unrelated.Priority {
								require.Equal(t, unrelated.Protocol, routes[i].Protocol)
								require.True(t, routes[i].Gw.Equal(unrelated.Gw), "foreign alternative changed")
								continue
							}
							found = true
							require.Equal(t, protocol, routes[i].Protocol)
							require.True(t, routes[i].Gw.Equal(net.ParseIP(gateway)), "route gateway = %s, want %s", routes[i].Gw, gateway)
						}
						require.True(t, found, "expected route disappeared")
					}
					writer.processEvent(makeSysribPayload([]incomingChange{entry}))
					entry.Action, entry.NextHop = routeaction.Update, netip.MustParseAddr(otherHop)
					writer.processEvent(makeSysribPayload([]incomingChange{entry}))
					assertRoute(100, nextHop)
					entry.Action = routeaction.Withdraw
					writer.processEvent(makeSysribPayload([]incomingChange{entry}))
					assertRoute(100, nextHop)

					// Once the conflict is removed, an Update can acquire the empty
					// slot, and subsequent Ze-owned replacement remains supported.
					require.NoError(t, h.RouteDel(foreign))
					entry.Action = routeaction.Update
					writer.processEvent(makeSysribPayload([]incomingChange{entry}))
					assertRoute(rtprotZE, otherHop)
					entry.NextHop = netip.MustParseAddr(nextHop)
					writer.processEvent(makeSysribPayload([]incomingChange{entry}))
					assertRoute(rtprotZE, nextHop)
					ownedEvent := routewatch.RouteEvent{
						Prefix: entry.Prefix, Protocol: rtprotZE, TableID: uint32(foreign.Table),
						Metric: uint32(foreign.Priority), Action: routewatch.ActionAdd,
					}
					if version == "ipv6" && ownedEvent.Metric == 0 {
						ownedEvent.Metric = 1024
					}
					awaitMonitorEvent(t, events, ownedEvent)
					if mode == "rich" {
						entry.Metric = 50
						writer.processEvent(makeSysribPayload([]incomingChange{entry}))
						ownedEvent.Action = routewatch.ActionRemove
						awaitMonitorEvent(t, events, ownedEvent)
						ownedEvent.Metric = 50
						assertRoute(rtprotZE, nextHop)
						foreign.Priority = 50
					}

					require.NoError(t, h.RouteDel(&netlink.Route{
						Dst: cidr, Protocol: rtprotZE, Table: foreign.Table, Priority: int(ownedEvent.Metric),
					}))
					ownedEvent.Action = routewatch.ActionRemove
					awaitMonitorEvent(t, events, ownedEvent)
					assertRoute(rtprotZE, nextHop)

					// An old in-memory claim does not authorize overwriting a new
					// external owner. Update and Withdraw must both preserve it.
					require.NoError(t, h.RouteReplace(foreign))
					foreignEvent := ownedEvent
					foreignEvent.Protocol, foreignEvent.Action = 100, routewatch.ActionAdd
					awaitMonitorEvent(t, events, foreignEvent)
					assertRoute(100, nextHop)
					entry.NextHop = netip.MustParseAddr(otherHop)
					writer.processEvent(makeSysribPayload([]incomingChange{entry}))
					assertRoute(100, nextHop)
					entry.Action = routeaction.Withdraw
					writer.processEvent(makeSysribPayload([]incomingChange{entry}))
					assertRoute(100, nextHop)
				})
			})
		}
	}
}

func TestFIBRestartReplayRetainsOwnedRoute(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		setupDummyLink(t, h)
		require.NoError(t, iface.LoadBackend("netlink"))
		t.Cleanup(func() { require.NoError(t, iface.CloseBackend()) })
		link, err := h.LinkByName("ze-mpls0")
		require.NoError(t, err)
		entry := incomingChange{
			Action: routeaction.Add, Prefix: netip.MustParsePrefix("198.51.100.0/24"),
			NextHop: netip.MustParseAddr("10.0.0.2"), Interface: "ze-mpls0", OnLink: true,
			Weight: 2, Metric: 25,
			ECMPPaths: []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("10.0.0.3"), Interface: "ze-mpls0", OnLink: true, Weight: 2}},
		}
		first := newFIBKernel(newTestBackend(h))
		first.processEvent(makeSysribPayload([]incomingChange{entry}))
		addProtocolRoute(t, h, "198.51.101.0/24", "10.0.0.2", rtprotZE)
		addProtocolRoute(t, h, "198.51.102.0/24", "10.0.0.2", 100)

		restarted := newFIBKernel(newTestBackend(h))
		stale := restarted.startupSweep()
		events, stopMonitor := startRouteMonitor(t, restarted, h)
		defer stopMonitor()
		entry.ECMPPaths[0].Weight = 4
		restarted.processEvent(makeSysribPayload([]incomingChange{entry}))
		ownedEvent := routewatch.RouteEvent{
			Prefix: entry.Prefix, Protocol: rtprotZE, TableID: unix.RT_TABLE_MAIN,
			Metric: entry.Metric, Action: routewatch.ActionAdd,
		}
		awaitMonitorEvent(t, events, ownedEvent)
		restarted.sweepStale(stale)
		awaitMonitorEvent(t, events, routewatch.RouteEvent{
			Prefix: netip.MustParsePrefix("198.51.101.0/24"), Protocol: rtprotZE,
			TableID: unix.RT_TABLE_MAIN, Action: routewatch.ActionRemove,
		})
		// A caller may reuse its event buffer after the write. Reassertion must
		// retain the accepted weight, not observe that later mutation.
		entry.ECMPPaths[0].Weight = 20
		_, cidr, err := net.ParseCIDR(entry.Prefix.String())
		require.NoError(t, err)
		require.NoError(t, h.RouteDel(&netlink.Route{Dst: cidr, Protocol: rtprotZE}))
		ownedEvent.Action = routewatch.ActionRemove
		awaitMonitorEvent(t, events, ownedEvent)
		routes, err := h.RouteList(nil, netlink.FAMILY_V4)
		require.NoError(t, err)
		found := false
		foreign := false
		for i := range routes {
			if routes[i].Dst == nil {
				continue
			}
			switch routes[i].Dst.String() {
			case entry.Prefix.String():
				found = true
				require.Equal(t, netlink.RouteProtocol(rtprotZE), routes[i].Protocol)
				require.Equal(t, 25, routes[i].Priority)
				require.Equal(t, unix.RT_TABLE_MAIN, routes[i].Table)
				require.Len(t, routes[i].MultiPath, 2)
				for member := range routes[i].MultiPath {
					require.Equal(t, link.Attrs().Index, routes[i].MultiPath[member].LinkIndex)
					require.NotZero(t, routes[i].MultiPath[member].Flags&unix.RTNH_F_ONLINK)
				}
				require.True(t, routes[i].MultiPath[0].Gw.Equal(net.ParseIP("10.0.0.2")))
				require.True(t, routes[i].MultiPath[1].Gw.Equal(net.ParseIP("10.0.0.3")))
				require.Equal(t, 1, routes[i].MultiPath[0].Hops)
				require.Equal(t, 3, routes[i].MultiPath[1].Hops, "replay lost the updated member weight")
				require.NotZero(t, routes[i].MultiPath[1].Flags&unix.RTNH_F_ONLINK)
			case "198.51.101.0/24":
				t.Fatal("unrefreshed startup route survived the sweep")
			case "198.51.102.0/24":
				foreign = true
				require.Equal(t, netlink.RouteProtocol(100), routes[i].Protocol)
			}
		}
		require.True(t, found, "sweep removed the replayed route")
		require.True(t, foreign, "sweep removed an unrelated foreign route")

		// A producer withdrawal is consumed after processEvent retires its cache.
		entry.Action = routeaction.Withdraw
		restarted.processEvent(makeSysribPayload([]incomingChange{entry}))
		awaitMonitorEvent(t, events, ownedEvent)
		remaining, err := h.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Dst: cidr}, netlink.RT_FILTER_DST)
		require.NoError(t, err)
		require.Empty(t, remaining, "monitor restored a genuinely withdrawn route")
	})
}

func TestFIBDefaultRouteRecovery(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		setupDummyLink(t, h)
		require.NoError(t, iface.LoadBackend("netlink"))
		t.Cleanup(func() { require.NoError(t, iface.CloseBackend()) })
		writer := newFIBKernel(newTestBackend(h))
		events, stopMonitor := startRouteMonitor(t, writer, h)
		defer stopMonitor()

		for _, prefix := range []string{"0.0.0.0/0", "::/0"} {
			entry := incomingChange{
				Action: routeaction.Add, Prefix: netip.MustParsePrefix(prefix),
				Interface: "ze-mpls0",
			}
			writer.processEvent(makeSysribPayload([]incomingChange{entry}))
			family, priority := netlink.FAMILY_V4, 0
			if entry.Prefix.Addr().Is6() {
				family, priority = netlink.FAMILY_V6, 1024
			}
			event := routewatch.RouteEvent{
				Prefix: entry.Prefix, Protocol: rtprotZE, TableID: unix.RT_TABLE_MAIN,
				Metric: uint32(priority), Action: routewatch.ActionAdd,
			}
			awaitMonitorEvent(t, events, event)
			_, cidr, err := net.ParseCIDR(prefix)
			require.NoError(t, err)
			route := &netlink.Route{Dst: cidr, Protocol: rtprotZE, Priority: priority}
			require.NoError(t, h.RouteDel(route))
			event.Action = routewatch.ActionRemove
			awaitMonitorEvent(t, events, event)
			routes, err := h.RouteListFiltered(family, route, netlink.RT_FILTER_DST|netlink.RT_FILTER_PROTOCOL)
			require.NoError(t, err)
			require.Len(t, routes, 1, "default route was not restored")
			require.Equal(t, priority, routes[0].Priority)

			entry.Action = routeaction.Withdraw
			writer.processEvent(makeSysribPayload([]incomingChange{entry}))
			awaitMonitorEvent(t, events, event)
			routes, err = h.RouteListFiltered(family, route, netlink.RT_FILTER_DST|netlink.RT_FILTER_PROTOCOL)
			require.NoError(t, err)
			require.Empty(t, routes, "default route survived withdrawal")
		}
	})
}
