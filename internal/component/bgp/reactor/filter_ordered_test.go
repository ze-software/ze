package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

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

// The modify-failure guard, driven from its own entry point.
//
// T1-1 made five call sites fail closed when buildModifiedPayload reports that
// it could not apply the modifications it was given. Four of the five were
// tested; the two in this file were not. Every test that touched the branch
// called buildModifiedPayload DIRECTLY, which proves the helper reports the
// failure and says nothing about whether these two callers act on it
// (ai/rules/evidence.md: drive the guard from its entry point, never
// the helper alone).
//
// The ingress one is the sharper of the two. It converts an import-modify
// failure from accept-unmodified into a DROP on the receive path, so an
// unexercised guard here is silent route loss rather than a leak.
//
// Reaching the branch needs the policy chain to return a text delta, which needs
// a filter answer. r.api is a concrete *pluginserver.Server and its answer comes
// off a live plugin socket, so these tests set r.policyFilterSeam (nil in
// production, see filter_chain.go policyFilterFunc) and leave EVERYTHING else on
// the real path: the real chain runs, the real delta-to-ops extraction runs, and
// the real buildModifiedPayload refuses.
//
// The refusal is induced the way a deployment would meet it: attrModHandlers has
// no entry for the code the filter asked to change, so the operation is never
// applied and the build reports modifyFailureNoHandler. Forwarding then would
// send a route the policy had not changed.

// modifyingFilter returns a seam that answers every filter call with a text
// delta setting LOCAL_PREF, which textDeltaToModOps turns into one AttrModSet
// operation on code 5.
func modifyingFilter() PolicyFilterFunc {
	return func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		return PolicyResponse{Action: PolicyModify, Delta: "local-preference 200"}
	}
}

// VALIDATES: AC-12. runIngressPolicyChain drops the route (accept == false) when
// the import chain asked for a modification that could not be applied.
// PREVENTS: installing a route the import policy rejected. An import filter can
// be security or ACL policy, so accepting the unmodified route puts exactly what
// the policy exists to strip into the RIB.
func TestRunIngressPolicyChainModifyFailureFailsClosed(t *testing.T) {
	addr := netip.MustParseAddr("10.0.0.6")
	peer := NewPeer(&PeerSettings{
		Address:       addr,
		LocalAS:       65000,
		PeerAS:        65001,
		ImportFilters: []filterapi.FilterRef{{Name: "set-local-pref"}},
	})

	r := &Reactor{
		api:              &pluginserver.Server{}, // non-nil: past the r.api guard
		policyFilterSeam: modifyingFilter(),
		// No handler for LOCAL_PREF, so the operation the filter asked for is
		// never applied and the build reports it.
		attrModHandlers: map[uint8]filterapi.AttrModHandler{},
	}

	body := []byte{0, 0, 0, 0} // withdrawn-len 0, attr-len 0
	res := r.runIngressPolicyChain(peer, addr, 65001, testWireUpdate(body), body)

	require.False(t, res.accept,
		"an import modification that could not be applied must DROP the route, never accept it unmodified")
	assert.Nil(t, res.modifiedPayload, "a dropped route carries no payload")
	assert.False(t, res.teardown, "a modify failure is not a session-teardown request")
}

// VALIDATES: AC-12. runEgressPolicyChainASN4 suppresses the route for this peer
// (accept == false) AND marks failed, when the export chain asked for a
// modification that could not be applied.
// PREVENTS: sending the route UNFILTERED, which leaks whatever the export policy
// was stripping (RFC 6996 private ASNs, RFC 7947 control communities). failed
// also keeps the stored-route relay's completeness check from recording a step
// that could not run as "policy said no".
func TestRunEgressPolicyChainASN4ModifyFailureFailsClosed(t *testing.T) {
	r := &Reactor{
		api:              &pluginserver.Server{},
		policyFilterSeam: modifyingFilter(),
		attrModHandlers:  map[uint8]filterapi.AttrModHandler{},
	}
	filters := []filterapi.FilterRef{{Name: "set-local-pref"}}

	body := []byte{0, 0, 0, 0}
	res := r.runEgressPolicyChainASN4(filters, "10.0.0.7", 65001, 65000, testWireUpdate(body), false)

	require.False(t, res.accept,
		"an export modification that could not be applied must suppress the route, never send it unmodified")
	require.True(t, res.failed,
		"the chain COULD NOT RUN to completion; that is not a policy decision (egressStepResult.failed)")
	assert.Nil(t, res.wireOverride, "a suppressed route carries no wire override")
}

