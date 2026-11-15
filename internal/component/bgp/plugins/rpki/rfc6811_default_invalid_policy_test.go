// Design: docs/guide/rpki.md -- route disposition must reflect the operator's policy.
// Related: rpki_config.go -- implicit actions; rpki.go -- buildDecisions
// RFC naming: untagged -- this default-policy defect probe has a failing baseline and earns no conformance credit.
package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRPKIInvalidExclusionRequiresExplicitConfiguration carries parsed operator
// configuration through the actual origin-validation decision builder.
//
// VALIDATES: RFC 6811 Section 2: "An implementation MUST NOT exclude a route
// from the Adj-RIB-In or from consideration in the decision process as a side
// effect of its validation state, unless explicitly configured to do so."
// PREVENTS: the parser silently inventing invalid=reject when only a cache is
// configured; an explicitly configured rejection must still exclude Invalid.
func TestRPKIInvalidExclusionRequiresExplicitConfiguration(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "cache alone", true: "explicit reject"}[explicit], func(t *testing.T) {
			input := `{"rpki":{"cache-server":{"192.0.2.1":{"trusted-network":"true"}}}}`
			if explicit {
				input = `{"rpki":{"cache-server":{"192.0.2.1":{"trusted-network":"true"}},"action":{"invalid":"reject"}}}`
			}
			config, err := parseRPKIConfig(input)
			require.NoError(t, err)
			require.Len(t, config.CacheServers, 1)
			rp := &rPKIPlugin{cache: newROACache(), aspaCache: newASPACache()}
			rp.originInvalidAction.Store(uint32(config.OriginInvalidAction))
			rp.originNotFoundAction.Store(uint32(config.OriginNotFoundAction))
			rp.cache.Add(makeVRP("192.0.2.0/24", 24, 64500))
			decisions := rp.buildDecisions([]validationRequest{
				{prefix: "192.0.2.0/24", state: rp.cache.Validate("192.0.2.0/24", 64501), aspaState: aspaStateNone},
				{prefix: "192.0.2.0/24", state: rp.cache.Validate("192.0.2.0/24", 64500), aspaState: aspaStateNone},
			})
			require.Len(t, decisions, 2)
			assert.Equal(t, !explicit, decisions[0].Accept,
				"only an operator-stated invalid=reject excludes the Invalid route")
			assert.Equal(t, explicit, decisions[0].Ineligible)
			assert.Equal(t, ValidationInvalid, decisions[0].ValState)
			assert.True(t, decisions[1].Accept, "Valid route is independent of invalid disposition")
			assert.Equal(t, ValidationValid, decisions[1].ValState)
		})
	}
}
