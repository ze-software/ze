package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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
		Mode:   "ze-verify",
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
	mustReadFileContains(t, root, filepath.ToSlash(filepath.Join(stageLogDir, "01-first.log")), "first failed")
	mustReadFileContains(t, root, filepath.ToSlash(filepath.Join(stageLogDir, "02-second.log")), "second passed")
	mustReadFileContains(t, root, filepath.ToSlash(filepath.Join(stageLogDir, "03-third.log")), "third failed")
	mustReadFileContains(t, root, failuresLogPath, "## Stage: first")
	mustReadFileContains(t, root, failuresLogPath, "## Stage: third")
	mustReadFileContains(t, root, combinedLogPath, "FAIL  2 verify stage(s) failed")
	mustReadFileContains(t, root, combinedLogPath, "Read first: "+failuresLogPath)

	mustReadFileContains(t, root, statusPath, "exit=1")
}

func TestVerifyRunProducesStageAndGroupSummaries(t *testing.T) {
	root := t.TempDir()
	stages := []stage{
		{Name: "ze-unit-test-cached", Rerun: "make ze-unit-test-cached"},
		{Name: "ze-functional-test", Rerun: "make ze-functional-test"},
		{Name: "ze-verify-wiring-docs", Rerun: "make ze-verify-wiring-docs"},
	}
	outputs := map[string]string{
		"ze-unit-test-cached":   readFixture(t, "go-test-mixed.log"),
		"ze-functional-test":    readFixture(t, "functional-groups.log"),
		"ze-verify-wiring-docs": readFixture(t, "wiring-failure.log"),
	}
	code, err := runVerify(context.Background(), verifyConfig{
		Root:   root,
		Mode:   "ze-verify",
		Stages: stages,
		Now:    fixedNow,
		Out:    io.Discard,
		RunStage: func(_ context.Context, _ string, st stage, w io.Writer) int {
			_, _ = io.WriteString(w, outputs[st.Name])
			return 1
		},
		WriteStatus: testStatusWriter,
	})
	if err != nil {
		t.Fatalf("run verify: %v", err)
	}
	if code == 0 {
		t.Fatalf("expected verify failure")
	}

	failureText := readFile(t, root, failuresLogPath)
	for _, want := range []string{
		"Group: package:codeberg.org/thomas-mangin/ze/internal/example/alpha",
		"Rerun: go test ./internal/example/alpha -run '^TestAlpha$'",
		"Group: package:codeberg.org/thomas-mangin/ze/internal/example/beta",
		"Stage: plugin",
		"Rerun: ze-test bgp plugin 1 2",
		"Group: subcheck:ze-doc-test",
		"Rerun: make ze-doc-test",
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
		{Name: "ze-verify-wiring-docs", Rerun: "make ze-verify-wiring-docs"},
		{Name: "ze-exabgp-test", Rerun: "make ze-exabgp-test"},
	}
	outputs := map[string]string{
		"ze-unit-test-cached":   readFixture(t, "go-test-mixed.log"),
		"ze-functional-test":    readFixture(t, "functional-groups.log"),
		"ze-verify-wiring-docs": readFixture(t, "wiring-failure.log"),
		"ze-exabgp-test":        readFixture(t, "exabgp-summary.log"),
	}

	code, err := runVerify(context.Background(), verifyConfig{
		Root:   root,
		Mode:   "ze-verify",
		Stages: stages,
		Now:    fixedNow,
		Out:    io.Discard,
		RunStage: func(_ context.Context, _ string, st stage, w io.Writer) int {
			_, _ = io.WriteString(w, outputs[st.Name])
			return 1
		},
		WriteStatus: testStatusWriter,
	})
	if err != nil {
		t.Fatalf("run verify: %v", err)
	}
	if code != 1 {
		t.Fatalf("expected failure exit, got %d", code)
	}
	for _, want := range []string{"package:codeberg.org/thomas-mangin/ze/internal/example/alpha", "plugin:timeout:bfd", "subcheck:ze-doc-test", "exabgp:failed"} {
		mustReadFileContains(t, root, failuresLogPath, want)
	}
}

