package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

type fakeStageRun struct {
	Output string
	Code   int
}

func TestVerifyRunWritesArtifactsAndContinuesAfterStageFailures(t *testing.T) {
	root := t.TempDir()
	stages := []stage{{Name: "first", Rerun: "make first"}, {Name: "second", Rerun: "make second"}, {Name: "third", Rerun: "make third"}}
	runs := map[string]fakeStageRun{
		"first":  {Output: "first failed\n", Code: 1},
		"second": {Output: "second passed\n", Code: 0},
		"third":  {Output: "third failed\n", Code: 2},
	}
	var executed []string

	code, err := runVerify(context.Background(), verifyConfig{
		Root:   root,
		Mode:   "ze-precommit-verify",
		Stages: stages,
		Now:    fixedNow,
		Out:    io.Discard,
		RunStage: func(_ context.Context, _ string, st stage, w io.Writer) int {
			executed = append(executed, st.Name)
			run := runs[st.Name]
			_, _ = io.WriteString(w, run.Output)
			return run.Code
		},
		WriteStatus: testStatusWriter,
		SelectScope: testScopeSelector,
	})
	if err != nil {
		t.Fatalf("run verify: %v", err)
	}
	if code != 1 {
		t.Fatalf("expected non-zero verify exit collapsed to 1, got %d", code)
	}
	if strings.Join(executed, ",") != "first,second,third" {
		t.Fatalf("stages did not continue in order: %v", executed)
	}

	mustReadFileContains(t, root, combinedLogPath, "first failed")
	mustReadFileContains(t, root, combinedLogPath, "second passed")
	mustReadFileContains(t, root, combinedLogPath, "third failed")
	// Stage logs live in this run's own directory, and the index is what names
	// them: the documented paths belong to whichever run published last.
	var idx verifyIndex
	if err := json.Unmarshal([]byte(readFile(t, root, failuresJSONPath)), &idx); err != nil {
		t.Fatalf("decode failure json: %v", err)
	}
	for i, want := range []string{"first failed", "second passed", "third failed"} {
		detail := idx.Stages[i].DetailLog
		if path.Dir(detail) != idx.RunDir {
			t.Fatalf("stage %d log %q is not in this run's directory %q", i+1, detail, idx.RunDir)
		}
		mustReadFileContains(t, root, detail, want)
	}
	if path.Base(idx.Stages[0].DetailLog) != "01-first.log" {
		t.Fatalf("stage logs lost their run-order prefix: %q", idx.Stages[0].DetailLog)
	}
	mustReadFileContains(t, root, failuresLogPath, "## Stage: first")
	mustReadFileContains(t, root, failuresLogPath, "## Stage: third")
	mustReadFileContains(t, root, combinedLogPath, "FAIL  2 verify stage(s) failed")
	mustReadFileContains(t, root, combinedLogPath, "Read first: "+idx.RunDir)
	mustReadFileContains(t, root, combinedLogPath, "published at "+failuresLogPath)

	mustReadFileContains(t, root, statusPath, "exit=1")
}

func TestVerifyRunProducesStageAndGroupSummaries(t *testing.T) {
	root := t.TempDir()
	stages := []stage{
		{Name: "ze-unit-test-cached", Rerun: "make ze-unit-test-cached"},
		{Name: "ze-functional-test", Rerun: "make ze-functional-test"},
		{Name: "ze-doc-wiring-check", Rerun: "make ze-doc-wiring-check"},
	}
	outputs := map[string]string{
		"ze-unit-test-cached": readFixture(t, "go-test-mixed.log"),
		"ze-functional-test":  readFixture(t, "functional-groups.log"),
		"ze-doc-wiring-check": readFixture(t, "wiring-failure.log"),
	}
	code, err := runVerify(context.Background(), verifyConfig{
		Root:   root,
		Mode:   "ze-precommit-verify",
		Stages: stages,
		Now:    fixedNow,
		Out:    io.Discard,
		RunStage: func(_ context.Context, _ string, st stage, w io.Writer) int {
			_, _ = io.WriteString(w, outputs[st.Name])
			return 1
		},
		WriteStatus: testStatusWriter,
		SelectScope: testScopeSelector,
	})
	if err != nil {
		t.Fatalf("run verify: %v", err)
	}
	if code == 0 {
		t.Fatalf("expected verify failure")
	}

	failureText := readFile(t, root, failuresLogPath)
	for _, want := range []string{
		"Group: package:github.com/ze-software/ze/internal/example/alpha",
		"Rerun: go test ./internal/example/alpha -run '^TestAlpha$'",
		"Group: package:github.com/ze-software/ze/internal/example/beta",
		"Stage: plugin",
		"Rerun: ze-test bgp plugin 1 2",
		"Group: subcheck:ze-doc-verify",
		"Rerun: make ze-doc-verify",
	} {
		if !strings.Contains(failureText, want) {
			t.Fatalf("failure index missing %q:\n%s", want, failureText)
		}
	}

	var idx verifyIndex
	if err := json.Unmarshal([]byte(readFile(t, root, failuresJSONPath)), &idx); err != nil {
		t.Fatalf("decode failure json: %v", err)
	}
	if len(idx.Stages) != 3 || len(idx.Stages[0].Groups) != 2 || len(idx.Stages[1].Groups) != 1 || len(idx.Stages[2].Groups) != 1 {
		t.Fatalf("unexpected json group shape: %+v", idx.Stages)
	}
}

func TestVerifyRunMixedFailureFixture(t *testing.T) {
	root := t.TempDir()
	stages := []stage{
		{Name: "ze-unit-test-cached", Rerun: "make ze-unit-test-cached"},
		{Name: "ze-functional-test", Rerun: "make ze-functional-test"},
		{Name: "ze-doc-wiring-check", Rerun: "make ze-doc-wiring-check"},
		{Name: "ze-functional-exabgp-test", Rerun: "make ze-functional-exabgp-test"},
	}
	outputs := map[string]string{
		"ze-unit-test-cached":       readFixture(t, "go-test-mixed.log"),
		"ze-functional-test":        readFixture(t, "functional-groups.log"),
		"ze-doc-wiring-check":       readFixture(t, "wiring-failure.log"),
		"ze-functional-exabgp-test": readFixture(t, "exabgp-summary.log"),
	}

	code, err := runVerify(context.Background(), verifyConfig{
		Root:   root,
		Mode:   "ze-precommit-verify",
		Stages: stages,
		Now:    fixedNow,
		Out:    io.Discard,
		RunStage: func(_ context.Context, _ string, st stage, w io.Writer) int {
			_, _ = io.WriteString(w, outputs[st.Name])
			return 1
		},
		WriteStatus: testStatusWriter,
		SelectScope: testScopeSelector,
	})
	if err != nil {
		t.Fatalf("run verify: %v", err)
	}
	if code != 1 {
		t.Fatalf("expected failure exit, got %d", code)
	}
	for _, want := range []string{"package:github.com/ze-software/ze/internal/example/alpha", "plugin:timeout:bfd", "subcheck:ze-doc-verify", "exabgp:failed"} {
		mustReadFileContains(t, root, failuresLogPath, want)
	}
}

func TestVerifyRunAllPassFixture(t *testing.T) {
	root := t.TempDir()
	code, err := runVerify(context.Background(), verifyConfig{
		Root:   root,
		Mode:   "ze-precommit-verify",
		Stages: []stage{{Name: "ze-lint", Rerun: "make ze-lint"}, {Name: "ze-unit-test-cached", Rerun: "make ze-unit-test-cached"}},
		Now:    fixedNow,
		Out:    io.Discard,
		RunStage: func(_ context.Context, _ string, _ stage, w io.Writer) int {
			_, _ = io.WriteString(w, "ok\n")
			return 0
		},
		WriteStatus: testStatusWriter,
		SelectScope: testScopeSelector,
	})
	if err != nil {
		t.Fatalf("run verify: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected pass exit, got %d", code)
	}
	mustReadFileContains(t, root, failuresLogPath, "No failures.")
	mustReadFileContains(t, root, combinedLogPath, "PASS  all 2 verify stage(s)")

	mustReadFileContains(t, root, statusPath, "exit=0")
	mustReadFileContains(t, root, statusPath, "mode=ze-precommit-verify")
	mustReadFileContains(t, root, statusPath, "skipped=")
}

func TestVerifyRunFunctionalFixtureWithRelatedPluginFailures(t *testing.T) {
	groups := classifyFunctional("tmp/verify/functional.log", readFixture(t, "functional-groups.log"))
	if len(groups) != 1 {
		t.Fatalf("expected one functional group, got %+v", groups)
	}
	if groups[0].GroupID != "plugin:timeout:bfd" || strings.Join(groups[0].Related, ",") != "1,2" {
		t.Fatalf("unexpected functional group: %+v", groups[0])
	}
}

func TestClassifyFunctionalSuiteFallbackNamesTheSuiteTarget(t *testing.T) {
	groups := classifyFunctional("tmp/verify/install.log", "FAIL  1 suite(s) failed: install\n")
	if len(groups) != 1 {
		t.Fatalf("expected one suite fallback group, got %+v", groups)
	}
	if want := "make ze-functional-install-test"; groups[0].Rerun != want {
		t.Fatalf("install rerun = %q, want %q", groups[0].Rerun, want)
	}
}

// allSuitesRE captures the one all_suites assignment in the functional recipe.
var allSuitesRE = regexp.MustCompile(`(?m)^\s*all_suites="([^"]+)"`)

// gatingFunctionalSuites returns the suites `make ze-functional-test` runs, read
// from the all_suites line that is their single source of truth.
func gatingFunctionalSuites(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRootFromScriptsStatus(), "mk", "test-functional.mk"))
	if err != nil {
		t.Fatalf("read mk/test-functional.mk: %v", err)
	}
	m := allSuitesRE.FindAllStringSubmatch(string(b), -1)
	if len(m) != 1 {
		t.Fatalf("mk/test-functional.mk must hold exactly one all_suites= line, found %d", len(m))
	}
	suites := strings.Fields(m[0][1])
	if len(suites) < 20 {
		t.Fatalf("parsed only %d gating suites; the all_suites regex has rotted. This test must not pass vacuously.", len(suites))
	}
	return suites
}

// TestFunctionalSuiteRerunNamesARealMakeTarget
//
// VALIDATES: the rerun command functionalSuiteRerun puts on every ordinary
// functional failure group is a make target that exists, for every suite the
// gating run can fail on.
// PREVENTS: a failure report that tells the reader to type a command make
// answers with `No rule to make target`. The rerun string is never executed by
// the code that prints it, so nothing else goes red when a suite is added to
// all_suites without a target, or when the target family is renamed. That is
// exactly how `make ze-<suite>-test` survived: it named no target for any of the
// 24 suites (plan/journal/failure-report-names-a-command-that-does-not-exist.md).
func TestFunctionalSuiteRerunNamesARealMakeTarget(t *testing.T) {
	targets := declaredMakeTargets(t)
	for _, suite := range gatingFunctionalSuites(t) {
		rerun := functionalSuiteRerun(suite)
		target, ok := strings.CutPrefix(rerun, "make ")
		if !ok {
			t.Errorf("functionalSuiteRerun(%q) = %q, which is not a make command", suite, rerun)
			continue
		}
		if !targets[target] {
			t.Errorf("functionalSuiteRerun(%q) = %q, but no target %q is declared in Makefile or mk/*.mk", suite, rerun, target)
		}
	}
}

func TestVerifyRunCapsInlineMembersAndExcerptLines(t *testing.T) {
	root := t.TempDir()
	members := make([]string, 12)
	for i := range members {
		members[i] = string(rune('A' + i))
	}
	lines := make([]string, 25)
	for i := range lines {
		lines[i] = "line " + string(rune('a'+i%26))
	}
	payload := failureGroup{Stage: "plugin", GroupID: "plugin:mismatch:rib", Kind: "mismatch", Related: members, Summary: "many failures", Rerun: "ze-test bgp plugin A B", Parallel: "group", Excerpt: lines}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}

	_, err = runVerify(context.Background(), verifyConfig{
		Root:   root,
		Mode:   "ze-precommit-verify",
		Stages: []stage{{Name: "ze-functional-test", Rerun: "make ze-functional-test"}},
		Now:    fixedNow,
		Out:    io.Discard,
		RunStage: func(_ context.Context, _ string, _ stage, w io.Writer) int {
			_, _ = io.WriteString(w, "VERIFY FAILURE GROUP: "+string(b)+"\n")
			return 1
		},
		WriteStatus: testStatusWriter,
		SelectScope: testScopeSelector,
	})
	if err != nil {
		t.Fatalf("run verify: %v", err)
	}
	failureText := readFile(t, root, failuresLogPath)
	if !strings.Contains(failureText, "+4 more") {
		t.Fatalf("member cap missing from failure index:\n%s", failureText)
	}
	if !strings.Contains(failureText, "see detail log") {
		t.Fatalf("excerpt cap missing from failure index:\n%s", failureText)
	}
}

func TestGoTestFailuresGroupByPackage(t *testing.T) {
	text := `# github.com/ze-software/ze/internal/example/build
internal/example/build/bad.go:3:2: undefined: nope
FAIL	github.com/ze-software/ze/internal/example/build [build failed]
--- FAIL: TestOne (0.00s)
    one_test.go:10: no
FAIL
FAIL	github.com/ze-software/ze/internal/example/one	0.01s
`
	groups := classifyGoTest(stage{Name: "ze-unit-test-cached"}, "tmp/verify/unit.log", text)
	if len(groups) != 2 {
		t.Fatalf("expected two package groups, got %+v", groups)
	}
	if groups[0].Kind != "build" || groups[0].Rerun != "go test ./internal/example/build" {
		t.Fatalf("unexpected build group: %+v", groups[0])
	}
	if groups[1].GroupID != "package:github.com/ze-software/ze/internal/example/one" || groups[1].Rerun != "go test ./internal/example/one -run '^TestOne$'" {
		t.Fatalf("unexpected test group: %+v", groups[1])
	}
}

func TestLintFailuresGroupByPackageAndLinter(t *testing.T) {
	groups := classifyLint(stage{Name: "ze-lint"}, "tmp/verify/lint.log", readFixture(t, "lint-mixed.log"))
	if len(groups) != 3 {
		t.Fatalf("expected three lint groups, got %+v", groups)
	}
	if groups[0].GroupID != "lint:cmd/ze-test:revive" || groups[0].Rerun != "golangci-lint run ./cmd/ze-test" {
		t.Fatalf("unexpected first lint group: %+v", groups[0])
	}
	if groups[1].GroupID != "lint:cmd/ze-test:gofmt" || groups[1].Rerun != "golangci-lint run ./cmd/ze-test" {
		t.Fatalf("unexpected second lint group: %+v", groups[1])
	}
	if groups[2].GroupID != "lint:internal/test/runner:revive" || groups[2].Rerun != "golangci-lint run ./internal/test/runner" {
		t.Fatalf("unexpected third lint group: %+v", groups[2])
	}
	// The kind is the constant word, and the consumer cannot work without that.
	// PATH_BEARING_GROUP_KINDS (scripts/dev/commit_helper.py) is an ALLOWLIST, so
	// a kind it does not hold charges the red to the committing session. Putting
	// the linter here would make the set golangci-lint's to enumerate, and every
	// linter nobody listed would charge a red it can attribute to a file.
	for _, g := range groups {
		if g.Kind != "lint" {
			t.Fatalf("kind = %q for %s, want lint: PATH_BEARING_GROUP_KINDS "+
				"(scripts/dev/commit_helper.py) cannot enumerate linter names", g.Kind, g.GroupID)
		}
	}
}

func TestVetFailuresGroupByPackage(t *testing.T) {
	groups := classifyVet(stage{Name: "ze-evidence-vet"}, "tmp/verify/vet.log", readFixture(t, "vet-mixed.log"))
	if len(groups) != 2 {
		t.Fatalf("expected two vet groups, got %+v", groups)
	}
	if groups[0].GroupID != "vet:./scripts/evidence/install" || groups[0].Rerun != "GOOS=linux go vet ./scripts/evidence/install" {
		t.Fatalf("unexpected first vet group: %+v", groups[0])
	}
	if groups[1].GroupID != "vet:./scripts/evidence/runtime" || groups[1].Rerun != "GOOS=linux go vet ./scripts/evidence/runtime" {
		t.Fatalf("unexpected second vet group: %+v", groups[1])
	}
}

func TestWiringDocFailuresGroupBySubcheck(t *testing.T) {
	text := "Running ze-doc-verify...\noutput\nze-doc-verify failed\n"
	groups := classifyWiringDocs(stage{Name: "ze-doc-wiring-check"}, "tmp/verify/wiring.log", text)
	if len(groups) != 1 {
		t.Fatalf("expected one group, got %+v", groups)
	}
	if groups[0].GroupID != "subcheck:ze-doc-verify" || groups[0].Rerun != "make ze-doc-verify" {
		t.Fatalf("unexpected wiring group: %+v", groups[0])
	}
}

func TestExabgpVerifyModeSummaryUsesNewlinesAndExactReproducers(t *testing.T) {
	text := readFixture(t, "exabgp-summary.log")
	groups := classifyExabgp(stage{Name: "ze-functional-exabgp-test"}, "tmp/verify/exabgp.log", text)
	if len(groups) != 2 {
		t.Fatalf("expected failed and timeout groups, got %+v", groups)
	}
	for _, group := range groups {
		if strings.Contains(strings.Join(group.Excerpt, "\n"), "\r") {
			t.Fatalf("exabgp excerpt contains carriage return: %+v", group.Excerpt)
		}
		if !strings.Contains(group.Rerun, "./test/exabgp-compat/bin/functional encoding --timeout 180") {
			t.Fatalf("rerun does not use repo path: %s", group.Rerun)
		}
	}
}

// VALIDATES: stagesForMode's stage list is the ACTUAL source of truth for
// `make ze-precommit-verify`/`ze-precommit-verify-changed` -- the Makefile targets invoke this
// program and enumerate no stages of their own.
// PREVENTS: a verification gate landing somewhere that never executes under
// `make ze-precommit-verify`. A duplicate _ze-verify-impl/_ze-verify-changed-impl pair
// used to make that trap easy: they read like the stage list, had zero callers,
// and had silently drifted (ze-tier-check, ze-iface-resolution-check and
// ze-plugin-boundary-check were listed there but never ran). They were deleted
// by plan/spec-fixit-verify-stage-ssot.md; this test keeps the remaining list
// honest.
//
// ze-unit-hook-test is in the list for the same reason, having hit the same trap from
// the other side: it was reachable ONLY by typing `make ze-unit-hook-test` by hand --
// absent from ze-standard-test (Makefile), from this stage list, and from
// .github/workflows/ (whose verify job's only step is `make ze-precommit-verify`). Its
// checks guard the agent hooks, whose
// failure mode is silent and fail-CLOSED: a session-id mismatch between
// lib/session-id.sh and pretool-writeedit.py blocks every agent from writing while
// reporting work was never done (real incident, 2026-07-16). A guard nobody runs
// does not guard.
func TestStagesForModeIncludesStaticAnalysisGates(t *testing.T) {
	requiredStages := []string{
		"ze-tier-check",
		"ze-iface-resolution-check",
		"ze-plugin-boundary-check",
		"ze-unit-hook-test",
	}
	for _, mode := range []string{"ze-precommit-verify", "ze-precommit-verify-changed"} {
		names := map[string]bool{}
		for _, st := range stagesForMode(mode, "make") {
			names[st.Name] = true
		}
		for _, want := range requiredStages {
			if !names[want] {
				t.Errorf("stagesForMode(%q) missing stage %q", mode, want)
			}
		}
	}
}

