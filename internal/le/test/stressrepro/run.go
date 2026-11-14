// Design: docs/functional-tests.md -- bounded concurrent reproduction under CPU and GC pressure
// Detail: process.go -- process-group execution and race builds

package teststressrepro

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/anmitsu/go-shlex"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
	"github.com/ze-software/ze/internal/test/runner"
)

var crashSignatures = []string{
	"slice bounds out of range",
	"panic:",
	"fatal error:",
	"DATA RACE",
	"runtime error:",
	"index out of range",
	"invalid memory address",
	"nil pointer dereference",
}

var usageSignatures = []string{
	"unknown command:",
	"unknown suite:",
	"flag provided but not defined",
}

const usageBanner = "\nCommands:\n"

// Report records the bounded run and the first reproduction, if one completed.
type Report struct {
	Suite       string   `json:"suite"`
	Test        string   `json:"test,omitempty"`
	Burners     int      `json:"burners"`
	Parallel    int      `json:"parallel"`
	Iterations  int      `json:"iterations"`
	Completed   int      `json:"completed"`
	Executed    int      `json:"executed,omitempty"`
	Failed      int      `json:"failed,omitempty"`
	Command     []string `json:"command,omitempty"`
	Race        bool     `json:"race"`
	Reproduced  bool     `json:"reproduced"`
	Exit        int      `json:"failure-exit,omitempty"`
	Signature   string   `json:"signature,omitempty"`
	Excerpt     string   `json:"excerpt,omitempty"`
	Log         string   `json:"log,omitempty"`
	SetupError  string   `json:"setup-error,omitempty"`
	Interrupted bool     `json:"interrupted,omitempty"`
}

