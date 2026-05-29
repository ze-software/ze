package conntrack

import (
	"net/netip"
	"testing"
	"time"
)

func testEntry(srcPort uint16, bytes, packets uint64) FlowEntry {
	return FlowEntry{
		SrcAddr:  netip.MustParseAddr("10.0.0.1"),
		DstAddr:  netip.MustParseAddr("10.0.0.2"),
		SrcPort:  srcPort,
		DstPort:  80,
		Protocol: 6,
		Bytes:    bytes,
		Packets:  packets,
		LastSeen: time.Now(),
	}
}

func TestConntrackDelta(t *testing.T) {
	d := NewDeltaTracker()

	// Each periodic dump advances the generation.
	e1 := d.ComputeDelta(testEntry(1001, 1000, 10), d.BeginDump())
	if e1.Bytes != 1000 {
		t.Fatalf("first bytes = %d, want 1000", e1.Bytes)
	}
	if e1.Packets != 10 {
		t.Fatalf("first packets = %d, want 10", e1.Packets)
	}

	e2 := d.ComputeDelta(testEntry(1001, 2500, 25), d.BeginDump())
	if e2.Bytes != 1500 {
		t.Fatalf("delta bytes = %d, want 1500", e2.Bytes)
	}
	if e2.Packets != 15 {
		t.Fatalf("delta packets = %d, want 15", e2.Packets)
	}

	e3 := d.ComputeDelta(testEntry(1001, 3000, 30), d.BeginDump())
	if e3.Bytes != 500 {
		t.Fatalf("delta bytes = %d, want 500", e3.Bytes)
	}
	if e3.Packets != 5 {
		t.Fatalf("delta packets = %d, want 5", e3.Packets)
	}
}

func TestConntrackDeltaWrap(t *testing.T) {
	d := NewDeltaTracker()

	d.ComputeDelta(testEntry(2001, 1000, 50), d.BeginDump())

	// A later dump observes lower counters (counter wrap / reset): report the
	// full new value.
	e := d.ComputeDelta(testEntry(2001, 200, 5), d.BeginDump())
	if e.Bytes != 200 {
		t.Fatalf("wrap bytes = %d, want 200", e.Bytes)
	}
	if e.Packets != 5 {
		t.Fatalf("wrap packets = %d, want 5", e.Packets)
	}
}

// TestConntrackDeltaDestroyThenStaleDump reproduces the dump/destroy race:
// a flow is exported by a periodic dump, then a destroy event (ComputeDeltaFinal)
// computes its residual delta, and finally a dump whose snapshot was captured
// just before the teardown processes the same flow at its final cumulative.
// That trailing observation MUST yield a zero delta -- not the full total -- so
// the flow is not double counted. (Regression: an earlier RemoveFlow
// hard-deleted the baseline on destroy, making the stale dump a first
// observation that re-exported the entire cumulative.)
func TestConntrackDeltaDestroyThenStaleDump(t *testing.T) {
	d := NewDeltaTracker()
	now := time.Now()

	// Periodic dump #1: first observation exports the full 500 bytes.
	first := d.ComputeDelta(testEntry(3001, 500, 5), d.BeginDump())
	if first.Bytes != 500 || first.Packets != 5 {
		t.Fatalf("first dump = %d bytes / %d pkts, want 500/5", first.Bytes, first.Packets)
	}

	// Dump #2 snapshot epoch begins (it captured the flow at its final 800).
	gen2 := d.BeginDump()

	// Destroy event: final cumulative 800 -> residual 300 exported, tombstoned.
	// It rides the latest dump generation (gen2).
	destroyed := d.ComputeDeltaFinal(testEntry(3001, 800, 8), now)
	if destroyed.Bytes != 300 || destroyed.Packets != 3 {
		t.Fatalf("destroy delta = %d bytes / %d pkts, want 300/3", destroyed.Bytes, destroyed.Packets)
	}

	// The dump #2 entry (same generation, snapshot taken before teardown) at the
	// final cumulative 800: must be a zero delta, otherwise the flow is double
	// counted.
	stale := d.ComputeDelta(testEntry(3001, 800, 8), gen2)
	if stale.Bytes != 0 || stale.Packets != 0 {
		t.Fatalf("stale dump delta = %d bytes / %d pkts, want 0/0 (double-count regression)", stale.Bytes, stale.Packets)
	}
}

