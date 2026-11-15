// Design: docs/architecture/route-selection.md -- side-effect-free prospective election
package locrib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// A preview uses normal ranking, replaces only its own contribution and leaves
// stored paths, the routing revision and event subscribers untouched.
func TestPreviewSelectedDoesNotInstallOrPublish(t *testing.T) {
	r := NewRIB()
	prefix := netip.MustParsePrefix("198.51.100.0/24")
	source := redistevents.RegisterProtocol("prospective-test")
	stored := Path{Source: source, Instance: 1, NextHop: netip.MustParseAddr("192.0.2.1"), HasDistanceOverride: true, DistanceOverride: 20}
	r.Insert(family.IPv4Unicast, prefix, stored)
	revision := r.Revision()
	events := 0
	unsubscribe := r.OnChange(func(Change) { events++ })
	defer unsubscribe()
	offered := stored
	offered.NextHop = netip.MustParseAddr("192.0.2.2")
	preview, found := r.PreviewSelected(family.IPv4Unicast, prefix, offered)
	require.True(t, found)
	require.Equal(t, offered.NextHop, preview.NextHop)
	actual, found := r.Best(family.IPv4Unicast, prefix)
	require.True(t, found)
	require.Equal(t, stored.NextHop, actual.NextHop)
	require.Equal(t, revision, r.Revision())
	require.Zero(t, events)

	// A second, worse source cannot replace the selected contribution.
	offered.Instance = 2
	offered.DistanceOverride = 200
	preview, found = r.PreviewSelected(family.IPv4Unicast, prefix, offered)
	require.True(t, found)
	require.Equal(t, stored.NextHop, preview.NextHop)
	require.Equal(t, revision, r.Revision())
	require.Zero(t, events)
}