// Text preserves the old tool's concise result and capture-path contract.
func (r Report) Text() string {
	var b textbuf.Buffer
	_, _ = fmt.Fprintf(&b, "stress-repro: suite=%s sel=%s burners=%d parallel=%d iterations=%d race=%t\n",
		r.Suite, valueOr(r.Test, "--all"), r.Burners, r.Parallel, r.Iterations, r.Race)
	if r.Log != "" {
		_, _ = fmt.Fprintf(&b, "  log: %s\n", r.Log)
	}
	switch {
	case r.SetupError != "":
		_, _ = fmt.Fprintf(&b, "stress-repro: %s\n", r.SetupError)
	case r.Reproduced:
		what := r.Signature
		if what == "" {
			what = fmt.Sprintf("non-zero exit %d", r.Exit)
		} else {
			what = "'" + what + "'"
		}
		_, _ = fmt.Fprintf(&b, "*** REPRODUCED on invocation %d (exit %d, signature: %s) ***\n", r.Completed, r.Exit, what)
		if r.Excerpt != "" {
			b.Str("--- crash excerpt ---\n")
			for line := range strings.SplitSeq(r.Excerpt, "\n") {
				_, _ = fmt.Fprintf(&b, "  %s\n", line)
			}
			b.Str("--- end excerpt ---\n")
		}
		_, _ = fmt.Fprintf(&b, "full capture: %s\n", r.Log)
	case r.Interrupted:
		_, _ = fmt.Fprintf(&b, "not reproduced in %d invocation(s) under load (interrupted). log: %s\n", r.Completed, r.Log)
	default:
		_, _ = fmt.Fprintf(&b, "not reproduced in %d invocation(s) under load. log: %s\n", r.Completed, r.Log)
	}
	return b.String()
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

type invocation struct {
	root        string
	environment []string
	selector    string
	// harness is the child's argv after `le test`, derived by harnessArgs.
	harness   []string
	test      string
	zeBin     string
	testBin   string
	timeout   time.Duration
	extraTags string
	// parallelFactor is the enclosing batch's contention factor. Zero means
	// no enclosing headroom; Invoke still retains any inherited runner factor.
	parallelFactor int
}

type processResult struct {
	code   int
	output string
	err    error
}

// succeeded reports whether the process started and exited 0.
func (r processResult) succeeded() bool {
	return r.err == nil && r.code == 0
}

type processRunner interface {
	Invoke(context.Context, invocation) processResult
	build(ctx context.Context, root, output, tags string, race bool) processResult
	// buildPlugin compiles the test plugin main package pkg to output. It
	// takes no feature tag: a test plugin links the SDK alone.
	buildPlugin(ctx context.Context, root, output, pkg string) processResult
}

type runDependencies struct {
	selected *selectedRun
	runner   processRunner
	now      func() time.Time
	cpuCount int
	pid      int
	burners  func(context.Context, int) func()
	// progress receives a copy of the capture log: the held slot's registry
	// log, or nil when the run holds no slot of its own.
	progress io.Writer
}

func realDependencies() runDependencies {
	return runDependencies{
		runner: realProcessRunner{}, now: time.Now, cpuCount: runtime.NumCPU(), pid: os.Getpid(), burners: startBurners,
	}
}

// runAt installs signal cancellation and runs the reproducer in root under
// admission. args are the words that followed the command name, which the job
// registry fingerprints.
func runAt(root string, args []string, opts Options) (Report, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return admitRun(ctx, root, args, opts, realDependencies())
}

// admitLabel is the job registry label a stress run holds its slot under.
const admitLabel = "test-stress-repro"

// admitRun holds ONE job registry slot for the whole stress run, so its N
// parallel children run inside it.
//
// Every child is the same `le test <suite> ...` over the same tree, and a runner
// admits itself (harnesstool.RunnerAnswer). Without a parent slot, child 1 would
// claim and children 2..N would attach to its verdict or queue behind it: N
// requested repetitions would collapse into one. Naming the claimed entry as the
// job parent makes every child answer KindInside and run for real.
//
// The run never attaches (MayAttach is false): a stress run is a sample, and
// another run's verdict says nothing about this run's iterations.
func admitRun(ctx context.Context, root string, args []string, opts Options, deps runDependencies) (Report, int) {
	base := Report{Suite: opts.Suite, Test: opts.Test, Iterations: opts.Iterations, Race: opts.Race}
	admission, err := job.NewIn(root)
	if err != nil {
		return setupFailure(base, fmt.Errorf("job admission: %w", err))
	}
	admission.MayAttach = false

	argv := make([]string, 0, 3+len(args))
	argv = append(argv, "le")
	argv = append(argv, strings.Fields(area)...)
	argv = append(argv, args...)
	ticket, err := admission.Admit(admitLabel, argv)
	if err != nil {
		return setupFailure(base, fmt.Errorf("job admission: %w", err))
	}

	switch ticket.Kind {
	case job.KindInside:
		// A parent already holds the slot and is named in the environment
		// every child inherits.
		return run(ctx, root, opts, deps)
	case job.KindClaimed:
		return runClaimed(ctx, root, opts, deps, ticket)
	case job.KindUnspecified, job.KindAttached, job.KindUnadmitted:
	}
	panic("BUG: a stress run that may not attach was answered a ticket of kind " + ticket.Kind.String())
}

// runClaimed runs the stress run in the slot the ticket claimed and releases it
// with the run's code.
//
// The entry is named as the job parent for the run's duration, then restored,
// so every child `le test <suite>` runs inside this slot. The capture log is
// teed to the slot's log, because the registry breaker reads that growth as
// liveness. The log grows once per completed invocation, so a silence lasts at
// most one invocation timeout.
func runClaimed(ctx context.Context, root string, opts Options, deps runDependencies, ticket *job.Ticket) (Report, int) {
	base := Report{Suite: opts.Suite, Test: opts.Test, Iterations: opts.Iterations, Race: opts.Race}

	//nolint:gosec // this run's own registry log, named from a validated label
	progress, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(ticket.Log)),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		ticket.Release(2)
		return setupFailure(base, fmt.Errorf("open job log: %w", err))
	}
	defer func() { _ = progress.Close() }()

	restoreParent, err := job.NameParent(filepath.Join(root, filepath.FromSlash(ticket.Entry)))
	if err != nil {
		ticket.Release(2)
		return setupFailure(base, fmt.Errorf("name the job parent: %w", err))
	}
	deps.progress = progress
	report, code := run(ctx, root, opts, deps)
	if err := restoreParent(); err != nil {
		leaction.ReportError(fmt.Errorf("restore %s: %w", job.ParentKey, err))
	}
	ticket.Release(code)
	return report, code
}

