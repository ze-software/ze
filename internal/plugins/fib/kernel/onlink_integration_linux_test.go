//go:build integration && linux

// Design: docs/architecture/rib/unified-locrib.md -- the selected forwarding target
// Related: ../../../component/sysrib/sysrib.go -- direct adjacency and ECMP resolution
// Related: nexthop_linux.go -- RTNH_F_ONLINK and RTN_UNREACHABLE programming
//
// These are kernel forwarding decisions, not netlink message-shape assertions.
// Paths enter the real Loc-RIB, pass through the running system RIB and its JSON
// event contract, and reach the real FIB writer. The input is the learned-route
// shape; this test does not stand in for IS-IS adjacency or SPF tests.
package fibkernel

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // real interface-name resolver
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RFC requirement: RFC1195-4.4-1 positive -- a learned IS-IS gateway outside the outgoing interface's logical subnet survives Loc-RIB, sysrib and kernel installation as the direct forwarding next hop.
// RFC requirement: RFC1195-4.4-1 negative -- a covering IP route cannot replace a next hop marked physically adjacent; clearing that mark exercises the different recursive result.
// RFC requirement: RFC1195-4.5-1 positive -- the selected unsupported-prefix route produces kernel EHOSTUNREACH even with a usable default route.
// RFC requirement: RFC1195-4.5-1 negative -- removing the terminal rejection restores forwarding via the same usable default rather than leaving a stale discard.
func TestFIBOnLinkAdjacencyAndUnsupportedPrefix(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		const device = "ze-onlink"
		link := &netlink.Dummy{Name: device}
		err = h.LinkAdd(link)
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("requires CAP_NET_ADMIN in the test network namespace to create %s: %v", device, err)
		}
		require.NoError(t, err)
		addLoopback(t, h)
		require.NoError(t, h.LinkSetUp(link))
		addr, err := netlink.ParseAddr("172.31.254.1/24")
		require.NoError(t, err)
		require.NoError(t, h.AddrAdd(link, addr))
		actual, err := h.LinkByName(device)
		require.NoError(t, err)
		ifindex := actual.Attrs().Index
		require.NoError(t, iface.LoadBackend("netlink"))
		t.Cleanup(func() { require.NoError(t, iface.CloseBackend()) })

		isis := redistevents.RegisterProtocol("isis")
		connected := redistevents.RegisterProtocol("connected")
		redistevents.RegisterOSInstalled(connected)
		ctx, cancel := context.WithTimeout(context.Background(), withholdWait)
		defer cancel()
		bus := newWithholdBus()
		setEventBus(bus)
		writer := newFIBKernel(newTestBackend(h))
		pending := make(chan *sysribevents.BestChangeBatch, 64)
		unsubscribe := sysribevents.BestChange.Subscribe(bus, func(batch *sysribevents.BestChangeBatch) {
			select {
			case pending <- batch:
			case <-ctx.Done():
			}
		})
		defer unsubscribe()
		startSysRIB(ctx, t, bus, rpc.ConfigSection{Root: sysribRoot, Data: "{}"})

		// The event queue is transport only. Run the actual writer on this
		// namespace-locked thread: iface's netlink name lookup uses its caller's
		// namespace, whereas the system-RIB worker runs on another thread.
		apply := func(prefix netip.Prefix, action routeaction.Action) {
			t.Helper()
			for {
				select {
				case batch := <-pending:
					wire, marshalErr := json.Marshal(batch)
					require.NoError(t, marshalErr)
					var received sysribevents.BestChangeBatch
					require.NoError(t, json.Unmarshal(wire, &received))
					writer.processEvent(&received)
					for _, change := range received.Changes {
						if change.Prefix == prefix && change.Action == action {
							return
						}
					}
				case <-ctx.Done():
					t.Fatalf("no %s for %s reached the real FIB writer: %v", action, prefix, ctx.Err())
				}
			}
		}

		routes := locrib.Default()
		require.NotNil(t, routes)
		subnet := netip.MustParsePrefix("172.31.254.0/24")
		cover := netip.MustParsePrefix("192.0.2.0/24")
		learned := netip.MustParsePrefix("198.51.100.0/24")
		unsupported := netip.MustParsePrefix("203.0.113.0/24")
		defaultRoute := netip.MustParsePrefix("0.0.0.0/0")
		t.Cleanup(func() {
			for _, prefix := range []netip.Prefix{cover, learned, unsupported, defaultRoute} {
				routes.Remove(family.IPv4Unicast, prefix, isis, 0)
				routes.Remove(family.IPv4Unicast, prefix, isis, 1)
			}
			routes.Remove(family.IPv4Unicast, subnet, connected, 0)
		})
		routes.Insert(family.IPv4Unicast, subnet, locrib.Path{Source: connected, Interface: device})
		gateway := netip.MustParseAddr("172.31.254.2")
		coverPath := locrib.Path{Source: isis, NextHop: gateway, Interface: device, AdminDistance: 115, Metric: 10}
		routes.Insert(family.IPv4Unicast, cover, coverPath)
		apply(cover, routeaction.Add)
		requireOnLinkDecision(t, h, "192.0.2.9", cover, gateway, ifindex, false)

		primary := coverPath
		primary.NextHop = netip.MustParseAddr("192.0.2.2")
		primary.OnLink = true
		primary.Weight = 3
		routes.Insert(family.IPv4Unicast, learned, primary)
		apply(learned, routeaction.Add)
		requireOnLinkDecision(t, h, "198.51.100.9", learned, primary.NextHop, ifindex, true)

		// Changing only OnLink changes the forwarding decision: without direct
		// adjacency semantics the covering route's gateway must be used.
		primary.OnLink = false
		routes.Insert(family.IPv4Unicast, learned, primary)
		apply(learned, routeaction.Update)
		requireOnLinkDecision(t, h, "198.51.100.9", learned, gateway, ifindex, false)
		primary.OnLink = true
		routes.Insert(family.IPv4Unicast, learned, primary)
		apply(learned, routeaction.Update)

		sibling := primary
		sibling.Instance = 1
		sibling.NextHop = netip.MustParseAddr("192.0.2.3")
		sibling.Weight = 2
		routes.Insert(family.IPv4Unicast, learned, sibling)
		apply(learned, routeaction.Update)
		requireOnLinkMultipath(t, h, learned, ifindex, primary.NextHop, sibling.NextHop, true)
		sibling.OnLink = false
		routes.Insert(family.IPv4Unicast, learned, sibling)
		apply(learned, routeaction.Update)
		requireOnLinkMultipath(t, h, learned, ifindex, primary.NextHop, gateway, false)

		// A best whose gateway has no covering route yields the prefix to its
		// usable sibling. The promoted sibling must keep its direct-link flag.
		sibling.OnLink = true
		routes.Insert(family.IPv4Unicast, learned, sibling)
		apply(learned, routeaction.Update)
		primary.OnLink = false
		primary.NextHop = netip.MustParseAddr("203.0.113.2")
		routes.Insert(family.IPv4Unicast, learned, primary)
		apply(learned, routeaction.Update)
		requireOnLinkDecision(t, h, "198.51.100.9", learned, sibling.NextHop, ifindex, true)

		// A usable default is the positive control. The more-specific
		// unsupported prefix must reject the packet, not disappear and expose
		// that default. Removing the rejection restores the same forwarding.
		routes.Insert(family.IPv4Unicast, defaultRoute, coverPath)
		apply(defaultRoute, routeaction.Add)
		before, err := h.RouteGet(net.ParseIP("203.0.113.9"))
		require.NoError(t, err)
		require.Len(t, before, 1)
		require.Equal(t, gateway.String(), before[0].Gw.String())
		require.Equal(t, ifindex, before[0].LinkIndex)
		routes.Insert(family.IPv4Unicast, unsupported, locrib.Path{
			Source: isis, AdminDistance: 115, RouteType: routetype.Unreachable,
		})
		apply(unsupported, routeaction.Add)
		_, err = h.RouteGet(net.ParseIP("203.0.113.9"))
		require.ErrorIs(t, err, unix.EHOSTUNREACH, "unsupported prefix fell through to the usable default")
		routes.Remove(family.IPv4Unicast, unsupported, isis, 0)
		apply(unsupported, routeaction.Withdraw)
		after, err := h.RouteGet(net.ParseIP("203.0.113.9"))
		require.NoError(t, err)
		require.Len(t, after, 1)
		require.Equal(t, gateway.String(), after[0].Gw.String())
		require.Equal(t, ifindex, after[0].LinkIndex)
	})
}

