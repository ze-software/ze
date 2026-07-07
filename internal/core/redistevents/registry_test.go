package redistevents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWouldLoop verifies the single definition of the redistribution loop
// invariant: a route from source protocol loops iff it would be redistributed
// back into the same protocol (source == dest).
//
// VALIDATES: WouldLoop(a, b) is true iff a == b, on opaque protocol names.
// PREVENTS: divergence between the three guard sites that previously inlined
// this comparison (config evaluator, egress fan-out, late-join replay).
func TestWouldLoop(t *testing.T) {
	tests := []struct {
		name   string
		source string
		dest   string
		want   bool
	}{
		{name: "equal non-empty loops", source: "bgp", dest: "bgp", want: true},
		{name: "differing does not loop", source: "ospf", dest: "bgp", want: false},
		{name: "source empty does not loop", source: "", dest: "bgp", want: false},
		{name: "dest empty does not loop", source: "bgp", dest: "", want: false},
		{name: "both empty matches string equality", source: "", dest: "", want: true},
		{name: "bgp sub-source name differs from umbrella", source: "ibgp", dest: "bgp", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, WouldLoop(tc.source, tc.dest))
		})
	}
}

// TestWouldLoopNoAlloc pins AC-4: the predicate is an O(1) string compare with
// zero heap allocation (it sits on the redistribution fan-out path).
func TestWouldLoopNoAlloc(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		_ = WouldLoop("bgp", "ospf")
		_ = WouldLoop("bgp", "bgp")
	})
	assert.Zero(t, allocs, "WouldLoop must not allocate")
}