func run(ctx context.Context, root string, opts Options, deps runDependencies) (Report, int) {
	if deps.cpuCount <= 0 {
		deps.cpuCount = 8
	}
	parallel := opts.Parallel
	if parallel == 0 {
		parallel = max(2, deps.cpuCount/2)
	}
	burnerCount := opts.Burners
	if burnerCount == 0 {
		burnerCount = 2 * deps.cpuCount
	}
	report := Report{
		Suite: opts.Suite, Test: opts.Test, Burners: burnerCount, Parallel: parallel,
		Iterations: opts.Iterations, Race: opts.Race,
	}

	outDir := filepath.Join(root, "tmp", "stress-repro")
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return setupFailure(report, fmt.Errorf("create capture directory: %w", err))
	}
	var slug string
	var harness []string
	var err error
	if deps.selected != nil {
		slug = runSlugWords([]string{opts.Suite, opts.Test})
		harness = slices.Clone(deps.selected.request.Runner)
	} else {
		slug, err = runSlug(opts.Suite, opts.Test)
		if err == nil {
			harness, err = harnessArgs(opts.Suite)
		}
	}
	if err != nil {
		return setupFailure(report, err)
	}
	// Resolved before the daemon build and the burners, so a suite name that
	// runs nothing is refused before any load starts.
	stamp := deps.now().Format("20060102-150405")

	// The daemon under test is built from the tree in hand unless ZE_BIN pins
	// one. The shared bin/ze is never a fallback: it is whatever some earlier
	// build left there, and a verdict over it says nothing about this tree.
	var testBin, zeBin string
	var pinned bool
	if deps.selected != nil {
		testBin = deps.selected.request.Executable
		zeBin, pinned = deps.selected.zeBin, true
	} else {
		testBin, err = os.Executable()
		if err != nil {
			return setupFailure(report, fmt.Errorf("the harness is this le, and it cannot name its own file: %w", err))
		}
		zeBin, pinned = pinnedBinary("ze.bin")
	}
	if opts.Race || !pinned {
		built, buildErr := buildDaemon(ctx, root, outDir, opts, deps)
		if buildErr != nil {
			return setupFailure(report, buildErr)
		}
		zeBin = built
		defer func() { _ = os.RemoveAll(filepath.Dir(filepath.Dir(built))) }()
	}
	if err := ensureBinaries(zeBin, testBin); err != nil {
		return setupFailure(report, err)
	}

	spec := invocation{
		root: root, harness: harness, test: opts.Test, zeBin: zeBin, testBin: testBin,
		timeout: time.Duration(opts.Timeout) * time.Second, extraTags: opts.Tags,
	}
	if deps.selected != nil {
		spec.selector = deps.selected.request.Selector
		spec.environment = deps.selected.request.Environment
		spec.timeout = deps.selected.request.Timeout
	}
	argv, err := spec.argv()
	if err != nil {
		return setupFailure(report, err)
	}
	if deps.selected != nil {
		report.Command = append([]string{testBin}, argv...)
	}
	// The digest keeps the full selection's identity after sanitizing and
	// bounding its readable prefix. Exclusive creation keeps fresh samples
	// separate even when their selection and timestamp are identical.
	identity := sha256.Sum256([]byte(opts.Suite + "\x00" + opts.Test))
	pattern := slug + "-" + hex.EncodeToString(identity[:]) + "-" + stamp + "-*.log"
	captureFile, err := os.CreateTemp(outDir, pattern)
	if err != nil {
		return setupFailure(report, fmt.Errorf("create capture log: %w", err))
	}
	report.Log = captureFile.Name()
	defer func() { _ = captureFile.Close() }()
	var log io.Writer = captureFile
	if deps.progress != nil {
		log = io.MultiWriter(captureFile, deps.progress)
	}
	_, _ = fmt.Fprintf(log, "stress-repro %s %s\nburners=%d parallel=%d race=%t ncpu=%d\n",
		opts.Suite, stamp, burnerCount, parallel, opts.Race, deps.cpuCount)
	if _, err := io.WriteString(log, "argv="); err != nil {
		return setupFailure(report, fmt.Errorf("record capture argv: %w", err))
	}
	if err := json.NewEncoder(log).Encode(argv); err != nil {
		return setupFailure(report, fmt.Errorf("record capture argv: %w", err))
	}

	deadline := deps.now().Add(time.Duration(opts.Minutes * float64(time.Minute)))
	if deps.selected != nil {
		deadline = deps.now().Add(deps.selected.request.Budget)
	}
	runCtx, cancelRun := context.WithDeadline(ctx, deadline)
	defer cancelRun()
	stopBurners := deps.burners(runCtx, burnerCount)
	defer stopBurners()

	// ran counts the invocations that finished on their own, and failed the
	// ones among them that exited non-zero without counting as a hit. When
	// every finished invocation failed, nothing passed, so "not reproduced"
	// would be a verdict about a run that tested nothing.
	ran, failed := 0, 0
	for report.Completed < opts.Iterations && runCtx.Err() == nil && !report.Reproduced && report.SetupError == "" {
		batch := min(parallel, opts.Iterations-report.Completed)
		batchSpec := spec
		batchSpec.parallelFactor = 1
		if batch > 1 {
			batchSpec.parallelFactor = runner.ParallelTimeoutHeadroom
		}
		batchCtx, cancelBatch := context.WithCancel(runCtx)
		results := make(chan processResult, batch)
		for range batch {
			go func() { results <- deps.runner.Invoke(batchCtx, batchSpec) }()
		}
		for range batch {
			result := <-results
			if report.Reproduced || report.SetupError != "" {
				continue
			}
			report.Completed++
			if runCtx.Err() != nil &&
				(errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded)) {
				continue
			}
			if usage := usageErrorSignature(result.output); usage != "" {
				_, _ = fmt.Fprintf(log, "\n===== invocation %d USAGE ERROR =====\n%s", report.Completed, result.output)
				report.SetupError = fmt.Sprintf("%q never reached a test (%s); check the suite name", opts.Suite, usage)
				cancelBatch()
				continue
			}
			if result.err != nil && !errors.Is(result.err, context.Canceled) && !errors.Is(result.err, context.DeadlineExceeded) {
				_, _ = fmt.Fprintf(log, "\n===== invocation %d SETUP ERROR =====\n%s\n", report.Completed, result.err)
				report.SetupError = "could not run le test: " + result.err.Error()
				cancelBatch()
				continue
			}
			ran++
			if result.code != 0 {
				report.Failed++
			}
			signature := crashSignature(result.output)
			hit := signature != "" || (opts.AnyFailure && result.code != 0)
			if deps.selected != nil && !hit {
				count, err := deps.selected.request.Count(result.output)
				if err != nil {
					report.SetupError = err.Error()
					_, _ = fmt.Fprintf(log, "\n===== invocation %d POPULATION ERROR =====\n%s\n%s", report.Completed, err, result.output)
					cancelBatch()
					continue
				}
				if count != 1 {
					report.SetupError = "native stress child did not execute exactly one selected test"
					cancelBatch()
					continue
				}
				report.Executed += count
			}
			if !hit && result.code != 0 {
				failed++
			}
			label := "ok"
			if signature != "" {
				label = "CRASH:" + signature
			} else if result.code != 0 {
				label = "FAIL"
			}
			_, _ = fmt.Fprintf(log, "\n===== invocation %d exit=%d %s =====\n", report.Completed, result.code, label)
			if hit {
				_, _ = io.WriteString(log, result.output)
				report.Reproduced = true
				report.Exit = result.code
				report.Signature = signature
				if signature != "" {
					report.Excerpt = crashExcerpt(result.output)
				}
				cancelBatch()
			} else {
				_, _ = io.WriteString(log, outputTail(result.output, 500)+"\n")
			}
		}
		cancelBatch()
	}
	if report.SetupError != "" {
		return report, 2
	}
	if report.Reproduced {
		return report, 0
	}
	report.Interrupted = ctx.Err() != nil
	if report.Interrupted {
		return report, 1
	}
	if ran == 0 {
		report.SetupError = "no invocation finished before the deadline, so the run has no verdict; raise minutes or timeout"
		return report, 2
	}
	if failed == ran {
		var tb textbuf.Buffer
		report.SetupError = tb.Str("every one of ").Str(strconv.Itoa(ran)).
			Str(" invocation(s) failed without a crash signature, so nothing passed and the run has no verdict; read ").
			Str(report.Log).Str(", and pass any-failure if the failure is the reproduction").String()
		return report, 2
	}
	return report, 1
}

