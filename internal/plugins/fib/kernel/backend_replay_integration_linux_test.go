//go:build integration && linux

// Design: docs/guide/plugins.md -- installed-route repair replay.
// Related: backend_replay_linux.go -- fresh identity and noncreating change.

package fibkernel

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func kernelReplayRoute(prefix string) RichRoute {
	return RichRoute{
		Prefix: netip.MustParsePrefix(prefix), RouteType: sysribevents.RouteTypeBlackhole,
		TableID: 173, Metric: 71,
	}
}

func TestKernelReplayPlainRoute(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, _ int) {
		backend := &netlinkBackend{handle: h}
		rich := RichRoute{
			Prefix:  netip.MustParsePrefix("203.0.127.0/24"),
			NextHop: netip.MustParseAddr("198.18.81.2"),
		}
		require.NoError(t, backend.addRoute(rich.Prefix.String(), rich.NextHop.String()))
		route, err := buildRichRoute(rich)
		require.NoError(t, err)
		before := identityProbeSnapshot(t, h, route)
		require.NotEmpty(t, before, "the plain installation must establish useful forwarding")
		require.Len(t, identityProbeRows(t, h, route), 1)
		handled, err := backend.reassertRichRoute(rich)
		assert.True(t, handled, "omitted type, table and metric must match the existing plain installation")
		assert.NoError(t, err)
		assert.Equal(t, before, identityProbeSnapshot(t, h, route))
	})
}

func TestKernelReplayUnchangedRoute(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, _ int) {
		backend := &netlinkBackend{handle: h}
		for _, routeType := range []sysribevents.RouteType{
			sysribevents.RouteTypeUnicast, sysribevents.RouteTypeBlackhole,
			sysribevents.RouteTypeUnreachable, sysribevents.RouteTypeProhibit,
		} {
			rich := kernelReplayRoute("203.0.118.0/24")
			rich.RouteType = routeType
			if routeType == sysribevents.RouteTypeUnicast {
				rich.NextHop = netip.MustParseAddr("198.18.81.2")
				rich.PathMTU = 1400
				rich.Weight = 3
				rich.ECMPPaths = []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("198.18.81.3"), Weight: 2}}
			}
			require.NoError(t, backend.replaceRichRoute(rich))
			route, err := buildRichRoute(rich)
			require.NoError(t, err)
			before := identityProbeSnapshot(t, h, route)
			handled, err := backend.reassertRichRoute(rich)
			assert.True(t, handled, "type %d must use the fresh exact-identity reassertion", routeType)
			assert.NoError(t, err)
			assert.Equal(t, before, identityProbeSnapshot(t, h, route), "reassertion must retain all forwarding attributes")
			require.NoError(t, h.RouteDel(route))
		}
	})
}

func TestKernelReplayRepairsMissingRoute(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, _ int) {
		backend := &netlinkBackend{handle: h}
		rich := kernelReplayRoute("203.0.119.0/24")
		require.NoError(t, backend.replaceRichRoute(rich))
		route, err := buildRichRoute(rich)
		require.NoError(t, err)
		before := identityProbeSnapshot(t, h, route)
		handled, err := backend.reassertRichRoute(rich)
		require.True(t, handled, "establish a successful live replay before deletion")
		require.NoError(t, err)
		require.NoError(t, h.RouteDel(route))
		handled, err = backend.reassertRichRoute(rich)
		assert.False(t, handled, "a prior success must not hide current absence")
		assert.NoError(t, err)
		assert.Empty(t, identityProbeRows(t, h, route), "the probe must not create a missing route")
		require.NoError(t, backend.replaceRichRoute(rich), "the ordinary path must repair the missing route")
		assert.Equal(t, before, identityProbeSnapshot(t, h, route))
	})
}

func TestKernelReplayRepairsModifiedRoute(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, _ int) {
		backend := &netlinkBackend{handle: h}
		rich := kernelReplayRoute("203.0.120.0/24")
		rich.RouteType = sysribevents.RouteTypeUnicast
		rich.NextHop = netip.MustParseAddr("198.18.81.2")
		rich.PathMTU = 1400
		require.NoError(t, backend.replaceRichRoute(rich))
		route, err := buildRichRoute(rich)
		require.NoError(t, err)
		before := identityProbeSnapshot(t, h, route)
		handled, err := backend.reassertRichRoute(rich)
		require.True(t, handled)
		require.NoError(t, err)
		modified := rich
		modified.NextHop = netip.MustParseAddr("198.18.81.3")
		modified.PathMTU = 1300
		changed, err := buildRichRoute(modified)
		require.NoError(t, err)
		require.NoError(t, h.RouteReplace(changed))
		modifiedSnapshot := identityProbeSnapshot(t, h, route)
		require.NotEqual(t, before, modifiedSnapshot, "the external edit must alter useful forwarding state")
		handled, err = backend.reassertRichRoute(rich)
		assert.False(t, handled, "a prior success must not conceal changed live forwarding")
		assert.NoError(t, err)
		assert.Equal(t, modifiedSnapshot, identityProbeSnapshot(t, h, route))
		require.NoError(t, backend.replaceRichRoute(rich))
		assert.Equal(t, before, identityProbeSnapshot(t, h, route), "fallback must restore gateway and MTU")
	})
}

