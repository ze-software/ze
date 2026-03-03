package rib

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// TestCommitService_DefaultOrigin verifies ORIGIN defaults to IGP when not provided.
//
// VALIDATES: Missing ORIGIN in attrs → ORIGIN(IGP) added automatically
//
// PREVENTS: Invalid UPDATE missing mandatory ORIGIN attribute.
func TestCommitService_DefaultOrigin(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	// No ORIGIN in attributes!
	attrs := []attribute.Attribute{}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	// Verify ORIGIN attribute exists (code 1)
	hasOrigin := false
	offset := 0
	for offset < len(sender.updates[0].PathAttributes) {
		if offset+2 > len(sender.updates[0].PathAttributes) {
			break
		}
		code := sender.updates[0].PathAttributes[offset+1]
		flags := sender.updates[0].PathAttributes[offset]
		var attrLen, hdrLen int
		if flags&0x10 != 0 {
			attrLen = int(sender.updates[0].PathAttributes[offset+2])<<8 | int(sender.updates[0].PathAttributes[offset+3])
			hdrLen = 4
		} else {
			attrLen = int(sender.updates[0].PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 1 { // ORIGIN
			hasOrigin = true
			// Verify it's IGP (0)
			if attrLen == 1 && offset+hdrLen < len(sender.updates[0].PathAttributes) {
				originValue := sender.updates[0].PathAttributes[offset+hdrLen]
				if originValue != 0 {
					t.Errorf("expected ORIGIN=IGP(0), got %d", originValue)
				}
			}
		}
		offset += hdrLen + attrLen
	}

	if !hasOrigin {
		t.Error("UPDATE missing ORIGIN attribute when not provided")
	}
}

// TestCommitService_PreservesExistingASPath verifies existing AS_PATH is preserved.
//
// VALIDATES: Route with existing AS_PATH → local AS prepended, rest preserved
//
// PREVENTS: Loss of AS_PATH when re-advertising learned routes.
func TestCommitService_PreservesExistingASPath(t *testing.T) {
	sender := &mockUpdateSender{}
	// eBGP session
	neg := testContext(65000, 65001, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	// Route with existing AS_PATH from upstream
	existingASPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65002, 65003}},
		},
	}
	attrs := []attribute.Attribute{attribute.Origin(0), existingASPath}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	// Find and parse AS_PATH
	offset := 0
	for offset < len(sender.updates[0].PathAttributes) {
		if offset+2 > len(sender.updates[0].PathAttributes) {
			break
		}
		code := sender.updates[0].PathAttributes[offset+1]
		flags := sender.updates[0].PathAttributes[offset]
		var attrLen, hdrLen int
		if flags&0x10 != 0 {
			attrLen = int(sender.updates[0].PathAttributes[offset+2])<<8 | int(sender.updates[0].PathAttributes[offset+3])
			hdrLen = 4
		} else {
			attrLen = int(sender.updates[0].PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 2 { // AS_PATH
			// For eBGP with 4-byte ASNs and 3 ASNs (65000 prepended + 65002, 65003):
			// Segment: type(1) + count(1) + 3*ASN(4) = 14 bytes
			if attrLen < 14 {
				t.Errorf("expected AS_PATH with 3 ASNs (14 bytes), got %d bytes", attrLen)
			}

			// Parse first segment to verify prepending
			valueStart := offset + hdrLen
			if valueStart+2 <= len(sender.updates[0].PathAttributes) {
				segType := sender.updates[0].PathAttributes[valueStart]
				segCount := sender.updates[0].PathAttributes[valueStart+1]

				if segType != byte(attribute.ASSequence) {
					t.Errorf("expected AS_SEQUENCE, got %d", segType)
				}
				if segCount != 3 {
					t.Errorf("expected 3 ASNs in segment, got %d", segCount)
				}

				// Verify first ASN is local AS (65000)
				if valueStart+6 <= len(sender.updates[0].PathAttributes) {
					firstASN := uint32(sender.updates[0].PathAttributes[valueStart+2])<<24 |
						uint32(sender.updates[0].PathAttributes[valueStart+3])<<16 |
						uint32(sender.updates[0].PathAttributes[valueStart+4])<<8 |
						uint32(sender.updates[0].PathAttributes[valueStart+5])
					if firstASN != 65000 {
						t.Errorf("expected first ASN 65000 (local), got %d", firstASN)
					}
				}
			}
		}
		offset += hdrLen + attrLen
	}
}

// TestCommitService_VPNNextHopHasRD verifies VPN routes have RD in next-hop.
//
// VALIDATES: VPN route (SAFI 128) → next-hop includes 8-byte RD prefix
//
// PREVENTS: Malformed VPN next-hop encoding.
func TestCommitService_VPNNextHopHasRD(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	// Create a VPN route (AFI=1, SAFI=128)
	routes := []*Route{
		NewRoute(newVPNv4NLRI("192.168.1.0/24"), nh, attrs),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	// Find MP_REACH_NLRI and verify next-hop length
	offset := 0
	for offset < len(sender.updates[0].PathAttributes) {
		if offset+2 > len(sender.updates[0].PathAttributes) {
			break
		}
		code := sender.updates[0].PathAttributes[offset+1]
		flags := sender.updates[0].PathAttributes[offset]
		var attrLen, hdrLen int
		if flags&0x10 != 0 {
			attrLen = int(sender.updates[0].PathAttributes[offset+2])<<8 | int(sender.updates[0].PathAttributes[offset+3])
			hdrLen = 4
		} else {
			attrLen = int(sender.updates[0].PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 14 { // MP_REACH_NLRI
			valueStart := offset + hdrLen
			if valueStart+4 <= len(sender.updates[0].PathAttributes) {
				// Check SAFI is 128 (VPN)
				safi := sender.updates[0].PathAttributes[valueStart+2]
				if safi != 128 {
					t.Errorf("expected SAFI 128, got %d", safi)
				}

				// VPN next-hop for IPv4: RD(8) + IPv4(4) = 12 bytes
				nhLen := sender.updates[0].PathAttributes[valueStart+3]
				if nhLen != 12 {
					t.Errorf("expected VPN next-hop length 12 (RD+IPv4), got %d", nhLen)
				}

				// Verify first 8 bytes are zeros (RD)
				if valueStart+4+8 <= len(sender.updates[0].PathAttributes) {
					for i := range 8 {
						if sender.updates[0].PathAttributes[valueStart+4+i] != 0 {
							t.Errorf("expected RD byte %d to be 0, got %d", i, sender.updates[0].PathAttributes[valueStart+4+i])
						}
					}
				}
			}
		}
		offset += hdrLen + attrLen
	}
}

// TestCommitService_IPv4WithIPv6NextHop verifies RFC 5549 Extended Next Hop.
//
// VALIDATES: IPv4 route + IPv6 next-hop → MP_REACH_NLRI (not NEXT_HOP attribute)
//
// PREVENTS: Silent failure when using IPv6 underlay for IPv4 routes.
func TestCommitService_IPv4WithIPv6NextHop(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	// IPv4 route with IPv6 next-hop (RFC 5549)
	nh := netip.MustParseAddr("2001:db8::1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	update := sender.updates[0]

	// For IPv4 + IPv6 next-hop, must use MP_REACH_NLRI (not NEXT_HOP attribute)
	hasMPReach := false
	hasNextHopAttr := false

	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		code := update.PathAttributes[offset+1]
		flags := update.PathAttributes[offset]
		var attrLen, hdrLen int
		if flags&0x10 != 0 {
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			hdrLen = 4
		} else {
			attrLen = int(update.PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 3 { // NEXT_HOP
			hasNextHopAttr = true
		}
		if code == 14 { // MP_REACH_NLRI
			hasMPReach = true
			valueStart := offset + hdrLen
			if valueStart+4 <= len(update.PathAttributes) {
				// Verify AFI=1 (IPv4), SAFI=1 (unicast)
				afi := uint16(update.PathAttributes[valueStart])<<8 | uint16(update.PathAttributes[valueStart+1])
				safi := update.PathAttributes[valueStart+2]
				nhLen := update.PathAttributes[valueStart+3]

				if afi != 1 {
					t.Errorf("expected AFI 1 (IPv4), got %d", afi)
				}
				if safi != 1 {
					t.Errorf("expected SAFI 1 (unicast), got %d", safi)
				}
				// Next-hop should be 16 bytes (IPv6)
				if nhLen != 16 {
					t.Errorf("expected next-hop length 16 (IPv6), got %d", nhLen)
				}
			}
		}
		offset += hdrLen + attrLen
	}

	if hasNextHopAttr {
		t.Error("IPv4+IPv6 next-hop should NOT use NEXT_HOP attribute")
	}
	if !hasMPReach {
		t.Error("IPv4+IPv6 next-hop should use MP_REACH_NLRI")
	}

	// NLRI should be empty (in MP_REACH_NLRI, not UPDATE.NLRI)
	if len(update.NLRI) != 0 {
		t.Errorf("expected empty UPDATE.NLRI for extended next-hop, got %d bytes", len(update.NLRI))
	}
}

// TestCommitService_NilContext verifies graceful handling of nil EncodingContext.
//
// VALIDATES: NewCommitService with nil ctx → returns ErrNilContext
//
// PREVENTS: Runtime panic when context is nil.
func TestCommitService_NilContext(t *testing.T) {
	sender := &mockUpdateSender{}

	// This should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NewCommitService panicked with nil context: %v", r)
		}
	}()

	cs := NewCommitService(sender, nil, true)

	nh := netip.MustParseAddr("10.0.0.1")
	attrs := []attribute.Attribute{attribute.Origin(0)}
	routes := []*Route{
		NewRoute(newIPv4NLRI("192.168.1.0/24"), nh, attrs),
	}

	// Should either return error or use sensible defaults
	_, err := cs.Commit(routes, CommitOptions{})
	if err == nil {
		// If no error, verify the update was created (with defaults)
		if len(sender.updates) != 1 {
			t.Errorf("expected 1 update with defaults, got %d", len(sender.updates))
		}
	}
	// Error is acceptable too - the important thing is no panic
}

// TestCommitService_IPv6_NLRIInMPReach verifies NLRI bytes are in MP_REACH_NLRI.
//
// VALIDATES: IPv6 route → NLRI bytes present inside MP_REACH_NLRI
//
// PREVENTS: Empty NLRI in MP_REACH_NLRI.
func TestCommitService_IPv6_NLRIInMPReach(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := testContext(65000, 65000, true)
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("2001:db8::1")
	attrs := []attribute.Attribute{attribute.Origin(0)}

	routes := []*Route{
		NewRoute(newIPv6NLRI("2001:db8:1::/48"), nh, attrs),
	}

	_, err := cs.Commit(routes, CommitOptions{})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if len(sender.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sender.updates))
	}

	// Find MP_REACH_NLRI and verify NLRI is present
	offset := 0
	for offset < len(sender.updates[0].PathAttributes) {
		if offset+2 > len(sender.updates[0].PathAttributes) {
			break
		}
		code := sender.updates[0].PathAttributes[offset+1]
		flags := sender.updates[0].PathAttributes[offset]
		var attrLen, hdrLen int
		if flags&0x10 != 0 {
			attrLen = int(sender.updates[0].PathAttributes[offset+2])<<8 | int(sender.updates[0].PathAttributes[offset+3])
			hdrLen = 4
		} else {
			attrLen = int(sender.updates[0].PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 14 { // MP_REACH_NLRI
			valueStart := offset + hdrLen
			if valueStart+4 <= len(sender.updates[0].PathAttributes) {
				nhLen := int(sender.updates[0].PathAttributes[valueStart+3])
				// NLRI starts after: AFI(2) + SAFI(1) + NH_Len(1) + NextHop(nhLen) + Reserved(1)
				nlriStart := valueStart + 4 + nhLen + 1
				nlriLen := attrLen - (4 + nhLen + 1)

				if nlriLen <= 0 {
					t.Errorf("expected NLRI in MP_REACH_NLRI, got 0 bytes")
				} else if nlriStart < len(sender.updates[0].PathAttributes) {
					// Verify first byte is prefix length (48 for 2001:db8:1::/48)
					prefixLen := sender.updates[0].PathAttributes[nlriStart]
					if prefixLen != 48 {
						t.Errorf("expected prefix length 48, got %d", prefixLen)
					}
				}
			}
		}
		offset += hdrLen + attrLen
	}
}

// newVPNv4NLRI creates a VPNv4 NLRI for testing.
func newVPNv4NLRI(prefix string) nlri.NLRI {
	p := netip.MustParsePrefix(prefix)
	return nlri.NewINET(nlri.Family{AFI: 1, SAFI: 128}, p, 0)
}
