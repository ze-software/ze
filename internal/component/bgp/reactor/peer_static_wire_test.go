// Related: peer_static_wire.go — the delta under test
// Related: peer_settings_apply.go — the reload swap that calls it
package reactor

import (
	"encoding/hex"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/report"
)

// staticRouteAt builds one IPv4 unicast static route with an explicit next hop,
// the shape `static { route <prefix> next-hop <addr>; }` parses to.
func staticRouteAt(prefix, nextHop string) StaticRoute {
	return StaticRoute{
		Prefix:  netip.MustParsePrefix(prefix),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr(nextHop)),
	}
}

// bgpFrames splits a recorded write stream into whole BGP messages, using the
// length octets of each header (RFC 4271 Section 4.1). A trailing partial frame
// is an error rather than a silent truncation: the tests below count frames, and
// a miscount must not read as a passing assertion.
func bgpFrames(t *testing.T, wire []byte) [][]byte {
	t.Helper()
	var frames [][]byte
	for len(wire) > 0 {
		require.GreaterOrEqual(t, len(wire), 19, "a trailing fragment is not a BGP message")
		length := int(wire[16])<<8 | int(wire[17])
		require.GreaterOrEqual(t, length, 19, "a BGP message is at least its header")
		require.LessOrEqual(t, length, len(wire), "the last message is truncated")
		frames = append(frames, wire[:length])
		wire = wire[length:]
	}
	return frames
}

// TestStaticRouteDeltaNamesAddedAndRemoved pins the arithmetic a reload runs: what
// the session must be sent to move from the set it holds to the set configured.
//
// VALIDATES: the delta is computed against the held set, so an unchanged route is
// neither re-announced nor withdrawn.
// PREVENTS: the whole set being re-sent on every reload, which re-floods a peer
// with routes it already has (RFC 4271 Section 9.2).
func TestStaticRouteDeltaNamesAddedAndRemoved(t *testing.T) {
	held := []StaticRoute{
		staticRouteAt("10.0.0.0/24", "10.9.9.9"),
		staticRouteAt("10.0.1.0/24", "10.9.9.9"),
	}
	wanted := []StaticRoute{
		staticRouteAt("10.0.0.0/24", "10.9.9.9"),
		staticRouteAt("10.0.2.0/24", "10.9.9.9"),
	}

	announce, withdraw := staticRouteDelta(held, wanted)

	require.Len(t, announce, 1, "only the new prefix is announced")
	assert.Equal(t, netip.MustParsePrefix("10.0.2.0/24"), announce[0].Prefix)
	require.Len(t, withdraw, 1, "only the prefix the configuration dropped is withdrawn")
	assert.Equal(t, netip.MustParsePrefix("10.0.1.0/24"), withdraw[0].Prefix)
}

// TestStaticRouteDeltaReannouncesChangedAttributes pins the case where the prefix
// stays and its attributes change.
//
// VALIDATES: an edited route is announced and NOT withdrawn. RFC 4271 Section 3.1
// has a second advertisement of one destination replace the first, so the
// announcement is the whole update.
// PREVENTS: a withdrawal beside the announcement, which would take the new route
// away again and blackhole the prefix the operator just edited.
func TestStaticRouteDeltaReannouncesChangedAttributes(t *testing.T) {
	held := []StaticRoute{staticRouteAt("10.0.0.0/24", "10.9.9.9")}
	wanted := []StaticRoute{staticRouteAt("10.0.0.0/24", "10.8.8.8")}

	announce, withdraw := staticRouteDelta(held, wanted)

	require.Len(t, announce, 1, "the edited route is re-announced")
	assert.Empty(t, withdraw, "the prefix is still configured, so nothing is withdrawn")
}

// TestStaticWireAfterDeltaDropsARouteThatDidNotReachTheWire pins what the peer is
// recorded as holding when an announcement failed.
//
// VALIDATES: the record is what reached the wire, not what was configured.
// PREVENTS: a route believed delivered after a failed send. The next reload would
// then find it unchanged, announce nothing, and leave the prefix unreachable with
// the configuration and the daemon both saying it is up.
func TestStaticWireAfterDeltaDropsARouteThatDidNotReachTheWire(t *testing.T) {
	wanted := []StaticRoute{
		staticRouteAt("10.0.0.0/24", "10.9.9.9"),
		staticRouteAt("10.0.2.0/24", "10.9.9.9"),
	}
	announce := []StaticRoute{wanted[1]}

	after := staticWireAfterDelta(wanted, announce, nil)

	require.Len(t, after, 1, "the route whose announcement failed is not recorded as held")
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/24"), after[0].Prefix)
}

