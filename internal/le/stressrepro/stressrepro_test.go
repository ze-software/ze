package stressrepro

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/leroot"
)

func TestOldOptionTableMapsExactlyToKeywords(t *testing.T) {
	args := []string{
		"suite", "bgp plugin", "test", "test/web/commit-flow.wb", "iterations", "17",
		"parallel", "3", "burners", "5", "minutes", "1.5", "timeout", "42",
		"race", "any-failure", "tags", "ze_bgp ze_ospf",
	}
	got, err := parseOptions(args)
	if err != nil {
		t.Fatalf("parse complete old table: %v", err)
	}
	want := Options{
		Suite: "bgp plugin", Test: "test/web/commit-flow.wb", Iterations: 17,
		Parallel: 3, Burners: 5, Minutes: 1.5, Timeout: 42, Race: true,
		AnyFailure: true, Tags: "ze_bgp ze_ospf",
	}
	if got != want {
		t.Fatalf("options = %#v, want %#v", got, want)
	}
}

func TestOldDefaultsAreUnchanged(t *testing.T) {
	got, err := parseOptions([]string{"suite", "rsvpte"})
	if err != nil {
		t.Fatal(err)
	}
	want := Options{Suite: "rsvpte", Iterations: 80, Minutes: 20, Timeout: 120}
	if got != want {
		t.Fatalf("defaults = %#v, want %#v", got, want)
	}
}

func TestNoInventedAliasesOrFlagSpellings(t *testing.T) {
	for _, args := range [][]string{
		{"suite", "bgp", "duration", "1m"},
		{"suite", "bgp", "workers", "2"},
		{"suite", "bgp", "seed", "9"},
		{"suite", "bgp", "--iterations", "2"},
	} {
		if _, err := parseOptions(args); err == nil {
			t.Errorf("accepted grammar outside the old table: %v", args)
		}
	}
}

func TestActionComesBeforeSuiteAndBareCommandListsGrammar(t *testing.T) {
	payload, code := Answer(nil)
	if code != 0 {
		t.Fatalf("bare command exited %d", code)
	}
	listing, ok := payload.(ActionList)
	if !ok || len(listing.Actions) != 1 || listing.Actions[0].Action != "run" {
		t.Fatalf("bare command = %#v", payload)
	}
	for _, keyword := range []string{
		"test", "iterations", "parallel", "burners", "minutes", "timeout", "race", "any-failure", "tags",
	} {
		if !strings.Contains(listing.Actions[0].Usage, keyword) {
			t.Errorf("run grammar omits %q: %s", keyword, listing.Actions[0].Usage)
		}
	}
	if _, code := Answer([]string{"bgp"}); code != 2 {
		t.Errorf("identifier-first invocation exited %d, want 2", code)
	}
}

func TestRegistrationOwnsTheArgumentAwareAction(t *testing.T) {
	if !leroot.Owns(area) {
		t.Fatalf("le does not own %q", area)
	}
	if got := Subs(); !strings.HasPrefix(got, "run suite <suite>") {
		t.Fatalf("argument-aware hint = %q", got)
	}
}

func TestOptionValueBoundsAndTypesAreRefused(t *testing.T) {
	cases := [][]string{
		{"suite", "bgp", "iterations", "-1"},
		{"suite", "bgp", "parallel", "-1"},
		{"suite", "bgp", "burners", "-1"},
		{"suite", "bgp", "minutes", "-1"},
		{"suite", "bgp", "timeout", "0"},
		{"suite", "bgp", "iterations", "many"},
		{"suite", "bgp", "minutes", "long"},
		{"suite", "bgp", "minutes", "NaN"},
		{"suite", "bgp", "timeout", "9223372036854775807"},
		{"suite"},
		{"test", "4"},
	}
	for _, args := range cases {
		if _, err := parseOptions(args); err == nil {
			t.Errorf("accepted invalid options %v", args)
		}
	}
}

