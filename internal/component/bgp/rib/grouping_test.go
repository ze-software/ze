package rib

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// Helper to create test routes with attributes.
func testRouteWithAttrs(prefix, nextHop string, attrs []attribute.Attribute) *Route {
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
	for i := range 10 {
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

// ==============================================================
// Two-Level Grouping Tests (AttributeGroup + ASPathGroup)
// ==============================================================

// Helper to create test routes with AS_PATH.
func testRouteWithASPath(prefix, nextHop string, attrs []attribute.Attribute, asPath *attribute.ASPath) *Route {
	p := netip.MustParsePrefix(prefix)
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	n := nlri.NewINET(family, p, 0)
	return NewRouteWithASPath(n, netip.MustParseAddr(nextHop), attrs, asPath)
}

// TestHashASPathString_NilASPath verifies nil AS_PATH hashes to empty string.
//
// VALIDATES: Nil AS_PATH produces consistent empty key for grouping.
//
// PREVENTS: Panic on nil AS_PATH or inconsistent grouping.
func TestHashASPathString_NilASPath(t *testing.T) {
	result := hashASPathString(nil)
	if result != "" {
		t.Errorf("hashASPathString(nil) = %q, want empty string", result)
	}
}

// TestHashASPathString_EmptyASPath verifies empty AS_PATH hashes consistently.
//
// VALIDATES: Empty AS_PATH (no segments) produces consistent key.
//
// PREVENTS: Empty and nil AS_PATH being treated differently when they should be same.
func TestHashASPathString_EmptyASPath(t *testing.T) {
	asPath := &attribute.ASPath{Segments: nil}
	result := hashASPathString(asPath)
	// Empty segments should produce empty string (same as nil)
	if result != "" {
		t.Errorf("hashASPathString(empty) = %q, want empty string", result)
	}
}

// TestHashASPathString_SingleSegment verifies AS_PATH with one segment hashes correctly.
//
// VALIDATES: AS_PATH with segments produces non-empty, consistent key.
//
// PREVENTS: Different AS_PATHs hashing to same key.
func TestHashASPathString_SingleSegment(t *testing.T) {
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}
	result := hashASPathString(asPath)
	if result == "" {
		t.Error("hashASPathString returned empty string for non-empty AS_PATH")
	}

	// Same AS_PATH should produce same hash
	asPath2 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}
	result2 := hashASPathString(asPath2)
	if result != result2 {
		t.Errorf("same AS_PATH produced different hashes: %q vs %q", result, result2)
	}
}

// TestHashASPathString_DifferentASPaths verifies different AS_PATHs hash differently.
//
// VALIDATES: Different AS_PATHs produce different keys.
//
// PREVENTS: Routes with different AS_PATHs being incorrectly grouped.
func TestHashASPathString_DifferentASPaths(t *testing.T) {
	asPath1 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}
	asPath2 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}

	hash1 := hashASPathString(asPath1)
	hash2 := hashASPathString(asPath2)

	if hash1 == hash2 {
		t.Error("different AS_PATHs produced same hash")
	}
}

// TestGroupByAttributesTwoLevel_SameAttrsSameASPath verifies routes group together.
//
// VALIDATES: Routes with same attrs and same AS_PATH → 1 AttributeGroup, 1 ASPathGroup.
//
// PREVENTS: Unnecessary UPDATE fragmentation.
func TestGroupByAttributesTwoLevel_SameAttrsSameASPath(t *testing.T) {
	attrs := []attribute.Attribute{attribute.OriginIGP}
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}

	routes := []*Route{
		testRouteWithASPath("10.0.0.0/24", "1.2.3.4", attrs, asPath),
		testRouteWithASPath("10.1.0.0/24", "1.2.3.4", attrs, asPath),
	}

	groups := GroupByAttributesTwoLevel(routes)

	if len(groups) != 1 {
		t.Errorf("got %d AttributeGroups, want 1", len(groups))
	}
	if len(groups[0].ByASPath) != 1 {
		t.Errorf("got %d ASPathGroups, want 1", len(groups[0].ByASPath))
	}
	if len(groups[0].ByASPath[0].Routes) != 2 {
		t.Errorf("got %d routes in ASPathGroup, want 2", len(groups[0].ByASPath[0].Routes))
	}
}

