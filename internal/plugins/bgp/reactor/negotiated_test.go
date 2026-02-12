package reactor

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"github.com/stretchr/testify/require"
)

// TestNegotiatedCapabilitiesHas_True verifies Has returns true for negotiated families.
//
// VALIDATES: Has() returns true for families in the map.
//
// PREVENTS: Missing family detection leading to routes not being sent.
func TestNegotiatedCapabilitiesHas_True(t *testing.T) {
	nc := &NegotiatedCapabilities{
		families: map[nlri.Family]bool{
			nlri.IPv4Unicast: true,
			nlri.IPv6Unicast: true,
		},
	}

	require.True(t, nc.Has(nlri.IPv4Unicast), "should have IPv4 unicast")
	require.True(t, nc.Has(nlri.IPv6Unicast), "should have IPv6 unicast")
}

// TestNegotiatedCapabilitiesHas_False verifies Has returns false for non-negotiated families.
//
// VALIDATES: Has() returns false for families not in the map.
//
// PREVENTS: Sending routes for families peer doesn't support.
func TestNegotiatedCapabilitiesHas_False(t *testing.T) {
	nc := &NegotiatedCapabilities{
		families: map[nlri.Family]bool{
			nlri.IPv4Unicast: true,
		},
	}

	require.False(t, nc.Has(nlri.IPv6Unicast), "should not have IPv6 unicast")
	require.False(t, nc.Has(nlri.IPv4VPN), "should not have IPv4 VPN")
}

// TestNegotiatedCapabilitiesHas_Nil verifies Has handles nil receiver safely.
//
// VALIDATES: Has() returns false for nil receiver without panic.
//
// PREVENTS: Panic when checking families on uninitialized peer.
func TestNegotiatedCapabilitiesHas_Nil(t *testing.T) {
	var nc *NegotiatedCapabilities
	require.False(t, nc.Has(nlri.IPv4Unicast), "nil receiver should return false")
}

// TestNegotiatedCapabilitiesFamilies_Order verifies Families returns sorted order.
//
// VALIDATES: Families() returns families sorted by AFI then SAFI.
//
// PREVENTS: Non-deterministic EOR ordering breaking tests.
func TestNegotiatedCapabilitiesFamilies_Order(t *testing.T) {
	nc := &NegotiatedCapabilities{
		families: map[nlri.Family]bool{
			nlri.IPv6Unicast:        true, // AFI=2, SAFI=1
			nlri.IPv4VPN:            true, // AFI=1, SAFI=128
			nlri.IPv4Unicast:        true, // AFI=1, SAFI=1
			nlri.IPv6LabeledUnicast: true, // AFI=2, SAFI=4
		},
	}

	families := nc.Families()
	require.Len(t, families, 4)

	// Should be sorted: IPv4Unicast (1,1), IPv4VPN (1,128), IPv6Unicast (2,1), IPv6LabeledUnicast (2,4)
	require.Equal(t, nlri.IPv4Unicast, families[0], "first should be IPv4 unicast")
	require.Equal(t, nlri.IPv4VPN, families[1], "second should be IPv4 VPN")
	require.Equal(t, nlri.IPv6Unicast, families[2], "third should be IPv6 unicast")
	require.Equal(t, nlri.IPv6LabeledUnicast, families[3], "fourth should be IPv6 labeled unicast")
}

// TestNegotiatedCapabilitiesFamilies_Nil verifies Families handles nil receiver safely.
//
// VALIDATES: Families() returns nil for nil receiver without panic.
//
// PREVENTS: Panic when iterating families on uninitialized peer.
func TestNegotiatedCapabilitiesFamilies_Nil(t *testing.T) {
	var nc *NegotiatedCapabilities
	require.Nil(t, nc.Families(), "nil receiver should return nil")
}

// TestNegotiatedCapabilitiesExtendedMessage verifies ExtendedMessage flag.
//
// VALIDATES: ExtendedMessage flag is accessible.
//
// PREVENTS: Wrong max message size calculation.
func TestNegotiatedCapabilitiesExtendedMessage(t *testing.T) {
	nc := &NegotiatedCapabilities{
		families:        map[nlri.Family]bool{nlri.IPv4Unicast: true},
		ExtendedMessage: true,
	}

	require.True(t, nc.ExtendedMessage, "should have extended message")

	nc2 := &NegotiatedCapabilities{
		families:        map[nlri.Family]bool{nlri.IPv4Unicast: true},
		ExtendedMessage: false,
	}

	require.False(t, nc2.ExtendedMessage, "should not have extended message")
}

// TestNegotiatedCapabilitiesFamilies_AllIncluded verifies all negotiated families are returned.
//
// VALIDATES: RFC 4724 Section 4 - EOR must be sent for ALL negotiated families,
// "including the case when there is no update to send".
//
// PREVENTS: Missing EORs for families that have no routes configured.
//
// Context: sendInitialRoutes() iterates over nc.Families() to send EORs.
// This test ensures all families are included, not just those with routes.
func TestNegotiatedCapabilitiesFamilies_AllIncluded(t *testing.T) {
	// Simulate a peer that negotiated multiple families but may have routes
	// only for some of them. Per RFC 4724, EORs must be sent for ALL.
	nc := &NegotiatedCapabilities{
		families: map[nlri.Family]bool{
			nlri.IPv4Unicast: true, // Has routes configured
			nlri.IPv6Unicast: true, // Has routes configured
			nlri.IPv4VPN:     true, // NO routes configured
			nlri.IPv6VPN:     true, // NO routes configured
			nlri.IPv4MVPN:    true, // NO routes configured
		},
	}

	families := nc.Families()

	// ALL 5 families must be returned, not just the 2 with routes
	require.Len(t, families, 5, "all negotiated families must be included for EOR")

	// Verify each family is present
	familySet := make(map[nlri.Family]bool)
	for _, f := range families {
		familySet[f] = true
	}

	require.True(t, familySet[nlri.IPv4Unicast], "IPv4 unicast must be included")
	require.True(t, familySet[nlri.IPv6Unicast], "IPv6 unicast must be included")
	require.True(t, familySet[nlri.IPv4VPN], "IPv4 VPN must be included (even without routes)")
	require.True(t, familySet[nlri.IPv6VPN], "IPv6 VPN must be included (even without routes)")
	require.True(t, familySet[nlri.IPv4MVPN], "IPv4 MVPN must be included (even without routes)")
}

// TestNegotiatedCapabilitiesFamilies_Empty verifies empty families returns empty slice.
//
// VALIDATES: Families() returns empty slice (not nil) for no families.
//
// PREVENTS: Nil slice causing issues in for-range loops.
func TestNegotiatedCapabilitiesFamilies_Empty(t *testing.T) {
	nc := &NegotiatedCapabilities{
		families: map[nlri.Family]bool{},
	}

	families := nc.Families()
	require.NotNil(t, families, "should return empty slice, not nil")
	require.Len(t, families, 0, "should have no families")
}
