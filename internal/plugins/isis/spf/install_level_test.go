// Design: docs/architecture/rib/unified-locrib.md -- the fields a Loc-RIB import filter reads.
//
// VALIDATES: an IS-IS route reaches the Loc-RIB carrying the level SPF selected
// it at (ribLevel), level 1 and level 2 alike, and no route tag.
// PREVENTS: a Loc-RIB `match { level [ 2 ]; }` filter judging every IS-IS route
// as level-less, which would hide or admit routes against the operator's config.

package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// TestISISInstallCarriesLevel installs one level-1 and one level-2 route
// through Apply and reads each path back from the Loc-RIB. Method: two prefixes
// at different levels, so a producer that wrote one constant level fails one of
// the two.
func TestISISInstallCarriesLevel(t *testing.T) {
	loc := locrib.NewRIB()
	in := NewInstaller(loc)

	l1 := netip.MustParsePrefix("10.21.0.0/24")
	l2 := netip.MustParsePrefix("10.22.0.0/24")
	nh := netip.MustParseAddr("10.0.0.1")
	in.Apply([]RouteEntry{
		{Prefix: l1, Metric: 10, Level: Level1, NextHops: []NextHop{{Addr: nh, Interface: "eth0"}}},
		{Prefix: l2, Metric: 20, Level: Level2, NextHops: []NextHop{{Addr: nh, Interface: "eth0"}}},
	})

	for _, tc := range []struct {
		prefix netip.Prefix
		want   ribpolicy.ISISLevel
	}{
		{l1, ribpolicy.ISISLevel1},
		{l2, ribpolicy.ISISLevel2},
	} {
		paths := lookupPaths(t, loc, tc.prefix)
		if len(paths) != 1 {
			t.Fatalf("%s: inserted %d paths, want 1", tc.prefix, len(paths))
		}
		if paths[0].Level != tc.want {
			t.Errorf("%s: path level = %d, want %d", tc.prefix, paths[0].Level, tc.want)
		}
		if paths[0].HasTag {
			t.Errorf("%s: IS-IS path carries a tag (%d); Ze has no IS-IS route tag", tc.prefix, paths[0].Tag)
		}
	}
}
