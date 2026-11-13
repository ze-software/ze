//go:build integration && linux

// Design: docs/architecture/core-design.md -- FIB kernel ownership.
// Related: backend_linux.go -- the ownership checks retained on probe misses.

package fibkernel

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

func withIPv4IdentityProbe(t *testing.T, fn func(*netlink.Handle, int)) {
	t.Helper()
	if !userns.Enter(t) {
		return
	}
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		link := &netlink.Dummy{Name: "ze-id-probe"}
		require.NoError(t, h.LinkAdd(link))
		actual, err := h.LinkByName(link.Name)
		require.NoError(t, err)
		require.NoError(t, h.LinkSetUp(actual))
		address, err := netlink.ParseAddr("198.18.81.1/24")
		require.NoError(t, err)
		require.NoError(t, h.AddrAdd(actual, address))
		control := identityProbeRoute(t, "198.19.250.0/24", unix.RTN_BLACKHOLE)
		control.Protocol = 42
		require.NoError(t, h.RouteAdd(control))
		fn(h, actual.Attrs().Index)
		assert.Len(t, identityProbeRows(t, h, control), 1, "the unrelated foreign control must survive every probe")
	})
}

func identityProbeRoute(t *testing.T, prefix string, routeType int) *netlink.Route {
	t.Helper()
	_, destination, err := net.ParseCIDR(prefix)
	require.NoError(t, err)
	return &netlink.Route{Dst: destination, Table: unix.RT_TABLE_MAIN, Protocol: rtprotZE, Type: routeType, Priority: 71}
}

func identityProbeRows(t *testing.T, h *netlink.Handle, route *netlink.Route) []netlink.Route {
	t.Helper()
	family := netlink.FAMILY_V6
	if route.Dst.IP.To4() != nil {
		family = netlink.FAMILY_V4
	}
	rows, err := h.RouteListFiltered(family, &netlink.Route{Dst: route.Dst, Table: route.Table}, netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
	require.NoError(t, err)
	return rows
}

func identityProbeSnapshot(t *testing.T, h *netlink.Handle, route *netlink.Route) []byte {
	t.Helper()
	data, err := json.Marshal(identityProbeRows(t, h, route))
	require.NoError(t, err)
	return data
}

func TestIPv4RouteIdentityProbeExact(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, index int) {
		for _, routeType := range []int{unix.RTN_UNICAST, unix.RTN_BLACKHOLE, unix.RTN_UNREACHABLE, unix.RTN_PROHIBIT} {
			route := identityProbeRoute(t, "203.0.113.0/24", routeType)
			if routeType == unix.RTN_UNICAST {
				route.LinkIndex = index
				route.Gw = net.ParseIP("198.18.81.2")
				route.MTU = 1400
				route.AdvMSS = 1300
			}
			require.NoError(t, h.RouteAdd(route))
			before := identityProbeSnapshot(t, h, route)
			assert.ErrorIs(t, h.RouteProbeIPv4(route), unix.EEXIST, "exact route type %d must be identified", routeType)
			assert.Equal(t, before, identityProbeSnapshot(t, h, route), "an exact probe must not mutate route type %d", routeType)
			require.NoError(t, h.RouteDel(route))
		}
		weighted := identityProbeRoute(t, "203.0.114.0/24", unix.RTN_UNICAST)
		weighted.Table = 173
		weighted.Priority = 0
		weighted.MultiPath = []*netlink.NexthopInfo{
			{LinkIndex: index, Gw: net.ParseIP("198.18.81.2"), Hops: 2},
			{LinkIndex: index, Gw: net.ParseIP("198.18.81.3"), Hops: 1},
		}
		require.NoError(t, h.RouteAdd(weighted))
		before := identityProbeSnapshot(t, h, weighted)
		assert.ErrorIs(t, h.RouteProbeIPv4(weighted), unix.EEXIST, "weighted targets, explicit zero metric and named table must match")
		assert.Equal(t, before, identityProbeSnapshot(t, h, weighted))
	})
}

