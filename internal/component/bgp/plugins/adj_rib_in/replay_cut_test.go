// Tests for the peer-up replay cut: which stored routes a replay carries, and
// which it leaves to the caller's live forward rail.
//
// Driven from handleCommand, not from buildReplayRoutes, because the defect these
// pin lived in how the command's max-msg-id ARGUMENT was interpreted, not in the
// bound itself (ai/rules/evidence.md: drive the guard from its entry
// point).

package adj_rib_in

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/seqmap"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// replayCutFixture stores one route from 10.0.0.1 carrying the given reactor
// MessageID, and captures whatever a replay relays.
func replayCutFixture(t *testing.T, msgID uint64) (*AdjRIBInManager, *[]rpc.StoredRoute) {
	t.Helper()
	r := newTestManager(t)

	m := seqmap.New[compactRouteKey, *RawRoute]()
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
		MsgID: msgID,
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m

	// newTestManager's SDK plugin has closed connections, so the real relay call
	// fails; the stub is what lets the test observe what the replay selected.
	var relayed []rpc.StoredRoute
	r.routeRelayer = func(_ string, routes []rpc.StoredRoute) {
		relayed = append(relayed, routes...)
	}
	return r, &relayed
}

// TestReplayCutZeroBoundsTheReplay is the regression that closes
// test/plugin/llgr-readvertise-multipeer.ci's duplicate.
//
// VALIDATES: a max-msg-id argument of "0" is a REAL cut -- it excludes every
// stored route with a known MessageID, because the caller has taken delivery of
// none of them yet and its live rail owns all of them.
// PREVENTS: the peer receiving the same UPDATE twice, byte-identical and back to
// back -- once replayed here, once forwarded live. bgp-rs captures the cut as its
// own seenMsgID, which is 0 for every peer that establishes before that plugin has
// processed its first UPDATE (measured on 39 of 40 runs of that test), and
// selectForwardTargets excludes a peer only on `msgID <= ForwardFrom`, never true
// at ForwardFrom 0. While 0 also meant "unbounded" here, both rails carried the
// route.
func TestReplayCutZeroBoundsTheReplay(t *testing.T) {
	r, relayed := replayCutFixture(t, 9)

	status, data, err := r.handleCommand("request bgp adj-rib-in replay",
		[]string{"10.0.0.99", "0", "0"}, "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	assert.Empty(t, *relayed,
		"a cut of 0 means the caller has seen nothing yet, so its live rail owns every known-MessageID route")
	assert.Contains(t, string(mustMarshal(t, data)), `"replayed":0`)
}

// TestReplayCutOmittedIsUnbounded keeps the contract for a caller that tracks no
// cut, which is how this plugin's own self-replay and bgp-rr both call it.
//
// VALIDATES: omitting max-msg-id replays every stored route regardless of MessageID.
// PREVENTS: bounding a replay for a caller that never asked for one, which would
// silently drop the routes a newly-established peer exists to receive.
func TestReplayCutOmittedIsUnbounded(t *testing.T) {
	r, relayed := replayCutFixture(t, 9)

	status, _, err := r.handleCommand("request bgp adj-rib-in replay",
		[]string{"10.0.0.99", "0"}, "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	require.Len(t, *relayed, 1, "no cut argument means no bound")
	assert.Equal(t, "180a0000", (*relayed)[0].NLRIHex)
}

// TestReplayCutKeepsUnknownMessageID pins the one route a bounded replay still
// carries.
//
// VALIDATES: a stored route whose MessageID is unknown (0, the legacy text/JSON
// ingest path) is replayed even under a cut of 0.
// PREVENTS: dropping a route outright because its provenance is unknown. A
// duplicate BGP UPDATE is idempotent at the receiver; a dropped one is not.
func TestReplayCutKeepsUnknownMessageID(t *testing.T) {
	r, relayed := replayCutFixture(t, 0)

	status, _, err := r.handleCommand("request bgp adj-rib-in replay",
		[]string{"10.0.0.99", "0", "0"}, "")
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	require.Len(t, *relayed, 1,
		"an unknown MessageID cannot be attributed to either rail, so it stays with the replay")
}

// TestReplayCutBoundaryIsInclusive pins the exact boundary the two rails meet at.
//
// VALIDATES: a route AT the cut is replayed; the next MessageID above it is not.
// PREVENTS: an off-by-one at the seam. rs/server_forward.go selectForwardTargets
// excludes a peer for `msgID <= ForwardFrom`, so the replay must own exactly
// `msgID <= cut` -- one notch either way is a duplicate or a lost route.
func TestReplayCutBoundaryIsInclusive(t *testing.T) {
	for _, tc := range []struct {
		name   string
		msgID  uint64
		cutArg string
		want   int
	}{
		{"below the cut", 4, "5", 1},
		{"at the cut", 5, "5", 1},
		{"one above the cut", 6, "5", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, relayed := replayCutFixture(t, tc.msgID)

			status, _, err := r.handleCommand("request bgp adj-rib-in replay",
				[]string{"10.0.0.99", "0", tc.cutArg}, "")
			require.NoError(t, err)
			assert.Equal(t, statusDone, status)
			assert.Len(t, *relayed, tc.want)
		})
	}
}

// TestReplayCutRejectsMalformedArgument keeps the parse failure loud.
//
// VALIDATES: a non-numeric max-msg-id is an error, not a silent fallback to
// unbounded.
// PREVENTS: a typo or a truncated argument quietly restoring the doubled-delivery
// behavior this cut exists to stop.
func TestReplayCutRejectsMalformedArgument(t *testing.T) {
	r, relayed := replayCutFixture(t, 9)

	status, _, err := r.handleCommand("request bgp adj-rib-in replay",
		[]string{"10.0.0.99", "0", "not-a-number"}, "")
	require.Error(t, err)
	assert.Equal(t, statusError, status)
	assert.Contains(t, err.Error(), "invalid max-msg-id")
	assert.Empty(t, *relayed, "a rejected command relays nothing")
}