// VALIDATES: AC-12. runEgressPolicyChain, the forwarded-route entry, reaches the
// same guard through the shared body rather than accepting first.
// PREVENTS: the forwarded (route-reflected) rail leaking when a fix lands only on
// the shared body. This is the same split the nil-api tests above pin.
func TestRunEgressPolicyChainModifyFailureFailsClosed(t *testing.T) {
	r := &Reactor{
		api:              &pluginserver.Server{},
		policyFilterSeam: modifyingFilter(),
		attrModHandlers:  map[uint8]filterapi.AttrModHandler{},
	}
	filters := []filterapi.FilterRef{{Name: "set-local-pref"}}

	body := []byte{0, 0, 0, 0}
	res := r.runEgressPolicyChain(filters, "10.0.0.8", 65001, 65000, testWireUpdate(body))

	require.False(t, res.accept, "forwarded egress path must suppress on a modify failure")
	require.True(t, res.failed, "a step that could not run is not a policy decision")
}

// The raw-override guard, driven from its own entry points.
//
// A raw=true filter answers with a full UPDATE-body replacement rather than a
// text delta, for surgery the delta cannot express. decodeFilterRawOverride
// (filter_chain.go) returns nil for anything shorter than four bytes, which is
// the shortest possible body (withdrawn-len(2) + attr-len(2)). Nil had ONE
// meaning at both call sites: "no raw override", so a filter that asked for a
// replacement it could not encode was indistinguishable from a filter that never
// asked, and the route went out UNMODIFIED carrying exactly what the raw filter
// was rewriting.
//
// PolicyFilterChain makes a raw response terminal and returns Text: current, so
// the text-delta branch below the raw check does not run either: there is no
// second chance to notice.
//
// Spec: plan/spec-fixit-stored-route-relay-hardening.md (AC-9).

// shortRawFilter returns a seam answering with a modify whose raw override is
// too short to be an UPDATE body. Three bytes: one below the four-byte minimum,
// which is the boundary the guard turns on.
func shortRawFilter() PolicyFilterFunc {
	return func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		return PolicyResponse{Action: PolicyModify, Raw: []byte{0x00, 0x00, 0x00}}
	}
}

// VALIDATES: AC-9. runEgressPolicyChainASN4 suppresses (accept == false) AND
// marks failed when a raw filter's override cannot be decoded.
// PREVENTS: forwarding the ORIGINAL body after a filter asked for it to be
// replaced -- the RFC 6996 private-ASN leak class this file exists to stop, and
// silently, because the discarded nil looked exactly like "no override".
func TestRunEgressPolicyChainASN4ShortRawOverrideFailsClosed(t *testing.T) {
	rec := captureWarnPeers(t)

	r := &Reactor{
		api:              &pluginserver.Server{},
		policyFilterSeam: shortRawFilter(),
		attrModHandlers:  map[uint8]filterapi.AttrModHandler{},
	}
	filters := []filterapi.FilterRef{{Name: "mp-reach-surgery"}}

	body := []byte{0, 0, 0, 0}
	res := r.runEgressPolicyChainASN4(filters, "10.0.0.11", 65001, 65000, testWireUpdate(body), false)

	require.False(t, res.accept,
		"a raw override that cannot be decoded must suppress the route, never send the original unmodified")
	require.True(t, res.failed,
		"the filter asked for a replacement ze could not apply; that is not a policy decision (egressStepResult.failed)")
	assert.Nil(t, res.wireOverride, "a suppressed route carries no wire override")
	require.Contains(t, rec.warnedPeers(), "10.0.0.11",
		"the guard miss must be logged at Warn naming the peer, not silently discarded")
}

// VALIDATES: AC-9 (ingress side). runIngressPolicyChain drops the route when a
// raw filter's override cannot be decoded.
// PREVENTS: caching and dispatching the unmodified route on the RECEIVE path,
// which is route content the import policy had rewritten reaching the RIB.
func TestRunIngressPolicyChainShortRawOverrideFailsClosed(t *testing.T) {
	rec := captureWarnPeers(t)

	addr := netip.MustParseAddr("10.0.0.12")
	peer := NewPeer(&PeerSettings{
		Address:       addr,
		LocalAS:       65000,
		PeerAS:        65001,
		ImportFilters: []filterapi.FilterRef{{Name: "mp-reach-surgery"}},
	})
	r := &Reactor{
		api:              &pluginserver.Server{},
		policyFilterSeam: shortRawFilter(),
		attrModHandlers:  map[uint8]filterapi.AttrModHandler{},
	}

	body := []byte{0, 0, 0, 0}
	res := r.runIngressPolicyChain(peer, addr, 65001, testWireUpdate(body), body)

	require.False(t, res.accept,
		"an import raw override that cannot be decoded must DROP the route, never accept it unmodified")
	assert.Nil(t, res.modifiedPayload, "a dropped route carries no payload")
	assert.False(t, res.teardown, "an undecodable raw override is not a session-teardown request")
	require.Contains(t, rec.warnedPeers(), "10.0.0.12",
		"the ingress guard miss must be logged at Warn naming the peer")
}

