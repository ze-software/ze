package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"

	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/filter_remove_private_as"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// VALIDATES: remove-private-as filter refs resolve through FilterTypes registration.
// PREVENTS: configured chains reaching runtime as unresolved short names.
func TestCanonicalizeRemovePrivateASRef(t *testing.T) {
	reg := &FilterRegistry{entries: map[string]FilterEntry{
		"STRIP": {Name: "STRIP", Type: "remove-private-as"},
	}}
	typesMap := registry.FilterTypesMap()

	assert.Equal(t, "bgp-filter-remove-private-as:STRIP", canonicalizeOne("STRIP", reg, typesMap))
	assert.Equal(t, "bgp-filter-remove-private-as:STRIP", canonicalizeOne("remove-private-as:STRIP", reg, typesMap))
	assert.Equal(t, "bgp-filter-remove-private-as:STRIP", canonicalizeOne("bgp-filter-remove-private-as:STRIP", reg, typesMap))
	assert.Equal(t, "inactive:bgp-filter-remove-private-as:STRIP", canonicalizeOne("inactive:remove-private-as:STRIP", reg, typesMap))
}
