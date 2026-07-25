// Design: docs/architecture/testing/ci-format.md — test runner framework
// Detail: runner_exec.go — test execution and process orchestration
// Detail: runner_validate.go — result validation (JSON, logging, HTTP)
// Detail: runner_output.go — output capture, saving, and parsing
// Related: timing.go — timing baseline persistence and slow detection

package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/sessionpath"
)

var logger = slogutil.LazyLogger("test.runner")

// Test-runner env vars (also read by internal/test/cli, which imports this package).
var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.tags", Type: "string", Description: "Extra Go build tags for test builds (comma or space separated)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bin", Type: "string", Description: "Pre-built ze binary path for the test runner (absolute or repo-relative)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.test.bin", Type: "string", Description: "Pre-built ze-test binary path for the test runner (absolute or repo-relative)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.test.no.build", Type: "bool", Description: "Skip the in-process go build and require pre-built test binaries"})
)

const binNameZePeer = "ze-peer"

// TestPluginBuildTag enables internal/test/plugins for functional-test DUTs.
const TestPluginBuildTag = "zetest"

// featureGatesFile is the feature-gate manifest (repo-relative): the single
// source of truth for compile-out-able features, shared with the Makefile,
// the generator, and dep_audit.py. See ai/rules/feature-gate-registration.md.
const featureGatesFile = "feature-gates.txt"

// TestBuildTags returns ZE_TAGS plus the tags for functional test builds.
// ze_core + ze_distro provide the full ze command surface. ze_setup adds
// the provision plugin (install remote) and appliance tooling. zetest adds
// test-only plugins on top. The default-on per-feature compile-out tags
// (ze_lg, ze_ssh, ze_web, ...) are read from feature-gates.txt so the
// functional-test ze binary exercises the same feature set as `make ze`
// (ZE_FEATURES) without a hand-maintained list. See plan/spec-feature-gate-0-umbrella.md.
func TestBuildTags() string {
	tags := strings.FieldsFunc(env.Get("ze.tags"), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	tags = append(tags, TestPluginBuildTag, "ze_core", "ze_distro", "ze_setup")
	tags = append(tags, featureGateTags()...)
	return textbuf.Join(tags, ",")
}

// featureGateTags reads the default-on feature tags from feature-gates.txt (the
// first column of each non-comment line), deduplicated. Mirrors ZE_FEATURES in
// the Makefile. Returns nil if the manifest cannot be read (the build would then
// fail loudly on the first feature schema, the same signal as before).
func featureGateTags() []string {
	root, ok := findRepoRoot()
	if !ok {
		logger().Warn("feature-gate manifest not found; functional-test ze may lack feature tags", "file", featureGatesFile)
		return nil
	}
	data, err := os.ReadFile(filepath.Join(root, featureGatesFile)) //nolint:gosec // fixed manifest filename under the discovered repo root
	if err != nil {
		logger().Warn("feature-gate manifest unreadable; functional-test ze may lack feature tags", "file", featureGatesFile, "error", err)
		return nil
	}
	var tags []string
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || seen[fields[0]] {
			continue
		}
		seen[fields[0]] = true
		tags = append(tags, fields[0])
	}
	return tags
}

// findRepoRoot walks up from the working directory to the module root (the dir
// holding go.mod). Test binaries run inside the module, so this resolves the
// repo root regardless of which package the calling test lives in.
func findRepoRoot() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// RunOptions configures test execution.
type RunOptions struct {
	Timeout     time.Duration
	Parallel    int
	Verbose     bool
	DebugNicks  []string
	Quiet       bool
	SaveDir     string
	SkipTimings bool // skip timing recording (set during stress mode)
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
	timings  Timings // rolling timing baseline

	// concurrency is the resolved number of tests run in parallel for the
	// current Run. Set at the top of Run; read by runTest to decide whether to
	// apply ParallelTimeoutHeadroom. Zero outside a Run.
	concurrency int

	// extraBinaries maps binary name -> build spec for additional
	// binaries that should be built alongside ze and ze-test.
	extraBinaries map[string]ExtraBinary
}

