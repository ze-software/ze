// Design: docs/architecture/config/yang-config-design.md -- leaf value validation

package config

import (
	"strings"
	"testing"
)

// TestValidateLeafValueRangeNamesTheFix checks the refusal of a ranged
// numeric leaf: a value outside the range, and a whole number too large for
// the leaf's type, are refused with the range as the why and the fix line
// holding the range in the value's place; a value that is no number keeps its
// type error (AC-35).
// VALIDATES: ValidateLeafValue and validateNumericRanges name the range to use.
// PREVENTS: an operator told only "invalid uint16" for a value one past the
// range, with no word on what to write instead.
func TestValidateLeafValueRangeNamesTheFix(t *testing.T) {
	t.Parallel()

	node := &LeafNode{Type: TypeUint16, Ranges: []NumericRange{{Min: "1", Max: "255"}}}

	path := []string{"router", "hops"}
	if err := ValidateLeafValue(node, path, "255"); err != nil {
		t.Fatalf("255 is the last valid value: %v", err)
	}
	for _, value := range []string{"0", "256", "65536", "99999999999999999999"} {
		err := ValidateLeafValue(node, path, value)
		if err == nil {
			t.Fatalf("%s is outside 1..255 and was accepted", value)
		}
		want := "router hops: \"" + value + "\" is outside the accepted range\nhops accepts a value in 1..255\nfix: set router hops <1..255>"
		if err.Error() != want {
			t.Fatalf("the refusal of %s is\n%v\nwant\n%s", value, err, want)
		}
	}
	for _, value := range []string{"-1", "abc", ""} {
		err := ValidateLeafValue(node, path, value)
		if err == nil {
			t.Fatalf("%q is no number and was accepted", value)
		}
		if strings.Contains(err.Error(), "outside the accepted range") {
			t.Fatalf("%q is no number, so it is a type error, not a range one: %v", value, err)
		}
	}
}
