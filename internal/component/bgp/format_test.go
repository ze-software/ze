package bgp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// mustFamily looks up a registered family by name or fails the test.
func mustFamily(t *testing.T, name string) family.Family {
	t.Helper()
	f, ok := family.LookupFamily(name)
	if !ok {
		t.Fatalf("family.LookupFamily(%q) missing; call family.RegisterTestFamilies in TestMain", name)
	}
	return f
}

// TestFormatAnnounceCommand_MinimalRoute verifies command with only required fields.
//
// VALIDATES: FormatAnnounceCommand outputs "update text nhop <nh> nlri <fam> add <prefix>".
// PREVENTS: Missing required fields in replay commands.
func TestFormatAnnounceCommand_MinimalRoute(t *testing.T) {
	route := &Route{
		Family:  family.IPv4Unicast,
		Prefix:  "10.0.0.0/24",
		NextHop: "10.0.0.1",
	}

	cmd := FormatAnnounceCommand(route)
	assert.Equal(t, "update text nhop 10.0.0.1 nlri add 10.0.0.0/24", cmd)
}

// TestFormatAnnounceCommand_FullAttributes verifies all attribute fields in command.
//
// VALIDATES: FormatAnnounceCommand includes origin, as-path, med, local-preference, communities.
// PREVENTS: Attributes silently dropped from replay commands.
func TestFormatAnnounceCommand_FullAttributes(t *testing.T) {
	med := uint32(100)
	localPref := uint32(200)
	route := &Route{
		Family:          family.IPv4Unicast,
		Prefix:          "10.0.0.0/24",
		NextHop:         "10.0.0.1",
		Origin:          new(attribute.OriginIGP),
		ASPath:          []uint32{65001, 65002},
		MED:             &med,
		LocalPreference: &localPref,
		Communities:     []attribute.Community{attribute.Community(65001<<16 | 100)},
	}

	cmd := FormatAnnounceCommand(route)
	assert.Contains(t, cmd, "origin igp")
	assert.Contains(t, cmd, "as-path [65001 65002]")
	assert.Contains(t, cmd, "med 100")
	assert.Contains(t, cmd, "local-preference 200")
	assert.Contains(t, cmd, "community [65001:100]")
	assert.Contains(t, cmd, "nhop 10.0.0.1")
	assert.Contains(t, cmd, "nlri add 10.0.0.0/24")
}

// TestFormatAnnounceCommand_WithPathID verifies RFC 7911 path-id in command.
//
// VALIDATES: FormatAnnounceCommand includes "path-information N" as per-NLRI modifier.
// PREVENTS: Path-id silently dropped, breaking ADD-PATH replay.
func TestFormatAnnounceCommand_WithPathID(t *testing.T) {
	route := &Route{
		Family:  family.IPv4Unicast,
		Prefix:  "10.0.0.0/24",
		NextHop: "10.0.0.1",
		PathID:  42,
		AddPath: true,
	}

	cmd := FormatAnnounceCommand(route)
	assert.Contains(t, cmd, "nlri path-information 42 add 10.0.0.0/24")
}

// TestFormatAnnounceCommand_IPv6 verifies IPv6 route formatting.
//
// VALIDATES: FormatAnnounceCommand works with IPv6 family and addresses.
// PREVENTS: IPv6 handling broken in component package.
func TestFormatAnnounceCommand_IPv6(t *testing.T) {
	route := &Route{
		Family:  family.IPv6Unicast,
		Prefix:  "2001:db8::/32",
		NextHop: "::1",
		Origin:  new(attribute.OriginIGP),
	}

	cmd := FormatAnnounceCommand(route)
	assert.Equal(t, "update text origin igp nhop ::1 mp ipv6/unicast add 2001:db8::/32", cmd)
}

// TestFormatAnnounceCommand_ExtendedCommunities verifies extended and large community output.
//
// VALIDATES: FormatAnnounceCommand includes large-community and extended-community fields.
// PREVENTS: Extended community types silently dropped.
func TestFormatAnnounceCommand_ExtendedCommunities(t *testing.T) {
	route := &Route{
		Family:              family.IPv4Unicast,
		Prefix:              "10.0.0.0/24",
		NextHop:             "10.0.0.1",
		LargeCommunities:    []attribute.LargeCommunity{{GlobalAdmin: 65001, LocalData1: 0, LocalData2: 100}},
		ExtendedCommunities: []attribute.ExtendedCommunity{{0x00, 0x02, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x64}},
	}

	cmd := FormatAnnounceCommand(route)
	assert.Contains(t, cmd, "large-community [65001:0:100]")
	assert.Contains(t, cmd, "extended-community [0002fde900000064]")
}

