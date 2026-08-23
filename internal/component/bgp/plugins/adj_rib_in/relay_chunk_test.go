// Tests for the relay chunker. They assert two things: a peer-up replay reaches
// the engine in frames the IPC transport accepts, and every stored route travels
// exactly once whatever the chunk boundaries fall on.
//
// The size bound is driven as a pure function against encoding/json. The thing
// under test is arithmetic over the SERIALIZED form, and a live engine would
// hide it. The last-index property is driven from handleCommand instead. That is
// the number a caller converges on, and chunking is the change that can move it
// (ai/rules/evidence.md).

package adj_rib_in

import (
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/seqmap"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRelayRouteJSONMaxBoundsMarshal pins relayRouteJSONFixed against the real
// encoder.
//
// VALIDATES: relayRouteJSONMax is an UPPER bound on what one route adds to the
// serialized routes array, for the widest form of every field.
// PREVENTS: a field added to rpc.StoredRoute making the bound read low, which
// puts a chunk over rpc.MaxMessageSize and loses every route in it at once.
func TestRelayRouteJSONMaxBoundsMarshal(t *testing.T) {
	cases := []struct {
		name  string
		route rpc.StoredRoute
	}{
		{"empty", rpc.StoredRoute{}},
		{"widest-scalars", rpc.StoredRoute{
			PathID:      ^uint32(0),
			NLRIFraming: rpc.NLRIFramingSourceWire,
		}},
		{"prefix-only-framing", rpc.StoredRoute{
			SourcePeer:  "2001:db8:0000:0000:0000:0000:0000:0001",
			Family:      "ipv6-flowspec",
			AttrHex:     strings.Repeat("ab", 2048),
			NextHopHex:  strings.Repeat("cd", 32),
			NLRIHex:     strings.Repeat("ef", 64),
			PathID:      ^uint32(0),
			NLRIFraming: rpc.NLRIFramingPrefixOnly,
		}},
		{"unrecorded-framing", rpc.StoredRoute{
			SourcePeer: "10.0.0.1",
			Family:     "ipv4-unicast",
			AttrHex:    "40010100",
			NextHopHex: "0a000001",
			NLRIHex:    "180a0000",
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.route)
			require.NoError(t, err)
			// +1 for the comma that separates this route from the next one.
			assert.GreaterOrEqual(t, relayRouteJSONMax(&tc.route), len(encoded)+1,
				"the bound must cover the encoded route and its separator")
		})
	}
}

// TestRelayChunkStaysUnderFrameCeiling is the AC-3 assertion, made against the
// serialized frame rather than against a route count.
//
// VALIDATES: a replay of routes carrying a full-size attribute block is cut into
// chunks that each serialize below rpc.MaxMessageSize.
// PREVENTS: the count-based chunk this replaced. AttrHex is hex, so it is twice
// the attribute block, and 130 routes carrying a 64 KiB block serialize to about
// 17 MB in ONE frame -- which WriteMessage refuses whole, losing every route.
func TestRelayChunkStaysUnderFrameCeiling(t *testing.T) {
	// One shared string: a Go string is copied by header, so 130 routes carrying
	// the same 64 KiB attribute block cost 131 KB, not 17 MB.
	attrHex := strings.Repeat("ab", 65535)
	routes := make([]rpc.StoredRoute, 130)
	for i := range routes {
		routes[i] = rpc.StoredRoute{
			SourcePeer: "10.0.0.1",
			Family:     "ipv4-unicast",
			AttrHex:    attrHex,
			NextHopHex: "0a000001",
			NLRIHex:    "180a0000",
		}
	}

	chunks := 0
	covered := 0
	for start := 0; start < len(routes); {
		end := relayChunkEnd(routes, start, relayChunkBudget)
		require.Greater(t, end, start, "a chunk must advance the walk")

		encoded, err := json.Marshal(rpc.RelayStoredRouteInput{
			Destination: "192.0.2.1",
			Routes:      routes[start:end],
		})
		require.NoError(t, err)
		assert.LessOrEqual(t, len(encoded), rpc.MaxMessageSize,
			"chunk starting at %d serializes to %d bytes", start, len(encoded))

		chunks++
		covered += end - start
		start = end
	}

	assert.Greater(t, chunks, 1, "this fixture must exceed one frame, or it proves nothing")
	assert.Equal(t, len(routes), covered, "every route travels exactly once")
}

