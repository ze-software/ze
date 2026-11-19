//go:build !debug

// Design: docs/architecture/plugin/rib-storage-design.md -- retained selected output
// Related: rib_advertise_capture.go -- label content outlives pool slot identity
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/core/family"
)

// TestAdvertiseCaptureLabelSlotReuseChangesRevision uses release-mode pool
// reuse to give a different label stack the old winner's numeric handle. The
// new stack must replace the captured NLRI while the old snapshot stays intact.
func TestAdvertiseCaptureLabelSlotReuseChangesRevision(t *testing.T) {
	r, peer := advertiseCaptureRIB(t)
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	prefix := netip.MustParsePrefix("198.18.76.0/24")
	cidr := ipv4Prefix(24, 198, 18, 76)
	routes := r.bgpPeers[advertiseCaptureOwner]
	attrs := unicastAttrs(peer.As4(), 30, 100)
	advertiseCapturePath(t, r, advertiseCaptureOwner, fam, cidr, attrs, 0, 0)
	const oldLabel uint32 = 900001
	old := pool.InternLabels([]uint32{oldLabel})
	require.True(t, old.IsValid())
	require.True(t, routes.SetLabelsIfRouteExists(fam, cidr, old))
	_, changed := r.checkBestPathChange(fam, cidr, false, nil)
	require.True(t, changed)
	first, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, labeledNLRI(0, false, prefix, []uint32{oldLabel}), first[0].keyNLRI)

	// Removing storage releases its label ref; the selected byte snapshot and
	// bestPrev's copied label values do not own a pool reference.
	routes.Remove(fam, cidr)
	replacement := attrpool.InvalidHandle
	var replacementLabel uint32
	// Each candidate is released before the next. The finite label range
	// crosses every shard while retaining at most one label-pool reference.
	for label := uint32(1); label <= 4096; label++ {
		handle := pool.InternLabels([]uint32{label})
		require.True(t, handle.IsValid())
		if handle == old {
			replacement, replacementLabel = handle, label
			break
		}
		require.NoError(t, pool.Labels.Release(handle))
	}
	require.True(t, replacement.IsValid(), "the regression must exercise actual production slot reuse")
	transferred := false
	t.Cleanup(func() {
		if !transferred {
			require.NoError(t, pool.Labels.Release(replacement))
		}
	})
	advertiseCapturePath(t, r, advertiseCaptureOwner, fam, cidr, attrs, 0, 0)
	require.True(t, routes.SetLabelsIfRouteExists(fam, cidr, replacement))
	transferred = true
	_, changed = r.checkBestPathChange(fam, cidr, false, nil)
	require.True(t, changed, "the existing election sees the changed label stack")
	current, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Greater(t, current[0].revision, first[0].revision)
	require.Equal(t, labeledNLRI(0, false, prefix, []uint32{replacementLabel}), current[0].keyNLRI)
	require.Equal(t, labeledNLRI(0, false, prefix, []uint32{oldLabel}), first[0].keyNLRI)
}
