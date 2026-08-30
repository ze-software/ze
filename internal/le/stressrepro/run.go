// Design: docs/functional-tests.md -- bounded concurrent reproduction under CPU and GC pressure
// Detail: process.go -- process-group execution and race builds

package stressrepro

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/anmitsu/go-shlex"
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
	Suite       string `json:"suite"`
	Test        string `json:"test,omitempty"`
	Burners     int    `json:"burners"`
	Parallel    int    `json:"parallel"`
	Iterations  int    `json:"iterations"`
	Completed   int    `json:"completed"`
	Race        bool   `json:"race"`
	Reproduced  bool   `json:"reproduced"`
	Exit        int    `json:"failure-exit,omitempty"`
	Signature   string `json:"signature,omitempty"`
	Excerpt     string `json:"excerpt,omitempty"`
	Log         string `json:"log,omitempty"`
	SetupError  string `json:"setup-error,omitempty"`
	Interrupted bool   `json:"interrupted,omitempty"`
}

// Text preserves the old tool's concise result and capture-path contract.
func (r Report) Text() string {
	var b strings.Builder
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
			_, _ = b.WriteString("--- crash excerpt ---\n")
			for line := range strings.SplitSeq(r.Excerpt, "\n") {
				_, _ = fmt.Fprintf(&b, "  %s\n", line)
			}
			_, _ = b.WriteString("--- end excerpt ---\n")
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
	root      string
	suite     string
	test      string
	zeBin     string
	testBin   string
	timeout   time.Duration
	extraTags string
}

type processResult struct {
	code   int
	output string
	err    error
}

type processRunner interface {
	Invoke(context.Context, invocation) processResult
	buildRace(context.Context, string, string, string) processResult
}

type runDependencies struct {
	runner   processRunner
	now      func() time.Time
	cpuCount int
	pid      int
	burners  func(context.Context, int) func()
}

func realDependencies() runDependencies {
	return runDependencies{
		runner: realProcessRunner{}, now: time.Now, cpuCount: runtime.NumCPU(), pid: os.Getpid(), burners: startBurners,
	}
}