// VALIDATES: the alloc-ceiling gate (ze-alloc-check) is registered in the full
// `ze-precommit-verify` stage list -- the ACTUAL source of truth CI runs via
// `make ze-precommit-verify` (.github/workflows/verify.yml) -- and is deliberately absent from
// the fast `ze-precommit-verify-changed` inline loop (spec-fixit-perf-alloc-ci-gate AC-3).
// PREVENTS: the perf-alloc regression gate merging a per-op heap allocation
// undetected because the stage was never wired into the runner CI executes,
// and PREVENTS it silently bloating the per-edit dev loop.
func TestStagesIncludeAllocGate(t *testing.T) {
	full := map[string]bool{}
	for _, st := range stagesForMode("ze-precommit-verify", "make") {
		full[st.Name] = true
	}
	if !full["ze-alloc-check"] {
		t.Errorf("stagesForMode(\"ze-precommit-verify\") missing ze-alloc-check; CI would never run the alloc gate")
	}

	changed := map[string]bool{}
	for _, st := range stagesForMode("ze-precommit-verify-changed", "make") {
		changed[st.Name] = true
	}
	if changed["ze-alloc-check"] {
		t.Errorf("stagesForMode(\"ze-precommit-verify-changed\") must NOT include ze-alloc-check (keeps the inline dev loop fast)")
	}
}

// VALIDATES: the documentation-consistency gate (doc drift + corpus path
// references) is wired into the ACTUAL stage list CI runs via `make ze-precommit-verify`
// (.github/workflows/verify.yml), for the default full mode. Before this the gate was
// dark: stagesForMode enumerated every ze-precommit-verify stage and named no ze-doc-*
// target, so `check_doc_links.py` could exit 1 with broken discovery-layer refs
// while CI stayed green (spec-fixit-doc-gate-and-refs AC-1).
// PREVENTS: a broken index/rule path reference or a doc count drift merging
// undetected because no gate that CI runs ever exercised the doc checks.
func TestStagesForModeIncludesDocGate(t *testing.T) {
	names := map[string]bool{}
	for _, st := range stagesForMode("ze-precommit-verify", "make") {
		names[st.Name] = true
	}
	for _, want := range []string{"ze-doc-verify", "ze-doc-links-check"} {
		if !names[want] {
			t.Errorf("stagesForMode(%q) missing doc-gate stage %q; the doc checks would never run under CI", "ze-precommit-verify", want)
		}
	}
}

// VALIDATES: the doc gate is ALSO wired into the fast `ze-precommit-verify-changed` inline
// loop, not only the full run (spec-fixit-doc-gate-and-refs AC-1, R-3). A single
// wired branch would let changed-mode sessions stay green on a broken reference.
// PREVENTS: the changed-mode verify path skipping the doc-consistency gate.
func TestStagesForModeChangedIncludesDocGate(t *testing.T) {
	names := map[string]bool{}
	for _, st := range stagesForMode("ze-precommit-verify-changed", "make") {
		names[st.Name] = true
	}
	for _, want := range []string{"ze-doc-verify", "ze-doc-links-check"} {
		if !names[want] {
			t.Errorf("stagesForMode(%q) missing doc-gate stage %q; changed-mode would skip the doc checks", "ze-precommit-verify-changed", want)
		}
	}
}

// The full gate's own copy of the index is what proves a full run happened over
// a session's Go code (ai/rules/git-safety.md, owner directive 2026-08-17). The
// shared artifact cannot carry that: any session running the cheaper changed
// mode rewrites it. So the copy exists for the full mode and for nothing else.
func TestOnlyTheFullModeWritesTheFullVerifyIndex(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want bool
	}{
		{modeFullVerify, true},
		{"ze-precommit-verify-changed", false},
	} {
		root := t.TempDir()
		if _, err := runVerify(context.Background(), verifyConfig{
			Root:   root,
			Mode:   tc.mode,
			Stages: []stage{{Name: "one", Rerun: "make one"}},
			Now:    fixedNow,
			Out:    io.Discard,
			RunStage: func(context.Context, string, stage, io.Writer) int {
				return 0
			},
			WriteStatus: testStatusWriter,
			SelectScope: testScopeSelector,
		}); err != nil {
			t.Fatalf("%s: run verify: %v", tc.mode, err)
		}
		// The shared artifact is written by every mode, so its presence says
		// nothing; only the full-mode copy discriminates.
		mustReadFileContains(t, root, failuresJSONPath, tc.mode)
		_, err := os.Stat(filepath.Join(root, fullVerifyJSONPath))
		if tc.want && err != nil {
			t.Fatalf("%s must write %s: %v", tc.mode, fullVerifyJSONPath, err)
		}
		if !tc.want && err == nil {
			t.Fatalf("%s must NOT write %s: a cheaper run would certify a Go commit", tc.mode, fullVerifyJSONPath)
		}
	}
}

// Several sessions share this checkout and start verify runs at the same
// moment. Before per-run directories the artifact paths were package constants,
// so the second run truncated the first one's combined log and rewrote its
// failure index, and a failure summary could describe a run the reader never
// started.
//
// The two runs here are held inside their stage until BOTH have started, so the
// overlap is real rather than incidental.
func TestConcurrentRunsDoNotShareArtifactPaths(t *testing.T) {
	root := t.TempDir()
	started := make(chan struct{}, 2)
	release := make(chan struct{})

	runOne := func(tag string) {
		_, err := runVerify(context.Background(), verifyConfig{
			Root:   root,
			Mode:   modeFullVerify,
			Stages: []stage{{Name: tag, Rerun: "make " + tag}},
			Now:    fixedNow,
			Out:    io.Discard,
			RunStage: func(_ context.Context, _ string, st stage, w io.Writer) int {
				if _, err := io.WriteString(w, st.Name+" ran\n"); err != nil {
					t.Errorf("%s: write stage output: %v", st.Name, err)
				}
				started <- struct{}{}
				<-release
				return 1
			},
			WriteStatus: testStatusWriter,
			SelectScope: testScopeSelector,
		})
		if err != nil {
			t.Errorf("%s: run verify: %v", tag, err)
		}
	}

	done := make(chan struct{})
	for _, tag := range []string{"alpha", "beta"} {
		go func() {
			defer func() { done <- struct{}{} }()
			runOne(tag)
		}()
	}
	<-started
	<-started
	close(release)
	<-done
	<-done

	indexes := map[string]verifyIndex{}
	for _, dir := range runDirNames(t, root) {
		var idx verifyIndex
		rel := path.Join(stageLogDir, dir, "ze-verify-failures.json")
		if err := json.Unmarshal([]byte(readFile(t, root, rel)), &idx); err != nil {
			t.Fatalf("decode %s: %v", rel, err)
		}
		if idx.RunDir != path.Join(stageLogDir, dir) {
			t.Fatalf("index in %s claims run directory %q", dir, idx.RunDir)
		}
		if len(idx.Stages) != 1 {
			t.Fatalf("index in %s holds %d stages", dir, len(idx.Stages))
		}
		tag := idx.Stages[0].Stage
		if _, seen := indexes[tag]; seen {
			t.Fatalf("two run directories claim stage %q", tag)
		}
		indexes[tag] = idx
		// Each artifact set describes ONE run: its combined log holds its own
		// stage output and none of the other run's, and its stage log is its
		// own file.
		combined := readFile(t, root, idx.CombinedLog)
		if !strings.Contains(combined, tag+" ran") {
			t.Fatalf("combined log of %s lost its own output:\n%s", tag, combined)
		}
		for _, other := range []string{"alpha", "beta"} {
			if other != tag && strings.Contains(combined, other+" ran") {
				t.Fatalf("combined log of %s carries %s output:\n%s", tag, other, combined)
			}
		}
		mustReadFileContains(t, root, idx.Stages[0].DetailLog, tag+" ran")
	}
	if len(indexes) != 2 {
		t.Fatalf("expected one artifact directory per run, got %d", len(indexes))
	}
	if indexes["alpha"].Stages[0].DetailLog == indexes["beta"].Stages[0].DetailLog {
		t.Fatalf("both runs wrote the same stage log %q", indexes["alpha"].Stages[0].DetailLog)
	}

	// AC-10: the documented paths still exist and still hold ONE run's whole
	// index, because commit_helper.py reads them.
	for _, published := range []string{failuresJSONPath, fullVerifyJSONPath} {
		var idx verifyIndex
		if err := json.Unmarshal([]byte(readFile(t, root, published)), &idx); err != nil {
			t.Fatalf("decode published %s: %v", published, err)
		}
		if idx.Mode != modeFullVerify || idx.GeneratedAt == "" {
			t.Fatalf("published %s lost the keys the commit gate reads: %+v", published, idx)
		}
		want, ok := indexes[idx.Stages[0].Stage]
		if !ok || want.RunDir != idx.RunDir {
			t.Fatalf("published %s belongs to no run: %+v", published, idx)
		}
	}
	mustReadFileContains(t, root, combinedLogPath, " ran")
	mustReadFileContains(t, root, failuresLogPath, "Run directory: ")
}

// A run used to overwrite the previous run's artifacts, so the footprint was
// one run. Keeping each run's own directory has to stay bounded, or tmp/verify
// grows without limit.
func TestOldRunDirectoriesArePruned(t *testing.T) {
	root := t.TempDir()
	runs := filepath.Join(root, filepath.FromSlash(stageLogDir))
	if err := os.MkdirAll(runs, 0o750); err != nil {
		t.Fatalf("create stage log dir: %v", err)
	}
	for i := range maxRetainedRunDirs + 4 {
		name := runDirPrefix + "20260101T00000" + strconv.Itoa(i) + "Z-old"
		if err := os.MkdirAll(filepath.Join(runs, name), 0o750); err != nil {
			t.Fatalf("create stale run dir: %v", err)
		}
	}
	// mk/alloc-gate.mk writes this beside the run directories; pruning must not
	// see it as a run.
	keep := filepath.Join(runs, "alloc-gate-bench.txt")
	if err := os.WriteFile(keep, []byte("bench\n"), 0o600); err != nil {
		t.Fatalf("write alloc gate log: %v", err)
	}

	if _, err := runVerify(context.Background(), verifyConfig{
		Root:        root,
		Mode:        modeFullVerify,
		Stages:      []stage{{Name: "one", Rerun: "make one"}},
		Now:         fixedNow,
		Out:         io.Discard,
		RunStage:    func(context.Context, string, stage, io.Writer) int { return 0 },
		WriteStatus: testStatusWriter,
		SelectScope: testScopeSelector,
	}); err != nil {
		t.Fatalf("run verify: %v", err)
	}

	names := runDirNames(t, root)
	if len(names) != maxRetainedRunDirs {
		t.Fatalf("expected %d run directories after pruning, got %d: %v", maxRetainedRunDirs, len(names), names)
	}
	if slices.Contains(names, runDirPrefix+"20260101T000000Z-old") {
		t.Fatalf("the oldest run directory survived pruning: %v", names)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("pruning removed a file that is not a run directory: %v", err)
	}
}

// Only a FULL run writes tmp/ze-verify-full.json, and eight sessions sharing
// this checkout reach ten cheaper ze-precommit-verify-changed runs in a day.
// Pruning by age alone would delete the full run's directory while that
// published path still pointed into it, and full_verify_coverage in
// scripts/dev/commit_helper.py returns "uncovered" when it cannot read the
// file -- refusing every commit carrying Go until somebody spends 20 minutes on
// a fresh full run.
func TestPruningKeepsTheRunAPublishedPathPointsInto(t *testing.T) {
	root := t.TempDir()
	// A monotonic clock makes the full run the OLDEST directory by name, so it
	// is the first one age-based pruning would take.
	tick := 0
	clock := func() time.Time {
		tick++
		return fixedNow().Add(time.Duration(tick) * time.Minute)
	}
	runMode := func(mode string) {
		if _, err := runVerify(context.Background(), verifyConfig{
			Root:        root,
			Mode:        mode,
			Stages:      []stage{{Name: "one", Rerun: "make one"}},
			Now:         clock,
			Out:         io.Discard,
			RunStage:    func(context.Context, string, stage, io.Writer) int { return 0 },
			WriteStatus: testStatusWriter,
			SelectScope: testScopeSelector,
		}); err != nil {
			t.Fatalf("%s: run verify: %v", mode, err)
		}
	}

	runMode(modeFullVerify)
	var full verifyIndex
	if err := json.Unmarshal([]byte(readFile(t, root, fullVerifyJSONPath)), &full); err != nil {
		t.Fatalf("decode published full index: %v", err)
	}
	for range maxRetainedRunDirs + 2 {
		runMode("ze-precommit-verify-changed")
	}

	// The commit gate reads this path, so it must still resolve and still carry
	// the keys full_verify_coverage reads.
	var kept verifyIndex
	if err := json.Unmarshal([]byte(readFile(t, root, fullVerifyJSONPath)), &kept); err != nil {
		t.Fatalf("%s no longer resolves after %d changed-mode runs: %v", fullVerifyJSONPath, maxRetainedRunDirs+2, err)
	}
	if kept.Mode != modeFullVerify || kept.GeneratedAt != full.GeneratedAt || kept.RunDir != full.RunDir {
		t.Fatalf("%s stopped describing the full run: got %+v, want mode %s run-dir %s generated-at %s",
			fullVerifyJSONPath, kept, modeFullVerify, full.RunDir, full.GeneratedAt)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(full.RunDir))); err != nil {
		t.Fatalf("the full run's artifact directory was pruned while %s pointed into it: %v", fullVerifyJSONPath, err)
	}

	// Pinning must not defeat the bound it lives inside.
	names := runDirNames(t, root)
	if len(names) > maxRetainedRunDirs {
		t.Fatalf("pinning stopped pruning: %d run directories kept, budget is %d: %v", len(names), maxRetainedRunDirs, names)
	}
}

// runDirNames lists the per-run artifact directories under stageLogDir.
func runDirNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(stageLogDir)))
	if err != nil {
		t.Fatalf("read run directories: %v", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), runDirPrefix) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
}

func testStatusWriter(root string, code int, mode, skipped string, _ treeSnapshot, now time.Time) error {
	content := "exit=" + strconv.Itoa(code) + "\n" + "timestamp=" + now.UTC().Format(time.RFC3339) + "\nmode=" + mode + "\nskipped=" + skipped + "\ngit_sha=test\ntree_hash=test\n"
	return os.WriteFile(filepath.Join(root, statusPath), []byte(content), 0o644)
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "verify", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func mustReadFileContains(t *testing.T, root, rel, want string) {
	t.Helper()
	got := readFile(t, root, rel)
	if !strings.Contains(got, want) {
		t.Fatalf("%s missing %q:\n%s", rel, want, got)
	}
}

// ── Stage-list SSOT guards (plan/spec-fixit-verify-stage-ssot.md) ───────────
//
// stagesForMode is the ONLY live verify stage list: `make ze-precommit-verify` and
// `make ze-precommit-verify-changed` both shell out to this runner (Makefile), and CI
// shards this same list, each shard reading it with `make ze-precommit-verify-list`
// (.github/workflows/verify.yml). A gate absent from stagesForMode therefore never
// runs anywhere.
//
// The goldens below are deliberately hand-maintained literals, not derived from
// stagesForMode: their entire job is to be a change-detector. Deriving them from
// the function under test would make the comparison vacuous. Editing the stage
// list must be a two-place, deliberate act -- which is exactly what the dead
// _ze-verify-impl Makefile targets failed to be before they were deleted.
//
// Comparison is ORDERED, not merely set-equal: stage order is load-bearing
// (cheap static gates run before the expensive test stages so a red surfaces
// fast), so a silent reordering is a regression worth failing on.

var goldenStagesZeVerify = []string{
	"ze-lint",
	"ze-tier-check",
	"ze-rfc-check",
	"ze-iface-resolution-check",
	"ze-plugin-boundary-check",
	"ze-config-coercion-check",
	"ze-fs-persistence-check",
	"ze-dash-stdio-check",
	"ze-port-defaults-check",
	"ze-config-claims-check",
	"ze-test-sensitivity-check",
	"ze-test-weakened-check",
	"ze-staticcheck-feature-matrix-check",
	"ze-repository-tracked-build-check",
	"ze-platform-vet",
	"ze-doc-wiring-check",
	"ze-doc-verify",
	"ze-doc-links-check",
	"ze-repository-tree-check",
	"ze-generated-files-check",
	"ze-vendor-web-check",
	"ze-htmx-upgrade-check",
	"ze-evidence-vet",
	"ze-unit-hook-test",
	"ze-dependency-vulnerability-check",
	"ze-unit-test-cached",
	"ze-unit-test-race-changed",
	"ze-alloc-check",
	"ze-functional-test",
	"ze-functional-exabgp-test",
}

var goldenStagesZeVerifyChanged = []string{
	"ze-lint-changed",
	"ze-tier-check",
	"ze-rfc-check",
	"ze-iface-resolution-check",
	"ze-plugin-boundary-check",
	"ze-config-coercion-check",
	"ze-fs-persistence-check",
	"ze-dash-stdio-check",
	"ze-port-defaults-check",
	"ze-config-claims-check",
	"ze-test-sensitivity-check",
	"ze-test-weakened-check",
	"ze-staticcheck-feature-matrix-check",
	"ze-repository-tracked-build-check",
	"ze-platform-vet",
	"ze-doc-wiring-check",
	"ze-doc-verify",
	"ze-doc-links-check",
	"ze-repository-tree-check",
	"ze-generated-files-check",
	"ze-vendor-web-check",
	"ze-htmx-upgrade-check",
	"ze-unit-hook-test",
	"ze-dependency-vulnerability-check",
	"ze-unit-test-changed",
	"ze-functional-test",
	"ze-functional-exabgp-test",
}

// TestStagesForModeMatchesGolden locks the full stage list of BOTH modes.
//
// VALIDATES: stagesForMode returns exactly the committed golden list, in order,
// with no duplicates, for `ze-precommit-verify` and `ze-precommit-verify-changed` alike (AC-2).
// PREVENTS: a stage being dropped, silently reordered, or added to only one of
// the two hand-duplicated mode branches -- the precise drift that let the dead
// _ze-verify-impl targets diverge from the live list for an unknown period.
func TestStagesForModeMatchesGolden(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		golden []string
	}{
		{"ze-precommit-verify", goldenStagesZeVerify},
		{"ze-precommit-verify-changed", goldenStagesZeVerifyChanged},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			var got []string
			seen := map[string]bool{}
			for _, st := range stagesForMode(tc.mode, "make") {
				if seen[st.Name] {
					t.Errorf("stagesForMode(%q) lists %q twice", tc.mode, st.Name)
				}
				seen[st.Name] = true
				got = append(got, st.Name)
			}
			if len(got) != len(tc.golden) {
				t.Fatalf("stagesForMode(%q) has %d stages, golden has %d\n got:    %v\n golden: %v\n%s",
					tc.mode, len(got), len(tc.golden), got, tc.golden, goldenEditGuidance)
			}
			for i := range got {
				if got[i] != tc.golden[i] {
					t.Errorf("stagesForMode(%q)[%d] = %q, golden %q\n%s", tc.mode, i, got[i], tc.golden[i], goldenEditGuidance)
				}
			}
		})
	}
}

// TestVerifyStagesIncludeStaticcheckFeatureMatrix pins the matrix gate in both
// live verification modes and keeps the tracked-build final-link stage after it.
//
// VALIDATES: full and changed verification both run the matrix gate (AC-8).
// PREVENTS: one verification branch omitting the gate, or the new type-check
// stage replacing tracked-build coverage.
func TestVerifyStagesIncludeStaticcheckFeatureMatrix(t *testing.T) {
	for _, mode := range []string{"ze-precommit-verify", "ze-precommit-verify-changed"} {
		t.Run(mode, func(t *testing.T) {
			var matrixIndex, trackedBuildIndex = -1, -1
			for i, st := range stagesForMode(mode, "make") {
				switch st.Name {
				case "ze-staticcheck-feature-matrix-check":
					if matrixIndex >= 0 {
						t.Fatalf("%s lists ze-staticcheck-feature-matrix-check more than once", mode)
					}
					matrixIndex = i
				case "ze-repository-tracked-build-check":
					trackedBuildIndex = i
				}
			}
			if matrixIndex < 0 {
				t.Fatalf("%s does not include ze-staticcheck-feature-matrix-check", mode)
			}
			if trackedBuildIndex <= matrixIndex {
				t.Fatalf("%s must retain ze-repository-tracked-build-check after the matrix gate", mode)
			}
		})
	}
}

