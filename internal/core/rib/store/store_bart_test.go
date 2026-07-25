//go:build !maprib

package store

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/family"
)

// TestBARTMasksPrefix verifies the BART backend normalizes prefixes to their
// canonical masked form. 10.0.0.1/24 and 10.0.0.0/24 map to the same slot;
// re-inserting via either address returns the same entry. This is BART trie
// behavior; the map backend (under -tags maprib) treats them as distinct keys
// because netip.Prefix comparison keys on the full address.
//
// VALIDATES: trie backend masks off host bits in the prefix key.
// PREVENTS: caller relying on host-bit preservation (a latent portability bug
// across the !maprib / maprib backends).
func TestBARTMasksPrefix(t *testing.T) {
	s := NewStore[int](family.IPv4Unicast)

	unmasked := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, 0, 1}), 24)
	masked := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, 0, 0}), 24)

	s.Insert(unmasked, 1)
	assert.Equal(t, 1, s.Len(), "insert with host bits set produces one entry")

	v, ok := s.Lookup(masked)
	assert.True(t, ok, "lookup via canonical form finds the entry BART masked to")
	assert.Equal(t, 1, v)
}

func TestStoreLookupLPM(t *testing.T) {
	s := NewStore[int](family.IPv4Unicast)

	s.Insert(netip.MustParsePrefix("10.0.0.0/8"), 1)
	s.Insert(netip.MustParsePrefix("10.1.0.0/16"), 2)
	s.Insert(netip.MustParsePrefix("10.1.2.0/24"), 3)

	tests := []struct {
		name    string
		addr    netip.Addr
		wantVal int
		wantPfx netip.Prefix
		wantOK  bool
	}{
		{"most specific /24", netip.MustParseAddr("10.1.2.5"), 3, netip.MustParsePrefix("10.1.2.0/24"), true},
		{"mid specificity /16", netip.MustParseAddr("10.1.3.1"), 2, netip.MustParsePrefix("10.1.0.0/16"), true},
		{"least specific /8", netip.MustParseAddr("10.2.0.1"), 1, netip.MustParsePrefix("10.0.0.0/8"), true},
		{"no match", netip.MustParseAddr("192.168.1.1"), 0, netip.Prefix{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, pfx, ok := s.LookupLPM(tt.addr)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantVal, val)
				assert.Equal(t, tt.wantPfx, pfx)
			}
		})
	}
}

func TestStoreLookupLPM_Invalid(t *testing.T) {
	s := NewStore[int](family.IPv4Unicast)
	s.Insert(netip.MustParsePrefix("10.0.0.0/8"), 1)

	val, pfx, ok := s.LookupLPM(netip.Addr{})
	assert.False(t, ok)
	assert.Equal(t, 0, val)
	assert.Equal(t, netip.Prefix{}, pfx)
}

func TestStoreLookupLPM_IPv6(t *testing.T) {
	s := NewStore[int](family.IPv6Unicast)

	s.Insert(netip.MustParsePrefix("2001:db8::/32"), 10)
	s.Insert(netip.MustParsePrefix("2001:db8:1::/48"), 20)

	val, pfx, ok := s.LookupLPM(netip.MustParseAddr("2001:db8:1::1"))
	assert.True(t, ok)
	assert.Equal(t, 20, val)
	assert.Equal(t, netip.MustParsePrefix("2001:db8:1::/48"), pfx)

	val, pfx, ok = s.LookupLPM(netip.MustParseAddr("2001:db8:2::1"))
	assert.True(t, ok)
	assert.Equal(t, 10, val)
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), pfx)
}
