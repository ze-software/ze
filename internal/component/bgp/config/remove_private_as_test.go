package bgpconfig

import (
	"testing"

	"github.com/ze-software/ze/internal/component/policy"

	"github.com/stretchr/testify/assert"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_remove_private_as"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// VALIDATES: remove-private-as filter refs resolve through FilterTypes registration.
// PREVENTS: configured chains reaching runtime as unresolved short names.
func TestCanonicalizeRemovePrivateASRef(t *testing.T) {
	reg := mustPolicyRegistry(t, policy.Instance{Name: "STRIP", Type: "remove-private-as"})
	typesMap := registry.FilterTypesMap()

	assert.Equal(t, "bgp-filter-remove-private-as:STRIP", policy.CanonicalName("STRIP", reg, typesMap))
	assert.Equal(t, "bgp-filter-remove-private-as:STRIP", policy.CanonicalName("remove-private-as:STRIP", reg, typesMap))
	assert.Equal(t, "bgp-filter-remove-private-as:STRIP", policy.CanonicalName("bgp-filter-remove-private-as:STRIP", reg, typesMap))
	// A deactivated ref keeps its structural flag while the name canonicalizes.
	got, bound := canonicalizeFilterRefs([]policy.Ref{{Name: "remove-private-as:STRIP", Inactive: true}}, reg)
	assert.Equal(t, []policy.Ref{{Name: "bgp-filter-remove-private-as:STRIP", Inactive: true}}, got)
	assert.Nil(t, bound)
}
