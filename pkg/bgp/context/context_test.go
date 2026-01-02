package context

import (
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

// TestEncodingContextHash_Deterministic verifies that identical contexts produce identical hashes.
//
// VALIDATES: Hash() returns same value for structurally identical contexts.
//
// PREVENTS: Registry deduplication failures from non-deterministic hashing.
func TestEncodingContextHash_Deterministic(t *testing.T) {
	ctx1 := &EncodingContext{
		ASN4:    true,
		IsIBGP:  false,
		LocalAS: 65000,
		PeerAS:  65001,
		AddPath: map[nlri.Family]bool{
			{AFI: 1, SAFI: 1}: true,
			{AFI: 2, SAFI: 1}: false,
		},
		ExtendedNextHop: map[nlri.Family]nlri.AFI{
			{AFI: 1, SAFI: 1}: nlri.AFIIPv6, // IPv4 unicast can use IPv6 NH
		},
	}

	ctx2 := &EncodingContext{
		ASN4:    true,
		IsIBGP:  false,
		LocalAS: 65000,
		PeerAS:  65001,
		AddPath: map[nlri.Family]bool{
			{AFI: 1, SAFI: 1}: true,
			{AFI: 2, SAFI: 1}: false,
		},
		ExtendedNextHop: map[nlri.Family]nlri.AFI{
			{AFI: 1, SAFI: 1}: nlri.AFIIPv6,
		},
	}

	hash1 := ctx1.Hash()
	hash2 := ctx2.Hash()

	if hash1 != hash2 {
		t.Errorf("identical contexts have different hashes: %x != %x", hash1, hash2)
	}

	// Same context should return same hash on multiple calls
	h1 := ctx1.Hash()
	h2 := ctx1.Hash()
	if h1 != h2 {
		t.Error("same context returns different hashes on multiple calls")
	}
}

// TestEncodingContextHash_Different verifies that different contexts produce different hashes.
//
// VALIDATES: Hash() returns different values for structurally different contexts.
//
// PREVENTS: False deduplication of distinct contexts.
func TestEncodingContextHash_Different(t *testing.T) {
	base := &EncodingContext{
		ASN4:    true,
		IsIBGP:  false,
		LocalAS: 65000,
		PeerAS:  65001,
		AddPath: map[nlri.Family]bool{
			{AFI: 1, SAFI: 1}: true,
		},
	}

	tests := []struct {
		name   string
		modify func(*EncodingContext)
	}{
		{
			name: "different ASN4",
			modify: func(ctx *EncodingContext) {
				ctx.ASN4 = false
			},
		},
		{
			name: "different IsIBGP",
			modify: func(ctx *EncodingContext) {
				ctx.IsIBGP = true
			},
		},
		{
			name: "different LocalAS",
			modify: func(ctx *EncodingContext) {
				ctx.LocalAS = 65002
			},
		},
		{
			name: "different PeerAS",
			modify: func(ctx *EncodingContext) {
				ctx.PeerAS = 65002
			},
		},
		{
			name: "different AddPath",
			modify: func(ctx *EncodingContext) {
				ctx.AddPath = map[nlri.Family]bool{
					{AFI: 1, SAFI: 1}: false,
				}
			},
		},
		{
			name: "additional family",
			modify: func(ctx *EncodingContext) {
				ctx.AddPath = map[nlri.Family]bool{
					{AFI: 1, SAFI: 1}: true,
					{AFI: 2, SAFI: 1}: true,
				}
			},
		},
	}

	baseHash := base.Hash()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a modified copy
			modified := &EncodingContext{
				ASN4:    base.ASN4,
				IsIBGP:  base.IsIBGP,
				LocalAS: base.LocalAS,
				PeerAS:  base.PeerAS,
				AddPath: make(map[nlri.Family]bool),
			}
			for k, v := range base.AddPath {
				modified.AddPath[k] = v
			}
			tc.modify(modified)

			if modified.Hash() == baseHash {
				t.Errorf("modified context has same hash as base")
			}
		})
	}
}

// TestEncodingContextAddPathFor_True verifies AddPathFor returns true when enabled.
//
// VALIDATES: AddPathFor returns true for family with ADD-PATH enabled.
//
// PREVENTS: Wrong encoding that omits path ID when ADD-PATH is negotiated.
func TestEncodingContextAddPathFor_True(t *testing.T) {
	ctx := &EncodingContext{
		AddPath: map[nlri.Family]bool{
			{AFI: 1, SAFI: 1}:   true,
			{AFI: 2, SAFI: 1}:   false,
			{AFI: 1, SAFI: 128}: true,
		},
	}

	if !ctx.AddPathFor(nlri.Family{AFI: 1, SAFI: 1}) {
		t.Error("AddPathFor should return true for IPv4 unicast")
	}

	if !ctx.AddPathFor(nlri.Family{AFI: 1, SAFI: 128}) {
		t.Error("AddPathFor should return true for IPv4 MPLS VPN")
	}
}

