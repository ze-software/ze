//go:build !maprib

package store

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
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
