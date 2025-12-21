package rib

import (
	"net/netip"
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/message"
)

// TestCommitService_IPv4_HasNextHop verifies IPv4 unicast UPDATE has NEXT_HOP attribute.
//
// VALIDATES: IPv4 unicast routes → NEXT_HOP attribute (code 3) in path attributes
//
// PREVENTS: Missing mandatory NEXT_HOP attribute in IPv4 unicast UPDATEs.
func TestCommitService_IPv4_HasNextHop(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := &message.Negotiated{ASN4: true, LocalAS: 65000, PeerAS: 65000}
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
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

	// Verify NLRI is in UPDATE.NLRI (for IPv4 unicast)
	if len(update.NLRI) == 0 {
		t.Error("expected NLRI in UPDATE.NLRI for IPv4 unicast")
	}

	// Verify path attributes contain NEXT_HOP (attribute code 3)
	hasNextHop := false
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		code := update.PathAttributes[offset+1]
		flags := update.PathAttributes[offset]
		var attrLen int
		if flags&0x10 != 0 { // Extended length
			if offset+4 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			offset += 4
		} else {
			if offset+3 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])
			offset += 3
		}

		if code == 3 { // NEXT_HOP
			hasNextHop = true
			// Verify next-hop value is 10.0.0.1
			if attrLen != 4 {
				t.Errorf("expected NEXT_HOP length 4, got %d", attrLen)
			}
			if offset+attrLen <= len(update.PathAttributes) {
				nhBytes := update.PathAttributes[offset : offset+attrLen]
				if nhBytes[0] != 10 || nhBytes[1] != 0 || nhBytes[2] != 0 || nhBytes[3] != 1 {
					t.Errorf("expected next-hop 10.0.0.1, got %v", nhBytes)
				}
			}
		}
		offset += attrLen
	}

	if !hasNextHop {
		t.Error("UPDATE missing mandatory NEXT_HOP attribute for IPv4 unicast")
	}
}

// TestCommitService_IPv6_UsesMPReachNLRI verifies IPv6 routes use MP_REACH_NLRI.
//
// VALIDATES: IPv6 routes → MP_REACH_NLRI attribute with next-hop and NLRI inside
//
// PREVENTS: IPv6 NLRIs incorrectly placed in UPDATE.NLRI field.
func TestCommitService_IPv6_UsesMPReachNLRI(t *testing.T) {
	sender := &mockUpdateSender{}
	neg := &message.Negotiated{ASN4: true, LocalAS: 65000, PeerAS: 65000}
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

	update := sender.updates[0]

	// For IPv6, UPDATE.NLRI MUST be empty
	if len(update.NLRI) != 0 {
		t.Errorf("expected empty UPDATE.NLRI for IPv6, got %d bytes", len(update.NLRI))
	}

	// Verify path attributes contain MP_REACH_NLRI (attribute code 14)
	hasMPReach := false
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		code := update.PathAttributes[offset+1]
		flags := update.PathAttributes[offset]
		var attrLen int
		var hdrLen int
		if flags&0x10 != 0 { // Extended length
			if offset+4 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			hdrLen = 4
		} else {
			if offset+3 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 14 { // MP_REACH_NLRI
			hasMPReach = true
			valueStart := offset + hdrLen
			if valueStart+5 > len(update.PathAttributes) {
				t.Fatal("MP_REACH_NLRI too short")
			}

			// Parse AFI/SAFI
			afi := uint16(update.PathAttributes[valueStart])<<8 | uint16(update.PathAttributes[valueStart+1])
			safi := update.PathAttributes[valueStart+2]

			if afi != 2 {
				t.Errorf("expected AFI 2 (IPv6), got %d", afi)
			}
			if safi != 1 {
				t.Errorf("expected SAFI 1 (unicast), got %d", safi)
			}

			// Verify next-hop length (should be 16 for single IPv6)
			nhLen := update.PathAttributes[valueStart+3]
			if nhLen != 16 {
				t.Errorf("expected next-hop length 16, got %d", nhLen)
			}
		}
		offset += hdrLen + attrLen
	}

	if !hasMPReach {
		t.Error("UPDATE missing MP_REACH_NLRI attribute for IPv6")
	}
}

// TestCommitService_ASN4_EncodesASPath verifies AS_PATH uses 4-byte encoding when ASN4=true.
//
// VALIDATES: ASN4=true → 4-byte AS numbers in AS_PATH
//
// PREVENTS: Wrong AS_PATH encoding based on capability negotiation.
func TestCommitService_ASN4_EncodesASPath(t *testing.T) {
	sender := &mockUpdateSender{}
	// eBGP session with ASN4
	neg := &message.Negotiated{ASN4: true, LocalAS: 65000, PeerAS: 65001}
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
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

	// Verify AS_PATH attribute exists (code 2)
	hasASPath := false
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		code := update.PathAttributes[offset+1]
		flags := update.PathAttributes[offset]
		var attrLen int
		var hdrLen int
		if flags&0x10 != 0 {
			if offset+4 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			hdrLen = 4
		} else {
			if offset+3 > len(update.PathAttributes) {
				break
			}
			attrLen = int(update.PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 2 { // AS_PATH
			hasASPath = true
			// For eBGP with 1 AS, expect: segment type(1) + count(1) + ASN(4) = 6 bytes
			if attrLen < 6 {
				t.Errorf("expected AS_PATH length >= 6 for 4-byte AS, got %d", attrLen)
			}
		}
		offset += hdrLen + attrLen
	}

	if !hasASPath {
		t.Error("UPDATE missing AS_PATH attribute")
	}
}

// TestCommitService_iBGP_NoASPrepend verifies iBGP sessions don't prepend local AS.
//
// VALIDATES: LocalAS == PeerAS → empty AS_PATH (no prepending)
//
// PREVENTS: Incorrect AS prepending on iBGP sessions.
func TestCommitService_iBGP_NoASPrepend(t *testing.T) {
	sender := &mockUpdateSender{}
	// iBGP session (same AS)
	neg := &message.Negotiated{ASN4: true, LocalAS: 65000, PeerAS: 65000}
	cs := NewCommitService(sender, neg, true)

	nh := netip.MustParseAddr("10.0.0.1")
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

	// Find AS_PATH and verify it's empty (no segments)
	offset := 0
	for offset < len(update.PathAttributes) {
		if offset+2 > len(update.PathAttributes) {
			break
		}
		code := update.PathAttributes[offset+1]
		flags := update.PathAttributes[offset]
		var attrLen int
		var hdrLen int
		if flags&0x10 != 0 {
			attrLen = int(update.PathAttributes[offset+2])<<8 | int(update.PathAttributes[offset+3])
			hdrLen = 4
		} else {
			attrLen = int(update.PathAttributes[offset+2])
			hdrLen = 3
		}

		if code == 2 { // AS_PATH
			// iBGP should have empty AS_PATH (length 0)
			if attrLen != 0 {
				t.Errorf("expected empty AS_PATH for iBGP, got length %d", attrLen)
			}
		}
		offset += hdrLen + attrLen
	}
}
