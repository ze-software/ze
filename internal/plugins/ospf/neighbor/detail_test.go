// VALIDATES: spec-ospf-ext-14 AC-10/AC-11 -- Table.DetailSnapshot exposes the full
// per-neighbor state the summary omits: DD sequence, list sizes, dead-time, the last NSM
// event, and the raw Options / advertised Interface ID.
// PREVENTS: a neighbor detail view that drops the last-event or list-size fields.
package neighbor

import (
	"testing"
	"time"
)

func TestNeighborDetailSnapshot(t *testing.T) {
	tbl, cfg := testTable(t, "broadcast")
	peer := rid(t, "10.0.0.2")
	now := time.Unix(1, 0)
	tbl.Hello(hello(cfg, peer, true, now))

	details := tbl.DetailSnapshot()
	if len(details) != 1 {
		t.Fatalf("detail rows = %d, want 1", len(details))
	}
	d := details[0]
	if d.RouterID != "10.0.0.2" {
		t.Fatalf("router id = %q", d.RouterID)
	}
	if d.State == "" {
		t.Fatalf("state must be set")
	}
	if d.LastEvent != "2-way-received" {
		t.Fatalf("last event = %q, want 2-way-received", d.LastEvent)
	}
	if d.DeadTime <= 0 {
		t.Fatalf("dead time = %d, want > 0", d.DeadTime)
	}
}
