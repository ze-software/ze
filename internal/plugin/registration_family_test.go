package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFamilyRegistrationWithDecode verifies parsing of "declare family <afi> <safi> decode".
//
// VALIDATES: Plugin can claim exclusive NLRI decoding for a family.
// PREVENTS: Family decode claims being silently ignored.
func TestFamilyRegistrationWithDecode(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		wantFamily         string
		wantDecodeFamilies []string
	}{
		{
			name:               "ipv4_flowspec_decode",
			input:              "declare family ipv4 flow decode",
			wantFamily:         "ipv4/flow",
			wantDecodeFamilies: []string{"ipv4/flow"},
		},
		{
			name:               "ipv6_flowspec_decode",
			input:              "declare family ipv6 flow decode",
			wantFamily:         "ipv6/flow",
			wantDecodeFamilies: []string{"ipv6/flow"},
		},
		{
			name:               "l2vpn_evpn_decode",
			input:              "declare family l2vpn evpn decode",
			wantFamily:         "l2vpn/evpn",
			wantDecodeFamilies: []string{"l2vpn/evpn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &PluginRegistration{}
			err := reg.ParseLine(tt.input)
			require.NoError(t, err)
			assert.Contains(t, reg.Families, tt.wantFamily)
			assert.Equal(t, tt.wantDecodeFamilies, reg.DecodeFamilies)
		})
	}
}

// TestFamilyRegistrationWithoutDecode verifies backward compatibility.
//
// VALIDATES: Existing "declare family X" without decode still works.
// PREVENTS: Breaking existing plugins that declare families without decode.
func TestFamilyRegistrationWithoutDecode(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		wantFamily         string
		wantDecodeFamilies []string
	}{
		{
			name:               "ipv4_flowspec_no_decode",
			input:              "declare family ipv4 flow",
			wantFamily:         "ipv4/flow",
			wantDecodeFamilies: nil, // Empty - no decode claim
		},
		{
			name:               "all_families_no_decode",
			input:              "declare family all",
			wantFamily:         "all",
			wantDecodeFamilies: nil, // Cannot claim decode for "all"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &PluginRegistration{}
			err := reg.ParseLine(tt.input)
			require.NoError(t, err)
			assert.Contains(t, reg.Families, tt.wantFamily)
			assert.Equal(t, tt.wantDecodeFamilies, reg.DecodeFamilies)
		})
	}
}

// TestFamilyAllCannotDecode verifies "declare family all decode" is rejected.
//
// VALIDATES: Cannot claim decode for "all" families.
// PREVENTS: Plugin claiming decode for all families (undefined behavior).
func TestFamilyAllCannotDecode(t *testing.T) {
	reg := &PluginRegistration{}
	err := reg.ParseLine("declare family all decode")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all")
}

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
	assert.Equal(t, "flowspec", registry.LookupFamily("IPV4/FLOWSPEC"))
	assert.Equal(t, "flowspec", registry.LookupFamily("IPv4/FlowSpec"))
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
		DecodeFamilies: []string{"IPv4/FlowSpec"},
	}
	err := registry.Register(plugin)
	require.NoError(t, err)

	// Lookup should work with any case
	assert.Equal(t, "flowspec", registry.LookupFamily("ipv4/flow"))
	assert.Equal(t, "flowspec", registry.LookupFamily("IPV4/FLOWSPEC"))
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
		DecodeFamilies: []string{"IPV4/FLOWSPEC"},
	}
	err = registry.Register(plugin2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "family conflict")
}

// TestFamilyDecodeFlowSpecVPN verifies flowspec-vpn SAFI is valid.
//
// VALIDATES: flowspec-vpn is a recognized SAFI.
// PREVENTS: Plugin unable to register flowspec-vpn decode.
func TestFamilyDecodeFlowSpecVPN(t *testing.T) {
	reg := &PluginRegistration{}
	err := reg.ParseLine("declare family ipv4 flow-vpn decode")
	require.NoError(t, err)
	assert.Contains(t, reg.Families, "ipv4/flow-vpn")
	assert.Contains(t, reg.DecodeFamilies, "ipv4/flow-vpn")
}

// TestFamilyRegistrationWithEncode verifies "declare family <afi> <safi> encode" works.
//
// VALIDATES: "encode" keyword registers family same as "decode".
// PREVENTS: "encode" declarations being silently ignored.
func TestFamilyRegistrationWithEncode(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		wantFamily         string
		wantDecodeFamilies []string
	}{
		{
			name:               "ipv4_flowspec_encode",
			input:              "declare family ipv4 flow encode",
			wantFamily:         "ipv4/flow",
			wantDecodeFamilies: []string{"ipv4/flow"},
		},
		{
			name:               "ipv6_flowspec_encode",
			input:              "declare family ipv6 flow encode",
			wantFamily:         "ipv6/flow",
			wantDecodeFamilies: []string{"ipv6/flow"},
		},
		{
			name:               "l2vpn_evpn_encode",
			input:              "declare family l2vpn evpn encode",
			wantFamily:         "l2vpn/evpn",
			wantDecodeFamilies: []string{"l2vpn/evpn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &PluginRegistration{}
			err := reg.ParseLine(tt.input)
			require.NoError(t, err)
			assert.Contains(t, reg.Families, tt.wantFamily)
			assert.Equal(t, tt.wantDecodeFamilies, reg.DecodeFamilies)
		})
	}
}

// TestFamilyRegistrationBothEncodeAndDecode verifies plugin can declare both.
//
// VALIDATES: Plugin declaring both encode and decode registers once.
// PREVENTS: Duplicate family entries in DecodeFamilies.
func TestFamilyRegistrationBothEncodeAndDecode(t *testing.T) {
	reg := &PluginRegistration{}

	// Declare encode first
	err := reg.ParseLine("declare family ipv4 flow encode")
	require.NoError(t, err)

	// Then declare decode for same family
	err = reg.ParseLine("declare family ipv4 flow decode")
	require.NoError(t, err)

	// Should have family listed twice in Families (both declarations)
	// but only once in DecodeFamilies (deduplication)
	assert.Equal(t, []string{"ipv4/flow", "ipv4/flow"}, reg.Families)
	assert.Equal(t, []string{"ipv4/flow"}, reg.DecodeFamilies) // No duplicate
}

// TestFamilyAllCannotEncode verifies "declare family all encode" is rejected.
//
// VALIDATES: Cannot claim encode for "all" families.
// PREVENTS: Plugin claiming encode for all families (undefined behavior).
func TestFamilyAllCannotEncode(t *testing.T) {
	reg := &PluginRegistration{}
	err := reg.ParseLine("declare family all encode")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all")
}
