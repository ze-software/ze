// Design: docs/guide/rpki.md -- ASPA path verification.
// Related: aspa_verify.go -- verifyASPA
package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestASPAZeroBesideProvidersIsNoWildcard compares each SPAS holding AS 0 beside other providers
// with the same SPAS without AS 0.
//
// VALIDATES: draft-ietf-sidrops-aspa-verification Section 4, "an unexpected presence of AS 0 has
// no influence on the AS_PATH verification procedures": a mixed SPAS verifies a path exactly as
// the SPAS without AS 0 does, whether that answer is Valid or Invalid.
// PREVENTS: AS 0 beside a real provider invalidating the hop, or acting as a wildcard provider.
func TestASPAZeroBesideProvidersIsNoWildcard(t *testing.T) {
	path := []uint32{100, 200, 300}
	verify := func(providers []uint32) uint8 {
		c := newASPACache()
		c.Set(200, []uint32{100})
		c.Set(300, providers)
		return verifyASPA(c, path)
	}

	t.Run("AS 0 beside the path's provider", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1 positive -- the SPAS {0, 200} verifies the path Valid, the same result the SPAS {200} gives.
		assert.Equal(t, ASPAValid, verify([]uint32{200}))
		assert.Equal(t, ASPAValid, verify([]uint32{0, 200}))
	})

	t.Run("AS 0 beside an unrelated provider", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1 negative -- the SPAS {0, 999}, which does not list the path's provider 200, verifies the path Invalid, the same result the SPAS {999} gives: AS 0 mixed with other providers authorizes nothing.
		assert.Equal(t, ASPAInvalid, verify([]uint32{999}))
		assert.Equal(t, ASPAInvalid, verify([]uint32{0, 999}))
	})
}
