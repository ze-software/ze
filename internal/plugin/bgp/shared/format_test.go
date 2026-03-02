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

// TestFormatAnnounceCommand_VPN verifies VPN route with RD and labels.
//
// VALIDATES: FormatAnnounceCommand includes rd and label modifiers in NLRI section.
// PREVENTS: VPN watchdog routes missing route distinguisher or labels.
func TestFormatAnnounceCommand_VPN(t *testing.T) {
	route := &Route{
		Family:  "ipv4/vpn",
		Prefix:  "10.0.0.0/24",
		NextHop: "10.0.0.1",
		Origin:  "igp",
		RD:      "65000:100",
		Labels:  []uint32{1000},
	}

	cmd := FormatAnnounceCommand(route)
	assert.Equal(t, "update text origin igp nhop 10.0.0.1 nlri ipv4/vpn rd 65000:100 label 1000 add 10.0.0.0/24", cmd)
}

// TestFormatAnnounceCommand_NhopSelf verifies nhop self keyword.
//
// VALIDATES: FormatAnnounceCommand passes "self" as nhop value.
// PREVENTS: nhop self resolved prematurely instead of by engine per-peer.
func TestFormatAnnounceCommand_NhopSelf(t *testing.T) {
	route := &Route{
		Family:  "ipv4/unicast",
		Prefix:  "10.0.0.0/24",
		NextHop: "self",
		Origin:  "igp",
	}

	cmd := FormatAnnounceCommand(route)
	assert.Equal(t, "update text origin igp nhop self nlri ipv4/unicast add 10.0.0.0/24", cmd)
}

// TestFormatWithdrawCommand verifies withdrawal command generation.
//
// VALIDATES: FormatWithdrawCommand produces "update text nlri <family> del <prefix>".
// PREVENTS: Withdrawal commands missing family or prefix.
func TestFormatWithdrawCommand(t *testing.T) {
	tests := []struct {
		name  string
		route *Route
		want  string
	}{
		{
			name:  "basic ipv4",
			route: &Route{Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
			want:  "update text nlri ipv4/unicast del 10.0.0.0/24",
		},
		{
			name:  "ipv6",
			route: &Route{Family: "ipv6/unicast", Prefix: "2001:db8:1::/48"},
			want:  "update text nlri ipv6/unicast del 2001:db8:1::/48",
		},
		{
			name:  "with path-id",
			route: &Route{Family: "ipv4/unicast", Prefix: "10.0.0.0/24", PathID: 42},
			want:  "update text nlri ipv4/unicast path-information 42 del 10.0.0.0/24",
		},
		{
			name:  "vpn with rd and label",
			route: &Route{Family: "ipv4/vpn", Prefix: "10.0.0.0/24", RD: "65000:100", Labels: []uint32{1000}},
			want:  "update text nlri ipv4/vpn rd 65000:100 label 1000 del 10.0.0.0/24",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := FormatWithdrawCommand(tt.route)
			assert.Equal(t, tt.want, cmd)
		})
	}
}
