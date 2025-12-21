package message

import (
	"encoding/hex"
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

// TestBuildEOR_IPv4Unicast verifies IPv4 unicast EOR is an empty UPDATE.
//
// VALIDATES: Empty UPDATE (no withdrawn, no attributes, no NLRI) for IPv4 unicast
//
// PREVENTS: Wrong EOR format for IPv4 unicast family.
func TestBuildEOR_IPv4Unicast(t *testing.T) {
	family := nlri.Family{AFI: 1, SAFI: 1}
	update := BuildEOR(family)

	if update == nil {
		t.Fatal("BuildEOR returned nil")
	}

	// IPv4 unicast EOR is empty UPDATE
	if len(update.WithdrawnRoutes) != 0 {
		t.Errorf("expected empty WithdrawnRoutes, got %d bytes", len(update.WithdrawnRoutes))
	}
	if len(update.PathAttributes) != 0 {
		t.Errorf("expected empty PathAttributes, got %d bytes", len(update.PathAttributes))
	}
	if len(update.NLRI) != 0 {
		t.Errorf("expected empty NLRI, got %d bytes", len(update.NLRI))
	}

	// Should be detected as EOR
	if !update.IsEndOfRIB() {
		t.Error("IPv4 unicast EOR should be detected by IsEndOfRIB()")
	}
}

// TestBuildEOR_IPv6Unicast verifies IPv6 unicast EOR uses MP_UNREACH_NLRI.
//
// VALIDATES: MP_UNREACH_NLRI with AFI=2, SAFI=1 and empty withdrawn for IPv6 unicast
//
// PREVENTS: Wrong EOR format for non-IPv4-unicast families.
func TestBuildEOR_IPv6Unicast(t *testing.T) {
	family := nlri.Family{AFI: 2, SAFI: 1}
	update := BuildEOR(family)

	if update == nil {
		t.Fatal("BuildEOR returned nil")
	}

	// IPv6 unicast EOR uses MP_UNREACH_NLRI attribute
	if len(update.WithdrawnRoutes) != 0 {
		t.Errorf("expected empty WithdrawnRoutes, got %d bytes", len(update.WithdrawnRoutes))
	}
	if len(update.NLRI) != 0 {
		t.Errorf("expected empty NLRI, got %d bytes", len(update.NLRI))
	}

	// Must have MP_UNREACH_NLRI attribute (code 15)
	if len(update.PathAttributes) == 0 {
		t.Fatal("expected MP_UNREACH_NLRI attribute, got empty PathAttributes")
	}

	// Parse attribute header: flags(1) + code(1) + length(1 or 2) + value
	// MP_UNREACH_NLRI value for EOR: AFI(2) + SAFI(1) = 3 bytes
	// Expected: flags=0x80 (optional), code=15, length=3, value=00 02 01
	attrs := update.PathAttributes
	if len(attrs) < 3 {
		t.Fatalf("PathAttributes too short: %d bytes", len(attrs))
	}

	flags := attrs[0]
	code := attrs[1]

	// Check attribute code is MP_UNREACH_NLRI (15)
	if code != 15 {
		t.Errorf("expected attribute code 15 (MP_UNREACH_NLRI), got %d", code)
	}

	// Check flags include optional (0x80)
	if flags&0x80 == 0 {
		t.Errorf("expected optional flag set, got flags 0x%02x", flags)
	}

	// Extended length is always used for consistency
	if flags&0x10 == 0 {
		t.Errorf("expected extended length flag set, got flags 0x%02x", flags)
	}
	if len(attrs) < 4 {
		t.Fatalf("PathAttributes too short for extended length: %d bytes", len(attrs))
	}
	valueLen := int(attrs[2])<<8 | int(attrs[3])
	valueOffset := 4

	// Value should be AFI(2) + SAFI(1) = 3 bytes for EOR
	if valueLen != 3 {
		t.Errorf("expected MP_UNREACH_NLRI length 3, got %d", valueLen)
	}

	if len(attrs) < valueOffset+valueLen {
		t.Fatalf("PathAttributes too short for value: %d bytes, need %d", len(attrs), valueOffset+valueLen)
	}

	value := attrs[valueOffset : valueOffset+valueLen]

	// Check AFI=2 (big-endian)
	afi := uint16(value[0])<<8 | uint16(value[1])
	if afi != 2 {
		t.Errorf("expected AFI 2, got %d", afi)
	}

	// Check SAFI=1
	safi := value[2]
	if safi != 1 {
		t.Errorf("expected SAFI 1, got %d", safi)
	}
}

// TestBuildEOR_VPNv4 verifies VPNv4 EOR uses MP_UNREACH_NLRI with correct AFI/SAFI.
//
// VALIDATES: MP_UNREACH_NLRI with AFI=1, SAFI=128 for VPNv4
//
// PREVENTS: Wrong family encoding in EOR for MPLS families.
func TestBuildEOR_VPNv4(t *testing.T) {
	family := nlri.Family{AFI: 1, SAFI: 128}
	update := BuildEOR(family)

	if update == nil {
		t.Fatal("BuildEOR returned nil")
	}

	// VPNv4 uses MP_UNREACH_NLRI (not empty UPDATE like IPv4 unicast)
	if len(update.PathAttributes) == 0 {
		t.Fatal("expected MP_UNREACH_NLRI attribute for VPNv4")
	}

	attrs := update.PathAttributes
	code := attrs[1]
	if code != 15 {
		t.Errorf("expected attribute code 15, got %d", code)
	}

	// Check AFI/SAFI in value (extended length format, offset=4)
	value := attrs[4:]
	afi := uint16(value[0])<<8 | uint16(value[1])
	safi := value[2]

	if afi != 1 {
		t.Errorf("expected AFI 1, got %d", afi)
	}
	if safi != 128 {
		t.Errorf("expected SAFI 128, got %d", safi)
	}
}

// TestBuildEOR_WireFormat verifies exact wire format for IPv6 unicast EOR.
//
// VALIDATES: Exact byte-for-byte wire format matches expected
//
// PREVENTS: Encoding bugs that would cause interop failures.
func TestBuildEOR_WireFormat(t *testing.T) {
	family := nlri.Family{AFI: 2, SAFI: 1}
	update := BuildEOR(family)

	packed, err := update.Pack(nil)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}

	// Expected wire format (30 bytes total):
	// Marker: ffffffffffffffffffffffffffffffff (16)
	// Length: 001e (2) = 30
	// Type: 02 (1) = UPDATE
	// Withdrawn Length: 0000 (2)
	// Attr Length: 0007 (2) = 7 bytes
	// MP_UNREACH_NLRI: 90 0f 00 03 00 02 01 (7)
	//   flags=90 (optional + ext length), code=0f (15), len=0003, afi=0002, safi=01

	expectedHex := "ffffffffffffffffffffffffffffffff" + // marker
		"001e" + // length 30
		"02" + // type UPDATE
		"0000" + // withdrawn length
		"0007" + // attr length
		"900f0003000201" // MP_UNREACH_NLRI

	expected, _ := hex.DecodeString(expectedHex)

	if len(packed) != len(expected) {
		t.Errorf("wrong length: got %d, want %d", len(packed), len(expected))
		t.Logf("got:  %s", hex.EncodeToString(packed))
		t.Logf("want: %s", expectedHex)
		return
	}

	for i := range packed {
		if packed[i] != expected[i] {
			t.Errorf("mismatch at byte %d: got 0x%02x, want 0x%02x", i, packed[i], expected[i])
			t.Logf("got:  %s", hex.EncodeToString(packed))
			t.Logf("want: %s", expectedHex)
			return
		}
	}
}
