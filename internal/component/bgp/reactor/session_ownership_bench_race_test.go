//go:build race

// Design: docs/architecture/update-building.md -- one final ownership and duplicate boundary
// Related: session_ownership_bench_test.go -- the allocation guard this file stands in for

package reactor

import "testing"

// TestAdjOutForwardedWriteAllocations is skipped under the race detector,
// which drops a quarter of sync.Pool puts at random and moves values to the
// heap for its own instrumentation: the counts it yields belong to the
// detector, not to this build. The guard runs in every build without -race.
func TestAdjOutForwardedWriteAllocations(t *testing.T) {
	t.Skip("allocation counts under -race are the detector's: run without -race (session_ownership_bench_test.go)")
}
