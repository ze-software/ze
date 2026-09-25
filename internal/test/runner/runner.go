// Design: docs/architecture/testing/ci-format.md — test runner framework
// Detail: runner_exec.go — test execution and process orchestration
// Detail: runner_validate.go — result validation (JSON, logging, HTTP)
// Detail: runner_output.go — output capture, saving, and parsing
// Related: timing.go — timing baseline persistence and slow detection

package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
	"github.com/ze-software/ze/internal/test/harnessbin"
	"github.com/ze-software/ze/internal/test/sessionpath"
)

var logger = slogutil.LazyLogger("test.runner")

// envTypeString is the env registry's name for a free-text value.
const envTypeString = "string"

// Test-runner env vars (also read by internal/test/cli, which imports this package).
// The harness variables le.test.bin and le.test.no.build are registered in
// internal/test/harnessbin, which resolves their retired ze. spellings.
var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.tags", Type: envTypeString, Description: "Extra Go build tags for test builds (comma or space separated)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bin", Type: envTypeString, Description: "Pre-built ze binary path for the test runner (absolute or repo-relative)"})
)

// Names of the binaries the runner builds, shims, and execs.
const (
	binNameZe     = "ze"
	binNameZePeer = "ze-peer"
	binNameZeTest = "ze-test" // the harness as a .ci execs it; the file is harnessbin.Name
)

// TestPluginBuildTag enables internal/test/plugins for functional-test DUTs.
const TestPluginBuildTag = "zetest"

// featureGatesFile is the feature-gate manifest (repo-relative): the single
// source of truth for compile-out-able features, written by
// `./le feature-tags write`. See ai/rules/plugins.md.
const featureGatesFile = "feature-gates.txt"

// TestBuildTags returns ZE_TAGS plus the tags for functional test builds.
// ze_core + ze_distro provide the full ze command surface. ze_setup adds
// the provision plugin (install remote) and appliance tooling. zetest adds
// test-only plugins on top. The default-on per-feature compile-out tags
// (ze_lg, ze_ssh, ze_web, ...) are read from feature-gates.txt so the
// functional-test ze binary exercises the same feature set as the native Go
// builders without a hand-maintained list. See plan/spec-feature-gate-0-umbrella.md.
func TestBuildTags() (string, error) {
	gates, err := featureGateTags()
	if err != nil {
		return "", err
	}

	tags := zeTagsFromEnv()
	tags = append(tags, TestPluginBuildTag, "ze_core", "ze_distro", "ze_setup")
	tags = append(tags, gates...)
	return textbuf.Join(tags, ","), nil
}

// testHelperBuildTags returns the tags for the ze-test helper binary, using the
// same generated feature manifest as the daemon build.
//
// The feature-gate tags are NOT optional decoration here. ze-test links the
// engine's own plugin registry so `ze-test plugin-external <name>` can run a
// registered plugin's RunEngine over a real TLS connect-back
// (internal/test/cli/cmd_plugin_external.go). Registration happens in each
// plugin package's init(), which a feature gate compiles out: built with a bare
// `ze_test`, the helper's registry.Lookup misses every gated plugin and the
// launcher exits 1 with "unknown registered plugin" before the plugin under test
// can say anything. The daemon, built from TestBuildTags, then reports only a
// TLS connect-back failure -- so as112-external-refuses and
// flowexport-external-refuses waited out their await=stderr fence for a refusal
// that no process was ever alive to emit. Two build recipes for one binary is
// what drifted; both now derive their feature set from feature-gates.txt.
func testHelperBuildTags() (string, error) {
	gates, err := featureGateTags()
	if err != nil {
		return "", err
	}

	tags := append([]string{"ze_test"}, gates...)
	tags = append(tags, zeTagsFromEnv()...)
	return textbuf.Join(tags, ","), nil
}

