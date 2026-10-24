// Design: docs/architecture/route-selection.md -- Step 0, stale depreference
// RFC: rfc/short/rfc9494.md -- Section 4.3, a received LLGR_STALE route is least preferred
// Related: rib_structured.go -- handleReceivedStructured writes ReceivedLLGRStale from meta
// Related: rib_commands.go -- extractCandidate hands best path the effective level

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// receivedStaleUpdate announces 10.0.0.0/8 with LOCAL_PREF 200 and a COMMUNITIES
// attribute holding the one community given.
func receivedStaleUpdate(community [4]byte) []byte {
	return []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x1c, // Total Path Attribute Length 28
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0xc8, // LOCAL_PREF = 200
		0xc0, 0x08, 0x04, community[0], community[1], community[2], community[3], // COMMUNITIES
		0x08, 0x0a, // NLRI 10.0.0.0/8
	}
}

// receivedPlainUpdate announces 10.0.0.0/8 with LOCAL_PREF 200 and no COMMUNITIES.
func receivedPlainUpdate() []byte {
	return []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x15, // Total Path Attribute Length 21
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0xc8, // LOCAL_PREF = 200
		0x08, 0x0a, // NLRI 10.0.0.0/8
	}
}

var (
	communityLLGRStaleWire = [4]byte{0xff, 0xff, 0x00, 0x06}
	communityNoExportWire  = [4]byte{0xff, 0xff, 0xff, 0x01}
)

// ingressStaleMeta is what the GR ingress step (gr_ingress.go) writes for a
// received LLGR_STALE route from a neighbor Ze sent the LLGR Capability to. A
// route from any other neighbor reaches the RIB with no meta at all.
func ingressStaleMeta() map[string]any {
	return map[string]any{"stale": uint8(2)}
}

// feedReceivedIBGPMeta is feedReceivedIBGP with the route metadata the
// reactor's ingress steps put on the received event.
func feedReceivedIBGPMeta(r *RIBManager, peer netip.Addr, ctxID bgpctx.ContextID, body []byte, meta map[string]any) {
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, _ := wu.Attrs()
	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: peer.String(),
		PeerAS:      rpc.RemoteASOf(65000),
		LocalAS:     65000,
		Meta:        meta,
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   body,
			WireUpdate: wu,
			AttrsWire:  attrs,
			Meta:       meta,
		},
	})
}

// receivedStaleSetup feeds the contest every test below runs: the LOWER peer
// address announces 10.0.0.0/8 at LOCAL_PREF 200 with the given community and
// ingress metadata, the higher one announces it fresh with no LOCAL_PREF (the
// default 100). Every criterion after Step 0 prefers the first route.
func receivedStaleSetup(t *testing.T, community [4]byte, meta map[string]any) (*RIBManager, bgpctx.ContextID, netip.Addr, netip.Addr) {
	t.Helper()
	r := newTestRIBManager(t)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	low := netip.MustParseAddr("192.0.2.1")
	high := netip.MustParseAddr("192.0.2.2")
	feedReceivedIBGPMeta(r, low, ctxID, receivedStaleUpdate(community), meta)
	feedReceivedIBGP(r, high, ctxID, identifierTestUpdate())
	return r, ctxID, low, high
}

// receivedStaleWinner runs the decision process over the two candidates.
func receivedStaleWinner(t *testing.T, r *RIBManager) *bestPathExplanation {
	t.Helper()
	candidates := gatherCandidatesHeld(t, r, family.IPv4Unicast, []byte{8, 10}, false)
	require.Len(t, candidates, 2, "both peers announced the prefix")
	explanation := SelectBestExplain(candidates)
	require.NotNil(t, explanation)
	require.Len(t, explanation.Steps, 1)
	return explanation
}

// TestRFC9494ReceivedLLGRStaleIsLeastPreferred verifies a route a neighbor sends
// with LLGR_STALE loses route selection to a fresh route, through the received
// UPDATE entry point.
//
// VALIDATES: handleReceivedStructured stores the GR ingress step's stale level
// and extractCandidate raises the candidate to the depreference level, so Step 0
// decides against it although it has the higher LOCAL_PREF and the lower peer
// address.
// PREVENTS: a received LLGR_STALE route competing as a fresh one, which it did
// while only a route Ze marked stale itself was depreferenced.
//
// RFC requirement: RFC9494-4.3-1 positive -- a route received carrying LLGR_STALE at
// LOCAL_PREF 200 from 192.0.2.1, which the ingress step marked stale level 2 because
// Ze sent that neighbor the LLGR Capability, loses to a fresh LOCAL_PREF 100 route from 192.0.2.2,
// decided at BestStepStale: "upon receiving a route from that neighbor with the
// LLGR_STALE community ... Treat the route as the least preferred in route selection".
// MUTATION: drop the ReceivedLLGRStale branch in extractCandidate, and this goes red.
func TestRFC9494ReceivedLLGRStaleIsLeastPreferred(t *testing.T) {
	r, _, _, high := receivedStaleSetup(t, communityLLGRStaleWire, ingressStaleMeta())

	explanation := receivedStaleWinner(t, r)
	assert.Equal(t, BestStepStale, explanation.Steps[0].Step, "Step 0 decides")
	assert.Equal(t, high.String(), explanation.Winner.PeerAddr, "the fresh route wins")
}

