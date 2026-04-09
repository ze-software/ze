package redistribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterAndLookup verifies source registration and lookup.
//
// VALIDATES: RegisterSource stores source, LookupSource retrieves it.
// PREVENTS: Registered sources lost or returned with wrong fields.
func TestRegisterAndLookup(t *testing.T) {
	RegisterBGPSources()
	src, ok := LookupSource("ibgp")
	require.True(t, ok)
	assert.Equal(t, "ibgp", src.Name)
	assert.Equal(t, "bgp", src.Protocol)

	src, ok = LookupSource("ebgp")
	require.True(t, ok)
	assert.Equal(t, "ebgp", src.Name)
	assert.Equal(t, "bgp", src.Protocol)
}

// TestLookupMissing verifies unknown source returns false.
//
// VALIDATES: LookupSource returns false for unregistered name.
// PREVENTS: Panic or incorrect match on unknown source.
func TestLookupMissing(t *testing.T) {
	RegisterBGPSources()
	_, ok := LookupSource("ospf")
	assert.False(t, ok)
}

// TestSourceNames verifies sorted name list.
//
// VALIDATES: SourceNames returns all registered names sorted.
// PREVENTS: Missing or unsorted names in autocomplete.
func TestSourceNames(t *testing.T) {
	RegisterBGPSources()
	names := SourceNames()
	assert.Contains(t, names, "ebgp")
	assert.Contains(t, names, "ibgp")
	// Verify sorted
	for i := 1; i < len(names); i++ {
		assert.True(t, names[i-1] <= names[i], "names not sorted: %s > %s", names[i-1], names[i])
	}
}
