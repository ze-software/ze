package flowexport

import "testing"

// VALIDATES: Refresh signals an out-of-band dump and coalesces -- repeated calls
// before the run loop consumes the signal leave at most one pending.
// PREVENTS: a burst of AttackDetected events queueing a backlog of full-table
// conntrack dumps (one per event) that would pile CPU/netlink load onto the box
// exactly during an attack.
func TestConntrackWorkerRefreshCoalesces(t *testing.T) {
	w := newConntrackWorker(nil, ConntrackConfig{ActiveTimeout: 60})

	if len(w.refreshCh) != 0 {
		t.Fatalf("new worker: refresh pending = %d, want 0", len(w.refreshCh))
	}
	w.Refresh()
	w.Refresh()
	w.Refresh()
	if got := len(w.refreshCh); got != 1 {
		t.Fatalf("after 3 Refresh calls: pending = %d, want 1 (coalesced)", got)
	}

	// Draining (as the run loop does) re-arms the signal.
	<-w.refreshCh
	w.Refresh()
	if got := len(w.refreshCh); got != 1 {
		t.Fatalf("after drain + Refresh: pending = %d, want 1", got)
	}
}

// VALIDATES: Refresh is a no-op on a nil worker.
// PREVENTS: a panic when the exporter delegates through refreshConntrack before
// the conntrack worker exists (conntrack disabled) or after teardown.
func TestConntrackWorkerRefreshNilSafe(t *testing.T) {
	var w *conntrackWorker
	w.Refresh() // must not panic
}
