// Design: plan/learned/955-ospf-1-types.md -- AreaID integer and dotted forms

package types

import "testing"

// VALIDATES: AC-3 - integer and dotted Area IDs normalize to the same 4-byte value.
// PREVENTS: treating area 0 and area 0.0.0.0 as distinct areas.
func TestAreaIDIntegerAndDottedForms(t *testing.T) {
	areaZero, err := ParseAreaID("0")
	if err != nil {
		t.Fatalf("ParseAreaID integer zero returned error: %v", err)
	}
	dottedZero, err := ParseAreaID("0.0.0.0")
	if err != nil {
		t.Fatalf("ParseAreaID dotted zero returned error: %v", err)
	}
	if areaZero != dottedZero || areaZero != BackboneArea {
		t.Fatalf("area zero mismatch: integer=%v dotted=%v backbone=%v", areaZero, dottedZero, BackboneArea)
	}
	if !areaZero.IsBackbone() {
		t.Fatalf("area 0 did not report backbone")
	}
	if got := areaZero.String(); got != "0.0.0.0" {
		t.Fatalf("area zero String() = %q, want 0.0.0.0", got)
	}

	one, err := ParseAreaID("1")
	if err != nil {
		t.Fatalf("ParseAreaID integer one returned error: %v", err)
	}
	dottedOne, err := ParseAreaID("0.0.0.1")
	if err != nil {
		t.Fatalf("ParseAreaID dotted one returned error: %v", err)
	}
	if one != dottedOne {
		t.Fatalf("area 1 mismatch: integer=%v dotted=%v", one, dottedOne)
	}

	maxArea, err := ParseAreaID("4294967295")
	if err != nil {
		t.Fatalf("ParseAreaID max returned error: %v", err)
	}
	maxDotted, err := ParseAreaID("255.255.255.255")
	if err != nil {
		t.Fatalf("ParseAreaID max dotted returned error: %v", err)
	}
	if maxArea != maxDotted {
		t.Fatalf("max area mismatch: integer=%v dotted=%v", maxArea, maxDotted)
	}
}

// VALIDATES: AC-2 - AreaIDFromBytes rejects lengths other than 4.
// PREVENTS: wire parsers accepting truncated or overlong Area IDs.
func TestAreaIDFromBytesRejectsWrongLength(t *testing.T) {
	for _, input := range [][]byte{{1, 2, 3}, {1, 2, 3, 4, 5}} {
		if _, err := AreaIDFromBytes(input); err == nil {
			t.Fatalf("AreaIDFromBytes(%v) succeeded, want error", input)
		}
	}
}