// goldenEditGuidance is deliberately explicit about BOTH branches. The obvious
// wrong move on seeing one sub-test fail is to update that one golden and ship,
// which blesses exactly the divergence TestStagesForModeBranchesAgree exists to
// catch.
const goldenEditGuidance = "If deliberate: add the stage to BOTH branches of stagesForMode, " +
	"then update BOTH goldens. A gate in only one branch is a gate that does not run " +
	"in the other mode."

// modeSpecificStages are the stages that legitimately differ between the two
// modes, and the ONLY differences allowed. Everything else must be common.
//
// The two entries per row are (full-verify name, changed-mode name); an empty
// changed-mode name means the stage is intentionally absent from the fast loop.
var modeSpecificStages = []struct {
	full, changed, why string
}{
	{"ze-lint", "ze-lint-changed", "changed mode lints only changed packages"},
	{"ze-unit-test-cached", "ze-unit-test-changed", "changed mode runs only changed packages' tests"},
	{"ze-unit-test-race-changed", "", "race pass is full-verify only"},
	{"ze-evidence-vet", "", "evidence vetting is full-verify only"},
	{"ze-alloc-check", "", "alloc ceiling is full-verify only (spec-fixit-perf-alloc-ci-gate AC-3)"},
}

// TestStagesForModeBranchesAgree is the divergence check the goldens cannot be.
//
// VALIDATES: after removing the documented mode-specific stages, the two
// branches of stagesForMode carry the IDENTICAL set of gates (AC-2's "fails if
// the two branches diverge").
// PREVENTS: a gate wired into `ze-precommit-verify` but not `ze-precommit-verify-changed` (or the
// reverse), which is invisible to the per-mode goldens: each golden is its own
// independent change-detector, so updating just the one that failed makes the
// suite green with the fast dev loop silently missing the gate. An independent
// reviewer demonstrated that exact blessed-divergence pass against the goldens
// alone, which is why this test exists.
func TestStagesForModeBranchesAgree(t *testing.T) {
	set := func(mode string) map[string]bool {
		m := map[string]bool{}
		for _, st := range stagesForMode(mode, "make") {
			m[st.Name] = true
		}
		return m
	}
	full, changed := set("ze-precommit-verify"), set("ze-precommit-verify-changed")

	for _, ms := range modeSpecificStages {
		if ms.full != "" {
			if !full[ms.full] {
				t.Errorf("modeSpecificStages lists %q as a full-verify stage, but stagesForMode(%q) does not emit it; the allowlist has rotted", ms.full, "ze-precommit-verify")
			}
			delete(full, ms.full)
		}
		if ms.changed != "" {
			if !changed[ms.changed] {
				t.Errorf("modeSpecificStages lists %q as a changed-mode stage, but stagesForMode(%q) does not emit it; the allowlist has rotted", ms.changed, "ze-precommit-verify-changed")
			}
			delete(changed, ms.changed)
		} else if changed[ms.full] {
			// ms.full is documented as full-verify-ONLY, yet the changed branch
			// now emits it. Report that directly: falling through would delete
			// it from `full` only, and the symmetric-difference check below
			// would then blame it for running in changed-mode but not full --
			// the exact opposite of the truth.
			t.Errorf("stage %q is documented full-verify-only in modeSpecificStages (%s), but stagesForMode(%q) now emits it too; update modeSpecificStages if that is deliberate", ms.full, ms.why, "ze-precommit-verify-changed")
			delete(changed, ms.full)
		}
	}

	for name := range full {
		if !changed[name] {
			t.Errorf("stage %q runs under `ze-precommit-verify` but NOT `ze-precommit-verify-changed`; add it to both branches, or document it in modeSpecificStages with a reason", name)
		}
	}
	for name := range changed {
		if !full[name] {
			t.Errorf("stage %q runs under `ze-precommit-verify-changed` but NOT `ze-precommit-verify`; add it to both branches, or document it in modeSpecificStages with a reason", name)
		}
	}
}

// TestStagesForModeIncludesRegenCheck pins the generated-file staleness gate
// into BOTH mode branches.
//
// VALIDATES: the write-safe, read-only regen check runs under `make ze-precommit-verify`
// and `make ze-precommit-verify-changed` (AC-4).
// PREVENTS: a stale generated file reaching a commit unnoticed because the only
// gate that detected it -- `ze-generated-files-reconcile` -- runs nowhere. Scoped honestly:
// this is NEW coverage for the ai/* discovery indexes, feature_tags' outputs
// (.golangci.yml, gokrazy/ze/config.json, docs/guide/quickstart.md) and
// mk/test-fuzz-targets.mk. It is REDUNDANT (a second, uncached path) for
// plugin/all/all.go, which TestGeneratedPluginImportsCurrent and
// TestVerifyWiringDocsChecksPluginImports already covered. CLAUDE.md/AGENTS.md
// and the skill mirrors are NOT covered here at all -- see the two documented
// exclusions above the target in the Makefile.
// PREVENTS: the MUTATING `ze-generated-files-reconcile` being wired instead: it depends on
// `ze-generated-files-update`, which rewrites generated files before diffing them, so verify
// would leave a dirty working tree (spec R-1).
func TestStagesForModeIncludesRegenCheck(t *testing.T) {
	for _, mode := range []string{"ze-precommit-verify", "ze-precommit-verify-changed"} {
		names := map[string]bool{}
		for _, st := range stagesForMode(mode, "make") {
			names[st.Name] = true
		}
		if !names["ze-generated-files-check"] {
			t.Errorf("stagesForMode(%q) missing ze-generated-files-check; generated-file staleness would go unguarded", mode)
		}
		if names["ze-generated-files-reconcile"] {
			t.Errorf("stagesForMode(%q) wires the MUTATING ze-generated-files-reconcile; it runs ze-generated-files-update and would leave verify with a dirty tree", mode)
		}
	}
}

// TestStagesForModeIncludesValidateTree pins the ze-repository-check half that a shared
// checkout can carry into BOTH mode branches.
//
// VALIDATES: `make ze-repository-tree-check` runs under `make ze-precommit-verify` and
// `make ze-precommit-verify-changed`, and the target it names exists.
// PREVENTS: the five validate.py checks going back to having no automatic
// caller at all. Until 2026-08-09 nothing ran them: no Makefile target depended
// on ze-repository-check, no hook called it, commit_helper.py did not, and stagesForMode
// named neither. Their only callers were two sentences of prose in
// ai/skills/ze-review.md and ai/skills/ze-close.md.
// PREVENTS: the whole `ze-repository-check` target being wired instead. Two of its five
// checks (check_cross_package_wiring, check_cli_handler_coverage in
// scripts/dev/validate.py) take `changed_files` -- `git diff HEAD` plus
// untracked files -- as their subject. Several sessions share this checkout, so
// those files are largely another session's half-written work, and both checks
// demand a completeness (a cross-package caller, a .ci test) that a file mid-edit
// cannot show. Wiring them would red a verify run whose author changed none of
// the files being judged.
func TestStagesForModeIncludesValidateTree(t *testing.T) {
	for _, mode := range []string{"ze-precommit-verify", "ze-precommit-verify-changed"} {
		names := map[string]bool{}
		for _, st := range stagesForMode(mode, "make") {
			names[st.Name] = true
		}
		if !names["ze-repository-tree-check"] {
			t.Errorf("stagesForMode(%q) missing ze-repository-tree-check; the source-anchor and spec-AC checks would run nowhere", mode)
		}
		if names["ze-repository-check"] {
			t.Errorf("stagesForMode(%q) wires the full ze-repository-check; its two changed-file checks judge other sessions' uncommitted files in this shared checkout", mode)
		}
	}

	corpus := makefileCorpus(t)
	if !strings.Contains(corpus, "\nze-repository-tree-check:\n") {
		t.Error("no ze-repository-tree-check target in the Makefile corpus: the stage would fail with 'No rule to make target'")
	}
	if !strings.Contains(corpus, "--changed-file ''") {
		t.Error("ze-repository-tree-check no longer declares an empty changed set; without it validate.py falls back to git diff HEAD and the two changed-file checks come back")
	}
}

// repoRootFromScriptsStatus returns the repo root relative to this package dir.
func repoRootFromScriptsStatus() string { return filepath.Join("..", "..") }

// makefileCorpus concatenates the Makefile and every mk/*.mk fragment.
func makefileCorpus(t *testing.T) string {
	t.Helper()
	root := repoRootFromScriptsStatus()
	var sb strings.Builder
	b, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	sb.Write(b)
	frags, err := filepath.Glob(filepath.Join(root, "mk", "*.mk"))
	if err != nil {
		t.Fatalf("glob mk/*.mk: %v", err)
	}
	if len(frags) == 0 {
		t.Fatal("no mk/*.mk fragments found: layout changed? This test must not pass vacuously.")
	}
	for _, f := range frags {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		sb.WriteByte('\n')
		sb.Write(b)
	}
	return sb.String()
}

// makeTargetRE matches a rule head at the start of a line: `name:` or `name: deps`,
// excluding `::=`-style assignments and pattern rules.
var makeTargetRE = regexp.MustCompile(`(?m)^([A-Za-z0-9_.-]+)[ \t]*:(?:[^=]|$)`)

func declaredMakeTargets(t *testing.T) map[string]bool {
	t.Helper()
	targets := map[string]bool{}
	for _, m := range makeTargetRE.FindAllStringSubmatch(makefileCorpus(t), -1) {
		targets[m[1]] = true
	}
	if len(targets) < 50 {
		t.Fatalf("parsed only %d make targets; the rule-head regex has rotted. This test must not pass vacuously.", len(targets))
	}
	return targets
}

// TestStagesAreRealMakeTargets closes the gap between the stage list and the
// build system.
//
// VALIDATES: every name stagesForMode emits is a make target that actually
// exists in the Makefile or an mk/*.mk fragment.
// PREVENTS: a typo'd or renamed stage. Nothing else catches it: the goldens are
// hand-written alongside the stage list, so a consistent typo in both passes
// every other unit test and only surfaces at verify time as
// `make: *** No rule to make target 'ze-tier-chekc'` -- after the developer has
// already waited through the earlier stages, and in CI as a hard red with no
// pointer to the cause.
func TestStagesAreRealMakeTargets(t *testing.T) {
	targets := declaredMakeTargets(t)
	for _, mode := range []string{"ze-precommit-verify", "ze-precommit-verify-changed"} {
		for _, st := range stagesForMode(mode, "make") {
			if !targets[st.Name] {
				t.Errorf("stagesForMode(%q) emits stage %q, but no such make target is declared in Makefile or mk/*.mk", mode, st.Name)
			}
		}
	}
}

// TestStructuralGatesAreLiveStages is the Go twin of the Python check in
// scripts/dev/commit_helper_test.py.
//
// Both are needed, and the reason is `go test` caching, not belt-and-braces.
// The Python one runs as a subprocess under TestPythonUnitTests, so
// verify_run.go is not one of its cache inputs: editing verify_run.go alone
// serves it a cached PASS (and under ze-precommit-verify-changed, changed-pkgs.sh maps a
// *.go edit to ./scripts/status only). This one calls stagesForMode in-process,
// so any edit to it invalidates this package and the check re-runs. Conversely
// an edit to commit_helper.py invalidates ./scripts/dev and re-runs the Python
// one. Each covers the other's blind side.
//
// VALIDATES: every name in commit_helper.py's STRUCTURAL_GATES is a stage that
// stagesForMode actually emits.
// PREVENTS: a STRUCTURAL_GATES entry that can never match. structural_gate_reds
// compares those names against the `stage` field of tmp/ze-verify-failures.json,
// which verify_run.go fills from stagesForMode; a name absent from the stage
// list gates nothing while reading as a live commit-blocking safety net.
func TestStructuralGatesAreLiveStages(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRootFromScriptsStatus(), "scripts", "dev", "commit_helper.py"))
	if err != nil {
		t.Fatalf("read commit_helper.py: %v", err)
	}
	const marker = "STRUCTURAL_GATES = frozenset("
	_, rest, found := strings.Cut(string(src), marker)
	if !found {
		t.Fatal("STRUCTURAL_GATES not found in commit_helper.py -- renamed? Update this test; it must not pass vacuously.")
	}
	literal, _, found := strings.Cut(rest, ")")
	if !found {
		t.Fatal("malformed STRUCTURAL_GATES frozenset literal")
	}
	gates := regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(literal, -1)
	if len(gates) < 3 {
		t.Fatalf("parsed only %d STRUCTURAL_GATES entries; the literal's shape changed. This test must not pass vacuously.", len(gates))
	}

	live := map[string]bool{}
	for _, mode := range []string{"ze-precommit-verify", "ze-precommit-verify-changed"} {
		for _, st := range stagesForMode(mode, "make") {
			live[st.Name] = true
		}
	}
	for _, g := range gates {
		if !live[g[1]] {
			t.Errorf("STRUCTURAL_GATES names %q, which stagesForMode never emits: structural_gate_reds can never match it, so it gates nothing", g[1])
		}
	}
}

// regenCheckPrereqs is the full prerequisite set ze-generated-files-check must
// carry, and why each one is there. Asserted as an EXACT set, not a floor: an
// earlier version only checked `len(prereqs) >= 4`, and a reviewer showed that
// half the list (including the only guard for ai/INSTRUCTIONS.md) could be
// deleted with every test still green.
var regenCheckPrereqs = map[string]string{
	"ze-plugin-imports-check":  "plugin_imports.go -> internal/component/plugin/all/all.go",
	"ze-yang-glue-check":       "yang_glue.go -> yang/*/register.go, embed.go",
	"ze-feature-tags-check":    "feature_tags.go -> .golangci.yml, gokrazy/ze/config.json, docs/guide/quickstart.md",
	"ze-templ-output-check":    "templ generate -> internal/**/*_templ.go",
	"ze-fuzz-targets-check":    "fuzz-targets.py -> mk/test-fuzz-targets.mk",
	"ze-vendor-web-check":      "sync_web.go -> the vendored asset copy in each internal/**/assets/",
	"ze-web-assets-check":      "web_assets.go -> the per-page asset set in each internal/**/page_assets.go",
	"ze-doc-index-check":       "code_to_docs.py -> ai/CODE-TO-DOCS.md",
	"ze-rules-render-check":    "rules_points.py -> ai/rules/<rule>.md, rendered from ai/rules/points/",
	"ze-rules-index-check":     "rules_index.py -> ai/rules/INDEX.md",
	"ze-rules-condensed-check": "rules_condensed.py -> ai/rules/TRIGGERS.md, ai/rules/CORE.md",
	"ze-rules-lint":            "rules_lint.py -> ai/rules/*.md format contract (read-only validator, no generated output)",
	"ze-arch-map-check":        "arch_map.py -> the architecture lists in ai/INSTRUCTIONS.md (NOT covered by ze-doc-verify)",
	"ze-discovery-index-check": "package_map.py, docs_to_code.py -> the ai/ discovery indexes",
	"ze-test-health-check":     "testing_health.py -> docs/features/test-health.md, test/health/latest.json, the sensitivity baseline",
}

// generatorChecks maps every generator reachable from `make generate` or
// `make ze-generated-files-update` to the regenCheckPrereqs entry that guards its output.
//
// An empty value means DELIBERATELY EXCLUDED, and the reason is recorded in the
// exclusion block above ze-generated-files-check in the Makefile. In short: every
// output of skill_sync.sh (reached via ze-ai-skills-sync) is gitignored, so nothing
// committed can drift and a fresh CI checkout -- where those files do not exist
// at all -- would red on every run.
var generatorChecks = map[string]string{
	// `generate:` recipe
	"scripts/codegen/yang_glue.go":      "ze-yang-glue-check",
	"scripts/codegen/plugin_imports.go": "ze-plugin-imports-check",
	"scripts/codegen/feature_tags.go":   "ze-feature-tags-check",
	"github.com/a-h/templ/cmd/templ":    "ze-templ-output-check",
	"scripts/dev/fuzz-targets.py":       "ze-fuzz-targets-check",
	"scripts/vendor/sync_web.go":        "ze-vendor-web-check",
	"scripts/codegen/web_assets.go":     "ze-web-assets-check",
	// `ze-generated-files-update:` prerequisite targets
	"ze-ai-instructions-generate": "ze-arch-map-check",
	"ze-ai-skills-sync":           "", // gitignored outputs -- excluded on purpose
	"ze-doc-index-update":         "ze-doc-index-check",
	"ze-rules-render-update":      "ze-rules-render-check",
	"ze-rules-index-update":       "ze-rules-index-check",
	"ze-rules-condensed-update":   "ze-rules-condensed-check",
	"ze-discovery-index-update":   "ze-discovery-index-check",
	"ze-arch-map-update":          "ze-arch-map-check",
	"generate":                    "", // expanded into its own generators above
	// Scripts run INSIDE those sub-targets' recipes. Walking only the target
	// names left these invisible: a reviewer showed a new script added to
	// ze-discovery-index-update's recipe would be accepted with no check at all.
	"scripts/dev/arch_map.py":        "ze-arch-map-check",
	"scripts/dev/code_to_docs.py":    "ze-doc-index-check",
	"scripts/dev/rules_index.py":     "ze-rules-index-check",
	"scripts/dev/rules_points.py":    "ze-rules-render-check",
	"scripts/dev/rules_condensed.py": "ze-rules-condensed-check",
	"scripts/dev/package_map.py":     "ze-discovery-index-check",
	"scripts/dev/docs_to_code.py":    "ze-discovery-index-check",
	"scripts/dev/skill_sync.sh":      "", // gitignored outputs -- excluded on purpose
	"scripts/dev/testing_health.py":  "ze-test-health-check",
	"ze-test-health-update":          "ze-test-health-check",
}

// recipeBody returns the lines of `target`'s recipe: everything after the rule
// head until the first line that starts at column 0.
//
// Line-based on purpose. A regex span like `(?ms)^generate:.*?\n\n` looks
// equivalent and is not: GNU make permits blank lines inside a recipe, so a
// stray blank line silently truncates the scan and an unguarded generator
// added below it is accepted. A reviewer proved exactly that.
//
// A recipe that DELEGATES contributes the delegated recipe too. A target routed
// through the shared job admission point keeps only one command of its own --
// `scripts/dev/ze-run.sh <label> $(MAKE) <impl-target>` -- and its work moves to
// the impl target (plan/spec-shared-machine-job-admission.md). A caller asking
// what `ze-lint` RUNS wants both, and following the `$(MAKE)` invocation is what
// keeps that answer right as more targets are routed the same way. Naming the
// impl target at each call site instead would have to be repeated for every one
// of them, and would go stale silently the day a target stops delegating.
func recipeBody(t *testing.T, corpus, target string) (head string, body []string) {
	t.Helper()
	lines := strings.Split(corpus, "\n")
	for i, ln := range lines {
		if !strings.HasPrefix(ln, target+":") {
			continue
		}
		head = strings.TrimPrefix(ln, target+":")
		// Fold GNU make backslash continuations into the head. Without this a
		// prerequisite list wrapped across lines (valid make, identical
		// semantics) silently loses every prerequisite after the first line AND
		// contributes a literal "\\" field -- which made this test report four
		// nonexistent coverage regressions.
		j := i
		for strings.HasSuffix(strings.TrimRight(head, " \t"), "\\") {
			head = strings.TrimSuffix(strings.TrimRight(head, " \t"), "\\")
			if j+1 >= len(lines) {
				j = len(lines) - 1 // clamp: continued head ran off the corpus
				break
			}
			j++
			head += " " + lines[j]
		}
		for _, next := range lines[j+1:] {
			if next != "" && !strings.HasPrefix(next, "\t") && !strings.HasPrefix(next, " ") {
				break // a new rule head or variable at column 0 ends the recipe
			}
			body = append(body, next)
		}
		return head, withDelegatedRecipes(corpus, body, map[string]bool{target: true})
	}
	t.Fatalf("target %q not found in the Makefile corpus. This test must not pass vacuously.", target)
	return "", nil
}