// TestRelayChunkCoversEveryRouteOnce drives the walk at a budget small enough to
// cut on every boundary.
//
// VALIDATES: consecutive chunks partition the routes in order, with no route
// dropped and none sent twice.
// PREVENTS: an off-by-one at the chunk boundary, which on the real budget is
// reachable only with a 16 MB fixture.
func TestRelayChunkCoversEveryRouteOnce(t *testing.T) {
	routes := make([]rpc.StoredRoute, 7)
	for i := range routes {
		routes[i] = rpc.StoredRoute{
			SourcePeer: "10.0.0.1",
			Family:     "ipv4-unicast",
			NLRIHex:    "180a0000",
		}
	}
	perRoute := relayRouteJSONMax(&routes[0])

	for _, budget := range []int{perRoute, 2 * perRoute, 3*perRoute - 1, 100 * perRoute} {
		var walked []int
		for start := 0; start < len(routes); {
			end := relayChunkEnd(routes, start, budget)
			require.Greater(t, end, start, "a chunk must advance the walk")
			walked = append(walked, end-start)
			start = end
		}

		total := 0
		for _, n := range walked {
			total += n
		}
		assert.Equal(t, len(routes), total, "budget %d: chunks %v cover every route once", budget, walked)
	}
}

// TestRelayChunkAlwaysAdvances pins the one-route floor.
//
// VALIDATES: a route whose own serialized size exceeds the budget still forms a
// chunk, so the walk stops.
// PREVENTS: an infinite loop in relayRoutes. The route in this fixture cannot
// exist on the wire, because RFC 8654 Section 3 caps an UPDATE at 65535 octets.
// The floor is a guarantee that the walk stops, not a supported case, and this
// test is what says the guarantee is deliberate.
func TestRelayChunkAlwaysAdvances(t *testing.T) {
	routes := []rpc.StoredRoute{
		{AttrHex: strings.Repeat("a", 4096)},
		{AttrHex: "40010100"},
	}
	assert.Equal(t, 1, relayChunkEnd(routes, 0, 16), "an oversized route travels alone")
	assert.Equal(t, 2, relayChunkEnd(routes, 1, 16), "the walk resumes after it")
}

// TestReplayLastIndexSurvivesMultiChunkRelay is the R-2 assertion: the number
// bgp-rs converges on must not follow the chunk boundaries.
//
// VALIDATES: a replay cut into several relay calls still reports one last-index,
// the highest sequence over ALL its routes, and one replayed count.
// PREVENTS: a delta-replay loop that never terminates or that re-sends, which is
// what a last-index taken from the final chunk would produce.
func TestReplayLastIndexSurvivesMultiChunkRelay(t *testing.T) {
	r := newTestManager(t)

	// Same shared-string trick as the ceiling test: 130 routes, 131 KB of memory.
	attrHex := strings.Repeat("ab", 65535)
	const routeCount = 130
	m := seqmap.New[compactRouteKey, *RawRoute]()
	for i := range routeCount {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i / 256), byte(i % 256), 0}), 24)
		m.Put(routeKeyFromStrings(family.IPv4Unicast, prefix.String(), 0), uint64(i+1), &RawRoute{
			Family: family.IPv4Unicast, AttrHex: attrHex,
			NHopHex: "0a000001", NLRIHex: "180a0000",
		})
	}
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m

	var chunkSizes []int
	r.routeRelayer = func(_ string, routes []rpc.StoredRoute) error {
		chunkSizes = append(chunkSizes, len(routes))
		return nil
	}

	status, data, err := r.handleCommand("request bgp adj-rib-in replay", []string{"127.0.0.2"}, "")
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	require.Greater(t, len(chunkSizes), 1, "the fixture must cross a frame boundary, or it proves nothing")
	relayed := 0
	for _, n := range chunkSizes {
		relayed += n
	}
	assert.Equal(t, routeCount, relayed, "chunks %v carry every stored route", chunkSizes)

	result, ok := data.(map[string]any)
	require.True(t, ok, "replay answers with a map")
	assert.Equal(t, uint64(routeCount), result["last-index"],
		"last-index is the highest sequence in the whole replay, not in the last chunk")
	assert.Equal(t, routeCount, result["replayed"])
}

// TestReplayCommandReportsRelayFailure drives the statusError path that the
// relay seam left unreachable while it returned nothing.
//
// VALIDATES: a relay failure surfaces as statusError with the destination and
// the cause, rather than as a replay the caller reads as complete.
// PREVENTS: bgp-rs running its delta-convergence loop against a replay that
// never happened.
func TestReplayCommandReportsRelayFailure(t *testing.T) {
	r := newTestManager(t)

	m := seqmap.New[compactRouteKey, *RawRoute]()
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m

	relayErr := errors.New("engine refused the frame")
	r.routeRelayer = func(_ string, _ []rpc.StoredRoute) error { return relayErr }

	status, _, err := r.handleCommand("request bgp adj-rib-in replay", []string{"127.0.0.2"}, "")
	assert.Equal(t, statusError, status)
	require.ErrorIs(t, err, relayErr, "the cause reaches the caller")
	assert.Contains(t, err.Error(), "127.0.0.2", "the destination names which replay failed")
}