// TestEncodingContextAddPathFor_False verifies AddPathFor returns false when disabled.
//
// VALIDATES: AddPathFor returns false for family with ADD-PATH disabled or absent.
//
// PREVENTS: Wrong encoding that includes path ID when ADD-PATH is not negotiated.
func TestEncodingContextAddPathFor_False(t *testing.T) {
	ctx := &EncodingContext{
		AddPath: map[nlri.Family]bool{
			{AFI: 1, SAFI: 1}: true,
			{AFI: 2, SAFI: 1}: false,
		},
	}

	if ctx.AddPathFor(nlri.Family{AFI: 2, SAFI: 1}) {
		t.Error("AddPathFor should return false for IPv6 unicast (explicitly false)")
	}

	// Family not in map at all
	if ctx.AddPathFor(nlri.Family{AFI: 1, SAFI: 2}) {
		t.Error("AddPathFor should return false for family not in map")
	}
}

// TestEncodingContextAddPathFor_NilMap verifies AddPathFor handles nil map safely.
//
// VALIDATES: AddPathFor returns false when AddPath map is nil.
//
// PREVENTS: Panic from nil map access.
func TestEncodingContextAddPathFor_NilMap(t *testing.T) {
	ctx := &EncodingContext{
		AddPath: nil,
	}

	if ctx.AddPathFor(nlri.Family{AFI: 1, SAFI: 1}) {
		t.Error("AddPathFor should return false for nil map")
	}
}

// TestEncodingContextToPackContext verifies PackContext conversion.
//
// VALIDATES: ToPackContext creates correct nlri.PackContext for given family.
//
// PREVENTS: NLRI encoding with wrong ADD-PATH or ASN4 setting.
func TestEncodingContextToPackContext(t *testing.T) {
	ctx := &EncodingContext{
		ASN4: true,
		AddPath: map[nlri.Family]bool{
			{AFI: 1, SAFI: 1}: true,
			{AFI: 2, SAFI: 1}: false,
		},
	}

	// IPv4 unicast with ADD-PATH
	pc := ctx.ToPackContext(nlri.Family{AFI: 1, SAFI: 1})
	if pc == nil {
		t.Fatal("ToPackContext returned nil")
	}
	if !pc.ASN4 {
		t.Error("PackContext.ASN4 should be true")
	}
	if !pc.AddPath {
		t.Error("PackContext.AddPath should be true for IPv4 unicast")
	}

	// IPv6 unicast without ADD-PATH
	pc2 := ctx.ToPackContext(nlri.Family{AFI: 2, SAFI: 1})
	if pc2 == nil {
		t.Fatal("ToPackContext returned nil")
	}
	if !pc2.ASN4 {
		t.Error("PackContext.ASN4 should be true")
	}
	if pc2.AddPath {
		t.Error("PackContext.AddPath should be false for IPv6 unicast")
	}
}

// TestEncodingContextExtendedNextHopFor verifies ExtendedNextHopFor lookup.
//
// VALIDATES: ExtendedNextHopFor returns correct next-hop AFI per family.
//
// PREVENTS: Wrong next-hop encoding when RFC 8950 is negotiated.
func TestEncodingContextExtendedNextHopFor(t *testing.T) {
	ctx := &EncodingContext{
		ExtendedNextHop: map[nlri.Family]nlri.AFI{
			{AFI: 1, SAFI: 1}: nlri.AFIIPv6, // IPv4 unicast can use IPv6 NH
		},
	}

	// IPv4 unicast should return AFIIPv6 (can use IPv6 next-hop)
	nhAFI := ctx.ExtendedNextHopFor(nlri.Family{AFI: 1, SAFI: 1})
	if nhAFI != nlri.AFIIPv6 {
		t.Errorf("ExtendedNextHopFor should return AFIIPv6 for IPv4 unicast, got %v", nhAFI)
	}

	// Family not in map should return 0
	nhAFI2 := ctx.ExtendedNextHopFor(nlri.Family{AFI: 2, SAFI: 1})
	if nhAFI2 != 0 {
		t.Errorf("ExtendedNextHopFor should return 0 for family not in map, got %v", nhAFI2)
	}

	// Nil map should return 0
	ctx2 := &EncodingContext{}
	nhAFI3 := ctx2.ExtendedNextHopFor(nlri.Family{AFI: 1, SAFI: 1})
	if nhAFI3 != 0 {
		t.Errorf("ExtendedNextHopFor should return 0 for nil map, got %v", nhAFI3)
	}
}

// Ensure PackContext import is used.
var _ = nlri.PackContext{}