func requireOnLinkDecision(t *testing.T, h *netlink.Handle, destination string, prefix netip.Prefix, gateway netip.Addr, ifindex int, onLink bool) {
	t.Helper()
	route, err := forwardingDecision(t, h, destination)
	require.NoError(t, err)
	require.NotNil(t, route.Dst)
	require.Equal(t, prefix.String(), route.Dst.String())
	require.Equal(t, gateway.String(), route.Gw.String())
	require.Equal(t, ifindex, route.LinkIndex)
	require.Equal(t, unix.RTN_UNICAST, route.Type)
	require.Equal(t, netlink.RouteProtocol(rtprotZE), route.Protocol)
	flags := 0
	if onLink {
		flags = unix.RTNH_F_ONLINK
	}
	require.Equal(t, flags, route.Flags&unix.RTNH_F_ONLINK)
}

func requireOnLinkMultipath(t *testing.T, h *netlink.Handle, prefix netip.Prefix, ifindex int, primary, sibling netip.Addr, siblingOnLink bool) {
	t.Helper()
	route, err := forwardingDecision(t, h, "198.51.100.9")
	require.NoError(t, err)
	require.NotNil(t, route.Dst)
	require.Equal(t, prefix.String(), route.Dst.String())
	require.Equal(t, unix.RTN_UNICAST, route.Type)
	require.Equal(t, netlink.RouteProtocol(rtprotZE), route.Protocol)
	require.Len(t, route.MultiPath, 2)
	members := map[string]*netlink.NexthopInfo{}
	for _, member := range route.MultiPath {
		members[member.Gw.String()] = member
		require.Equal(t, ifindex, member.LinkIndex)
	}
	first, ok := members[primary.String()]
	require.True(t, ok, "primary adjacency missing from kernel multipath: %+v", route.MultiPath)
	require.Equal(t, unix.RTNH_F_ONLINK, first.Flags)
	require.Equal(t, 2, first.Hops)
	second, ok := members[sibling.String()]
	require.True(t, ok, "sibling adjacency missing from kernel multipath: %+v", route.MultiPath)
	flags := 0
	if siblingOnLink {
		flags = unix.RTNH_F_ONLINK
	}
	require.Equal(t, flags, second.Flags)
	require.Equal(t, 1, second.Hops)
}