func TestKernelReplayPreservesForeignReplacement(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, _ int) {
		backend := &netlinkBackend{handle: h}
		rich := kernelReplayRoute("203.0.121.0/24")
		require.NoError(t, backend.replaceRichRoute(rich))
		route, err := buildRichRoute(rich)
		require.NoError(t, err)
		handled, err := backend.reassertRichRoute(rich)
		require.True(t, handled)
		require.NoError(t, err)
		foreign := *route
		foreign.Protocol, foreign.Type = 42, unix.RTN_UNREACHABLE
		require.NoError(t, h.RouteReplace(&foreign))
		before := identityProbeSnapshot(t, h, route)
		handled, err = backend.reassertRichRoute(rich)
		assert.False(t, handled, "a cached successful replay cannot authorize replacing the new owner")
		assert.NoError(t, err)
		assert.Equal(t, before, identityProbeSnapshot(t, h, route))
		assert.ErrorIs(t, backend.replaceRichRoute(rich), unix.EEXIST)
		assert.Equal(t, before, identityProbeSnapshot(t, h, route), "ordinary fallback must preserve foreign ownership")
	})
}

func TestKernelReplayForeignAliasOrder(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, _ int) {
		backend := &netlinkBackend{handle: h}
		for _, foreignFirst := range []bool{false, true} {
			rich := kernelReplayRoute("203.0.122.0/24")
			own, err := buildRichRoute(rich)
			require.NoError(t, err)
			foreign := *own
			foreign.Protocol, foreign.Type = 42, unix.RTN_UNREACHABLE
			first, second := own, &foreign
			if foreignFirst {
				first, second = &foreign, own
			}
			require.NoError(t, h.RouteAdd(first))
			require.NoError(t, h.RouteAppend(second))
			rows := identityProbeRows(t, h, own)
			require.Len(t, rows, 2)
			require.Equal(t, first.Protocol, rows[0].Protocol)
			before := identityProbeSnapshot(t, h, own)
			handled, err := backend.reassertRichRoute(rich)
			assert.True(t, handled, "the exact owned alias exists in either order")
			if foreignFirst {
				assert.ErrorIs(t, err, unix.EEXIST, "the preferred foreign alias must refuse replacement")
			} else {
				assert.NoError(t, err, "the preferred owned alias can be reasserted")
			}
			assert.Equal(t, before, identityProbeSnapshot(t, h, own), "both aliases and their order must survive")
			require.NoError(t, h.RouteDel(own))
			require.NoError(t, h.RouteDel(&foreign))
		}
	})
}

func TestKernelReplayIPv6Fallback(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, _ int) {
		backend := &netlinkBackend{handle: h}
		rich := kernelReplayRoute("2001:db8:123::/64")
		require.NoError(t, backend.replaceRichRoute(rich))
		route, err := buildRichRoute(rich)
		require.NoError(t, err)
		before := identityProbeSnapshot(t, h, route)
		handled, err := backend.reassertRichRoute(rich)
		assert.False(t, handled, "IPv6 must retain its ordinary ownership lookup")
		assert.NoError(t, err)
		assert.Equal(t, before, identityProbeSnapshot(t, h, route))
		require.NoError(t, backend.replaceRichRoute(rich))
		foreign := *route
		foreign.Protocol, foreign.Type = 42, unix.RTN_UNREACHABLE
		require.NoError(t, h.RouteReplace(&foreign))
		foreignSnapshot := identityProbeSnapshot(t, h, route)
		handled, err = backend.reassertRichRoute(rich)
		assert.False(t, handled)
		assert.NoError(t, err)
		assert.ErrorIs(t, backend.replaceRichRoute(rich), unix.EEXIST)
		assert.Equal(t, foreignSnapshot, identityProbeSnapshot(t, h, route))
	})
}

func TestKernelReplayFullPathCleansPriorities(t *testing.T) {
	withIPv4IdentityProbe(t, func(h *netlink.Handle, _ int) {
		backend := &netlinkBackend{handle: h}
		for index, state := range []string{"changed", "startup", "pending"} {
			rich := kernelReplayRoute(fmt.Sprintf("203.0.%d.0/24", 124+index))
			if state == "startup" {
				rich.TableID = unix.RT_TABLE_MAIN
			}
			entry := incomingChange{Action: routeaction.Add, Prefix: rich.Prefix, RouteType: rich.RouteType, TableID: rich.TableID, Metric: rich.Metric}
			writer := newFIBKernel(backend)
			writer.processEvent(makeSysribPayload([]incomingChange{entry}))
			route, err := buildRichRoute(rich)
			require.NoError(t, err)
			require.Len(t, identityProbeRows(t, h, route), 1, "%s control must install a useful owned route", state)
			older := *route
			older.Priority = 33
			require.NoError(t, h.RouteAdd(&older))
			foreign := *route
			foreign.Protocol, foreign.Priority = 42, 5000
			require.NoError(t, h.RouteAdd(&foreign))
			if state == "changed" {
				entry.Metric++
			}
			if state == "startup" {
				writer = newFIBKernel(backend)
				writer.startupSweep()
				require.True(t, writer.retained[entry.Prefix.String()], "startup control must exercise retained ownership")
			}
			if state == "pending" {
				writer.pending[entry.Prefix.String()] = time.Now()
			}
			processFIBReplay(writer, &entry)
			rows := identityProbeRows(t, h, route)
			require.Len(t, rows, 2, "%s replay must remove every superseded owned priority", state)
			for _, row := range rows {
				if row.Protocol == rtprotZE {
					assert.Equal(t, int(entry.Metric), row.Priority, "%s must preserve the current owned slot", state)
					continue
				}
				assert.Equal(t, foreign.Protocol, row.Protocol)
				assert.Equal(t, foreign.Priority, row.Priority, "%s must preserve the unrelated foreign alternative", state)
			}
			assert.Empty(t, writer.pending, "%s replay must finish a full successful repair", state)
		}
	})
}
