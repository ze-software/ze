// VALIDATES: an interface address makes its prefix a Loc-RIB path with source
// `connected`, no next-hop and the declared distance; removing the last address
// withdraws it; and a second address in the same prefix inserts nothing new.
// PREVENTS: `rib { distance { connected N } }` staying inert because the prefix
// was never a candidate at all, and a refcounted prefix inserting or withdrawing
// once per address rather than once per prefix.

package connected

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribdistance "github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	connectedevents "github.com/ze-software/ze/internal/plugins/connected/events"
)

// recordingSink stands in for the Loc-RIB, recording what the observer published
// so a test reads operations rather than final state. It is the shape a FORKED
// connected plugin holds in production (register wiring in connected.go).
type recordingSink struct {
	inserted []locrib.Path
	prefixes []netip.Prefix
	removed  []netip.Prefix
	flushes  int
}

func (s *recordingSink) InsertForward(_ family.Family, prefix netip.Prefix, p locrib.Path) {
	s.inserted = append(s.inserted, p)
	s.prefixes = append(s.prefixes, prefix)
}

func (s *recordingSink) Remove(_ family.Family, prefix netip.Prefix, _ redistevents.ProtocolID, _ uint32) {
	s.removed = append(s.removed, prefix)
}

func (s *recordingSink) Flush() { s.flushes++ }

// observerWithSink builds an observer publishing to a recording sink.
func observerWithSink(t *testing.T) (*routeObserver, *recordingSink) {
	t.Helper()
	sink := &recordingSink{}
	obs := newRouteObserver(&recordingBus{})
	obs.setLocRIB(nil, sink)
	return obs, sink
}

// declareConnectedDistance publishes d as the operator's
// `rib { distance { connected } }` for the duration of the test, the way sysrib's
// publishDistances does at configure time.
func declareConnectedDistance(t *testing.T, d uint8) {
	t.Helper()
	ribdistance.Set(func(protocol string) (uint8, bool) {
		if protocol == "connected" {
			return d, true
		}
		return 0, false
	})
	t.Cleanup(func() { ribdistance.Set(nil) })
}

func TestAddrAddedInsertsAConnectedPath(t *testing.T) {
	obs, sink := observerWithSink(t)

	obs.handleAddrAdded(makePayload("10.0.0.1", 24))

	if len(sink.inserted) != 1 {
		t.Fatalf("inserted %d paths, want 1", len(sink.inserted))
	}
	if got := sink.prefixes[0]; got != netip.MustParsePrefix("10.0.0.0/24") {
		t.Errorf("prefix = %s, want the network 10.0.0.0/24 rather than the address", got)
	}
	if sink.inserted[0].NextHop.IsValid() {
		t.Errorf("NextHop = %v, want none: a connected prefix is reached directly", sink.inserted[0].NextHop)
	}
	if sink.flushes == 0 {
		t.Error("the forked sink was never flushed, so the insert never left the process")
	}
}

func TestConnectedPathCarriesTheRegisteredSource(t *testing.T) {
	obs, sink := observerWithSink(t)
	obs.handleAddrAdded(makePayload("10.0.0.1", 24))

	if len(sink.inserted) != 1 {
		t.Fatalf("inserted %d paths, want 1", len(sink.inserted))
	}
	if sink.inserted[0].Source != connectedevents.ProtocolID {
		t.Errorf("Source = %d, want the registered connected protocol id %d",
			sink.inserted[0].Source, connectedevents.ProtocolID)
	}
	if got := redistevents.ProtocolName(sink.inserted[0].Source); got != "connected" {
		t.Errorf("the path's source resolves to %q, want \"connected\"", got)
	}
}

func TestAddrRemovedWithdrawsTheConnectedPath(t *testing.T) {
	obs, sink := observerWithSink(t)

	obs.handleAddrAdded(makePayload("10.0.0.1", 24))
	obs.handleAddrRemoved(makePayload("10.0.0.1", 24))

	if len(sink.removed) != 1 {
		t.Fatalf("withdrew %d prefixes, want 1", len(sink.removed))
	}
	if sink.removed[0] != netip.MustParsePrefix("10.0.0.0/24") {
		t.Errorf("withdrew %s, want 10.0.0.0/24", sink.removed[0])
	}
}

