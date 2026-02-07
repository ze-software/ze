package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFamilyConflictDetection verifies two plugins cannot claim same family decode.
//
// VALIDATES: Family decode conflicts are detected at registration.
// PREVENTS: Two plugins claiming the same family's NLRI decoding.
func TestFamilyConflictDetection(t *testing.T) {
	registry := NewPluginRegistry()

	// First plugin registers flowspec family decode
	plugin1 := &PluginRegistration{
		Name:           "flowspec",
		DecodeFamilies: []string{"ipv4/flow"},
	}
	err := registry.Register(plugin1)
	require.NoError(t, err)

	// Second plugin tries same family - should fail
	plugin2 := &PluginRegistration{
		Name:           "flowspec2",
		DecodeFamilies: []string{"ipv4/flow"},
	}
	err = registry.Register(plugin2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "family conflict")
	assert.Contains(t, err.Error(), "ipv4/flow")
	assert.Contains(t, err.Error(), "flowspec") // Original plugin name
}

// TestFamilyLookup verifies LookupFamily returns correct plugin.
//
// VALIDATES: Family → plugin lookup works after registration.
// PREVENTS: Decode requests routing to wrong plugin.
func TestFamilyLookup(t *testing.T) {
	registry := NewPluginRegistry()

	plugin := &PluginRegistration{
		Name:           "flowspec",
		DecodeFamilies: []string{"ipv4/flow", "ipv6/flow"},
	}
	err := registry.Register(plugin)
	require.NoError(t, err)

	// Lookup registered families
	assert.Equal(t, "flowspec", registry.LookupFamily("ipv4/flow"))
	assert.Equal(t, "flowspec", registry.LookupFamily("ipv6/flow"))
}

// TestFamilyLookupUnknown verifies LookupFamily returns empty for unknown family.
//
// VALIDATES: Unknown family lookup returns empty string.
// PREVENTS: Panic or incorrect result on unknown family.
func TestFamilyLookupUnknown(t *testing.T) {
	registry := NewPluginRegistry()

	// No plugins registered
	assert.Equal(t, "", registry.LookupFamily("ipv4/flow"))
	assert.Equal(t, "", registry.LookupFamily(""))
	assert.Equal(t, "", registry.LookupFamily("ipv4/unknown"))
}

// TestFamilyLookupEmptyString verifies empty family string returns empty.
//
// VALIDATES: Edge case of empty string family lookup.
// PREVENTS: Map lookup issues with empty key.
// BOUNDARY: Empty string is invalid family.
func TestFamilyLookupEmptyString(t *testing.T) {
	registry := NewPluginRegistry()

	plugin := &PluginRegistration{
		Name:           "flowspec",
		DecodeFamilies: []string{"ipv4/flow"},
	}
	err := registry.Register(plugin)
	require.NoError(t, err)

	assert.Equal(t, "", registry.LookupFamily(""))
}

// TestMultipleFamilyRegistration verifies plugin can register multiple families.
//
// VALIDATES: Single plugin can decode multiple families.
// PREVENTS: Only first family being registered.
func TestMultipleFamilyRegistration(t *testing.T) {
	registry := NewPluginRegistry()

	plugin := &PluginRegistration{
		Name:           "flowspec",
		DecodeFamilies: []string{"ipv4/flow", "ipv6/flow", "ipv4/flow-vpn"},
	}
	err := registry.Register(plugin)
	require.NoError(t, err)

	assert.Equal(t, "flowspec", registry.LookupFamily("ipv4/flow"))
	assert.Equal(t, "flowspec", registry.LookupFamily("ipv6/flow"))
	assert.Equal(t, "flowspec", registry.LookupFamily("ipv4/flow-vpn"))
}

// TestFamilyLookupCaseInsensitive verifies family lookup is case-insensitive.
//
// VALIDATES: Lookup normalizes family to lowercase.
// PREVENTS: Case mismatch causing lookup failures.
func TestFamilyLookupCaseInsensitive(t *testing.T) {
	registry := NewPluginRegistry()

	plugin := &PluginRegistration{
		Name:           "flowspec",
		DecodeFamilies: []string{"ipv4/flow"},
	}
	err := registry.Register(plugin)
	require.NoError(t, err)

	// All case variations should work
	assert.Equal(t, "flowspec", registry.LookupFamily("ipv4/flow"))
	assert.Equal(t, "flowspec", registry.LookupFamily("IPV4/FLOW"))
	assert.Equal(t, "flowspec", registry.LookupFamily("IPv4/Flow"))
}

// TestFamilyRegisterCaseInsensitive verifies registration normalizes family case.
//
// VALIDATES: Registration normalizes family to lowercase.
// PREVENTS: Mixed-case DecodeFamilies causing lookup failures.
func TestFamilyRegisterCaseInsensitive(t *testing.T) {
	registry := NewPluginRegistry()

	// Register with MIXED CASE
	plugin := &PluginRegistration{
		Name:           "flowspec",
		DecodeFamilies: []string{"IPv4/Flow"},
	}
	err := registry.Register(plugin)
	require.NoError(t, err)

	// Lookup should work with any case
	assert.Equal(t, "flowspec", registry.LookupFamily("ipv4/flow"))
	assert.Equal(t, "flowspec", registry.LookupFamily("IPV4/FLOW"))
}

// TestFamilyConflictCaseInsensitive verifies conflict detection is case-insensitive.
//
// VALIDATES: Conflict detection normalizes family to lowercase.
// PREVENTS: Same family registered twice with different cases.
func TestFamilyConflictCaseInsensitive(t *testing.T) {
	registry := NewPluginRegistry()

	// First plugin registers lowercase
	plugin1 := &PluginRegistration{
		Name:           "plugin1",
		DecodeFamilies: []string{"ipv4/flow"},
	}
	err := registry.Register(plugin1)
	require.NoError(t, err)

	// Second plugin tries UPPERCASE - should still conflict
	plugin2 := &PluginRegistration{
		Name:           "plugin2",
		DecodeFamilies: []string{"IPV4/FLOW"},
	}
	err = registry.Register(plugin2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "family conflict")
}
