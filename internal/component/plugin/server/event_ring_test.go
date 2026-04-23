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
