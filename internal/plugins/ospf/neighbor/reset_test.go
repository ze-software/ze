// VALIDATES: spec-ospf-13 AC-9 -- Table.ResetAll tears down every active neighbor (clear
// ip ospf neighbor / process) and returns the count; a second call returns 0 because all
// neighbors are already Down.
// PREVENTS: a clear that misses neighbors or miscounts, or that panics on an empty table.
package neighbor

import (
	"testing"
	"time"
)

func TestTableResetAll(t *testing.T) {
	tbl, cfg := testTable(t, NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	if reason := tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0))); reason != "" {
		t.Fatalf("hello: %s", reason)
	}

	if n := tbl.ResetAll(); n != 1 {
		t.Fatalf("ResetAll = %d, want 1 (the active neighbor torn down)", n)
	}
	if n := tbl.ResetAll(); n != 0 {
		t.Fatalf("second ResetAll = %d, want 0 (already Down)", n)
	}
}

func TestTableResetAllEmpty(t *testing.T) {
	tbl, _ := testTable(t, NetworkPointToPoint)
	if n := tbl.ResetAll(); n != 0 {
		t.Fatalf("ResetAll on an empty table = %d, want 0", n)
	}
}
