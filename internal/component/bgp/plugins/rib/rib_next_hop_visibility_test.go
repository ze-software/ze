// Design: docs/architecture/route-selection.md -- show bgp rib and inject visibility for unresolved next hops
// Related: rib_pipeline.go, rib_commands.go, rib_next_hop.go
package rib

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func showBGPRIBRows(t *testing.T, r *RIBManager, pipe ...string) []map[string]any {
	t.Helper()
	status, payload, err := r.handleCommand("show bgp rib", "*", nil, ribPipe(pipe...)...)
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	var doc struct {
		Routes []map[string]any `json:"routes"`
	}
	require.NoError(t, json.Unmarshal([]byte(dataStr(t, payload)), &doc))
	return doc.Routes
}

func findRowByPrefix(t *testing.T, rows []map[string]any, prefix string) map[string]any {
	t.Helper()
	for _, row := range rows {
		if row[rowKeyPrefix] == prefix {
			return row
		}
	}
	require.Failf(t, "missing route row", "prefix %s not found in %v", prefix, rows)
	return nil
}

// TestShowBGPRIBMarksUnresolvedNextHop verifies AC-1: stored routes whose next
// hop fails forwarding admission carry next-hop-resolvable=false and
// ineligible-reason=next-hop-unresolvable in show bgp rib, filterable with
// | match next-hop-unresolvable, while resolvable and no-next-hop routes omit
// both fields and resolved routes lose both fields after a cover arrives.
//
// VALIDATES: show bgp rib marks unresolved and prospectively self-capturing
// routes, omits exclusion fields for valid/no-NH routes, and clears them once
// a covering route resolves the next hop.
// PREVENTS: operators seeing routes in show bgp rib with empty show bgp rib best
// and no explanation of why the route is ineligible.
func TestShowBGPRIBMarksUnresolvedNextHop(t *testing.T) {
	r, loc := nextHopReachabilityRIB(t)

	// Install one cover for the reachable next hop and a less-specific /8 cover
	// to exercise prospective self-capture on 10.80.4.0/24 via 10.80.4.1.
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/25"), igpRoute(0))
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/8"), igpRoute(0))

	unresolvedWire := ipv4Prefix(24, 10, 80, 1)
	resolvableWire := ipv4Prefix(24, 10, 80, 2)
	selfCaptureWire := ipv4Prefix(24, 10, 80, 4)

	nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("198.51.100.129"), 100, unresolvedWire)
	nextHopReachabilityPath(t, r, 2, netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("198.51.100.1"), 100, resolvableWire)
	nextHopReachabilityPath(t, r, 4, netip.MustParseAddr("192.0.2.4"), netip.MustParseAddr("10.80.4.1"), 100, selfCaptureWire)

	r.checkBestPathChange(family.IPv4Unicast, unresolvedWire, false, nil)
	r.checkBestPathChange(family.IPv4Unicast, resolvableWire, false, nil)
	r.checkBestPathChange(family.IPv4Unicast, selfCaptureWire, false, nil)

	// Route with no address next hop (manual inject without nexthop).
	status, _, err := r.handleCommand("request bgp rib inject", "", []string{"192.0.2.3", "ipv4/unicast", "10.80.3.0/24"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	rows := showBGPRIBRows(t, r, "received")

	unresolvedRow := findRowByPrefix(t, rows, "10.80.1.0/24")
	resolvableVal, hasResolvable := unresolvedRow["next-hop-resolvable"]
	require.True(t, hasResolvable, "unresolved route must include next-hop-resolvable")
	require.Equal(t, false, resolvableVal)
	require.Equal(t, "next-hop-unresolvable", unresolvedRow["ineligible-reason"])

	selfCaptureRow := findRowByPrefix(t, rows, "10.80.4.0/24")
	selfResolvableVal, hasSelfResolvable := selfCaptureRow["next-hop-resolvable"]
	require.True(t, hasSelfResolvable, "prospectively self-capturing route must include next-hop-resolvable")
	require.Equal(t, false, selfResolvableVal)
	require.Equal(t, "next-hop-unresolvable", selfCaptureRow["ineligible-reason"])

	resolvableRow := findRowByPrefix(t, rows, "10.80.2.0/24")
	_, hasResolvable = resolvableRow["next-hop-resolvable"]
	require.False(t, hasResolvable, "resolvable route must omit next-hop-resolvable")
	_, hasReason := resolvableRow["ineligible-reason"]
	require.False(t, hasReason, "resolvable route must omit ineligible-reason")

	noNHRow := findRowByPrefix(t, rows, "10.80.3.0/24")
	_, hasResolvable = noNHRow["next-hop-resolvable"]
	require.False(t, hasResolvable, "route without an address next hop must omit next-hop-resolvable")
	_, hasReason = noNHRow["ineligible-reason"]
	require.False(t, hasReason, "route without an address next hop must omit ineligible-reason")

	matchedRows := showBGPRIBRows(t, r, "received", "match", "next-hop-unresolvable")
	require.Len(t, matchedRows, 2)

	// Install a covering route for 198.51.100.129 and re-elect 10.80.1.0/24.
	loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.128/25"), igpRoute(0))
	r.checkBestPathChange(family.IPv4Unicast, unresolvedWire, false, nil)

	rowsAfter := showBGPRIBRows(t, r, "received")
	nowResolvableRow := findRowByPrefix(t, rowsAfter, "10.80.1.0/24")
	_, hasResolvable = nowResolvableRow["next-hop-resolvable"]
	require.False(t, hasResolvable, "resolved route must no longer carry next-hop-resolvable")
	_, hasReason = nowResolvableRow["ineligible-reason"]
	require.False(t, hasReason, "resolved route must no longer carry ineligible-reason")
}

// TestInjectRouteWarnsOnUnresolvedNextHop verifies AC-2: injecting a route
// with an unresolved IPv4 or IPv6 next hop retains the route in Adj-RIB-In,
// keeps the three base keys, adds next-hop-resolvable=false and a warning, and
// logs one WARN with doctor-bgp-next-hop-unresolvable; resolvable and
// no-next-hop injects return the exact 3-key response with no warning.
//
// VALIDATES: injectRoute warns on unresolved IPv4/IPv6 next hops while keeping
// Adj-RIB-In retention and preserving clean output for resolvable/no-NH injects.
// PREVENTS: silent best-path exclusion after request bgp rib inject reports done.
func TestInjectRouteWarnsOnUnresolvedNextHop(t *testing.T) {
	t.Run("unresolved_ipv4_next_hop", func(t *testing.T) {
		r, _ := nextHopReachabilityRIB(t)
		logs := captureRIBLogs(t)

		status, payload, err := r.handleCommand("request bgp rib inject", "", []string{
			"192.0.2.1", "ipv4/unicast", "10.90.1.0/24", "nexthop", "198.51.100.55",
		})
		require.NoError(t, err)
		require.Equal(t, statusDone, status)

		resp, ok := payload.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "10.90.1.0/24", resp[jsonKeyInjected])
		require.Equal(t, "192.0.2.1", resp[rowKeyPeer])
		require.Equal(t, "ipv4/unicast", resp[rowKeyFamily])

		resolvableVal, hasResolvable := resp["next-hop-resolvable"]
		require.True(t, hasResolvable, "unresolved inject must set next-hop-resolvable")
		require.Equal(t, false, resolvableVal)

		warning, hasWarning := resp["warning"].(string)
		require.True(t, hasWarning, "unresolved inject must include warning text")
		require.Contains(t, warning, "198.51.100.55")
		hasRemedy := strings.Contains(warning, "show rib")
		if !hasRemedy {
			hasRemedy = strings.Contains(warning, "show bgp rib")
		}
		require.True(t, hasRemedy, "warning must name show rib or show bgp rib: %q", warning)

		// Route stays retained in Adj-RIB-In.
		findRowByPrefix(t, showBGPRIBRows(t, r, "received"), "10.90.1.0/24")
		require.Equal(t, 1, logs.count(t, "WARN", codeDoctorBGPNextHopUnresolvable))
	})

	t.Run("unresolved_ipv6_next_hop", func(t *testing.T) {
		r, _ := nextHopReachabilityRIB(t)
		logs := captureRIBLogs(t)

		status, payload, err := r.handleCommand("request bgp rib inject", "", []string{
			"2001:db8::1", "ipv6/unicast", "2001:db8:90::/48", "nexthop", "2001:db8::55",
		})
		require.NoError(t, err)
		require.Equal(t, statusDone, status)

		resp, ok := payload.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "2001:db8:90::/48", resp[jsonKeyInjected])
		require.Equal(t, "2001:db8::1", resp[rowKeyPeer])
		require.Equal(t, "ipv6/unicast", resp[rowKeyFamily])

		resolvableVal, hasResolvable := resp["next-hop-resolvable"]
		require.True(t, hasResolvable, "unresolved IPv6 inject must set next-hop-resolvable")
		require.Equal(t, false, resolvableVal)

		warning, hasWarning := resp["warning"].(string)
		require.True(t, hasWarning, "unresolved IPv6 inject must include warning text")
		require.Contains(t, warning, "2001:db8::55")
		hasRemedy := strings.Contains(warning, "show rib")
		if !hasRemedy {
			hasRemedy = strings.Contains(warning, "show bgp rib")
		}
		require.True(t, hasRemedy, "warning must name show rib or show bgp rib: %q", warning)

		findRowByPrefix(t, showBGPRIBRows(t, r, "received"), "2001:db8:90::/48")
		require.Equal(t, 1, logs.count(t, "WARN", codeDoctorBGPNextHopUnresolvable))
	})

	t.Run("resolvable_ipv4_and_ipv6_omit_warning", func(t *testing.T) {
		r, loc := nextHopReachabilityRIB(t)
		logs := captureRIBLogs(t)

		loc.Insert(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), igpRoute(0))
		coverV6 := igpRoute(0)
		coverV6.NextHop = netip.MustParseAddr("fe80::1")
		loc.Insert(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::/64"), coverV6)
		require.True(t, igpcost.Resolve(loc, netip.MustParseAddr("198.51.100.1")).Resolved)
		require.True(t, igpcost.Resolve(loc, netip.MustParseAddr("2001:db8::10")).Resolved)

		status, payload, err := r.handleCommand("request bgp rib inject", "", []string{
			"192.0.2.1", "ipv4/unicast", "10.90.2.0/24", "nexthop", "198.51.100.1",
		})
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		require.Equal(t, map[string]any{
			jsonKeyInjected: "10.90.2.0/24",
			rowKeyPeer:      "192.0.2.1",
			rowKeyFamily:    "ipv4/unicast",
		}, payload)

		status, payload, err = r.handleCommand("request bgp rib inject", "", []string{
			"2001:db8::1", "ipv6/unicast", "2001:db8:91::/48", "nexthop", "2001:db8::10",
		})
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		require.Equal(t, map[string]any{
			jsonKeyInjected: "2001:db8:91::/48",
			rowKeyPeer:      "2001:db8::1",
			rowKeyFamily:    "ipv6/unicast",
		}, payload)

		require.Zero(t, logs.count(t, "WARN", codeDoctorBGPNextHopUnresolvable))
	})

	t.Run("no_next_hop_omits_warning", func(t *testing.T) {
		r, _ := nextHopReachabilityRIB(t)
		logs := captureRIBLogs(t)

		status, payload, err := r.handleCommand("request bgp rib inject", "", []string{
			"192.0.2.1", "ipv4/unicast", "10.90.3.0/24",
		})
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		require.Equal(t, map[string]any{
			jsonKeyInjected: "10.90.3.0/24",
			rowKeyPeer:      "192.0.2.1",
			rowKeyFamily:    "ipv4/unicast",
		}, payload)

		require.Zero(t, logs.count(t, "WARN", codeDoctorBGPNextHopUnresolvable))
	})
}

