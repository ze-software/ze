// Design: docs/architecture/config/yang-config-design.md -- leaf value validation

package config

import (
	"strings"
	"testing"
)

// TestValidateLeafValueRangeNamesTheFix checks the refusal of a ranged
// numeric leaf: a value outside the range, and a whole number too large for
// the leaf's type, are refused with the range and the fix; a value that is no
// number keeps its type error.
// VALIDATES: ValidateLeafValue and validateNumericRanges name the range to use.
// PREVENTS: an operator told only "invalid uint16" for a value one past the
// range, with no word on what to write instead.
func TestValidateLeafValueRangeNamesTheFix(t *testing.T) {
	t.Parallel()

	node := &LeafNode{Type: TypeUint16, Ranges: []NumericRange{{Min: "1", Max: "255"}}}

	if err := ValidateLeafValue(node, "255"); err != nil {
		t.Fatalf("255 is the last valid value: %v", err)
	}
	for _, value := range []string{"0", "256", "65536", "99999999999999999999"} {
		err := ValidateLeafValue(node, value)
		if err == nil {
			t.Fatalf("%s is outside 1..255 and was accepted", value)
		}
		if !strings.Contains(err.Error(), "outside range 1..255: set a value in 1..255") {
			t.Fatalf("the refusal of %s names no fix: %v", value, err)
		}
	}
	for _, value := range []string{"-1", "abc", ""} {
		err := ValidateLeafValue(node, value)
		if err == nil {
			t.Fatalf("%q is no number and was accepted", value)
		}
		if strings.Contains(err.Error(), "outside range") {
			t.Fatalf("%q is no number, so it is a type error, not a range one: %v", value, err)
		}
	}
}
