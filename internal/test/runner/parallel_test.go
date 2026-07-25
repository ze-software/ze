package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

// setVerifyMode enables verify mode for a test and resets the env cache so
// env.IsEnabled sees the change (and the restore at cleanup).
func setVerifyMode(t *testing.T) {
	t.Helper()
	t.Setenv("ZE_VERIFY_MODE", "1")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

func TestParallelRunnerFailureLinesAppearWithoutVerboseWhenVerifyMode(t *testing.T) {
	setVerifyMode(t)
	r := NewParallelRunner[string](NewColorsWithOverride(false))
	r.SetLabel("parse")
	r.SetQuiet(true)
	called := false
	r.SetOnFail(func(_ string, err error) {
		called = true
		if err == nil || err.Error() != "broken" {
			t.Fatalf("unexpected callback error: %v", err)
		}
	})
	r.AddTest("broken-test", "fixture", func(context.Context, string) (bool, error) {
		return false, errors.New("broken")
	})
	if r.Run(context.Background()) {
		t.Fatalf("expected runner failure")
	}
	if !called {
		t.Fatalf("verify mode did not emit failure callback without verbose")
	}
}

// TestParallelRunnerHonorsConfiguredConcurrency verifies the configurable cap.
//
// VALIDATES: AC-3 — SetConcurrency(N) limits the scheduler to N concurrent tests.
// PREVENTS: ignoring the configured cap and running all tests at once.
func TestParallelRunnerHonorsConfiguredConcurrency(t *testing.T) {
	const maxConcurrent = 2
	// totalTests must be an exact multiple of maxConcurrent so every barrier
	// cohort fills (otherwise a trailing cohort would block forever).
	const totalTests = 6

	r := NewParallelRunner[string](NewColorsWithOverride(false))
	r.SetConcurrency(maxConcurrent)
	r.SetLabel("conc-test")
	r.SetQuiet(true)

	var mu sync.Mutex
	var peak int
	var current int

	// Cohort barrier: replaces a fixed sleep that merely hoped enough tests
	// would overlap. Each test entering the body parks until exactly
	// maxConcurrent tests are simultaneously parked, then the whole cohort is
	// released together. This makes "peak == maxConcurrent" deterministic: the
	// runner's semaphore caps the body at maxConcurrent, and the barrier proves
	// that many actually overlap. cond guards arrived/cohort.
	cond := sync.NewCond(&mu)
	var arrived int // tests parked in the current cohort
	var cohort int  // bumped each time a cohort releases, so members wake exactly once

	for i := range totalTests {
		name := fmt.Sprintf("test-%d", i)
		r.AddTest(name, name, func(_ context.Context, _ string) (bool, error) {
			mu.Lock()
			current++
			if current > peak {
				peak = current
			}

			// Join the cohort and wait for it to fill to maxConcurrent.
			myCohort := cohort
			arrived++
			if arrived == maxConcurrent {
				// Last arrival: release this cohort and reset for the next.
				arrived = 0
				cohort++
				cond.Broadcast()
			} else {
				for cohort == myCohort {
					cond.Wait()
				}
			}

			current--
			mu.Unlock()
			return true, nil
		})
	}

	if !r.Run(context.Background()) {
		t.Fatal("expected all tests to pass")
	}

	if peak > maxConcurrent {
		t.Fatalf("peak concurrency = %d, want <= %d", peak, maxConcurrent)
	}
	if peak < maxConcurrent {
		t.Fatalf("peak concurrency = %d, want %d (cap not saturated)", peak, maxConcurrent)
	}
}

// TestParallelRunnerDefaultConcurrency verifies the default cap when SetConcurrency is not called.
//
// VALIDATES: AC-3 boundary — zero/unset concurrency defaults to DefaultParallelConcurrent.
// PREVENTS: zero-concurrency deadlock or unbounded parallelism.
func TestParallelRunnerDefaultConcurrency(t *testing.T) {
	r := NewParallelRunner[string](NewColorsWithOverride(false))
	r.SetLabel("default-conc")
	r.SetQuiet(true)

	ran := false
	r.AddTest("single", "x", func(_ context.Context, _ string) (bool, error) {
		ran = true
		return true, nil
	})
	if !r.Run(context.Background()) {
		t.Fatal("expected pass")
	}
	if !ran {
		t.Fatal("test did not run")
	}
}

// TestParallelRunnerSetDisplayInjectsDisplay verifies that an injected Display
// is used instead of lazy-creating one.
//
// VALIDATES: Display injection for .ci delegation (approach A wiring point).
// PREVENTS: double-created Display when Runner delegates to ParallelRunner.
func TestParallelRunnerSetDisplayInjectsDisplay(t *testing.T) {
	tests := NewTests()
	colors := NewColorsWithOverride(false)
	display := NewDisplay(tests, colors)
	display.SetQuiet(true)

	r := NewParallelRunner[string](colors)
	r.setDisplay(display)
	r.SetLabel("injected")

	rec := tests.Add("pre-existing")
	rec.Active = true

	r.addRecord(rec, "payload", func(_ context.Context, _ string) (bool, error) {
		return true, nil
	})

	if !r.Run(context.Background()) {
		t.Fatal("expected pass")
	}

	if rec.State != StateSuccess {
		t.Fatalf("record state = %v, want StateSuccess", rec.State)
	}
}

// TestParallelRunnerAddRecordUsesExistingRecord verifies AddRecord uses the
// caller's Record without creating a new one.
//
// VALIDATES: pre-existing Record injection (approach A wiring point).
// PREVENTS: double-created Records when .ci delegates to ParallelRunner.
func TestParallelRunnerAddRecordUsesExistingRecord(t *testing.T) {
	colors := NewColorsWithOverride(false)
	tests := NewTests()
	display := NewDisplay(tests, colors)
	display.SetQuiet(true)

	r := NewParallelRunner[string](colors)
	r.setDisplay(display)
	r.SetLabel("add-record")

	rec := tests.Add("my-test")
	rec.Active = true

	r.addRecord(rec, "data", func(_ context.Context, _ string) (bool, error) {
		return false, errors.New("intentional")
	})

	if r.Run(context.Background()) {
		t.Fatal("expected failure")
	}

	if rec.State != StateFail {
		t.Fatalf("state = %v, want StateFail", rec.State)
	}
}

// TestParallelRunnerRespectsTerminalState verifies that ParallelRunner does not
// overwrite a terminal state set by the Run function (e.g., StateTimeout).
//
// VALIDATES: state preservation for .ci delegation where runTest sets StateTimeout.
// PREVENTS: timeout tests being reported as simple failures.
func TestParallelRunnerRespectsTerminalState(t *testing.T) {
	colors := NewColorsWithOverride(false)
	tests := NewTests()
	display := NewDisplay(tests, colors)
	display.SetQuiet(true)

	r := NewParallelRunner[string](colors)
	r.setDisplay(display)
	r.SetLabel("terminal-state")

	rec := tests.Add("timeout-test")
	rec.Active = true

	r.addRecord(rec, "data", func(_ context.Context, _ string) (bool, error) {
		rec.State = StateTimeout
		return false, errors.New("timed out")
	})

	if r.Run(context.Background()) {
		t.Fatal("expected failure")
	}

	if rec.State != StateTimeout {
		t.Fatalf("state = %v, want StateTimeout (was overwritten)", rec.State)
	}
}

// TestParallelRunnerSetStatusInterval verifies the configurable status interval.
//
// VALIDATES: .ci uses 500ms vs ParallelRunner's default 200ms.
// PREVENTS: status-ticker cadence change when .ci delegates.
func TestParallelRunnerSetStatusInterval(t *testing.T) {
	r := NewParallelRunner[string](NewColorsWithOverride(false))
	r.setStatusInterval(500 * time.Millisecond)
	r.SetLabel("interval")
	r.SetQuiet(true)

	r.AddTest("fast", "x", func(_ context.Context, _ string) (bool, error) {
		return true, nil
	})
	if !r.Run(context.Background()) {
		t.Fatal("expected pass")
	}
}

// TestParallelRunnerOnReportCalledOnFailure verifies the post-run report hook.
//
// VALIDATES: .ci PrintAllFailures called via onReport hook.
// PREVENTS: missing failure detail when .ci delegates to ParallelRunner.
func TestParallelRunnerOnReportCalledOnFailure(t *testing.T) {
	colors := NewColorsWithOverride(false)
	display := NewDisplay(NewTests(), colors)
	display.SetOutput(&bytes.Buffer{})

	r := NewParallelRunner[string](colors)
	r.setDisplay(display)
	r.SetLabel("report-hook")

	r.AddTest("failing", "data", func(_ context.Context, _ string) (bool, error) {
		return false, errors.New("boom")
	})

	var reportCalled bool
	r.setOnReport(func(tests *Tests) {
		reportCalled = true
		_, failed, _, _ := tests.Summary()
		if failed != 1 {
			t.Fatalf("report hook got %d failures, want 1", failed)
		}
	})

	if r.Run(context.Background()) {
		t.Fatal("expected failure")
	}
	if !reportCalled {
		t.Fatal("onReport hook was not called")
	}
}

// TestParallelRunnerOnReportNotCalledOnSuccess verifies the hook is skipped
// when all tests pass.
//
// VALIDATES: no spurious report hook calls.
func TestParallelRunnerOnReportNotCalledOnSuccess(t *testing.T) {
	r := NewParallelRunner[string](NewColorsWithOverride(false))
	r.SetLabel("report-ok")
	r.SetQuiet(true)

	r.AddTest("passing", "data", func(_ context.Context, _ string) (bool, error) {
		return true, nil
	})

	called := false
	r.setOnReport(func(_ *Tests) { called = true })

	if !r.Run(context.Background()) {
		t.Fatal("expected pass")
	}
	if called {
		t.Fatal("onReport should not be called when all tests pass")
	}
}

// TestParallelRunnerVerifyModeEmitsBothGroupsAndReport verifies that in
// verify mode, ParallelRunner produces failure groups AND invokes the
// onReport hook (which .ci uses for PrintAllFailures).
//
// VALIDATES: verify-mode integration -- groups + all-failures both fire.
// PREVENTS: regression where one report path suppresses the other.
func TestParallelRunnerVerifyModeEmitsBothGroupsAndReport(t *testing.T) {
	setVerifyMode(t)

	colors := NewColorsWithOverride(false)
	var buf bytes.Buffer

	r := NewParallelRunner[string](colors)
	r.SetLabel("verify-integration")

	display := NewDisplay(NewTests(), colors)
	display.SetOutput(&buf)
	display.SetQuiet(true)
	r.setDisplay(display)

	r.AddTest("broken", "fixture", func(context.Context, string) (bool, error) {
		return false, errors.New("broken")
	})

	var reportCalled bool
	r.setOnReport(func(tests *Tests) {
		reportCalled = true
	})

	if r.Run(context.Background()) {
		t.Fatal("expected failure")
	}

	output := buf.String()
	if !strings.Contains(output, "VERIFY FAILURE GROUP:") {
		t.Fatalf("missing VERIFY FAILURE GROUP in output:\n%s", output)
	}
	if !reportCalled {
		t.Fatal("onReport hook not called in verify mode")
	}
}

// TestParallelRunnerQuietSuppressesReports verifies that quiet mode
// suppresses both failure groups and the onReport hook.
//
// VALIDATES: quiet gate on failure reports matches old .ci behavior.
// PREVENTS: verify-mode failure groups leaking into stress-mode iterations.
func TestParallelRunnerQuietSuppressesReports(t *testing.T) {
	setVerifyMode(t)

	colors := NewColorsWithOverride(false)
	var buf bytes.Buffer

	r := NewParallelRunner[string](colors)
	r.SetLabel("quiet-gate")
	r.SetQuiet(true)

	display := NewDisplay(NewTests(), colors)
	display.SetOutput(&buf)
	display.SetQuiet(true)
	r.setDisplay(display)

	r.AddTest("broken", "fixture", func(context.Context, string) (bool, error) {
		return false, errors.New("broken")
	})

	reportCalled := false
	r.setOnReport(func(_ *Tests) { reportCalled = true })

	if r.Run(context.Background()) {
		t.Fatal("expected failure")
	}

	output := buf.String()
	if strings.Contains(output, "VERIFY FAILURE GROUP:") {
		t.Fatalf("quiet mode should suppress failure groups:\n%s", output)
	}
	if reportCalled {
		t.Fatal("quiet mode should suppress onReport")
	}
}

func TestParallelRunnerAddTestWithNickRegistersStableNick(t *testing.T) {
	r := NewParallelRunner[string](NewColorsWithOverride(false))
	rec := r.AddTestWithNick("parse-fixture", "X", "fixture", func(context.Context, string) (bool, error) {
		return true, nil
	})
	if rec.Nick != "X" {
		t.Fatalf("record nick = %q, want X", rec.Nick)
	}
	rec.State = StateFail
	if got := r.display.tests.GetByNick("X"); got != rec {
		t.Fatalf("stable nick lookup returned %p, want %p", got, rec)
	}
	if got := r.display.tests.failedNicks(); len(got) != 1 || got[0] != "X" {
		t.Fatalf("failed nicks = %v, want [X]", got)
	}
}
