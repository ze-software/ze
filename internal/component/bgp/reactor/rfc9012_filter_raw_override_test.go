// Design: docs/architecture/wire/attributes.md -- carrier-aware tunnel validation.
// Related: filter_chain.go -- decodeFilterRawOverride, the raw answer under test.
// Related: session_tunnel_encap.go -- applyTunnelEncap, the guard the raw answer reuses.
// RFC 9012 Section 13 -- see rfc/short/rfc9012.md.

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// rawTunnelFilter answers every route with a raw replacement body.
func rawTunnelFilter(raw []byte) PolicyFilterFunc {
	return func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		return PolicyResponse{Action: PolicyModify, Raw: raw}
	}
}

// rawTunnelReactor is a reactor whose only filter answers with raw.
func rawTunnelReactor(raw []byte) *Reactor {
	return &Reactor{
		api:              &pluginserver.Server{},
		policyFilterSeam: rawTunnelFilter(raw),
		attrModHandlers:  map[uint8]filterapi.AttrModHandler{},
	}
}

// TestRFC9012FilterRawOverrideRemovesInvalidTLVs drives a raw filter answer
// through both chain entry points. Its goal: the body a filter hands back is a
// route ze distributes, so the Section 13 removal the receive path performs
// (applyTunnelEncap) runs on it too. Method: the raw answer carries a TLV whose
// Tunnel Egress Endpoint is malformed (short IPv4 address) beside a valid
// sibling, and the test reads the body the step hands on. Without a survivor,
// the route has no valid TLV and both steps refuse it.
// RFC 9012 Section 13: "There is one exception to this rule: if a TLV contains
// a malformed Tunnel Egress Endpoint sub-TLV (as defined in Section 3.1), the
// entire TLV MUST be ignored and MUST be removed from the Tunnel Encapsulation
// attribute before the route carrying that attribute is distributed."
// RFC requirement: RFC9012-13-14 negative -- a raw filter answer's malformed-endpoint TLVs are removed on the export and import chains while the valid sibling remains, and an export raw answer with no valid TLV is suppressed as failed.
func TestRFC9012FilterRawOverrideRemovesInvalidTLVs(t *testing.T) {
	endpoint := teSub(6, 0xFF, 0xFF, 0xFF, 0xFF, 0, 1, 10, 0, 0, 77)
	valid := teTLV(2, endpoint, teSub(9, 1))
	bad := teTLV(2, teSub(6, 0, 0, 0, 0, 0, 1, 10, 0, 0))

	mixed := append(append(append([]byte{}, bad...), valid...), bad...)
	answer := teCarryBody(mixed)
	original := append([]byte{}, answer...)
	cleaned := teCarryBody(valid)
	input := []byte{0, 0, 0, 0}
	policy := mustPolicy(t, []routepolicy.Ref{{Name: "tunnel-surgery"}}, routepolicy.FinalAccept)

	t.Run("export removes the malformed TLVs", func(t *testing.T) {
		r := rawTunnelReactor(answer)
		// RFC 9012 Section 13.
		res := r.runEgressPolicyChainASN4(policy, "10.0.0.21", 65001, 65000, false,
			policyview.Session{EBGP: true}, testWireUpdate(input), false)
		require.True(t, res.accept, "a raw answer with a valid TLV left is still sent")
		require.NotNil(t, res.wireOverride, "the raw answer reaches the caller")
		require.Equal(t, cleaned, res.wireOverride.Payload(),
			"the malformed-endpoint TLVs are removed before the route is distributed")
		require.Equal(t, original, answer, "the filter's answer bytes stay unchanged")
	})

	t.Run("import removes the malformed TLVs", func(t *testing.T) {
		addr := netip.MustParseAddr("10.0.0.22")
		peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ImportPolicy: policy})
		r := rawTunnelReactor(answer)
		res := r.runIngressPolicyChain(peer, addr, nil, testWireUpdate(input), input)
		require.True(t, res.accept, "a raw answer with a valid TLV left is still installed")
		require.Equal(t, cleaned, res.modifiedPayload,
			"the malformed-endpoint TLVs are removed before the route is cached")
	})

	t.Run("no valid TLV refuses the route", func(t *testing.T) {
		rec := captureWarnPeers(t)
		r := rawTunnelReactor(teCarryBody(bad))
		res := r.runEgressPolicyChainASN4(policy, "10.0.0.23", 65001, 65000, false,
			policyview.Session{EBGP: true}, testWireUpdate(input), false)
		require.False(t, res.accept, "a raw answer whose every TLV is malformed is not sent")
		require.True(t, res.failed, "a malformed raw answer is a guard miss, not a policy decision")
		assert.Nil(t, res.wireOverride)
		require.Contains(t, rec.warnedPeers(), "10.0.0.23", "the refusal is logged at Warn")
	})

	t.Run("a valid answer is sent unchanged", func(t *testing.T) {
		r := rawTunnelReactor(cleaned)
		res := r.runEgressPolicyChainASN4(policy, "10.0.0.24", 65001, 65000, false,
			policyview.Session{EBGP: true}, testWireUpdate(input), false)
		require.True(t, res.accept)
		require.NotNil(t, res.wireOverride)
		require.Equal(t, cleaned, res.wireOverride.Payload())
	})
}
