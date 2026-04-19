package store

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// makeNLRI builds IPv4 /24 NLRI wire bytes for testing: [prefix-len][bytes...].
func makeNLRI(octets ...byte) []byte {
	b := []byte{24}
	return append(b, octets...)
}

// makeAPNLRI prefixes a 4-byte path-id for ADD-PATH shape.
func makeAPNLRI(pathID uint32, octets ...byte) []byte {
	b := []byte{
		byte(pathID >> 24), byte(pathID >> 16), byte(pathID >> 8), byte(pathID),
		24,
	}
	return append(b, octets...)
}

// TestStoreInsertLookup verifies Insert then Lookup round-trips for both backends.
//
// VALIDATES: AC-6 -- Store[T] covers Insert/Lookup for addPath=false (trie) and addPath=true (map).
// PREVENTS: Lookup returning a zero value after Insert (backend dispatch broken).
func TestStoreInsertLookup(t *testing.T) {
	cases := []struct {
		name    string
		addPath bool
		nlri    []byte
	}{
		{"trie-ipv4", false, makeNLRI(10, 0, 0)},
		{"map-ap-ipv4", true, makeAPNLRI(42, 10, 0, 0)},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore[int](family.IPv4Unicast, tt.addPath)
			s.Insert(tt.nlri, 7)

			v, ok := s.Lookup(tt.nlri)
			require.True(t, ok, "value just inserted must be found")
			assert.Equal(t, 7, v)

			s.Insert(tt.nlri, 13)
			v, ok = s.Lookup(tt.nlri)
			require.True(t, ok)
			assert.Equal(t, 13, v, "re-insert overwrites")
		})
	}
}

// TestStoreLookupAbsent verifies Lookup on an absent key returns zero and false.
//
// VALIDATES: AC-6 -- Lookup for absent keys.
func TestStoreLookupAbsent(t *testing.T) {
	for _, addPath := range []bool{false, true} {
		s := NewStore[string](family.IPv4Unicast, addPath)
		var nlri []byte
		if addPath {
			nlri = makeAPNLRI(1, 10, 0, 0)
		} else {
			nlri = makeNLRI(10, 0, 0)
		}
		v, ok := s.Lookup(nlri)
		assert.False(t, ok)
		assert.Equal(t, "", v)
	}
}

// TestStoreDelete verifies Delete for present and absent keys on both backends.
//
// VALIDATES: AC-6 -- Delete semantics: true for present, false for absent.
func TestStoreDelete(t *testing.T) {
	for _, addPath := range []bool{false, true} {
		s := NewStore[int](family.IPv4Unicast, addPath)
		var nlri []byte
		if addPath {
			nlri = makeAPNLRI(7, 10, 0, 0)
		} else {
			nlri = makeNLRI(10, 0, 0)
		}

		assert.False(t, s.Delete(nlri), "delete on empty store returns false")

		s.Insert(nlri, 1)
		assert.True(t, s.Delete(nlri), "delete on present key returns true")
		assert.False(t, s.Delete(nlri), "second delete on same key returns false")

		_, ok := s.Lookup(nlri)
		assert.False(t, ok, "lookup after delete must be absent")
	}
}

// TestStoreLen verifies Len tracks Insert and Delete operations.
//
// VALIDATES: AC-6 -- Len reflects population accurately.
func TestStoreLen(t *testing.T) {
	for _, addPath := range []bool{false, true} {
		s := NewStore[int](family.IPv4Unicast, addPath)
		assert.Equal(t, 0, s.Len())

		nlris := [][]byte{}
		for i := range 5 {
			b := byte(i)
			var n []byte
			if addPath {
				n = makeAPNLRI(uint32(i+1), 10, b, 0)
			} else {
				n = makeNLRI(10, b, 0)
			}
			nlris = append(nlris, n)
			s.Insert(n, i)
		}
		assert.Equal(t, 5, s.Len())

		s.Delete(nlris[0])
		s.Delete(nlris[4])
		assert.Equal(t, 3, s.Len())
	}
}

