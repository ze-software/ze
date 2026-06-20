package yang

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMacBindingUniqueRetained guards the MAC physical-binding invariant: the
// `mac/address` leaf is unique-per-list (so a MAC binds at most one logical
// interface) but NOT required (so existing `interface eth0 {}` configs without a
// MAC still parse). Learned summary 523 documents a recurring gotcha where a
// linter hook silently reverted these YANG statements between sessions; this
// test fails loudly if the `unique` constraint is dropped again.
//
// VALIDATES: spec-iface-resolve-1-model AC-4 -- mac stays optional + unique.
func TestMacBindingUniqueRetained(t *testing.T) {
	n := strings.Count(ZeIfaceConfYANG, `unique "mac/address"`)
	assert.GreaterOrEqual(t, n, 3,
		"mac/address unique constraint must be retained on ethernet/veth/bridge (523 gotcha: a linter hook has reverted it before)")

	// mac stays OPTIONAL: the leaf must not carry ze:required. (Making it
	// required would break every existing config that omits a MAC.)
	assert.NotContains(t, ZeIfaceConfYANG, `ze:required "mac/address"`,
		"mac/address must remain optional (no ze:required)")
}