// TestGroupByAttributesTwoLevel_SameAttrsDiffASPath verifies AS_PATH separation.
//
// VALIDATES: Routes with same attrs but different AS_PATH → 1 AttributeGroup, N ASPathGroups.
//
// PREVENTS: Routes with different AS_PATHs sharing same UPDATE (RFC violation).
func TestGroupByAttributesTwoLevel_SameAttrsDiffASPath(t *testing.T) {
	attrs := []attribute.Attribute{attribute.OriginIGP}
	asPath1 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}
	asPath2 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}

	routes := []*Route{
		testRouteWithASPath("10.0.0.0/24", "1.2.3.4", attrs, asPath1),
		testRouteWithASPath("10.1.0.0/24", "1.2.3.4", attrs, asPath2),
	}

	groups := GroupByAttributesTwoLevel(routes)

	if len(groups) != 1 {
		t.Errorf("got %d AttributeGroups, want 1", len(groups))
	}
	if len(groups[0].ByASPath) != 2 {
		t.Errorf("got %d ASPathGroups, want 2", len(groups[0].ByASPath))
	}
}

// TestGroupByAttributesTwoLevel_DiffAttrs verifies attribute separation.
//
// VALIDATES: Routes with different attrs → separate AttributeGroups.
//
// PREVENTS: Routes with different attributes sharing same UPDATE.
func TestGroupByAttributesTwoLevel_DiffAttrs(t *testing.T) {
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}

	routes := []*Route{
		testRouteWithASPath("10.0.0.0/24", "1.2.3.4", []attribute.Attribute{attribute.OriginIGP}, asPath),
		testRouteWithASPath("10.1.0.0/24", "5.6.7.8", []attribute.Attribute{attribute.OriginIGP}, asPath), // Different NH
	}

	groups := GroupByAttributesTwoLevel(routes)

	if len(groups) != 2 {
		t.Errorf("got %d AttributeGroups, want 2 (different next-hops)", len(groups))
	}
}

// TestGroupByAttributesTwoLevel_NilASPath verifies nil AS_PATH handling.
//
// VALIDATES: Routes with nil AS_PATH are grouped together.
//
// PREVENTS: Panic or incorrect grouping for locally originated routes.
func TestGroupByAttributesTwoLevel_NilASPath(t *testing.T) {
	attrs := []attribute.Attribute{attribute.OriginIGP}

	routes := []*Route{
		testRouteWithASPath("10.0.0.0/24", "1.2.3.4", attrs, nil),
		testRouteWithASPath("10.1.0.0/24", "1.2.3.4", attrs, nil),
	}

	groups := GroupByAttributesTwoLevel(routes)

	if len(groups) != 1 {
		t.Errorf("got %d AttributeGroups, want 1", len(groups))
	}
	if len(groups[0].ByASPath) != 1 {
		t.Errorf("got %d ASPathGroups, want 1", len(groups[0].ByASPath))
	}
	if groups[0].ByASPath[0].ASPath != nil {
		t.Error("ASPathGroup.ASPath should be nil")
	}
}

// TestGroupByAttributesTwoLevel_MixedNilASPath verifies nil and non-nil separation.
//
// VALIDATES: Routes with nil AS_PATH separate from routes with AS_PATH.
//
// PREVENTS: Locally originated routes mixed with learned routes.
func TestGroupByAttributesTwoLevel_MixedNilASPath(t *testing.T) {
	attrs := []attribute.Attribute{attribute.OriginIGP}
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}

	routes := []*Route{
		testRouteWithASPath("10.0.0.0/24", "1.2.3.4", attrs, nil),
		testRouteWithASPath("10.1.0.0/24", "1.2.3.4", attrs, asPath),
	}

	groups := GroupByAttributesTwoLevel(routes)

	if len(groups) != 1 {
		t.Errorf("got %d AttributeGroups, want 1", len(groups))
	}
	if len(groups[0].ByASPath) != 2 {
		t.Errorf("got %d ASPathGroups, want 2 (nil and non-nil AS_PATH)", len(groups[0].ByASPath))
	}
}

