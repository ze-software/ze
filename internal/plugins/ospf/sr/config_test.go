package sr

import (
	"net/netip"
	"testing"
)

func TestSRGBSRLBNoOverlap(t *testing.T) {
	// Valid: SRGB and SRLB disjoint, both Size > 0.
	ok := SRConfig{
		Enabled: true,
		SRGB:    []LabelRange{{Base: 16000, Size: 8000}},
		SRLB:    []LabelRange{{Base: 40000, Size: 1000}},
	}
	if err := ok.Validate(nil); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	// SRGB overlaps SRLB.
	bad := SRConfig{
		Enabled: true,
		SRGB:    []LabelRange{{Base: 16000, Size: 8000}},
		SRLB:    []LabelRange{{Base: 20000, Size: 1000}},
	}
	if err := bad.Validate(nil); err == nil {
		t.Fatalf("overlapping SRGB/SRLB must be rejected")
	}
}

func TestSRRangeSizeMustBePositive(t *testing.T) {
	bad := SRConfig{Enabled: true, SRGB: []LabelRange{{Base: 16000, Size: 0}}}
	if err := bad.Validate(nil); err == nil {
		t.Fatalf("Range Size 0 must be rejected (RFC 8665 §3.2)")
	}
}

func TestSRRangeWithinLabelBounds(t *testing.T) {
	// Base below the reserved boundary (16) is invalid.
	bad := SRConfig{Enabled: true, SRGB: []LabelRange{{Base: 10, Size: 10}}}
	if err := bad.Validate(nil); err == nil {
		t.Fatalf("SRGB base below MinLabel must be rejected")
	}
	// Range extending past the 20-bit label space is invalid.
	over := SRConfig{Enabled: true, SRGB: []LabelRange{{Base: 1048570, Size: 100}}}
	if err := over.Validate(nil); err == nil {
		t.Fatalf("SRGB extending past MaxLabel must be rejected")
	}
}

func TestSROverlapWithReservedRanges(t *testing.T) {
	// A reserved range models the LDP / RSVP-TE label space.
	reserved := []LabelRange{{Base: 24000, Size: 1000}}
	bad := SRConfig{
		Enabled: true,
		SRGB:    []LabelRange{{Base: 16000, Size: 9000}}, // 16000..24999 overlaps 24000..24999
		SRLB:    []LabelRange{{Base: 40000, Size: 100}},
	}
	if err := bad.Validate(reserved); err == nil {
		t.Fatalf("SRGB overlapping the LDP/RSVP-TE reserved range must be rejected")
	}
	good := SRConfig{
		Enabled: true,
		SRGB:    []LabelRange{{Base: 16000, Size: 8000}}, // 16000..23999, no overlap
		SRLB:    []LabelRange{{Base: 40000, Size: 100}},
	}
	if err := good.Validate(reserved); err != nil {
		t.Fatalf("non-overlapping config rejected: %v", err)
	}
}

func TestSRInternalOverlap(t *testing.T) {
	bad := SRConfig{
		Enabled: true,
		SRGB:    []LabelRange{{Base: 16000, Size: 1000}, {Base: 16500, Size: 1000}},
	}
	if err := bad.Validate(nil); err == nil {
		t.Fatalf("overlapping SRGB ranges must be rejected (RFC 8665 §3.2)")
	}
}

func TestSRPrefixSIDIndexWithinSRGB(t *testing.T) {
	bad := SRConfig{
		Enabled:  true,
		SRGB:     []LabelRange{{Base: 16000, Size: 100}},
		Prefixes: []PrefixSIDConfig{{Prefix: netip.MustParsePrefix("10.0.0.1/32"), Index: 200}},
	}
	if err := bad.Validate(nil); err == nil {
		t.Fatalf("prefix-SID index beyond total SRGB size must be rejected")
	}
	good := SRConfig{
		Enabled:  true,
		SRGB:     []LabelRange{{Base: 16000, Size: 100}},
		Prefixes: []PrefixSIDConfig{{Prefix: netip.MustParsePrefix("10.0.0.1/32"), Index: 5}},
	}
	if err := good.Validate(nil); err != nil {
		t.Fatalf("valid prefix-SID index rejected: %v", err)
	}
}

func TestSRDisabledSkipsValidation(t *testing.T) {
	// A disabled SR block never fails validation regardless of contents.
	c := SRConfig{Enabled: false, SRGB: []LabelRange{{Base: 0, Size: 0}}}
	if err := c.Validate(nil); err != nil {
		t.Fatalf("disabled SR must skip validation: %v", err)
	}
}
