// Design: docs/architecture/update-building.md -- original source Peer identity.
package wireu

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSnapshotPreservesOriginalSourceOwner snapshots received bytes and then
// changes the original; the owned identity stays independent of sent provenance.
func TestSnapshotPreservesOriginalSourceOwner(t *testing.T) {
	original := NewWireUpdate([]byte{0, 0, 0, 0}, 0)
	original.SetSourceOwner(701)
	original.SetSentOrigin(&SentOrigin{SourceOwner: 702})
	owned := original.Snapshot()
	require.Equal(t, uint64(701), owned.SourceOwner())
	require.Nil(t, owned.SentOrigin(), "a received snapshot cannot borrow sent provenance")
	original.SetSourceOwner(703)
	require.Equal(t, uint64(701), owned.SourceOwner())
	var inline WireUpdate
	InitWireUpdate(&inline, []byte{0, 0, 0, 0}, 0)
	require.Zero(t, inline.SourceOwner())
	require.Zero(t, NewWireUpdate(nil, 0).SourceOwner())
}