func setupFailure(report Report, err error) (Report, int) {
	report.SetupError = err.Error()
	return report, 2
}

func ensureBinaries(paths ...string) error {
	var missing []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			missing = append(missing, path)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("missing prebuilt binaries: %v; point ZE_BIN at a built ze, or unset it so the run builds one from the tree", missing)
}

// pinnedBinary answers the path an environment key names, in its dotted or
// upper-case spelling, and whether the key was set at all.
func pinnedBinary(key string) (string, bool) {
	for _, spelling := range []string{key, strings.ToUpper(strings.ReplaceAll(key, ".", "_"))} {
		if value := os.Getenv(spelling); value != "" {
			absolute, err := filepath.Abs(value)
			if err == nil {
				return absolute, true
			}
			return value, true
		}
	}
	return "", false
}

// buildDaemon compiles ze from the working tree into a directory of its own
// under outDir and answers the binary's path. The binary sits in a bin/
// directory because ze derives its other directories from that parent. The
// ze-fakefilter test plugin is built beside it, as runner.Build does, because
// the runner puts ze's directory on every child's PATH and a .ci forks it from
// there. The caller MUST remove the directory two levels above the answer when
// done.
func buildDaemon(ctx context.Context, root, outDir string, opts Options, deps runDependencies) (string, error) {
	tags, err := daemonTags(root, opts.Tags)
	if err != nil {
		return "", err
	}
	name := "ze-"
	if opts.Race {
		name = "ze-race-"
	}
	binDir := filepath.Join(outDir, name+strconv.Itoa(deps.pid), "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return "", fmt.Errorf("create build directory: %w", err)
	}
	output := filepath.Join(binDir, "ze")
	build := deps.runner.build(ctx, root, output, tags, opts.Race)
	if !build.succeeded() {
		return "", buildFailure(binDir, build)
	}
	plugin := deps.runner.buildPlugin(ctx, root, filepath.Join(binDir, runner.FakeFilterBinary), runner.FakeFilterPackage)
	if !plugin.succeeded() {
		return "", buildFailure(binDir, plugin)
	}
	return output, nil
}

