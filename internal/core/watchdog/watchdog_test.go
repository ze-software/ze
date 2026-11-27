package watchdog

import (
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/flightrec"
	"github.com/ze-software/ze/internal/core/report"
)

func cleanupWatchdog(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		Stop()
		resetForTest()
		report.ResetForTest()
		flightrec.ResetForTest()
	})
}

// VALIDATES: AC-6 -- Start and Stop are idempotent, increment ticks while running, and cleanly join the worker goroutine on Stop.
// PREVENTS: leaked watchdog goroutines or double-start panics.
func TestWatchdogStartStopLifecycle(t *testing.T) {
	cleanupWatchdog(t)
	resetForTest()

	snap := Status()
	if snap.Running {
		t.Fatal("fresh watchdog reported Running=true")
	}
	if snap.Probes == nil {
		t.Error("Status().Probes is nil, want non-nil empty slice")
	}
	if snap.RecentStalls == nil {
		t.Error("Status().RecentStalls is nil, want non-nil empty slice")
	}

	// Default Start/Stop idempotency.
	Start()
	Start()
	if !Status().Running {
		t.Fatal("Status().Running = false after Start()")
	}
	Stop()
	Stop()
	if Status().Running {
		t.Fatal("Status().Running = true after Stop()")
	}

	// Fast ticker verifies ticks advance and worker stops cleanly.
	var tickCount atomic.Int32
	reached := make(chan struct{})
	unreg := RegisterProbe("heartbeat", func() (bool, string, time.Duration) {
		if tickCount.Add(1) == 2 {
			close(reached)
		}
		return false, "", 0
	})
	defer unreg()

	startWithConfig(5*time.Millisecond, 2*time.Second, 0)
	select {
	case <-reached:
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog did not execute 2 ticks within 2s")
	}

	Stop()
	after := Status()
	if after.Running {
		t.Error("Status().Running = true after Stop()")
	}
	if after.Ticks < 2 {
		t.Errorf("Status().Ticks = %d, want >= 2", after.Ticks)
	}
	if after.LastTickAt.IsZero() {
		t.Error("Status().LastTickAt is zero after ticks")
	}
}

// VALIDATES: AC-6 -- RegisterProbe sorts probes by name, ignores invalid inputs, and scopes unregister to its own registration ID.
// PREVENTS: stale unregister functions deleting a newer probe with the same name.
func TestWatchdogRegisterProbeAndUnregister(t *testing.T) {
	cleanupWatchdog(t)
	resetForTest()

	noop1 := RegisterProbe("", func() (bool, string, time.Duration) { return false, "", 0 })
	noop2 := RegisterProbe("nil-fn", nil)
	noop1()
	noop2()
	if got := len(Status().Probes); got != 0 {
		t.Fatalf("invalid probes registered: count = %d, want 0", got)
	}

	unregBeta := RegisterProbe("beta", func() (bool, string, time.Duration) { return false, "", 0 })
	unregAlpha1 := RegisterProbe("alpha", func() (bool, string, time.Duration) { return false, "", 0 })

	probes := Status().Probes
	if len(probes) != 2 {
		t.Fatalf("probe count = %d, want 2", len(probes))
	}
	if probes[0].Name != "alpha" || probes[1].Name != "beta" {
		t.Errorf("probes not sorted: got [%q, %q], want [alpha, beta]", probes[0].Name, probes[1].Name)
	}

	// Re-registering "alpha" gives a new ID; calling the old unregister must not delete it.
	unregAlpha2 := RegisterProbe("alpha", func() (bool, string, time.Duration) { return false, "", 0 })
	unregAlpha1()
	unregAlpha1() // idempotent
	if len(Status().Probes) != 2 {
		t.Fatalf("old unregister removed newer probe: count = %d, want 2", len(Status().Probes))
	}

	unregAlpha2()
	unregBeta()
	if got := len(Status().Probes); got != 0 {
		t.Fatalf("probes remain after unregister: count = %d, want 0", got)
	}
}