// NewRunner creates a test runner.
func NewRunner(tests *EncodingTests, baseDir string) (*Runner, error) {
	// Root this run's scratch in the session's own directory when an AI session
	// is active, so the working dirs, configs and sockets a suite leaves behind
	// are attributable and die with the session instead of accumulating as
	// unowned $TMPDIR/ze-functional-* dirs. EnsureScratchRoot returns "" when no
	// session is active, which is exactly what MkdirTemp reads as "use the
	// system temp dir" -- so a human or CI run is unchanged.
	tmpDir, err := os.MkdirTemp(sessionpath.EnsureScratchRoot(baseDir), "ze-functional-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	colors := NewColors()
	// Off-session this is <baseDir>/bin, as before. Under a session it is that
	// session's private bin/, so a direct `ze-test` invocation (the chaos
	// targets, ad-hoc runs, ZE_TEST_CANONICAL=1) can no longer rebuild the
	// shared bin/ze out from under a sibling session mid-test. Build here is
	// unlocked by design; isolation, not locking, is what makes it safe.
	binDir := sessionpath.BinDir(baseDir)

	zePath := filepath.Join(binDir, "ze")
	if v := env.Get("ze.bin"); v != "" {
		if !filepath.IsAbs(v) {
			v = filepath.Join(baseDir, v)
		}
		zePath = v
	}
	testBinPath := filepath.Join(binDir, "ze-test")
	if v := env.Get("ze.test.bin"); v != "" {
		if !filepath.IsAbs(v) {
			v = filepath.Join(baseDir, v)
		}
		testBinPath = v
	}

	return &Runner{
		tests:    tests,
		baseDir:  baseDir,
		tmpDir:   tmpDir,
		zePath:   zePath,
		testPath: testBinPath,
		colors:   colors,
		display:  NewDisplay(tests.Tests, colors),
		report:   newReport(colors),
		timings:  LoadTimings(baseDir),
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

// ExtraBinary describes an additional Go binary to build alongside ze.
type ExtraBinary struct {
	Pkg  string
	Tags string
}

// SetExtraBinaries configures additional Go binaries to build alongside ze.
func (r *Runner) SetExtraBinaries(binaries map[string]ExtraBinary) {
	r.extraBinaries = binaries
}

// Cleanup removes temporary files.
func (r *Runner) Cleanup() {
	if r.tmpDir != "" {
		_ = os.RemoveAll(r.tmpDir)
	}
}

// Build compiles the test binaries.
//
// ZE_TEST_NO_BUILD=1 skips the in-process `go build` and uses pre-built binaries
// already present at r.zePath / r.testPath. This lets a slow target (e.g. a QEMU
// VM whose only writable storage is a slow 9p mount) reuse binaries cross-compiled
// on a fast host, instead of compiling the whole tree inside the VM.
func (r *Runner) Build(ctx context.Context) error {
	if env.IsEnabled("ze.test.no.build") {
		return r.verifyPrebuilt()
	}

	r.display.buildStatus(true, nil)

	// Build ze (with version ldflags matching Makefile convention)
	now := time.Now()
	var tb textbuf.Buffer
	ldflags := tb.Str("-X main.version=").Str(now.Format("06.01.02")).Str(" -X main.buildDate=").Str(now.UTC().Format("2006-01-02T15:04:05Z")).String()
	cmd := exec.CommandContext(ctx, "go", "build", "-tags", TestBuildTags(), "-ldflags", ldflags, "-o", r.zePath, "./cmd/ze") //nolint:gosec // paths from internal runner
	cmd.Dir = r.baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		r.display.buildStatus(false, fmt.Errorf("%w: %s", err, output))
		return fmt.Errorf("build ze: %w", err)
	}

	// Build ze-test (provides peer subcommand)
	cmd = exec.CommandContext(ctx, "go", "build", "-tags", "ze_test", "-o", r.testPath, "./cmd/ze") //nolint:gosec // paths from internal runner
	cmd.Dir = r.baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		r.display.buildStatus(false, fmt.Errorf("%w: %s", err, output))
		return fmt.Errorf("build ze-test: %w", err)
	}

	// Build extra binaries (e.g., ze-chaos for chaos-web tests).
	for name, spec := range r.extraBinaries {
		outPath := filepath.Join(r.tmpDir, name)
		buildArgs := []string{"build"}
		if spec.Tags != "" {
			buildArgs = append(buildArgs, "-tags", spec.Tags)
		}
		buildArgs = append(buildArgs, "-o", outPath, spec.Pkg)
		cmd = exec.CommandContext(ctx, "go", buildArgs...) //nolint:gosec // paths from internal runner
		cmd.Dir = r.baseDir
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			r.display.buildStatus(false, fmt.Errorf("%w: %s", err, output))
			return fmt.Errorf("build %s: %w", name, err)
		}
	}

	r.display.buildStatus(false, nil)
	return nil
}

