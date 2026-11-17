// Design: docs/architecture/plugin/rib-storage-design.md -- retained attribute metadata.
// Related: attrparse.go, routeentry.go, familyrib.go -- parse, ownership and reconstruction.
// RFC: rfc/short/rfc4271.md -- Section 5, a previous AS's Partial bit survives propagation.

package storage

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// TestKnownTransitivePartialSurvivesStorage checks literal attribute bytes after
// a retained clone outlives its original. Equal complete values remain complete.
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS." These are storage tests, not Session or socket proofs.
func TestKnownTransitivePartialSurvivesStorage(t *testing.T) {
	cases := []struct {
		name string
		wire []byte
	}{
		{"aggregator", []byte{0xc0, 7, 8, 0, 0, 0xfd, 0xe9, 192, 0, 2, 1}},
		{"community", []byte{0xc0, 8, 4, 0, 1, 0, 1}},
		{"extended-community", []byte{0xc0, 16, 8, 0, 2, 0xfd, 0xe9, 0, 0, 0, 1}},
		{"large-community", []byte{0xc0, 32, 12, 0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 0, 0, 0, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			complete, err := ParseAttributes(tc.wire)
			require.NoError(t, err)
			defer complete.Release()

			partial := bytes.Clone(tc.wire)
			partial[0] = 0xe0
			original, err := ParseAttributes(partial)
			require.NoError(t, err)
			// The original MUST release its references before the clone is read.
			// The clone MUST own its references until the deferred release.
			clone := original.Clone()
			original.Release()
			require.NotNil(t, clone)
			defer clone.Release()

			partialStorageExpect(t, &complete, tc.wire)
			partialStorageExpect(t, clone, partial)
			partialStorageExpect(t, &complete, tc.wire)
		})
	}
	t.Run("independent-bits-with-later-unknown", func(t *testing.T) {
		// Code 250 follows LARGE_COMMUNITY, exercising its ordered emission
		// branch as well as the trailing branch in the cases above.
		first := []byte{
			0xe0, 7, 8, 0, 0, 0xfd, 0xe9, 192, 0, 2, 1,
			0xc0, 8, 4, 0, 1, 0, 1,
			0xe0, 16, 8, 0, 2, 0xfd, 0xe9, 0, 0, 0, 1,
			0xc0, 32, 12, 0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 0, 0, 0, 2,
			0xe0, 250, 1, 9,
		}
		second := []byte{
			0xc0, 7, 8, 0, 0, 0xfd, 0xe9, 192, 0, 2, 1,
			0xe0, 8, 4, 0, 1, 0, 1,
			0xc0, 16, 8, 0, 2, 0xfd, 0xe9, 0, 0, 0, 1,
			0xe0, 32, 12, 0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 0, 0, 0, 2,
			0xe0, 250, 1, 9,
		}
		left, err := ParseAttributes(first)
		require.NoError(t, err)
		defer left.Release()
		right, err := ParseAttributes(second)
		require.NoError(t, err)
		defer right.Release()
		partialStorageExpect(t, &left, first)
		partialStorageExpect(t, &right, second)
		partialStorageExpect(t, &left, first)
	})
}

// TestPartialOnlyReplacementUpdatesStoredAttributes changes only the received
// Partial flag through each storage backend and both public insertion shapes.
// RFC 4271 Section 5's previous-AS rule applies independently to each received
// replacement; a new complete attribute must not inherit an old Partial bit.
// Repeating the same bytes must preserve the useful route and exact attributes.
func TestPartialOnlyReplacementUpdatesStoredAttributes(t *testing.T) {
	backends := []struct {
		name   string
		family family.Family
		nlri   []byte
	}{
		{"cidr", family.IPv4Unicast, []byte{24, 198, 18, 71}},
		{"opaque", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, []byte{2, 3, 1, 2, 3}},
	}
	for _, backend := range backends {
		for _, addPath := range []bool{false, true} {
			name := backend.name + "/plain"
			if addPath {
				name = backend.name + "/add-path-zero"
			}
			for _, parsed := range []bool{false, true} {
				insertion := "raw"
				if parsed {
					insertion = "parsed"
				}
				t.Run(name+"/"+insertion, func(t *testing.T) {
					rib := newFamilyRIB(backend.family, addPath)
					defer rib.Release()
					key := bytes.Clone(backend.nlri)
					if addPath {
						key = append([]byte{0, 0, 0, 0}, key...)
					}
					// Two route keys with identical values remain independent even
					// while one path changes only its attribute header.
					twin := bytes.Clone(key)
					twin[len(twin)-1]++
					complete := []byte{0xc0, 8, 4, 0, 1, 0, 1}
					partial := []byte{0xe0, 8, 4, 0, 1, 0, 1}
					partialStorageInsert(t, rib, parsed, complete, twin)
					for _, attrs := range [][]byte{partial, complete, partial} {
						partialStorageInsert(t, rib, parsed, attrs, key)
						partialStorageExpectRoute(t, rib, key, attrs)
						partialStorageExpectRoute(t, rib, twin, complete)
						partialStorageInsert(t, rib, parsed, attrs, key)
						partialStorageExpectRoute(t, rib, key, attrs)
						assert.Equal(t, 2, rib.Len(), "an identical receipt keeps both useful routes")
					}
				})
			}
		}
	}
}

func partialStorageExpect(t *testing.T, entry *RouteEntry, expected []byte) {
	t.Helper()
	wire, err := entry.ToWireBytes()
	require.NoError(t, err)
	assert.Equal(t, expected, wire, "reconstruction must preserve the literal flags, length and value")
}

func partialStorageInsert(t *testing.T, rib *FamilyRIB, parsed bool, attrs, key []byte) {
	t.Helper()
	if !parsed {
		rib.Insert(attrs, key)
		return
	}
	entry, fingerprint, length, err := ParseRouteEntry(attrs)
	require.NoError(t, err)
	defer entry.Release()
	rib.InsertEntry(key, entry, fingerprint, length)
}

func partialStorageExpectRoute(t *testing.T, rib *FamilyRIB, key, expected []byte) {
	t.Helper()
	entry, found := rib.lookupEntry(key)
	require.True(t, found, "the received route must remain present")
	partialStorageExpect(t, &entry, expected)
}