// zeTagsFromEnv splits the ze.tags knob (comma or whitespace separated) into
// individual build tags.
func zeTagsFromEnv() []string {
	return strings.FieldsFunc(env.Get("ze.tags"), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
}

// featureGateTags answers the default-on feature tags, through the one reader of
// feature-gates.txt.
//
// An unreadable manifest is an ERROR rather than an empty list. The empty list
// builds a DUT with every gated feature compiled out. That binary answers a
// test about ze_web or ze_bgp with "unknown top-level keyword", which reads as
// a defect in the feature rather than in the build.
func featureGateTags() ([]string, error) {
	root, ok := findRepoRoot()
	if !ok {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("no checkout found above the working directory, so ").
			Str(featureGatesFile).Str(" cannot be read").String())
	}
	return repofeaturetags.DaemonTags(root)
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

	// binShimDir holds bare-named symlinks (ze, ze-test) to the binaries this
	// run actually resolved. It is what goes on a test child's PATH; see
	// setupBinShims for why the binaries' own directory must not.
	binShimDir string
}

// NewRunner creates a test runner.
func NewRunner(tests *EncodingTests, baseDir string) (*Runner, error) {
	// Root this run's scratch in the session's own directory when an AI session
	// is active, so the working dirs, configs and sockets a suite leaves behind
	// are attributable and go with that session's directory when the operator
	// removes it, instead of accumulating as unowned $TMPDIR/ze-functional-*
	// dirs. They do not "die with the session": nothing under tmp/session/ is
	// removed automatically, and `./le session reap` removes directories whose
	// owners are provably gone. EnsureScratchRoot returns "" when no
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

	zePath := filepath.Join(binDir, binNameZe)
	if v := env.Get("ze.bin"); v != "" {
		if !filepath.IsAbs(v) {
			v = filepath.Join(baseDir, v)
		}
		zePath = v
	}
	testBinPath := filepath.Join(binDir, harnessbin.Name)
	if v := harnessbin.TestBin(); v != "" {
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

// setupBinShims creates a directory of bare-named symlinks to the binaries this
// run resolved, and returns it. It is the ONLY directory the runner prepends to
// a test child's PATH.
//
// Putting filepath.Dir(r.zePath) there instead -- which is what the runner did
// until this existed -- is wrong twice over:
//
//   - A cross-compiled binary carries its target in the name
//     (for example ze-linux-arm64 from the native QEMU builder), and ZE_BIN
//     points at it directly, so there is no bare `ze` in that directory at all.
//     A test doing subprocess(["ze", ...]) then resolves whatever unrelated `ze` is
//     left in bin/ from some earlier build.
//   - Driving the QEMU VM from a darwin host, one directory holds BOTH
//     architectures (ze is darwin, ze-linux-arm64 is the VM's). The VM picked
//     up the darwin one and died with "OSError: [Errno 8] Exec format error:
//     'ze'" (test/static/static-table-interface.ci). The same hazard is called out
//     at internal/component/plugin/process/process.go:540.
//
// Symlinks rather than copies so this stays cheap on a slow 9p mount, and
// because they are transparent to ze's own path resolution: DefaultConfigDir
// runs filepath.EvalSymlinks before ConfigDirFromBinary
// (internal/core/paths/paths.go:83), so a shimmed ze resolves the same
// <prefix>/etc/ze as an unshimmed one.
func (r *Runner) setupBinShims() error {
	dir := filepath.Join(r.tmpDir, "binshim")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create bin shim dir: %w", err)
	}
	for name, target := range map[string]string{binNameZe: r.zePath, binNameZeTest: r.testPath} {
		abs, err := filepath.Abs(target)
		if err != nil {
			return fmt.Errorf("resolve %s binary %q: %w", name, target, err)
		}
		link := filepath.Join(dir, name)
		if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("replace stale shim %s: %w", link, err)
		}
		if err := os.Symlink(abs, link); err != nil {
			return fmt.Errorf("link %s -> %s: %w", link, abs, err)
		}
	}
	r.binShimDir = dir
	return nil
}

// childPathEnv returns the ready-to-append PATH= entry a test child process
// gets: the bare-name shim dir first, then the real binary directory (so a
// suite that reaches for another binary sitting beside ze still finds it), then
// the inherited PATH.
func (r *Runner) childPathEnv() string {
	parts := make([]string, 0, 3)
	if r.binShimDir != "" {
		parts = append(parts, r.binShimDir)
	}
	parts = append(parts, filepath.Dir(r.zePath))
	if existing := os.Getenv("PATH"); existing != "" {
		parts = append(parts, existing)
	}
	var tb textbuf.Buffer
	return tb.Str("PATH=").Join(parts, string(os.PathListSeparator)).String()
}

