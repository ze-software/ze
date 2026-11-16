// Design: docs/research/l2tpv2-ze-integration.md -- explicit per-counter availability
// Related: counter_reading_control_test.go -- original admitted-control propagation

package ppp

import (
	"errors"
	"math"
	"testing"
)

// A missing reading must differ from a measured zero without guessing from its value.
func TestCounterReadingDistinguishesUnknownFromMeasuredZero(t *testing.T) {
	var unknown CounterReading
	if _, err := unknown.Value(); !errors.Is(err, ErrCounterUnavailable) {
		t.Fatalf("zero reading invented a measurement: %v", err)
	}
	value, err := MeasuredCounter(0).Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != 0 {
		t.Fatalf("measured zero changed: %d", value)
	}
}

// Every uint64 count is representable; availability does not steal a sentinel value.
func TestCounterReadingPreservesMaximumCount(t *testing.T) {
	value, err := MeasuredCounter(math.MaxUint64).Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != math.MaxUint64 {
		t.Fatalf("maximum count changed: %d", value)
	}
}
