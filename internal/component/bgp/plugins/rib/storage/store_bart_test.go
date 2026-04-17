//go:build !maprib

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// TestStoreTrieDropsMalformed verifies the trie backend rejects NLRI with an
// invalid prefix length -- length is stored nowhere, Len stays zero. This
// behavior is trie-specific; the map backend (under -tags maprib) accepts any
// bytes because NLRIKey is a fixed-size copy.
//
// VALIDATES: AC-6 -- trie backend rejects malformed NLRI.
// PREVENTS: Corrupted wire bytes creating a phantom entry the map-only backend
// would otherwise store and serve back on lookup.
func TestStoreTrieDropsMalformed(t *testing.T) {
	s := NewStore[int](family.IPv4Unicast, false)
	malformed := []byte{33, 0, 0, 0, 0} // prefix-len 33 invalid for IPv4
	s.Insert(malformed, 1)
	assert.Equal(t, 0, s.Len(), "trie backend drops malformed input")

	_, ok := s.Lookup(malformed)
	assert.False(t, ok)
	assert.False(t, s.Delete(malformed))
	assert.False(t, s.Modify(malformed, func(*int) {}))
}
