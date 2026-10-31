package redistribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// TestUntaggedRouteMeetsMatchTagZero proves that `match tag [ 0 ]` on a
// redistribute import selects an untagged route, as the old `tag 0` leaf did and
// as FRR's route_match_tag does: an untagged route carries tag 0.
//
// VALIDATES: redistRoute presents the tag of every redistributed route, zero
// when the producer set none, so a compiled match chain compares it.
// PREVENTS: an untagged static route that `match tag [ 0 ]` silently drops,
// because the view called the zero tag absent.
func TestUntaggedRouteMeetsMatchTagZero(t *testing.T) {
	id := redistevents.RegisterProtocol("fakeredist")
	chain, err := policy.CompileRouteChain(chainAttachment, []string{"T0"}, policy.FinalAccept,
		map[string]any{"match": map[string]any{"T0": map[string]any{"tag": []any{"0"}}}})
	require.NoError(t, err)

	untagged := &redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.0.0.0/24")}
	view := redistRoute("fakeredist", id, family.IPv4Unicast, untagged)
	require.Equal(t, ribpolicy.ActionAccept, chain.Evaluate(view.Route).Action, "an untagged route meets match tag [ 0 ]")

	tagged := &redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.0.1.0/24"), Tag: 7}
	view = redistRoute("fakeredist", id, family.IPv4Unicast, tagged)
	require.Equal(t, ribpolicy.ActionReject, chain.Evaluate(view.Route).Action, "a route tagged 7 fails match tag [ 0 ]")
}