// TestGroupByAttributesTwoLevel_Empty verifies empty input handling.
//
// VALIDATES: Empty input returns empty output.
//
// PREVENTS: Panic on empty input.
func TestGroupByAttributesTwoLevel_Empty(t *testing.T) {
	groups := GroupByAttributesTwoLevel(nil)
	if len(groups) != 0 {
		t.Errorf("got %d groups for nil, want 0", len(groups))
	}

	groups = GroupByAttributesTwoLevel([]*Route{})
	if len(groups) != 0 {
		t.Errorf("got %d groups for empty slice, want 0", len(groups))
	}
}

// TestGroupByAttributesTwoLevel_Deterministic verifies consistent ordering.
//
// VALIDATES: Same input produces same order across multiple runs.
//
// PREVENTS: Non-deterministic UPDATE generation.
func TestGroupByAttributesTwoLevel_Deterministic(t *testing.T) {
	attrs := []attribute.Attribute{attribute.OriginIGP}
	asPath1 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}
	asPath2 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65002}},
		},
	}

	routes := []*Route{
		testRouteWithASPath("10.0.0.0/24", "1.2.3.4", attrs, asPath1),
		testRouteWithASPath("10.1.0.0/24", "1.2.3.4", attrs, asPath2),
		testRouteWithASPath("10.2.0.0/24", "5.6.7.8", attrs, asPath1),
	}

	// Capture first run's structure
	var firstAttrKeys []string
	var firstASPathCounts []int

	for i := range 10 {
		groups := GroupByAttributesTwoLevel(routes)

		attrKeys := make([]string, len(groups))
		aspCounts := make([]int, len(groups))
		for j, g := range groups {
			attrKeys[j] = g.Key
			aspCounts[j] = len(g.ByASPath)
		}

		if i == 0 {
			firstAttrKeys = attrKeys
			firstASPathCounts = aspCounts
		} else {
			for j := range attrKeys {
				if attrKeys[j] != firstAttrKeys[j] {
					t.Errorf("iteration %d: AttributeGroup key changed at index %d", i, j)
				}
				if aspCounts[j] != firstASPathCounts[j] {
					t.Errorf("iteration %d: ASPathGroup count changed at index %d", i, j)
				}
			}
		}
	}
}

// ==============================================================
// Bug Fix Tests
// ==============================================================

// TestGroupByAttributesTwoLevel_ASPathInAttrsVsField verifies routes group correctly
// when AS_PATH is in attrs slice vs asPath field.
//
// VALIDATES: Routes with AS_PATH in different locations but same effective attributes
//
//	are grouped in the same AttributeGroup.
//
// PREVENTS: Bug where AS_PATH in attrs causes different level-1 keys.
func TestGroupByAttributesTwoLevel_ASPathInAttrsVsField(t *testing.T) {
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}

	// Route A: AS_PATH in asPath field only
	attrsNoASPath := []attribute.Attribute{attribute.OriginIGP}
	routeA := testRouteWithASPath("10.0.0.0/24", "1.2.3.4", attrsNoASPath, asPath)

	// Route B: AS_PATH in attrs slice only (legacy path)
	attrsWithASPath := []attribute.Attribute{attribute.OriginIGP, asPath}
	routeB := testRouteWithASPath("10.1.0.0/24", "1.2.3.4", attrsWithASPath, nil)

	routes := []*Route{routeA, routeB}
	groups := GroupByAttributesTwoLevel(routes)

	// Both routes should be in the SAME AttributeGroup (same non-AS_PATH attrs)
	if len(groups) != 1 {
		t.Errorf("got %d AttributeGroups, want 1 (same non-AS_PATH attrs)", len(groups))
	}

	// Both routes have the same effective AS_PATH, so 1 ASPathGroup
	if len(groups) > 0 && len(groups[0].ByASPath) != 1 {
		t.Errorf("got %d ASPathGroups, want 1 (same AS_PATH)", len(groups[0].ByASPath))
	}
}

