// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

// Parallel execution constants.
const (
	DefaultParallelTimeout    = 30 * time.Second
	DefaultParallelConcurrent = 20
	StatusUpdateInterval      = 200 * time.Millisecond

	// ParallelTimeoutHeadroom widens each test's wall-clock budget when tests
	// run concurrently. Authored per-test timeouts (an explicit `timeout=` on a
	// cmd, or the baseline-derived SuggestedTimeout) are measured against an
	// uncontended run; under parallel execution tests share CPU and run slower,
	// so a budget set close to the uncontended runtime flakes under load. Many
	// .ci timeouts sit at 70-100% of the uncontended runtime, leaving no
	// headroom. Multiplying by this factor when concurrency > 1 absorbs the
	// contention; serial runs (-p 1, single-test debug) keep the tight value so
	// real slowdowns still surface quickly.
	ParallelTimeoutHeadroom = 3
)

// SuiteConcurrencyFloor is the smallest default concurrency DefaultSuiteConcurrency
// will hand out. It is the value ZE_PLUGIN_PARALLEL has been running the 530-test
// plugin suite at on GitHub's 4-vCPU hosted runner (mk/test-functional.mk), so it
// is a measured survivable figure on the smallest host this project builds on,
// not a guess.
const SuiteConcurrencyFloor = 8

// DefaultSuiteConcurrency is the per-suite default for `ze-test <suite>` when the
// operator passes no -p.
//
// It exists because "unset" and "all at once" were the same value. Every suite in
// internal/test/cli/register.go declared 0, Runner.Run turns a non-positive
// Parallel into len(selected), and so `ze-test ospf --all` launched all 97 ze
// daemons simultaneously. That is survivable on a development workstation and
// fatal on a small CI runner: on 2026-07-26 the GitHub job died mid-ospf-suite
// with the runner agent itself killed (exit 143, "the runner has received a
// shutdown signal"), after a wave of tests timed out under the thrash.
//
// Scaling with the host rather than pinning a constant keeps a big machine fast:
// at 2x CPUs a 16-core workstation still runs 32 concurrently, and any host with
// 48+ cores exceeds every suite's size, which is the old "all at once" behavior.
//
// -p 0 still means all: this is the DEFAULT, not a ceiling, so an operator who
// wants the old behavior asks for it explicitly.
func DefaultSuiteConcurrency() int {
	return max(SuiteConcurrencyFloor, 2*runtime.NumCPU())
}

var _ = env.MustRegister(env.EnvEntry{Key: "ze.verify.mode", Type: "bool", Description: "Set by the verify runner; suites emit machine-readable failure groups"})

func verifyModeEnabled() bool {
	return env.IsEnabled("ze.verify.mode")
}

// VerifyModeEnabled reports whether this run is part of a `make ze-verify`
// gate (the verify runner sets ZE_VERIFY_MODE=1). Suites use it to turn
// silent environment skips into hard failures.
func VerifyModeEnabled() bool {
	return verifyModeEnabled()
}

// parallelTest represents a test that can be run in parallel.
type parallelTest[T any] struct {
	Name   string
	Record *Record
	Test   T // The original test object
	Run    func(ctx context.Context, t T) (passed bool, err error)
}

// parallelRunner executes tests in parallel with progress display.
type parallelRunner[T any] struct {
	tests          []*parallelTest[T]
	display        *Display
	colors         *Colors
	quiet          bool
	verbose        bool
	label          string         // test suite label for header
	noHeader       bool           // if true, don't print header in Run (caller manages it)
	noSummary      bool           // if true, skip Summary/TimingDetail/DebugHints in Run
	onFail         func(T, error) // Called for each failed test (for verbose output)
	onReport       func(*Tests)   // Called after run when there are failures (for PrintAllFailures)
	baseDir        string         // project root for timing baseline persistence
	concurrency    int            // max concurrent tests; 0 means DefaultParallelConcurrent
	statusInterval time.Duration  // status ticker interval; 0 means StatusUpdateInterval
	hostLoad       *HostLoad      // snapshot at run start; suppresses baseline when contended
}

// NewParallelRunner creates a parallel test runner.
func NewParallelRunner[T any](colors *Colors) *parallelRunner[T] {
	return &parallelRunner[T]{
		colors: colors,
	}
}

// SetQuiet enables quiet mode.
func (r *parallelRunner[T]) SetQuiet(quiet bool) {
	r.quiet = quiet
}

