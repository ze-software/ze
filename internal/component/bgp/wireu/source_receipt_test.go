// Design: docs/architecture/memory/lifetime-contracts.md -- source identity is a scalar, not byte ownership.
// Related: wire_update.go -- SourceReceipt and byte-identical snapshots.
package wireu

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSnapshotPreservesOriginalSourceReceipt proves a retained byte copy keeps
// its issuing identity while borrowing none of the original sent callback view.
func TestSnapshotPreservesOriginalSourceReceipt(t *testing.T) {
	original := NewWireUpdate([]byte{0, 0, 0, 0}, 0)
	original.SetSourceReceipt(701)
	original.SetSentOrigin(&SentOrigin{SourceReceipt: 702})
	owned := original.Snapshot()
	require.Equal(t, uint64(701), owned.SourceReceipt())
	require.Nil(t, owned.SentOrigin(), "a sent callback's borrow is not retained")
	original.SetSourceReceipt(703)
	require.Equal(t, uint64(701), owned.SourceReceipt(), "the copy owns its original scalar")
}

// TestUnrecordedWireUpdateHasNoSourceReceipt proves both construction routes
// keep explicit zero until the Session stamps an original receive identity.
func TestUnrecordedWireUpdateHasNoSourceReceipt(t *testing.T) {
	ordinary := NewWireUpdate([]byte{0, 0, 0, 0}, 0)
	require.Zero(t, ordinary.SourceReceipt())
	var inline WireUpdate
	InitWireUpdate(&inline, []byte{0, 0, 0, 0}, 0)
	require.Zero(t, inline.SourceReceipt())
	require.Zero(t, inline.Snapshot().SourceReceipt())
}