// TestFormatAnnounceCommand_VPN verifies VPN route with RD and labels.
//
// VALIDATES: FormatAnnounceCommand includes rd and label modifiers in NLRI section.
// PREVENTS: VPN watchdog routes missing route distinguisher or labels.
func TestFormatAnnounceCommand_VPN(t *testing.T) {
	route := &Route{
		Family:  mustFamily(t, "ipv4/mpls-vpn"),
		Prefix:  "10.0.0.0/24",
		NextHop: "10.0.0.1",
		Origin:  new(attribute.OriginIGP),
		RD:      "65000:100",
		Labels:  []uint32{1000},
	}

	cmd := FormatAnnounceCommand(route)
	assert.Equal(t, "update text origin igp nhop 10.0.0.1 mp ipv4/mpls-vpn rd 65000:100 label 1000 add 10.0.0.0/24", cmd)
}

// TestFormatAnnounceCommand_NhopSelf verifies nhop self keyword.
//
// VALIDATES: FormatAnnounceCommand passes "self" as nhop value.
// PREVENTS: nhop self resolved prematurely instead of by engine per-peer.
func TestFormatAnnounceCommand_NhopSelf(t *testing.T) {
	route := &Route{
		Family:  family.IPv4Unicast,
		Prefix:  "10.0.0.0/24",
		NextHop: "self",
		Origin:  new(attribute.OriginIGP),
	}

	cmd := FormatAnnounceCommand(route)
	assert.Equal(t, "update text origin igp nhop self nlri add 10.0.0.0/24", cmd)
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
			route: &Route{Family: family.IPv4Unicast, Prefix: "10.0.0.0/24"},
			want:  "update text nlri del 10.0.0.0/24",
		},
		{
			name:  "ipv6",
			route: &Route{Family: family.IPv6Unicast, Prefix: "2001:db8:1::/48"},
			want:  "update text mp ipv6/unicast del 2001:db8:1::/48",
		},
		{
			name:  "with path-id",
			route: &Route{Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", PathID: 42, AddPath: true},
			want:  "update text nlri path-information 42 del 10.0.0.0/24",
		},
		{
			name:  "vpn with rd and label",
			route: &Route{Family: mustFamily(t, "ipv4/mpls-vpn"), Prefix: "10.0.0.0/24", RD: "65000:100", Labels: []uint32{1000}},
			want:  "update text mp ipv4/mpls-vpn rd 65000:100 label 1000 del 10.0.0.0/24",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := FormatWithdrawCommand(tt.route)
			assert.Equal(t, tt.want, cmd)
		})
	}
}

// TestAddPathIdentifierZeroIsAnIdentifier verifies replay commands carry the
// RFC 7911 Path Identifier exactly when the route carries one, whatever its value.
//
// VALIDATES: AC-10, RFC 7911 Sections 2 and 3: identifier 0 under ADD-PATH is
// written as "path-information 0" and as four identifier octets in update hex;
// a value the route does not carry is written nowhere.
// PREVENTS: Presence decided by comparing the identifier with zero, which
// re-advertised path 0 as a different route with no identifier.
func TestAddPathIdentifierZeroIsAnIdentifier(t *testing.T) {
	carried := &Route{Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", NextHop: "10.0.0.1", AddPath: true}
	assert.Contains(t, FormatAnnounceCommand(carried), "nlri path-information 0 add 10.0.0.0/24")
	assert.Equal(t, "update text nlri path-information 0 del 10.0.0.0/24", FormatWithdrawCommand(carried))
	assert.Equal(t, "00000000180a0000", RouteNLRIHex(carried))

	notCarried := &Route{Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", NextHop: "10.0.0.1", PathID: 42}
	assert.NotContains(t, FormatAnnounceCommand(notCarried), "path-information")
	assert.Equal(t, "update text nlri del 10.0.0.0/24", FormatWithdrawCommand(notCarried))
	assert.Equal(t, "180a0000", RouteNLRIHex(notCarried))

	raw := &Route{Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", RawNLRI: "000000001800a0000", AddPath: true}
	assert.Contains(t, FormatWithdrawCommand(raw), "update hex nlri ipv4/unicast addpath del")
}
