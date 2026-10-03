// Design: docs/architecture/bgp/structural-forwarding.md -- what a forwarded route carries
// Related: filter_ordered.go -- runIngressPolicyChain, where an import policy computes the preference
// Related: forward_local_pref.go -- applyFactsLocalPref, the LOCAL_PREF a forwarded route leaves with
// Related: rfc8950_reactor_a2_forward_test.go -- a2Forward and a2Parts, the harness

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// rfc4271ImportPolicyPayload runs a2InlinePayload, received from the external
// peer 203.0.113.9 (AS 65001), through that peer's import policy chain, the
// ingress step whose modified payload replaces the cached UPDATE before the RIB
// and the forward rails see it (reactor_notify.go). With policy, the chain's
// one filter answers "local-preference 200" (modifyingFilter); without, the
// peer has no import filter and the received payload is returned as it came.
func rfc4271ImportPolicyPayload(t *testing.T, policy bool) []byte {
	t.Helper()
	addr := netip.MustParseAddr("203.0.113.9")
	settings := &PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001}
	if policy {
		settings.ImportFilters = []filterapi.FilterRef{{Name: "set-local-pref"}}
	}
	r := &Reactor{
		api:              &pluginserver.Server{}, // non-nil: past the fail-closed r.api guard
		policyFilterSeam: modifyingFilter(),
		attrModHandlers:  attrModHandlersWithDefaults(),
	}

	body := a2InlinePayload()
	res := r.runIngressPolicyChain(NewPeer(settings), addr, 65001, testWireUpdate(body), body)
	require.True(t, res.accept, "the import policy accepts the route")
	if !policy {
		require.Nil(t, res.modifiedPayload, "no import policy leaves the received payload in place")
		return body
	}
	require.NotNil(t, res.modifiedPayload, "the import policy rewrote the route")
	return res.modifiedPayload
}

// TestRFC4271ImportPolicyLocalPrefIsReadvertisedToInternalPeers drives RFC 4271
// Section 9.1.1: "the return value MUST be used as the LOCAL_PREF value in any
// IBGP readvertisement."
//
// Method: a route learned from an external peer is passed through that peer's
// import policy chain, whose filter computes the degree of preference 200. The
// payload the chain returns is forwarded on the general rail to an internal and
// an external destination in one fan-out. The same route with no import policy
// is forwarded the same way.
//
// VALIDATES: the internal destination is sent LOCAL_PREF 200, the value the
// policy returned; the external destination is sent no LOCAL_PREF (Section
// 5.1.5). Without the policy the internal destination is sent 100, the default
// degree of preference, so the 200 came from the policy and not from a constant.
// PREVENTS: the forward rail sending a default or a received LOCAL_PREF toward
// an internal peer in place of the preference the import policy computed.
//
// RFC requirement: RFC4271-9.1.1-2 positive -- an external route whose import policy computes the degree of preference 200 is readvertised on the general forward rail to an internal peer with LOCAL_PREF 200, while an external peer in the same fan-out is sent no LOCAL_PREF.
// RFC requirement: RFC4271-9.1.1-2 negative -- the internal peer's LOCAL_PREF is the policy's return value and not a constant: the same route with no import policy is readvertised to the internal peer with LOCAL_PREF 100, never 200.
func TestRFC4271ImportPolicyLocalPrefIsReadvertisedToInternalPeers(t *testing.T) {
	internalAddr := netip.MustParseAddr("192.0.2.71")
	externalAddr := netip.MustParseAddr("192.0.2.72")
	for _, tc := range []struct {
		policy bool
		want   []byte
	}{
		{policy: true, want: []byte{0, 0, 0, 200}},
		{policy: false, want: []byte{0, 0, 0, 100}},
	} {
		payload := rfc4271ImportPolicyPayload(t, tc.policy)
		internal := a2Dest(t, internalAddr.String(), 65000, netip.Addr{}, false)
		external := a2Dest(t, externalAddr.String(), 65002, netip.Addr{}, false)

		got := a2Forward(t, false, payload, internal, external)

		toInternal, ok := got[internalAddr]
		require.True(t, ok, "policy=%v: the internal destination is owed the route", tc.policy)
		require.Equal(t, a2InlinePrefix, toInternal.nlri)
		require.Equal(t, tc.want, toInternal.localPref, "policy=%v: the LOCAL_PREF sent to the internal peer", tc.policy)

		toExternal, ok := got[externalAddr]
		require.True(t, ok, "policy=%v: the external destination is owed the route", tc.policy)
		require.Nil(t, toExternal.localPref, "policy=%v: no LOCAL_PREF to an external peer", tc.policy)
	}
}