// TestGroupByAttributesTwoLevel_DifferentASPathObjects verifies routes with different
// ASPath pointers but same content are grouped together.
//
// VALIDATES: Hash-based grouping works on content, not pointer identity.
//
// PREVENTS: Routes with identical AS_PATHs being split into separate groups.
func TestGroupByAttributesTwoLevel_DifferentASPathObjects(t *testing.T) {
	attrs := []attribute.Attribute{attribute.OriginIGP}

	// Two different ASPath objects with identical content
	asPath1 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}
	asPath2 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}

	// Verify they are different pointers
	if asPath1 == asPath2 {
		t.Skip("test requires different pointers")
	}

	routes := []*Route{
		testRouteWithASPath("10.0.0.0/24", "1.2.3.4", attrs, asPath1),
		testRouteWithASPath("10.1.0.0/24", "1.2.3.4", attrs, asPath2),
	}

	groups := GroupByAttributesTwoLevel(routes)

	if len(groups) != 1 {
		t.Errorf("got %d AttributeGroups, want 1", len(groups))
	}
	if len(groups) > 0 && len(groups[0].ByASPath) != 1 {
		t.Errorf("got %d ASPathGroups, want 1 (same AS_PATH content)", len(groups[0].ByASPath))
	}
	if len(groups) > 0 && len(groups[0].ByASPath) > 0 && len(groups[0].ByASPath[0].Routes) != 2 {
		t.Errorf("got %d routes in ASPathGroup, want 2", len(groups[0].ByASPath[0].Routes))
	}
}

// TestGroupByAttributesTwoLevel_ASPathInBothLocations verifies asPath field takes precedence
// when AS_PATH is in both attrs AND asPath field.
//
// VALIDATES: route.ASPath() takes precedence over attrs.
//
// PREVENTS: Incorrect grouping when AS_PATH is duplicated with different values.
func TestGroupByAttributesTwoLevel_ASPathInBothLocations(t *testing.T) {
	// AS_PATH in attrs
	attrsASPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}
	// Different AS_PATH in asPath field
	fieldASPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65002, 65003}},
		},
	}

	attrs := []attribute.Attribute{attribute.OriginIGP, attrsASPath}
	routes := []*Route{
		testRouteWithASPath("10.0.0.0/24", "1.2.3.4", attrs, fieldASPath),
	}

	groups := GroupByAttributesTwoLevel(routes)

	if len(groups) != 1 {
		t.Fatalf("got %d AttributeGroups, want 1", len(groups))
	}
	if len(groups[0].ByASPath) != 1 {
		t.Fatalf("got %d ASPathGroups, want 1", len(groups[0].ByASPath))
	}

	// The ASPathGroup should use fieldASPath (precedence), not attrsASPath
	aspGroup := groups[0].ByASPath[0]
	if aspGroup.ASPath == nil {
		t.Fatal("ASPathGroup.ASPath is nil, expected fieldASPath")
	}
	if len(aspGroup.ASPath.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(aspGroup.ASPath.Segments))
	}
	if len(aspGroup.ASPath.Segments[0].ASNs) != 2 {
		t.Errorf("expected 2 ASNs (from field), got %d", len(aspGroup.ASPath.Segments[0].ASNs))
	}
	if aspGroup.ASPath.Segments[0].ASNs[0] != 65002 {
		t.Errorf("expected first ASN 65002, got %d", aspGroup.ASPath.Segments[0].ASNs[0])
	}
}
