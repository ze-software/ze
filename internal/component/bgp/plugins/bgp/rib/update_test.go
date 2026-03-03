package rib

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/nlri"
)

// TestBuildGroupedUpdate_SingleRoute verifies UPDATE for single route.
//
// VALIDATES: Single route produces valid UPDATE with NLRI and attributes.
//
// PREVENTS: Empty or malformed UPDATE for valid route.
func TestBuildGroupedUpdate_SingleRoute(t *testing.T) {
	attrs := []attribute.Attribute{attribute.OriginIGP}
	routes := []*Route{
		testRouteWithAttrs("10.0.0.0/24", "1.2.3.4", attrs),
	}

	groups := GroupByAttributes(routes)
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}

	// ctx=nil means no ADD-PATH encoding
	update, err := BuildGroupedUpdate(&groups[0], false)
	if err != nil {
		t.Fatalf("BuildGroupedUpdate failed: %v", err)
	}

	// Verify NLRI is present
	if len(update.NLRI) == 0 {
		t.Error("NLRI is empty")
	}

	// Verify path attributes are present (at least ORIGIN + NEXT_HOP)
	if len(update.PathAttributes) == 0 {
		t.Error("PathAttributes is empty")
	}

	// Verify no withdrawals
	if len(update.WithdrawnRoutes) != 0 {
		t.Errorf("WithdrawnRoutes = %d bytes, want 0", len(update.WithdrawnRoutes))
	}
}

// TestBuildGroupedUpdate_MultipleRoutes verifies UPDATE with multiple NLRIs.
//
// VALIDATES: Multiple routes in group produce single UPDATE with multiple NLRIs.
//
// PREVENTS: Routes being split into multiple UPDATEs unnecessarily.
func TestBuildGroupedUpdate_MultipleRoutes(t *testing.T) {
	attrs := []attribute.Attribute{attribute.OriginIGP}
	routes := []*Route{
		testRouteWithAttrs("10.0.0.0/24", "1.2.3.4", attrs),
		testRouteWithAttrs("10.1.0.0/24", "1.2.3.4", attrs),
		testRouteWithAttrs("10.2.0.0/24", "1.2.3.4", attrs),
	}

	groups := GroupByAttributes(routes)
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}

	// ctx=nil means no ADD-PATH encoding
	update, err := BuildGroupedUpdate(&groups[0], false)
	if err != nil {
		t.Fatalf("BuildGroupedUpdate failed: %v", err)
	}

	// Count NLRIs in the packed bytes
	// Each IPv4 /24 is: 1 byte length (24) + 3 bytes prefix = 4 bytes
	expectedNLRILen := 3 * 4 // 3 routes * 4 bytes each
	if len(update.NLRI) != expectedNLRILen {
		t.Errorf("NLRI length = %d, want %d", len(update.NLRI), expectedNLRILen)
	}
}

// TestBuildGroupedUpdate_IncludesNextHop verifies NEXT_HOP attribute is added.
//
// VALIDATES: NEXT_HOP attribute is included in path attributes.
//
// PREVENTS: Missing mandatory NEXT_HOP attribute.
func TestBuildGroupedUpdate_IncludesNextHop(t *testing.T) {
	attrs := []attribute.Attribute{attribute.OriginIGP}
	routes := []*Route{
		testRouteWithAttrs("10.0.0.0/24", "1.2.3.4", attrs),
	}

	groups := GroupByAttributes(routes)
	// ctx=nil means no ADD-PATH encoding
	update, err := BuildGroupedUpdate(&groups[0], false)
	if err != nil {
		t.Fatalf("BuildGroupedUpdate failed: %v", err)
	}

	// Parse attributes to verify NEXT_HOP is present
	found := false
	data := update.PathAttributes
	for len(data) > 0 {
		flags, code, length, hdrLen, err := attribute.ParseHeader(data)
		if err != nil {
			t.Fatalf("ParseHeader failed: %v", err)
		}
		_ = flags

		if code == attribute.AttrNextHop {
			found = true
			// Verify next-hop value
			nhData := data[hdrLen : hdrLen+int(length)]
			nh, err := attribute.ParseNextHop(nhData)
			if err != nil {
				t.Fatalf("ParseNextHop failed: %v", err)
			}
			expected := netip.MustParseAddr("1.2.3.4")
			if nh.Addr != expected {
				t.Errorf("NextHop = %v, want %v", nh.Addr, expected)
			}
		}

		data = data[hdrLen+int(length):]
	}

	if !found {
		t.Error("NEXT_HOP attribute not found in PathAttributes")
	}
}

// TestBuildGroupedUpdate_IncludesOrigin verifies ORIGIN attribute is preserved.
//
// VALIDATES: ORIGIN attribute from route is included.
//
// PREVENTS: Missing mandatory ORIGIN attribute.
func TestBuildGroupedUpdate_IncludesOrigin(t *testing.T) {
	attrs := []attribute.Attribute{attribute.OriginEGP}
	routes := []*Route{
		testRouteWithAttrs("10.0.0.0/24", "1.2.3.4", attrs),
	}

	groups := GroupByAttributes(routes)
	// ctx=nil means no ADD-PATH encoding
	update, err := BuildGroupedUpdate(&groups[0], false)
	if err != nil {
		t.Fatalf("BuildGroupedUpdate failed: %v", err)
	}

	// Parse attributes to verify ORIGIN is present with correct value
	found := false
	data := update.PathAttributes
	for len(data) > 0 {
		_, code, length, hdrLen, err := attribute.ParseHeader(data)
		if err != nil {
			t.Fatalf("ParseHeader failed: %v", err)
		}

		if code == attribute.AttrOrigin {
			found = true
			originData := data[hdrLen : hdrLen+int(length)]
			origin, err := attribute.ParseOrigin(originData)
			if err != nil {
				t.Fatalf("ParseOrigin failed: %v", err)
			}
			if origin != attribute.OriginEGP {
				t.Errorf("Origin = %v, want EGP", origin)
			}
		}

		data = data[hdrLen+int(length):]
	}

	if !found {
		t.Error("ORIGIN attribute not found in PathAttributes")
	}
}

// TestBuildGroupedUpdate_EmptyGroup verifies empty group handling.
//
// VALIDATES: Empty group returns error or empty UPDATE.
//
// PREVENTS: Panic on empty input.
func TestBuildGroupedUpdate_EmptyGroup(t *testing.T) {
	group := &RouteGroup{
		Family: nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast},
	}

	// ctx=nil means no ADD-PATH encoding
	update, err := BuildGroupedUpdate(group, false)

	// Either error or empty UPDATE is acceptable
	if err == nil && len(update.NLRI) > 0 {
		t.Error("expected error or empty NLRI for empty group")
	}
}
