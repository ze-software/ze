// Design: docs/architecture/plugin/rib-storage-design.md -- address-wide retained source capture.
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// An operator purge uses original retained lease addresses, including every
// independently owned source at that address. Current metadata cannot replace
// those originals, and an unrelated address remains independently useful.
// MUTATION: capture only one current owner instead of every original lease.
func TestPurgeCaptureUsesOriginalLeaseOwnersAtAddress(t *testing.T) {
	r, _ := sourceLeaseTestRIB(t, true)
	source := netip.MustParseAddr("192.0.2.230")
	other := netip.MustParseAddr("192.0.2.232")
	first, err := r.sourceLeases.acquire(t.Context(), source, 1230, 8230, 2230)
	require.NoError(t, err)
	t.Cleanup(first.Release)
	second, err := r.sourceLeases.acquire(t.Context(), source, 1231, 8231, 2231)
	require.NoError(t, err)
	t.Cleanup(second.Release)
	survivor, err := r.sourceLeases.acquire(t.Context(), other, 1232, 8232, 2232)
	require.NoError(t, err)
	t.Cleanup(survivor.Release)
	r.peerMeta[1230] = &peerMetadata{Address: other}
	r.peerMeta[1231] = &peerMetadata{Address: other}
	require.ElementsMatch(t, []uint64{8230, 8231}, r.sourceLeases.receiptsAtAddress(source))
	require.Equal(t, []uint64{8232}, r.sourceLeases.receiptsAtAddress(other))
	require.Empty(t, r.sourceLeases.receiptsAtAddress(netip.MustParseAddr("192.0.2.233")))
	require.Equal(t, []uint64{8230}, r.sourceLeases.receiptsForPeer(1230))
	require.Equal(t, []uint64{8231}, r.sourceLeases.receiptsForPeer(1231))
}