func TestSuiteAndSelectorUseTheOldWhitespaceSplitting(t *testing.T) {
	words, err := shellWords(`bgp "plugin suite"`, `test/web/commit-flow.wb 97`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"bgp", "plugin suite", "test/web/commit-flow.wb", "97"}; !slices.Equal(words, want) {
		t.Fatalf("words = %q, want %q", words, want)
	}
	slug, err := runSlug("bgp plugin", "test/web/commit-flow.wb")
	if err != nil {
		t.Fatal(err)
	}
	if slug != "bgp-plugin-test-web-commit-flow.wb" || strings.ContainsRune(slug, '/') {
		t.Fatalf("unsafe slug %q", slug)
	}
}

type fakeRunner struct {
	mu          sync.Mutex
	results     []processResult
	invocations []invocation
	build       processResult
	buildRoot   string
	buildOutput string
	buildTags   string
	createBuild bool
	waitForStop bool
	started     chan struct{}
	stopped     chan struct{}
}

func (f *fakeRunner) Invoke(ctx context.Context, spec invocation) processResult {
	f.mu.Lock()
	index := len(f.invocations)
	f.invocations = append(f.invocations, spec)
	var result processResult
	if index < len(f.results) {
		result = f.results[index]
	}
	wait := f.waitForStop
	started := f.started
	stopped := f.stopped
	f.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if wait {
		<-ctx.Done()
		if stopped != nil {
			select {
			case stopped <- struct{}{}:
			default:
			}
		}
		return processResult{code: 1, err: ctx.Err()}
	}
	return result
}

func (f *fakeRunner) buildRace(_ context.Context, root, output, tags string) processResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buildRoot, f.buildOutput, f.buildTags = root, output, tags
	if f.createBuild {
		if err := os.WriteFile(output, []byte("race"), 0o755); err != nil {
			return processResult{code: 2, err: err}
		}
	}
	return f.build
}

func (f *fakeRunner) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.invocations)
}