// buildFailure removes the build directory above binDir and answers the
// failed build's error, its output or, when it printed none, its error.
func buildFailure(binDir string, build processResult) error {
	_ = os.RemoveAll(filepath.Dir(binDir))
	message := strings.TrimSpace(build.output)
	if message == "" && build.err != nil {
		message = build.err.Error()
	}
	return fmt.Errorf("stress build failed: %s", message)
}

// daemonBase is the personality a repro compiles, before the gates
// featuretags adds to it: the daemon, plus the setup surface the repro drives,
// plus the test-only plugins (fakeredist and the rest) a functional test names.
// The functional DUT build carries the same tag (runner.TestBuildTags); without
// it a repro of such a test fails at load, not at the race it is hunting.
const daemonBase = "ze_core ze_distro ze_setup " + runner.TestPluginBuildTag

// daemonTags answers the `-tags` value for the daemon build, with the caller's
// extra tag last.
func daemonTags(root, extra string) (string, error) {
	tags, err := repofeaturetags.DaemonBuildTags(root, daemonBase)
	if err != nil {
		return "", fmt.Errorf("read feature gates: %w", err)
	}
	if extra == "" {
		return tags, nil
	}

	var tb textbuf.Buffer
	return tb.Str(tags).Byte(' ').Str(extra).String(), nil
}

// runSlugMax leaves room below NAME_MAX=255 for the full SHA-256, timestamp,
// exclusive-creation suffix and extension in run's capture filename.
const runSlugMax = 96

func runSlug(suite, test string) (string, error) {
	words, err := shellWords(suite, test)
	if err != nil {
		return "", fmt.Errorf("split suite or test: %w", err)
	}
	return runSlugWords(words), nil
}

func runSlugWords(words []string) string {
	var slug textbuf.Buffer
	for _, word := range words {
		clean := slugPart(word)
		if clean == "" {
			continue
		}
		if slug.Len() > 0 {
			slug.Byte('-')
		}
		remaining := runSlugMax - slug.Len()
		slug.Str(clean[:min(len(clean), remaining)])
		if slug.Len() == runSlugMax {
			break
		}
	}
	if slug.Len() == 0 {
		return "run"
	}
	return slug.String()
}