// workDirPrefix names every per-test working directory under the session
// scratch root, so a directory left behind by a killed run says what made it.
const workDirPrefix = "ze-work-"

// repositoryAnchoredBinary answers whether a .ci step's binary resolves its
// arguments against the repository rather than against the test. `go test`
// takes ./... package patterns that mean nothing outside the module, and `./le`
// IS a relative path to the script in the repository root. Every other binary a
// .ci step names is given the test's own directory, so its runtime files land
// beside the test.
func repositoryAnchoredBinary(binName string) bool {
	return binName == "go" || binName == "le" || binName == "./le"
}

// childWorkingDirectory answers the directory one .ci step's child runs in.
//
// A child given no directory inherits the runner's, which is the repository
// root, and a ze daemon started there writes its runtime state into the
// checkout (see Record.WorkDir).
func (r *Runner) childWorkingDirectory(binName string, rec *Record) string {
	if repositoryAnchoredBinary(binName) {
		return r.baseDir
	}
	return rec.WorkDir
}

// Build compiles the test binaries.
//
// LE_TEST_NO_BUILD=1 skips the in-process `go build` and uses pre-built binaries
// already present at r.zePath / r.testPath. This lets a slow target (e.g. a QEMU
// VM whose only writable storage is a slow 9p mount) reuse binaries cross-compiled
// on a fast host, instead of compiling the whole tree inside the VM.
func (r *Runner) Build(ctx context.Context) error {
	if harnessbin.NoBuild() {
		return r.verifyPrebuilt()
	}

	r.display.buildStatus(true, nil)

	// Build ze with the repository's version ldflags.
	now := time.Now()
	var tb textbuf.Buffer
	ldflags := tb.Str("-X main.version=").Str(now.Format("06.01.02")).Str(" -X main.buildDate=").Str(now.UTC().Format("2006-01-02T15:04:05Z")).String()
	zeTags, err := TestBuildTags()
	if err != nil {
		r.display.buildStatus(false, err)
		return fmt.Errorf("build ze: %w", err)
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-tags", zeTags, "-ldflags", ldflags, "-o", r.zePath, "./cmd/ze") //nolint:gosec // paths from internal runner
	cmd.Dir = r.baseDir
	cmd.Env = childEnv("CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		r.display.buildStatus(false, fmt.Errorf("%w: %s", err, output))
		return fmt.Errorf("build ze: %w", err)
	}

	// Build ze-test (provides peer subcommand, and plugin-external's registry)
	helperTags, err := testHelperBuildTags()
	if err != nil {
		r.display.buildStatus(false, err)
		return fmt.Errorf("build ze-test: %w", err)
	}
	cmd = exec.CommandContext(ctx, "go", "build", "-tags", helperTags, "-o", r.testPath, "./cmd/ze") //nolint:gosec // paths from internal runner
	cmd.Dir = r.baseDir
	cmd.Env = childEnv("CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		r.display.buildStatus(false, fmt.Errorf("%w: %s", err, output))
		return fmt.Errorf("build ze-test: %w", err)
	}
	// Callers still exec the harness by its retired name, so a build of the
	// default file writes that name beside it (harnessbin.LinkRetired).
	if filepath.Base(r.testPath) == harnessbin.Name {
		if _, err := harnessbin.LinkRetired(r.testPath); err != nil {
			r.display.buildStatus(false, err)
			return fmt.Errorf("link %s: %w", harnessbin.RetiredName, err)
		}
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
		cmd.Env = childEnv("CGO_ENABLED=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			r.display.buildStatus(false, fmt.Errorf("%w: %s", err, output))
			return fmt.Errorf("build %s: %w", name, err)
		}
	}

	if err := r.setupBinShims(); err != nil {
		r.display.buildStatus(false, err)
		return err
	}

	r.display.buildStatus(false, nil)
	return nil
}

// verifyPrebuilt is the LE_TEST_NO_BUILD path: it checks that the binaries the
// runner would otherwise build already exist, rather than building them. Extra
// binaries (e.g. ze-chaos) are not supported in this mode and must be built
// normally.
func (r *Runner) verifyPrebuilt() error {
	r.display.buildStatus(true, nil)

	// Session scoping exists to stop one session's BUILD overwriting another's
	// binary; reading a binary someone already built clobbers nothing. So when
	// the session's own bin/ holds nothing, accept a pre-built set from the
	// shared bin/, where manual and cross-compiles place it. Reporting it
	// "missing" would break LE_TEST_NO_BUILD for anyone
	// who did exactly what the flag asks.
	//
	// Both binaries move together, to ONE directory: .ci tests exec `ze` and
	// `ze-stripped` by bare name off the single directory this runner puts on
	// their PATH (runner_exec.go), so resolving ze from one directory and ze-test
	// from another would pass both stat calls and still strand a test on a
	// sibling binary that is not beside it.
	//
	// An explicit ZE_BIN/LE_TEST_BIN is exempt: it names ONE binary, so a miss
	// there must fail loudly rather than silently run a different build.
	if env.Get("ze.bin") == "" && harnessbin.TestBin() == "" {
		_, zeErr := os.Stat(r.zePath)
		_, testErr := os.Stat(r.testPath)
		if zeErr != nil || testErr != nil {
			if dir := sessionpath.FindPrebuiltDir(r.baseDir, binNameZe, harnessbin.Name); dir != "" {
				r.zePath = filepath.Join(dir, binNameZe)
				r.testPath = filepath.Join(dir, harnessbin.Name)
			}
		}
	}

	for _, p := range []string{r.zePath, r.testPath} {
		if _, err := os.Stat(p); err != nil {
			buildErr := fmt.Errorf("LE_TEST_NO_BUILD set but %s is missing (cross-compile it first): %w", p, err)
			r.display.buildStatus(false, buildErr)
			return buildErr
		}
	}
	if len(r.extraBinaries) > 0 {
		buildErr := fmt.Errorf("LE_TEST_NO_BUILD does not support extra binaries: %v", r.extraBinaries)
		r.display.buildStatus(false, buildErr)
		return buildErr
	}
	// After the paths above are final: this is the branch where they most often
	// are NOT <repo>/bin/ze (ZE_BIN names a cross-compiled ze-linux-<arch>), so
	// it is the branch that most needs the bare-name shims.
	if err := r.setupBinShims(); err != nil {
		r.display.buildStatus(false, err)
		return err
	}
	r.display.buildStatus(false, nil)
	return nil
}

// Run executes selected tests by delegating to ParallelRunner[*Record].
// Runner keeps .ci-specific concerns (Build, process orchestration via runTest,
// PrintAllFailures). Scheduling is the single ParallelRunner engine.
func (r *Runner) Run(ctx context.Context, opts *RunOptions) bool {
	// Set on the RUNNER, so every child inherits it -- childEnv covers only the
	// sites that build an explicit env, and a site that leaves Cmd.Env nil hands
	// the child this process's environment instead. Measured before this line
	// existed: 72 ze samples in one ze-functional-ospf-test run, 7 with the variable.
	// A caller's own value wins; see childEnv for what the variable does and,
	// more importantly, does not buy.
	if os.Getenv("GOTRACEBACK") == "" { //nolint:forbidigo // Go runtime var, not a ze.* setting
		os.Setenv("GOTRACEBACK", "all") //nolint:errcheck,gosec // best-effort diagnostic
	}
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
	// Record the EFFECTIVE concurrency, not the requested cap. parallelFactor
	// (runner_exec_util.go) keys the 3x ParallelTimeoutHeadroom on
	// `concurrency > 1`, and its contract is that a single selected test keeps
	// the authored timeout so a real slowdown surfaces immediately. Once suites
	// carry a bounded DEFAULT (DefaultSuiteConcurrency) instead of 0, the
	// requested cap is 8/32/128 even for `ze-test <suite> <one-id>` -- so
	// storing the cap silently tripled every budget in exactly the single-test
	// debug loop ai/rules/testing.md tells people to use.
	r.concurrency = min(parallel, len(selected))

	load := snapshotHostLoad()

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

	// -v prints what the runner RAN for every test, passing ones included. The
	// option was carried from the command line into RunOptions and read by
	// nothing, so `ze-test <suite> -v` over a green suite printed no more than a
	// bare run (AC-8, spec-fixit-ci-runner-cannot-test-stdin).
	pr.SetVerbose(opts.Verbose)
	pr.setOnVerbose(func(tests *Tests) {
		r.report.printStepTraces(tests)
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
			rec.SetState(StateNone)
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