func TestVerifyRunAllPassFixture(t *testing.T) {
	root := t.TempDir()
	code, err := runVerify(context.Background(), verifyConfig{
		Root:   root,
		Mode:   "ze-verify",
		Stages: []stage{{Name: "ze-lint", Rerun: "make ze-lint"}, {Name: "ze-unit-test-cached", Rerun: "make ze-unit-test-cached"}},
		Now:    fixedNow,
		Out:    io.Discard,
		RunStage: func(_ context.Context, _ string, _ stage, w io.Writer) int {
			_, _ = io.WriteString(w, "ok\n")
			return 0
		},
		WriteStatus: testStatusWriter,
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
	mustReadFileContains(t, root, statusPath, "mode=ze-verify")
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

func TestClassifyFunctionalInstallFallbackUsesSuiteCommand(t *testing.T) {
	groups := classifyFunctional("tmp/verify/install.log", "FAIL  1 suite(s) failed: install\n")
	if len(groups) != 1 {
		t.Fatalf("expected one install fallback group, got %+v", groups)
	}
	if groups[0].Rerun != "bin/ze-test install --all" {
		t.Fatalf("install rerun = %q, want %q", groups[0].Rerun, "bin/ze-test install --all")
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
		Mode:   "ze-verify",
		Stages: []stage{{Name: "ze-functional-test", Rerun: "make ze-functional-test"}},
		Now:    fixedNow,
		Out:    io.Discard,
		RunStage: func(_ context.Context, _ string, _ stage, w io.Writer) int {
			_, _ = io.WriteString(w, "VERIFY FAILURE GROUP: "+string(b)+"\n")
			return 1
		},
		WriteStatus: testStatusWriter,
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
	text := `# codeberg.org/thomas-mangin/ze/internal/example/build
internal/example/build/bad.go:3:2: undefined: nope
FAIL	codeberg.org/thomas-mangin/ze/internal/example/build [build failed]
--- FAIL: TestOne (0.00s)
    one_test.go:10: no
FAIL
FAIL	codeberg.org/thomas-mangin/ze/internal/example/one	0.01s
`
	groups := classifyGoTest(stage{Name: "ze-unit-test-cached"}, "tmp/verify/unit.log", text)
	if len(groups) != 2 {
		t.Fatalf("expected two package groups, got %+v", groups)
	}
	if groups[0].Kind != "build" || groups[0].Rerun != "go test ./internal/example/build" {
		t.Fatalf("unexpected build group: %+v", groups[0])
	}
	if groups[1].GroupID != "package:codeberg.org/thomas-mangin/ze/internal/example/one" || groups[1].Rerun != "go test ./internal/example/one -run '^TestOne$'" {
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
}

func TestVetFailuresGroupByPackage(t *testing.T) {
	groups := classifyVet(stage{Name: "ze-vet-evidence"}, "tmp/verify/vet.log", readFixture(t, "vet-mixed.log"))
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
	text := "Running ze-doc-test...\noutput\nze-doc-test failed\n"
	groups := classifyWiringDocs(stage{Name: "ze-verify-wiring-docs"}, "tmp/verify/wiring.log", text)
	if len(groups) != 1 {
		t.Fatalf("expected one group, got %+v", groups)
	}
	if groups[0].GroupID != "subcheck:ze-doc-test" || groups[0].Rerun != "make ze-doc-test" {
		t.Fatalf("unexpected wiring group: %+v", groups[0])
	}
}

func TestExabgpVerifyModeSummaryUsesNewlinesAndExactReproducers(t *testing.T) {
	text := readFixture(t, "exabgp-summary.log")
	groups := classifyExabgp(stage{Name: "ze-exabgp-test"}, "tmp/verify/exabgp.log", text)
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
// `make ze-verify`/`ze-verify-changed` (Makefile:263-270 invoke this program,
// not the Makefile's own _ze-verify-impl/_ze-verify-changed-impl targets,
// which have zero callers anywhere in the repo and had silently drifted out
// of sync with this list -- ze-tier-check, ze-iface-resolution-check, and
// ze-plugin-boundary-check were all listed in the dead _impl targets but
// never actually ran).
// PREVENTS: a verification gate added to _ze-verify-impl/_ze-verify-changed-impl
// (the natural-looking place, since ze-tier-check/ze-iface-resolution-check
// already lived there) silently never executing under `make ze-verify`.
//
// ze-hook-test is in the list for the same reason, having hit the same trap from
// the other side: it was reachable ONLY by typing `make ze-hook-test` by hand --
// absent from ze-test (Makefile), from this stage list, and from .woodpecker/
// (whose only step is `make ze-verify`). Its checks guard the agent hooks, whose
// failure mode is silent and fail-CLOSED: a session-id mismatch between
// lib/session-id.sh and pretool-writeedit.py blocks every agent from writing while
// reporting work was never done (real incident, 2026-07-16). A guard nobody runs
// does not guard.
func TestStagesForModeIncludesStaticAnalysisGates(t *testing.T) {
	requiredStages := []string{
		"ze-tier-check",
		"ze-iface-resolution-check",
		"ze-plugin-boundary-check",
		"ze-hook-test",
	}
	for _, mode := range []string{"ze-verify", "ze-verify-changed"} {
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

// VALIDATES: the alloc-ceiling gate (ze-alloc-gate) is registered in the full
// `ze-verify` stage list -- the ACTUAL source of truth CI runs via
// `make ze-verify` (.woodpecker/verify.yml) -- and is deliberately absent from
// the fast `ze-verify-changed` inline loop (spec-fixit-perf-alloc-ci-gate AC-3).
// PREVENTS: the perf-alloc regression gate merging a per-op heap allocation
// undetected because the stage was never wired into the runner CI executes,
// and PREVENTS it silently bloating the per-edit dev loop.
func TestStagesIncludeAllocGate(t *testing.T) {
	full := map[string]bool{}
	for _, st := range stagesForMode("ze-verify", "make") {
		full[st.Name] = true
	}
	if !full["ze-alloc-gate"] {
		t.Errorf("stagesForMode(\"ze-verify\") missing ze-alloc-gate; CI would never run the alloc gate")
	}

	changed := map[string]bool{}
	for _, st := range stagesForMode("ze-verify-changed", "make") {
		changed[st.Name] = true
	}
	if changed["ze-alloc-gate"] {
		t.Errorf("stagesForMode(\"ze-verify-changed\") must NOT include ze-alloc-gate (keeps the inline dev loop fast)")
	}
}

// VALIDATES: the documentation-consistency gate (doc drift + corpus path
// references) is wired into the ACTUAL stage list CI runs via `make ze-verify`
// (.woodpecker/verify.yml), for the default full mode. Before this the gate was
// dark: stagesForMode enumerated every ze-verify stage and named no ze-doc-*
// target, so `check_doc_links.py` could exit 1 with broken discovery-layer refs
// while CI stayed green (spec-fixit-doc-gate-and-refs AC-1).
// PREVENTS: a broken index/rule path reference or a doc count drift merging
// undetected because no gate that CI runs ever exercised the doc checks.
func TestStagesForModeIncludesDocGate(t *testing.T) {
	names := map[string]bool{}
	for _, st := range stagesForMode("ze-verify", "make") {
		names[st.Name] = true
	}
	for _, want := range []string{"ze-doc-test", "ze-doc-links"} {
		if !names[want] {
			t.Errorf("stagesForMode(%q) missing doc-gate stage %q; the doc checks would never run under CI", "ze-verify", want)
		}
	}
}

// VALIDATES: the doc gate is ALSO wired into the fast `ze-verify-changed` inline
// loop, not only the full run (spec-fixit-doc-gate-and-refs AC-1, R-3). A single
// wired branch would let changed-mode sessions stay green on a broken reference.
// PREVENTS: the changed-mode verify path skipping the doc-consistency gate.
func TestStagesForModeChangedIncludesDocGate(t *testing.T) {
	names := map[string]bool{}
	for _, st := range stagesForMode("ze-verify-changed", "make") {
		names[st.Name] = true
	}
	for _, want := range []string{"ze-doc-test", "ze-doc-links"} {
		if !names[want] {
			t.Errorf("stagesForMode(%q) missing doc-gate stage %q; changed-mode would skip the doc checks", "ze-verify-changed", want)
		}
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
}

func testStatusWriter(root string, code int, mode, skipped string, now time.Time) error {
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
