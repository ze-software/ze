package network

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectedPrefixesIncludesLoopback verifies ConnectedPrefixes reports the
// loopback subnets, the two prefixes every host carries.
//
// VALIDATES: the returned prefixes are masked and cover 127.0.0.1 and ::1.
func TestConnectedPrefixesIncludesLoopback(t *testing.T) {
	prefixes := ConnectedPrefixes()
	require.NotEmpty(t, prefixes, "host reports no interface addresses")

	for _, p := range prefixes {
		assert.Equal(t, p.Masked(), p, "prefix %s is not masked", p)
	}

	assert.True(t, SharesSubnet(prefixes, netip.MustParseAddr("127.0.0.1")),
		"IPv4 loopback is not covered by %v", prefixes)
	assert.True(t, SharesSubnet(prefixes, netip.MustParseAddr("::1")),
		"IPv6 loopback is not covered by %v", prefixes)
}

// TestSharesSubnet checks the containment test that decides RFC 2545 Section 3's
// "shares a common subnet" condition.
//
// VALIDATES: a covered address reports true; an uncovered address, an invalid
// address and an empty prefix list all report false (fail closed).
func TestSharesSubnet(t *testing.T) {
	prefixes := []netip.Prefix{
		netip.MustParsePrefix("2001:db8:1::/64"),
		netip.MustParsePrefix("192.0.2.0/24"),
	}

	tests := []struct {
		name     string
		prefixes []netip.Prefix
		addr     netip.Addr
		want     bool
	}{
		{"ipv6 inside", prefixes, netip.MustParseAddr("2001:db8:1::5"), true},
		{"ipv6 outside", prefixes, netip.MustParseAddr("2001:db8:2::5"), false},
		{"ipv4 inside", prefixes, netip.MustParseAddr("192.0.2.7"), true},
		{"ipv4 outside", prefixes, netip.MustParseAddr("198.51.100.7"), false},
		{"ipv4-mapped inside", prefixes, netip.MustParseAddr("::ffff:192.0.2.7"), true},
		{"invalid address", prefixes, netip.Addr{}, false},
		{"empty prefix list", nil, netip.MustParseAddr("2001:db8:1::5"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SharesSubnet(tt.prefixes, tt.addr))
		})
	}
}
