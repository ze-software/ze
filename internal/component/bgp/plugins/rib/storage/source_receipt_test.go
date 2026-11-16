// Design: docs/architecture/plugin/rib-storage-design.md -- received source provenance
package storage

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// TestSourceReceiptStorageRefresh keeps attributes equal while replacing only
// the issuing session receipt, on both fingerprint and parsed-equality paths.
func TestSourceReceiptStorageRefresh(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fam     family.Family
		addPath bool
		raw     []byte
	}{
		{"cidr", family.IPv4Unicast, false, []byte{24, 192, 0, 2}},
		{"cidr-path-zero", family.IPv4Unicast, true, []byte{0, 0, 0, 0, 24, 192, 0, 2}},
		{"opaque", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, false, []byte{1, 1, 0}},
		{"opaque-path-zero", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, true, []byte{0, 0, 0, 0, 1, 1, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := NewPeerRIB("192.0.2.90")
			t.Cleanup(peer.Release)
			peer.SetAddPath(tc.fam, tc.addPath)
			entry, fp, size, err := ParseRouteEntry([]byte{0x40, 1, 1, 0})
			require.NoError(t, err)
			t.Cleanup(entry.Release)
			entry.MsgID, entry.SourcePolicyEpoch, entry.SourceReceipt = 19, 29, 39
			peer.InsertEntry(tc.fam, entry, fp, size, tc.raw)
			got, found := peer.Lookup(tc.fam, tc.raw)
			require.True(t, found)
			require.Equal(t, uint64(39), got.SourceReceipt)
			for _, fingerprint := range []uint64{fp, 0} {
				entry.SourceReceipt++
				peer.InsertEntry(tc.fam, entry, fingerprint, size, tc.raw)
				got, found = peer.Lookup(tc.fam, tc.raw)
				require.True(t, found)
				require.Equal(t, entry.SourceReceipt, got.SourceReceipt, "equal attributes must not preserve the previous session")
				require.Equal(t, uint64(19), got.MsgID)
				require.Equal(t, uint64(29), got.SourcePolicyEpoch)
			}
			entry.SourceReceipt = 0
			peer.InsertEntry(tc.fam, entry, fp, size, tc.raw)
			got, found = peer.Lookup(tc.fam, tc.raw)
			require.True(t, found)
			require.Zero(t, got.SourceReceipt, "an unrecorded producer cannot inherit authority")
		})
	}
}

// TestSourceReceiptCloneIsImmutable holds a retained clone while the source
// entry acquires a different original receipt.
func TestSourceReceiptCloneIsImmutable(t *testing.T) {
	entry, _, _, err := ParseRouteEntry([]byte{0x40, 1, 1, 0})
	require.NoError(t, err)
	t.Cleanup(entry.Release)
	entry.SourceReceipt = 47
	clone := entry.Clone()
	require.NotNil(t, clone)
	t.Cleanup(clone.Release)
	entry.SourceReceipt = 48
	require.Equal(t, uint64(47), clone.SourceReceipt)
}

// TestSourceReceiptRawInsertIsUnrecorded proves a byte-only reannouncement
// cannot retain a previously supplied source capability through a no-op insert.
func TestSourceReceiptRawInsertIsUnrecorded(t *testing.T) {
	for _, addPath := range []bool{false, true} {
		peer := NewPeerRIB("192.0.2.91")
		t.Cleanup(peer.Release)
		peer.SetAddPath(family.IPv4Unicast, addPath)
		raw := []byte{24, 192, 0, 2}
		if addPath {
			raw = append([]byte{0, 0, 0, 0}, raw...)
		}
		attrs := []byte{0x40, 1, 1, 0}
		entry, fp, size, err := ParseRouteEntry(attrs)
		require.NoError(t, err)
		entry.SourceReceipt = 67
		peer.InsertEntry(family.IPv4Unicast, entry, fp, size, raw)
		entry.Release()
		peer.Insert(family.IPv4Unicast, attrs, raw)
		got, found := peer.Lookup(family.IPv4Unicast, raw)
		require.True(t, found)
		require.Zero(t, got.SourceReceipt)
	}
}
