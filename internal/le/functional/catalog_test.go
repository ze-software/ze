package functional

import (
	"slices"
	"testing"
)

// TestGatingNamesIsAReadOnlyCatalogue proves a consumer cannot mutate the
// runner's authoritative list through the exported answer.
func TestGatingNamesIsAReadOnlyCatalogue(t *testing.T) {
	names := GatingNames()
	if !slices.Equal(names, Gating) {
		t.Fatalf("the catalogue is %v, want %v", names, Gating)
	}
	names[0] = "changed-by-consumer"
	if Gating[0] == names[0] {
		t.Fatal("the read-only catalogue aliases the runner's mutable slice")
	}
}
