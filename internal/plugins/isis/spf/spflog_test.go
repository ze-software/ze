// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- SPF-log ring boundary tests.
// Related: spflog.go -- the bounded SPF-run history under test.

package spf

import (
	"testing"
	"time"
)

// TestSPFLogRecordAndSnapshotOrder verifies a recorded run is read back newest
// first with all fields preserved (the `show isis spf-log` ordering, AC-6).
func TestSPFLogRecordAndSnapshotOrder(t *testing.T) {
	var l spfLog
	base := time.Unix(1_000, 0)
	l.setTrigger("lsdb-change")
	l.record(base, "l1", 250*time.Microsecond, 3)
	l.record(base.Add(time.Second), "l2", 500*time.Microsecond, 5)

	got := l.snapshot()
	if len(got) != 2 {
		t.Fatalf("snapshot len = %d, want 2", len(got))
	}
	// Newest first: the l2 run recorded second must be at index 0.
	if got[0].Level != "l2" || got[1].Level != "l1" {
		t.Fatalf("order wrong: got[0]=%q got[1]=%q, want l2 then l1", got[0].Level, got[1].Level)
	}
	if got[0].Trigger != "lsdb-change" {
		t.Errorf("trigger = %q, want lsdb-change", got[0].Trigger)
	}
	if got[1].Nodes != 3 {
		t.Errorf("nodes = %d, want 3", got[1].Nodes)
	}
	if got[1].DurationSeconds != (250 * time.Microsecond).Seconds() {
		t.Errorf("duration = %v, want %v", got[1].DurationSeconds, (250 * time.Microsecond).Seconds())
	}
}

// TestSPFLogDefaultTrigger verifies a run with no trigger set reports "manual"
// (a direct Run() with no engine debounce, e.g. a test).
func TestSPFLogDefaultTrigger(t *testing.T) {
	var l spfLog
	l.record(time.Unix(1, 0), "l1", time.Millisecond, 1)
	got := l.snapshot()
	if len(got) != 1 || got[0].Trigger != "manual" {
		t.Fatalf("default trigger = %+v, want one entry with trigger=manual", got)
	}
}

// TestSPFLogRingBound is the boundary test for the bounded ring: at and beyond
// spfLogCapacity the ring keeps exactly the most recent spfLogCapacity entries
// and evicts the oldest (resource-exhaustion bound, security review).
func TestSPFLogRingBound(t *testing.T) {
	var l spfLog
	// Record one past capacity. Each entry's Nodes encodes its insertion index so
	// we can assert which were evicted.
	for i := range spfLogCapacity + 1 {
		l.record(time.Unix(int64(i), 0), "l1", time.Microsecond, i)
	}
	got := l.snapshot()
	if len(got) != spfLogCapacity {
		t.Fatalf("ring len = %d, want exactly capacity %d", len(got), spfLogCapacity)
	}
	// Newest first: index 0 holds the last-inserted (Nodes==spfLogCapacity).
	if got[0].Nodes != spfLogCapacity {
		t.Errorf("newest Nodes = %d, want %d", got[0].Nodes, spfLogCapacity)
	}
	// The oldest surviving entry is insertion index 1 (index 0 was evicted).
	if got[len(got)-1].Nodes != 1 {
		t.Errorf("oldest surviving Nodes = %d, want 1 (index 0 evicted)", got[len(got)-1].Nodes)
	}
}

// TestSPFLogReset verifies `clear isis counters` empties the history.
func TestSPFLogReset(t *testing.T) {
	var l spfLog
	l.record(time.Unix(1, 0), "l1", time.Microsecond, 1)
	l.reset()
	if got := l.snapshot(); len(got) != 0 {
		t.Fatalf("after reset len = %d, want 0", len(got))
	}
}

// TestComputerSPFLogAfterRun verifies a real Run records into the Computer's log
// and that SetSPFLogTrigger/ResetSPFLog round-trip through the public API.
func TestComputerSPFLogAfterRun(t *testing.T) {
	c := NewComputer(Config{Levels: []Level{Level1}})
	c.SetSPFLogTrigger("lsdb-change")
	c.Run()
	log := c.SPFLog()
	if len(log) == 0 {
		t.Fatal("SPFLog empty after Run; want at least one entry")
	}
	if log[0].Trigger != "lsdb-change" {
		t.Errorf("trigger = %q, want lsdb-change", log[0].Trigger)
	}
	c.ResetSPFLog()
	if got := c.SPFLog(); len(got) != 0 {
		t.Errorf("after ResetSPFLog len = %d, want 0", len(got))
	}
}