// TestReloadDeliversStaticRouteDeltaOnTheRunningSession is the wire-level proof:
// a swap that changes the route set withdraws what left the configuration and
// announces what joined it, on the connection that is already up.
//
// VALIDATES: AC — a route added to `static { route ... }` is announced on the
// ESTABLISHED session, a route removed is withdrawn on it, and an unchanged route
// is left alone.
// PREVENTS: the restart that shipped before. StaticRoutes was outside
// hotSwappableSettings, so an edit tore the TCP connection down and every route
// the peer held was re-learned to deliver one change.
//
// DISCRIMINATION: the assertions are the BYTES of two frames written AFTER the
// initial send. Taking StaticRoutes back out of hotSwappableSettings leaves the
// wire empty here, and delivering the field without the delta leaves it empty too.
func TestReloadDeliversStaticRouteDeltaOnTheRunningSession(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	negotiateForwardTestFamilies(peer, family.IPv4Unicast)
	peer.settings.StaticRoutes = []StaticRoute{
		staticRouteAt("10.0.0.0/24", "10.9.9.9"),
		staticRouteAt("10.0.1.0/24", "10.9.9.9"),
	}

	peer.runInitialSyncNow()
	initial := len(conn.written())
	require.Greater(t, initial, 0, "the fixture must put the first set on the wire")

	next := *peer.settings
	next.StaticRoutes = []StaticRoute{
		staticRouteAt("10.0.0.0/24", "10.9.9.9"),
		staticRouteAt("10.0.2.0/24", "10.9.9.9"),
	}
	mustApplyHotSwappableSettings(t, peer, &next, hotSwappableSettings)

	frames := bgpFrames(t, conn.written()[initial:])
	require.Len(t, frames, 2, "one withdrawal and one announcement, and nothing for the route that did not change")

	// RFC 4271 Section 4.3: withdrawn-routes length 4, the prefix 10.0.1.0/24 as
	// <length, prefix>, then a zero path-attributes length.
	assert.Equal(t, "ffffffffffffffffffffffffffffffff001b020004180a00010000",
		hex.EncodeToString(frames[0]), "the dropped route is withdrawn")
	assert.Contains(t, hex.EncodeToString(frames[1]), "180a0002",
		"the added route's NLRI must reach the wire")
	assert.NotContains(t, hex.EncodeToString(frames[1]), "180a0000",
		"the unchanged route must not be re-announced")
}

// TestStaticWithdrawWithheldOnAConnectionThatAdvertisedNothing pins the RFC 4271
// Section 4.3 guard on the reload's own withdrawal rail.
//
// VALIDATES: a withdrawn route "is identified by its destination ... which
// unambiguously identifies the route in the context of the BGP speaker - BGP
// speaker connection to which it has been previously advertised", so a connection
// that advertised nothing is written nothing.
// PREVENTS: a reload naming a route on a connection that never carried one, which
// is what the API rail already declines to do (withdrawBatchFromPeers).
func TestStaticWithdrawWithheldOnAConnectionThatAdvertisedNothing(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	require.False(t, peer.hasAdvertised(), "the fixture must start with nothing advertised")

	peer.withdrawStaticRoutes(peer.currentSession(), []StaticRoute{staticRouteAt("10.0.1.0/24", "10.9.9.9")}, 4096, false)

	assert.Empty(t, conn.written(), "no withdrawal is written on a connection that advertised nothing")
}

// TestSendStaticRoutesReportsUnresolvedNextHop proves that an unresolved static
// route next hop is reported at WARN (the default log level) with its registered
// doctor code, peer, prefix, why, and fix, and raised on the operational report
// bus for both ungrouped and grouped static route sends.
//
// VALIDATES: AC-3 — answer-2 reporting for Peer.sendStaticRoutes and
// Peer.sendStaticRoutesGrouped when resolveNextHop returns an error.
// PREVENTS: a configured static route silently vanishing at Debug with no
// operator-visible warning or report-bus event.
func TestSendStaticRoutesReportsUnresolvedNextHop(t *testing.T) {
	const unresolvedPrefix = "10.0.1.0/24"
	routes := []StaticRoute{
		{Prefix: netip.MustParsePrefix(unresolvedPrefix), NextHop: bgptypes.RouteNextHop{}},
	}

	for _, tc := range []struct {
		name  string
		group bool
	}{
		{name: "ungrouped", group: false},
		{name: "grouped", group: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)

			sink := &syncBuffer{}
			t.Cleanup(swapRoutesLogger(slog.New(slog.NewJSONHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn}))))

			peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
			negotiateForwardTestFamilies(peer, family.IPv4Unicast)

			sent := peer.sendStaticRoutes(peer.currentSession(), routes, tc.group, 4096, false)

			assert.Empty(t, sent, "a static route with an unresolved next hop must not be recorded as sent")
			assert.Empty(t, conn.written(), "a static route with an unresolved next hop must not reach the wire")

			coded := closeRecordsWith(closeLogRecords(t, sink), "code", codeDoctorBGPRouteNotSent)
			require.Len(t, coded, 1, "unresolved static route must emit one coded WARN line")
			assert.Equal(t, "WARN", coded[0]["level"])
			assert.Equal(t, peer.addrString, coded[0]["peer"])
			assert.Equal(t, unresolvedPrefix, coded[0]["prefix"])
			assert.NotEmpty(t, coded[0]["why"], "WARN line must explain why it matters")
			assert.Equal(t, fixRouteNotSent, coded[0]["fix"])
			assert.NotEmpty(t, coded[0]["error"], "WARN line must carry the resolution error")

			events := report.Errors(0)
			require.Len(t, events, 1, "unresolved static route must raise one report-bus event")
			assert.Equal(t, reportSourceBGP, events[0].Source)
			assert.Equal(t, codeDoctorBGPRouteNotSent, events[0].Code)
			assert.Equal(t, peer.addrString, events[0].Subject)
			assert.Equal(t, fixRouteNotSent, events[0].Fix)
			assert.Contains(t, events[0].Message, unresolvedPrefix)
		})
	}
}
