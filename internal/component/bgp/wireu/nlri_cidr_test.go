// Design: docs/architecture/core-design.md -- NLRI subset extraction for policy retention
// Related: nlri_cidr.go -- DropCIDREntries, KeepCIDREntries
package wireu

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeepCIDREntriesMatchesComplementOfDrop(t *testing.T) {
	// Three /24 IPv4 prefixes: 10.99.0.0/24 (off 0), 10.99.1.0/24 (off 4), 10.99.2.0/24 (off 8).
	section := []byte{
		24, 10, 99, 0,
		24, 10, 99, 1,
		24, 10, 99, 2,
	}

	dropped, ok := DropCIDREntries(nil, section, false, []uint16{4})
	require.True(t, ok)
	require.Equal(t, []byte{24, 10, 99, 0, 24, 10, 99, 2}, dropped)

	kept, ok := KeepCIDREntries(nil, section, false, []uint16{4})
	require.True(t, ok)
	require.Equal(t, []byte{24, 10, 99, 1}, kept)

	keptFirstLast, ok := KeepCIDREntries(nil, section, false, []uint16{0, 8})
	require.True(t, ok)
	require.Equal(t, []byte{24, 10, 99, 0, 24, 10, 99, 2}, keptFirstLast)
}

func TestKeepCIDREntriesAddPathAndErrors(t *testing.T) {
	// Two ADD-PATH /24 entries (8 bytes each: 4-byte path ID + 1-byte len + 3-byte prefix).
	section := []byte{
		0, 0, 0, 1, 24, 10, 99, 0,
		0, 0, 0, 2, 24, 10, 99, 1,
	}
	kept, ok := KeepCIDREntries(nil, section, true, []uint16{8})
	require.True(t, ok)
	require.Equal(t, []byte{0, 0, 0, 2, 24, 10, 99, 1}, kept)

	// Offset inside an entry must fail closed.
	_, ok = KeepCIDREntries(nil, section, true, []uint16{4})
	require.False(t, ok)

	// Offset past the end must fail closed.
	_, ok = KeepCIDREntries(nil, section, true, []uint16{16})
	require.False(t, ok)
}
