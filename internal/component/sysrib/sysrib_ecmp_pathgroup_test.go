// Design: docs/architecture/isis/isis-9-spf-rib.md -- authority-selected ECMP.
// VALIDATES: the authority retains explicit producer members and emits every target.
// PREVENTS: ECMP collapsing to one target, or inventing a sibling for a single path.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/rib/nexthop"

	"github.com/ze-software/ze/internal/core/family"
)

// candidateBatch holds one typed producer candidate. It carries no selection marker.
func candidateBatch(proto string, fam family.Family, c *incomingChange) *incomingBatch {
	return makePayload(proto, fam, []incomingChange{*c})
}

// TestSysribECMPPathGroup offers a producer's explicit ECMP candidate to the
// real authority. Its selected value retains both next-hops through common
// forwarding: one primary and one member in ECMPPaths.
func TestSysribECMPPathGroup(t *testing.T) {
	bus := newTestEventBus()
	useForwardingBus(t, bus)
	s := newCandidateForwardingFixture(t, fixtureRouteOverride)

	pfx := netip.MustParsePrefix("10.50.0.0/24")
	nh1 := netip.MustParseAddr("10.0.0.1")
	nh2 := netip.MustParseAddr("10.0.0.2")

	// The producer's candidate names nh1 and its explicit equal-cost target nh2.
	// Authority owns the group handed to common forwarding.
	_, changes := s.offerCandidates(candidateBatch("isis", family.IPv4Unicast, &incomingChange{
		Action:       routeaction.Add,
		Prefix:       pfx,
		NextHop:      nh1,
		Priority:     115,
		Metric:       20,
		ECMPNextHops: []nexthop.NextHop{{Addr: nh2}},
	}))

	if len(changes) != 1 {
		t.Fatalf("expected 1 best-change for the ECMP prefix, got %d", len(changes))
	}
	c := changes[0]
	if len(c.ECMPPaths) != 1 {
		t.Fatalf("ECMPPaths = %v, want exactly 1 sibling next-hop", c.ECMPPaths)
	}
	got := map[netip.Addr]bool{c.NextHop: true}
	got[c.ECMPPaths[0].NextHop] = true
	if !got[nh1] || !got[nh2] {
		t.Errorf("multipath next-hops = {primary %s, ecmp %s}, want {%s, %s}",
			c.NextHop, c.ECMPPaths[0].NextHop, nh1, nh2)
	}
}

// TestSysribSinglePathNoECMP offers one candidate with no explicit siblings.
// Its authoritative forwarding value must keep ECMPPaths empty.
func TestSysribSinglePathNoECMP(t *testing.T) {
	bus := newTestEventBus()
	useForwardingBus(t, bus)
	s := newCandidateForwardingFixture(t, fixtureRouteOverride)

	pfx := netip.MustParsePrefix("10.51.0.0/24")
	_, changes := s.offerCandidates(candidateBatch("bgp", family.IPv4Unicast, &incomingChange{
		Action:   routeaction.Add,
		Prefix:   pfx,
		NextHop:  netip.MustParseAddr("192.0.2.1"),
		Priority: 20,
		Metric:   0,
		// ECMPNextHops nil: a single-Path Loc-RIB group carries no siblings.
	}))

	if len(changes) != 1 {
		t.Fatalf("expected 1 best-change for single-path prefix, got %d", len(changes))
	}
	if len(changes[0].ECMPPaths) != 0 {
		t.Errorf("single-Path prefix got ECMPPaths %v, want none", changes[0].ECMPPaths)
	}
}
