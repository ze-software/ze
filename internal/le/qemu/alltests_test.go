package qemu

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/functional"
)

// VALIDATES: the in-VM run covers EVERY functional suite this repository
// declares, refuses to start when it cannot, and names each phase it ran.
// PREVENTS: the defect measured in the former shell driver on 2026-08-26. It
// had 25 hand-written `fsuite` lines while the suite table declared 29. runner,
// flow-export, vpp and web appeared nowhere, so they never executed in the VM.
// Four of the driver's own comments said a suite left off that list "would
// execute NOWHERE". This run's completeness guard makes that silent failure
// impossible.

// recorder is a child runner that answers a fixed code and remembers every
// command line it receives. This lets a case assert the ABSOLUTE number of
// commands that a run emitted.
type recorder struct {
	code  map[string]int
	calls [][]string
	envs  [][]string
}

func (r *recorder) run(argv, environ []string) int {
	r.calls = append(r.calls, argv)
	r.envs = append(r.envs, environ)
	return r.code[strings.Join(argv, " ")]
}

// vmFixture builds a workspace that passes every precondition: the three
// binaries, a feature-gates file, and each integration package directory.
func vmFixture(t *testing.T) *allTestsRun {
	t.Helper()
	workspace := t.TempDir()

	writeFile(t, workspace, "feature-gates.txt", "ze_bgp on\nze_ssh on\nnot_a_tag on\nze_bgp on\n")
	for _, bin := range []string{"bin/ze", "bin/ze-stripped", "bin/ze-test"} {
		writeExecutable(t, workspace, bin)
	}
	for _, pkg := range integrationPackages {
		if err := os.MkdirAll(filepath.Join(workspace, strings.TrimSuffix(pkg, "/...")), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", pkg, err)
		}
	}

	return &allTestsRun{
		Workspace:   workspace,
		BinDir:      filepath.Join(t.TempDir(), "bin"),
		ZeBin:       "bin/ze",
		StrippedBin: "bin/ze-stripped",
		TestBin:     "bin/ze-test",
		Skip:        []string{functionalWeb},
		Parallel:    "4",
		Timeout:     "900s",
		BuildCache:  filepath.Join(workspace, "cache", "go-build"),
		ModuleCache: filepath.Join(workspace, "cache", "go-mod"),
		Note:        func(string) {},
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func writeExecutable(t *testing.T, root, rel string) {
	t.Helper()
	writeFile(t, root, rel, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(root, rel), 0o700); err != nil {
		t.Fatalf("chmod %s: %v", rel, err)
	}
}

// EVERY suite this repository declares is either run in the VM or excluded with
// a reason. This is the guard the shell's hand-written list does not have.
func TestEveryDeclaredSuiteIsEitherRunOrExcluded(t *testing.T) {
	listed := make(map[string]bool, len(vmSuites))
	for _, suite := range vmSuites {
		if listed[suite.Name] {
			t.Errorf("suite %q is listed twice, so it would run twice", suite.Name)
		}
		listed[suite.Name] = true
	}

	for _, suite := range functional.Suites {
		if !listed[suite.Name] && excludedSuites[suite.Name] == "" {
			t.Errorf("suite %q runs in no VM phase and carries no exclusion reason,"+
				" so its tests execute nowhere", suite.Name)
		}
	}
	for name := range listed {
		if _, declared := functional.SuiteNamed(name); !declared {
			t.Errorf("the VM runs %q, which is not a declared suite", name)
		}
	}
	if len(vmSuites) != len(functional.Suites) {
		t.Errorf("the VM lists %d suites and the repository declares %d",
			len(vmSuites), len(functional.Suites))
	}
}

// One command per suite that is not skipped, plus the unit phase and the
// integration phase. The count is ABSOLUTE: a run that stopped after five
// suites and a comparison against another run that stopped after five would
// agree.
func TestTheRunEmitsOneCommandPerSuitePlusTheTwoPhases(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run

	report, code := run.Execute()
	if code != 0 {
		t.Fatalf("a run whose every child answered 0 exited %d: %v", code, report.Failed)
	}

	wantCommands := len(vmSuites) - 1 + 2 // every suite but the skipped one, then two phases
	if len(rec.calls) != wantCommands {
		t.Fatalf("%d commands ran, want exactly %d", len(rec.calls), wantCommands)
	}
	if len(report.Phases) != len(vmSuites)+2 {
		t.Fatalf("%d phases reported, want exactly %d", len(report.Phases), len(vmSuites)+2)
	}
}

func TestASkippedSuiteEmitsNoCommandAndIsStillReported(t *testing.T) {
	run := vmFixture(t)
	run.Skip = []string{functionalWeb, "editor"}
	rec := &recorder{}
	run.Run = rec.run

	report, _ := run.Execute()

	for _, call := range rec.calls {
		if strings.Contains(strings.Join(call, " "), " editor") {
			t.Errorf("a skipped suite ran: %v", call)
		}
	}
	skipped := 0
	for _, phase := range report.Phases {
		if phase.Skipped {
			skipped++
		}
	}
	if skipped != 2 {
		t.Errorf("%d phases reported as skipped, want exactly 2", skipped)
	}
}

// The suite command is the script's command, with the `timeout` wrapper. The
// wall-clock cap runs the suite in its own process group. Thus a stuck ze cannot
// wedge the run.
func TestASuiteRunsUnderTheWallClockCap(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run
	run.Execute()

	first := strings.Join(rec.calls[0], " ")
	want := "timeout --kill-after=" + killAfter + " 900s " + filepath.Join(run.BinDir, "ze-test") +
		" bgp encode --all -p 4"
	if first != want {
		t.Errorf("the first suite command is\n  %s\nwant\n  %s", first, want)
	}
}

func TestAFailingSuiteIsNamedAndTheRunExitsNonZero(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{code: map[string]int{}}
	run.Run = rec.run

	// Fail the parse suite alone, so a run that stopped at the first failure
	// would emit fewer commands and this case would see it.
	run.Run = func(argv, environ []string) int {
		code := rec.run(argv, environ)
		if strings.Contains(strings.Join(argv, " "), "bgp parse") {
			return 2
		}
		return code
	}

	report, code := run.Execute()
	if code == 0 {
		t.Fatal("a run with a failing suite exited 0")
	}
	if len(report.Failed) != 1 || report.Failed[0] != "functional/parse" {
		t.Errorf("failures are %v, want exactly [functional/parse]", report.Failed)
	}
	wantCommands := len(vmSuites) - 1 + 2
	if len(rec.calls) != wantCommands {
		t.Errorf("%d commands ran after a failure, want exactly %d: a failing suite"+
			" must not stop the run", len(rec.calls), wantCommands)
	}
}

func TestAnUnmountedWorkspaceIsRefusedBeforeAnythingRuns(t *testing.T) {
	run := vmFixture(t)
	run.Workspace = filepath.Join(t.TempDir(), "absent")
	rec := &recorder{}
	run.Run = rec.run

	if _, code := run.Execute(); code == 0 {
		t.Fatal("a run with no workspace exited 0")
	}
	if len(rec.calls) != 0 {
		t.Errorf("%d commands ran without a workspace", len(rec.calls))
	}
}

func TestAMissingBinaryIsRefusedBeforeAnythingRuns(t *testing.T) {
	run := vmFixture(t)
	if err := os.Remove(filepath.Join(run.Workspace, "bin", "ze-test")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	rec := &recorder{}
	run.Run = rec.run

	if _, code := run.Execute(); code == 0 {
		t.Fatal("a run missing ze-test exited 0")
	}
	if len(rec.calls) != 0 {
		t.Errorf("%d commands ran without the test binary", len(rec.calls))
	}
}

// The native host action passes both build caches explicitly. Unset, the unit
// phase could compile into a guest-local or dangling host path and make the
// evidence irreproducible.
func TestAnUnsetBuildCacheIsRefusedBeforeAnythingRuns(t *testing.T) {
	run := vmFixture(t)
	run.BuildCache = ""
	rec := &recorder{}
	run.Run = rec.run

	if _, code := run.Execute(); code == 0 {
		t.Fatal("a run with no build cache exited 0")
	}
	if len(rec.calls) != 0 {
		t.Errorf("%d commands ran without a build cache", len(rec.calls))
	}
}

// Without feature-gates.txt, the integration tags collapse to
// `ze_core integration`. Every feature-gated surface then vanishes from the
// build. The tests that assert on it fail for a reason that is not a regression.
func TestAMissingFeatureGatesFileIsRefusedBeforeAnythingRuns(t *testing.T) {
	run := vmFixture(t)
	if err := os.Remove(filepath.Join(run.Workspace, "feature-gates.txt")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	rec := &recorder{}
	run.Run = rec.run

	if _, code := run.Execute(); code == 0 {
		t.Fatal("a run with no feature-gates.txt exited 0")
	}
	if len(rec.calls) != 0 {
		t.Errorf("%d commands ran without the feature-gate manifest", len(rec.calls))
	}
}

func TestTheIntegrationTagsCarryEveryFeatureGateOnce(t *testing.T) {
	run := vmFixture(t)
	tags, err := run.integrationTags()
	if err != nil {
		t.Fatalf("integrationTags: %v", err)
	}
	if tags != "ze_core integration ze_bgp ze_ssh" {
		t.Errorf("tags are %q, want \"ze_core integration ze_bgp ze_ssh\"", tags)
	}
}

// A package path that is not there is a bug in the LIST, not a test failure.
// `go test` reports it as `FAIL <pkg> [setup failed]` among real results.
func TestAnIntegrationPathThatDoesNotExistIsRefusedBeforeAnythingRuns(t *testing.T) {
	run := vmFixture(t)
	if err := os.RemoveAll(filepath.Join(run.Workspace, "internal", "component", "doctor")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	rec := &recorder{}
	run.Run = rec.run

	if _, code := run.Execute(); code == 0 {
		t.Fatal("a run naming a package path that does not exist exited 0")
	}
	if len(rec.calls) != 0 {
		t.Errorf("%d commands ran with a broken package list", len(rec.calls))
	}
}

// The test creates the optional VRRP transport entry. No OTHER optional entry
// is one of its ancestors. If the test created
// ./internal/plugins/isis/transport, it would also create
// ./internal/plugins/isis, which is optional too. The count would move by two
// for one directory.
const optionalUnderTest = "./internal/plugins/vrrp/transport/..."

func TestAnOptionalTransportPackageIsAddedOnlyWhenItIsThere(t *testing.T) {
	run := vmFixture(t)

	without, err := run.integrationArgs()
	if err != nil {
		t.Fatalf("integrationArgs: %v", err)
	}
	if strings.Contains(strings.Join(without, " "), optionalUnderTest) {
		t.Fatalf("an absent optional package is in the list: %v", without)
	}
	if len(without) != len(integrationPackages)+9 {
		t.Fatalf("%d arguments with no optional package present, want exactly %d",
			len(without), len(integrationPackages)+9)
	}

	if err := os.MkdirAll(filepath.Join(run.Workspace,
		strings.TrimSuffix(optionalUnderTest, "/...")), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	with, err := run.integrationArgs()
	if err != nil {
		t.Fatalf("integrationArgs: %v", err)
	}
	if len(with) != len(without)+1 {
		t.Fatalf("%d arguments with the optional package present, want %d",
			len(with), len(without)+1)
	}
	if !strings.Contains(strings.Join(with, " "), optionalUnderTest) {
		t.Errorf("the optional package is not in the list: %v", with)
	}
}

// The PATH shim is what lets a test exec `ze-stripped` by name.
func TestTheBinaryShimIsBuiltBeforeTheSuitesRun(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run
	run.Execute()

	for _, name := range []string{"ze", "ze-stripped", "ze-test"} {
		link := filepath.Join(run.BinDir, name)
		target, err := os.Readlink(link)
		if err != nil {
			t.Errorf("%s is not a symlink: %v", link, err)
			continue
		}
		if !strings.HasPrefix(target, run.Workspace) {
			t.Errorf("%s points at %s, which is outside the workspace", link, target)
		}
	}
}

// Every child sees the workspace as the repository root. The repo-invariant
// tests derive the tree from the binary's own path, which the shim breaks.
func TestEveryChildIsToldWhereTheRepositoryIs(t *testing.T) {
	run := vmFixture(t)
	rec := &recorder{}
	run.Run = rec.run
	run.Execute()

	for i, environ := range rec.envs {
		if !hasEnv(environ, repoRootKey, run.Workspace) {
			t.Fatalf("command %d does not name the repository root: %v", i, rec.calls[i])
		}
		if !hasEnv(environ, noBuildKey, "1") {
			t.Fatalf("command %d does not carry the no-build flag: %v", i, rec.calls[i])
		}
	}
}

func hasEnv(environ []string, key, value string) bool {
	var tb textbuf.Buffer
	return slices.Contains(environ, tb.Str(key).Byte('=').Str(value).String())
}

func TestTheReportNamesEveryFailedPhase(t *testing.T) {
	report := AllTestsReport{
		Phases: []PhaseResult{
			{Name: "functional/parse", Code: 2},
			{Name: "unit tests", Code: 0},
		},
		Failed: []string{"functional/parse"},
	}
	text := report.Text()
	if !strings.Contains(text, "functional/parse") {
		t.Errorf("the report does not name the failed phase: %q", text)
	}
	if strings.Contains((AllTestsReport{}).Text(), "functional/parse") {
		t.Error("an empty report names a phase")
	}
}