func TestIPv4RouteIdentityProbeChanged(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, index int) {
		route := identityProbeRoute(t, "203.0.115.0/24", unix.RTN_UNICAST)
		route.LinkIndex, route.Gw, route.MTU = index, net.ParseIP("198.18.81.2"), 1400
		require.NoError(t, h.RouteAdd(route))
		before := identityProbeSnapshot(t, h, route)
		for _, field := range []string{"protocol", "priority", "gateway", "mtu", "table"} {
			changed := *route
			switch field {
			case "protocol":
				changed.Protocol = 42
			case "priority":
				changed.Priority++
			case "gateway":
				changed.Gw = net.ParseIP("198.18.81.3")
			case "mtu":
				changed.MTU++
			case "table":
				changed.Table = 173
			}
			proposedBefore := identityProbeSnapshot(t, h, &changed)
			assert.ErrorIs(t, h.RouteProbeIPv4(&changed), unix.ENOENT, "changed %s must not borrow the original identity", field)
			assert.Equal(t, before, identityProbeSnapshot(t, h, route), "changed %s must not mutate the original", field)
			assert.Equal(t, proposedBefore, identityProbeSnapshot(t, h, &changed), "changed %s must not install a different route", field)
		}
	})
}

func TestIPv4RouteIdentityProbeMissing(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, _ int) {
		route := identityProbeRoute(t, "203.0.116.0/24", unix.RTN_BLACKHOLE)
		before := identityProbeSnapshot(t, h, route)
		assert.ErrorIs(t, h.RouteProbeIPv4(route), unix.ENOENT)
		assert.Equal(t, before, identityProbeSnapshot(t, h, route), "a miss must not install the proposed route")
	})
}

func TestIPv4RouteIdentityProbeForeignAliases(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, _ int) {
		for _, foreignFirst := range []bool{false, true} {
			own := identityProbeRoute(t, "203.0.117.0/24", unix.RTN_BLACKHOLE)
			foreign := *own
			foreign.Protocol, foreign.Type = 42, unix.RTN_UNREACHABLE
			first, second := own, &foreign
			if foreignFirst {
				first, second = &foreign, own
			}
			require.NoError(t, h.RouteAdd(first))
			beforeForeign := identityProbeSnapshot(t, h, own)
			if foreignFirst {
				assert.ErrorIs(t, h.RouteProbeIPv4(own), unix.ENOENT, "a foreign-only slot is not the desired identity")
				assert.Equal(t, beforeForeign, identityProbeSnapshot(t, h, own))
			}
			require.NoError(t, h.RouteAppend(second))
			rows := identityProbeRows(t, h, own)
			require.Len(t, rows, 2)
			require.Equal(t, first.Protocol, rows[0].Protocol, "the fixture must establish the requested preferred alias")
			before := identityProbeSnapshot(t, h, own)
			assert.ErrorIs(t, h.RouteProbeIPv4(own), unix.EEXIST, "the desired identity must exist in either alias order")
			if foreignFirst {
				assert.ErrorIs(t, h.RouteChange(own), unix.EEXIST, "reassertion cannot replace a preferred foreign alias")
			} else {
				assert.NoError(t, h.RouteChange(own), "the exact preferred identity can be reasserted")
			}
			assert.Equal(t, before, identityProbeSnapshot(t, h, own), "both protocols and alias order must survive")
			require.NoError(t, h.RouteDel(own))
			require.NoError(t, h.RouteDel(&foreign))
		}
	})
}

func TestIPv4RouteIdentityProbeRefusesIPv6(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, _ int) {
		foreign := identityProbeRoute(t, "2001:db8:117::/64", unix.RTN_BLACKHOLE)
		foreign.Protocol = 42
		require.NoError(t, h.RouteAdd(foreign))
		before := identityProbeSnapshot(t, h, foreign)
		proposed := *foreign
		proposed.Protocol, proposed.Type = rtprotZE, unix.RTN_UNREACHABLE
		assert.ErrorIs(t, h.RouteProbeIPv4(&proposed), unix.EAFNOSUPPORT)
		assert.Equal(t, before, identityProbeSnapshot(t, h, foreign), "IPv6 must be refused before its potentially mutating insertion path")
	})
}
