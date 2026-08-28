package qemu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: the run reads its knobs from the environment the native host
// action exports, defaults every one of them, and refuses to start outside the
// guest.
// PREVENTS: host and guest actions reading different variables or defaulting to
// different concurrency.

func TestTheRunReadsEveryKnobFromTheEnvironment(t *testing.T) {
	t.Setenv("ZE_BIN", "bin/ze-linux-arm64")
	t.Setenv("ZE_STRIPPED_BIN", "bin/ze-stripped-linux-arm64")
	t.Setenv("ZE_TEST_BIN", "bin/ze-test-linux-arm64")
	t.Setenv("ZE_QEMU_SKIP_SUITES", "web, editor ,,")
	t.Setenv("ZE_QEMU_PARALLEL", "8")
	t.Setenv("ZE_QEMU_SUITE_TIMEOUT", "1200s")
	t.Setenv("GOCACHE", "/cache/build")
	t.Setenv("GOMODCACHE", "/cache/mod")

	run := newAllTests()
	if run.ZeBin != "bin/ze-linux-arm64" || run.StrippedBin != "bin/ze-stripped-linux-arm64" ||
		run.TestBin != "bin/ze-test-linux-arm64" {
		t.Errorf("the binaries are %q, %q and %q", run.ZeBin, run.StrippedBin, run.TestBin)
	}
	if run.Parallel != "8" || run.Timeout != "1200s" {
		t.Errorf("concurrency is %q and the cap is %q", run.Parallel, run.Timeout)
	}
	if run.BuildCache != "/cache/build" || run.ModuleCache != "/cache/mod" {
		t.Errorf("the caches are %q and %q", run.BuildCache, run.ModuleCache)
	}
	// The blank and empty members are dropped: a trailing comma in the make
	// variable must not become a suite named "".
	if len(run.Skip) != 2 || run.Skip[0] != "web" || run.Skip[1] != "editor" {
		t.Errorf("the skip list is %v, want exactly [web editor]", run.Skip)
	}
}

// Every knob has one native default. A different silent default would change
// the nightly run even though no action that names the knob changed.
func TestEveryKnobHasTheNativeDefault(t *testing.T) {
	for _, key := range []string{
		"ZE_BIN", "ZE_STRIPPED_BIN", "ZE_TEST_BIN",
		"ZE_QEMU_SKIP_SUITES", "ZE_QEMU_PARALLEL", "ZE_QEMU_SUITE_TIMEOUT",
		"GOCACHE", "GOMODCACHE",
	} {
		t.Setenv(key, "")
	}

	run := newAllTests()
	if run.Workspace != guestWorkspace || run.BinDir != guestBinDir {
		t.Errorf("the guest paths are %q and %q", run.Workspace, run.BinDir)
	}
	if run.Parallel != defaultParallel || run.Timeout != defaultTimeout {
		t.Errorf("the defaults are %q and %q, want %q and %q",
			run.Parallel, run.Timeout, defaultParallel, defaultTimeout)
	}
	if len(run.Skip) != 1 || run.Skip[0] != defaultSkip {
		t.Errorf("the default skip list is %v, want exactly [%s]", run.Skip, defaultSkip)
	}
	if run.ZeBin != "bin/ze" || run.StrippedBin != "bin/ze-stripped" || run.TestBin != "bin/ze-test" {
		t.Errorf("the default binaries are %q, %q and %q", run.ZeBin, run.StrippedBin, run.TestBin)
	}
	// The caches have NO default: they are the one pair the host action must
	// supply and the guest action refuses to guess.
	if run.BuildCache != "" || run.ModuleCache != "" {
		t.Errorf("the caches defaulted to %q and %q, and a default here would hide the refusal",
			run.BuildCache, run.ModuleCache)
	}
}

