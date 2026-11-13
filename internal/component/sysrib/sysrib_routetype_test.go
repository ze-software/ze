// VALIDATES: the typed BGP producer field and real authoritative selected
// value retain the forwarding action through the common FIB output path.
// PREVENTS: a blackhole becoming forwarding on ingestion, update, replay or display.

package sysrib

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

var routeTypeFamily = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}

// The typed producer candidate retains its route type through authority selection.
func TestSysribCarriesRouteTypeFromEventBus(t *testing.T) {
	s := newCandidateForwardingFixture(t, fixtureRouteOverride)
	_, out := s.offerCandidates(&incomingBatch{
		Protocol: "bgp",
		Family:   routeTypeFamily,
		Changes: []incomingChange{{
			Action:    routeaction.Add,
			Prefix:    netip.MustParsePrefix("192.0.2.1/32"),
			NextHop:   netip.MustParseAddr("198.51.100.1"),
			Priority:  20,
			RouteType: routetype.Blackhole,
		}},
	})
	if len(out) != 1 {
		t.Fatalf("got %d outgoing changes, want 1", len(out))
	}
	if out[0].RouteType != routetype.Blackhole {
		t.Errorf("RouteType = %v, want blackhole: the FIB would install a forwarding route", out[0].RouteType)
	}
}

// The authority's selected adapter and its downstream FIB value both retain the type.
func TestSysribCarriesRouteTypeFromLocRIB(t *testing.T) {
	fixture := newCandidateForwardingFixture(t, fixtureRouteOverride)
	prefix := netip.MustParsePrefix("192.0.2.1/32")
	fixture.rib.Insert(routeTypeFamily, prefix, locrib.Path{
		Source: redistevents.RegisterProtocol("bgp"), IsBGP: true, IsEBGP: true,
		NextHop: netip.MustParseAddr("198.51.100.1"), DistanceOverride: 20,
		HasDistanceOverride: true, RouteType: routetype.Blackhole,
	})
	selected := selectedFromAuthority(t, fixture.rib, prefix, false)
	value, ok := selected.(routeViewSelectedAdd)
	if !ok {
		t.Fatalf("authority returned %T, want selected add", selected)
	}
	if value.route.routeType != routetype.Blackhole {
		t.Errorf("RouteType = %v, want blackhole", value.route.routeType)
	}
	batches, err := fixture.applySelectedBatch([]routeViewSelectedChange{selected})
	if err != nil {
		t.Fatal(err)
	}
	out := batches[routeTypeFamily]
	if len(out) != 1 {
		t.Fatalf("got %d outgoing changes, want 1", len(out))
	}
	if out[0].RouteType != routetype.Blackhole {
		t.Errorf("RouteType = %v after selected forwarding, want blackhole", out[0].RouteType)
	}
}

// A prefix already installed as a forwarding route, then re-announced as a
// discard with EVERY other field unchanged. recomputeForwarding suppresses an
// emission when the winner matches the previous best. The route type must be
// part of that comparison, or the kernel keeps forwarding.
func TestSysribEmitsOnRouteTypeOnlyChange(t *testing.T) {
	s := newCandidateForwardingFixture(t, fixtureRouteOverride)
	pfx := netip.MustParsePrefix("192.0.2.1/32")
	nh := netip.MustParseAddr("198.51.100.1")

	base := func(rt routetype.Type) *incomingBatch {
		return &incomingBatch{
			Protocol: "bgp",
			Family:   routeTypeFamily,
			Changes: []incomingChange{{
				Action:    routeaction.Add,
				Prefix:    pfx,
				NextHop:   nh,
				Priority:  20,
				RouteType: rt,
			}},
		}
	}

	if _, out := s.offerCandidates(base(routetype.Unicast)); len(out) != 1 {
		t.Fatalf("first announce produced %d changes, want 1", len(out))
	}

	_, out := s.offerCandidates(base(routetype.Blackhole))
	if len(out) != 1 {
		t.Fatalf("route-type-only change produced %d changes, want 1: the prefix keeps forwarding", len(out))
	}
	if out[0].RouteType != routetype.Blackhole {
		t.Errorf("RouteType = %v, want blackhole", out[0].RouteType)
	}
	if out[0].Action != routeaction.Update {
		t.Errorf("Action = %v, want update", out[0].Action)
	}
}

