package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
)

// VALIDATES: MUP family validation and argument conversion (registry-independent).
// PREVENTS: IPv4/IPv6 family mismatches reaching the encoder, incorrect arg serialization.

// TestValidateMUPFamilyMatch_Valid verifies matching prefix/family passes.
func TestValidateMUPFamilyMatch_Valid(t *testing.T) {
	tests := []struct {
		name string
		spec bgptypes.MUPRouteSpec
	}{
		{"ipv4_prefix", bgptypes.MUPRouteSpec{Prefix: "10.0.0.0/24", IsIPv6: false}},
		{"ipv6_prefix", bgptypes.MUPRouteSpec{Prefix: "2001:db8::/32", IsIPv6: true}},
		{"ipv4_address", bgptypes.MUPRouteSpec{Address: "10.0.0.1", IsIPv6: false}},
		{"ipv6_address", bgptypes.MUPRouteSpec{Address: "::1", IsIPv6: true}},
		{"no_prefix_no_address", bgptypes.MUPRouteSpec{IsIPv6: false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMUPFamilyMatch(tt.spec)
			assert.NoError(t, err)
		})
	}
}

// TestValidateMUPFamilyMatch_Mismatch verifies prefix/family mismatch is rejected.
func TestValidateMUPFamilyMatch_Mismatch(t *testing.T) {
	tests := []struct {
		name string
		spec bgptypes.MUPRouteSpec
	}{
		{"ipv4_prefix_ipv6_flag", bgptypes.MUPRouteSpec{Prefix: "10.0.0.0/24", IsIPv6: true}},
		{"ipv6_prefix_ipv4_flag", bgptypes.MUPRouteSpec{Prefix: "2001:db8::/32", IsIPv6: false}},
		{"ipv4_addr_ipv6_flag", bgptypes.MUPRouteSpec{Address: "10.0.0.1", IsIPv6: true}},
		{"ipv6_addr_ipv4_flag", bgptypes.MUPRouteSpec{Address: "::1", IsIPv6: false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMUPFamilyMatch(tt.spec)
			require.Error(t, err)
		})
	}
}

// TestMUPSpecToArgs_AllFields verifies argument conversion with all fields populated.
func TestMUPSpecToArgs_AllFields(t *testing.T) {
	spec := bgptypes.MUPRouteSpec{
		RouteType: "interwork-segment-discovery",
		RD:        "100:100",
		Prefix:    "10.0.0.0/24",
		Address:   "10.0.0.1",
		TEID:      "0x12345678",
		QFI:       9,
		Endpoint:  "10.0.0.2",
		Source:    "10.0.0.3",
	}

	args := mupSpecToArgs(spec)

	// First two should always be route-type.
	assert.Equal(t, "route-type", args[0])
	assert.Equal(t, "interwork-segment-discovery", args[1])

	// Check all fields are present.
	assert.Contains(t, args, "rd")
	assert.Contains(t, args, "prefix")
	assert.Contains(t, args, "address")
	assert.Contains(t, args, "teid")
	assert.Contains(t, args, "qfi")
	assert.Contains(t, args, "endpoint")
	assert.Contains(t, args, "source")
}

// TestMUPSpecToArgs_MinimalFields verifies only route-type with empty fields.
func TestMUPSpecToArgs_MinimalFields(t *testing.T) {
	spec := bgptypes.MUPRouteSpec{
		RouteType: "direct-segment-discovery",
	}

	args := mupSpecToArgs(spec)
	assert.Equal(t, []string{"route-type", "direct-segment-discovery"}, args)
}