// VALIDATES: AC-9 (the no-over-fire half). An EMPTY raw stays the ordinary "no
// override" case, and a four-byte raw -- the shortest legal body, the exact
// boundary -- is still applied.
// PREVENTS: the new guard turning every text-delta filter into a refusal, and
// pins the boundary at one-below/at rather than only at an extreme, which is the
// fixture trap in ai/rules/interop-and-goal-validation.md.
func TestPolicyChainRawOverrideBoundary(t *testing.T) {
	body := []byte{0, 0, 0, 0}

	t.Run("empty raw is not an override", func(t *testing.T) {
		rec := captureWarnPeers(t)
		r := &Reactor{
			api: &pluginserver.Server{},
			policyFilterSeam: func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
				return PolicyResponse{Action: PolicyAccept}
			},
			attrModHandlers: map[uint8]filterapi.AttrModHandler{},
		}

		res := r.runEgressPolicyChainASN4([]filterapi.FilterRef{{Name: "plain"}},
			"10.0.0.13", 65001, 65000, testWireUpdate(body), false)

		require.True(t, res.accept, "a filter that asked for no raw override must still accept")
		assert.False(t, res.failed, "no override requested is not a failure")
		assert.NotContains(t, rec.warnedPeers(), "10.0.0.13",
			"an absent override is not a guard miss -- it must not warn")
	})

	t.Run("four-byte raw is applied", func(t *testing.T) {
		r := &Reactor{
			api: &pluginserver.Server{},
			policyFilterSeam: func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
				return PolicyResponse{Action: PolicyModify, Raw: []byte{0, 0, 0, 0}}
			},
			attrModHandlers: map[uint8]filterapi.AttrModHandler{},
		}

		res := r.runEgressPolicyChainASN4([]filterapi.FilterRef{{Name: "mp-reach-surgery"}},
			"10.0.0.14", 65001, 65000, testWireUpdate(body), false)

		require.True(t, res.accept, "a decodable raw override must be accepted, not refused")
		assert.False(t, res.failed, "an applied override is not a failure")
		require.NotNil(t, res.wireOverride, "the raw override must reach the caller")
	})
}

// VALIDATES: AC-12 (the no-over-fire half). A filter delta that CAN be applied
// still modifies the route on both chains, so the guard has not been turned into
// a blanket refusal of every text delta.
// PREVENTS: the fail-closed branch swallowing the working path. A guard that
// denies everything passes every negative test above and is useless.
func TestPolicyChainAppliedModificationStillModifies(t *testing.T) {
	handlers := attrModHandlersWithDefaults()
	body := []byte{0, 0, 0, 0}

	t.Run("ingress", func(t *testing.T) {
		addr := netip.MustParseAddr("10.0.0.9")
		peer := NewPeer(&PeerSettings{
			Address:       addr,
			LocalAS:       65000,
			PeerAS:        65001,
			ImportFilters: []filterapi.FilterRef{{Name: "set-local-pref"}},
		})
		r := &Reactor{
			api:              &pluginserver.Server{},
			policyFilterSeam: modifyingFilter(),
			attrModHandlers:  handlers,
		}

		res := r.runIngressPolicyChain(peer, addr, 65001, testWireUpdate(body), body)

		require.True(t, res.accept, "an applicable modification must not be refused")
		require.NotNil(t, res.modifiedPayload, "the modified payload must reach the caller")
		assert.NotEqual(t, body, res.modifiedPayload, "the payload must actually differ")
	})

	t.Run("egress", func(t *testing.T) {
		r := &Reactor{
			api:              &pluginserver.Server{},
			policyFilterSeam: modifyingFilter(),
			attrModHandlers:  handlers,
		}

		res := r.runEgressPolicyChainASN4([]filterapi.FilterRef{{Name: "set-local-pref"}},
			"10.0.0.10", 65001, 65000, testWireUpdate(body), false)

		require.True(t, res.accept, "an applicable modification must not be refused")
		assert.False(t, res.failed, "an applied modification is not a failure")
		require.NotNil(t, res.wireOverride, "the wire override must reach the caller")
	})
}
