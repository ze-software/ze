// Design: docs/architecture/plugin/rib-storage-design.md -- received source leases.
package storage

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

func TestSourceReferenceCloneLifetime(t *testing.T) {
	wakes := 0
	source := NewSourceReference(1, 41, func() { wakes++ })
	entry := NewRouteEntry()
	entry.SetSourceReference(source)
	source.Release()
	clone := entry.Clone()
	require.NotNil(t, clone)
	entry.Release()
	entry.Release()
	require.Equal(t, int64(1), source.References())
	require.Zero(t, wakes)
	clone.Release()
	clone.Release()
	require.Zero(t, source.References())
	require.Equal(t, 1, wakes)
}

// The equal-attribute and raw-byte branches must transfer lifetime as well as
// metadata, for native routes and ADD-PATH's valid path identifier zero.
func TestSourceReferenceStorageRefresh(t *testing.T) {
	for _, fam := range []family.Family{family.IPv4Unicast, {AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}} {
		for _, addPath := range []bool{false, true} {
			t.Run(fam.String()+map[bool]string{false: "-single", true: "-path-zero"}[addPath], func(t *testing.T) {
				rib := newFamilyRIB(fam, addPath)
				t.Cleanup(rib.Release)
				raw := []byte{8, 10}
				if fam.SAFI == family.SAFIFlowSpec {
					raw = []byte{2, 1, 0}
				}
				if addPath {
					raw = append([]byte{0, 0, 0, 0}, raw...)
				}
				attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
				entry, fp, attrLen, err := ParseRouteEntry(attrs)
				require.NoError(t, err)
				defer entry.Release()
				first := NewSourceReference(1, 41, nil)
				entry.SourceReceipt = 7
				entry.SetSourceReference(first)
				first.Release()
				rib.InsertEntry(raw, entry, fp, attrLen)
				require.Equal(t, int64(2), first.References())
				stored, found := rib.lookupEntry(raw)
				require.True(t, found)
				pinned := stored.Clone()
				require.NotNil(t, pinned)
				second := NewSourceReference(2, 41, nil)
				entry.SourceReceipt = 8
				entry.SetSourceReference(second)
				second.Release()
				// Zero fingerprint takes parsed equality rather than the quick
				// fingerprint branch; both must replace the original reference.
				rib.InsertEntry(raw, entry, 0, attrLen)
				require.Equal(t, int64(1), first.References(), "only the owned transaction snapshot retains the old source")
				require.Equal(t, int64(2), second.References())
				pinned.Release()
				require.Zero(t, first.References())
				third := NewSourceReference(3, 41, nil)
				entry.SourceReceipt = 9
				entry.SetSourceReference(third)
				third.Release()
				rib.InsertEntry(raw, entry, fp, attrLen)
				require.Zero(t, second.References(), "fingerprint refresh also replaces ownership")
				rib.Insert(attrs, raw)
				require.Equal(t, int64(1), third.References(), "raw insertion owns no authorized source")
				stored, found = rib.lookupEntry(raw)
				require.True(t, found)
				require.Zero(t, stored.SourceReceipt)
				entry.Release()
				require.Zero(t, third.References())
			})
		}
	}
}
