package storage

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/bgp/nlri"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDirectNLRISet_AddRemove verifies IPv4 NLRI storage.
//
// VALIDATES: Direct wire byte storage for small NLRIs (1-5 bytes).
// PREVENTS: Memory overhead from pooling small NLRIs.
func TestDirectNLRISet_AddRemove(t *testing.T) {
	set := NewDirectNLRISet(nlri.IPv4Unicast, false)

	// Add three /24 prefixes
	prefix1 := []byte{24, 10, 0, 0}    // 10.0.0.0/24
	prefix2 := []byte{24, 10, 0, 1}    // 10.0.1.0/24
	prefix3 := []byte{24, 192, 168, 0} // 192.168.0.0/24

	set.Add(prefix1)
	set.Add(prefix2)
	set.Add(prefix3)

	assert.Equal(t, 3, set.Len())
	assert.True(t, set.Contains(prefix1))
	assert.True(t, set.Contains(prefix2))
	assert.True(t, set.Contains(prefix3))
	assert.False(t, set.Contains([]byte{24, 10, 0, 2})) // not added

	// Remove middle prefix
	removed := set.Remove(prefix2)
	assert.True(t, removed)
	assert.Equal(t, 2, set.Len())
	assert.False(t, set.Contains(prefix2))
	assert.True(t, set.Contains(prefix1))
	assert.True(t, set.Contains(prefix3))

	// Remove non-existent
	removed = set.Remove([]byte{24, 10, 0, 2})
	assert.False(t, removed)
	assert.Equal(t, 2, set.Len())

	// Remove first
	removed = set.Remove(prefix1)
	assert.True(t, removed)
	assert.Equal(t, 1, set.Len())

	// Remove last
	removed = set.Remove(prefix3)
	assert.True(t, removed)
	assert.Equal(t, 0, set.Len())
}

// TestDirectNLRISet_VariableLengths verifies different prefix lengths.
//
// VALIDATES: Variable-length NLRI parsing (1-5 bytes for IPv4).
// PREVENTS: Incorrect offset calculation corrupting data.
func TestDirectNLRISet_VariableLengths(t *testing.T) {
	set := NewDirectNLRISet(nlri.IPv4Unicast, false)

	// Different prefix lengths
	host := []byte{32, 10, 0, 0, 1} // 10.0.0.1/32 (5 bytes)
	slash24 := []byte{24, 10, 0, 0} // 10.0.0.0/24 (4 bytes)
	slash16 := []byte{16, 10, 0}    // 10.0.0.0/16 (3 bytes)
	slash8 := []byte{8, 10}         // 10.0.0.0/8  (2 bytes)
	defaultR := []byte{0}           // 0.0.0.0/0   (1 byte)

	set.Add(host)
	set.Add(slash24)
	set.Add(slash16)
	set.Add(slash8)
	set.Add(defaultR)

	assert.Equal(t, 5, set.Len())

	// Verify each can be found
	assert.True(t, set.Contains(host))
	assert.True(t, set.Contains(slash24))
	assert.True(t, set.Contains(slash16))
	assert.True(t, set.Contains(slash8))
	assert.True(t, set.Contains(defaultR))

	// Remove some
	set.Remove(slash16)
	assert.Equal(t, 4, set.Len())
	assert.False(t, set.Contains(slash16))

	// Others still present
	assert.True(t, set.Contains(host))
	assert.True(t, set.Contains(slash24))
}

// TestDirectNLRISet_Iterate verifies iteration over NLRIs.
//
// VALIDATES: Iterate callback receives all NLRIs.
// PREVENTS: Missing NLRIs during route replay.
func TestDirectNLRISet_Iterate(t *testing.T) {
	set := NewDirectNLRISet(nlri.IPv4Unicast, false)

	prefixes := [][]byte{
		{24, 10, 0, 0},
		{24, 10, 0, 1},
		{16, 172, 16},
		{8, 192},
	}

	for _, p := range prefixes {
		set.Add(p)
	}

	// Collect via iteration
	var collected [][]byte
	set.Iterate(func(n []byte) bool {
		// Copy since NLRI may be slice of internal buffer
		cp := make([]byte, len(n))
		copy(cp, n)
		collected = append(collected, cp)
		return true
	})

	require.Len(t, collected, len(prefixes))
	for _, p := range prefixes {
		found := false
		for _, c := range collected {
			if string(c) == string(p) {
				found = true
				break
			}
		}
		assert.True(t, found, "prefix %v not found in iteration", p)
	}
}