// TestConnectedRefcountInsertsOnce pins the refcount over the Loc-RIB half as
// well as the redistribute half: two addresses in one prefix are one prefix.
func TestConnectedRefcountInsertsOnce(t *testing.T) {
	obs, sink := observerWithSink(t)

	obs.handleAddrAdded(makePayload("10.0.0.1", 24))
	obs.handleAddrAdded(makePayload("10.0.0.2", 24))
	if len(sink.inserted) != 1 {
		t.Fatalf("inserted %d paths for two addresses in one prefix, want 1", len(sink.inserted))
	}

	obs.handleAddrRemoved(makePayload("10.0.0.1", 24))
	if len(sink.removed) != 0 {
		t.Fatalf("withdrew the prefix while an address still covers it: %v", sink.removed)
	}

	obs.handleAddrRemoved(makePayload("10.0.0.2", 24))
	if len(sink.removed) != 1 {
		t.Errorf("withdrew %d prefixes after the last address went, want 1", len(sink.removed))
	}
}

// TestConnectedStampsTheDeclaredDistance is AC-3 at the producer: the number the
// operator writes is the number selectBest ranks the connected prefix on, and it
// is read at insert so a reload takes effect on the next address event.
func TestConnectedStampsTheDeclaredDistance(t *testing.T) {
	declareConnectedDistance(t, 250)
	obs, sink := observerWithSink(t)

	obs.handleAddrAdded(makePayload("10.0.0.1", 24))

	if len(sink.inserted) != 1 {
		t.Fatalf("inserted %d paths, want 1", len(sink.inserted))
	}
	if sink.inserted[0].AdminDistance != 250 {
		t.Errorf("AdminDistance = %d, want the declared 250", sink.inserted[0].AdminDistance)
	}
}

// TestConnectedBootstrapDistanceIsZeroDeliberately pins the one fallback in this
// tree that IS zero. Zero is the best distance a route can hold, which is what a
// prefix on this box holds; every other producer's fallback is its own constant
// precisely so a zero nobody chose can never win a prefix.
func TestConnectedBootstrapDistanceIsZeroDeliberately(t *testing.T) {
	ribdistance.Set(nil)
	obs, sink := observerWithSink(t)

	obs.handleAddrAdded(makePayload("10.0.0.1", 24))

	if len(sink.inserted) != 1 {
		t.Fatalf("inserted %d paths, want 1", len(sink.inserted))
	}
	if sink.inserted[0].AdminDistance != DefaultAdminDistance {
		t.Errorf("AdminDistance = %d, want the bootstrap %d", sink.inserted[0].AdminDistance, DefaultAdminDistance)
	}
}

// TestConnectedWithNoLocRIBPublishesNothing is the in-process-only guard: with
// neither destination the observer must not panic, and the redistribute half must
// still work, because redistribution has never depended on the Loc-RIB.
func TestConnectedWithNoLocRIBPublishesNothing(t *testing.T) {
	bus := &recordingBus{}
	obs := newRouteObserver(bus)

	obs.handleAddrAdded(makePayload("10.0.0.1", 24))

	if len(bus.events()) != 1 {
		t.Errorf("redistribute emitted %d events, want 1: the bus does not depend on the Loc-RIB", len(bus.events()))
	}
}

// TestConnectedPrefixIsWhatTheResolverTerminatesOn is AC-4 and A-2 from the
// producer's end. The recursive next-hop resolver walks the Loc-RIB by longest
// prefix match and stops when it reaches a path with an INVALID next-hop, calling
// that a connected route in its own comment
// (internal/component/sysrib/nhresolver.go). It was written expecting connected
// routes in the Loc-RIB and had never seen one.
//
// The resolver's own half is TestRecursiveNHResolve_DirectlyConnected; this half
// proves the path the connected plugin publishes is the shape that terminates it,
// so a protocol next-hop covered only by an interface prefix now resolves.
func TestConnectedPrefixIsWhatTheResolverTerminatesOn(t *testing.T) {
	rib := locrib.NewRIB()
	obs := newRouteObserver(&recordingBus{})
	obs.setLocRIB(rib, nil)

	obs.handleAddrAdded(makePayload("10.0.0.1", 24))

	covered := netip.MustParseAddr("10.0.0.5")
	path, matched, found := rib.LPM(family.IPv4Unicast, covered)
	if !found {
		t.Fatalf("%s is covered by no Loc-RIB prefix, so a next-hop there resolves to nothing", covered)
	}
	if matched != netip.MustParsePrefix("10.0.0.0/24") {
		t.Errorf("longest match = %s, want 10.0.0.0/24", matched)
	}
	if path.NextHop.IsValid() {
		t.Errorf("NextHop = %v, want none: a valid one makes the resolver recurse instead of terminating", path.NextHop)
	}
}
