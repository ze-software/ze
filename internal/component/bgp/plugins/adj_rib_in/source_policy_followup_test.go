// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection

package adj_rib_in

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC8212SourcePolicyComplexFamilyEligibility proves the selection query
// finds a route of a family keyed by its NLRI identity. Goal: a VPN route the
// source policy denied is reported ineligible, never eligible because the
// query could not find it. Method: store a VPN route under the key a received
// NLRI produces, then ask routeEligible and routePresent with the native NLRI
// a caller carries, before apply, after apply and after abort.
func TestRFC8212SourcePolicyComplexFamilyEligibility(t *testing.T) {
	r := newTestManager(t)
	source := netip.MustParseAddr("10.0.0.1")
	vpn := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	// 112 bits: label 1 with bottom of stack, RD 0:65000:1, 10.1.0.0/24.
	wire := []byte{0x70, 0x00, 0x00, 0x11, 0x00, 0x00, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01, 0x0a, 0x01, 0x00}
	r.mu.Lock()
	key, err := r.routeKeyFromNLRI(vpn, wire, 0, false)
	r.mu.Unlock()
	require.NoError(t, err)
	r.ribIn[source] = newSeqMap()
	r.ribIn[source].Put(key, 1, &RawRoute{Family: vpn, AttrHex: "40010100", MsgID: 100, SourcePolicyEpoch: 5})
	r.peerUp[source] = true
	query := ribevents.ValidationRoute{Peer: source, Family: vpn, NLRI: string(wire)}

	require.True(t, r.routePresent(query), "the query finds the stored VPN route")
	require.True(t, r.routeEligible(query, 0))
	require.True(t, r.routeEligible(query, 100), "the stored generation matches")
	require.False(t, r.routeEligible(query, 101), "another generation does not")
	require.False(t, r.routeEligible(ribevents.ValidationRoute{Peer: source, Family: vpn}, 0),
		"a query that names no route is not eligible")

	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", source, 5, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", source, 5, 6))
	require.False(t, r.routeEligible(query, 0), "a denied VPN route is not eligible")
	require.True(t, r.routePresent(query), "the denied route stays stored for abort")
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", source, 5, 7))
	require.True(t, r.routeEligible(query, 0), "abort restores the VPN route")
}

// TestRFC8212SourcePolicyAbortRestoresPreviousBound proves an abort puts back
// the denial bound an earlier accepted transaction left. Goal: a route that an
// earlier finalized denial excluded never becomes eligible because a later
// transaction was aborted. Method: finalize one denial, admit a newer route,
// then prepare, apply and abort a second transaction, and read the bound.
func TestRFC8212SourcePolicyAbortRestoresPreviousBound(t *testing.T) {
	f := newAdjSourcePolicyFixture(t)
	r := f.r
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", f.source, 5, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyFinalize, "tx1", f.source, 5, 0))
	require.Equal(t, uint64(5), r.policyDenied[f.source])

	r.ribIn[f.source].Put(f.keys[0], 10, &RawRoute{Family: family.IPv4Unicast, AttrHex: "40010100",
		NLRIHex: "180a0100", MsgID: 300, SourcePolicyEpoch: 6})
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx2", f.source, 6, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx2", f.source, 6, 7))
	require.Equal(t, uint64(6), r.policyDenied[f.source])
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx2", f.source, 6, 8))
	require.Equal(t, uint64(5), r.policyDenied[f.source], "abort restores the earlier accepted bound")
	require.True(t, f.eligible(f.source, 0), "the restored route carries the abort epoch")

	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx3", f.source, 8, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx3", f.source, 8, 9))
	require.Equal(t, uint64(5), r.policyDenied[f.source], "an abort before apply leaves the bound alone")
}