// SetVerbose enables verbose output for failures.
func (r *parallelRunner[T]) SetVerbose(verbose bool) {
	r.verbose = verbose
}

// SetLabel sets the test suite label for the header.
func (r *parallelRunner[T]) SetLabel(label string) {
	r.label = label
}

// setNoHeader prevents Run from printing the section header.
// Use when the header is managed by the caller.
func (r *parallelRunner[T]) setNoHeader(v bool) {
	r.noHeader = v
}

// SetBaseDir sets the project root for timing baseline persistence.
func (r *parallelRunner[T]) SetBaseDir(dir string) {
	r.baseDir = dir
}

// SetOnFail sets the callback for failed tests.
// The callback receives the original test object and the error.
func (r *parallelRunner[T]) SetOnFail(fn func(T, error)) {
	r.onFail = fn
}

// SetConcurrency sets the maximum number of concurrent tests.
// Zero means DefaultParallelConcurrent.
func (r *parallelRunner[T]) SetConcurrency(n int) {
	r.concurrency = n
}

// setStatusInterval sets the status ticker interval.
// Zero means StatusUpdateInterval.
func (r *parallelRunner[T]) setStatusInterval(d time.Duration) {
	r.statusInterval = d
}

// setDisplay injects an existing Display instead of lazy-creating one.
// Use when the caller already owns a Display (e.g., .ci Runner).
func (r *parallelRunner[T]) setDisplay(d *Display) {
	r.display = d
}

// setOnReport sets a callback invoked after run when there are failures.
// Use for .ci's PrintAllFailures.
func (r *parallelRunner[T]) setOnReport(fn func(*Tests)) {
	r.onReport = fn
}

// setNoSummary suppresses Summary/TimingDetail/DebugHints in Run.
// Use for stress-mode iterations where the caller controls post-run output.
func (r *parallelRunner[T]) setNoSummary(v bool) {
	r.noSummary = v
}

// setHostLoad records the host load snapshot taken at run start.
// When contended, timing baseline updates are suppressed and failure
// groups include the load context.
func (r *parallelRunner[T]) setHostLoad(h *HostLoad) {
	r.hostLoad = h
}

// addRecord adds a test with a pre-existing Record. The Record must already
// be registered in the Display's Tests. Use when the caller owns the Records
// (e.g., .ci Runner delegates scheduling but keeps its own Record set).
func (r *parallelRunner[T]) addRecord(rec *Record, test T, runFn func(ctx context.Context, t T) (bool, error)) {
	r.tests = append(r.tests, &parallelTest[T]{
		Name:   rec.Name,
		Record: rec,
		Test:   test,
		Run:    runFn,
	})
}

// AddTest adds a test to the runner.
func (r *parallelRunner[T]) AddTest(name string, test T, runFn func(ctx context.Context, t T) (bool, error)) *Record {
	return r.addTest(name, "", test, runFn)
}

// AddTestWithNick adds a test to the runner with a stable caller-supplied nick.
func (r *parallelRunner[T]) AddTestWithNick(name, nick string, test T, runFn func(ctx context.Context, t T) (bool, error)) *Record {
	return r.addTest(name, nick, test, runFn)
}

func (r *parallelRunner[T]) addTest(name, nick string, test T, runFn func(ctx context.Context, t T) (bool, error)) *Record {
	if r.display == nil {
		// Lazy init - create Tests container and Display on first AddTest
		tests := NewTests()
		r.display = NewDisplay(tests, r.colors)
		r.display.SetQuiet(r.quiet)
		r.display.SetTimeout(DefaultParallelTimeout)
	}

	rec := r.display.tests.addWithNick(name, nick)
	rec.Active = true

	r.tests = append(r.tests, &parallelTest[T]{
		Name:   name,
		Record: rec,
		Test:   test,
		Run:    runFn,
	})

	return rec
}

