package trend

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/stats"
	"github.com/ze-software/ze/internal/test/sim"
)

// trendTestStart is the fake clock's start for every monitor test.
var trendTestStart = time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)

// trendTestWait bounds how long a test waits, in real time, for the monitor's
// goroutine to be scheduled. Nothing waits on it unless the monitor is broken.
const trendTestWait = 10 * time.Second

// isolateTrend gives the test a clean report bus and, when rules is not nil,
// a registry holding only rules. Both are restored in t.Cleanup.
func isolateTrend(t *testing.T, rules []rule) {
	t.Helper()
	report.ResetForTest()
	ruleRegistry.Lock()
	saved := ruleRegistry.list
	if rules != nil {
		ruleRegistry.list = append([]rule(nil), rules...)
	} else {
		ruleRegistry.list = append([]rule(nil), saved...)
	}
	ruleRegistry.Unlock()
	t.Cleanup(func() {
		ruleRegistry.Lock()
		ruleRegistry.list = saved
		ruleRegistry.Unlock()
		report.ResetForTest()
	})
}

// trendRun drives a started monitor through its fake clock.
type trendRun struct {
	t       *testing.T
	clock   *sim.FakeClock
	monitor *Monitor
}

// startTrend starts a monitor over detector on a fake clock, stops it in
// t.Cleanup, and waits until its ticker exists.
func startTrend(t *testing.T, detector *host.Detector) *trendRun {
	t.Helper()
	clk := sim.NewFakeClock(trendTestStart)
	monitor, err := Start(clk, detector)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(monitor.Stop)
	if !clk.AwaitTickers(1, trendTestWait) {
		t.Fatal("the monitor created no ticker")
	}
	return &trendRun{t: t, clock: clk, monitor: monitor}
}

// step moves the fake clock by d, fires the monitor's ticker, and waits until
// the monitor published the snapshot of that tick. Waiting on the publish,
// not on time, keeps every step deterministic, and no tick is fired while
// the previous one is pending.
func (r *trendRun) step(d time.Duration) {
	r.t.Helper()
	r.clock.Add(d)
	at := r.clock.Now()
	r.clock.FireTickers()
	deadline := time.After(trendTestWait)
	for {
		r.monitor.mu.RLock()
		sampled := r.monitor.sampledAt
		changed := r.monitor.changed
		r.monitor.mu.RUnlock()
		if !sampled.Before(at) {
			return
		}
		select {
		case <-changed:
		case <-deadline:
			r.t.Fatalf("the monitor did not publish the tick at %s", at)
		}
	}
}

// statusNamed answers the published status of the rule called name.
func statusNamed(t *testing.T, name string) RuleStatus {
	t.Helper()
	statuses, err := Rules()
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	for i := range statuses {
		if statuses[i].Name() == name {
			return statuses[i]
		}
	}
	t.Fatalf("rule %s is not listed", name)
	return RuleStatus{}
}

// warningOf answers the bus warning under (host, code, subject), if any.
func warningOf(code, subject string) (report.Issue, bool) {
	for _, issue := range report.Warnings() {
		if issue.Source == reportSource && issue.Code == code && issue.Subject == subject {
			return issue, true
		}
	}
	return report.Issue{}, false
}

// testRule builds a valid rule for a test: one bucket per sample, so each step
// closes a bucket and the run loop judges at every sample.
func testRule(name, code string, read func(*host.Detector) reading) rule {
	return rule{
		name: name, code: code, unit: UnitBytes,
		samplePeriod: 10 * time.Second,
		history: stats.LevelParams{BucketPeriod: 10 * time.Second, BucketCount: 8,
			GrowthRises: 1, GrowthHorizon: time.Hour},
		confirmAfter: 1, clearAfter: 1,
		read:     read,
		describe: func(finding) string { return "the test level keeps growing" },
		fix:      trendFix,
	}
}

// risingReader answers a level that grows by 10 at each read, against a limit
// of 1000, and counts its reads.
func risingReader(reads *atomic.Int64) func(*host.Detector) reading {
	return func(*host.Detector) reading {
		n := reads.Add(1)
		sample, err := newLevelReading(float64(10*n), 1000, 1)
		if err != nil {
			return unreadableReading{source: "test", err: err}
		}
		return sample
	}
}

// VALIDATES: AC-7 -- a rule registered from its own file, with no other edit,
// is sampled by the monitor on its own period and listed in the published
// state beside the built-in rule.
// Method: the real registry plus a test rule registered through registerRule,
// restored in t.Cleanup; the fake clock steps the test rule's period, which is
// shorter than the kernel-memory rule's.
func TestRuleRegisteredIsSampledAndListed(t *testing.T) {
	isolateTrend(t, nil)
	var reads atomic.Int64
	registerRule(testRule("test-registered", "doctor-test-registered", risingReader(&reads)))
	run := startTrend(t, &host.Detector{Root: t.TempDir()})

	statuses, err := Rules()
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	names := make([]string, 0, len(statuses))
	for _, status := range statuses {
		names = append(names, status.Name())
	}
	if got := strings.Join(names, ","); got != kernelMemoryRuleName+",test-registered" {
		t.Fatalf("listed rules = %s, want %s,test-registered", got, kernelMemoryRuleName)
	}
	if got := reads.Load(); got != 1 {
		t.Fatalf("reads after Start = %d, want 1: Start reads every rule once", got)
	}
	for range 3 {
		run.step(10 * time.Second)
	}
	if got := reads.Load(); got != 4 {
		t.Fatalf("reads after three 10s steps = %d, want 4: the rule is sampled on its own period", got)
	}
	level, limit, ok := statusNamed(t, "test-registered").Reading()
	if !ok || level != 40 || limit != 1000 {
		t.Fatalf("published reading = %v/%v (ok %v), want 40/1000", level, limit, ok)
	}
}

