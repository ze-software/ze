package server

import (
	"testing"
)

func TestGlobalEventRingAppend(t *testing.T) {
	r := NewEventRing(10)
	r.Append("bgp", "state")
	r.Append("l2tp", "session-up")
	r.Append("bgp", "update")

	snap := r.Snapshot(0, "")
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
	if snap[0].Namespace != "bgp" || snap[0].EventType != "update" {
		t.Errorf("newest = %s/%s, want bgp/update", snap[0].Namespace, snap[0].EventType)
	}
	if snap[2].Namespace != "bgp" || snap[2].EventType != "state" {
		t.Errorf("oldest = %s/%s, want bgp/state", snap[2].Namespace, snap[2].EventType)
	}
}

func TestGlobalEventRingOverflow(t *testing.T) {
	r := NewEventRing(3)
	for i := range 5 {
		r.Append("ns", string(rune('a'+i)))
	}
	snap := r.Snapshot(0, "")
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
	if snap[0].EventType != "e" {
		t.Errorf("newest = %s, want e", snap[0].EventType)
	}
	if snap[2].EventType != "c" {
		t.Errorf("oldest = %s, want c", snap[2].EventType)
	}
}

func TestGlobalEventRingFilterNamespace(t *testing.T) {
	r := NewEventRing(10)
	r.Append("bgp", "state")
	r.Append("l2tp", "session-up")
	r.Append("bgp", "update")
	r.Append("l2tp", "tunnel-up")

	bgp := r.Snapshot(0, "bgp")
	if len(bgp) != 2 {
		t.Fatalf("bgp count = %d, want 2", len(bgp))
	}
	for _, rec := range bgp {
		if rec.Namespace != "bgp" {
			t.Errorf("non-bgp event in filtered result: %s", rec.Namespace)
		}
	}

	l2tp := r.Snapshot(0, "l2tp")
	if len(l2tp) != 2 {
		t.Fatalf("l2tp count = %d, want 2", len(l2tp))
	}
}

func TestGlobalEventRingFilterCount(t *testing.T) {
	r := NewEventRing(10)
	for range 8 {
		r.Append("ns", "ev")
	}

	snap := r.Snapshot(3, "")
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
}

func TestGlobalEventRingEmpty(t *testing.T) {
	r := NewEventRing(10)
	snap := r.Snapshot(0, "")
	if snap == nil {
		t.Fatal("snapshot should be non-nil empty slice")
	}
	if len(snap) != 0 {
		t.Fatalf("count = %d, want 0", len(snap))
	}
}

func TestGlobalEventRingNamespaceCounts(t *testing.T) {
	r := NewEventRing(10)
	r.Append("bgp", "state")
	r.Append("bgp", "update")
	r.Append("l2tp", "session-up")

	counts := r.NamespaceCounts()
	if counts["bgp"] != 2 {
		t.Errorf("bgp = %d, want 2", counts["bgp"])
	}
	if counts["l2tp"] != 1 {
		t.Errorf("l2tp = %d, want 1", counts["l2tp"])
	}
}

// TestEventRingAppendWithMeta verifies that AppendWithMeta stores Source,
// Subject, and Direction on each EventRecord, preserves Append and SetOnAppend
// compatibility, and performs zero allocations on append.
//
// VALIDATES: AC-6 -- EventRing.AppendWithMeta retains Source, Subject, and
// Direction in newest-first order, delivers them to SetOnAppend, keeps Append
// compatible, and allocates 0 times per append.
// PREVENTS: emitter, peer subject, or direction metadata being dropped or
// introducing heap allocations on the event emit path.
func TestEventRingAppendWithMeta(t *testing.T) {
	r := NewEventRing(10)
	var observed EventRecord
	r.SetOnAppend(func(rec EventRecord) {
		observed = rec
	})

	r.Append("bgp", "open")
	r.AppendWithMeta("bgp", "update", "engine", "192.0.2.1", "receive")
	r.AppendWithMeta("bgp", "state", "bgp-peer", "192.0.2.2", "send")

	if observed.Source != "bgp-peer" || observed.Subject != "192.0.2.2" || observed.Direction != "send" {
		t.Errorf("SetOnAppend saw {Source:%q Subject:%q Direction:%q}, want {bgp-peer 192.0.2.2 send}",
			observed.Source, observed.Subject, observed.Direction)
	}

	snap := r.Snapshot(0, "")
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
	if snap[0].Source != "bgp-peer" || snap[0].Subject != "192.0.2.2" || snap[0].Direction != "send" {
		t.Errorf("newest record metadata = {Source:%q Subject:%q Direction:%q}, want {bgp-peer 192.0.2.2 send}",
			snap[0].Source, snap[0].Subject, snap[0].Direction)
	}
	if snap[1].Source != "engine" || snap[1].Subject != "192.0.2.1" || snap[1].Direction != "receive" {
		t.Errorf("middle record metadata = {Source:%q Subject:%q Direction:%q}, want {engine 192.0.2.1 receive}",
			snap[1].Source, snap[1].Subject, snap[1].Direction)
	}
	if snap[2].Source != "" || snap[2].Subject != "" || snap[2].Direction != "" {
		t.Errorf("Append compatibility record metadata = {Source:%q Subject:%q Direction:%q}, want empty",
			snap[2].Source, snap[2].Subject, snap[2].Direction)
	}

	r.SetOnAppend(nil)
	allocs := testing.AllocsPerRun(100, func() {
		r.AppendWithMeta("bgp", "update", "engine", "192.0.2.1", "receive")
	})
	if allocs != 0 {
		t.Errorf("AppendWithMeta allocated %.2f times per run, want 0", allocs)
	}
}