// TestShowBGPRIBUsesCachedNextHopDuringRemoteMetricOutage verifies that when
// forked route-metrics lookups are actively failing, show bgp rib returns
// immediately from cached metric answers (fail-closed on miss) without issuing
// blocking per-row RouteMetrics RPCs.
//
// VALIDATES: entryNextHopResolvedForShow uses prospectiveDistanceCached /
// igpDistanceCached while metricFailureActive is true, preserves cached
// resolvable answers, marks uncached rows next-hop-unresolvable, and invokes
// zero engine RPCs during show bgp rib.
// PREVENTS: 5s*N blocking RPC storms in show bgp rib during engine outages.
func TestShowBGPRIBUsesCachedNextHopDuringRemoteMetricOutage(t *testing.T) {
	registerBuiltinCommands()

	var failLookups atomic.Bool
	var blockLookups atomic.Bool
	var rpcCalls atomic.Int64

	clientSide, engineSide := net.Pipe()
	bridge := rpc.NewDirectBridge()
	bridge.SetDeliverEvents(func([]string) error { return nil })
	bridge.SetDispatchRPC(func(ctx context.Context, method string, body json.RawMessage) (json.RawMessage, error) {
		if method != rpc.MethodRouteMetrics {
			return nil, errors.New("unexpected engine RPC method: " + method)
		}
		rpcCalls.Add(1)
		if blockLookups.Load() {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		if failLookups.Load() {
			return nil, errors.New("controlled engine route-metrics failure")
		}
		var input rpc.RouteMetricsInput
		if err := json.Unmarshal(body, &input); err != nil {
			return nil, err
		}
		distances := make([]rpc.RouteMetric, len(input.Addresses))
		for i := range distances {
			distances[i] = rpc.RouteMetric{Resolved: true, Cost: 10}
		}
		return json.Marshal(rpc.RouteMetricsOutput{Revision: 1, Distances: distances})
	})
	bridge.SetReady()

	client := sdk.NewWithConn("bgp-rib", rpc.NewBridgedConn(clientSide, bridge))
	t.Cleanup(func() {
		require.NoError(t, client.Close())
		require.NoError(t, engineSide.Close())
	})

	r := newRIBManager(client)
	r.forkRIB = &remoteRIB{
		client: client,
		costs:  make(map[netip.Addr]igpcost.Distance),
	}

	cachedWire := ipv4Prefix(24, 10, 81, 1)
	uncachedWireA := ipv4Prefix(24, 10, 81, 2)
	uncachedWireB := ipv4Prefix(24, 10, 81, 3)

	cachedNH := netip.MustParseAddr("198.51.100.11")
	uncachedNHA := netip.MustParseAddr("198.51.100.12")
	uncachedNHB := netip.MustParseAddr("198.51.100.13")

	routes := nextHopReachabilityPath(t, r, 1, netip.MustParseAddr("192.0.2.1"), cachedNH, 100, cachedWire)
	routes.Insert(family.IPv4Unicast, aigpSelectionAttrs(uncachedNHA.As4(), nil, 100, 1), uncachedWireA)
	routes.Insert(family.IPv4Unicast, aigpSelectionAttrs(uncachedNHB.As4(), nil, 100, 1), uncachedWireB)

	// Warm the prospective cache for 10.81.1.0/24 while the engine is healthy.
	cachedPath := locrib.Path{
		Source:   bgpProtocolID,
		Instance: bgpLocRIBInstance,
		NextHop:  cachedNH,
		IsBGP:    true,
		IsEBGP:   false,
	}
	warmed := r.forkRIB.prospectiveDistance(family.IPv4Unicast, netip.MustParsePrefix("10.81.1.0/24"), cachedPath)
	require.True(t, warmed.Resolved)

	// Transition metricFailureActive() to true via one failed probe, then make
	// any subsequent RPC block until timeout so any stray RPC would stall and
	// increment rpcCalls.
	failLookups.Store(true)
	probePath := locrib.Path{
		Source:   bgpProtocolID,
		Instance: bgpLocRIBInstance,
		NextHop:  uncachedNHA,
		IsBGP:    true,
		IsEBGP:   false,
	}
	failed := r.forkRIB.prospectiveDistance(family.IPv4Unicast, netip.MustParsePrefix("10.81.2.0/24"), probePath)
	require.False(t, failed.Resolved)
	require.True(t, r.forkRIB.metricFailureActive())

	blockLookups.Store(true)
	callsBeforeShow := rpcCalls.Load()

	rows := showBGPRIBRows(t, r, "received")
	matchedRows := showBGPRIBRows(t, r, "received", "match", "next-hop-unresolvable")

	require.Equal(t, callsBeforeShow, rpcCalls.Load(),
		"show bgp rib must not issue RouteMetrics RPCs while metricFailureActive is true")

	cachedRow := findRowByPrefix(t, rows, "10.81.1.0/24")
	_, hasResolvable := cachedRow["next-hop-resolvable"]
	require.False(t, hasResolvable, "cached resolvable row must stay clean during outage")
	_, hasReason := cachedRow["ineligible-reason"]
	require.False(t, hasReason, "cached resolvable row must omit ineligible-reason during outage")

	for _, prefix := range []string{"10.81.2.0/24", "10.81.3.0/24"} {
		row := findRowByPrefix(t, rows, prefix)
		require.Equal(t, false, row["next-hop-resolvable"])
		require.Equal(t, "next-hop-unresolvable", row["ineligible-reason"])
	}
	require.Len(t, matchedRows, 2)
}