// VALIDATES: AC-8 -- a reader error and "not present" are distinct states. An
// error makes the rule unreadable, raises the unreadable warning and gives the
// monitor check a row naming the path and the error; "not present" reports
// nothing anywhere.
// Method: two test rules, one answering absentReading and one answering an
// unreadableReading; Start reads both once.
func TestReaderErrorAndNotPresentAreDistinctStates(t *testing.T) {
	failure := errors.New("permission denied")
	isolateTrend(t, []rule{
		testRule("test-absent", "doctor-test-absent", func(*host.Detector) reading { return absentReading{} }),
		testRule("test-failing", "doctor-test-failing", func(*host.Detector) reading {
			return unreadableReading{source: "/proc/test-counter", err: failure}
		}),
	})
	startTrend(t, &host.Detector{Root: t.TempDir()})

	absent := statusNamed(t, "test-absent")
	if absent.State() != StateNotPresent {
		t.Errorf("absent rule state = %s, want not-present", absent.State())
	}
	if _, _, failed := absent.Failure(); failed {
		t.Error("absent rule reports a failure")
	}
	if _, _, ok := absent.Reading(); ok {
		t.Error("absent rule reports a reading")
	}
	if _, raised := warningOf(CodeUnreadable, "test-absent"); raised {
		t.Error("absent rule raised the unreadable warning")
	}

	failing := statusNamed(t, "test-failing")
	if failing.State() != StateUnreadable {
		t.Errorf("failing rule state = %s, want unreadable", failing.State())
	}
	source, cause, failed := failing.Failure()
	if !failed || source != "/proc/test-counter" || cause != "permission denied" {
		t.Errorf("failure = %q %q (failed %v), want /proc/test-counter, permission denied", source, cause, failed)
	}
	issue, raised := warningOf(CodeUnreadable, "test-failing")
	if !raised {
		t.Fatal("failing rule raised no unreadable warning")
	}
	if issue.Fix != trendFix || !strings.Contains(issue.Message, "/proc/test-counter: permission denied") {
		t.Errorf("unreadable warning = %q fix %q", issue.Message, issue.Fix)
	}

	rows := checkMonitor(diagnostic.DoctorCheckContext{})
	if len(rows) != 1 {
		t.Fatalf("monitor check rows = %d, want 1 (the failing rule only): %+v", len(rows), rows)
	}
	row := rows[0]
	if row.Code != CodeUnreadable || row.Severity != diagnostic.SeverityWarning || row.Fix != trendFix ||
		!strings.Contains(row.Message, "test-failing") ||
		!strings.Contains(row.Message, "/proc/test-counter: permission denied") {
		t.Errorf("monitor check row = %+v", row)
	}
}

// VALIDATES: AC-11 -- Stop ends the monitor, unpublishes it and clears the
// monitor's own warnings, key by key, and leaves another warning of source
// host (a carrier change) on the bus.
// Method: a carrier warning raised first; a rising test rule that confirms
// after one judgment, and a failing one, so the monitor holds both kinds of
// its own key before Stop.
func TestTrendMonitorStopClearsOnlyItsOwnWarnings(t *testing.T) {
	var reads atomic.Int64
	isolateTrend(t, []rule{
		testRule("test-rising", "doctor-test-rising", risingReader(&reads)),
		testRule("test-failing", "doctor-test-failing", func(*host.Detector) reading {
			return unreadableReading{source: "/proc/test-counter", err: errors.New("permission denied")}
		}),
	})
	report.RaiseWarning(reportSource, "carrier-change", "eth0", "eth0 lost its carrier")
	run := startTrend(t, &host.Detector{Root: t.TempDir()})
	run.step(10 * time.Second)
	run.step(10 * time.Second)
	if _, raised := warningOf("doctor-test-rising", "test-rising"); !raised {
		t.Fatal("the rising rule raised no warning before Stop")
	}
	if _, raised := warningOf(CodeUnreadable, "test-failing"); !raised {
		t.Fatal("the failing rule raised no unreadable warning before Stop")
	}

	run.monitor.Stop()

	if _, raised := warningOf("doctor-test-rising", "test-rising"); raised {
		t.Error("Stop left the rising rule's warning")
	}
	if _, raised := warningOf(CodeUnreadable, "test-failing"); raised {
		t.Error("Stop left the unreadable warning")
	}
	if _, raised := warningOf("carrier-change", "eth0"); !raised {
		t.Error("Stop cleared the carrier warning, which is not the monitor's")
	}
	if _, err := Rules(); !errors.Is(err, ErrNotRunning) {
		t.Errorf("Rules after Stop = %v, want ErrNotRunning", err)
	}
}
