package redistribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerTestSources registers a few sources for testing.
func registerTestSources(t *testing.T) {
	t.Helper()
	require.NoError(t, RegisterSource(RouteSource{Name: "ibgp", Protocol: "bgp", Description: "iBGP learned routes"}))
	require.NoError(t, RegisterSource(RouteSource{Name: "ebgp", Protocol: "bgp", Description: "eBGP learned routes"}))
	require.NoError(t, RegisterSource(RouteSource{Name: "connected", Protocol: "connected", Description: "directly connected routes"}))
}

// TestRegisterAndLookup verifies source registration and lookup.
//
// VALIDATES: RegisterSource stores source, LookupSource retrieves it.
// PREVENTS: Registered sources lost or returned with wrong fields.
func TestRegisterAndLookup(t *testing.T) {
	registerTestSources(t)

	src, ok := LookupSource("ibgp")
	require.True(t, ok)
	assert.Equal(t, "ibgp", src.Name)
	assert.Equal(t, "bgp", src.Protocol)

	src, ok = LookupSource("ebgp")
	require.True(t, ok)
	assert.Equal(t, "ebgp", src.Name)
	assert.Equal(t, "bgp", src.Protocol)

	src, ok = LookupSource("connected")
	require.True(t, ok)
	assert.Equal(t, "connected", src.Name)
	assert.Equal(t, "connected", src.Protocol)
}

// TestLookupMissing verifies unknown source returns false.
//
// VALIDATES: LookupSource returns false for unregistered name.
// PREVENTS: Panic or incorrect match on unknown source.
func TestLookupMissing(t *testing.T) {
	registerTestSources(t)
	_, ok := LookupSource("rip")
	assert.False(t, ok)
}

// TestSourceNames verifies sorted name list.
//
// VALIDATES: SourceNames returns all registered names sorted.
// PREVENTS: Missing or unsorted names in autocomplete.
func TestSourceNames(t *testing.T) {
	registerTestSources(t)
	names := SourceNames()

	assert.Contains(t, names, "ibgp")
	assert.Contains(t, names, "ebgp")
	assert.Contains(t, names, "connected")

	// Verify sorted
	for i := 1; i < len(names); i++ {
		assert.True(t, names[i-1] <= names[i], "names not sorted: %s > %s", names[i-1], names[i])
	}
}

// TestIdempotentRegistration verifies registering the same source twice is a no-op.
//
// VALIDATES: RegisterSource is idempotent for identical entries.
// PREVENTS: Duplicate entries or errors on re-registration.
func TestIdempotentRegistration(t *testing.T) {
	require.NoError(t, RegisterSource(RouteSource{Name: "static", Protocol: "static", Description: "static routes"}))
	require.NoError(t, RegisterSource(RouteSource{Name: "static", Protocol: "static", Description: "static routes"}))

	src, ok := LookupSource("static")
	require.True(t, ok)
	assert.Equal(t, "static", src.Name)
}

// TestConflictingProtocol verifies that re-registering a source with a different protocol returns an error.
//
// VALIDATES: RegisterSource rejects protocol conflicts.
// PREVENTS: Silent overwrite when two components claim the same source name.
func TestConflictingProtocol(t *testing.T) {
	require.NoError(t, RegisterSource(RouteSource{Name: "conflict-test", Protocol: "bgp", Description: "test"}))
	err := RegisterSource(RouteSource{Name: "conflict-test", Protocol: "ospf", Description: "test"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSourceConflict)
}
