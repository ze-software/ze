// Design: docs/architecture/update-building.md -- native keys and bounded selected output.
// Related: selected_writer_test.go -- actual selected writer fixture.
package reactor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedNativeOpaqueKeysRemainIndependent uses two EVPN type-1 keys
// differing only in their route distinguisher, never a synthetic IP prefix.
func TestSelectedNativeOpaqueKeysRemainIndependent(t *testing.T) {
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	left := make([]byte, 27)
	left[0], left[1] = 1, 25
	right := bytes.Clone(left)
	right[9] = 1
	_, first, err := selectedNativeKey(rpc.SelectedRoute{Family: fam.String(), KeyNLRI: left, Revision: 1, Kind: rpc.SelectedRouteAbsent})
	require.NoError(t, err)
	_, second, err := selectedNativeKey(rpc.SelectedRoute{Family: fam.String(), KeyNLRI: right, Revision: 1, Kind: rpc.SelectedRouteAbsent})
	require.NoError(t, err)
	require.NotEqual(t, first, second)
}

// TestSelectedWriterNeverAdmittedChurnLeavesNoHistory submits unique absences
// through final completion. None was advertised or in flight at this recipient.
func TestSelectedWriterNeverAdmittedChurnLeavesNoHistory(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	for i := range 128 {
		route := rpc.SelectedRoute{Family: family.IPv4Unicast.String(), KeyNLRI: []byte{24, 198, 18, byte(i)},
			Revision: uint64(i + 1), Kind: rpc.SelectedRouteAbsent}
		answer, err := f.send(t, t.Context(), route)
		require.NoError(t, err)
		require.Equal(t, rpc.SelectedAbsent, answer.Results[0].Outcome)
	}
	require.Empty(t, f.conn.written())
	require.Empty(t, f.current)
	require.Empty(t, f.peer.adjOut.routes)
	require.Empty(t, f.peer.adjOut.selectedPaths)
	require.Empty(t, f.peer.adjOut.tombstones)
	require.Empty(t, f.peer.adjOut.released)
	require.Nil(t, f.peer.adjOut.entries.chunk)
}