// When typed on a build host, the command refuses to run. The repository is not
// mounted where the guest expects it. This command must never run the guest's
// plan against a developer tree.
func TestTheVerbRefusesToRunOutsideTheGuest(t *testing.T) {
	if _, err := os.Stat(guestWorkspace); err == nil {
		t.Skip("this machine has a /workspace, so the refusal cannot be observed here")
	}

	payload, code := Answer([]string{"all-tests"})
	if code == 0 {
		t.Errorf("the run answered code 0 outside the guest, with payload %v", payload)
	}
	// A run that never started answers NO payload. An empty report beside a
	// non-zero code renders "ALL PHASES PASSED", which is a verdict over
	// nothing (plan/journal/declared-format-contradicts-payload.md).
	if payload != nil {
		t.Errorf("a run that never started answered the payload %v", payload)
	}
}

// The same guard at the rendering, for any caller that holds a report rather
// than the command's answer.
func TestAReportOfNoPhasesDoesNotRenderAsSuccess(t *testing.T) {
	text := (AllTestsReport{}).Text()
	if strings.Contains(text, "PASSED") {
		t.Errorf("a report of no phases renders %q, which claims a verdict over nothing", text)
	}
	if text != "no phase ran\n" {
		t.Errorf("a report of no phases renders %q", text)
	}
}

// A binary path named by the host is resolved as follows. An ABSOLUTE path is
// used as it is. A relative path is relative to the workspace. The native host
// action passes relative paths. An operator who debugs by hand passes absolute paths.
func TestABinaryPathIsResolvedAgainstTheWorkspaceOnlyWhenItIsRelative(t *testing.T) {
	run := &allTestsRun{Workspace: "/workspace"}

	if got := run.workspacePath("bin/ze"); got != "/workspace/bin/ze" {
		t.Errorf("a relative path resolved to %q", got)
	}
	if got := run.workspacePath("/elsewhere/ze"); got != "/elsewhere/ze" {
		t.Errorf("an absolute path resolved to %q", got)
	}
}

// The shim is rebuilt over whatever was there. A VM that ran once already holds
// the last run's links, and a shim that refused to replace them would exec the
// previous run's binaries.
func TestTheShimReplacesTheLinksAPreviousRunLeft(t *testing.T) {
	run := vmFixture(t)
	if err := os.MkdirAll(run.BinDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stale := filepath.Join(run.BinDir, "ze-test")
	if err := os.Symlink("/nowhere/ze-test", stale); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if err := run.shim(); err != nil {
		t.Fatalf("shim: %v", err)
	}
	target, err := os.Readlink(stale)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if !strings.HasPrefix(target, run.Workspace) {
		t.Errorf("the shim still points at %q", target)
	}
}

// The summary counts what ran and what was skipped. An hour-long run's phases
// scrolled past long ago, so this line is what a reader acts on.
func TestTheSummaryCountsWhatRanAndWhatWasSkipped(t *testing.T) {
	report := AllTestsReport{Phases: []PhaseResult{
		{Name: "functional/encode", Code: 0},
		{Name: "functional/web", Skipped: true, Reason: "ZE_QEMU_SKIP_SUITES"},
		{Name: "unit tests", Code: 0},
	}}

	text := report.Text()
	if !strings.Contains(text, "2 ran, 1 skipped") {
		t.Errorf("the summary is %q, want it to count 2 ran and 1 skipped", text)
	}
	if !strings.Contains(text, "ALL PHASES PASSED") {
		t.Errorf("a run with no failure does not say so: %q", text)
	}

	report.Failed = []string{"functional/encode"}
	if text := report.Text(); !strings.Contains(text, "FAILED: functional/encode") {
		t.Errorf("a run with a failure does not name it: %q", text)
	}
}

// A skipped phase is not a failure. Counting one would turn the default skip
// list into a red run on every night.
func TestASkippedPhaseIsNeverCountedAsAFailure(t *testing.T) {
	var report AllTestsReport
	report.add(PhaseResult{Name: "functional/web", Skipped: true, Code: 1})

	if len(report.Failed) != 0 {
		t.Errorf("a skipped phase was counted as a failure: %v", report.Failed)
	}
}