// TestDirectNLRISet_IterateEarlyExit verifies early termination.
//
// VALIDATES: Iterate stops when callback returns false.
// PREVENTS: Unnecessary iteration when limit reached.
func TestDirectNLRISet_IterateEarlyExit(t *testing.T) {
	set := NewDirectNLRISet(nlri.IPv4Unicast, false)

	for i := 0; i < 10; i++ {
		set.Add([]byte{24, 10, 0, byte(i)})
	}

	count := 0
	set.Iterate(func(_ []byte) bool {
		count++
		return count < 3 // stop after 3
	})

	assert.Equal(t, 3, count)
}

// TestDirectNLRISet_AddPath verifies ADD-PATH support.
//
// VALIDATES: Path-ID prefix parsing in ADD-PATH mode.
// PREVENTS: Incorrect NLRI boundaries with 4-byte path-id prefix.
func TestDirectNLRISet_AddPath(t *testing.T) {
	set := NewDirectNLRISet(nlri.IPv4Unicast, true) // ADD-PATH enabled

	// ADD-PATH format: [path-id:4][prefix-len:1][prefix-bytes]
	nlri1 := []byte{0, 0, 0, 1, 24, 10, 0, 0} // path-id=1, 10.0.0.0/24
	nlri2 := []byte{0, 0, 0, 2, 24, 10, 0, 0} // path-id=2, 10.0.0.0/24 (same prefix, diff path)
	nlri3 := []byte{0, 0, 0, 1, 24, 10, 0, 1} // path-id=1, 10.0.1.0/24

	set.Add(nlri1)
	set.Add(nlri2)
	set.Add(nlri3)

	assert.Equal(t, 3, set.Len())
	assert.True(t, set.Contains(nlri1))
	assert.True(t, set.Contains(nlri2))
	assert.True(t, set.Contains(nlri3))

	// Remove nlri2 (same IP as nlri1 but different path-id)
	set.Remove(nlri2)
	assert.Equal(t, 2, set.Len())
	assert.True(t, set.Contains(nlri1))  // still present
	assert.False(t, set.Contains(nlri2)) // removed
	assert.True(t, set.Contains(nlri3))
}

// TestPooledNLRISet_AddRemove verifies IPv6+ NLRI pooling.
//
// VALIDATES: Pool-based storage for large NLRIs.
// PREVENTS: Memory waste from copying large IPv6/VPN NLRIs.
func TestPooledNLRISet_AddRemove(t *testing.T) {
	set := NewPooledNLRISet(nlri.IPv6Unicast, false)
	defer set.Release()

	// IPv6 /48 prefixes (7 bytes each)
	prefix1 := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01} // 2001:db8:1::/48
	prefix2 := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02} // 2001:db8:2::/48
	prefix3 := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x03} // 2001:db8:3::/48

	set.Add(prefix1)
	set.Add(prefix2)
	set.Add(prefix3)

	assert.Equal(t, 3, set.Len())
	assert.True(t, set.Contains(prefix1))
	assert.True(t, set.Contains(prefix2))
	assert.True(t, set.Contains(prefix3))
	assert.False(t, set.Contains([]byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x04}))

	// Remove middle
	removed := set.Remove(prefix2)
	assert.True(t, removed)
	assert.Equal(t, 2, set.Len())
	assert.False(t, set.Contains(prefix2))

	// Remove non-existent
	removed = set.Remove([]byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x04})
	assert.False(t, removed)
}