// runAt installs signal cancellation and runs the reproducer in root.
func runAt(root string, opts Options) (Report, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, root, opts, realDependencies())
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
	slug, err := runSlug(opts.Suite, opts.Test)
	if err != nil {
		return setupFailure(report, err)
	}
	stamp := deps.now().Format("20060102-150405")
	report.Log = filepath.Join(outDir, slug+"-"+stamp+".log")

	zeBin := binaryFromEnvironment(root, "ze.bin", "ze")
	testBin := binaryFromEnvironment(root, "ze.test.bin", "ze-test")
	var raceBin string
	if opts.Race {
		tags, tagErr := raceTags(root, opts.Tags)
		if tagErr != nil {
			return setupFailure(report, tagErr)
		}
		raceBin = filepath.Join(outDir, fmt.Sprintf("ze-race-%d", deps.pid))
		build := deps.runner.buildRace(ctx, root, raceBin, tags)
		if build.err != nil || build.code != 0 {
			message := strings.TrimSpace(build.output)
			if message == "" && build.err != nil {
				message = build.err.Error()
			}
			return setupFailure(report, fmt.Errorf("stress build failed: %s", message))
		}
		zeBin = raceBin
		defer func() { _ = os.Remove(raceBin) }()
	}
	if err := ensureBinaries(zeBin, testBin); err != nil {
		return setupFailure(report, err)
	}

	log, err := os.Create(report.Log)
	if err != nil {
		return setupFailure(report, fmt.Errorf("create capture log: %w", err))
	}
	defer func() { _ = log.Close() }()
	_, _ = fmt.Fprintf(log, "stress-repro %s %s\nburners=%d parallel=%d race=%t ncpu=%d\n",
		opts.Suite, stamp, burnerCount, parallel, opts.Race, deps.cpuCount)

	deadline := deps.now().Add(time.Duration(opts.Minutes * float64(time.Minute)))
	runCtx, cancelRun := context.WithDeadline(ctx, deadline)
	defer cancelRun()
	stopBurners := deps.burners(runCtx, burnerCount)
	defer stopBurners()

	spec := invocation{
		root: root, suite: opts.Suite, test: opts.Test, zeBin: zeBin, testBin: testBin,
		timeout: time.Duration(opts.Timeout) * time.Second, extraTags: opts.Tags,
	}
	for report.Completed < opts.Iterations && runCtx.Err() == nil && !report.Reproduced {
		batch := min(parallel, opts.Iterations-report.Completed)
		batchCtx, cancelBatch := context.WithCancel(runCtx)
		results := make(chan processResult, batch)
		for range batch {
			go func() { results <- deps.runner.Invoke(batchCtx, spec) }()
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
				report.SetupError = "could not run ze-test: " + result.err.Error()
				cancelBatch()
				continue
			}
			signature := crashSignature(result.output)
			hit := signature != "" || (opts.AnyFailure && result.code != 0)
			label := "ok"
			if signature != "" {
				label = "CRASH:" + signature
			} else if result.code != 0 {
				label = "FAIL"
			}
			_, _ = fmt.Fprintf(log, "\n===== invocation %d exit=%d %s =====\n", report.Completed, result.code, label)
			if hit {
				_, _ = log.WriteString(result.output)
				report.Reproduced = true
				report.Exit = result.code
				report.Signature = signature
				if signature != "" {
					report.Excerpt = crashExcerpt(result.output)
				}
				cancelBatch()
			} else {
				_, _ = log.WriteString(outputTail(result.output, 500) + "\n")
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
	return fmt.Errorf("missing prebuilt binaries: %v; build them first with a canonical native run, for example `ZE_TEST_CANONICAL=1 ./le functional parse`", missing)
}

func binaryFromEnvironment(root, key, name string) string {
	for _, spelling := range []string{key, strings.ToUpper(strings.ReplaceAll(key, ".", "_"))} {
		if value := os.Getenv(spelling); value != "" {
			absolute, err := filepath.Abs(value)
			if err == nil {
				return absolute
			}
			return value
		}
	}
	return filepath.Join(root, "bin", name)
}

func raceTags(root, extra string) (string, error) {
	file, err := os.Open(filepath.Join(root, "feature-gates.txt")) //nolint:gosec // the tracked feature-gate manifest under the checkout root
	if err != nil {
		return "", fmt.Errorf("read feature gates: %w", err)
	}
	defer func() { _ = file.Close() }()
	tagSet := map[string]struct{}{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tagSet[strings.Fields(line)[0]] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read feature gates: %w", err)
	}
	tags := []string{"ze_core", "ze_distro", "ze_setup"}
	features := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		features = append(features, tag)
	}
	sort.Strings(features)
	tags = append(tags, features...)
	if extra != "" {
		tags = append(tags, extra)
	}
	return strings.Join(tags, " "), nil
}

func runSlug(suite, test string) (string, error) {
	words, err := shellWords(suite, test)
	if err != nil {
		return "", fmt.Errorf("split suite or test: %w", err)
	}
	parts := make([]string, 0, len(words))
	for _, word := range words {
		clean := slugPart(word)
		if clean != "" {
			parts = append(parts, clean)
		}
	}
	if len(parts) == 0 {
		return "run", nil
	}
	return strings.Trim(strings.Join(parts, "-"), "-"), nil
}

func slugPart(word string) string {
	var clean strings.Builder
	clean.Grow(len(word))
	invalid := false
	for _, r := range word {
		valid := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'
		if valid {
			_, _ = clean.WriteRune(r)
			invalid = false
			continue
		}
		if !invalid {
			_ = clean.WriteByte('-')
			invalid = true
		}
	}
	return strings.Trim(clean.String(), "-")
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