// TestStoreIterate verifies every entry is visited and early return stops iteration.
//
// VALIDATES: AC-6 -- Iterate visits all; callback returning false halts.
func TestStoreIterate(t *testing.T) {
	for _, addPath := range []bool{false, true} {
		s := NewStore[int](family.IPv4Unicast, addPath)
		want := map[int]bool{}
		for i := range 4 {
			b := byte(i)
			var n []byte
			if addPath {
				n = makeAPNLRI(uint32(i+1), 10, b, 0)
			} else {
				n = makeNLRI(10, b, 0)
			}
			s.Insert(n, i)
			want[i] = true
		}

		seen := map[int]bool{}
		s.Iterate(func(_ []byte, v int) bool {
			seen[v] = true
			return true
		})
		assert.Equal(t, want, seen, "Iterate must yield every entry")

		count := 0
		s.Iterate(func(_ []byte, _ int) bool {
			count++
			return false
		})
		assert.Equal(t, 1, count, "returning false from callback stops iteration")
	}
}

// TestStoreModify verifies Modify applies mutations and reports presence.
//
// VALIDATES: AC-6 -- Modify mutates in place and returns true for present keys.
func TestStoreModify(t *testing.T) {
	for _, addPath := range []bool{false, true} {
		s := NewStore[int](family.IPv4Unicast, addPath)
		var nlri []byte
		if addPath {
			nlri = makeAPNLRI(5, 10, 0, 0)
		} else {
			nlri = makeNLRI(10, 0, 0)
		}

		assert.False(t, s.Modify(nlri, func(v *int) { *v = 99 }),
			"Modify on absent key returns false")

		s.Insert(nlri, 1)
		ok := s.Modify(nlri, func(v *int) { *v += 10 })
		assert.True(t, ok)

		v, found := s.Lookup(nlri)
		require.True(t, found)
		assert.Equal(t, 11, v, "mutation must persist")
	}
}

// TestStoreModifyAll verifies ModifyAll visits every entry and persists mutations.
//
// VALIDATES: AC-6 -- ModifyAll sweeps all entries with pointer access.
func TestStoreModifyAll(t *testing.T) {
	for _, addPath := range []bool{false, true} {
		s := NewStore[int](family.IPv4Unicast, addPath)
		for i := range 3 {
			b := byte(i)
			var n []byte
			if addPath {
				n = makeAPNLRI(uint32(i+1), 10, b, 0)
			} else {
				n = makeNLRI(10, b, 0)
			}
			s.Insert(n, i)
		}

		s.ModifyAll(func(v *int) { *v += 100 })

		var values []int
		s.Iterate(func(_ []byte, v int) bool {
			values = append(values, v)
			return true
		})
		sort.Ints(values)
		assert.Equal(t, []int{100, 101, 102}, values, "every entry must be mutated")
	}
}

// TestStoreMalformedNLRI_NoPanic verifies malformed NLRI bytes do not panic
// on any Store operation. The backend may silently drop (trie) or accept
// (map under -tags maprib) -- the invariant asserted here is that no call
// panics, regardless of backend.
//
// VALIDATES: AC-6 -- security concern: malformed input does not panic.
func TestStoreMalformedNLRI_NoPanic(t *testing.T) {
	s := NewStore[int](family.IPv4Unicast, false)
	malformed := []byte{33, 0, 0, 0, 0} // prefix-len 33 invalid for IPv4
	assert.NotPanics(t, func() { s.Insert(malformed, 1) })
	assert.NotPanics(t, func() { _, _ = s.Lookup(malformed) }) //nolint:errcheck // asserting no panic only
	assert.NotPanics(t, func() { _ = s.Delete(malformed) })
	assert.NotPanics(t, func() { _ = s.Modify(malformed, func(*int) {}) })
}