// Run executes all tests in parallel and returns success.
func (r *parallelRunner[T]) Run(ctx context.Context) bool {
	if len(r.tests) == 0 {
		return true
	}

	// Configure display
	if r.label != "" {
		r.display.SetLabel(r.label)
		if !r.noHeader {
			r.display.Header()
		}
	}
	conc := r.concurrency
	if conc <= 0 {
		conc = DefaultParallelConcurrent
	}
	r.display.SetParallel(conc, len(r.tests))
	r.display.Start()

	// Channels
	type result struct {
		test   *parallelTest[T]
		passed bool
		err    error
	}
	results := make(chan result, len(r.tests))
	done := make(chan struct{})

	// Run tests in parallel with semaphore
	var wg sync.WaitGroup
	sem := make(chan struct{}, conc)

	// option=exclusive:group=<name>: one lock per group, so members of a group
	// never overlap each other while unrelated tests keep running concurrently.
	// Built up-front (not lazily under a mutex) because r.tests is fixed here.
	exclusive := make(map[string]chan struct{})
	for _, t := range r.tests {
		if g := t.Record.ExclusiveGroup; g != "" && exclusive[g] == nil {
			exclusive[g] = make(chan struct{}, 1)
		}
	}

	for _, test := range r.tests {
		wg.Add(1)
		go func(t *parallelTest[T]) {
			defer wg.Done()

			// Take the group lock BEFORE the concurrency semaphore. Reversing these
			// would let members of one group occupy every semaphore slot while
			// blocked on each other, starving unrelated tests -- with a group of 8
			// and -p 4 that stalls the suite instead of just serializing the group.
			// Only ever one lock is held at a time, so there is no lock-order cycle.
			if lock := exclusive[t.Record.ExclusiveGroup]; lock != nil {
				lock <- struct{}{}
				defer func() { <-lock }()
			}

			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			// option=skip-os matches the current GOOS: mark skip without
			// running. Keeps the signal meaningful (feature is stubbed on
			// this OS, not "it regressed") -- see rules/os-specific-tests.md.
			if t.Record.SkipReason != "" {
				t.Record.State = StateSkip
				results <- result{test: t, passed: true, err: nil}
				return
			}

			t.Record.State = StateRunning
			t.Record.StartTime = time.Now()

			passed, err := t.Run(ctx, t.Test)
			t.Record.Duration = time.Since(t.Record.StartTime)

			// Respect terminal states set by the Run function (e.g.,
			// StateTimeout from .ci's runTest). Only set Success/Fail
			// when the state is still Running.
			switch t.Record.State {
			case StateSuccess, StateFail, StateTimeout, StateSkip:
				// already terminal
			default:
				if passed {
					t.Record.State = StateSuccess
				} else {
					t.Record.State = StateFail
					t.Record.Error = err
				}
			}

			results <- result{test: t, passed: passed, err: err}
		}(test)
	}

	// Close results when all done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Periodic status update with context cancellation support
	interval := r.statusInterval
	if interval <= 0 {
		interval = StatusUpdateInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.display.Status()
			}
		}
	}()

	// Collect results
	type failure struct {
		test T
		err  error
	}
	var failures []failure
	for res := range results {
		if !res.passed {
			failures = append(failures, failure{test: res.test.Test, err: res.err})
		}
		r.display.TestFinished(res.test.Record.Nick, res.test.Record.State, res.test.Record.Duration)
		r.display.Status()
	}

	close(done)
	r.display.Newline()

	if !r.noSummary {
		r.display.Summary()
	}

	// Record and display timing baseline.
	// Skip timed-out tests -- their duration is the kill time, not actual runtime.
	// Skip ALL recording when the run is contended to prevent baseline pollution.
	contended := r.hostLoad != nil && r.hostLoad.Contended()
	if r.baseDir != "" && r.label != "" {
		timings := LoadTimings(r.baseDir)
		if !contended {
			for _, t := range r.tests {
				if t.Record.Duration > 0 && t.Record.State != StateTimeout {
					timings.Record(r.label, t.Name, t.Record.Duration)
				}
			}
		}
		if !r.noSummary {
			r.display.timingDetail(r.label, timings)
		}
		if !contended {
			if err := timings.Save(r.baseDir); err != nil {
				logger().Warn("save timings failed", "error", err)
			}
		}
	}
	if !r.quiet && len(failures) > 0 {
		if verifyModeEnabled() {
			report := newReport(r.colors)
			report.SetOutput(r.display.output)
			report.SetLabel(r.label)
			report.setHostLoad(r.hostLoad)
			report.printFailureGroups(r.display.tests)
		}
		if r.onReport != nil {
			r.onReport(r.display.tests)
		}
	}

	if !r.noSummary {
		r.display.debugHints()
	}

	// Verify mode must include concise failure detail in saved logs without
	// making normal interactive runs verbose.
	if (r.verbose || verifyModeEnabled()) && r.onFail != nil && len(failures) > 0 {
		for _, f := range failures {
			r.onFail(f.test, f.err)
		}
	}

	_, failed, timedOut, _ := r.display.tests.Summary()
	return failed == 0 && timedOut == 0
}