// TestPooledNLRISet_Iterate verifies iteration.
//
// VALIDATES: Iterate returns original bytes from pool.
// PREVENTS: Corrupt data on route replay.
func TestPooledNLRISet_Iterate(t *testing.T) {
	set := NewPooledNLRISet(nlri.IPv6Unicast, false)
	defer set.Release()

	prefixes := [][]byte{
		{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01},
		{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02},
		{64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x03, 0x00, 0x00},
	}

	for _, p := range prefixes {
		set.Add(p)
	}

	var collected [][]byte
	set.Iterate(func(n []byte) bool {
		cp := make([]byte, len(n))
		copy(cp, n)
		collected = append(collected, cp)
		return true
	})

	require.Len(t, collected, len(prefixes))
	for _, p := range prefixes {
		found := false
		for _, c := range collected {
			if string(c) == string(p) {
				found = true
				break
			}
		}
		assert.True(t, found, "prefix not found")
	}
}

// TestPooledNLRISet_AddPath verifies ADD-PATH with pooling.
//
// VALIDATES: Path-ID handling in pooled storage.
// PREVENTS: Wrong route removed when same prefix has multiple paths.
func TestPooledNLRISet_AddPath(t *testing.T) {
	set := NewPooledNLRISet(nlri.IPv6Unicast, true) // ADD-PATH
	defer set.Release()

	// ADD-PATH: [path-id:4][prefix-len:1][prefix-bytes]
	nlri1 := []byte{0, 0, 0, 1, 48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}
	nlri2 := []byte{0, 0, 0, 2, 48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01} // same prefix, diff path

	set.Add(nlri1)
	set.Add(nlri2)

	assert.Equal(t, 2, set.Len())
	assert.True(t, set.Contains(nlri1))
	assert.True(t, set.Contains(nlri2))

	set.Remove(nlri1)
	assert.Equal(t, 1, set.Len())
	assert.False(t, set.Contains(nlri1))
	assert.True(t, set.Contains(nlri2))
}

// TestNewNLRISet_FamilySelection verifies factory selects correct type.
//
// VALIDATES: IPv4 gets Direct, others get Pooled.
// PREVENTS: Wrong storage type causing memory overhead.
func TestNewNLRISet_FamilySelection(t *testing.T) {
	tests := []struct {
		family     nlri.Family
		wantDirect bool
	}{
		{nlri.IPv4Unicast, true},
		{nlri.IPv4Multicast, true},
		{nlri.IPv6Unicast, false},
		{nlri.IPv6Multicast, false},
		// VPN and others also pooled
		{nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFI(128)}, false}, // VPNv4
		{nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFI(128)}, false}, // VPNv6
	}

	for _, tt := range tests {
		t.Run(tt.family.String(), func(t *testing.T) {
			set := NewNLRISet(tt.family, false)
			defer set.Release()

			_, isDirect := set.(*DirectNLRISet)
			assert.Equal(t, tt.wantDirect, isDirect, "family %s: expected Direct=%v", tt.family, tt.wantDirect)
		})
	}
}

// TestDirectNLRISet_Release verifies Release is no-op.
//
// VALIDATES: Release doesn't crash for Direct storage.
// PREVENTS: Panic when releasing non-pooled set.
func TestDirectNLRISet_Release(t *testing.T) {
	set := NewDirectNLRISet(nlri.IPv4Unicast, false)
	set.Add([]byte{24, 10, 0, 0})
	set.Release() // should be no-op

	// Can still use after release (data owned by GC)
	assert.Equal(t, 1, set.Len())
}

// TestPooledNLRISet_DuplicateAdd verifies duplicate protection.
//
// VALIDATES: Adding same NLRI twice doesn't create duplicate handles.
// PREVENTS: Handle leaks and index corruption from duplicates.
func TestPooledNLRISet_DuplicateAdd(t *testing.T) {
	set := NewPooledNLRISet(nlri.IPv6Unicast, false)
	defer set.Release()

	prefix := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}

	set.Add(prefix)
	set.Add(prefix) // Duplicate - should be no-op
	set.Add(prefix) // Another duplicate

	assert.Equal(t, 1, set.Len()) // Only one entry
	assert.True(t, set.Contains(prefix))

	// Remove should work correctly
	removed := set.Remove(prefix)
	assert.True(t, removed)
	assert.Equal(t, 0, set.Len())
}