// TestConntrackDeltaIntermediateStaleDump is the generation-counter regression:
// a dump snapshot that captured an INTERMEDIATE cumulative (700, between the
// last export 600 and the final 800) is processed AFTER the destroy event
// already recorded 800. Without generations the trailing 700 reads 700 < 800,
// is mistaken for a counter reset, and re-exports the full 700 while reviving
// the flow as live -- an over-count. With generations the stale dump entry
// shares the destroy's generation and is not newer than the terminal baseline,
// so it yields zero.
func TestConntrackDeltaIntermediateStaleDump(t *testing.T) {
	d := NewDeltaTracker()
	now := time.Now()

	// Dump #1: last export at cumulative 600.
	first := d.ComputeDelta(testEntry(3050, 600, 6), d.BeginDump())
	if first.Bytes != 600 || first.Packets != 6 {
		t.Fatalf("first dump = %d bytes / %d pkts, want 600/6", first.Bytes, first.Packets)
	}

	// Dump #2 snapshot epoch begins (it captured the flow at the intermediate 700).
	gen2 := d.BeginDump()

	// Destroy event recording the final 800 is processed first: residual 200.
	destroyed := d.ComputeDeltaFinal(testEntry(3050, 800, 8), now)
	if destroyed.Bytes != 200 || destroyed.Packets != 2 {
		t.Fatalf("destroy delta = %d bytes / %d pkts, want 200/2", destroyed.Bytes, destroyed.Packets)
	}

	// The trailing dump #2 entry at the intermediate 700 MUST yield zero.
	stale := d.ComputeDelta(testEntry(3050, 700, 7), gen2)
	if stale.Bytes != 0 || stale.Packets != 0 {
		t.Fatalf("intermediate stale dump = %d bytes / %d pkts, want 0/0 (over-count regression)", stale.Bytes, stale.Packets)
	}
}

// TestConntrackDeltaTombstoneReclaimed verifies a cleanly torn-down flow is
// reclaimed by SweepTombstones within the grace window -- the fast reclaim path
// that keeps residency (and peak map size) bounded by grace rather than by 2x
// active-timeout.
func TestConntrackDeltaTombstoneReclaimed(t *testing.T) {
	d := NewDeltaTracker()
	now := time.Now()
	grace := 5 * time.Second

	d.ComputeDeltaFinal(testEntry(3100, 1000, 10), now)
	if d.Len() != 1 {
		t.Fatalf("len after destroy = %d, want 1 (tombstone retained)", d.Len())
	}

	// Within grace: NOT reclaimed (a stale dump could still arrive).
	d.SweepTombstones(grace, now.Add(grace-time.Second))
	if d.Len() != 1 {
		t.Fatalf("len within grace = %d, want 1 (must survive for stale dumps)", d.Len())
	}

	// Past grace: reclaimed.
	d.SweepTombstones(grace, now.Add(grace+time.Second))
	if d.Len() != 0 {
		t.Fatalf("len past grace = %d, want 0 (tombstone reclaimed)", d.Len())
	}
}

// TestConntrackDeltaPortReuseRevivesEntry guards the generation ordering: when a
// fresh flow reuses a torn-down flow's 5-tuple (kernel source-port reuse), it
// arrives in a LATER dump cycle (a higher generation), so its reset-lower
// counters MUST clear the tombstone and revive the entry as live -- otherwise
// SweepTombstones would reclaim a live flow, which would then be re-observed as
// a first observation and re-export its full cumulative on every dump (a silent
// over-count).
func TestConntrackDeltaPortReuseRevivesEntry(t *testing.T) {
	d := NewDeltaTracker()
	now := time.Now()
	grace := 5 * time.Second

	// Flow A on port 3200 is torn down at cumulative 1000.
	d.ComputeDeltaFinal(testEntry(3200, 1000, 10), now)

	// Flow B reuses port 3200; its counters start fresh (200 < 1000). A LATER
	// dump observes it -> full 200 exported, and the entry must become live again.
	revived := d.ComputeDelta(testEntry(3200, 200, 2), d.BeginDump())
	if revived.Bytes != 200 || revived.Packets != 2 {
		t.Fatalf("port-reuse delta = %d bytes / %d pkts, want 200/2 (reset reported in full)", revived.Bytes, revived.Packets)
	}

	// Past the grace window the entry MUST survive: it is a live flow now, not a
	// tombstone.
	d.SweepTombstones(grace, now.Add(grace+time.Second))
	if d.Len() != 1 {
		t.Fatalf("len after reuse + sweep = %d, want 1 (revived flow wrongly swept)", d.Len())
	}

	// And the live flow still tracks deltas correctly (300 -> +100).
	next := d.ComputeDelta(testEntry(3200, 300, 3), d.BeginDump())
	if next.Bytes != 100 || next.Packets != 1 {
		t.Fatalf("post-reuse delta = %d bytes / %d pkts, want 100/1", next.Bytes, next.Packets)
	}
}

func TestConntrackDeltaCleanup(t *testing.T) {
	d := NewDeltaTracker()

	gen := d.BeginDump()
	old := FlowEntry{
		SrcAddr:  netip.MustParseAddr("10.0.0.1"),
		DstAddr:  netip.MustParseAddr("10.0.0.2"),
		SrcPort:  4001,
		DstPort:  80,
		Protocol: 6,
		Bytes:    100,
		Packets:  1,
		LastSeen: time.Now().Add(-2 * time.Hour),
	}
	d.ComputeDelta(old, gen)

	fresh := testEntry(4002, 200, 2)
	d.ComputeDelta(fresh, gen)

	if d.Len() != 2 {
		t.Fatalf("len before cleanup = %d, want 2", d.Len())
	}

	d.Cleanup(1 * time.Hour)

	if d.Len() != 1 {
		t.Fatalf("len after cleanup = %d, want 1", d.Len())
	}
}
