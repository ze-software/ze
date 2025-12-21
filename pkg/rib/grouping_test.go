package rib

import (
	"net/netip"
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

// Helper to create test routes with attributes.
func testRouteWithAttrs(prefix string, nextHop string, attrs []attribute.Attribute) *Route {
	p := netip.MustParsePrefix(prefix)
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	n := nlri.NewINET(family, p, 0)
	return NewRoute(n, netip.MustParseAddr(nextHop), attrs)
}

// TestGroupByAttributes_SingleGroup verifies routes with same attributes group together.
//
// VALIDATES: Routes with identical attributes form a single group.
//
// PREVENTS: Unnecessary UPDATE message fragmentation.
func TestGroupByAttributes_SingleGroup(t *testing.T) {
	// All routes have same next-hop and origin
	attrs := []attribute.Attribute{
		attribute.OriginIGP,
	}

	routes := []*Route{
		testRouteWithAttrs("10.0.0.0/24", "1.2.3.4", attrs),
		testRouteWithAttrs("10.1.0.0/24", "1.2.3.4", attrs),
		testRouteWithAttrs("10.2.0.0/24", "1.2.3.4", attrs),
	}

	groups := GroupByAttributes(routes)

	if len(groups) != 1 {
		t.Errorf("got %d groups, want 1", len(groups))
	}

	if len(groups[0].Routes) != 3 {
		t.Errorf("group has %d routes, want 3", len(groups[0].Routes))
	}
}

// TestGroupByAttributes_MultipleGroups verifies routes with different attributes separate.
//
// VALIDATES: Routes with different next-hops form separate groups.
//
// PREVENTS: Incorrect attribute assignment in UPDATE messages.
func TestGroupByAttributes_MultipleGroups(t *testing.T) {
	attrsA := []attribute.Attribute{
		attribute.OriginIGP,
	}
	attrsB := []attribute.Attribute{
		attribute.OriginEGP,
	}

	routes := []*Route{
		testRouteWithAttrs("10.0.0.0/24", "1.2.3.4", attrsA),
		testRouteWithAttrs("10.1.0.0/24", "1.2.3.4", attrsA),
		testRouteWithAttrs("10.2.0.0/24", "5.6.7.8", attrsB), // Different next-hop and origin
	}

	groups := GroupByAttributes(routes)

	if len(groups) != 2 {
		t.Errorf("got %d groups, want 2", len(groups))
	}

	// Find group sizes
	sizes := make(map[int]int)
	for _, g := range groups {
		sizes[len(g.Routes)]++
	}

	if sizes[2] != 1 || sizes[1] != 1 {
		t.Errorf("expected one group of 2 and one group of 1, got sizes: %v", sizes)
	}
}

// TestGroupByAttributes_Empty verifies empty input returns empty output.
//
// VALIDATES: Empty route slice returns empty group slice.
//
// PREVENTS: Panic or error on empty input.
func TestGroupByAttributes_Empty(t *testing.T) {
	groups := GroupByAttributes(nil)

	if len(groups) != 0 {
		t.Errorf("got %d groups, want 0", len(groups))
	}

	groups = GroupByAttributes([]*Route{})

	if len(groups) != 0 {
		t.Errorf("got %d groups for empty slice, want 0", len(groups))
	}
}

// TestGroupByAttributes_DifferentNextHop verifies next-hop difference causes separation.
//
// VALIDATES: Different next-hops cause route separation.
//
// PREVENTS: Routes with different next-hops being grouped together.
func TestGroupByAttributes_DifferentNextHop(t *testing.T) {
	attrs := []attribute.Attribute{
		attribute.OriginIGP,
	}

	routes := []*Route{
		testRouteWithAttrs("10.0.0.0/24", "1.2.3.4", attrs),
		testRouteWithAttrs("10.1.0.0/24", "5.6.7.8", attrs), // Same attrs, different NH
	}

	groups := GroupByAttributes(routes)

	if len(groups) != 2 {
		t.Errorf("got %d groups, want 2 (different next-hops)", len(groups))
	}
}

// TestGroupByAttributes_DeterministicOrder verifies consistent ordering.
//
// VALIDATES: Groups are returned in deterministic order.
//
// PREVENTS: Non-deterministic UPDATE generation.
func TestGroupByAttributes_DeterministicOrder(t *testing.T) {
	attrsA := []attribute.Attribute{attribute.OriginIGP}
	attrsB := []attribute.Attribute{attribute.OriginEGP}

	routes := []*Route{
		testRouteWithAttrs("10.0.0.0/24", "1.2.3.4", attrsA),
		testRouteWithAttrs("10.1.0.0/24", "5.6.7.8", attrsB),
	}

	// Run multiple times to verify determinism
	var firstOrder []string
	for i := 0; i < 10; i++ {
		groups := GroupByAttributes(routes)
		order := make([]string, len(groups))
		for j, g := range groups {
			order[j] = g.Key
		}
		if i == 0 {
			firstOrder = order
		} else {
			for j, k := range order {
				if k != firstOrder[j] {
					t.Errorf("iteration %d: order changed at index %d", i, j)
				}
			}
		}
	}
}

// TestGroupByAttributes_PreservesFamily verifies family is preserved in groups.
//
// VALIDATES: RouteGroup includes correct address family.
//
// PREVENTS: Wrong AFI/SAFI in generated UPDATEs.
func TestGroupByAttributes_PreservesFamily(t *testing.T) {
	attrs := []attribute.Attribute{attribute.OriginIGP}
	routes := []*Route{
		testRouteWithAttrs("10.0.0.0/24", "1.2.3.4", attrs),
	}

	groups := GroupByAttributes(routes)

	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}

	expectedFamily := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	if groups[0].Family != expectedFamily {
		t.Errorf("family = %v, want %v", groups[0].Family, expectedFamily)
	}
}

// TestRouteGroup_NLRIs verifies NLRI extraction from group.
//
// VALIDATES: NLRIs() returns all NLRIs in the group.
//
// PREVENTS: Missing NLRIs in UPDATE message.
func TestRouteGroup_NLRIs(t *testing.T) {
	attrs := []attribute.Attribute{attribute.OriginIGP}
	routes := []*Route{
		testRouteWithAttrs("10.0.0.0/24", "1.2.3.4", attrs),
		testRouteWithAttrs("10.1.0.0/24", "1.2.3.4", attrs),
	}

	groups := GroupByAttributes(routes)

	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}

	nlris := groups[0].NLRIs()
	if len(nlris) != 2 {
		t.Errorf("got %d NLRIs, want 2", len(nlris))
	}
}