func stressTree(t *testing.T) string {
	t.Helper()
	for _, key := range []string{"ze.bin", "ZE_BIN", "ze.test.bin", "ZE_TEST_BIN"} {
		t.Setenv(key, "")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ze", "ze-test"} {
		if err := os.WriteFile(filepath.Join(root, "bin", name), []byte("stub"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "feature-gates.txt"), []byte("ze_ospf x\nze_bgp x\nze_bgp y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func testDependencies(runner processRunner) (runDependencies, *int) {
	stops := 0
	return runDependencies{
		runner: runner, now: time.Now, cpuCount: 8, pid: 123,
		burners: func(context.Context, int) func() { return func() { stops++ } },
	}, &stops
}

func baseOptions() Options {
	return Options{Suite: "bgp", Iterations: 1, Parallel: 1, Burners: 1, Minutes: 1, Timeout: 1}
}

func TestCPUDefaultsMatchTheOldScript(t *testing.T) {
	root := stressTree(t)
	fake := &fakeRunner{results: []processResult{{}}}
	deps, _ := testDependencies(fake)
	opts := baseOptions()
	opts.Parallel, opts.Burners = 0, 0
	report, code := run(context.Background(), root, opts, deps)
	if code != 1 || report.Parallel != 4 || report.Burners != 16 {
		t.Fatalf("report/code = %#v/%d", report, code)
	}
}

func TestCrashSignatureReproducesEvenWithZeroExitAndCapturesFullOutput(t *testing.T) {
	root := stressTree(t)
	full := strings.Repeat("x", 700) + "\npanic: broken\nstack"
	fake := &fakeRunner{results: []processResult{{output: full}}}
	deps, stops := testDependencies(fake)
	report, code := run(context.Background(), root, baseOptions(), deps)
	if code != 0 || !report.Reproduced || report.Signature != "panic:" || report.Exit != 0 {
		t.Fatalf("report/code = %#v/%d", report, code)
	}
	if !strings.Contains(report.Excerpt, "panic: broken") ||
		!strings.Contains(report.Text(), "--- crash excerpt ---") {
		t.Fatalf("crash excerpt was not reported: %#v", report)
	}
	capture, err := os.ReadFile(report.Log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capture), full) {
		t.Fatal("first crash output was truncated")
	}
	if *stops != 1 {
		t.Fatalf("burners stopped %d times", *stops)
	}
}

func TestAnyFailureControlsPlainNonzeroExit(t *testing.T) {
	for _, test := range []struct {
		name string
		any  bool
		code int
	}{
		{name: "crashes only", code: 1},
		{name: "any failure", any: true, code: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := stressTree(t)
			fake := &fakeRunner{results: []processResult{{code: 9, output: "assertion failed"}}}
			deps, _ := testDependencies(fake)
			opts := baseOptions()
			opts.AnyFailure = test.any
			report, code := run(context.Background(), root, opts, deps)
			if code != test.code || report.Reproduced != test.any ||
				test.any && report.Exit != 9 {
				t.Fatalf("report/code = %#v/%d", report, code)
			}
		})
	}
}

func TestUsageErrorIsSetupFailureEvenInAnyFailureMode(t *testing.T) {
	root := stressTree(t)
	output := "unknown suite: reload\nze-test\n\nCommands:\n  bgp Run BGP\n"
	fake := &fakeRunner{results: []processResult{{code: 1, output: output}}}
	deps, _ := testDependencies(fake)
	opts := baseOptions()
	opts.AnyFailure = true
	report, code := run(context.Background(), root, opts, deps)
	if code != 2 || report.Reproduced || !strings.Contains(report.SetupError, "never reached a test") {
		t.Fatalf("report/code = %#v/%d", report, code)
	}
}

func TestQuotedUsagePhraseInsideARealFailureIsNotSetupError(t *testing.T) {
	root := stressTree(t)
	output := "stderr does not contain unknown command: traffic-control\nfail 1/40\n"
	fake := &fakeRunner{results: []processResult{{code: 1, output: output}}}
	deps, _ := testDependencies(fake)
	opts := baseOptions()
	opts.AnyFailure = true
	report, code := run(context.Background(), root, opts, deps)
	if code != 0 || !report.Reproduced || report.SetupError != "" {
		t.Fatalf("report/code = %#v/%d", report, code)
	}
}

func TestTimeoutIsExit124AndOnlyAnyFailureCapturesIt(t *testing.T) {
	root := stressTree(t)
	fake := &fakeRunner{results: []processResult{{code: 124, output: "timed out", err: context.DeadlineExceeded}}}
	deps, _ := testDependencies(fake)
	opts := baseOptions()
	opts.AnyFailure = true
	report, code := run(context.Background(), root, opts, deps)
	if code != 0 || report.Exit != 124 {
		t.Fatalf("report/code = %#v/%d", report, code)
	}
}

func TestIterationsRunInBoundedParallelRounds(t *testing.T) {
	root := stressTree(t)
	fake := &fakeRunner{results: make([]processResult, 7)}
	deps, _ := testDependencies(fake)
	opts := baseOptions()
	opts.Iterations, opts.Parallel = 7, 3
	report, code := run(context.Background(), root, opts, deps)
	if code != 1 || report.Completed != 7 || fake.count() != 7 {
		t.Fatalf("report/code/calls = %#v/%d/%d", report, code, fake.count())
	}
}

func TestFirstCompletedFailureCancelsAndWaitsForSibling(t *testing.T) {
	root := stressTree(t)
	started := make(chan struct{}, 2)
	stopped := make(chan struct{}, 2)
	fake := &fakeRunner{
		results: []processResult{{output: "fatal error: first"}},
		started: started, stopped: stopped,
	}
	// The second call waits for cancellation while the first returns the hit.
	fake.results = append(fake.results, processResult{})
	deps, _ := testDependencies(&orderedRunner{first: fake, stopped: stopped})
	opts := baseOptions()
	opts.Iterations, opts.Parallel = 2, 2
	report, code := run(context.Background(), root, opts, deps)
	if code != 0 || report.Signature != "fatal error:" || report.Completed != 1 {
		t.Fatalf("report/code = %#v/%d", report, code)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("in-flight sibling was not canceled and awaited")
	}
}

type orderedRunner struct {
	mu      sync.Mutex
	calls   int
	first   *fakeRunner
	stopped chan struct{}
}

func (r *orderedRunner) Invoke(ctx context.Context, spec invocation) processResult {
	r.mu.Lock()
	call := r.calls
	r.calls++
	r.mu.Unlock()
	if call == 0 {
		return processResult{output: "fatal error: first"}
	}
	<-ctx.Done()
	r.stopped <- struct{}{}
	return processResult{code: 1, err: ctx.Err()}
}

func (r *orderedRunner) buildRace(ctx context.Context, root, output, tags string) processResult {
	return r.first.buildRace(ctx, root, output, tags)
}

func TestCancellationStopsInvocationsAndBurners(t *testing.T) {
	root := stressTree(t)
	fake := &fakeRunner{waitForStop: true, started: make(chan struct{}, 1), stopped: make(chan struct{}, 1)}
	deps, stops := testDependencies(fake)
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	var report Report
	var code int
	go func() {
		report, code = run(ctx, root, baseOptions(), deps)
		close(finished)
	}()
	<-fake.started
	cancel()
	<-finished
	if code != 1 || !report.Interrupted || *stops != 1 {
		t.Fatalf("report/code/stops = %#v/%d/%d", report, code, *stops)
	}
	select {
	case <-fake.stopped:
	default:
		t.Fatal("invocation did not observe cancellation")
	}
}

func TestRaceBuildUsesDerivedSortedTagsAndRemovesTemporaryBinary(t *testing.T) {
	root := stressTree(t)
	fake := &fakeRunner{createBuild: true, results: []processResult{{}}}
	deps, _ := testDependencies(fake)
	opts := baseOptions()
	opts.Race, opts.Tags = true, "ze_extra"
	report, code := run(context.Background(), root, opts, deps)
	if code != 1 {
		t.Fatalf("report/code = %#v/%d", report, code)
	}
	wantTags := "ze_core ze_distro ze_setup ze_bgp ze_ospf ze_extra"
	if fake.buildRoot != root || fake.buildTags != wantTags {
		t.Fatalf("race build root/tags = %q/%q", fake.buildRoot, fake.buildTags)
	}
	if _, err := os.Stat(fake.buildOutput); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("race binary survived cleanup: %v", err)
	}
}