// VALIDATES: AC-6 -- an unhealthy probe captures bounded goroutine stacks, triggers flightrec, raises CodeDoctorDaemonStallDetected, and respects cooldown.
// PREVENTS: unthrottled stall captures or missing diagnostic evidence when a subsystem wedges.
func TestWatchdogStallCaptureCooldownAndReport(t *testing.T) {
	cleanupWatchdog(t)
	resetForTest()
	report.ResetForTest()
	flightrec.ResetForTest()

	if err := flightrec.Start(); err != nil {
		t.Fatalf("flightrec.Start: %v", err)
	}

	var calls atomic.Int32
	done := make(chan struct{})
	const targetProbe = "config-tx-lock"
	unreg := RegisterProbe(targetProbe, func() (bool, string, time.Duration) {
		if calls.Add(1) == 3 {
			close(done)
		}
		return true, "txLock held past threshold", 46 * time.Second
	})
	defer unreg()

	startWithConfig(5*time.Millisecond, 5*time.Second, time.Hour)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog did not run 3 stalled ticks within 2s")
	}
	Stop()

	snap := Status()
	if snap.StallCount != 1 {
		t.Fatalf("StallCount = %d, want 1 (cooldown must throttle repeats)", snap.StallCount)
	}
	if len(snap.RecentStalls) != 1 {
		t.Fatalf("len(RecentStalls) = %d, want 1", len(snap.RecentStalls))
	}
	rec := snap.RecentStalls[0]
	if rec.Probe != targetProbe {
		t.Errorf("StallRecord.Probe = %q, want %q", rec.Probe, targetProbe)
	}
	if rec.Detail != "txLock held past threshold" {
		t.Errorf("StallRecord.Detail = %q, want %q", rec.Detail, "txLock held past threshold")
	}
	if rec.StallMS != 46000 {
		t.Errorf("StallRecord.StallMS = %d, want 46000", rec.StallMS)
	}
	if rec.GoroutineCount <= 0 {
		t.Errorf("StallRecord.GoroutineCount = %d, want > 0", rec.GoroutineCount)
	}
	if rec.Stacks == "" {
		t.Error("StallRecord.Stacks is empty")
	}
	if len(rec.Stacks) > maxStackBytes {
		t.Errorf("len(StallRecord.Stacks) = %d, exceeds %d", len(rec.Stacks), maxStackBytes)
	}
	if rec.TraceSnapshotID == "" {
		t.Error("StallRecord.TraceSnapshotID is empty when flightrec is enabled")
	} else if _, ok := flightrec.SnapshotByID(rec.TraceSnapshotID); !ok {
		t.Errorf("flightrec.SnapshotByID(%q) not found", rec.TraceSnapshotID)
	}

	if len(snap.Probes) != 1 || !snap.Probes[0].Stalled || snap.Probes[0].StallMS != 46000 {
		t.Errorf("ProbeState = %+v, want stalled with 46000ms", snap.Probes)
	}

	errs := report.Errors(0)
	found := false
	for _, issue := range errs {
		if issue.Source == reportSource && issue.Code == CodeDoctorDaemonStallDetected && issue.Subject == targetProbe {
			found = true
			if issue.Fix != reportFix {
				t.Errorf("report Issue.Fix = %q, want %q", issue.Fix, reportFix)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected report error %q for probe %q, got %+v", CodeDoctorDaemonStallDetected, targetProbe, errs)
	}
}

// VALIDATES: AC-6 -- the 8-slot stall ring evicts oldest entries and returns the newest 8 newest-first.
// PREVENTS: unbounded memory growth under repeated stalls.
func TestWatchdogStallRingEviction(t *testing.T) {
	cleanupWatchdog(t)
	resetForTest()
	report.ResetForTest()

	const totalStalls = 12
	var seq atomic.Int32
	done := make(chan struct{})
	unreg := RegisterProbe("evict-probe", func() (bool, string, time.Duration) {
		n := seq.Add(1)
		if n > totalStalls {
			return false, "", 0
		}
		if n == totalStalls {
			close(done)
		}
		return true, "stall-" + strconv.Itoa(int(n)), time.Second
	})
	defer unreg()

	startWithConfig(5*time.Millisecond, 2*time.Second, 0)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("watchdog did not capture 12 stalls within 3s")
	}
	Stop()

	snap := Status()
	if snap.StallCount != totalStalls {
		t.Fatalf("StallCount = %d, want %d", snap.StallCount, totalStalls)
	}
	if len(snap.RecentStalls) != maxStallRecords {
		t.Fatalf("len(RecentStalls) = %d, want %d", len(snap.RecentStalls), maxStallRecords)
	}
	if snap.RecentStalls[0].Detail != "stall-12" {
		t.Errorf("newest stall = %q, want stall-12", snap.RecentStalls[0].Detail)
	}
	if snap.RecentStalls[maxStallRecords-1].Detail != "stall-5" {
		t.Errorf("oldest retained stall = %q, want stall-5", snap.RecentStalls[maxStallRecords-1].Detail)
	}
}

// VALIDATES: AC-7 -- CodeDoctorDaemonStallDetected registers both CodeMeta and EventCode at package init.
// PREVENTS: ze show diagnostic or doctor-coverage failing for daemon stall events.
func TestWatchdogDoctorCodeRegistered(t *testing.T) {
	meta := diagnostic.Lookup(CodeDoctorDaemonStallDetected)
	if meta == nil {
		t.Fatalf("diagnostic.Lookup(%q) = nil", CodeDoctorDaemonStallDetected)
	}
	if meta.Title == "" || meta.Description == "" || len(meta.Examples) == 0 {
		t.Errorf("incomplete CodeMeta for %q: %+v", CodeDoctorDaemonStallDetected, meta)
	}
	if !slices.Contains(diagnostic.EventCodes(), CodeDoctorDaemonStallDetected) {
		t.Errorf("%q not registered in diagnostic.EventCodes()", CodeDoctorDaemonStallDetected)
	}
}