// TestRFC9494ReceivedOtherCommunityCompetesNormally verifies the treatment is
// confined to LLGR_STALE: the same route carrying NO_EXPORT instead wins on
// LOCAL_PREF.
//
// VALIDATES: any other COMMUNITIES value leaves the route fresh.
// PREVENTS: a check that depreferences every route that carries COMMUNITIES.
//
// RFC requirement: RFC9494-4.3-1 negative -- the same contest with NO_EXPORT in place of
// LLGR_STALE is decided at BestStepLocalPref for the 192.0.2.1 route.
func TestRFC9494ReceivedOtherCommunityCompetesNormally(t *testing.T) {
	r, _, low, _ := receivedStaleSetup(t, communityNoExportWire, nil)

	explanation := receivedStaleWinner(t, r)
	assert.Equal(t, BestStepLocalPref, explanation.Steps[0].Step)
	assert.Equal(t, low.String(), explanation.Winner.PeerAddr, "LOCAL_PREF 200 wins")
}

// TestRFC9494ReceivedLLGRStaleRefreshedFreshCompetesNormally verifies a refresh
// of the route without LLGR_STALE returns it to normal selection.
//
// VALIDATES: the received mark follows the route's current attributes.
// PREVENTS: a received mark that outlives the UPDATE that carried it.
func TestRFC9494ReceivedLLGRStaleRefreshedFreshCompetesNormally(t *testing.T) {
	r, ctxID, low, _ := receivedStaleSetup(t, communityLLGRStaleWire, ingressStaleMeta())

	feedReceivedIBGP(r, low, ctxID, receivedPlainUpdate())

	explanation := receivedStaleWinner(t, r)
	assert.Equal(t, BestStepLocalPref, explanation.Steps[0].Step)
	assert.Equal(t, low.String(), explanation.Winner.PeerAddr)
}

// TestRFC9494ReceivedLLGRStaleSurvivesPurgeStale verifies the received mark is
// not the retention mark: purge-stale, which the GR plugin runs at TCP loss and
// at each EOR, leaves the route in place and still least preferred.
//
// VALIDATES: a received LLGR_STALE route is retained like any fresh route.
// PREVENTS: storing the received mark in StaleLevel, which purge-stale deletes on.
func TestRFC9494ReceivedLLGRStaleSurvivesPurgeStale(t *testing.T) {
	r, _, low, high := receivedStaleSetup(t, communityLLGRStaleWire, ingressStaleMeta())

	status, _, err := r.purgeStaleCommand([]string{low.String()})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	explanation := receivedStaleWinner(t, r)
	assert.Equal(t, BestStepStale, explanation.Steps[0].Step)
	assert.Equal(t, high.String(), explanation.Winner.PeerAddr)
}

// TestRFC9494ReceivedLLGRStaleFromNonLLGRNeighborCompetesNormally verifies the
// same LLGR_STALE route from a neighbor Ze did not send the LLGR Capability to
// competes as a fresh one: the GR ingress step wrote no stale level for it.
//
// VALIDATES: AC-14: the RIB depreferences on the ingress step's decision, never
// on the community alone.
// PREVENTS: a RIB that reads LLGR_STALE from the attributes and applies the
// Section 4.3 treatment to a neighbor the obligation does not cover.
//
// RFC requirement: RFC9494-4.3-1 negative -- the LLGR_STALE route at LOCAL_PREF 200 from
// 192.0.2.1 with no ingress stale level is decided at BestStepLocalPref and wins.
func TestRFC9494ReceivedLLGRStaleFromNonLLGRNeighborCompetesNormally(t *testing.T) {
	r, _, low, _ := receivedStaleSetup(t, communityLLGRStaleWire, nil)

	explanation := receivedStaleWinner(t, r)
	assert.Equal(t, BestStepLocalPref, explanation.Steps[0].Step)
	assert.Equal(t, low.String(), explanation.Winner.PeerAddr, "LOCAL_PREF 200 wins")
}

// TestRFC9494ReceivedStaleEqualRefreshFollowsLatestUpdate verifies the mark is
// written on the no-op insert branch: the byte-identical UPDATE re-sent with a
// different ingress decision moves the route in both directions.
//
// VALIDATES: AC-15 equal-attribute branch: the stored entry is kept and its mark
// rewritten, fresh to stale and back.
// PREVENTS: a mark that only a changed attribute set can update.
func TestRFC9494ReceivedStaleEqualRefreshFollowsLatestUpdate(t *testing.T) {
	r, ctxID, low, high := receivedStaleSetup(t, communityLLGRStaleWire, nil)

	feedReceivedIBGPMeta(r, low, ctxID, receivedStaleUpdate(communityLLGRStaleWire), ingressStaleMeta())
	explanation := receivedStaleWinner(t, r)
	assert.Equal(t, BestStepStale, explanation.Steps[0].Step, "the re-sent UPDATE marked stale")
	assert.Equal(t, high.String(), explanation.Winner.PeerAddr)

	feedReceivedIBGPMeta(r, low, ctxID, receivedStaleUpdate(communityLLGRStaleWire), nil)
	explanation = receivedStaleWinner(t, r)
	assert.Equal(t, BestStepLocalPref, explanation.Steps[0].Step, "the re-sent UPDATE unmarked")
	assert.Equal(t, low.String(), explanation.Winner.PeerAddr)
}
