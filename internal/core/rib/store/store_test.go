package store

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// pfx4 builds an IPv4 /24 prefix 10.b.0.0/24.
func pfx4(b byte) netip.Prefix {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{10, b, 0, 0}), 24)
}

// TestStoreInsertLookup verifies Insert then Lookup round-trips.
//
// VALIDATES: Store.Insert / Store.Lookup round-trip.
// PREVENTS: Lookup returning a zero value after Insert.
func TestStoreInsertLookup(t *testing.T) {
	s := NewStore[int](family.IPv4Unicast)
	p := pfx4(0)
	s.Insert(p, 7)

	v, ok := s.Lookup(p)
	require.True(t, ok, "value just inserted must be found")
	assert.Equal(t, 7, v)

	s.Insert(p, 13)
	v, ok = s.Lookup(p)
	require.True(t, ok)
	assert.Equal(t, 13, v, "re-insert overwrites")
}

// TestStoreLookupAbsent verifies Lookup on an absent key returns zero and false.
func TestStoreLookupAbsent(t *testing.T) {
	s := NewStore[string](family.IPv4Unicast)
	v, ok := s.Lookup(pfx4(0))
	assert.False(t, ok)
	assert.Equal(t, "", v)
}

// TestStoreDelete verifies Delete for present and absent keys.
func TestStoreDelete(t *testing.T) {
	s := NewStore[int](family.IPv4Unicast)
	p := pfx4(0)

	assert.False(t, s.Delete(p), "delete on empty store returns false")

	s.Insert(p, 1)
	assert.True(t, s.Delete(p), "delete on present key returns true")
	assert.False(t, s.Delete(p), "second delete on same key returns false")

	_, ok := s.Lookup(p)
	assert.False(t, ok, "lookup after delete must be absent")
}

// TestStoreLen verifies Len tracks Insert and Delete operations.
func TestStoreLen(t *testing.T) {
	s := NewStore[int](family.IPv4Unicast)
	assert.Equal(t, 0, s.Len())

	prefixes := []netip.Prefix{}
	for i := range 5 {
		p := pfx4(byte(i))
		prefixes = append(prefixes, p)
		s.Insert(p, i)
	}
	assert.Equal(t, 5, s.Len())

	s.Delete(prefixes[0])
	s.Delete(prefixes[4])
	assert.Equal(t, 3, s.Len())
}

// TestStoreIterate verifies every entry is visited and early return stops iteration.
func TestStoreIterate(t *testing.T) {
	s := NewStore[int](family.IPv4Unicast)
	want := map[int]bool{}
	for i := range 4 {
		s.Insert(pfx4(byte(i)), i)
		want[i] = true
	}

	seen := map[int]bool{}
	s.Iterate(func(_ netip.Prefix, v int) bool {
		seen[v] = true
		return true
	})
	assert.Equal(t, want, seen, "Iterate must yield every entry")

	count := 0
	s.Iterate(func(_ netip.Prefix, _ int) bool {
		count++
		return false
	})
	assert.Equal(t, 1, count, "returning false from callback stops iteration")
}

// TestStoreModify verifies Modify applies mutations and reports presence.
func TestStoreModify(t *testing.T) {
	s := NewStore[int](family.IPv4Unicast)
	p := pfx4(0)

	assert.False(t, s.Modify(p, func(v *int) { *v = 99 }),
		"Modify on absent key returns false")

	s.Insert(p, 1)
	ok := s.Modify(p, func(v *int) { *v += 10 })
	assert.True(t, ok)

	v, found := s.Lookup(p)
	require.True(t, found)
	assert.Equal(t, 11, v, "mutation must persist")
}

// TestStoreModifyAll verifies ModifyAll visits every entry and persists mutations.
func TestStoreModifyAll(t *testing.T) {
	s := NewStore[int](family.IPv4Unicast)
	for i := range 3 {
		s.Insert(pfx4(byte(i)), i)
	}

	s.ModifyAll(func(v *int) { *v += 100 })

	var values []int
	s.Iterate(func(_ netip.Prefix, v int) bool {
		values = append(values, v)
		return true
	})
	slices.Sort(values)
	assert.Equal(t, []int{100, 101, 102}, values, "every entry must be mutated")
}

// TestStoreInvalidPrefix verifies invalid netip.Prefix values are silently
// ignored on every operation. No panic, no phantom entry.
func TestStoreInvalidPrefix(t *testing.T) {
	s := NewStore[int](family.IPv4Unicast)
	var invalid netip.Prefix // zero value
	assert.NotPanics(t, func() { s.Insert(invalid, 1) })
	assert.Equal(t, 0, s.Len())
	_, ok := s.Lookup(invalid)
	assert.False(t, ok)
	assert.False(t, s.Delete(invalid))
	assert.False(t, s.Modify(invalid, func(*int) {}))
}

// TestStoreCoveringMostSpecificFirst verifies the covering read answers every
// stored prefix that contains an address, and nothing else, in the order a
// covering lookup promises.
//
// VALIDATES: Store.Covering visits the containing prefixes, host route
// included, from the most specific to the least, and stops when fn says so.
// PREVENTS: a covering read that misses a less specific prefix, answers one
// that does not contain the address, or yields in an order a caller would
// have to sort again.
func TestStoreCoveringMostSpecificFirst(t *testing.T) {
	s := NewStore[string](family.IPv4Unicast)
	for _, p := range []string{
		"0.0.0.0/0", "10.0.0.0/8", "10.1.0.0/16", "10.1.2.3/32",
		"10.2.0.0/16", "10.1.3.0/24", "192.0.2.0/24",
	} {
		s.Insert(netip.MustParsePrefix(p), p)
	}

	var got []string
	s.Covering(netip.MustParseAddr("10.1.2.3"), func(pfx netip.Prefix, v string) bool {
		assert.Equal(t, pfx.String(), v, "the value is the one stored under that prefix")
		got = append(got, pfx.String())
		return true
	})
	assert.Equal(t, []string{"10.1.2.3/32", "10.1.0.0/16", "10.0.0.0/8", "0.0.0.0/0"}, got)

	var first []string
	s.Covering(netip.MustParseAddr("10.1.2.3"), func(pfx netip.Prefix, _ string) bool {
		first = append(first, pfx.String())
		return false
	})
	assert.Equal(t, []string{"10.1.2.3/32"}, first, "a false return stops the read")

	var none int
	s.Covering(netip.Addr{}, func(netip.Prefix, string) bool { none++; return true })
	assert.Zero(t, none, "an invalid address visits nothing")
}

// TestStoreCoveringIPv6 verifies the covering read on an IPv6 store, and that
// an IPv4 address never reaches IPv6 entries.
//
// VALIDATES: Store.Covering over IPv6 prefixes, most specific first.
// PREVENTS: an address of one version answered from the other version's routes.
func TestStoreCoveringIPv6(t *testing.T) {
	s := NewStore[int](family.IPv6Unicast)
	for i, p := range []string{"::/0", "2001:db8::/32", "2001:db8:1::/48", "2001:db9::/32"} {
		s.Insert(netip.MustParsePrefix(p), i)
	}

	var got []string
	s.Covering(netip.MustParseAddr("2001:db8:1::5"), func(pfx netip.Prefix, _ int) bool {
		got = append(got, pfx.String())
		return true
	})
	assert.Equal(t, []string{"2001:db8:1::/48", "2001:db8::/32", "::/0"}, got)

	var crossed int
	s.Covering(netip.MustParseAddr("10.1.2.3"), func(netip.Prefix, int) bool { crossed++; return true })
	assert.Zero(t, crossed, "an IPv4 address is covered by no IPv6 prefix")
}
