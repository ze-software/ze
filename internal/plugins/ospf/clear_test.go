// VALIDATES: spec-ospf-13 AC-9 -- the engine clear methods delegate to the neighbor table
// and SPF computer and are safe on a fresh engine (no adjacencies, SPF idle).
// PREVENTS: a clear command that panics or miscounts when nothing is up yet.
package ospf

import "testing"

func TestEngineClearMethods(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)

	if n := eng.clearNeighbors(); n != 0 {
		t.Fatalf("clearNeighbors on a fresh engine = %d, want 0", n)
	}
	if n := eng.clearProcess(); n != 0 {
		t.Fatalf("clearProcess on a fresh engine = %d, want 0", n)
	}
	eng.clearCounters() // must not panic with SPF idle
}
