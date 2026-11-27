package server

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestReloadHistoryRingAndFiltering(t *testing.T) {
	s := &Server{}

	if got := s.ReloadHistory(0, ""); got == nil || len(got) != 0 {
		t.Fatalf("empty ReloadHistory = %#v, want non-nil empty slice", got)
	}

	const total = maxReloadHistory + 5
	for i := 1; i <= total; i++ {
		opID := "op-" + strconv.Itoa(i)
		outcome := ReloadOutcomeApplied
		errMsg := ""
		if i%2 == 0 {
			outcome = ReloadOutcomeFailed
			errMsg = "failed-" + strconv.Itoa(i)
		}
		gen := s.MarkReloadProcessedWithRecord(ReloadRecord{
			OpID:         opID,
			Trigger:      "sighup",
			Outcome:      outcome,
			StartedAt:    time.Unix(int64(i), 0).UTC(),
			FinishedAt:   time.Unix(int64(i), 0).UTC().Add(5 * time.Millisecond),
			DurationMS:   5,
			ChangedRoots: []string{"bgp"},
			TxState:      "committed",
			Plugins: []ReloadPluginStep{
				{Plugin: "bgp", VerifyMS: 1, ApplyMS: 2, Status: "committed"},
			},
			Error: errMsg,
		})
		if gen != uint64(i) {
			t.Fatalf("generation = %d, want %d", gen, i)
		}
	}

	gen, outcome, at, lastOpID := s.ReloadStatusWithOp()
	if gen != total {
		t.Fatalf("ReloadStatusWithOp gen = %d, want %d", gen, total)
	}
	if outcome != ReloadOutcomeApplied {
		t.Fatalf("ReloadStatusWithOp outcome = %q, want %q", outcome, ReloadOutcomeApplied)
	}
	if at.IsZero() {
		t.Fatal("ReloadStatusWithOp at is zero")
	}
	if lastOpID != "op-"+strconv.Itoa(total) {
		t.Fatalf("ReloadStatusWithOp opID = %q, want %q", lastOpID, "op-"+strconv.Itoa(total))
	}

	all := s.ReloadHistory(0, "")
	if len(all) != maxReloadHistory {
		t.Fatalf("ReloadHistory len = %d, want %d", len(all), maxReloadHistory)
	}
	if all[0].Generation != total {
		t.Fatalf("newest generation = %d, want %d", all[0].Generation, total)
	}
	if all[len(all)-1].Generation != total-maxReloadHistory+1 {
		t.Fatalf("oldest retained generation = %d, want %d", all[len(all)-1].Generation, total-maxReloadHistory+1)
	}

	limited := s.ReloadHistory(3, "")
	if len(limited) != 3 {
		t.Fatalf("limited len = %d, want 3", len(limited))
	}
	if limited[0].Generation != total || limited[2].Generation != total-2 {
		t.Fatalf("limited generations = [%d, %d, %d], want [%d, %d, %d]",
			limited[0].Generation, limited[1].Generation, limited[2].Generation, total, total-1, total-2)
	}

	targetOp := "op-" + strconv.Itoa(total-2)
	byOp := s.ReloadHistory(0, targetOp)
	if len(byOp) != 1 {
		t.Fatalf("filtered by %s len = %d, want 1", targetOp, len(byOp))
	}
	if byOp[0].OpID != targetOp || byOp[0].Generation != total-2 {
		t.Fatalf("filtered record = %+v, want op=%s gen=%d", byOp[0], targetOp, total-2)
	}

	// Evicted op-id returns empty non-nil slice.
	if evicted := s.ReloadHistory(0, "op-1"); evicted == nil || len(evicted) != 0 {
		t.Fatalf("evicted op-1 = %#v, want non-nil empty slice", evicted)
	}
}

func TestMarkReloadProcessedBackwardCompatibility(t *testing.T) {
	s := &Server{}
	s.MarkReloadProcessed(true)
	s.MarkReloadProcessed(false)

	gen, outcome, _, opID := s.ReloadStatusWithOp()
	if gen != 2 || outcome != ReloadOutcomeFailed || opID != "" {
		t.Fatalf("ReloadStatusWithOp = (%d, %q, %q), want (2, failed, \"\")", gen, outcome, opID)
	}
	hist := s.ReloadHistory(0, "")
	if len(hist) != 2 {
		t.Fatalf("ReloadHistory len = %d, want 2", len(hist))
	}
	if hist[0].Generation != 2 || hist[0].Outcome != ReloadOutcomeFailed {
		t.Fatalf("newest entry = %+v, want gen=2 failed", hist[0])
	}
	if hist[1].Generation != 1 || hist[1].Outcome != ReloadOutcomeApplied {
		t.Fatalf("older entry = %+v, want gen=1 applied", hist[1])
	}
}

func TestReloadTraceContextAndBounds(t *testing.T) {
	trace := &ReloadTrace{}
	ctx := WithReloadTrace(context.Background(), trace)
	if reloadTraceFrom(ctx) != trace {
		t.Fatal("reloadTraceFrom did not recover attached trace")
	}

	roots := make([]string, 0, maxReloadChangedRoots+10)
	for i := range maxReloadChangedRoots + 10 {
		roots = append(roots, "root-"+strconv.Itoa(100-i))
	}
	roots = append(roots, "root-100", "")
	trace.recordRoots(roots)
	trace.recordTx("committed", []ReloadPluginStep{{Plugin: "bgp", Status: "committed"}})

	gotRoots, txState, steps := trace.Snapshot()
	if len(gotRoots) != maxReloadChangedRoots {
		t.Fatalf("gotRoots len = %d, want %d", len(gotRoots), maxReloadChangedRoots)
	}
	if txState != "committed" || len(steps) != 1 || steps[0].Plugin != "bgp" {
		t.Fatalf("Snapshot txState=%q steps=%+v", txState, steps)
	}
}

func TestEventRingAppendAndSnapshotWithOp(t *testing.T) {
	ring := NewEventRing(8)
	ring.appendWithOp("config", "verify", "engine", "", "", "op-10")
	ring.AppendWithMeta("bgp", "update", "engine", "10.0.0.1", "in")
	ring.appendWithOp("config", "apply", "engine", "", "", "op-10")
	ring.appendWithOp("config", "verify", "engine", "", "", "op-11")

	gotOp10 := ring.SnapshotWithOp(0, "", "op-10")
	if len(gotOp10) != 2 {
		t.Fatalf("op-10 count = %d, want 2", len(gotOp10))
	}
	if gotOp10[0].EventType != "apply" || gotOp10[1].EventType != "verify" {
		t.Fatalf("op-10 order = [%s, %s], want [apply, verify]", gotOp10[0].EventType, gotOp10[1].EventType)
	}
	if gotOp10[0].OpID != "op-10" || gotOp10[1].OpID != "op-10" {
		t.Fatalf("op-10 OpID mismatch: %+v", gotOp10)
	}

	limited := ring.SnapshotWithOp(1, "config", "op-10")
	if len(limited) != 1 || limited[0].EventType != "apply" {
		t.Fatalf("limited config/op-10 = %+v, want [apply]", limited)
	}

	all := ring.Snapshot(0, "")
	if len(all) != 4 {
		t.Fatalf("Snapshot all len = %d, want 4", len(all))
	}
}
