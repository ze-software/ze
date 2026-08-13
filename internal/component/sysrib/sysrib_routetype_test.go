// VALIDATES: the forwarding action reaches the FIB on BOTH rails out of the BGP
// RIB. The in-process Loc-RIB rail runs through changeToBatch. The
// cross-process event-bus rail runs through processEvent. It also survives a
// change that alters nothing else.
// PREVENTS: sysrib dropping the route type between the protocol RIB and the FIB
// backends, which leaves sysribevents.RouteTypeBlackhole a field only the test
// plugin can produce. Also prevents the same-best short circuit from
// suppressing a prefix that turned into a discard with an unchanged next-hop.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

var routeTypeFamily = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}

// The event-bus rail: an incoming best-change carrying a route type must leave
// sysrib carrying the same one.
func TestSysribCarriesRouteTypeFromEventBus(t *testing.T) {
	s := newSysRIB()
	_, out := s.processEvent(&incomingBatch{
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

// The Loc-RIB rail. changeToBatch is the only translation from a locrib.Change
// into the batch processEvent consumes. A field it drops is invisible to every
// in-process deployment.
func TestSysribCarriesRouteTypeFromLocRIB(t *testing.T) {
	batch := changeToBatch(locrib.Change{
		Family: routeTypeFamily,
		Prefix: netip.MustParsePrefix("192.0.2.1/32"),
		Kind:   locrib.ChangeAdd,
		Best: locrib.Path{
			Source:        redistevents.RegisterProtocol("bgp"),
			NextHop:       netip.MustParseAddr("198.51.100.1"),
			AdminDistance: 20,
			RouteType:     routetype.Blackhole,
		},
	})
	if batch == nil {
		t.Fatal("changeToBatch returned nil")
	}
	if got := batch.Changes[0].RouteType; got != routetype.Blackhole {
		t.Errorf("RouteType = %v, want blackhole", got)
	}

	s := newSysRIB()
	_, out := s.processEvent(batch)
	if len(out) != 1 {
		t.Fatalf("got %d outgoing changes, want 1", len(out))
	}
	if out[0].RouteType != routetype.Blackhole {
		t.Errorf("RouteType = %v after processEvent, want blackhole", out[0].RouteType)
	}
}

// A prefix already installed as a forwarding route, then re-announced as a
// discard with EVERY other field unchanged. recomputeBest suppresses an
// emission when the winner matches the previous best. The route type must be
// part of that comparison, or the kernel keeps forwarding.
func TestSysribEmitsOnRouteTypeOnlyChange(t *testing.T) {
	s := newSysRIB()
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

	if _, out := s.processEvent(base(routetype.Unicast)); len(out) != 1 {
		t.Fatalf("first announce produced %d changes, want 1", len(out))
	}

	_, out := s.processEvent(base(routetype.Blackhole))
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
	s := newSysRIB()
	pfx := netip.MustParsePrefix("192.0.2.1/32")
	if _, out := s.processEvent(&incomingBatch{
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
// processEvent proves the two spell one field, not two that share a name.
func TestIncomingChangeRouteTypeIsRibeventsField(t *testing.T) {
	produced := []ribevents.BestChangeEntry{{
		Action:    routeaction.Add,
		Prefix:    netip.MustParsePrefix("192.0.2.1/32"),
		NextHop:   netip.MustParseAddr("198.51.100.1"),
		Priority:  20,
		RouteType: routetype.Blackhole,
	}}
	s := newSysRIB()
	_, out := s.processEvent(&incomingBatch{
		Protocol: "bgp",
		Family:   routeTypeFamily,
		Changes:  produced,
	})
	if len(out) != 1 || out[0].RouteType != routetype.Blackhole {
		t.Errorf("producer-typed entry did not carry RouteType through processEvent: %+v", out)
	}
}