// verifyPrebuilt is the ZE_TEST_NO_BUILD path: it checks that the binaries the
// runner would otherwise build already exist, rather than building them. Extra
// binaries (e.g. ze-chaos) are not supported in this mode and must be built
// normally.
func (r *Runner) verifyPrebuilt() error {
	r.display.buildStatus(true, nil)

	// Session scoping exists to stop one session's BUILD overwriting another's
	// binary; reading a binary someone already built clobbers nothing. So when
	// the session's own bin/ holds nothing, accept a pre-built set from the
	// shared bin/ -- that is where `make ze` off-session and every cross-compile
	// put it, and reporting it "missing" would break ZE_TEST_NO_BUILD for anyone
	// who did exactly what the flag asks.
	//
	// Both binaries move together, to ONE directory: .ci tests exec `ze` and
	// `ze-stripped` by bare name off the single directory this runner puts on
	// their PATH (runner_exec.go), so resolving ze from one directory and ze-test
	// from another would pass both stat calls and still strand a test on a
	// sibling binary that is not beside it.
	//
	// An explicit ZE_BIN/ZE_TEST_BIN is exempt: it names ONE binary, so a miss
	// there must fail loudly rather than silently run a different build.
	if env.Get("ze.bin") == "" && env.Get("ze.test.bin") == "" {
		_, zeErr := os.Stat(r.zePath)
		_, testErr := os.Stat(r.testPath)
		if zeErr != nil || testErr != nil {
			if dir := sessionpath.FindPrebuiltDir(r.baseDir, "ze", "ze-test"); dir != "" {
				r.zePath = filepath.Join(dir, "ze")
				r.testPath = filepath.Join(dir, "ze-test")
			}
		}
	}

	for _, p := range []string{r.zePath, r.testPath} {
		if _, err := os.Stat(p); err != nil {
			buildErr := fmt.Errorf("ZE_TEST_NO_BUILD set but %s is missing (cross-compile it first): %w", p, err)
			r.display.buildStatus(false, buildErr)
			return buildErr
		}
	}
	if len(r.extraBinaries) > 0 {
		buildErr := fmt.Errorf("ZE_TEST_NO_BUILD does not support extra binaries: %v", r.extraBinaries)
		r.display.buildStatus(false, buildErr)
		return buildErr
	}
	r.display.buildStatus(false, nil)
	return nil
}

// Run executes selected tests by delegating to ParallelRunner[*Record].
// Runner keeps .ci-specific concerns (Build, process orchestration via runTest,
// PrintAllFailures). Scheduling is the single ParallelRunner engine.
func (r *Runner) Run(ctx context.Context, opts *RunOptions) bool {
	r.display.SetQuiet(opts.Quiet)
	r.display.SetTimeout(opts.Timeout)

	selected := r.tests.Selected()
	if len(selected) == 0 {
		logger().Info("no tests selected")
		return true
	}

	parallel := opts.Parallel
	if parallel <= 0 {
		parallel = len(selected)
	}
	r.concurrency = parallel

	load := SnapshotHostLoad()

	pr := NewParallelRunner[*Record](r.colors)
	pr.setDisplay(r.display)
	pr.SetConcurrency(parallel)
	pr.setStatusInterval(500 * time.Millisecond)
	pr.SetQuiet(opts.Quiet)
	pr.SetLabel(r.display.label)
	pr.setNoHeader(true)
	pr.setHostLoad(&load)

	if !opts.SkipTimings {
		pr.SetBaseDir(r.baseDir)
	}
	if opts.SkipTimings {
		pr.setNoSummary(true)
	}

	for _, rec := range selected {
		pr.addRecord(rec, rec, func(runCtx context.Context, rec *Record) (bool, error) {
			success := r.runTest(runCtx, rec, opts)
			if !success {
				return false, rec.Error
			}
			return true, nil
		})
	}

	pr.setOnReport(func(tests *Tests) {
		r.report.printAllFailures(tests)
	})

	return pr.Run(ctx)
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

	// Create stress-mode options (suppress per-iteration output and timing recording)
	stressOpts := *opts
	stressOpts.Quiet = true       // Suppress verbose output per iteration
	stressOpts.SkipTimings = true // Don't pollute baseline with stress-load timings

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
			rec.StepTrace = rec.StepTrace[:0]
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