// A full-table replay rebuilds every entry from stored state rather than from an
// incoming change, so it is a separate producer and drops the field separately.
func TestSysribReplayCarriesRouteType(t *testing.T) {
	s := newCandidateForwardingFixture(t, fixtureRouteOverride)
	pfx := netip.MustParsePrefix("192.0.2.1/32")
	if _, out := s.offerCandidates(&incomingBatch{
		Protocol: "bgp",
		Family:   routeTypeFamily,
		Changes: []incomingChange{{
			Action:    routeaction.Add,
			Prefix:    pfx,
			NextHop:   netip.MustParseAddr("198.51.100.1"),
			Priority:  20,
			RouteType: routetype.Blackhole,
		}},
	}); len(out) != 1 {
		t.Fatalf("announce produced %d changes, want 1", len(out))
	}

	best := s.best[prefixKey{family: routeTypeFamily, prefix: pfx}]
	if best == nil {
		t.Fatal("no stored best for the announced prefix")
	}
	if best.routeType != routetype.Blackhole {
		t.Errorf("stored routeType = %v, want blackhole: replay would emit a forwarding route", best.routeType)
	}
}

// The RIB plugin writes ribevents.BestChangeEntry. sysrib reads incomingChange.
// Building the batch from the PRODUCER's named type and feeding it to
// the candidate/authority handoff proves the two spell one field, not two that share a name.
func TestIncomingChangeRouteTypeIsRibeventsField(t *testing.T) {
	produced := []ribevents.BestChangeEntry{{
		Action:    routeaction.Add,
		Prefix:    netip.MustParsePrefix("192.0.2.1/32"),
		NextHop:   netip.MustParseAddr("198.51.100.1"),
		Priority:  20,
		RouteType: routetype.Blackhole,
	}}
	s := newCandidateForwardingFixture(t, fixtureRouteOverride)
	_, out := s.offerCandidates(&incomingBatch{
		Protocol: "bgp",
		Family:   routeTypeFamily,
		Changes:  produced,
	})
	if len(out) != 1 || out[0].RouteType != routetype.Blackhole {
		t.Errorf("producer-typed entry did not carry RouteType through processEvent: %+v", out)
	}
}

// `show rib` is the operator's view of what sysrib installed. Without the
// forwarding action on it, a prefix programmed as a discard is indistinguishable
// from one that forwards, and the operator has no way to confirm a blackhole
// took effect.
func TestShowRIBRendersRouteType(t *testing.T) {
	s := newCandidateForwardingFixture(t, fixtureRouteOverride)
	if _, out := s.offerCandidates(&incomingBatch{
		Protocol: "bgp",
		Family:   routeTypeFamily,
		Changes: []incomingChange{
			{
				Action:    routeaction.Add,
				Prefix:    netip.MustParsePrefix("192.0.2.1/32"),
				NextHop:   netip.MustParseAddr("198.51.100.1"),
				Priority:  20,
				RouteType: routetype.Blackhole,
			},
			{
				Action:   routeaction.Add,
				Prefix:   netip.MustParsePrefix("192.0.2.0/24"),
				NextHop:  netip.MustParseAddr("198.51.100.1"),
				Priority: 20,
			},
		},
	}); len(out) != 2 {
		t.Fatalf("announce produced %d changes, want 2", len(out))
	}

	data, err := s.showRIB()
	if err != nil {
		t.Fatalf("showRIB: %v", err)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	rendered := string(encoded)

	if !strings.Contains(rendered, `"route-type":"blackhole"`) {
		t.Errorf("the discard route renders no forwarding action: %s", rendered)
	}
	// The forwarding route MUST NOT carry the key. An operator scanning the
	// output for discards would otherwise have to read every value.
	if strings.Count(rendered, "route-type") != 1 {
		t.Errorf("route-type rendered for a forwarding route too: %s", rendered)
	}
}
