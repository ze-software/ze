// Design: docs/architecture/testing/ci-format.md — test runner framework
// Detail: runner_exec.go — test execution and process orchestration
// Detail: runner_validate.go — result validation (JSON, logging, HTTP)
// Detail: runner_output.go — output capture, saving, and parsing

package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

var logger = slogutil.LazyLogger("test.runner")

const binNameZePeer = "ze-peer"

// RunOptions configures test execution.
type RunOptions struct {
	Timeout    time.Duration
	Parallel   int
	Verbose    bool
	DebugNicks []string
	Quiet      bool
	SaveDir    string
}

// DefaultRunOptions returns sensible defaults.
func DefaultRunOptions() *RunOptions {
	return &RunOptions{
		Timeout:  20 * time.Second,
		Parallel: DefaultParallelConcurrent,
		Verbose:  false,
		Quiet:    false,
	}
}

// Runner executes encoding tests.
type Runner struct {
	tests    *EncodingTests
	baseDir  string
	tmpDir   string
	zePath   string
	testPath string // ze-test binary (used for peer subcommand)
	display  *Display
	report   *Report
	colors   *Colors

	// extraBinaries maps binary name -> Go package path for additional
	// binaries that should be built alongside ze and ze-test.
	// Example: {"ze-chaos": "./cmd/ze-chaos"}
	extraBinaries map[string]string
}

// NewRunner creates a test runner.
func NewRunner(tests *EncodingTests, baseDir string) (*Runner, error) {
	tmpDir, err := os.MkdirTemp("", "ze-functional-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	colors := NewColors()
	binDir := filepath.Join(baseDir, "bin")
	return &Runner{
		tests:    tests,
		baseDir:  baseDir,
		tmpDir:   tmpDir,
		zePath:   filepath.Join(binDir, "ze"),
		testPath: filepath.Join(binDir, "ze-test"),
		colors:   colors,
		display:  NewDisplay(tests.Tests, colors),
		report:   NewReport(colors),
	}, nil
}

// Display returns the runner's display for summary output.
func (r *Runner) Display() *Display {
	return r.display
}

// Report returns the runner's report generator.
func (r *Runner) Report() *Report {
	return r.report
}

// SetExtraBinaries configures additional Go binaries to build alongside ze.
// The map keys are binary names and values are Go package paths.
// Example: runner.SetExtraBinaries(map[string]string{"ze-chaos": "./cmd/ze-chaos"}).
func (r *Runner) SetExtraBinaries(binaries map[string]string) {
	r.extraBinaries = binaries
}

// Cleanup removes temporary files.
func (r *Runner) Cleanup() {
	if r.tmpDir != "" {
		_ = os.RemoveAll(r.tmpDir)
	}
}

// Build compiles the test binaries.
func (r *Runner) Build(ctx context.Context) error {
	r.display.BuildStatus(true, nil)

	// Build ze
	cmd := exec.CommandContext(ctx, "go", "build", "-o", r.zePath, "./cmd/ze") //nolint:gosec // paths from internal runner
	cmd.Dir = r.baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		r.display.BuildStatus(false, fmt.Errorf("%w: %s", err, output))
		return fmt.Errorf("build ze: %w", err)
	}

	// Build ze-test (provides peer subcommand)
	cmd = exec.CommandContext(ctx, "go", "build", "-o", r.testPath, "./cmd/ze-test") //nolint:gosec // paths from internal runner
	cmd.Dir = r.baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		r.display.BuildStatus(false, fmt.Errorf("%w: %s", err, output))
		return fmt.Errorf("build ze-test: %w", err)
	}

	// Build extra binaries (e.g., ze-chaos for chaos-web tests).
	for name, pkg := range r.extraBinaries {
		outPath := filepath.Join(r.tmpDir, name)
		cmd = exec.CommandContext(ctx, "go", "build", "-o", outPath, pkg) //nolint:gosec // paths from internal runner
		cmd.Dir = r.baseDir
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			r.display.BuildStatus(false, fmt.Errorf("%w: %s", err, output))
			return fmt.Errorf("build %s: %w", name, err)
		}
	}

	r.display.BuildStatus(false, nil)
	return nil
}

// Run executes selected tests.
func (r *Runner) Run(ctx context.Context, opts *RunOptions) bool {
	r.display.SetQuiet(opts.Quiet)
	r.display.SetTimeout(opts.Timeout)

	selected := r.tests.Selected()
	if len(selected) == 0 {
		logger().Info("no tests selected")
		return true
	}

	// Set parallel for batch display
	parallel := opts.Parallel
	if parallel <= 0 {
		parallel = len(selected)
	}
	r.display.SetParallel(parallel, len(selected))

	r.display.Start()

	type result struct {
		record  *Record
		success bool
	}

	results := make(chan result, len(selected))
	done := make(chan struct{})
	var wg sync.WaitGroup

	// Semaphore for concurrency limit
	sem := make(chan struct{}, parallel)

	for _, rec := range selected {
		wg.Add(1)
		go func(rec *Record) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			success := r.runTest(ctx, rec, opts)
			results <- result{record: rec, success: success}
		}(rec)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Periodic status update
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				r.display.Status()
			}
		}
	}()

	allSuccess := true
	for res := range results {
		if res.success {
			res.record.State = StateSuccess
		} else {
			if res.record.State != StateTimeout {
				res.record.State = StateFail
			}
			allSuccess = false
		}
		r.display.Status()
	}

	close(done)
	r.display.Newline()

	// Print failure reports
	if !allSuccess && !opts.Quiet {
		r.report.PrintAllFailures(r.tests.Tests)
	}

	return allSuccess
}

// RunWithCount runs each test count times for stress testing.
// Returns StressResult with stats, iteration timings, and overall success.
func (r *Runner) RunWithCount(ctx context.Context, opts *RunOptions, count int) *StressResult {
	stats := NewStressStats(r.tests.Tests)
	result := &StressResult{
		Stats:              stats,
		IterationDurations: make([]time.Duration, 0, count),
		AllPassed:          true,
	}

	totalStart := time.Now()

	// Create stress-mode options (suppress per-iteration failure reports)
	stressOpts := *opts
	stressOpts.Quiet = true // Suppress verbose output per iteration

	for i := 1; i <= count; i++ {
		// Check for cancellation before each iteration
		select {
		case <-ctx.Done():
			result.TotalDuration = time.Since(totalStart)
			result.AllPassed = false
			return result
		default:
		}

		iterStart := time.Now()

		if !opts.Quiet {
			fmt.Printf("\n%s Iteration %d/%d\n", r.colors.Cyan("==>"), i, count)
		}

		// Reset test states for this iteration
		for _, rec := range r.tests.Selected() {
			rec.State = StateNone
			rec.Error = nil
			rec.Duration = 0
		}

		// Run iteration (with quiet mode to suppress failure reports)
		success := r.Run(ctx, &stressOpts)

		iterDuration := time.Since(iterStart)
		result.IterationDurations = append(result.IterationDurations, iterDuration)

		if !opts.Quiet {
			fmt.Printf("%s Iteration %d: %s\n", r.colors.Cyan("==>"), i, formatDuration(iterDuration))
		}

		if !success {
			result.AllPassed = false
		}

		// Collect stats from this iteration (only terminal states)
		for _, rec := range r.tests.Selected() {
			if s, ok := stats[rec.Nick]; ok {
				// Only record terminal states
				if rec.State == StateSuccess || rec.State == StateFail || rec.State == StateTimeout {
					s.Add(rec.State, rec.Duration)
				}
			}
		}
	}

	result.TotalDuration = time.Since(totalStart)
	return result
}