// TestDirectNLRISet_MalformedData verifies bounds protection.
//
// VALIDATES: Malformed data doesn't cause panic.
// PREVENTS: Buffer overrun from corrupted prefix length.
func TestDirectNLRISet_MalformedData(t *testing.T) {
	set := NewDirectNLRISet(nlri.IPv4Unicast, false)

	// Add valid prefix
	set.Add([]byte{24, 10, 0, 0})

	// Manually corrupt: append truncated prefix (claims /32 but no bytes)
	set.data = append(set.data, 32) // prefix len 32 = 4 bytes needed, but none provided
	set.count++

	// Iterate should handle gracefully (stop at malformed data)
	count := 0
	set.Iterate(func(n []byte) bool {
		count++
		return true
	})

	// Should only see the valid prefix
	assert.Equal(t, 1, count)
}

// TestDirectNLRISet_BoundaryPrefixLen verifies prefix length edge cases.
//
// VALIDATES: Valid prefix lengths accepted, truncated data handled.
// PREVENTS: Out-of-bounds access on malformed NLRI.
func TestDirectNLRISet_BoundaryPrefixLen(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantCount int
	}{
		{
			name:      "prefix_len_0_valid",
			data:      []byte{0}, // 0.0.0.0/0 - 1 byte total
			wantCount: 1,
		},
		{
			name:      "prefix_len_32_valid",
			data:      []byte{32, 10, 0, 0, 1}, // 10.0.0.1/32 - 5 bytes total
			wantCount: 1,
		},
		{
			name:      "prefix_len_33_truncated",
			data:      []byte{33, 10, 0, 0, 1}, // Invalid: /33 needs 5 bytes, have 4
			wantCount: 0,                       // Should stop - can't read full NLRI
		},
		{
			name:      "prefix_len_255_truncated",
			data:      []byte{255}, // /255 needs 32 bytes, have 0
			wantCount: 0,
		},
		{
			name:      "empty_data",
			data:      []byte{},
			wantCount: 0,
		},
		{
			name:      "two_valid_prefixes",
			data:      []byte{24, 10, 0, 0, 16, 172, 16}, // /24 + /16
			wantCount: 2,
		},
		{
			name:      "valid_then_truncated",
			data:      []byte{24, 10, 0, 0, 32}, // Valid /24, then truncated /32
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := &DirectNLRISet{
				data:   tt.data,
				count:  tt.wantCount + 1, // Intentionally wrong to verify iteration
				family: nlri.IPv4Unicast,
			}

			count := 0
			set.Iterate(func(_ []byte) bool {
				count++
				return true
			})

			assert.Equal(t, tt.wantCount, count)
		})
	}
}

// TestDirectNLRISet_AddPathBoundary verifies ADD-PATH truncation handling.
//
// VALIDATES: ADD-PATH with truncated data handled gracefully.
// PREVENTS: Panic when ADD-PATH header incomplete.
func TestDirectNLRISet_AddPathBoundary(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantCount int
	}{
		{
			name:      "valid_addpath",
			data:      []byte{0, 0, 0, 1, 24, 10, 0, 0}, // path-id=1, 10.0.0.0/24
			wantCount: 1,
		},
		{
			name:      "truncated_pathid",
			data:      []byte{0, 0, 0}, // Only 3 bytes of path-id
			wantCount: 0,
		},
		{
			name:      "truncated_prefix",
			data:      []byte{0, 0, 0, 1, 32, 10}, // path-id + /32 but only 2 prefix bytes
			wantCount: 0,
		},
		{
			name:      "two_valid_addpath",
			data:      []byte{0, 0, 0, 1, 24, 10, 0, 0, 0, 0, 0, 2, 16, 172, 16},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := &DirectNLRISet{
				data:    tt.data,
				count:   10, // Wrong count to verify iteration stops correctly
				family:  nlri.IPv4Unicast,
				addPath: true,
			}

			count := 0
			set.Iterate(func(_ []byte) bool {
				count++
				return true
			})

			assert.Equal(t, tt.wantCount, count)
		})
	}
}
