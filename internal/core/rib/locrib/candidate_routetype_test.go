// VALIDATES: RouteType is carry-through metadata on a Loc-RIB Path. Equal
// compares it, so the FIB re-programs when only the forwarding action changes.
// key() ignores it, so it never changes which path wins arbitration.
// PREVENTS: a prefix that becomes a blackhole with an unchanged next-hop being
// deduped away and never reaching the kernel. Also prevents a route type read
// as a second identity axis, which lets one source hold two Paths.

package locrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/rib/routetype"
)

func routeTypePath(t routetype.Type) Path {
	return Path{
		Source:        1,
		Instance:      7,
		NextHop:       netip.MustParseAddr("192.0.2.254"),
		AdminDistance: 20,
		Metric:        0,
		RouteType:     t,
	}
}

// A route-type-only change MUST be visible to Equal. sysrib suppresses an
// emission when the best is Equal to the previous one. A route type excluded
// here leaves an already-installed prefix forwarding after the operator
// blackholed it.
func TestPathEqualRouteType(t *testing.T) {
	unicast := routeTypePath(routetype.Unicast)
	blackhole := routeTypePath(routetype.Blackhole)

	if unicast.Equal(blackhole) {
		t.Error("Equal ignores RouteType: a prefix turning into a blackhole would be deduped away")
	}
	if !unicast.Equal(routeTypePath(routetype.Unicast)) {
		t.Error("Equal is false for two identical Paths")
	}
}

// The route type MUST NOT enter the identity key. A source re-advertising a
// prefix with a new forwarding action is the SAME path updated. Insert must
// replace it in place.
func TestPathKeyIgnoresRouteType(t *testing.T) {
	if routeTypePath(routetype.Unicast).key() != routeTypePath(routetype.Blackhole).key() {
		t.Error("key() reads RouteType: one source would hold two Paths for one prefix")
	}
}

// An unstamped Path keeps the zero value, which the FIB reads as "no opinion"
// and installs as an ordinary route. Every producer that never heard of route
// types keeps working unchanged.
func TestPathRouteTypeDefaultsUnset(t *testing.T) {
	var p Path
	if p.RouteType != 0 {
		t.Errorf("zero Path RouteType = %v, want unset", p.RouteType)
	}
	if p.RouteType.Discards() {
		t.Error("an unstamped Path discards traffic")
	}
}
