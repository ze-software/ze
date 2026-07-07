package redistevents

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: AC-7 -- ReleaseBatch zeroes a filled entry's per-entry value
// fields (OriginAS, Metric) so a recycled batch never leaks the prior route's
// origin AS or metric to the next producer. Relies on clear(b.Entries) in
// ReleaseBatch covering value-type additions with no per-field reset.
// PREVENTS: a pooled batch handing a stale OriginAS to a producer that leaves
// the field zero, injecting a wrong origin AS into a redistributed route.
func TestRouteChangeBatchPoolResetsOriginAS(t *testing.T) {
	b := AcquireBatch()
	b.Entries = append(b.Entries, RouteChangeEntry{
		Action:   ActionAdd,
		Prefix:   netip.MustParsePrefix("192.0.2.0/24"),
		Metric:   100,
		OriginAS: 64512,
	})
	// Keep a view over the backing array so we can inspect the slot the entry
	// lived in after ReleaseBatch truncates len back to 0.
	backing := b.Entries[:cap(b.Entries)]

	ReleaseBatch(b)

	assert.Equal(t, uint32(0), backing[0].OriginAS, "ReleaseBatch must zero the entry's OriginAS")
	assert.Equal(t, uint32(0), backing[0].Metric, "ReleaseBatch must zero the entry's Metric")

	// A re-acquired batch presents no entries; growing it back into the
	// recycled backing array must expose only zero-valued entries.
	b2 := AcquireBatch()
	require.Empty(t, b2.Entries, "re-acquired batch must start with no entries")
	grown := b2.Entries[:1]
	assert.Equal(t, uint32(0), grown[0].OriginAS, "recycled entry must have zero OriginAS")
	ReleaseBatch(b2)
}
