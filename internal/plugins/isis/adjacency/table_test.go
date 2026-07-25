// Design: plan/spec-isis-5-adjacency.md -- neighbor table keying + snapshot.
//
// VALIDATES: the per-circuit table keys by (System ID, level) so two LAN
// neighbors and one P2P adjacency are tracked correctly; the snapshot returns
// the per-neighbor fields; the grace period holds a Down record before deletion
// (Reap honors deleteAt); and MaxNeighbors caps the table.
// PREVENTS: mis-keyed duplicate adjacencies, a Down record vanishing before the
// grace period, or an unbounded table.

package adjacency

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// TestISISTableClear: `clear isis adjacency` drops every record and reports the
// count, so neighbors re-learn from the next Hello (spec-isis-13 AC-8).
func TestISISTableClear(t *testing.T) {
	tab := NewTable()
	tab.Get(types.SystemID{0, 0, 0, 0, 0, 0xa}, Level1)
	tab.Get(types.SystemID{0, 0, 0, 0, 0, 0xb}, Level2)
	if n := tab.Clear(); n != 2 {
		t.Fatalf("Clear returned %d, want 2", n)
	}
	if tab.Len() != 0 {
		t.Fatalf("after Clear Len = %d, want 0", tab.Len())
	}
	// Clearing an empty table is a no-op returning 0.
	if n := tab.Clear(); n != 0 {
		t.Fatalf("Clear on empty table returned %d, want 0", n)
	}
}

// TestISISNeighbourTableLANKeying: two LAN neighbors are keyed distinctly by
// System ID, and a P2P circuit holds one adjacency per level.
func TestISISNeighbourTableLANKeying(t *testing.T) {
	tab := NewTable()
	sysA := types.SystemID{0, 0, 0, 0, 0, 0xa}
	sysB := types.SystemID{0, 0, 0, 0, 0, 0xb}

	a1, created := tab.Get(sysA, Level1)
	if !created {
		t.Fatal("first Get should create")
	}
	a2, created := tab.Get(sysA, Level1)
	if created || a1 != a2 {
		t.Fatal("second Get for same key must return the same record")
	}
	// A different neighbor is a distinct record.
	b1, _ := tab.Get(sysB, Level1)
	if b1 == a1 {
		t.Fatal("distinct neighbors must be distinct records")
	}
	// Same neighbor, different level is a distinct record (L1L2 circuit).
	aL2, _ := tab.Get(sysA, Level2)
	if aL2 == a1 {
		t.Fatal("same neighbor at a different level must be a distinct record")
	}
	if tab.Len() != 3 {
		t.Fatalf("table Len = %d, want 3", tab.Len())
	}
}

// TestISISNeighbourTableSnapshot: the snapshot returns the per-neighbor fields,
// and the grace period holds a Down record until Reap after deleteAt.
func TestISISNeighbourTableSnapshot(t *testing.T) {
	tab := NewTable()
	sys := types.SystemID{0, 0, 0, 0, 0, 1}
	adj, _ := tab.Get(sys, Level1)
	adj.State = StateUp
	adj.SNPA = SNPA{0x02, 0, 0, 0, 0, 1}
	adj.IPv4 = netip.MustParseAddr("192.0.2.1")
	adj.HoldTime = 30
	now := time.Unix(2_000_000, 0)
	adj.HoldExpiry = now.Add(30 * time.Second)

	snap := tab.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot len = %d, want 1", len(snap))
	}
	row := snap[0]
	if row.SystemID != sys.String() {
		t.Errorf("SystemID = %q, want %q", row.SystemID, sys.String())
	}
	if row.State != "up" {
		t.Errorf("State = %q, want up", row.State)
	}
	if row.IPv4 != "192.0.2.1" {
		t.Errorf("IPv4 = %q, want 192.0.2.1", row.IPv4)
	}
	if row.SNPA != "02:00:00:00:00:01" {
		t.Errorf("SNPA = %q, want 02:00:00:00:00:01", row.SNPA)
	}
	if row.HoldExpiry != adj.HoldExpiry.Unix() {
		t.Errorf("HoldExpiry = %d, want %d", row.HoldExpiry, adj.HoldExpiry.Unix())
	}

	// Drop the adjacency: it stays in the table through the grace period.
	Down(adj, now, 120*time.Second)
	if tab.Len() != 1 {
		t.Fatalf("Down should not delete the record (grace period); Len = %d", tab.Len())
	}
	if removed := tab.Reap(now.Add(60 * time.Second)); removed != 0 {
		t.Fatalf("Reap inside grace period removed %d, want 0", removed)
	}
	if removed := tab.Reap(now.Add(121 * time.Second)); removed != 1 {
		t.Fatalf("Reap after grace removed %d, want 1", removed)
	}
	if tab.Len() != 0 {
		t.Fatalf("table not empty after grace-period reap: Len = %d", tab.Len())
	}
}

// TestISISNeighbourTableMaxNeighbors: the table caps the per-circuit adjacency
// count so a flood of distinct System IDs cannot exhaust memory.
func TestISISNeighbourTableMaxNeighbors(t *testing.T) {
	tab := NewTable()
	for i := range MaxNeighbors {
		sys := types.SystemID{0, 0, 0, 0, byte(i >> 8), byte(i)}
		if a, _ := tab.Get(sys, Level1); a == nil {
			t.Fatalf("Get(%d) returned nil before reaching the cap", i)
		}
	}
	// One past the cap: a new key must be refused.
	over := types.SystemID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	if a, created := tab.Get(over, Level1); a != nil || created {
		t.Fatalf("Get past MaxNeighbors should refuse: adj=%v created=%v", a, created)
	}
	if tab.Len() != MaxNeighbors {
		t.Fatalf("table Len = %d, want %d", tab.Len(), MaxNeighbors)
	}
}

// TestISISTableUpCount: UpCount reflects only Up adjacencies.
func TestISISTableUpCount(t *testing.T) {
	tab := NewTable()
	up, _ := tab.Get(types.SystemID{0, 0, 0, 0, 0, 1}, Level1)
	up.State = StateUp
	init, _ := tab.Get(types.SystemID{0, 0, 0, 0, 0, 2}, Level1)
	init.State = StateInitializing
	if got := tab.UpCount(); got != 1 {
		t.Fatalf("UpCount = %d, want 1", got)
	}
}
