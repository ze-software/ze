//go:build integration && linux

// Design: docs/architecture/core-design.md -- resolved gateways reach the Linux FIB.
// Related: ../../../component/sysrib/nhresolver.go -- recursive terminal adjacency
// Related: nexthop_linux.go -- destination-aware RTA_GATEWAY and RTA_VIA encoding
package fibkernel

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/nexthop"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink" // real interface-name resolver
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A recursive IPv4 route first resolves to one IPv6 link-local gateway, then
// to a weighted group using the same gateway address on distinct interfaces.
// Kernel FIB lookups must select both scoped targets, including in a mixed
// IPv4/IPv6 gateway group. Every route passes through the running system RIB
// and the actual FIB writer, with no direct invocation of the netlink builder.
func TestFIBRecursiveIPv4ViaIPv6(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		addLoopback(t, h)
		devices := []string{"ze-via-a", "ze-via-b"}
		indices := make([]int, len(devices))
		for index, device := range devices {
			link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: device}}
			err = h.LinkAdd(link)
			if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
				t.Skipf("requires CAP_NET_ADMIN to create %s: %v", device, err)
			}
			require.NoError(t, err)
			require.NoError(t, h.LinkSetUp(link))
			address, parseErr := netlink.ParseAddr("fe80::1/64")
			require.NoError(t, parseErr)
			address.Flags = unix.IFA_F_NODAD
			err = h.AddrAdd(link, address)
			if errors.Is(err, unix.EAFNOSUPPORT) || errors.Is(err, unix.EACCES) {
				t.Skipf("requires enabled IPv6 on %s: %v", device, err)
			}
			require.NoError(t, err)
			actual, lookupErr := h.LinkByName(device)
			require.NoError(t, lookupErr)
			indices[index] = actual.Attrs().Index
			address, parseErr = netlink.ParseAddr([]string{"172.31.253.1/24", "172.31.254.1/24"}[index])
			require.NoError(t, parseErr)
			require.NoError(t, h.AddrAdd(link, address))
		}
		firstLink, err := h.LinkByName(devices[0])
		require.NoError(t, err)
		globalAddress, err := netlink.ParseAddr("2001:db8:200::1/64")
		require.NoError(t, err)
		globalAddress.Flags = unix.IFA_F_NODAD
		require.NoError(t, h.AddrAdd(firstLink, globalAddress))
		require.NoError(t, iface.LoadBackend("netlink"))
		t.Cleanup(func() { require.NoError(t, iface.CloseBackend()) })

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

		// Interface lookup uses the calling thread's namespace. Deliver the
		// sysrib JSON payload to the writer on this namespace-locked thread.
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
					t.Fatalf("no %s for %s reached the FIB writer: %v", action, prefix, ctx.Err())
				}
			}
		}

		routes := locrib.Default()
		require.NotNil(t, routes)
		protocol := redistevents.RegisterProtocol("static")
		connected := redistevents.RegisterProtocol("connected")
		redistevents.RegisterOSInstalled(connected)
		covers := []netip.Prefix{netip.MustParsePrefix("192.0.2.11/32"), netip.MustParsePrefix("192.0.2.12/32")}
		prefix := netip.MustParsePrefix("198.51.100.0/24")
		ipv6Prefix := netip.MustParsePrefix("2001:db8:100::/64")
		plainPrefix := netip.MustParsePrefix("203.0.113.0/24")
		globalSubnet := netip.MustParsePrefix("2001:db8:200::/64")
		t.Cleanup(func() {
			routes.Remove(family.IPv4Unicast, prefix, protocol, 0)
			for _, cover := range covers {
				routes.Remove(family.IPv4Unicast, cover, protocol, 0)
			}
			routes.Remove(family.IPv6Unicast, ipv6Prefix, protocol, 0)
			routes.Remove(family.IPv4Unicast, plainPrefix, protocol, 0)
			routes.Remove(family.IPv6Unicast, globalSubnet, connected, 0)
		})
		gateway := netip.MustParseAddr("fe80::2")
		for index, cover := range covers {
			routes.Insert(family.IPv4Unicast, cover, locrib.Path{
				Source: protocol, NextHop: gateway, Interface: devices[index], OnLink: true,
				AdminDistance: 1, Metric: 10,
			})
			apply(cover, routeaction.Add)
		}
		path := locrib.Path{
			Source: protocol, NextHop: covers[0].Addr(), AdminDistance: 1, Metric: 77, Weight: 3,
		}
		routes.Insert(family.IPv4Unicast, prefix, path)
		apply(prefix, routeaction.Add)
		route := crossFamilyDecision(t, h, "198.51.100.9", prefix, 77)
		requireViaGateway(t, route.Gw, route.Via, gateway)
		require.Equal(t, indices[0], route.LinkIndex)
		require.Equal(t, unix.RTNH_F_ONLINK, route.Flags&unix.RTNH_F_ONLINK)

		path.ECMP = []nexthop.NextHop{{Addr: covers[1].Addr(), Weight: 2}}
		routes.Insert(family.IPv4Unicast, prefix, path)
		apply(prefix, routeaction.Update)
		requireScopedGroup := func() {
			t.Helper()
			group := crossFamilyDecision(t, h, "198.51.100.9", prefix, 77)
			require.Len(t, group.MultiPath, 2)
			members := make(map[int]*netlink.NexthopInfo, 2)
			for _, member := range group.MultiPath {
				requireViaGateway(t, member.Gw, member.Via, gateway)
				require.Equal(t, unix.RTNH_F_ONLINK, member.Flags&unix.RTNH_F_ONLINK)
				members[member.LinkIndex] = member
			}
			for index, ifindex := range indices {
				require.Contains(t, members, ifindex, "same-address scoped member was lost")
				require.Equal(t, 2-index, members[ifindex].Hops)
			}
		}
		requireScopedGroup()

		// An unusable member must fail the update and preserve the previous
		// complete group, rather than silently install only the usable paths.
		previousFailures := report.Errors(1)
		path.ECMP = append(path.ECMP, nexthop.NextHop{
			Addr: netip.MustParseAddr("fe80::3"), Interface: "ze-via-missing", OnLink: true,
		})
		routes.Insert(family.IPv4Unicast, prefix, path)
		apply(prefix, routeaction.Update)
		requireScopedGroup()
		failures := report.Errors(1)
		require.Len(t, failures, 1)
		if len(previousFailures) > 0 {
			require.True(t, failures[0].Raised.After(previousFailures[0].Raised), "failed update emitted no new error")
		}
		require.Equal(t, reportSourceFIB, failures[0].Source)
		require.Equal(t, reportCodeFIBSyncFailure, failures[0].Code)
		require.Equal(t, prefix.String(), failures[0].Subject)

		// Replacing the bad member with an IPv4 gateway exercises both wire
		// attributes inside the same multipath request.
		ipv4Gateway := netip.MustParseAddr("192.0.2.2")
		path.ECMP = []nexthop.NextHop{
			{Addr: covers[1].Addr(), Weight: 2},
			{Addr: ipv4Gateway, Interface: devices[0], OnLink: true, Weight: 4},
		}
		routes.Insert(family.IPv4Unicast, prefix, path)
		apply(prefix, routeaction.Update)
		route = crossFamilyDecision(t, h, "198.51.100.9", prefix, 77)
		require.Len(t, route.MultiPath, 3)
		var sameFamily int
		for _, member := range route.MultiPath {
			if member.Via != nil {
				requireViaGateway(t, member.Gw, member.Via, gateway)
				continue
			}
			sameFamily++
			require.True(t, member.Gw.Equal(ipv4Gateway.AsSlice()))
			require.Equal(t, indices[0], member.LinkIndex)
			require.Equal(t, 3, member.Hops)
		}
		require.Equal(t, 1, sameFamily)

		// The same-family primary paths remain ordinary RTA_GATEWAY routes.
		path.NextHop, path.Interface, path.OnLink, path.ECMP = ipv4Gateway, devices[0], true, nil
		routes.Insert(family.IPv4Unicast, prefix, path)
		apply(prefix, routeaction.Update)
		route = crossFamilyDecision(t, h, "198.51.100.9", prefix, 77)
		require.Nil(t, route.Via)
		require.True(t, route.Gw.Equal(ipv4Gateway.AsSlice()))
		require.Equal(t, indices[0], route.LinkIndex)
		path.NextHop = gateway
		routes.Insert(family.IPv6Unicast, ipv6Prefix, path)
		apply(ipv6Prefix, routeaction.Add)
		route = crossFamilyDecision(t, h, "2001:db8:100::9", ipv6Prefix, 77)
		require.Nil(t, route.Via)
		require.True(t, route.Gw.Equal(gateway.AsSlice()))
		require.Equal(t, indices[0], route.LinkIndex)

		// A connected route without device metadata models the connected
		// producer. With zero metric and no other forwarding fields, this
		// selected route takes the plain backend's add and replace paths.
		routes.Insert(family.IPv6Unicast, globalSubnet, locrib.Path{Source: connected})
		plain := locrib.Path{Source: protocol, NextHop: netip.MustParseAddr("2001:db8:200::2"), AdminDistance: 1}
		routes.Insert(family.IPv4Unicast, plainPrefix, plain)
		apply(plainPrefix, routeaction.Add)
		route = crossFamilyDecision(t, h, "203.0.113.9", plainPrefix, 0)
		requireViaGateway(t, route.Gw, route.Via, plain.NextHop)
		require.Equal(t, indices[0], route.LinkIndex)
		plain.NextHop = netip.MustParseAddr("2001:db8:200::3")
		routes.Insert(family.IPv4Unicast, plainPrefix, plain)
		apply(plainPrefix, routeaction.Update)
		route = crossFamilyDecision(t, h, "203.0.113.9", plainPrefix, 0)
		requireViaGateway(t, route.Gw, route.Via, plain.NextHop)
		require.Equal(t, indices[0], route.LinkIndex)

		routes.Remove(family.IPv4Unicast, prefix, protocol, 0)
		apply(prefix, routeaction.Withdraw)
		requireUnroutable(t, h, "198.51.100.9")
	})
}

func crossFamilyDecision(t *testing.T, h *netlink.Handle, destination string, prefix netip.Prefix, metric int) netlink.Route {
	t.Helper()
	route, err := forwardingDecision(t, h, destination)
	require.NoError(t, err)
	require.NotNil(t, route.Dst)
	require.Equal(t, prefix.String(), route.Dst.String())
	require.Equal(t, netlink.RouteProtocol(rtprotZE), route.Protocol)
	require.Equal(t, unix.RTN_UNICAST, route.Type)
	require.Equal(t, unix.RT_TABLE_MAIN, route.Table)
	require.Equal(t, metric, route.Priority)
	return route
}

func requireViaGateway(t *testing.T, gw []byte, destination netlink.Destination, gateway netip.Addr) {
	t.Helper()
	require.Empty(t, gw)
	via, ok := destination.(*netlink.Via)
	require.True(t, ok, "kernel gateway is not RTA_VIA: %v", destination)
	require.Equal(t, unix.AF_INET6, via.AddrFamily)
	require.True(t, via.Addr.Equal(gateway.AsSlice()), "kernel gateway = %v, want %s", via.Addr, gateway)
}
