package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// valleyCache holds a path 100 -> 200 -> 300 (neighbor to origin) whose
// origin climbs to 200 and whose neighbor 100 is a customer of 200. The path
// has a down-ramp, so it is Valid only for the downstream procedure.
func valleyCache() *aSPACache {
	c := newASPACache()
	c.Set(300, []uint32{200})
	c.Set(200, []uint32{999})
	c.Set(100, []uint32{200})
	return c
}

// TestASPAUpstreamAppliesToCustomerPeerAndRSRoutes selects the verification
// procedure from the configured BGP role and runs it on a path with a
// down-ramp.
//
// VALIDATES: draft-ietf-sidrops-aspa-verification Section 5.5 -- the upstream
// procedure applies to a route received from a Customer or Peer, by an RS from
// an RS-client, and by an RS-client from an RS.
// PREVENTS: a role mapping that verifies those routes with the downstream
// procedure and accepts a path with a down-ramp.
func TestASPAUpstreamAppliesToCustomerPeerAndRSRoutes(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1 positive -- local roles provider, peer, rs and rs-client select the upstream procedure, which finds a path with a down-ramp Invalid.
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1 negative -- local role customer, a route received from a provider, does not select it: the same path verifies Valid downstream.
	segments := []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: []uint32{100, 200, 300}}}
	role := func(local string) map[string]any {
		return map[string]any{"role": map[string]any{"import": local}}
	}

	for _, local := range []string{"provider", "peer", "rs", "rs-client"} {
		mode := configuredASPAMode(role(local), nil)
		assert.Equal(t, aspaUpstream, mode, "local role %s", local)
		state, _ := aspaStateForPath(valleyCache(), segments, mode)
		assert.Equal(t, ASPAInvalid, state, "local role %s accepted a down-ramp", local)
	}

	mode := configuredASPAMode(role("customer"), nil)
	assert.Equal(t, aspaDownstream, mode)
	state, _ := aspaStateForPath(valleyCache(), segments, mode)
	assert.Equal(t, ASPAValid, state)
}