func slugPart(word string) string {
	var clean textbuf.Buffer
	clean.Grow(len(word))
	invalid := false
	for _, r := range word {
		valid := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'
		if valid {
			clean.WriteRune(r)
			invalid = false
			continue
		}
		if !invalid {
			clean.Byte('-')
			invalid = true
		}
	}
	return strings.Trim(clean.String(), "-")
}

// harnessArgs derives the child's argv after `le test` from the functional suite
// registry (testfunctional.Suites), so `suite plugin` runs `le test bgp plugin`
// and `suite ui` runs `le test ui`: a suite name is never passed through as a
// `le test` verb: passed through, plugin and reload launched a `le test <suite>` line that met
// "test is a namespace". The first word of suite names the registered suite; the
// words after it (`--draft`) follow the suite's own arguments. The registry's
// select-everything word is dropped, because Invoke chooses the selector: `--all`
// or the `test` words. Every other registered argument, such as reload's serial
// `-p 1`, is kept, because it is part of what that suite needs to run.
func harnessArgs(suite string) ([]string, error) {
	words, err := shellWords(suite)
	if err != nil {
		return nil, fmt.Errorf("split suite: %w", err)
	}
	if len(words) == 0 {
		return nil, errors.New("run requires suite <suite>")
	}
	registered, found := testfunctional.SuiteNamed(words[0])
	if !found {
		return nil, unknownSuiteError(words[0])
	}
	args := make([]string, 0, len(registered.Args)+len(words)-1)
	for _, word := range registered.Args {
		if word == testfunctional.AllTests {
			continue
		}
		args = append(args, word)
	}
	return append(args, words[1:]...), nil
}

// unknownSuiteError names the word, the suites that exist, and the command to
// type, all read from the registry the run would launch from.
func unknownSuiteError(name string) error {
	var message textbuf.Buffer
	message.Str("suite ").Quoted(name).
		Str(" is not a functional test suite, so stress-repro has no le test command to run\n").
		Str("valid suites: ")
	for index, registered := range testfunctional.Suites {
		if index > 0 {
			message.Str(", ")
		}
		message.Str(registered.Name)
	}
	message.Str("\nrun one by its name, for example:\n  ./le test stress-repro run suite ").
		Str(testfunctional.Suites[0].Name)
	return errors.New(message.String())
}

func shellWords(values ...string) ([]string, error) {
	var words []string
	for _, value := range values {
		if value == "" {
			continue
		}
		split, err := shlex.Split(value, true)
		if err != nil {
			return nil, err
		}
		words = append(words, split...)
	}
	return words, nil
}

func usageErrorSignature(output string) string {
	if !strings.Contains(output, usageBanner) {
		return ""
	}
	for _, signature := range usageSignatures {
		if strings.Contains(output, signature) {
			return signature
		}
	}
	return ""
}

func crashExcerpt(output string) string {
	lines := strings.Split(output, "\n")
	index := -1
	for i, line := range lines {
		if crashSignature(line) != "" {
			index = i
			break
		}
	}
	if index < 0 {
		return ""
	}
	low := max(0, index-2)
	high := min(len(lines), index+40)
	return strings.Join(lines[low:high], "\n")
}

func crashSignature(output string) string {
	for _, signature := range crashSignatures {
		if strings.Contains(output, signature) {
			return signature
		}
	}
	return ""
}

func outputTail(output string, count int) string {
	runes := []rune(output)
	if len(runes) <= count {
		return output
	}
	return string(runes[len(runes)-count:])
}

func startBurners(ctx context.Context, count int) func() {
	burnCtx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	workers.Add(count)
	for range count {
		go func() {
			defer workers.Done()
			burn(burnCtx)
		}()
	}
	return func() {
		cancel()
		workers.Wait()
	}
}

func burn(ctx context.Context) {
	var state uint32
	sink := make([][]byte, 0, 97)
	for ctx.Err() == nil {
		for range 50_000 {
			state = state*1103515245 + 12345
		}
		sink = append(sink, make([]byte, 64*1024))
		if len(sink) > 96 {
			copy(sink, sink[len(sink)/2:])
			sink = sink[:len(sink)-len(sink)/2]
		}
	}
	runtime.KeepAlive(state)
	runtime.KeepAlive(sink)
}