func TestRaceBuildFailureAndMissingBinariesAreSetupErrors(t *testing.T) {
	t.Run("build", func(t *testing.T) {
		root := stressTree(t)
		fake := &fakeRunner{build: processResult{code: 1, output: "compiler failed"}}
		deps, _ := testDependencies(fake)
		opts := baseOptions()
		opts.Race = true
		report, code := run(context.Background(), root, opts, deps)
		if code != 2 || !strings.Contains(report.SetupError, "compiler failed") || fake.count() != 0 {
			t.Fatalf("report/code/calls = %#v/%d/%d", report, code, fake.count())
		}
	})
	t.Run("missing", func(t *testing.T) {
		root := stressTree(t)
		if err := os.Remove(filepath.Join(root, "bin", "ze-test")); err != nil {
			t.Fatal(err)
		}
		fake := &fakeRunner{}
		deps, _ := testDependencies(fake)
		report, code := run(context.Background(), root, baseOptions(), deps)
		if code != 2 || !strings.Contains(report.SetupError, "missing prebuilt binaries") || fake.count() != 0 {
			t.Fatalf("report/code/calls = %#v/%d/%d", report, code, fake.count())
		}
	})
}

func TestInvocationStartFailureIsSetupError(t *testing.T) {
	root := stressTree(t)
	fake := &fakeRunner{results: []processResult{{code: 2, err: errors.New("permission denied")}}}
	deps, _ := testDependencies(fake)
	report, code := run(context.Background(), root, baseOptions(), deps)
	if code != 2 || !strings.Contains(report.SetupError, "permission denied") {
		t.Fatalf("report/code = %#v/%d", report, code)
	}
}
