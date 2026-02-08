package runner

import (
	"context"
	"sync"
	"time"
)

// Parallel execution constants.
const (
	DefaultParallelTimeout    = 30 * time.Second
	DefaultParallelConcurrent = 20
	StatusUpdateInterval      = 200 * time.Millisecond
)

// ParallelTest represents a test that can be run in parallel.
type ParallelTest[T any] struct {
	Name   string
	Record *Record
	Test   T // The original test object
	Run    func(ctx context.Context, t T) (passed bool, err error)
}

// ParallelRunner executes tests in parallel with progress display.
type ParallelRunner[T any] struct {
	tests   []*ParallelTest[T]
	display *Display
	colors  *Colors
	quiet   bool
	verbose bool
	onFail  func(T, error) // Called for each failed test (for verbose output)
}

// NewParallelRunner creates a parallel test runner.
func NewParallelRunner[T any](colors *Colors) *ParallelRunner[T] {
	return &ParallelRunner[T]{
		colors: colors,
	}
}

// SetQuiet enables quiet mode.
func (r *ParallelRunner[T]) SetQuiet(quiet bool) {
	r.quiet = quiet
}

// SetVerbose enables verbose output for failures.
func (r *ParallelRunner[T]) SetVerbose(verbose bool) {
	r.verbose = verbose
}

// SetOnFail sets the callback for failed tests.
// The callback receives the original test object and the error.
func (r *ParallelRunner[T]) SetOnFail(fn func(T, error)) {
	r.onFail = fn
}

// AddTest adds a test to the runner.
func (r *ParallelRunner[T]) AddTest(name string, test T, runFn func(ctx context.Context, t T) (bool, error)) *Record {
	if r.display == nil {
		// Lazy init - create Tests container and Display on first AddTest
		tests := NewTests()
		r.display = NewDisplay(tests, r.colors)
		r.display.SetQuiet(r.quiet)
		r.display.SetTimeout(DefaultParallelTimeout)
	}

	rec := r.display.tests.Add(name)
	rec.Active = true

	r.tests = append(r.tests, &ParallelTest[T]{
		Name:   name,
		Record: rec,
		Test:   test,
		Run:    runFn,
	})

	return rec
}

// Run executes all tests in parallel and returns success.
func (r *ParallelRunner[T]) Run(ctx context.Context) bool {
	if len(r.tests) == 0 {
		return true
	}

	// Configure display
	r.display.SetParallel(DefaultParallelConcurrent, len(r.tests))
	r.display.Start()

	// Channels
	type result struct {
		test   *ParallelTest[T]
		passed bool
		err    error
	}
	results := make(chan result, len(r.tests))
	done := make(chan struct{})

	// Run tests in parallel with semaphore
	var wg sync.WaitGroup
	sem := make(chan struct{}, DefaultParallelConcurrent)

	for _, test := range r.tests {
		wg.Add(1)
		go func(t *ParallelTest[T]) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			t.Record.State = StateRunning
			t.Record.StartTime = time.Now()

			passed, err := t.Run(ctx, t.Test)
			t.Record.Duration = time.Since(t.Record.StartTime)

			if passed {
				t.Record.State = StateSuccess
			} else {
				t.Record.State = StateFail
				t.Record.Error = err
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
	go func() {
		ticker := time.NewTicker(StatusUpdateInterval)
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
		r.display.Status()
	}

	close(done)
	r.display.Newline()
	r.display.Summary()

	// Call onFail for each failure if verbose and callback set
	if r.verbose && r.onFail != nil && len(failures) > 0 {
		for _, f := range failures {
			r.onFail(f.test, f.err)
		}
	}

	_, failed, _, _ := r.display.tests.Summary()
	return failed == 0
}
