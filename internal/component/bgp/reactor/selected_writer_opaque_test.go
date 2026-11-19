// Design: docs/architecture/update-building.md -- native selected output.
// Related: selected_writer_test.go -- actual writer and original source leases.
package reactor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedWriterOpaqueKeysRetireIndependently observes actual MP output
// for two EVPN keys differing only in their route distinguisher. Removing the
// first key MUST leave the second key's current slot and native bytes intact.
func TestSelectedWriterOpaqueKeysRetireIndependently(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	f.peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{
		family.IPv4Unicast: true, fam: true,
	}})
	f.peer.session.negotiated = forwardFixtureNegotiation(f.peer)
	f.peer.refreshForwardFacts()
	left := f.route(t, 1, "192.0.2.79", 0, false)
	left.Family, left.Route.Family = fam.String(), fam.String()
	left.KeyNLRI = make([]byte, 27)
	left.KeyNLRI[0], left.KeyNLRI[1] = 1, 25
	left.Route.NLRIHex = hex.EncodeToString(left.KeyNLRI)
	right := left
	right.Revision, right.KeyNLRI = 2, bytes.Clone(left.KeyNLRI)
	right.KeyNLRI[9] = 1
	rightStored := *left.Route
	right.Route = &rightStored
	right.Route.NLRIHex = hex.EncodeToString(right.KeyNLRI)
	for _, route := range []rpc.SelectedRoute{left, right} {
		answer, err := f.send(t, t.Context(), route)
		require.NoError(t, err)
		require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	}
	absent := rpc.SelectedRoute{Family: fam.String(), KeyNLRI: left.KeyNLRI, Revision: 3, Kind: rpc.SelectedRouteAbsent}
	answer, err := f.send(t, t.Context(), absent)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedAbsent, answer.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 3)
	for i, native := range [][]byte{left.KeyNLRI, right.KeyNLRI} {
		reach, present := bodyPathAttr(t, bodies[i], 14)
		require.True(t, present)
		require.GreaterOrEqual(t, len(reach), 5)
		require.Equal(t, uint16(fam.AFI), binary.BigEndian.Uint16(reach[:2]))
		require.Equal(t, byte(fam.SAFI), reach[2])
		require.Equal(t, native, reach[5+int(reach[3]):])
	}
	withdrawal, present := bodyPathAttr(t, bodies[2], 15)
	require.True(t, present)
	require.GreaterOrEqual(t, len(withdrawal), 3)
	require.Equal(t, uint16(fam.AFI), binary.BigEndian.Uint16(withdrawal[:2]))
	require.Equal(t, byte(fam.SAFI), withdrawal[2])
	require.Equal(t, left.KeyNLRI, withdrawal[3:])
	require.Len(t, f.peer.adjOut.routes, 1)
	require.Len(t, f.peer.adjOut.selectedPaths, 1)
	_, remaining, err := selectedNativeKey(right)
	require.NoError(t, err)
	_, found := f.peer.adjOut.selectedPaths[remaining]
	require.True(t, found)
}

// TestSelectedWriterSourceFactsExcludeReconstructionMetadata prevents an
// authenticated operation's receipt and stale projection from becoming
// authority in a reusable original source-session snapshot.
func TestSelectedWriterSourceFactsExcludeReconstructionMetadata(t *testing.T) {
	_, facts := sourceReceiptFixture(t)
	operation := facts.source
	operation.receipt, operation.metadata = 701, selectedStaleMeta
	captured, err := newSourceReceiptFacts(operation, facts.connectionID)
	require.NoError(t, err)
	require.Zero(t, captured.source.receipt)
	require.Nil(t, captured.source.metadata)
}
