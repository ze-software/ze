// Design: docs/architecture/rib/unified-locrib.md -- unchanged offers retain producer originals.
// VALIDATES: avoiding member copies never skips storing the latest offer.

package locrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

func TestLocRIBUnchangedEffectivePathKeepsLatestProducerValues(t *testing.T) {
	rib := NewRIB()
	prefix := netip.MustParsePrefix("198.18.231.0/24")
	path := explicitForwardingCandidate()
	path.Tag, path.HasTag = 1, true
	path.DistanceOverride = 30
	usePolicy(t, rib, map[redistevents.ProtocolID]ribpolicy.Chain{path.Source: chainFunc(func(ribpolicy.Route) ribpolicy.Verdict {
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept, Sets: ribpolicy.Sets{
			Tag: 7, HasTag: true, Distance: 20, HasDistance: true,
		}}
	})}, nil)
	rib.Insert(family.IPv4Unicast, prefix, path)
	before := rib.Revision()
	notifications := 0
	unsubscribe := rib.OnChange(func(Change) { notifications++ })
	defer unsubscribe()
	path.Tag, path.DistanceOverride = 2, 40
	rib.Insert(family.IPv4Unicast, prefix, path)
	if notifications != 0 || rib.Revision() != before {
		t.Fatal("equal effective offer changed the selected view")
	}
	clearPolicy(rib)
	best, found := rib.Best(family.IPv4Unicast, prefix)
	if !found {
		t.Fatal("clearing the accepting policy lost the route")
	}
	if best.Tag != 2 || !best.HasTag || best.AdminDistance != 40 || best.DistanceOverride != 40 || !best.HasDistanceOverride {
		t.Fatalf("equal effective offer lost its producer originals: %+v", best)
	}
	if notifications != 1 || rib.Revision() != before+1 {
		t.Fatal("restoring the latest producer values did not notify selection")
	}
}
