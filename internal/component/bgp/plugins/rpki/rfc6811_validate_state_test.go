// Design: docs/guide/rpki.md -- origin validation of received routes.
// Related: rpki.go -- validateNLRIs
package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouteStateIsSetFromTheLookup validates one received route through validateNLRIs, the path an
// UPDATE takes, and reads the state the route is given.
//
// VALIDATES: RFC 6811 Section 2, "The validation state of the Route MUST be set to reflect the
// result of the lookup": the state returned for the route and the state its validation request
// carries to the decision worker both equal the lookup result, for each of the three results.
// PREVENTS: a route left in a default state, or one keeping a state an earlier lookup produced.
func TestRouteStateIsSetFromTheLookup(t *testing.T) {
	nlri := []byte{24, 10, 0, 0} // 10.0.0.0/24
	validate := func(t *testing.T, rp *rPKIPlugin) (uint8, uint8) {
		t.Helper()
		results := rp.validateNLRIs("192.0.2.50", "", "", 65001, 71, "ipv4/unicast",
			nlri, false, false, 65001, false, aspaStateNone, false)
		requests := drainRequests(rp.validateCh)
		require.Len(t, requests, 1)
		return results["10.0.0.0/24"], requests[0].state
	}

	t.Run("each lookup result becomes the route's state", func(t *testing.T) {
		// RFC requirement: RFC6811-2-1 positive -- for a Valid, an Invalid and a NotFound lookup, the state validateNLRIs returns for the route and the state its validation request carries both equal that lookup result.
		for _, test := range []struct {
			name string
			vrps []VRP
			want uint8
		}{
			{name: "valid", vrps: []VRP{makeVRP("10.0.0.0/24", 24, 65001)}, want: ValidationValid},
			{name: "invalid", vrps: []VRP{makeVRP("10.0.0.0/24", 24, 65002)}, want: ValidationInvalid},
			{name: "not found", want: ValidationNotFound},
		} {
			t.Run(test.name, func(t *testing.T) {
				rp, _ := groupMemberPlugin(t)
				rp.cache.Replace(test.vrps)
				returned, carried := validate(t, rp)
				assert.Equal(t, test.want, rp.cache.Validate("10.0.0.0/24", 65001), "the lookup itself")
				assert.Equal(t, test.want, returned)
				assert.Equal(t, test.want, carried)
			})
		}
	})

	t.Run("a changed lookup is not masked by the earlier state", func(t *testing.T) {
		// RFC requirement: RFC6811-2-1 negative -- after the VRP set changes so the same route's lookup turns from Valid to Invalid, the state set on the route is Invalid, not the Valid it held before.
		rp, _ := groupMemberPlugin(t)
		rp.cache.Replace([]VRP{makeVRP("10.0.0.0/24", 24, 65001)})
		returned, carried := validate(t, rp)
		require.Equal(t, ValidationValid, returned)
		require.Equal(t, ValidationValid, carried)

		rp.cache.Replace([]VRP{makeVRP("10.0.0.0/24", 24, 65002)})
		returned, carried = validate(t, rp)
		assert.Equal(t, ValidationInvalid, returned)
		assert.Equal(t, ValidationInvalid, carried)
	})
}
