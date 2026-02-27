package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFormatRouteCommand_MinimalRoute verifies command with only required fields.
//
// VALIDATES: FormatRouteCommand outputs "update text nhop <nh> nlri <fam> add <prefix>".
// PREVENTS: Missing required fields in replay commands.
func TestFormatRouteCommand_MinimalRoute(t *testing.T) {
	route := &Route{
		Family:  "ipv4/unicast",
		Prefix:  "10.0.0.0/24",
		NextHop: "10.0.0.1",
	}

	cmd := FormatRouteCommand(route)
	assert.Equal(t, "update text nhop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24", cmd)
}

// TestFormatRouteCommand_FullAttributes verifies all attribute fields in command.
//
// VALIDATES: FormatRouteCommand includes origin, as-path, med, local-preference, communities.
// PREVENTS: Attributes silently dropped from replay commands.
func TestFormatRouteCommand_FullAttributes(t *testing.T) {
	med := uint32(100)
	localPref := uint32(200)
	route := &Route{
		Family:          "ipv4/unicast",
		Prefix:          "10.0.0.0/24",
		NextHop:         "10.0.0.1",
		Origin:          "igp",
		ASPath:          []uint32{65001, 65002},
		MED:             &med,
		LocalPreference: &localPref,
		Communities:     []string{"65001:100"},
	}

	cmd := FormatRouteCommand(route)
	assert.Contains(t, cmd, "origin igp")
	assert.Contains(t, cmd, "as-path [65001 65002]")
	assert.Contains(t, cmd, "med 100")
	assert.Contains(t, cmd, "local-preference 200")
	assert.Contains(t, cmd, "community [65001:100]")
	assert.Contains(t, cmd, "nhop 10.0.0.1")
	assert.Contains(t, cmd, "nlri ipv4/unicast add 10.0.0.0/24")
}

// TestFormatRouteCommand_WithPathID verifies RFC 7911 path-id in command.
//
// VALIDATES: FormatRouteCommand includes "path-information N" as per-NLRI modifier.
// PREVENTS: Path-id silently dropped, breaking ADD-PATH replay.
func TestFormatRouteCommand_WithPathID(t *testing.T) {
	route := &Route{
		Family:  "ipv4/unicast",
		Prefix:  "10.0.0.0/24",
		NextHop: "10.0.0.1",
		PathID:  42,
	}

	cmd := FormatRouteCommand(route)
	assert.Contains(t, cmd, "nlri ipv4/unicast path-information 42 add 10.0.0.0/24")
}

// TestFormatRouteCommand_IPv6 verifies IPv6 route formatting.
//
// VALIDATES: FormatRouteCommand works with IPv6 family and addresses.
// PREVENTS: IPv6 handling broken in shared package.
func TestFormatRouteCommand_IPv6(t *testing.T) {
	route := &Route{
		Family:  "ipv6/unicast",
		Prefix:  "2001:db8::/32",
		NextHop: "::1",
		Origin:  "igp",
	}

	cmd := FormatRouteCommand(route)
	assert.Equal(t, "update text origin igp nhop ::1 nlri ipv6/unicast add 2001:db8::/32", cmd)
}

// TestFormatRouteCommand_ExtendedCommunities verifies extended and large community output.
//
// VALIDATES: FormatRouteCommand includes large-community and extended-community fields.
// PREVENTS: Extended community types silently dropped.
func TestFormatRouteCommand_ExtendedCommunities(t *testing.T) {
	route := &Route{
		Family:              "ipv4/unicast",
		Prefix:              "10.0.0.0/24",
		NextHop:             "10.0.0.1",
		LargeCommunities:    []string{"65001:0:100"},
		ExtendedCommunities: []string{"target:65001:100"},
	}

	cmd := FormatRouteCommand(route)
	assert.Contains(t, cmd, "large-community [65001:0:100]")
	assert.Contains(t, cmd, "extended-community [target:65001:100]")
}
