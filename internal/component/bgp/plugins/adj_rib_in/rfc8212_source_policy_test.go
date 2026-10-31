// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on, not the Section 3 verdict itself, which rfc8212_default_reject_test.go and the test/plugin/rfc8212-* carriers prove.
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: source_policy.go -- the handler these tests drive

package adj_rib_in

import (
	"context"
	"net/netip"
	"testing"
	"time"
	"unique"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// runSourcePolicy calls the registered handler with one validated request.
func runSourcePolicy(t *testing.T, r *AdjRIBInManager, op rpc.SourcePolicyOperation, txn string, peer netip.Addr, epoch, next uint64) error {
	t.Helper()
	input := rpc.SourcePolicyInput{TransactionID: txn, Operation: op,
		Source: rpc.SourcePolicyIdentity{Peer: peer.String(), PeerOwner: 1, SessionGeneration: 3,
			PolicyEpoch: epoch, ReceiveFrontier: 9, TemplateGeneration: 1},
		NextPolicyEpoch: next, DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}
	require.NoError(t, input.Validate(time.Now()))
	return r.handleSourcePolicy(context.Background(), input)
}

// adjSourcePolicyFixture stores, for source, IPv4 unicast 10.1.0.0/24 under
// ADD-PATH identifiers zero and seven and one VPN route, all admitted at
// epoch 5, and one route of another source.
type adjSourcePolicyFixture struct {
	r      *AdjRIBInManager
	source netip.Addr
	other  netip.Addr
	keys   []compactRouteKey
}

func newAdjSourcePolicyFixture(t *testing.T) *adjSourcePolicyFixture {
	t.Helper()
	r := newTestManager(t)
	f := &adjSourcePolicyFixture{r: r, source: netip.MustParseAddr("10.0.0.1"), other: netip.MustParseAddr("10.0.0.2")}
	prefix := netip.MustParsePrefix("10.1.0.0/24")
	f.keys = []compactRouteKey{
		{Fam: family.IPv4Unicast, Prefix: prefix, PathID: 0},
		{Fam: family.IPv4Unicast, Prefix: prefix, PathID: 7},
		{Fam: family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}, NLRI: unique.Make("vpn-route")},
	}
	r.ribIn[f.source] = newSeqMap()
	for i, key := range f.keys {
		r.ribIn[f.source].Put(key, uint64(i+1), &RawRoute{Family: key.Fam, AttrHex: "40010100",
			NLRIHex: "180a0100", PathID: key.PathID, HasPathID: key.Fam == family.IPv4Unicast,
			MsgID: uint64(100 + i), SourcePolicyEpoch: 5})
	}
	r.ribIn[f.other] = newSeqMap()
	r.ribIn[f.other].Put(f.keys[0], 9, &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100", NLRIHex: "180a0100", MsgID: 200, SourcePolicyEpoch: 5})
	r.peerUp[f.source], r.peerUp[f.other] = true, true
	return f
}

// eligible asks the selection query for the source's unicast path id.
func (f *adjSourcePolicyFixture) eligible(peer netip.Addr, pathID uint32) bool {
	return f.r.routeEligible(ribevents.ValidationRoute{Peer: peer, Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("10.1.0.0/24"), PathID: pathID}, 0)
}

// replayed counts the source's routes a replay to a third peer carries.
func (f *adjSourcePolicyFixture) replayed(t *testing.T) int {
	t.Helper()
	routes, _ := f.r.buildReplayRoutes(netip.MustParseAddr("10.0.0.3"), 0, replayCut{})
	count := 0
	for i := range routes {
		if routes[i].SourcePeer == f.source.String() {
			count++
		}
	}
	return count
}

// TestRFC8212SourcePolicyAdjRIBInLifecycle drives prepare, apply, abort and
// finalize through bgp-adj-rib-in's registered handler.
//
// VALIDATES: apply makes the source's paths, ADD-PATH identifiers zero and
// nonzero and a VPN route, ineligible for selection and replay while another
// source stays eligible; a duplicate apply returns its recorded outcome; abort
// restores the same paths under the fresh epoch; finalize after abort is
// refused.
// PREVENTS: a denied path replayed or selected, and a rollback that loses it.
func TestRFC8212SourcePolicyAdjRIBInLifecycle(t *testing.T) {
	f := newAdjSourcePolicyFixture(t)
	r := f.r
	require.Equal(t, 3, f.replayed(t))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", f.source, 5, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6))
	require.False(t, f.eligible(f.source, 0), "ADD-PATH identifier zero is denied")
	require.False(t, f.eligible(f.source, 7), "ADD-PATH identifier seven is denied")
	require.True(t, f.eligible(f.other, 0), "another source is untouched")
	require.Equal(t, 0, f.replayed(t), "a denied path, VPN included, is not replayed")
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6), "duplicate apply")
	require.Equal(t, 3, r.ribIn[f.source].Len(), "apply keeps the paths for abort")

	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", f.source, 5, 7))
	require.True(t, f.eligible(f.source, 0))
	require.True(t, f.eligible(f.source, 7))
	require.Equal(t, 3, f.replayed(t), "abort restores replay without Route Refresh")
	r.ribIn[f.source].Range(func(_ compactRouteKey, _ uint64, route *RawRoute) bool {
		require.Equal(t, uint64(7), route.SourcePolicyEpoch, "a restored path carries the fresh epoch")
		return true
	})
	require.Nil(t, r.sourcePolicy[f.source].installed, "abort releases the pins")
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyFinalize, "tx1", f.source, 5, 0), "finalize after abort is refused")
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", f.source, 5, 8), "a second abort with another target is refused")
}

// TestRFC8212SourcePolicyAdjRIBInFinalizeAndRefusals proves an accepted
// denial keeps no denied state, a withdrawn path is not resurrected, and a
// stale, foreign or unprepared token changes nothing.
func TestRFC8212SourcePolicyAdjRIBInFinalizeAndRefusals(t *testing.T) {
	f := newAdjSourcePolicyFixture(t)
	r := f.r
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "old", f.source, 4, 0), "a token older than an admitted path")
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6), "apply without prepare")
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", f.source, 5, 0))
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "other", f.source, 5, 6), "a foreign transaction")
	require.True(t, f.eligible(f.source, 0), "no refusal changed eligibility")

	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6))
	r.mu.Lock()
	r.ribIn[f.source].Delete(f.keys[1]) // The source withdrew identifier seven.
	r.mu.Unlock()
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", f.source, 5, 7))
	require.True(t, f.eligible(f.source, 0))
	require.False(t, f.eligible(f.source, 7), "a withdrawn path is not restored")

	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx2", f.source, 7, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx2", f.source, 7, 8))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyFinalize, "tx2", f.source, 7, 0))
	require.Equal(t, 0, r.ribIn[f.source].Len(), "denied paths are not retained for a later recovery")
	require.Equal(t, 1, r.ribIn[f.other].Len())

	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx3", f.source, 8, 0))
	r.mu.Lock()
	r.removePeerInstalled(f.source) // The session ended.
	r.mu.Unlock()
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx3", f.source, 8, 9), "the session that prepared ended")
}
