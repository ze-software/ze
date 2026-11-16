// Design: docs/architecture/bgp/interface-event-reactions.md -- connected-address membership
// Related: connected.go -- address-only subnet comparison at lifecycle boundaries

package network

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSharesSubnetScopedAddress checks actual scoped addresses against local
// interface prefixes. A transport zone must not hide an address inside a known
// subnet; removing that zone must not admit an address outside the prefix.
func TestSharesSubnetScopedAddress(t *testing.T) {
	linkLocal := netip.MustParsePrefix("fe80::1/64")
	global := netip.MustParsePrefix("2001:db8:1::1/64")
	v4 := netip.MustParsePrefix("192.0.2.1/24")
	tests := []struct {
		name     string
		prefixes []netip.Prefix
		addr     netip.Addr
		want     bool
	}{
		{"scoped-link-local", []netip.Prefix{linkLocal}, netip.MustParseAddr("fe80::2%if0"), true},
		{"scoped-global", []netip.Prefix{global}, netip.MustParseAddr("2001:db8:1::2%if0"), true},
		{"both-interface-and-peer-scoped", []netip.Prefix{netip.PrefixFrom(netip.MustParseAddr("fe80::1%if0"), 64)}, netip.MustParseAddr("fe80::2%if0"), true},
		{"unscoped-link-local", []netip.Prefix{linkLocal}, netip.MustParseAddr("fe80::2"), true},
		{"unscoped-global", []netip.Prefix{global}, netip.MustParseAddr("2001:db8:1::2"), true},
		{"native-ipv4", []netip.Prefix{v4}, netip.MustParseAddr("192.0.2.2"), true},
		{"mapped-ipv4", []netip.Prefix{v4}, netip.MustParseAddr("::ffff:192.0.2.2"), true},
		{"scoped-link-local-outside", []netip.Prefix{netip.MustParsePrefix("fe80::1/128")}, netip.MustParseAddr("fe80::2%if0"), false},
		{"scoped-global-outside", []netip.Prefix{global}, netip.MustParseAddr("2001:db8:2::2%if0"), false},
		{"native-ipv4-outside", []netip.Prefix{v4}, netip.MustParseAddr("198.51.100.2"), false},
		{"mapped-ipv4-outside", []netip.Prefix{v4}, netip.MustParseAddr("::ffff:198.51.100.2"), false},
		{"scoped-address-without-prefixes", nil, netip.MustParseAddr("fe80::2%if0"), false},
		{"invalid-address", []netip.Prefix{linkLocal}, netip.Addr{}, false},
		{"invalid-prefix", []netip.Prefix{{}}, netip.MustParseAddr("fe80::2%if0"), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, SharesSubnet(test.prefixes, test.addr))
		})
	}
}
