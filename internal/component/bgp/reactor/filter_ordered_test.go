package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The egress/ingress policy chains are guards whose purpose is to reject. Their
// former guard fused two unrelated conditions into one permissive early return:
//
//	if len(exportFilters) == 0 || r.api == nil { return egressStepResult{accept: true} }
//
// len == 0 is a legitimate accept (no export/import policy configured); r.api ==
// nil while filters ARE configured is a guard MISS -- the filter engine that
// enforces the policy is absent. Returning accept: true silently sent the route
// UNFILTERED, the exact fail-open the siblings policyFilterFunc
// (filter_chain.go:368-371: Warn + PolicyReject) and default-originate
// (peer_initial_sync.go:718-722) already reject loudly. These tests drive the
// reactor's own chain methods (the guard's entry points -- forwardUpdateCore
// calls runEgressPolicyChain at reactor_api_forward.go:500; reactor_notify.go:424
// calls runIngressPolicyChain) and pin each split case.
//
// captureWarnPeers and peerWithExportFilters live in the sibling test files
// (egress_inject_filter_test.go); testWireUpdate lives in session_prefix_test.go.
//
// Spec: plan/spec-fixit-private-asn-leak-deferred-nil-api-fail-open.md (AC-1, AC-2, AC-4).

// TestRunEgressPolicyChainASN4NilAPIWithExportFiltersFailsClosed drives the
// shared export chain body -- the one BOTH egress paths reach.
//
// VALIDATES: AC-1. runEgressPolicyChainASN4 with export filters + nil api must
// suppress (accept == false) AND Warn, matching filter_chain.go:368-371.
// PREVENTS: a peer with an export policy sending routes UNFILTERED when the API
// server is absent (an RFC 6996 private-ASN / policy leak), silently.
func TestRunEgressPolicyChainASN4NilAPIWithExportFiltersFailsClosed(t *testing.T) {
	rec := captureWarnPeers(t)

	r := &Reactor{} // r.api == nil: the whole subject
	filters := []filterapi.FilterRef{{Name: "reject-private-asn"}}

	res := r.runEgressPolicyChainASN4(filters, "10.0.0.1", 65001, 65000, testWireUpdate([]byte{0, 0, 0, 0}), false)

	require.False(t, res.accept,
		"nil API server with export filters configured must suppress the route (fail closed), not accept it unfiltered")
	assert.Nil(t, res.wireOverride, "a suppressed route carries no wire override")
	require.Contains(t, rec.warnedPeers(), "10.0.0.1",
		"the guard miss must be logged at Warn naming the peer, not silently dropped")
}

// TestRunEgressPolicyChainNilAPIWithExportFiltersFailsClosed drives the
// forwarded-route egress entry, which delegates to the shared body.
//
// VALIDATES: AC-1. runEgressPolicyChain must not accept before delegating; nil
// api + export filters suppresses and warns.
// PREVENTS: the forwarded (route-reflected) path leaking unfiltered routes when
// the split fixes only the shared body but leaves the entry permissive.
func TestRunEgressPolicyChainNilAPIWithExportFiltersFailsClosed(t *testing.T) {
	rec := captureWarnPeers(t)

	r := &Reactor{}
	filters := []filterapi.FilterRef{{Name: "reject-private-asn"}}

	res := r.runEgressPolicyChain(filters, "10.0.0.2", 65001, 65000, testWireUpdate([]byte{0, 0, 0, 0}))

	require.False(t, res.accept,
		"forwarded egress path: nil API server with export filters must suppress, not accept")
	require.Contains(t, rec.warnedPeers(), "10.0.0.2",
		"the guard miss must be logged at Warn naming the peer")
}

// TestRunEgressPolicyChainNoExportFiltersAccepts pins the legitimate accept.
//
// VALIDATES: AC-2. No export policy configured is not a guard miss: accept, no
// warn, even with a nil api.
// PREVENTS: the fail-closed split over-firing and denying peers that simply have
// no export policy.
func TestRunEgressPolicyChainNoExportFiltersAccepts(t *testing.T) {
	rec := captureWarnPeers(t)

	r := &Reactor{}

	res := r.runEgressPolicyChain(nil, "10.0.0.3", 65001, 65000, testWireUpdate([]byte{0, 0, 0, 0}))

	require.True(t, res.accept,
		"no export policy configured is a legitimate accept, not a guard miss")
	assert.NotContains(t, rec.warnedPeers(), "10.0.0.3",
		"no filters is not a miss -- it must not warn")
}

// TestRunIngressPolicyChainNilAPIWithImportFiltersFailsClosed gives the ingress
// chain the same fail-closed treatment (AC-4: same, not different).
//
// VALIDATES: AC-4. An import filter is equally a guard; nil api + import filters
// drops the route (accept == false) AND warns.
// PREVENTS: accepting unfiltered INBOUND routes (bypassing a security/ACL import
// policy) when the filter engine is absent.
func TestRunIngressPolicyChainNilAPIWithImportFiltersFailsClosed(t *testing.T) {
	rec := captureWarnPeers(t)

	r := &Reactor{}
	addr := netip.MustParseAddr("10.0.0.4")
	peer := NewPeer(&PeerSettings{
		Address:       addr,
		LocalAS:       65000,
		PeerAS:        65001,
		ImportFilters: []filterapi.FilterRef{{Name: "block-bogons"}},
	})

	res := r.runIngressPolicyChain(peer, addr, 65001, testWireUpdate([]byte{0, 0, 0, 0}), []byte{0, 0, 0, 0})

	require.False(t, res.accept,
		"ingress: nil API server with import filters must drop the route (fail closed), not accept unfiltered")
	require.Contains(t, rec.warnedPeers(), "10.0.0.4",
		"the ingress guard miss must be logged at Warn naming the peer")
}

// TestRunIngressPolicyChainNoImportFiltersAccepts pins the ingress accept.
//
// VALIDATES: AC-2 (ingress side). No import policy configured is a legitimate
// accept: accept, no warn, even with a nil api.
// PREVENTS: the ingress split denying peers that simply have no import policy.
func TestRunIngressPolicyChainNoImportFiltersAccepts(t *testing.T) {
	rec := captureWarnPeers(t)

	r := &Reactor{}
	addr := netip.MustParseAddr("10.0.0.5")
	peer := NewPeer(&PeerSettings{
		Address: addr,
		LocalAS: 65000,
		PeerAS:  65001,
	})

	res := r.runIngressPolicyChain(peer, addr, 65001, testWireUpdate([]byte{0, 0, 0, 0}), []byte{0, 0, 0, 0})

	require.True(t, res.accept,
		"no import policy configured is a legitimate accept, not a guard miss")
	assert.NotContains(t, rec.warnedPeers(), "10.0.0.5",
		"no filters is not a miss -- it must not warn")
}
