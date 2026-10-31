// VALIDATES: rfc.Requirement stays small enough to range over by value. The
// package ranges over []Requirement in some sixty loops, and gocritic's
// rangeValCopy refuses each one once the struct reaches its threshold.
// .golangci.yml sets it: "rangeValCopy: sizeThreshold: 160", and the checker
// warns when "size >= c.sizeThreshold"
// (vendor/github.com/go-critic/go-critic/checkers/rangeValCopy_checker.go).
// So the size must stay strictly below 160 bytes.

package rfc

import (
	"testing"
	"unsafe"
)

// requirementRangeValCopyThreshold is gocritic's rangeValCopy sizeThreshold
// in .golangci.yml.
const requirementRangeValCopyThreshold = 160

func TestRequirementStaysUnderRangeValCopy(t *testing.T) {
	size := unsafe.Sizeof(Requirement{})
	if size >= requirementRangeValCopyThreshold {
		t.Fatalf("rfc.Requirement is %d bytes, at or above gocritic's rangeValCopy threshold of %d: every `range []Requirement` loop is refused. Narrow or pack a field (Line beside Ticked and Derived is the precedent) rather than rewriting the loops",
			size, requirementRangeValCopyThreshold)
	}
}
