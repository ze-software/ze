package locrib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
)

// Stable test ProtocolIDs. We do not use redistevents.RegisterProtocol
// because it is process-global; parallel tests would race on the registry
// state. Loc-RIB only cares about the numeric value of Source, not its
// registered name.
const (
	idStatic redistevents.ProtocolID = 1
	idBGP    redistevents.ProtocolID = 2
	idOSPF   redistevents.ProtocolID = 3
)

var (
	famV4 = family.IPv4Unicast
	pfx   = netip.MustParsePrefix("10.0.0.0/24")
)

func pathStatic(metric uint32) Path {
	return Path{
		Source:        idStatic,
		Instance:      0,
		NextHop:       netip.MustParseAddr("192.0.2.1"),
		AdminDistance: 1,
		Metric:        metric,
	}
}

func pathBGP(instance, metric uint32) Path {
	return Path{
		Source:        idBGP,
		Instance:      instance,
		NextHop:       netip.MustParseAddr("192.0.2.2"),
		AdminDistance: 20,
		Metric:        metric,
	}
}

func pathOSPF(metric uint32) Path {
	return Path{
		Source:        idOSPF,
		Instance:      0,
		NextHop:       netip.MustParseAddr("192.0.2.3"),
		AdminDistance: 110,
		Metric:        metric,
	}
}

// TestInsertSelectsByAdminDistance validates the cross-protocol best-path
// ranking: Static (1) beats BGP (20) beats OSPF (110).
//
// VALIDATES: selectBest picks the lowest AdminDistance across protocols.
// PREVENTS: routes from a less-trusted protocol overriding a more-trusted one.
func TestInsertSelectsByAdminDistance(t *testing.T) {
	r := NewRIB()

	// First insert: OSPF only -- it wins by default.
	best, changed := r.Insert(famV4, pfx, pathOSPF(10))
	require.True(t, changed)
	assert.Equal(t, idOSPF, best.Source)

	// BGP arrives with lower distance -- becomes new best.
	best, changed = r.Insert(famV4, pfx, pathBGP(1, 50))
	require.True(t, changed)
	assert.Equal(t, idBGP, best.Source)

	// Static trumps BGP.
	best, changed = r.Insert(famV4, pfx, pathStatic(0))
	require.True(t, changed)
	assert.Equal(t, idStatic, best.Source)

	// Another OSPF re-advertise: best unchanged.
	_, changed = r.Insert(famV4, pfx, pathOSPF(5))
	assert.False(t, changed, "re-advertising a non-best path does not change best")
}

// TestTiebreakByMetric validates the within-AdminDistance tiebreaker: lower
// Metric wins.
func TestTiebreakByMetric(t *testing.T) {
	r := NewRIB()

	r.Insert(famV4, pfx, pathBGP(1, 100))
	best, changed := r.Insert(famV4, pfx, pathBGP(2, 50))
	require.True(t, changed, "lower-metric BGP should become new best")
	assert.Equal(t, uint32(50), best.Metric)
	assert.Equal(t, uint32(2), best.Instance)
}

// TestUpsertReplacesSameSourceInstance verifies that re-inserting with the
// same (Source, Instance) overwrites in place rather than appending.
func TestUpsertReplacesSameSourceInstance(t *testing.T) {
	r := NewRIB()
	r.Insert(famV4, pfx, pathBGP(1, 100))
	r.Insert(famV4, pfx, pathBGP(1, 50))

	g, ok := r.Lookup(famV4, pfx)
	require.True(t, ok)
	assert.Len(t, g.Paths, 1, "same (source,instance) must upsert in place")
	assert.Equal(t, uint32(50), g.Paths[0].Metric)
}

// TestRemoveFallsBackToNextBest validates that removing the current best
// surfaces the next-best Path.
func TestRemoveFallsBackToNextBest(t *testing.T) {
	r := NewRIB()
	r.Insert(famV4, pfx, pathStatic(0))
	r.Insert(famV4, pfx, pathBGP(1, 10))

	best, changed := r.Remove(famV4, pfx, idStatic, 0)
	require.True(t, changed, "removing best must report change")
	assert.Equal(t, idBGP, best.Source, "BGP falls through as new best")

	_, changed = r.Remove(famV4, pfx, idBGP, 1)
	require.True(t, changed)

	_, ok := r.Lookup(famV4, pfx)
	assert.False(t, ok, "last path removed deletes the prefix entry")
}

// TestRemoveAbsent is a no-op returning (zero, false).
func TestRemoveAbsent(t *testing.T) {
	r := NewRIB()
	_, changed := r.Remove(famV4, pfx, idBGP, 1)
	assert.False(t, changed)
}

// TestBestAndLookup verify the read-only accessors.
func TestBestAndLookup(t *testing.T) {
	r := NewRIB()
	r.Insert(famV4, pfx, pathStatic(0))
	r.Insert(famV4, pfx, pathBGP(1, 10))

	best, ok := r.Best(famV4, pfx)
	require.True(t, ok)
	assert.Equal(t, idStatic, best.Source)

	g, ok := r.Lookup(famV4, pfx)
	require.True(t, ok)
	assert.Len(t, g.Paths, 2)
	assert.Equal(t, 1, r.Len(famV4), "two paths share one prefix => Len is 1")
}

// TestInvalidPathRejected verifies that a Path with Source=Unspecified is
// silently rejected.
func TestInvalidPathRejected(t *testing.T) {
	r := NewRIB()
	_, changed := r.Insert(famV4, pfx, Path{}) // zero Source == Unspecified
	assert.False(t, changed)
	assert.Equal(t, 0, r.Len(famV4))
}

// TestIterate walks every prefix in the family.
func TestIterate(t *testing.T) {
	r := NewRIB()
	r.Insert(famV4, netip.MustParsePrefix("10.0.0.0/24"), pathBGP(1, 10))
	r.Insert(famV4, netip.MustParsePrefix("10.1.0.0/24"), pathBGP(1, 10))
	r.Insert(famV4, netip.MustParsePrefix("10.2.0.0/24"), pathBGP(1, 10))

	seen := map[netip.Prefix]bool{}
	r.Iterate(famV4, func(p netip.Prefix, _ PathGroup) bool {
		seen[p] = true
		return true
	})
	assert.Len(t, seen, 3)
}

// TestFamilies returns every family that has at least one prefix.
func TestFamilies(t *testing.T) {
	r := NewRIB()
	assert.Empty(t, r.Families())

	r.Insert(famV4, pfx, pathBGP(1, 10))
	assert.Equal(t, []family.Family{famV4}, r.Families())
}