// withDelegatedRecipes appends the recipe of every target the body invokes
// through $(MAKE), so a delegating recipe reads as the commands it runs.
//
// Additive: the delegating line stays, because a caller may be asking about the
// delegation itself. `seen` carries the target already being read, so a
// recursive target contributes its recipe once and a cycle terminates.
func withDelegatedRecipes(corpus string, body []string, seen map[string]bool) []string {
	out := append([]string(nil), body...)
	for _, ln := range body {
		for _, target := range makeInvocationTargets(ln) {
			if seen[target] {
				continue
			}
			seen[target] = true
			sub := optionalRecipeBody(corpus, target)
			if len(sub) == 0 {
				continue
			}
			out = append(out, withDelegatedRecipes(corpus, sub, seen)...)
		}
	}
	return out
}

// makeTargetName is the shape of a target a recipe can hand to $(MAKE). It
// excludes a flag (-C, --no-print-directory), a `VAR=value` override, and a
// `$(VAR)` reference, none of which names a rule.
var makeTargetName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9._-]*$`)

// makeInvocationTargets returns the target names a single recipe line asks
// $(MAKE) to build. A line with no recursive make yields nothing.
func makeInvocationTargets(line string) []string {
	fields := strings.Fields(line)
	var targets []string
	seenMake := false
	for _, f := range fields {
		if f == "$(MAKE)" || f == "${MAKE}" {
			seenMake = true
			continue
		}
		if !seenMake {
			continue
		}
		if strings.ContainsAny(f, ";&|") {
			seenMake = false // a shell separator ends the make invocation
			continue
		}
		if makeTargetName.MatchString(f) {
			targets = append(targets, f)
		}
	}
	return targets
}

// optionalRecipeBody is recipeBody for a name that MIGHT not be a target.
//
// The deeper walk feeds it every field of a sub-target's rule head, and a
// prerequisite is legally a file (`ze-doc-index-update: bin/ze`), a variable
// (`$(DEPS)`), or the order-only separator `|`. Those are not rule heads, and
// hard-failing on them would report "target not found ... must not pass
// vacuously" at a reader who did nothing wrong. Skip them instead; a real
// generator hidden behind a file prerequisite is not a shape this repo uses.
func optionalRecipeBody(corpus, target string) []string {
	if target == "" || target == "|" || strings.HasPrefix(target, "$") {
		return nil
	}
	lines := strings.Split(corpus, "\n")
	for i, ln := range lines {
		if !strings.HasPrefix(ln, target+":") {
			continue
		}
		var body []string
		for _, next := range lines[i+1:] {
			if next != "" && !strings.HasPrefix(next, "\t") && !strings.HasPrefix(next, " ") {
				break
			}
			body = append(body, next)
		}
		return body
	}
	return nil
}

// producerScripts extracts every generator invocation from a make recipe.
//
// Deliberately NOT limited to `scripts/**.{go,py}`: an earlier version was, and
// a reviewer showed that `@go run ./cmd/newgen` was silently exempt from the
// coverage requirement. The interpreter set below covers every form this repo
// actually uses -- including `$(GO) run`, which is house style at the ze-precommit-verify
// and ze-precommit-verify-changed recipes, and `uv run`, used by the ExaBGP suite. A
// HONEST LIMIT -- this is best-effort detection, NOT a guarantee. An earlier
// version of this comment claimed a generator in an unlisted form "is NOT
// silently ignored, because the exact-count assertions fail when the parsed set
// changes size". A reviewer measured that false twice over: (a) there is NO
// count assertion for generators found inside SUB-TARGET recipes -- the only
// counts are on `generate:`'s 4 and `ze-generated-files-update`'s 9 prerequisites; and (b) even
// inside `generate:`, a form yielding no capture leaves the count at 4, so the
// tripwire never fires. Measured misses at the time: `go run -tags x ./cmd/y`,
// `python3 -m pkg.mod`, `$(MAKE) some-target`. The first two are now matched
// below; `$(MAKE)` is not, and a recursive-make generator would still slip
// through. Extend the patterns when a new form appears rather than trusting
// this to catch it.
//
// Normalisation matters as much as matching:
//   - line comments are stripped first, so a commented-out `# go run x.go`
//     cannot produce a phantom finding;
//   - backslash continuations are folded, so a wrapped command does not yield
//     the literal "\\" as a "generator" -- the same bug round 3 fixed in
//     recipeBody's rule head, which was still live down here;
//   - a `$(VAR)`/`${VAR}` interpreter prefix is accepted, since the operand is
//     what we care about.
func producerScripts(recipe string) []string {
	// Fold continuations, then strip comments. Order matters: a `#` on a
	// continued line comments out the whole logical line.
	recipe = continuationRE.ReplaceAllString(recipe, " ")
	var kept []string
	for ln := range strings.SplitSeq(recipe, "\n") {
		kept = append(kept, stripShellComment(ln))
	}
	joined := strings.Join(kept, "\n")

	var out []string
	seen := map[string]bool{}
	for _, re := range producerREs {
		for _, m := range re.FindAllStringSubmatch(joined, -1) {
			p := strings.TrimSpace(m[len(m)-1])
			if p == "" || strings.HasPrefix(p, "-") || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// stripShellComment removes a `#` comment from a recipe line, respecting shell
// quoting.
//
// A blunt strings.Index(ln, "#") is wrong in the DANGEROUS direction: a `#`
// inside quotes is a literal, not a comment, so `@echo "# hi" && python3
// scripts/dev/newgen.py` had everything after the quote stripped and the
// generator went UNDETECTED. That is a false negative in a guard whose whole
// job is to notice an unguarded generator -- strictly worse than the false
// positive (a commented-out `# go run x.go` counting as real) that the blunt
// strip was added to fix. Track quote state so both cases are right.
func stripShellComment(ln string) string {
	var qs rune // 0 = unquoted, else the open quote character
	for i, r := range ln {
		switch {
		case qs != 0:
			if r == qs {
				qs = 0
			}
		case r == '\'' || r == '"':
			qs = r
		case r == '#':
			return ln[:i]
		}
	}
	return ln
}

// continuationRE matches a backslash-newline (plus the next line's leading
// whitespace) so a wrapped recipe line reads as one logical line.
var continuationRE = regexp.MustCompile(`\\\n[ \t]*`)

// interpreterPrefix allows a literal name or a make variable reference:
// `go`, `$(GO)`, `${GO}`, `python3`, `uv`, `bash`, `sh`.
const interpreterPrefix = `(?:\$[({][A-Za-z_][A-Za-z0-9_]*[)}]|[A-Za-z0-9_./-]+)`

var producerREs = []*regexp.Regexp{
	// `go run <pkg>`, `$(GO) run <pkg>`, `uv run <script>`, plus the flagged
	// form `go run -tags x <pkg>`: skip leading flags (and a flag's separate
	// argument) so the PACKAGE is captured rather than `-tags`.
	regexp.MustCompile(interpreterPrefix + `\s+run\s+(?:-\S+\s+(?:[^-\s]\S*\s+)?)*(\S+)`),
	// `python3 -m pkg.mod`
	regexp.MustCompile(`python3?\s+-m\s+(\S+)`),
	// `python3 <script>.py`, `$(PYTHON) <script>.py`
	regexp.MustCompile(interpreterPrefix + `\s+(\S+\.py)`),
	// `bash <script>.sh`, `sh <script>.sh`
	regexp.MustCompile(`\b(?:ba)?sh\s+(\S+\.sh)`),
	// a script invoked directly as the first word of a recipe line
	regexp.MustCompile(`(?m)^\t@?(\S+\.(?:sh|py))\b`),
}

// TestRegenCheckReadonlyCoversGenerators enforces the spec's coverage
// obligation as a test rather than a comment.
//
// VALIDATES: ze-generated-files-check carries exactly the documented
// prerequisite set, and every generator reachable from `make generate` or
// `make ze-generated-files-update` is guarded by one of them (or is an explicit, reasoned
// exclusion).
// PREVENTS: two distinct drifts. (a) A new generator added to `generate` or
// `ze-generated-files-update` with no read-only check, leaving its output's staleness unguarded
// everywhere -- the drift class this spec exists to kill, one level up from the
// stage list; two of the four codegen scripts (feature_tags, fuzz-targets) were
// previously guarded ONLY by the mutating ze-generated-files-reconcile's `git diff`, which
// runs nowhere. (b) A prerequisite being quietly dropped from the target, which
// matters most for ze-arch-map-check: unlike the other doc checks it is NOT
// also run by ze-doc-verify, so deleting it would leave ai/INSTRUCTIONS.md's
// architecture lists with no guard at all.
func TestRegenCheckReadonlyCoversGenerators(t *testing.T) {
	corpus := makefileCorpus(t)

	head, _ := recipeBody(t, corpus, "ze-generated-files-check")
	got := map[string]bool{}
	for f := range strings.FieldsSeq(head) {
		got[f] = true
	}
	for want, why := range regenCheckPrereqs {
		if !got[want] {
			t.Errorf("ze-generated-files-check is missing prerequisite %q (%s); that output's staleness would be unguarded under `make ze-precommit-verify`", want, why)
		}
	}
	for have := range got {
		if _, known := regenCheckPrereqs[have]; !known {
			t.Errorf("ze-generated-files-check has undocumented prerequisite %q; add it to regenCheckPrereqs with the generator and output it guards", have)
		}
	}

	// Every generator reachable from `make generate` or `make ze-generated-files-update`.
	//
	// Walk the sub-targets' RECIPES too, not just their names. ze-discovery-index-update
	// and ze-ai-instructions-generate run generator scripts of their own, so stopping at
	// the name would let a fourth script added inside ze-discovery-index-update's recipe
	// go completely unguarded -- proven by a reviewer.
	var producers []string
	_, genBody := recipeBody(t, corpus, "generate")
	producers = append(producers, producerScripts(strings.Join(genBody, "\n"))...)
	if len(producers) != 7 {
		t.Fatalf("parsed %d generators from the `generate:` recipe (%v), expected 7; the recipe changed. Update generatorChecks to match. This test must not pass vacuously.", len(producers), producers)
	}

	regenHead, _ := recipeBody(t, corpus, "ze-generated-files-update")
	subTargets := strings.Fields(regenHead)
	// EXACT, not a floor: with `>= 5` a prerequisite could be DELETED from
	// ze-generated-files-update unnoticed, so `make ze-generated-files-update` would stop regenerating an output
	// that ze-generated-files-check (an un-parkable structural gate) still checks
	// -- a red whose documented remediation does not fix it.
	if len(subTargets) != 9 {
		t.Fatalf("`ze-generated-files-update` has %d prerequisites (%v), expected 9; if that is deliberate, update this count and generatorChecks together.", len(subTargets), subTargets)
	}
	producers = append(producers, subTargets...)
	for _, sub := range subTargets {
		if sub == "generate" {
			continue // already expanded above
		}
		_, subBody := recipeBody(t, corpus, sub)
		producers = append(producers, producerScripts(strings.Join(subBody, "\n"))...)
		// A sub-target may itself have prerequisites (ze-ai-instructions-generate:
		// ze-arch-map-update); walk those recipes too, one level.
		subHead, _ := recipeBody(t, corpus, sub)
		for sub2 := range strings.FieldsSeq(subHead) {
			sub2Body := optionalRecipeBody(corpus, sub2)
			producers = append(producers, producerScripts(strings.Join(sub2Body, "\n"))...)
		}
	}

	for _, producer := range producers {
		target, known := generatorChecks[producer]
		if !known {
			t.Errorf("`make generate`/`ze-generated-files-update` runs %q, but generatorChecks does not say which read-only check guards its output. Wire one into ze-generated-files-check and record it, or record it as a reasoned exclusion -- otherwise its staleness is unguarded everywhere.", producer)
			continue
		}
		if target == "" {
			continue // documented exclusion
		}
		if !got[target] {
			t.Errorf("%q is guarded by %q, but that is not a prerequisite of ze-generated-files-check, so it never runs under `make ze-precommit-verify`", producer, target)
		}
	}
}

// templCallerTargets are the make targets allowed to run the templ CLI. The
// write path (generate) and the read-only path (ze-templ-output-check) hold
// different obligations, so a third caller must be judged here before it runs.
var templCallerTargets = []string{"generate", "ze-templ-output-check"}

// makeRuleHead matches a rule head at column 0 and captures its prerequisites.
// The negative lookahead-free `[^=]` on the tail is what keeps `FOO := bar` and
// `FOO: = bar` out: a variable assignment is not a rule.
var makeRuleHead = regexp.MustCompile(`^([A-Za-z0-9_./%-]+):(?:([^=].*)?)$`)

// TestTemplCheckIsReadOnlyAndReportsOrphans pins the two properties that let a
// check-mode gate run over a working tree.
//
// VALIDATES: every `templ generate -check` recipe line also passes
// -keep-orphaned-files, and ze-templ-output-check keeps the
// ze-templ-orphan-check prerequisite that runs
// scripts/dev/templ_orphan_check.py.
// PREVENTS: two opposite failures from one flag. Without it templ DELETES an
// orphaned *_templ.go in -check mode, because HandleEvent
// (vendor/github.com/a-h/templ/cmd/templ/generatecmd/eventhandler.go) calls
// os.Remove before any writer is consulted, and only keepOrphanedFiles gates
// that call. With it templ says NOTHING about the orphan, because the same
// branch returns before the check writer sees the file. So the flag makes the
// gate read-only, and the python check is then the only report of a generated
// file whose source is gone.
//
// The caller set is DERIVED, but only from LITERAL recipe lines. A templ run
// reached through a make variable, a shell script or a recursive $(MAKE) is
// invisible here. What this test does catch is a second call site added to any
// Makefile or mk/*.mk that spells the vendored package path. A literal `templ`
// against a PATH binary is invisible too, and nothing installs one today. The
// check's own logic is tested in
// scripts/dev/templ_orphan_check_test.py.
func TestTemplCheckIsReadOnlyAndReportsOrphans(t *testing.T) {
	corpus := makefileCorpus(t)
	const detector = "ze-templ-orphan-check"

	prereqs := map[string]string{}
	templLines := map[string][]string{}
	var callers []string
	target := ""
	for ln := range strings.SplitSeq(corpus, "\n") {
		if strings.HasPrefix(ln, "\t") {
			if cmd := stripShellComment(ln); strings.Contains(cmd, "a-h/templ/cmd/templ") {
				if !slices.Contains(callers, target) {
					callers = append(callers, target)
				}
				templLines[target] = append(templLines[target], strings.TrimSpace(cmd))
			}
			continue
		}
		m := makeRuleHead.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		target = m[1]
		prereqs[target] = m[2]
	}

	slices.Sort(callers)
	if !slices.Equal(callers, templCallerTargets) {
		t.Fatalf("targets running the templ CLI are %v, expected %v; a new caller must be judged read-only or write before it is added here", callers, templCallerTargets)
	}
	for _, caller := range callers {
		for _, cmd := range templLines[caller] {
			if strings.Contains(cmd, "-check") && !strings.Contains(cmd, "-keep-orphaned-files") {
				t.Errorf("`make %s` runs templ in check mode without -keep-orphaned-files (%q), so the check deletes an orphaned *_templ.go instead of leaving the tree alone", caller, cmd)
			}
		}
	}

	if !slices.Contains(strings.Fields(prereqs["ze-templ-output-check"]), detector) {
		t.Errorf("ze-templ-output-check lost the %q prerequisite (its prerequisites are %q); with -keep-orphaned-files templ reports no orphan, so nothing would", detector, strings.TrimSpace(prereqs["ze-templ-output-check"]))
	}
	if _, ok := prereqs[detector]; !ok {
		t.Fatalf("%q is not a target in the Makefile: the prerequisite above resolves to nothing and no orphan is reported", detector)
	}
	_, detectorBody := recipeBody(t, corpus, detector)
	if !strings.Contains(strings.Join(detectorBody, "\n"), "scripts/dev/templ_orphan_check.py") {
		t.Errorf("%q does not run scripts/dev/templ_orphan_check.py; its recipe is %q", detector, strings.Join(detectorBody, "\n"))
	}
}

// TestMakeDryRunDetectsDashN guards the refusal that keeps `make -n ze-precommit-verify`
// from forging a verified state.
//
// VALIDATES: makeDryRun reads MAKEFLAGS the way GNU make writes it -- short
// flags concatenated in the first field with no leading dash.
// PREVENTS: the single worst failure this runner can have. ze-precommit-verify's recipe
// contains $(MAKE), which GNU make executes even under -n, propagating -n to
// every stage sub-make; each then echoes its recipe and exits 0. Writing
// tmp/ze-verify.status from that run stamps exit=0 against the CURRENT tree
// hash, so verify-status.sh reports FRESH and commit_helper.py sees no
// structural-gate reds -- one dry run would certify a completely unverified
// tree. A false NEGATIVE here re-opens that hole; a false POSITIVE would refuse
// real verify runs, so both directions are pinned.
func TestMakeDryRunDetectsDashN(t *testing.T) {
	for _, tc := range []struct {
		makeflags string
		want      bool
		why       string
	}{
		{"", false, "no flags"},
		{"n", true, "make -n"},
		{"n -j4 --jobserver-auth=3,4", true, "make -n -j4"},
		{"n -- FOO=bar", true, "make --just-print FOO=bar"},
		{"rRn", true, "short flags concatenated"},
		{"s", false, "make -s is not a dry run"},
		{"rR", false, "no n among the short flags"},
		{"-- FOO=nnn", false, "an 'n' in a variable value must not match"},
		{"--debug=n", false, "a long option is not the short-flag field"},
		{"w", false, "make -w"},
		// Every string below was captured from a real GNU make run against a
		// scratch Makefile, not guessed. The -j cases matter most: MAKEFLAGS
		// starts with a SPACE there, so the first field is "-j8" and the
		// leading-dash check is what keeps a parallel verify from being refused.
		{" -j8 --jobserver-auth=3,4", false, "make -j8"},
		{" -j8 --jobserver-auth=3,4 --no-print-directory", false, "make -j8 --no-print-directory"},
		{" --no-print-directory", false, "make --no-print-directory"},
		{"k --no-print-directory", false, "make -k --no-print-directory"},
		{"k -j8 --jobserver-auth=3,4", false, "make -k -j8"},
		{"i", false, "make -i"},
		{"B", false, "make -B"},
		{"Bs", false, "make -B -s"},
		{"Bn", true, "make -B -n"},
		{"B -j8 --jobserver-auth=3,4", false, "MAKEFLAGS=-j8 exported, make -B"},
		// -t and -q are no-execute modes too, and -t is the dangerous one: like
		// -n it still executes $(MAKE) recipe lines, but a .PHONY stage then
		// reports "Nothing to be done" and exits 0 -- QUIETER than -n, which at
		// least echoes each recipe. Measured: with MAKEFLAGS=t a .PHONY target
		// whose recipe is `exit 9` exits 0.
		{"t", true, "make -t / --touch: stages no-op and 'succeed'"},
		{"t --no-print-directory", true, "make -t, real observed MAKEFLAGS"},
		{"q", true, "make -q / --question: runs nothing; would clobber the record with a false red"},
		{"Bt", true, "-t among concatenated short flags"},
		{"rRt", true, "-t after -r -R"},
		// Guard the negative side of the widened match: these letters are in no
		// short-flag position that means no-execute.
		{"w", false, "make -w has no n/t/q"},
		{" -j8 --jobserver-auth=3,4 -- TARGET=not-a-flag", false, "n/t/q in a variable value must not match"},
		// GNU make 3.81 (the macOS system make) writes a command-line variable
		// override as the FIRST MAKEFLAGS word with no `--` separator and no
		// leading space (captured: `make ze-precommit-verify ZE_VERIFY_LOG=tmp/x.log` ->
		// MAKEFLAGS="ZE_VERIFY_LOG=tmp/ze-verify-gate12.log"). A flags word can
		// never contain '=', so an override word must not be read as flags --
		// without that, the 't' in "tmp" refused the exact invocation the
		// bash-output rule recommends (feature-gate-12 session, 2026-07-23).
		{"ZE_VERIFY_LOG=tmp/ze-verify-gate12.log", false, "GNU make 3.81 bare variable override; 't'/'q' in the value must not match"},
		{"n ZE_VERIFY_LOG=tmp/x.log", true, "real -n still detected ahead of a 3.81-style override"},
	} {
		if got := makeDryRun(tc.makeflags); got != tc.want {
			t.Errorf("makeDryRun(%q) = %v, want %v (%s)", tc.makeflags, got, tc.want, tc.why)
		}
	}
}

// TestStagesForModeRejectsUnknownMode pins the fail-closed behavior of an
// unrecognized mode.
//
// VALIDATES: stagesForMode returns no stages for a mode it does not know, which
// runVerify turns into exit 2 ("no verify stages configured").
// PREVENTS: a typo silently running the FULL verify list and then recording
// mode=<typo> in tmp/ze-verify.status, which verify-status.sh renders as
// FRESH(<typo>) -- a verified-looking record for a mode nobody asked for. The
// unknown mode used to fall through to the default branch, which WAS the full
// list.
func TestStagesForModeRejectsUnknownMode(t *testing.T) {
	for _, mode := range []string{"ze-verify-chnaged", "", "garbage", "--list"} {
		if got := stagesForMode(mode, "make"); len(got) != 0 {
			t.Errorf("stagesForMode(%q) returned %d stages, want 0 (fail closed)", mode, len(got))
		}
	}
	// ...while the two real modes still work, so this is not vacuous.
	for _, mode := range []string{"ze-precommit-verify", "ze-precommit-verify-changed"} {
		if got := stagesForMode(mode, "make"); len(got) == 0 {
			t.Errorf("stagesForMode(%q) returned no stages", mode)
		}
	}
}

// ── Supply-chain / SCA workflow guards (plan/spec-fixit-supply-chain-hardening) ─
//
// These read the CI workflow files directly (the source of truth CI executes) and
// assert their SHAPE, the same technique scripts/dev/github_workflows_test.go uses
// for the verify/nightly workflows.

// stripYAMLComments removes `#` comments from every line of a YAML document,
// respecting single/double quotes. Assertions must match executable content, not
// prose in a header or trailing comment: govulncheck.yml's comments mention
// push/pull_request, and codeql.yml's comments name the build tags -- a raw
// strings.Contains would then trip the "no push trigger" guard or pass the "-tags"
// guard on the comment alone.
func stripYAMLComments(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = stripShellComment(ln)
	}
	return strings.Join(lines, "\n")
}

// yamlMappingEntries returns exact direct string keys. It does not expand
// aliases or YAML merge keys.
func yamlMappingEntries(t *testing.T, node *yaml.Node, context string) map[string]*yaml.Node {
	t.Helper()
	if node.Kind != yaml.MappingNode || len(node.Content) == 0 || len(node.Content)%2 != 0 {
		t.Fatalf("%s must be a non-empty YAML mapping", context)
	}
	entries := make(map[string]*yaml.Node, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		if key.Kind != yaml.ScalarNode || key.ShortTag() != "!!str" {
			t.Fatalf("%s has a non-string or indirect key", context)
		}
		if _, duplicate := entries[key.Value]; duplicate {
			t.Fatalf("%s has duplicate key %q", context, key.Value)
		}
		entries[key.Value] = node.Content[i+1]
	}
	return entries
}

// topLevelYAMLMap returns the direct entries in one top-level YAML mapping.
func topLevelYAMLMap(t *testing.T, body, key string) map[string]*yaml.Node {
	t.Helper()
	decoder := yaml.NewDecoder(strings.NewReader(body))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("parse workflow YAML: %v", err)
	}
	var trailing yaml.Node
	switch err := decoder.Decode(&trailing); {
	case err == nil:
		t.Fatal("workflow must contain exactly one YAML document")
	case errors.Is(err, io.EOF):
	default:
		t.Fatalf("parse trailing workflow YAML: %v", err)
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		t.Fatal("workflow must contain one top-level YAML document")
	}
	root := yamlMappingEntries(t, document.Content[0], "workflow")
	value, found := root[key]
	if !found {
		t.Fatalf("workflow has no top-level %q mapping", key)
	}
	return yamlMappingEntries(t, value, "workflow top-level "+key)
}

// TestGovulncheckScheduledWorkflow pins the dedicated SCA workflow's scheduled
// and manual triggers while normal local verification also runs the scan.
//
// VALIDATES: .github/workflows/govulncheck.yml remains scheduled and manually
// dispatchable, invokes `make ze-dependency-vulnerability-check`, never triggers on push/pull_request,
// and both local verification modes contain the exact `ze-dependency-vulnerability-check` stage.
// PREVENTS: (a) the dedicated scheduled or manual trigger disappearing; (b) the
// dedicated workflow moving onto push/pull_request events; (c) either normal
// verification mode silently omitting the mandatory vulnerability scan.
func TestGovulncheckScheduledWorkflow(t *testing.T) {
	raw := readFile(t, repoRootFromScriptsStatus(), ".github/workflows/govulncheck.yml")
	body := stripYAMLComments(raw)
	triggers := topLevelYAMLMap(t, raw, "on")

	schedule, scheduled := triggers["schedule"]
	if !scheduled {
		t.Error("govulncheck.yml must declare a direct scheduled trigger")
	} else {
		if schedule.Kind != yaml.SequenceNode || len(schedule.Content) == 0 {
			t.Error("govulncheck.yml scheduled trigger must be a non-empty cron list")
		} else {
			for i, item := range schedule.Content {
				entry := yamlMappingEntries(t, item, "scheduled trigger entry")
				cron, present := entry["cron"]
				if !present {
					t.Errorf("govulncheck.yml schedule entry %d has no direct cron key", i)
					continue
				}
				if cron.Kind != yaml.ScalarNode ||
					cron.ShortTag() != "!!str" ||
					strings.TrimSpace(cron.Value) == "" {
					t.Errorf("govulncheck.yml schedule entry %d must contain a non-empty string cron value", i)
				}
			}
		}
	}
	if _, manual := triggers["workflow_dispatch"]; !manual {
		t.Error("govulncheck.yml must declare a direct manual trigger")
	}
	for _, forbidden := range []string{"push", "pull_request"} {
		if _, present := triggers[forbidden]; present {
			t.Errorf("govulncheck.yml must NOT declare a direct %q trigger", forbidden)
		}
	}
	// Invokes the single-source-of-truth make target.
	if !strings.Contains(body, "make ze-dependency-vulnerability-check") {
		t.Errorf("govulncheck.yml must run `make ze-dependency-vulnerability-check`; body (comments stripped):\n%s", body)
	}
	// govulncheck is an exact stagesForMode entry in BOTH local verification
	// modes, so neither path can silently skip the mandatory scan.
	for _, mode := range []string{"ze-precommit-verify", "ze-precommit-verify-changed"} {
		found := false
		for _, st := range stagesForMode(mode, "make") {
			if st.Name == "ze-dependency-vulnerability-check" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("stagesForMode(%q) must contain the exact stage %q", mode, "ze-dependency-vulnerability-check")
		}
	}
}

// TestGovulncheckTargetUsesLinuxAMD64Analysis pins the platform boundary of the
// local SCA target.
//
// VALIDATES: `ze-dependency-vulnerability-check` runs one host-native `$(GO) run` invocation whose
// exec wrapper gives only the scanner process GOOS=linux and GOARCH=amd64.
// PREVENTS: cross-compiling the govulncheck tool so it cannot run on a non-Linux
// host, or silently analyzing a host-specific dependency graph instead of Linux.
func TestGovulncheckTargetUsesLinuxAMD64Analysis(t *testing.T) {
	_, body := recipeBody(t, makefileCorpus(t), "ze-dependency-vulnerability-check")

	var invocations []string
	for _, line := range body {
		line = strings.TrimPrefix(strings.TrimSpace(line), "@")
		if strings.Contains(line, "$(GO) run") {
			invocations = append(invocations, line)
		}
	}
	if len(invocations) != 1 {
		t.Fatalf("ze-dependency-vulnerability-check must have exactly one host-native `$(GO) run` invocation, got %d. Recipe:\n%s", len(invocations), strings.Join(body, "\n"))
	}

	const wantInvocation = "$(GO) run -exec='env GOOS=linux GOARCH=amd64' golang.org/x/vuln/cmd/govulncheck@latest ./..."
	if invocation := invocations[0]; invocation != wantInvocation {
		t.Errorf("ze-dependency-vulnerability-check must use the exact host-native Linux/amd64 scanner invocation.\nWant: %q\nGot:  %q", wantInvocation, invocation)
	}
}

// TestCodeQLBuildUsesShippedTags pins that CodeQL analyzes the feature-gated
// surface with CGO disabled, not a bare or host-dependent build.
//
// VALIDATES: the manual Go build step in .github/workflows/codeql.yml compiles the
// shipped tag combinations (ze_core/ze_distro/ze_appliance/ze_setup) with
// CGO_ENABLED=0. Thus, code behind ze_core and ze_appliance enters the CodeQL
// database without a host C toolchain.
// PREVENTS: a tagless or host-dependent build excluding the largest attack surface.
func TestCodeQLBuildUsesShippedTags(t *testing.T) {
	body := stripYAMLComments(readFile(t, repoRootFromScriptsStatus(), ".github/workflows/codeql.yml"))
	var buildLines []string
	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "go build ") {
			continue
		}
		buildLines = append(buildLines, line)
		if !strings.HasPrefix(line, "CGO_ENABLED=0 go build ") {
			t.Errorf("codeql.yml Go build must set CGO_ENABLED=0: %s", line)
		}
	}
	if len(buildLines) != 3 {
		t.Fatalf("codeql.yml must contain exactly three shipped Go build commands, got %d: %v", len(buildLines), buildLines)
	}

	if !strings.Contains(body, "-tags") {
		t.Fatalf("codeql.yml build step must pass -tags (the shipped feature set); body (comments stripped):\n%s", body)
	}
	for _, tag := range []string{"ze_core", "ze_distro", "ze_appliance", "ze_setup"} {
		if !strings.Contains(body, tag) {
			t.Errorf("codeql.yml build must build the %q tag so its feature-gated code enters the CodeQL DB", tag)
		}
	}
	// Drift guard: the shipped bin/ze / bin/ze-appliance builds add the default-on
	// service feature set $(ZE_FEATURES) (feature-gates.txt). A static workflow
	// cannot expand a Makefile variable, so codeql.yml's tag lists are GENERATED
	// from feature-gates.txt by scripts/codegen/feature_tags.go (make generate);
	// this asserts none has fallen behind the manifest -- a regenerate skipped, or
	// a tag added without regenerating, would silently exclude the gated code from
	// SAST (adding a service tag without teaching CodeQL to build it).
	for _, tag := range shippedFeatureTags(t) {
		if !strings.Contains(body, tag) {
			t.Errorf("codeql.yml build omits shipped feature tag %q (feature-gates.txt): its feature-gated code would be excluded from CodeQL analysis", tag)
		}
	}
	// A bare `go build ./...` (no tags) compiles none of the gated surface.
	if strings.Contains(body, "go build ./...") {
		t.Errorf("codeql.yml still contains a bare `go build ./...`: the feature-gated surface would be excluded from CodeQL analysis")
	}
}

// shippedFeatureTags returns the default-on service feature tags ($(ZE_FEATURES)
// in the Makefile): the first whitespace-field of each feature-gates.txt line that
// begins with "ze_". This mirrors `awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt`.
func shippedFeatureTags(t *testing.T) []string {
	t.Helper()
	body := readFile(t, repoRootFromScriptsStatus(), "feature-gates.txt")
	seen := map[string]bool{}
	var tags []string
	for line := range strings.SplitSeq(body, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if tag := fields[0]; strings.HasPrefix(tag, "ze_") && !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	if len(tags) == 0 {
		t.Fatalf("feature-gates.txt yielded no ze_ feature tags; parser or file changed")
	}
	return tags
}

// makeVarRefRE matches a `$(NAME)` or `${NAME}` variable reference. A command
// substitution (`$$(cmd)`) and a shell variable (`$$pkgs`) do not match: the
// name shape below admits neither.
var makeVarRefRE = regexp.MustCompile(`\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]`)

// makeVarDefRE matches a simple variable definition at column 0.
var makeVarDefRE = regexp.MustCompile(`(?m)^([A-Za-z_][A-Za-z0-9_]*)\s*[:?+]?=[ \t]*(.*)$`)

// expandMakeVars resolves simple Makefile variable references in one line.
//
// A test that asks what a recipe RUNS has to see through the variables the
// recipe is written with. `ze-lint` invokes `$(ZE_LINT)`, which carries both
// the linter and its memory ceiling, so a matcher looking for the literal
// `golangci-lint` reads a recipe that runs it as a recipe that does not.
//
// Deliberately limited: only a definition at column 0 is read, the last one
// wins as it does after make finishes parsing, and an unknown name is left
// alone so it stays visible in whatever the caller reports.
func expandMakeVars(corpus, line string) string {
	values := map[string]string{}
	for _, m := range makeVarDefRE.FindAllStringSubmatch(corpus, -1) {
		values[m[1]] = strings.TrimSpace(m[2])
	}
	// Bounded rounds resolve nesting (ZE_LINT holds $(ZE_LINT_MEMLIMIT))
	// without looping forever on a self-referential definition.
	for range 5 {
		expanded := makeVarRefRE.ReplaceAllStringFunc(line, func(ref string) string {
			name := makeVarRefRE.FindStringSubmatch(ref)[1]
			if v, ok := values[name]; ok {
				return v
			}
			return ref
		})
		if expanded == line {
			break
		}
		line = expanded
	}
	return line
}

// integrationLintPass reports whether a recipe body carries the second
// golangci-lint invocation: GOOS=linux plus the integration build tag.
//
// The corpus is read so the body's variable references resolve; see
// expandMakeVars.
func integrationLintPass(corpus string, body []string) bool {
	for _, ln := range body {
		ln = expandMakeVars(corpus, ln)
		if strings.Contains(ln, "GOOS=linux") && strings.Contains(ln, "golangci-lint") &&
			strings.Contains(ln, "--build-tags integration") {
			return true
		}
	}
	return false
}

// TestLintCoversIntegrationTaggedFiles guards the full lint's coverage of the
// two file populations golangci-lint's single analyzed build cannot reach.
//
// VALIDATES: `ze-lint` runs a second golangci-lint pass under GOOS=linux with
// the integration build tag, and that `integration` reached the linter WITHOUT
// entering either manifest it does not belong in.
// PREVENTS: the regression this gate was written for. golangci-lint analyses ONE
// build -- the host GOOS with .golangci.yml's build-tags -- so 77 tracked files
// behind //go:build integration were reported on by nothing, in CI or locally,
// and carried 132 findings when first measured. Delete the second pass and they
// go silent again with every gate still green.
//
// It also pins the two shapes the fix must NOT take. `.golangci.yml` build-tags
// is GENERATED from feature-gates.txt (scripts/codegen/feature_tags.go), so a
// hand-added `integration` there is reverted by the next `make generate` and
// fails dep_audit.py --check in between; and `integration` gates tests by
// capability, so in feature-gates.txt every other consumer of that manifest
// would read it as a shippable feature.
func TestLintCoversIntegrationTaggedFiles(t *testing.T) {
	corpus := makefileCorpus(t)
	_, body := recipeBody(t, corpus, "ze-lint")
	if !integrationLintPass(corpus, body) {
		t.Errorf("ze-lint runs no `GOOS=linux golangci-lint run --build-tags integration` pass: "+
			"every //go:build integration file is unlinted again. Recipe:\n%s", strings.Join(body, "\n"))
	}

	golangci := readFile(t, repoRootFromScriptsStatus(), ".golangci.yml")
	if !strings.Contains(golangci, "\n    - ze_core\n") {
		t.Fatalf(".golangci.yml has no `- ze_core` build-tags entry: the list shape changed and this test must not pass vacuously")
	}
	for line := range strings.SplitSeq(golangci, "\n") {
		if strings.TrimSpace(line) == "- integration" {
			t.Errorf(".golangci.yml lists `integration` in build-tags. That file is GENERATED by " +
				"scripts/codegen/feature_tags.go: the entry is reverted by the next `make generate`. " +
				"The integration tag reaches the linter from the ze-lint recipe instead")
		}
	}

	for _, tag := range shippedFeatureTags(t) {
		if tag == "integration" {
			t.Errorf("feature-gates.txt lists `integration`: it gates tests by capability, not a " +
				"compile-out-able feature, so every consumer of the manifest would treat it as shippable")
		}
	}
}

// TestChangedLintCoversIntegrationTaggedFiles is TestLintCoversIntegrationTaggedFiles
// for the path most edits actually face.
//
// VALIDATES: `ze-lint-changed` runs the same second pass, and that a failure in
// the first pass cannot be masked by a clean second one.
// PREVENTS: a new QEMU integration test landing unlinted.
// `ai/rules/commands.md` makes ze-lint-changed the gate to run before you claim
// done, and ze-precommit-verify-changed runs it in place of ze-lint
// (stagesForMode). Full-lint-only coverage would therefore leave exactly the new
// files uncovered.
func TestChangedLintCoversIntegrationTaggedFiles(t *testing.T) {
	corpus := makefileCorpus(t)
	_, body := recipeBody(t, corpus, "ze-lint-changed")
	if !integrationLintPass(corpus, body) {
		t.Errorf("ze-lint-changed runs no `GOOS=linux golangci-lint run --build-tags integration` pass: "+
			"a new //go:build integration file passes the changed-file gate unlinted. Recipe:\n%s", strings.Join(body, "\n"))
	}
	// Fail-closed chaining: the recipe is one shell. A `;` between the two passes
	// would make the recipe's status the SECOND pass's. A real finding in the
	// first pass would then exit 0. TestChangedLintRunsEveryBuildFlavor checks
	// the whole chain, the flavor driver included. This one keeps the assertion
	// beside the integration pass it is about.
	passes, _ := lintPassLines(corpus, body)
	if len(passes) < 2 {
		t.Fatalf("ze-lint-changed runs %d lint passes, so there is no chain to judge. Recipe:\n%s",
			len(passes), strings.Join(body, "\n"))
	}
	first := body[passes[0]]
	if !strings.HasSuffix(strings.TrimRight(first, " \t"), "&& \\") {
		t.Errorf("ze-lint-changed chains its first golangci-lint pass without `&&`, so its exit "+
			"status is discarded when a later pass is clean. Line: %q", first)
	}
}

// lintPassLines returns the recipe lines that run a lint pass, expanded, in
// recipe order. It also returns the index of the last non-blank line of the
// body.
//
// A pass is golangci-lint (through ZE_LINT_RUN, or spelled out) or the flavor
// driver (through ZE_LINT_FLAVOR_RUN). The expansion is what makes both
// visible. The raw line reads `$(ZE_LINT_RUN) $$pkgs`, which does not hold the
// word golangci-lint. It holds no spelling that a substring test for the flavor
// driver would catch either, because ZE_LINT_FLAVOR_RUN is not a superstring of
// ZE_LINT_RUN.
func lintPassLines(corpus string, body []string) (passes []int, lastLine int) {
	lastLine = -1
	for index, ln := range body {
		if strings.TrimSpace(ln) != "" {
			lastLine = index
		}
		expanded := expandMakeVars(corpus, ln)
		if strings.Contains(expanded, "golangci-lint") || strings.Contains(expanded, "lint_flavors.py") {
			passes = append(passes, index)
		}
	}
	return passes, lastLine
}

// flavorLintPass reports whether a recipe body runs the build-flavor lint
// driver, and returns the line that does.
//
// The corpus is read so the body's variable references resolve: the recipes
// call $(ZE_LINT_FLAVOR_RUN), which carries the driver and both of its
// ceilings. See expandMakeVars.
func flavorLintPass(corpus string, body []string) (string, bool) {
	for _, ln := range body {
		ln = expandMakeVars(corpus, ln)
		if strings.Contains(ln, "lint_flavors.py") {
			return ln, true
		}
	}
	return "", false
}

// TestLintRunsEveryBuildFlavor guards the full lint's coverage of every build
// the two golangci-lint passes above it cannot analyze.
//
// VALIDATES: `ze-lint` runs scripts/dev/lint_flavors.py, which lints one pass
// for each personality tag, capability tag, GOOS and GOARCH a tracked file
// names, and then fails when a tracked Go file is reached by nothing.
// PREVENTS: the regression this gate was written for. golangci-lint analyzes ONE
// build for each run. On a Linux host 245 tracked own-source Go files were
// reported on by nothing (measured 2026-08-24). 101 of them sit behind a
// non-Linux GOOS, and 46 sit behind ze_installer. That is the initrd's PID 1,
// which carried a `SA5000: assignment to nil map` nobody had ever been told
// about. The rest sit behind ze_distro, ze_appliance, ze_setup, debug, race,
// live and the other tags. Delete this pass and they go silent again with every
// gate still green.
func TestLintRunsEveryBuildFlavor(t *testing.T) {
	corpus := makefileCorpus(t)
	_, body := recipeBody(t, corpus, "ze-lint")
	if _, ok := flavorLintPass(corpus, body); !ok {
		t.Errorf("ze-lint runs no scripts/dev/lint_flavors.py pass: every file behind a "+
			"personality tag, a capability tag or a non-host GOOS is unlinted again. Recipe:\n%s",
			strings.Join(body, "\n"))
	}

	// The package pattern is the other half of the coverage. Four roots left
	// every file under scripts/ unlinted, this test's own file included.
	pkgs := expandMakeVars(corpus, "$(ZE_LINT_PKGS)")
	if strings.TrimSpace(pkgs) != "./..." {
		t.Errorf("ZE_LINT_PKGS is %q, not ./...: a package outside the named roots is unlinted, "+
			"and nothing says so -- golangci-lint is loud only when a package it was POINTED at "+
			"has no buildable file", pkgs)
	}
}

// TestChangedLintRunsEveryBuildFlavor is TestLintRunsEveryBuildFlavor for the
// path most edits actually face.
//
// VALIDATES: `ze-lint-changed` runs the same driver, scoped to the changed
// packages, and that no earlier pass in the chain can have its exit status
// discarded.
// PREVENTS: a new //go:build ze_installer file landing unlinted.
// `ai/rules/commands.md` makes ze-lint-changed the gate to run before you claim
// done, and ze-precommit-verify-changed runs it in place of ze-lint
// (stagesForMode). Full-lint-only coverage would therefore leave exactly the new
// files uncovered.
func TestChangedLintRunsEveryBuildFlavor(t *testing.T) {
	corpus := makefileCorpus(t)
	_, body := recipeBody(t, corpus, "ze-lint-changed")
	line, ok := flavorLintPass(corpus, body)
	if !ok {
		t.Fatalf("ze-lint-changed runs no scripts/dev/lint_flavors.py pass: a new file behind a "+
			"personality tag passes the changed-file gate unlinted. Recipe:\n%s",
			strings.Join(body, "\n"))
	}
	if !strings.Contains(line, "--scope") {
		t.Errorf("ze-lint-changed runs the flavor driver over the whole tree rather than the "+
			"changed packages, which costs the changed-file gate its reason to exist. Line: %q", line)
	}
	// Fail-closed chaining: the recipe is one shell. A `;` anywhere in the chain
	// would make the recipe's status the LAST command's. A real finding in an
	// earlier pass would then exit 0. The flavor driver is judged here too, which
	// a substring test for ZE_LINT_RUN cannot do. ZE_LINT_FLAVOR_RUN is not a
	// superstring of it, so the driver line was walked past.
	passes, lastLine := lintPassLines(corpus, body)
	if len(passes) < 3 {
		t.Fatalf("ze-lint-changed runs %d lint passes, and the recipe has three: two golangci-lint "+
			"passes and the flavor driver. Recipe:\n%s", len(passes), strings.Join(body, "\n"))
	}
	for _, index := range passes[:len(passes)-1] {
		if !strings.HasSuffix(strings.TrimRight(body[index], " \t"), "&& \\") {
			t.Errorf("ze-lint-changed chains a lint pass without `&&`, so its exit status is "+
				"discarded when a later pass is clean. Line: %q", body[index])
		}
	}
	// The last pass ends the recipe. A command after it would take the recipe's
	// exit status, which is the same masking read from the other end.
	if final := passes[len(passes)-1]; final != lastLine {
		t.Errorf("ze-lint-changed runs %q after its last lint pass, so the recipe's exit status "+
			"is that command's rather than the lint's", strings.TrimSpace(body[lastLine]))
	}
}

// TestLintFlavorDriverCarriesTheLinterCeilings pins the flavor driver to the
// same two ceilings every other golangci-lint run in this repository has.
//
// VALIDATES: the driver is invoked with GOMEMLIMIT and with `-j`, both derived
// from the machine exactly as ZE_LINT_RUN derives them.
// PREVENTS: a job taking more than its declared share. ZE_RUN_SLOTS divides the
// box by that share to decide how many jobs run at once
// (plan/spec-shared-machine-job-admission.md). A lint pass that escapes the
// ceiling therefore breaks the slot arithmetic rather than just running hot.
// The driver runs golangci-lint ITSELF, so the Makefile's ZE_LINT_RUN cannot
// carry it.
func TestLintFlavorDriverCarriesTheLinterCeilings(t *testing.T) {
	corpus := makefileCorpus(t)
	_, body := recipeBody(t, corpus, "ze-lint")
	line, ok := flavorLintPass(corpus, body)
	if !ok {
		t.Fatalf("ze-lint runs no flavor driver; TestLintRunsEveryBuildFlavor says why that matters")
	}
	memory := expandMakeVars(corpus, "$(ZE_LINT_MEMLIMIT)")
	workers := expandMakeVars(corpus, "-j $(GO_TEST_PROCS)")
	if !strings.Contains(line, "GOMEMLIMIT="+memory) {
		t.Errorf("the flavor driver runs without GOMEMLIMIT=%s, so its golangci-lint runs have no "+
			"memory ceiling. Line: %q", memory, line)
	}
	if !strings.Contains(line, workers) {
		t.Errorf("the flavor driver runs without %q, so its golangci-lint runs take one worker per "+
			"core instead of their declared share. Line: %q", workers, line)
	}
}

// TestWriteVerifyStatusNamesTheTreeThatWasVerified pins the freshness
// certificate to the evidence behind it.
//
// VALIDATES: tree_hash records the tree the stages READ, and a tree that moved
// while the run was in flight is certified as nothing at all.
// PREVENTS: the certificate outrunning its evidence. computeTreeHash used to be
// called only at the end, so a run whose early stages judged one tree stamped
// whichever tree existed when it finished. verify-status.sh reports FRESH on an
// exact hash match, and commit_helper.py gates on that, so in a shared checkout
// a run spanning another session's edits certified content it never verified.
func TestWriteVerifyStatusNamesTheTreeThatWasVerified(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

	// The tree did not move: the hash the run started with is the answer, and it
	// is a REAL hash of this tree rather than a constant, so a later
	// verify-status.sh comparison can match it.
	root := t.TempDir()
	still := snapshotTree(root)
	if err := writeVerifyStatus(root, 0, "ze-precommit-verify", "", still, now); err != nil {
		t.Fatalf("write status: %v", err)
	}
	got := readTreeHash(t, root)
	if got != still.hash {
		t.Errorf("tree_hash = %q, want the hash the run started with (%q)", got, still.hash)
	}
	if got == treeMovedSentinel {
		t.Error("an unchanged tree must not be certified as moved")
	}

	// The tree moved: no single hash is true for the run, so the certificate
	// must name none. The sentinel cannot equal any tree's hash, which is what
	// makes verify-status.sh report STALE rather than FRESH.
	moved := treeSnapshot{hash: "a-tree-that-is-not-this-one", manifest: still.manifest}
	if err := writeVerifyStatus(root, 0, "ze-precommit-verify", "", moved, now); err != nil {
		t.Fatalf("write status: %v", err)
	}
	got = readTreeHash(t, root)
	if got != treeMovedSentinel {
		t.Errorf("tree_hash = %q, want %q: a run that spanned an edit certifies nothing", got, treeMovedSentinel)
	}
	if got == still.hash {
		t.Error("a moved tree must never be certified with a real hash")
	}
}

// readTreeHash returns the `tree_hash` field of the written status file, which
// is the one field these tests ask about: it carries either the hash of the tree
// the stages read or treeMovedSentinel, and that is the whole-tree verdict.
func readTreeHash(t *testing.T, root string) string {
	t.Helper()
	const key = "tree_hash"
	data, err := os.ReadFile(filepath.Join(root, statusPath))
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, key+"="); ok {
			return rest
		}
	}
	t.Fatalf("no %s field in %q", key, string(data))
	return ""
}

// TestVerifyRunCertifiesOnlyATreeThatHeldStill drives the certificate from
// runVerify, which is the entry point that produces it.
//
// VALIDATES: a stage that changes the tree makes the run certify nothing, and a
// run over a still tree certifies the tree the stages read.
// PREVENTS: the fix regressing to hashing after the stage loop. The direct test
// of writeVerifyStatus cannot see that: every other runVerify test installs a
// fake WriteStatus that ignores the hash, so moving the computeTreeHash call
// back below the loop leaves the whole suite green while the certificate again
// stamps a tree the early stages never read. A guard is driven from its entry
// point or it is not driven (ai/rules/evidence.md).
func TestVerifyRunCertifiesOnlyATreeThatHeldStill(t *testing.T) {
	run := func(t *testing.T, mutate bool) string {
		t.Helper()
		root := t.TempDir()
		// computeTreeHash asks git for HEAD, the diff and the untracked set, and
		// gitOutput returns "" for every error. Outside a repository it therefore
		// hashes the same empty answer whatever the files say, so the tree must be
		// a real one or this test cannot see a change at all.
		// The run writes its own stage logs under tmp/, and computeTreeHash counts
		// untracked files. The real checkout gitignores tmp/, so those artifacts
		// never move the hash; without the same ignore here every run would
		// certify itself as moved. That dependency is worth stating: if tmp/ ever
		// left .gitignore, every verify would refuse to certify its own tree.
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("tmp/\n"), 0o600); err != nil {
			t.Fatalf("gitignore: %v", err)
		}
		for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
			cmd := exec.CommandContext(context.Background(), "git", args...)
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				// Never skip. computeTreeHash IS a git question, so a tree this
				// test cannot build is a broken environment rather than a reason
				// to stop asserting: a skipped guard reads exactly like a passing
				// one, and this file's subject is a certificate that over-claimed.
				t.Fatalf("git unavailable, so the tree-hash guard cannot be driven: %v: %s", err, out)
			}
		}
		if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("seed\n"), 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		var written string
		_, err := runVerify(context.Background(), verifyConfig{
			Root:   root,
			Mode:   "ze-precommit-verify",
			Stages: []stage{{Name: "only", Rerun: "make only"}},
			Now:    fixedNow,
			Out:    io.Discard,
			RunStage: func(_ context.Context, _ string, _ stage, w io.Writer) int {
				_, _ = io.WriteString(w, "ran\n")
				if mutate {
					// A concurrent session editing the checkout mid-run, which is
					// the case the certificate used to mis-certify.
					_ = os.WriteFile(filepath.Join(root, "seed.txt"), []byte("edited\n"), 0o600)
				}
				return 0
			},
			WriteStatus: func(_ string, _ int, _, _ string, start treeSnapshot, _ time.Time) error {
				// The real writer's decision, reproduced here so the assertion is
				// about what runVerify HANDS the writer rather than about the file.
				written = start.hash
				if end := computeTreeHash(root); end != start.hash {
					written = treeMovedSentinel
				}
				return nil
			},
			SelectScope: testScopeSelector,
		})
		if err != nil {
			t.Fatalf("run verify: %v", err)
		}
		return written
	}

	if got := run(t, true); got != treeMovedSentinel {
		t.Errorf("a stage changed the tree and the run certified %q, want %q: the hash was "+
			"taken after the stages rather than before them", got, treeMovedSentinel)
	}
	still := run(t, false)
	if still == treeMovedSentinel {
		t.Errorf("a tree that held still certified %q, want its real hash", still)
	}
	if still == "" {
		t.Error("runVerify handed the writer no start hash at all")
	}
}

// verifyRepoRoot builds a throwaway git repository whose working tree carries
// two files that already differ from HEAD, and returns its root.
//
// A real repository is required: gitOutput returns "" for every git error, so
// outside one the per-path record is empty whatever the files say and no test
// could see a path move. tmp/ is gitignored exactly as the checkout gitignores
// it, so the run's own artifacts never read as a concurrent edit.
func verifyRepoRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write(".gitignore", "tmp/\n")
	write("mine.txt", "mine\n")
	write("theirs.txt", "theirs\n")
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-q", "-m", "fixture"},
	} {
		cmd := exec.CommandContext(context.Background(), "git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			// Never skip. The record under test IS a git question, so a tree
			// this test cannot build is a broken environment rather than a
			// reason to stop asserting: a skipped guard reads exactly like a
			// passing one (ai/rules/testing.md).
			t.Fatalf("git %v failed, so the freshness record cannot be driven: %v: %s", args, err, out)
		}
	}
	// Both files differ from HEAD before the run starts, so both carry a row in
	// the per-path record and the assertions can tell which one moved.
	write("mine.txt", "mine edited\n")
	write("theirs.txt", "theirs edited\n")
	return root
}

// verifyStatusCheck asks scripts/dev/verify-status.sh about root and returns its
// exit code with its output. That script is the only consumer that parses the
// record, so the assertions read the record through it rather than through its
// bytes.
func verifyStatusCheck(t *testing.T, root string, paths ...string) (int, string) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "dev", "verify-status.sh"))
	if err != nil {
		t.Fatalf("locate verify-status.sh: %v", err)
	}
	cmd := exec.CommandContext(context.Background(), "bash", append([]string{script, "check"}, paths...)...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), string(out)
	}
	t.Fatalf("run verify-status.sh check %v: %v: %s", paths, err, out)
	return 0, ""
}

// TestWriteVerifyStatusRecordsMovedPathsNotSentinel drives the freshness record
// from runVerify, the entry point that produces it, and reads it back through
// verify-status.sh, the only consumer that parses it.
//
// VALIDATES: a concurrent edit during a run costs the run the paths that moved
// and nothing else. The record NAMES them, and every path that held still still
// answers FRESH.
// PREVENTS: one foreign edit voiding a whole run. writeVerifyStatus wrote
// treeMovedSentinel into tree_hash and nothing else, and no tree hashes to that
// value, so the entire record read STALE for ever -- including for the asking
// session's own files, which nobody had touched. Six sessions share this
// checkout and it carries hundreds of uncommitted files, so a 25-minute run
// essentially never sees a still tree: the live tmp/ze-verify.status held that
// sentinel after a 79-minute PASS.
func TestWriteVerifyStatusRecordsMovedPathsNotSentinel(t *testing.T) {
	// A partial pass reports STALE whatever the paths say, so the ambient
	// setting must not decide what this test measures.
	t.Setenv("ZE_SKIP_SUITES", "")

	run := func(t *testing.T, moved ...string) string {
		t.Helper()
		root := verifyRepoRoot(t)
		code, err := runVerify(context.Background(), verifyConfig{
			Root:   root,
			Mode:   "ze-precommit-verify",
			Stages: []stage{{Name: "only", Rerun: "make only"}},
			Now:    fixedNow,
			Out:    io.Discard,
			RunStage: func(_ context.Context, _ string, _ stage, w io.Writer) int {
				_, _ = io.WriteString(w, "ran\n")
				for _, rel := range moved {
					// Another session editing the shared checkout mid-run.
					_ = os.WriteFile(filepath.Join(root, rel), []byte("moved by another session\n"), 0o600)
				}
				return 0
			},
			SelectScope: testScopeSelector,
		})
		if err != nil {
			t.Fatalf("run verify: %v", err)
		}
		if code != 0 {
			t.Fatalf("verify exit = %d, want 0: only a PASS is worth asking about", code)
		}
		return root
	}

	t.Run("nothing moved", func(t *testing.T) {
		root := run(t)
		for _, rel := range []string{"mine.txt", "theirs.txt"} {
			if code, out := verifyStatusCheck(t, root, rel); code != 0 {
				t.Errorf("check %s exited %d (%s), want FRESH: the tree held still for the whole run",
					rel, code, strings.TrimSpace(out))
			}
		}
		if code, out := verifyStatusCheck(t, root); code != 0 {
			t.Errorf("whole-tree check exited %d (%s), want FRESH", code, strings.TrimSpace(out))
		}
		if data := readManifest(t, root); strings.Contains(data, movedDuringRun) {
			t.Errorf("a still tree recorded a moved path:\n%s", data)
		}
	})

	t.Run("a path the asker did not name moved", func(t *testing.T) {
		root := run(t, "theirs.txt")
		if code, out := verifyStatusCheck(t, root, "mine.txt"); code != 0 {
			t.Errorf("check mine.txt exited %d (%s), want FRESH: another session's edit to theirs.txt "+
				"must not void the verdict about a file it never touched", code, strings.TrimSpace(out))
		}
		if got := readManifest(t, root); !strings.Contains(got, movedDuringRun+" theirs.txt") {
			t.Errorf("the record does not name the path that moved:\n%s", got)
		}
	})

	t.Run("the path the asker named moved", func(t *testing.T) {
		root := run(t, "theirs.txt")
		if code, out := verifyStatusCheck(t, root, "theirs.txt"); code == 0 {
			t.Errorf("check theirs.txt reported FRESH (%s), want STALE: a path that moved while the "+
				"stages ran was verified at neither content", strings.TrimSpace(out))
		}
		// Granularity, not leniency. The whole-tree question still has no true
		// answer, so it keeps the sentinel and keeps reporting STALE.
		if got := readTreeHash(t, root); got != treeMovedSentinel {
			t.Errorf("tree_hash = %q, want %q: no single hash is true for a run that spanned an edit", got, treeMovedSentinel)
		}
		if code, out := verifyStatusCheck(t, root); code == 0 {
			t.Errorf("whole-tree check reported FRESH (%s) after the tree moved", strings.TrimSpace(out))
		}
		// The record charges the move to the path that moved, and leaves every
		// other path a fingerprint a later check can match.
		got := readManifest(t, root)
		if strings.Contains(got, movedDuringRun+" mine.txt") {
			t.Errorf("a path that held still was recorded as moved:\n%s", got)
		}
		if strings.Contains(got, treeMovedSentinel) {
			t.Errorf("the per-path record carries the whole-run sentinel, so it voids every path:\n%s", got)
		}
	})

	// The record is a comparison of two snapshots, so an edit that begins AND
	// ends inside the run window is invisible to it. The window has two shapes
	// and this subtest drives the second: theirs.txt is already dirty at run
	// start, is written during the stages, and is put back before the end
	// snapshot, so both snapshots hold the same fingerprint and the path answers
	// FRESH though some stages read content nobody verified. The whole-tree hash
	// answers the same way for the same reason.
	//
	// This pins a documented limitation, not a requirement
	// (docs/architecture/testing/verify-freshness-scope.md, "A path that moved
	// while the run was in flight"). Closing it needs a third observation of the
	// tree, which no acceptance criterion of this spec asks for. A failure here
	// means somebody closed it: update the doc paragraph and this expectation
	// together, and do not restore the blind spot to keep the test green.
	t.Run("an edit reverted before the run ended is invisible", func(t *testing.T) {
		root := verifyRepoRoot(t)
		theirs := filepath.Join(root, "theirs.txt")
		start, err := os.ReadFile(theirs)
		if err != nil {
			t.Fatalf("read the fixture the stages start from: %v", err)
		}
		code, err := runVerify(context.Background(), verifyConfig{
			Root:   root,
			Mode:   "ze-precommit-verify",
			Stages: []stage{{Name: "only", Rerun: "make only"}},
			Now:    fixedNow,
			Out:    io.Discard,
			RunStage: func(_ context.Context, _ string, _ stage, w io.Writer) int {
				_, _ = io.WriteString(w, "ran\n")
				_ = os.WriteFile(theirs, []byte("edited mid-run\n"), 0o600)
				_ = os.WriteFile(theirs, start, 0o600)
				return 0
			},
			SelectScope: testScopeSelector,
		})
		if err != nil {
			t.Fatalf("run verify: %v", err)
		}
		if code != 0 {
			t.Fatalf("verify exit = %d, want 0: only a PASS is worth asking about", code)
		}
		if code, out := verifyStatusCheck(t, root, "theirs.txt"); code != 0 {
			t.Errorf("check theirs.txt exited %d (%s), want FRESH: the two snapshots agree, "+
				"which is the blind spot the doc names", code, strings.TrimSpace(out))
		}
		if got := readManifest(t, root); strings.Contains(got, movedDuringRun) {
			t.Errorf("a path the two snapshots agreed on was recorded as moved:\n%s", got)
		}
		if got := readTreeHash(t, root); got == treeMovedSentinel {
			t.Errorf("tree_hash = %q: the whole-tree hash has the same blind spot and must "+
				"answer the same way, or the two granularities disagree about one run", got)
		}
	})
}

// readManifest returns the per-path record written beside the status file.
func readManifest(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, manifestPath))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	return string(data)
}

// testScopeSelector is the injected change-set selector for every runVerify
// test that is not about the change set. It answers without running the real
// selector, which would cost a `go list` over the whole tree per test and would
// make every one of them depend on what the working tree happens to hold.
func testScopeSelector(string, io.Writer) (changeSetAnswer, error) {
	return changeSetAnswer{Packages: []string{"./scripts/status"}, Tags: []string{"ze_ssh"}}, nil
}

// VALIDATES: the change set is selected ONCE per run, published in the run's own
// artifact directory, and named to every stage through ZE_VERIFY_SCOPE_PACKAGES.
// PREVENTS: the per-consumer cost this phase exists to remove -- every scoped
// stage paying for its own reverse-import walk -- and the cross-session
// collision a shared tmp/ name would carry, since two sessions verify this
// checkout at once.
func TestVerifyRunSelectsTheChangeSetOncePerRun(t *testing.T) {
	root := t.TempDir()
	answer := []string{"./internal/core/family", "./internal/component/bgp"}

	selections := 0
	var named []string
	code, err := runVerify(context.Background(), verifyConfig{
		Root:   root,
		Mode:   "ze-precommit-verify-changed",
		Stages: []stage{{Name: "lint", Rerun: "make lint"}, {Name: "unit", Rerun: "make unit"}},
		Now:    fixedNow,
		Out:    io.Discard,
		RunStage: func(_ context.Context, _ string, st stage, w io.Writer) int {
			_, _ = io.WriteString(w, "ran\n")
			for _, entry := range st.Env {
				if value, ok := strings.CutPrefix(entry, scopePackagesEnv+"="); ok {
					named = append(named, value)
				}
			}
			return 0
		},
		WriteStatus: testStatusWriter,
		SelectScope: func(string, io.Writer) (changeSetAnswer, error) {
			selections++
			return changeSetAnswer{Packages: answer, Tags: []string{"ze_ssh"}}, nil
		},
	})
	if err != nil {
		t.Fatalf("run verify: %v", err)
	}
	if code != 0 {
		t.Fatalf("verify exit = %d, want 0", code)
	}

	if selections != 1 {
		t.Fatalf("the selector ran %d times for a 2-stage run, want 1: the whole point of publishing the answer is that a run pays for it once", selections)
	}
	if len(named) != 2 {
		t.Fatalf("%d of 2 stages were told where the change set is: %v", len(named), named)
	}
	if named[0] != named[1] {
		t.Fatalf("two stages of one run were given different change sets:\n  %s\n  %s", named[0], named[1])
	}

	runDir := filepath.Join(root, filepath.FromSlash(stageLogDir), runDirNames(t, root)[0])
	want := filepath.Join(runDir, scopePackagesFile)
	if named[0] != want {
		t.Fatalf("stages read %s, want the run's own %s", named[0], want)
	}
	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read the published change set: %v", err)
	}
	if got := strings.TrimSpace(string(body)); got != strings.Join(answer, "\n") {
		t.Fatalf("published change set is %q, want %q", got, strings.Join(answer, "\n"))
	}
}

// VALIDATES: two runs of one checkout publish their change sets at different
// paths.
// PREVENTS: the collision that a documented tmp/ name would carry -- a second
// session's run rewriting the answer between two stages of the first, so a
// scoped stage lints a tree nobody selected for it.
func TestVerifyRunPublishesTheChangeSetPerRun(t *testing.T) {
	root := t.TempDir()

	published := func() string {
		var named string
		if _, err := runVerify(context.Background(), verifyConfig{
			Root:   root,
			Mode:   "ze-precommit-verify-changed",
			Stages: []stage{{Name: "lint", Rerun: "make lint"}},
			Now:    fixedNow,
			Out:    io.Discard,
			RunStage: func(_ context.Context, _ string, st stage, w io.Writer) int {
				_, _ = io.WriteString(w, "ran\n")
				for _, entry := range st.Env {
					if value, ok := strings.CutPrefix(entry, scopePackagesEnv+"="); ok {
						named = value
					}
				}
				return 0
			},
			WriteStatus: testStatusWriter,
			SelectScope: testScopeSelector,
		}); err != nil {
			t.Fatalf("run verify: %v", err)
		}
		return named
	}

	first, second := published(), published()
	if first == "" || second == "" {
		t.Fatalf("a run did not name its change set: %q, %q", first, second)
	}
	if first == second {
		t.Fatalf("both runs published their change set at %s, so the second overwrote what the first stages read", first)
	}
}

// VALIDATES: a selector that cannot answer widens the run to every package.
// PREVENTS: the fail-open turning into a fail-closed. An unanswered selection
// reaching a stage as an EMPTY package list is read by _ze-lint-changed-impl as
// "No changed Go packages to lint", so the stage would report success having
// linted nothing at all (ai/rules/evidence.md).
func TestVerifyRunWidensWhenTheChangeSetCannotBeSelected(t *testing.T) {
	root := t.TempDir()

	var named, namedTags string
	code, err := runVerify(context.Background(), verifyConfig{
		Root:   root,
		Mode:   "ze-precommit-verify-changed",
		Stages: []stage{{Name: "lint", Rerun: "make lint"}},
		Now:    fixedNow,
		Out:    io.Discard,
		RunStage: func(_ context.Context, _ string, st stage, w io.Writer) int {
			_, _ = io.WriteString(w, "ran\n")
			for _, entry := range st.Env {
				if value, ok := strings.CutPrefix(entry, scopePackagesEnv+"="); ok {
					named = value
				}
				if strings.HasPrefix(entry, scopeTagsEnv+"=") {
					namedTags = entry
				}
			}
			return 0
		},
		WriteStatus: testStatusWriter,
		SelectScope: func(string, io.Writer) (changeSetAnswer, error) {
			return changeSetAnswer{}, errors.New("go list: the toolchain is wedged")
		},
	})
	if err != nil {
		t.Fatalf("run verify: %v", err)
	}
	if code != 0 {
		t.Fatalf("verify exit = %d, want 0: a widened change set is an answer, not a failure", code)
	}
	if named == "" {
		t.Fatal("the stage was told nothing about the change set, so it would select its own")
	}
	body, err := os.ReadFile(named)
	if err != nil {
		t.Fatalf("read the published change set: %v", err)
	}
	if got := strings.TrimSpace(string(body)); got != everyPackageWord {
		t.Fatalf("published change set is %q, want %q: a failed selection must widen, never empty", got, everyPackageWord)
	}
	// The tag half widens by being ABSENT. An empty feature-tag answer is a real
	// answer -- no Go package changed, so only the two shipped combinations can
	// move -- so publishing one here would scope the staticcheck matrix to 2 rows
	// on a run whose change set is unknown.
	if namedTags != "" {
		t.Fatalf("a failed selection published a feature-tag answer (%s), which the matrix would read as a narrow change set", namedTags)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(named), scopeTagsFile)); !os.IsNotExist(err) {
		t.Fatalf("a failed selection wrote %s (stat error %v), so a later stage could still narrow on it", scopeTagsFile, err)
	}
}

// VALIDATES: scripts/dev/changed-pkgs.sh, the one thing the make recipes call,
// answers from the file the run published.
// PREVENTS: the two answers drifting. The script holds no selection logic of its
// own since this phase, so this is the whole consumer contract:
// _ze-lint-changed-impl and _ze-unit-test-changed-impl expand what it prints.
func TestChangedPkgsReadsThePublishedChangeSet(t *testing.T) {
	script := filepath.Join("..", "..", "scripts", "dev", "changed-pkgs.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("locate changed-pkgs.sh: %v", err)
	}

	run := func(t *testing.T, answerPath string) (string, string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "bash", script) //nolint:gosec // fixed repository script
		cmd.Env = append(os.Environ(), scopePackagesEnv+"="+answerPath)
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("changed-pkgs.sh failed: %v\nstderr:\n%s", err, stderr.String())
		}
		return stdout.String(), stderr.String()
	}

	t.Run("the published answer is what the recipes get", func(t *testing.T) {
		answer := filepath.Join(t.TempDir(), scopePackagesFile)
		if err := os.WriteFile(answer, []byte("./internal/core/family\n./scripts/status\n"), 0o600); err != nil {
			t.Fatalf("write the change set: %v", err)
		}
		stdout, _ := run(t, answer)
		if stdout != "./internal/core/family\n./scripts/status\n" {
			t.Fatalf("changed-pkgs.sh printed %q, not what the run published", stdout)
		}
	})

	t.Run("a widened answer stays widened", func(t *testing.T) {
		answer := filepath.Join(t.TempDir(), scopePackagesFile)
		if err := os.WriteFile(answer, []byte(everyPackageWord+"\n"), 0o600); err != nil {
			t.Fatalf("write the change set: %v", err)
		}
		stdout, _ := run(t, answer)
		if strings.TrimSpace(stdout) != everyPackageWord {
			t.Fatalf("changed-pkgs.sh printed %q for a widened run, want %q", stdout, everyPackageWord)
		}
	})

	t.Run("an unreadable answer widens rather than emptying", func(t *testing.T) {
		stdout, stderr := run(t, filepath.Join(t.TempDir(), "never-written.txt"))
		if strings.TrimSpace(stdout) != everyPackageWord {
			t.Fatalf("changed-pkgs.sh printed %q when the published answer was missing, want %q: an empty answer reads as nothing to verify", stdout, everyPackageWord)
		}
		if !strings.Contains(stderr, "never-written.txt") {
			t.Fatalf("the widening was silent about the path that caused it:\n%s", stderr)
		}
	})

	t.Run("an empty answer is an answer", func(t *testing.T) {
		answer := filepath.Join(t.TempDir(), scopePackagesFile)
		if err := os.WriteFile(answer, nil, 0o600); err != nil {
			t.Fatalf("write the change set: %v", err)
		}
		stdout, _ := run(t, answer)
		if stdout != "" {
			t.Fatalf("changed-pkgs.sh printed %q for an empty change set, want nothing: no changed path is compiled or read by a Go package", stdout)
		}
	})
}

// VALIDATES: the production selector call itself, which every other test in this
// file replaces with testScopeSelector.
// PREVENTS: a change set that is only ever answered by a stub. The wiring above
// proves the answer reaches the stages; this proves there is an answer -- the
// selector runs from the runner's working directory and writes package patterns
// the make recipes can expand.
func TestSelectScopePackagesRunsTheRealSelector(t *testing.T) {
	root := filepath.Join("..", "..")
	answer, err := selectChangeSet(root, io.Discard)
	if err != nil {
		t.Fatalf("select the change set: %v", err)
	}
	if len(answer.Packages) == 0 {
		t.Fatal("the selector answered nothing at all: an empty answer reads as nothing to lint or test")
	}
	for _, pkg := range answer.Packages {
		if !strings.HasPrefix(pkg, "./") {
			t.Fatalf("the selector answered %q, which no make recipe can expand as a package pattern", pkg)
		}
	}
	// The tag half comes from the same run. It is legitimately EMPTY for a
	// change no gated package compiles, so the assertion is on the shape of what
	// is there, not on there being something.
	for _, tag := range answer.Tags {
		if !strings.HasPrefix(tag, "ze_") {
			t.Fatalf("the selector answered feature tag %q, which feature-gates.txt cannot declare", tag)
		}
	}
}

// VALIDATES: the feature-tag half of the change set is published in the run's
// own directory and named to every stage through ZE_VERIFY_SCOPE_TAGS, from the
// SAME selector run that answered the packages.
// PREVENTS: the staticcheck matrix paying 874s to judge 38 rows for a change
// that can move 3 of them, and the second graph walk a second selector call
// would cost (plan/spec-verify-scope-3-selector-consumers.md).
func TestVerifyRunNamesTheFeatureScopeToEveryStage(t *testing.T) {
	root := t.TempDir()
	tags := []string{"ze_ssh", "ze_web"}

	selections := 0
	var named []string
	code, err := runVerify(context.Background(), verifyConfig{
		Root:   root,
		Mode:   "ze-precommit-verify-changed",
		Stages: []stage{{Name: "matrix", Rerun: "make matrix"}, {Name: "lint", Rerun: "make lint"}},
		Now:    fixedNow,
		Out:    io.Discard,
		RunStage: func(_ context.Context, _ string, st stage, w io.Writer) int {
			_, _ = io.WriteString(w, "ran\n")
			for _, entry := range st.Env {
				if value, ok := strings.CutPrefix(entry, scopeTagsEnv+"="); ok {
					named = append(named, value)
				}
			}
			return 0
		},
		WriteStatus: testStatusWriter,
		SelectScope: func(string, io.Writer) (changeSetAnswer, error) {
			selections++
			return changeSetAnswer{Packages: []string{"./internal/component/ssh"}, Tags: tags}, nil
		},
	})
	if err != nil {
		t.Fatalf("run verify: %v", err)
	}
	if code != 0 {
		t.Fatalf("verify exit = %d, want 0", code)
	}
	if selections != 1 {
		t.Fatalf("the selector ran %d times for a 2-stage run, want 1: both answers come from one run", selections)
	}
	if len(named) != 2 || named[0] != named[1] {
		t.Fatalf("%d of 2 stages were told where the feature scope is, and they must agree: %v", len(named), named)
	}

	runDir := filepath.Join(root, filepath.FromSlash(stageLogDir), runDirNames(t, root)[0])
	want := filepath.Join(runDir, scopeTagsFile)
	if named[0] != want {
		t.Fatalf("stages read %s, want the run's own %s", named[0], want)
	}
	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read the published feature scope: %v", err)
	}
	if got := strings.TrimSpace(string(body)); got != strings.Join(tags, "\n") {
		t.Fatalf("published feature scope is %q, want %q", got, strings.Join(tags, "\n"))
	}
}

// VALIDATES: the --print=both document is read as two answers, and a truncated
// one is an error rather than an empty half.
// PREVENTS: a half-read answer narrowing the run. An empty tag list is a REAL
// answer -- judge the two shipped matrix rows and nothing else -- so a missing
// "# tags" section must never arrive as one.
func TestParseChangeSetAnswerReadsBothSections(t *testing.T) {
	for _, tc := range []struct {
		name         string
		out          string
		wantPackages []string
		wantTags     []string
		wantErr      string
	}{
		{
			name:         "both halves",
			out:          "# packages\n./a\n./b\n# tags\nze_ssh\n",
			wantPackages: []string{"./a", "./b"},
			wantTags:     []string{"ze_ssh"},
		},
		{
			name: "both halves empty",
			out:  "# packages\n# tags\n",
		},
		{
			name:         "no feature is reachable",
			out:          "# packages\n./docs\n# tags\n",
			wantPackages: []string{"./docs"},
		},
		{
			name:    "the tag section is missing",
			out:     "# packages\n./a\n",
			wantErr: "1 of the 2 sections",
		},
		{
			name:    "an answer before any section",
			out:     "./a\n# packages\n./b\n# tags\n",
			wantErr: "before naming a section",
		},
		{
			name:    "an unknown section",
			out:     "# packages\n./a\n# suites\nweb\n# tags\n",
			wantErr: "unknown section",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answer, err := parseChangeSetAnswer(tc.out)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse the answer: %v", err)
			}
			if strings.Join(answer.Packages, ",") != strings.Join(tc.wantPackages, ",") {
				t.Fatalf("packages = %v, want %v", answer.Packages, tc.wantPackages)
			}
			if strings.Join(answer.Tags, ",") != strings.Join(tc.wantTags, ",") {
				t.Fatalf("tags = %v, want %v", answer.Tags, tc.wantTags)
			}
		})
	}
}

// VALIDATES: the one consumer of ZE_VERIFY_SCOPE_TAGS spells it exactly as this
// runner sets it.
// PREVENTS: the silent half of a rename. A drift here fails OPEN -- the matrix
// sees no answer and judges all 38 rows -- so no gate goes red, no test fails,
// and the 874s this spec exists to remove comes back unannounced.
func TestTheStaticcheckMatrixReadsTheFeatureScopeVariable(t *testing.T) {
	const consumer = "../checks/staticcheck_feature_matrix.go"
	source, err := os.ReadFile(consumer)
	if err != nil {
		t.Fatalf("read %s: %v", consumer, err)
	}
	body := string(source)
	if want := "scopeTagsEnv = \"" + scopeTagsEnv + "\""; !strings.Contains(body, want) {
		t.Fatalf("%s does not declare %s, so this runner publishes an answer nothing reads", consumer, want)
	}
	if !strings.Contains(body, "os.Getenv(scopeTagsEnv)") {
		t.Fatalf("%s declares the variable but never reads it, so every run judges every row", consumer)
	}
}

// declaredLine builds one VERIFY FAILURE GROUP: line the way a producer does,
// through encoding/json, so the tests exercise the escaping a real producer gets
// rather than a hand-written string the author already knows is safe.
func declaredLine(t *testing.T, g failureGroup) string {
	t.Helper()
	payload, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal declared group: %v", err)
	}
	return declaredGroupPrefix + " " + string(payload) + "\n"
}

// completeLine is the producer's statement that it declared a group for every
// failure it reported, carrying the number it printed.
func completeLine(count int) string {
	return declaredCompletePrefix + " " + strconv.Itoa(count) + "\n"
}

// TestMalformedGroupLineIsReported
//
// VALIDATES: a VERIFY FAILURE GROUP: line whose JSON does not parse becomes a
// group of its own, and that group names no path the checkout holds, so the
// commit helper charges the gate instead of reading the red as somebody else's.
// PREVENTS: the failure vanishing. The parser used to `continue` past an
// unmarshal error, which deleted the failure the producer printed the line for:
// the stage stayed red and the failure index held nothing about it (spec R-2).
func TestMalformedGroupLineIsReported(t *testing.T) {
	text := declaredGroupPrefix + " {\"stage\":\"ze-doc-wiring-check\",\n" +
		completeLine(1)
	groups, complete := parseDeclaredGroups("tmp/verify/wiring.log", text)
	if len(groups) != 1 {
		t.Fatalf("expected the unparseable line to become one group, got %+v", groups)
	}
	if !complete {
		t.Fatalf("an unparseable line still counts toward the declared total, got complete=false")
	}
	if groups[0].Kind != "unparsed" {
		t.Fatalf("kind = %q, want unparsed", groups[0].Kind)
	}
	if !strings.Contains(groups[0].Summary, "did not parse") {
		t.Fatalf("summary does not say the line failed to parse: %q", groups[0].Summary)
	}
	if len(groups[0].Excerpt) != 1 || !strings.Contains(groups[0].Excerpt[0], "ze-doc-wiring-check") {
		t.Fatalf("the unreadable line is not quoted in the excerpt: %+v", groups[0].Excerpt)
	}
	if _, err := os.Stat(filepath.Join(repoRootFromScriptsStatus(), groups[0].Related[0])); err == nil {
		t.Fatalf("related %q names a real path, so the commit helper could read it as attribution", groups[0].Related[0])
	}
}

// TestClassifyWiringDocsPrefersDeclaredGroups
//
// VALIDATES: when the wiring gate declares a group for every failure it reported
// and states the count, classifyStage records those groups and not the ones its
// prose regexes would have built.
// PREVENTS: the 71 unattributable debt rows this spec exists to remove. The
// prose path records a CHECK NAME in Related, which resolves to no file, so
// structural_gate_reds charges the gate to whoever happened to be committing.
func TestClassifyWiringDocsPrefersDeclaredGroups(t *testing.T) {
	declared := failureGroup{
		Stage:   "ze-doc-wiring-check",
		GroupID: "files:wiring",
		Kind:    "files",
		Related: []string{"internal/component/ssh/server.go"},
		Summary: "wiring check failed",
		Rerun:   "make ze-doc-wiring-check",
	}
	text := "Running ze-doc-verify...\n" +
		"Wiring check FAILED:\n" +
		declaredLine(t, declared) +
		"ze-doc-verify failed\n" +
		completeLine(1)

	groups := classifyStage(stage{Name: "ze-doc-wiring-check"}, "tmp/verify/wiring.log", text)
	if len(groups) != 1 {
		t.Fatalf("expected only the declared group, got %+v", groups)
	}
	if groups[0].GroupID != "files:wiring" || groups[0].Kind != "files" {
		t.Fatalf("unexpected group: %+v", groups[0])
	}
	if want := []string{"internal/component/ssh/server.go"}; !slices.Equal(groups[0].Related, want) {
		t.Fatalf("related = %v, want %v", groups[0].Related, want)
	}
	if groups[0].DetailLog != "tmp/verify/wiring.log" {
		t.Fatalf("detail log = %q, want the stage's own log", groups[0].DetailLog)
	}
}

// TestClassifyWiringDocsFallsBackToProse
//
// VALIDATES: declared groups are used all-or-nothing. Anything short of "one
// group for every failure I reported, and here is the count" sends the stage
// back to its prose classifier.
// PREVENTS: a silent drop. classifyStage replaces its groups with genericGroup
// only when the slice is EMPTY, so a partial capture would fill the slice and
// take the failures it missed out of the index with it (spec R-1).
func TestClassifyWiringDocsFallsBackToProse(t *testing.T) {
	one := failureGroup{Stage: "ze-doc-wiring-check", GroupID: "files:wiring", Kind: "files", Related: []string{"internal/component/ssh/server.go"}}
	prose := "Running ze-doc-verify...\nze-doc-verify failed\n"

	cases := []struct {
		name string
		text string
	}{
		{"no count at all", prose + declaredLine(t, one)},
		{"count higher than the groups declared", prose + declaredLine(t, one) + completeLine(2)},
		{"count lower than the groups declared", prose + declaredLine(t, one) + declaredLine(t, one) + completeLine(1)},
		{"two counts", prose + declaredLine(t, one) + completeLine(1) + completeLine(1)},
		{"an unreadable count", prose + declaredLine(t, one) + declaredCompletePrefix + " many\n"},
		{"nothing declared", prose},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups := classifyStage(stage{Name: "ze-doc-wiring-check"}, "tmp/verify/wiring.log", tc.text)
			if len(groups) != 1 {
				t.Fatalf("expected the one prose group, got %+v", groups)
			}
			if groups[0].GroupID != "subcheck:ze-doc-verify" {
				t.Fatalf("group = %+v, want the prose subcheck group", groups[0])
			}
		})
	}
}

// TestAStageWithNoClassifierCanDeclareItsOwnGroups
//
// VALIDATES: the declared-group protocol is read for EVERY stage, not only for
// the six classifyStage dispatches on.
// PREVENTS: the mechanism reaching one gate. 24 of the 30 stages have no
// classifier, so they fall through to genericGroup, whose Kind is "stage";
// PATH_BEARING_GROUP_KINDS (scripts/dev/commit_helper.py) does not hold "stage"
// and group_related_paths returns nothing for it before any path lookup, so
// those stages cannot be attributed whatever they print.
func TestAStageWithNoClassifierCanDeclareItsOwnGroups(t *testing.T) {
	st := stage{Name: "ze-generated-files-check", Rerun: "make ze-generated-files-check"}
	declared := failureGroup{
		GroupID: "files:generated",
		Kind:    "files",
		Related: []string{"internal/component/plugin/all/all.go"},
		Summary: "a generated file is out of date",
	}
	text := "some prose the runner has no classifier for\n" + declaredLine(t, declared) + completeLine(1)

	groups := classifyStage(st, "tmp/verify/generated.log", text)
	if len(groups) != 1 || groups[0].GroupID != "files:generated" {
		t.Fatalf("expected the declared group, got %+v", groups)
	}
	if groups[0].Stage != st.Name {
		t.Fatalf("stage = %q, want %q", groups[0].Stage, st.Name)
	}
	if groups[0].Rerun != st.Rerun {
		t.Fatalf("rerun = %q, want the stage's own %q", groups[0].Rerun, st.Rerun)
	}

	// Without the declaration the same stage still falls through to genericGroup,
	// which is what makes the line above the whole difference.
	fallback := classifyStage(st, "tmp/verify/generated.log", "some prose the runner has no classifier for\n")
	if fallback[0].Kind != "stage" {
		t.Fatalf("undeclared fallback kind = %q, want stage", fallback[0].Kind)
	}
}

// TestATruncatedStageLogBecomesItsOwnGroup
//
// VALIDATES: a stage whose log could not be read to the end gets a failure group
// saying so, of a kind the commit helper charges, whether the stage declared its
// own groups or was classified.
// PREVENTS: the truncation marker reaching no reader at all. splitLines appends
// logTruncatedMarker LAST; excerptFromText keeps only the first maxExcerptLines+1
// non-empty lines and is the only producer of Excerpt; and a log holding a line
// over maxLogLineBytes is by definition longer than that cap, so the marker was
// always past it. No classifier's regex matched the marker either, so the read
// that ended early entered no group and the failure index reported the prefix
// the scanner happened to reach as if it were the whole log
// (ai/rules/evidence.md: a partial answer must not look like a whole one).
func TestATruncatedStageLogBecomesItsOwnGroup(t *testing.T) {
	overLong := strings.Repeat("x", maxLogLineBytes+1)

	t.Run("a classified stage is charged for what it could not read", func(t *testing.T) {
		st := stage{Name: "ze-doc-wiring-check", Rerun: "make ze-doc-wiring-check"}
		text := "Running ze-doc-verify...\nze-doc-verify failed\n" + overLong + "\n"
		groups := classifyStage(st, "tmp/verify/wiring.log", text)

		truncated := groupByID(groups, "subcheck:stage-log-truncated")
		if truncated == nil {
			t.Fatalf("no group carries the truncated read, so nothing reports it: %+v", groups)
		}
		if truncated.Kind != "subcheck" {
			t.Fatalf("kind = %q, want subcheck -- PATH_BEARING_GROUP_KINDS (scripts/dev/commit_helper.py) must not hold it, so the red is charged", truncated.Kind)
		}
		if !strings.HasPrefix(truncated.Summary, logTruncatedMarker) {
			t.Fatalf("summary = %q, want the marker, which names the scanner error", truncated.Summary)
		}
		if len(groups) < 2 {
			t.Fatalf("the classifier's own group was replaced rather than added to: %+v", groups)
		}
	})

	t.Run("a declaring stage is charged too", func(t *testing.T) {
		// The count can still agree when the log is cut AFTER it, and the tail
		// is unclassified all the same.
		st := stage{Name: "ze-generated-files-check", Rerun: "make ze-generated-files-check"}
		declared := failureGroup{GroupID: "files:generated", Kind: "files", Related: []string{"internal/component/plugin/all/all.go"}}
		text := declaredLine(t, declared) + completeLine(1) + overLong + "\n"

		groups := classifyStage(st, "tmp/verify/generated.log", text)
		if groupByID(groups, "files:generated") == nil {
			t.Fatalf("the declared group was lost: %+v", groups)
		}
		if groupByID(groups, "subcheck:stage-log-truncated") == nil {
			t.Fatalf("the truncated read is not reported beside the declared group: %+v", groups)
		}
	})

	t.Run("an ordinary log earns no such group", func(t *testing.T) {
		st := stage{Name: "ze-doc-wiring-check", Rerun: "make ze-doc-wiring-check"}
		groups := classifyStage(st, "tmp/verify/wiring.log", "Running ze-doc-verify...\nze-doc-verify failed\n")
		if groupByID(groups, "subcheck:stage-log-truncated") != nil {
			t.Fatalf("a complete log was reported as truncated: %+v", groups)
		}
	})
}

// TestTheFunctionalStageKeepsItsSummaryReconciliation
//
// VALIDATES: ze-functional-test reaches classifyFunctional even when its log
// carries declared groups and a matching count, so the FAIL summary still adds a
// group for every suite no declared group covered.
// PREVENTS: the latent coupling. classifiedGroups asks for declared groups
// before it dispatches, for every other stage, and classifyFunctional
// deliberately ignores the completeness count because the FAIL summary is the
// stronger statement. That held only while no functional producer emitted a
// terminator -- the wiring gate is the only emitter in the tree today -- and the
// day one did, the shortcut would silently replace the stronger check with the
// weaker one.
func TestTheFunctionalStageKeepsItsSummaryReconciliation(t *testing.T) {
	st := stage{Name: functionalStage, Rerun: "make ze-functional-test"}
	declared := failureGroup{GroupID: "case:auth-1", Kind: "files", Stage: "auth", Related: []string{"test/auth/one.ci"}}
	text := declaredLine(t, declared) + completeLine(1) + "FAIL 2 suite(s) failed: auth bgp\n"

	groups := classifyStage(st, "tmp/verify/functional.log", text)
	if groupByID(groups, "case:auth-1") == nil {
		t.Fatalf("the declared group was lost: %+v", groups)
	}
	if groupByID(groups, "suite:bgp") == nil {
		t.Fatalf("the FAIL summary named bgp and no declared group covered it, so it must still earn a group: %+v", groups)
	}
}

// groupByID returns the group with this id, or nil when the slice holds none.
func groupByID(groups []failureGroup, id string) *failureGroup {
	for i := range groups {
		if groups[i].GroupID == id {
			return &groups[i]
		}
	}
	return nil
}

// TestAPathologicalPathCannotForgeAGroup
//
// VALIDATES: a file name carrying a quote, a newline and the protocol's own
// prefix travels through the group line as one path and forges no second group.
// PREVENTS: a check-out with a hostile or merely odd filename injecting groups
// into the failure index, where a forged group naming a foreign path would make
// a red look like somebody else's and drop it from the commit helper's charge.
func TestAPathologicalPathCannotForgeAGroup(t *testing.T) {
	evil := "test/we\"ird\n" + declaredGroupPrefix + " {\"kind\":\"forged\",\"related\":[\"docs\"]}\n.ci"
	line := declaredLine(t, failureGroup{GroupID: "files:ci-sleep-justification", Kind: "files", Related: []string{evil}})
	if strings.Count(line, "\n") != 1 {
		t.Fatalf("the group line is not one line: %q", line)
	}
	groups, complete := parseDeclaredGroups("tmp/verify/wiring.log", line+completeLine(1))
	if !complete || len(groups) != 1 {
		t.Fatalf("expected one group, got %d (complete=%v): %+v", len(groups), complete, groups)
	}
	if groups[0].Kind != "files" {
		t.Fatalf("kind = %q, want files: the payload forged a group", groups[0].Kind)
	}
	if !slices.Equal(groups[0].Related, []string{evil}) {
		t.Fatalf("related = %q, want the one pathological path back unchanged", groups[0].Related)
	}
}

// TestTheWiringGateSpeaksTheProtocolThisRunnerReads
//
// VALIDATES: a group line the wiring gate actually prints parses here, into the
// fields the commit helper attributes on, and its count is accepted.
// PREVENTS: the two ends drifting apart in silence. The prefixes and the JSON
// keys are separate literals in Python and Go, and every drift fails OPEN: the
// runner reads no group, falls back to its prose classifier, and the gate goes
// back to being unattributable with no test red and no gate failure to say so.
func TestTheWiringGateSpeaksTheProtocolThisRunnerReads(t *testing.T) {
	const emit = "import sys; sys.path.insert(0, 'scripts/dev'); " +
		"import verify_wiring_docs as gate; " +
		"gate.declare_failure_group('wiring', ['internal/component/ssh/server.go'], " +
		"'an exported symbol has no non-test reference', 'make ze-doc-wiring-check'); " +
		"gate.declare_groups_complete()"
	// Read the gate before running it. go test caches on the files the test
	// binary opened, and it cannot see a file an exec'd interpreter reads, so
	// without this the result survives an edit to the producer and the drift
	// this test exists to catch would be reported from the cache as green.
	gate := filepath.Join(repoRootFromScriptsStatus(), "scripts", "dev", "verify_wiring_docs.py")
	source, err := os.ReadFile(gate)
	if err != nil {
		t.Fatalf("read %s: %v", gate, err)
	}
	if !strings.Contains(string(source), "def declare_groups_complete") {
		t.Fatalf("%s no longer declares the completeness line this runner requires", gate)
	}

	cmd := exec.CommandContext(t.Context(), "python3", "-c", emit)
	cmd.Dir = repoRootFromScriptsStatus()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run the gate's own emitter: %v\n%s", err, out)
	}

	groups, complete := parseDeclaredGroups("tmp/verify/wiring.log", string(out))
	if !complete {
		t.Fatalf("the gate's completeness line was not accepted:\n%s", out)
	}
	if len(groups) != 1 {
		t.Fatalf("expected one group, got %+v", groups)
	}
	if groups[0].GroupID != "files:wiring" || groups[0].Kind != "files" {
		t.Fatalf("group-id/kind = %q/%q, want files:wiring/files", groups[0].GroupID, groups[0].Kind)
	}
	if want := []string{"internal/component/ssh/server.go"}; !slices.Equal(groups[0].Related, want) {
		t.Fatalf("related = %v, want %v -- the commit helper attributes on this field", groups[0].Related, want)
	}
	if groups[0].Rerun != "make ze-doc-wiring-check" {
		t.Fatalf("rerun = %q, want the gate's own target", groups[0].Rerun)
	}
}

// TestSplitLinesReportsATruncatedRead
//
// VALIDATES: splitLines says so when its scanner stops early. An over-long line
// yields the truncation marker as the last element, a log of ordinary lines is
// returned unchanged, and a stage log truncated before its count makes
// parseDeclaredGroups report incomplete, so the caller keeps its classifier
// instead of trusting the groups the prefix happened to carry.
// PREVENTS: the silent short read. The scanner ran with the DEFAULT 64 KiB token
// limit and nobody read Err(), so one over-long line ended the loop and
// discarded that line AND EVERY LINE AFTER IT. Every classifier in this file
// reads through splitLines, so the failure index then recorded whichever
// failures the surviving prefix happened to contain and said nothing about the
// rest (ai/rules/evidence.md: a partial answer must not look like a whole one).
func TestSplitLinesReportsATruncatedRead(t *testing.T) {
	t.Run("an ordinary log is returned unchanged", func(t *testing.T) {
		text := "### Stage ze-doc-wiring-check\nWiring check FAILED:\nlast line without a newline"
		want := []string{"### Stage ze-doc-wiring-check", "Wiring check FAILED:", "last line without a newline"}
		if got := splitLines(text); !slices.Equal(got, want) {
			t.Fatalf("splitLines(%q) = %q, want %q", text, got, want)
		}
	})

	t.Run("an over-long line ends the read and says so", func(t *testing.T) {
		text := "before the long line\n" + strings.Repeat("x", maxLogLineBytes+1) + "\nafter the long line\n"
		lines := splitLines(text)
		if len(lines) != 2 {
			t.Fatalf("expected the readable prefix and one marker, got %d lines", len(lines))
		}
		if lines[0] != "before the long line" {
			t.Fatalf("the readable prefix was lost: %q", lines[0])
		}
		marker := lines[len(lines)-1]
		if !strings.HasPrefix(marker, logTruncatedMarker) {
			t.Fatalf("the last line is %q, want the truncation marker -- a short slice with no marker reads as a complete log", marker)
		}
		if !strings.Contains(marker, bufio.ErrTooLong.Error()) {
			t.Fatalf("the marker does not name the scanner error, so a human cannot act on it: %q", marker)
		}
		if slices.Contains(lines, "after the long line") {
			t.Fatalf("the scanner is expected to stop at the over-long line; the marker is what makes the loss visible, got %q", lines)
		}
	})

	t.Run("a truncated read makes the declared count disagree", func(t *testing.T) {
		first := failureGroup{
			GroupID: "files:wiring",
			Kind:    "files",
			Related: []string{"internal/component/ssh/server.go"},
			Summary: "an exported symbol has no non-test reference",
			Rerun:   "make ze-doc-wiring-check",
		}
		second := failureGroup{
			GroupID: "subcheck:ci-sleep-ratchet",
			Kind:    "subcheck",
			Summary: "the ci-sleep count rose above the baseline",
			Rerun:   "make ze-doc-wiring-check",
		}
		text := declaredLine(t, first) +
			strings.Repeat("x", maxLogLineBytes+1) + "\n" +
			declaredLine(t, second) +
			completeLine(2)

		groups, complete := parseDeclaredGroups("tmp/verify/wiring.log", text)
		if complete {
			t.Fatalf("a log whose count was truncated away must not be trusted, got %d group(s) accepted", len(groups))
		}
		if len(groups) != 1 {
			t.Fatalf("expected only the group before the truncation, got %+v", groups)
		}
		if !slices.ContainsFunc(splitLines(text), func(line string) bool {
			return strings.HasPrefix(line, logTruncatedMarker)
		}) {
			t.Fatalf("the stage log carries no truncation marker, so the fallback happens for a reason nobody can see")
		}
	})
}
