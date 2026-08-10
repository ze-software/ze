package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
		"Group: package:github.com/ze-software/ze/internal/example/alpha",
		"Rerun: go test ./internal/example/alpha -run '^TestAlpha$'",
		"Group: package:github.com/ze-software/ze/internal/example/beta",
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
	for _, want := range []string{"package:github.com/ze-software/ze/internal/example/alpha", "plugin:timeout:bfd", "subcheck:ze-doc-test", "exabgp:failed"} {
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
// `make ze-verify`/`ze-verify-changed` -- the Makefile targets invoke this
// program and enumerate no stages of their own.
// PREVENTS: a verification gate landing somewhere that never executes under
// `make ze-verify`. A duplicate _ze-verify-impl/_ze-verify-changed-impl pair
// used to make that trap easy: they read like the stage list, had zero callers,
// and had silently drifted (ze-tier-check, ze-iface-resolution-check and
// ze-plugin-boundary-check were listed there but never ran). They were deleted
// by plan/spec-fixit-verify-stage-ssot.md; this test keeps the remaining list
// honest.
//
// ze-hook-test is in the list for the same reason, having hit the same trap from
// the other side: it was reachable ONLY by typing `make ze-hook-test` by hand --
// absent from ze-test (Makefile), from this stage list, and from
// .github/workflows/ (whose verify job's only step is `make ze-verify`). Its
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
// `make ze-verify` (.github/workflows/verify.yml) -- and is deliberately absent from
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
// (.github/workflows/verify.yml), for the default full mode. Before this the gate was
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

// ── Stage-list SSOT guards (plan/spec-fixit-verify-stage-ssot.md) ───────────
//
// stagesForMode is the ONLY live verify stage list: `make ze-verify` and
// `make ze-verify-changed` both shell out to this runner (Makefile), and CI's
// only step is `make ze-verify` (.github/workflows/verify.yml). A gate absent from
// stagesForMode therefore never runs anywhere.
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
	"ze-tracked-build-check",
	"ze-platform-vet",
	"ze-verify-wiring-docs",
	"ze-doc-test",
	"ze-doc-links",
	"ze-validate-tree",
	"ze-regen-check-readonly",
	"ze-vet-evidence",
	"ze-hook-test",
	"ze-unit-test-cached",
	"ze-unit-test-race-changed",
	"ze-alloc-gate",
	"ze-functional-test",
	"ze-exabgp-test",
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
	"ze-tracked-build-check",
	"ze-platform-vet",
	"ze-verify-wiring-docs",
	"ze-doc-test",
	"ze-doc-links",
	"ze-validate-tree",
	"ze-regen-check-readonly",
	"ze-hook-test",
	"ze-unit-test-changed",
	"ze-functional-test",
	"ze-exabgp-test",
}

// TestStagesForModeMatchesGolden locks the full stage list of BOTH modes.
//
// VALIDATES: stagesForMode returns exactly the committed golden list, in order,
// with no duplicates, for `ze-verify` and `ze-verify-changed` alike (AC-2).
// PREVENTS: a stage being dropped, silently reordered, or added to only one of
// the two hand-duplicated mode branches -- the precise drift that let the dead
// _ze-verify-impl targets diverge from the live list for an unknown period.
func TestStagesForModeMatchesGolden(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		golden []string
	}{
		{"ze-verify", goldenStagesZeVerify},
		{"ze-verify-changed", goldenStagesZeVerifyChanged},
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
	{"ze-vet-evidence", "", "evidence vetting is full-verify only"},
	{"ze-alloc-gate", "", "alloc ceiling is full-verify only (spec-fixit-perf-alloc-ci-gate AC-3)"},
}

// TestStagesForModeBranchesAgree is the divergence check the goldens cannot be.
//
// VALIDATES: after removing the documented mode-specific stages, the two
// branches of stagesForMode carry the IDENTICAL set of gates (AC-2's "fails if
// the two branches diverge").
// PREVENTS: a gate wired into `ze-verify` but not `ze-verify-changed` (or the
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
	full, changed := set("ze-verify"), set("ze-verify-changed")

	for _, ms := range modeSpecificStages {
		if ms.full != "" {
			if !full[ms.full] {
				t.Errorf("modeSpecificStages lists %q as a full-verify stage, but stagesForMode(%q) does not emit it; the allowlist has rotted", ms.full, "ze-verify")
			}
			delete(full, ms.full)
		}
		if ms.changed != "" {
			if !changed[ms.changed] {
				t.Errorf("modeSpecificStages lists %q as a changed-mode stage, but stagesForMode(%q) does not emit it; the allowlist has rotted", ms.changed, "ze-verify-changed")
			}
			delete(changed, ms.changed)
		} else if changed[ms.full] {
			// ms.full is documented as full-verify-ONLY, yet the changed branch
			// now emits it. Report that directly: falling through would delete
			// it from `full` only, and the symmetric-difference check below
			// would then blame it for running in changed-mode but not full --
			// the exact opposite of the truth.
			t.Errorf("stage %q is documented full-verify-only in modeSpecificStages (%s), but stagesForMode(%q) now emits it too; update modeSpecificStages if that is deliberate", ms.full, ms.why, "ze-verify-changed")
			delete(changed, ms.full)
		}
	}

	for name := range full {
		if !changed[name] {
			t.Errorf("stage %q runs under `ze-verify` but NOT `ze-verify-changed`; add it to both branches, or document it in modeSpecificStages with a reason", name)
		}
	}
	for name := range changed {
		if !full[name] {
			t.Errorf("stage %q runs under `ze-verify-changed` but NOT `ze-verify`; add it to both branches, or document it in modeSpecificStages with a reason", name)
		}
	}
}

// TestStagesForModeIncludesRegenCheck pins the generated-file staleness gate
// into BOTH mode branches.
//
// VALIDATES: the write-safe, read-only regen check runs under `make ze-verify`
// and `make ze-verify-changed` (AC-4).
// PREVENTS: a stale generated file reaching a commit unnoticed because the only
// gate that detected it -- `ze-regen-check` -- runs nowhere. Scoped honestly:
// this is NEW coverage for the ai/* discovery indexes, feature_tags' outputs
// (.golangci.yml, gokrazy/ze/config.json, docs/guide/quickstart.md) and
// mk/test-fuzz-targets.mk. It is REDUNDANT (a second, uncached path) for
// plugin/all/all.go, which TestGeneratedPluginImportsCurrent and
// TestVerifyWiringDocsChecksPluginImports already covered. CLAUDE.md/AGENTS.md
// and the skill mirrors are NOT covered here at all -- see the two documented
// exclusions above the target in the Makefile.
// PREVENTS: the MUTATING `ze-regen-check` being wired instead: it depends on
// `ze-regen`, which rewrites generated files before diffing them, so verify
// would leave a dirty working tree (spec R-1).
func TestStagesForModeIncludesRegenCheck(t *testing.T) {
	for _, mode := range []string{"ze-verify", "ze-verify-changed"} {
		names := map[string]bool{}
		for _, st := range stagesForMode(mode, "make") {
			names[st.Name] = true
		}
		if !names["ze-regen-check-readonly"] {
			t.Errorf("stagesForMode(%q) missing ze-regen-check-readonly; generated-file staleness would go unguarded", mode)
		}
		if names["ze-regen-check"] {
			t.Errorf("stagesForMode(%q) wires the MUTATING ze-regen-check; it runs ze-regen and would leave verify with a dirty tree", mode)
		}
	}
}

// TestStagesForModeIncludesValidateTree pins the ze-validate half that a shared
// checkout can carry into BOTH mode branches.
//
// VALIDATES: `make ze-validate-tree` runs under `make ze-verify` and
// `make ze-verify-changed`, and the target it names exists.
// PREVENTS: the five validate.py checks going back to having no automatic
// caller at all. Until 2026-08-09 nothing ran them: no Makefile target depended
// on ze-validate, no hook called it, commit_helper.py did not, and stagesForMode
// named neither. Their only callers were two sentences of prose in
// ai/skills/ze-review.md and ai/skills/ze-close.md.
// PREVENTS: the whole `ze-validate` target being wired instead. Two of its five
// checks (check_cross_package_wiring, check_cli_handler_coverage in
// scripts/dev/validate.py) take `changed_files` -- `git diff HEAD` plus
// untracked files -- as their subject. Several sessions share this checkout, so
// those files are largely another session's half-written work, and both checks
// demand a completeness (a cross-package caller, a .ci test) that a file mid-edit
// cannot show. Wiring them would red a verify run whose author changed none of
// the files being judged.
func TestStagesForModeIncludesValidateTree(t *testing.T) {
	for _, mode := range []string{"ze-verify", "ze-verify-changed"} {
		names := map[string]bool{}
		for _, st := range stagesForMode(mode, "make") {
			names[st.Name] = true
		}
		if !names["ze-validate-tree"] {
			t.Errorf("stagesForMode(%q) missing ze-validate-tree; the source-anchor and spec-AC checks would run nowhere", mode)
		}
		if names["ze-validate"] {
			t.Errorf("stagesForMode(%q) wires the full ze-validate; its two changed-file checks judge other sessions' uncommitted files in this shared checkout", mode)
		}
	}

	corpus := makefileCorpus(t)
	if !strings.Contains(corpus, "\nze-validate-tree:\n") {
		t.Error("no ze-validate-tree target in the Makefile corpus: the stage would fail with 'No rule to make target'")
	}
	if !strings.Contains(corpus, "--changed-file ''") {
		t.Error("ze-validate-tree no longer declares an empty changed set; without it validate.py falls back to git diff HEAD and the two changed-file checks come back")
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
	for _, mode := range []string{"ze-verify", "ze-verify-changed"} {
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
// serves it a cached PASS (and under ze-verify-changed, changed-pkgs.sh maps a
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
	for _, mode := range []string{"ze-verify", "ze-verify-changed"} {
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

// regenCheckPrereqs is the full prerequisite set ze-regen-check-readonly must
// carry, and why each one is there. Asserted as an EXACT set, not a floor: an
// earlier version only checked `len(prereqs) >= 4`, and a reviewer showed that
// half the list (including the only guard for ai/INSTRUCTIONS.md) could be
// deleted with every test still green.
var regenCheckPrereqs = map[string]string{
	"ze-plugin-imports-check":  "plugin_imports.go -> internal/component/plugin/all/all.go",
	"ze-yang-glue-check":       "yang_glue.go -> yang/*/register.go, embed.go",
	"ze-feature-tags-check":    "feature_tags.go -> .golangci.yml, gokrazy/ze/config.json, docs/guide/quickstart.md",
	"ze-fuzz-targets-check":    "fuzz-targets.py -> mk/test-fuzz-targets.mk",
	"ze-doc-check-stale":       "code_to_docs.py -> ai/CODE-TO-DOCS.md",
	"ze-rules-render-check":    "rules_points.py -> ai/rules/<rule>.md, rendered from ai/rules/points/",
	"ze-rules-index-check":     "rules_index.py -> ai/rules/INDEX.md",
	"ze-rules-condensed-check": "rules_condensed.py -> ai/rules/TRIGGERS.md, ai/rules/CORE.md",
	"ze-rules-lint":            "rules_lint.py -> ai/rules/*.md format contract (read-only validator, no generated output)",
	"ze-arch-map-check":        "arch_map.py -> the architecture lists in ai/INSTRUCTIONS.md (NOT covered by ze-doc-test)",
	"ze-discovery-index-check": "package_map.py, docs_to_code.py -> the ai/ discovery indexes",
	"ze-test-health-check":     "testing_health.py -> docs/features/test-health.md, test/health/latest.json, the sensitivity baseline",
}

// generatorChecks maps every generator reachable from `make generate` or
// `make ze-regen` to the regenCheckPrereqs entry that guards its output.
//
// An empty value means DELIBERATELY EXCLUDED, and the reason is recorded in the
// exclusion block above ze-regen-check-readonly in the Makefile. In short: every
// output of skill_sync.sh (reached via ze-ai-sync) is gitignored, so nothing
// committed can drift and a fresh CI checkout -- where those files do not exist
// at all -- would red on every run.
var generatorChecks = map[string]string{
	// `generate:` recipe
	"scripts/codegen/yang_glue.go":      "ze-yang-glue-check",
	"scripts/codegen/plugin_imports.go": "ze-plugin-imports-check",
	"scripts/codegen/feature_tags.go":   "ze-feature-tags-check",
	"scripts/dev/fuzz-targets.py":       "ze-fuzz-targets-check",
	// `ze-regen:` prerequisite targets
	"ze-ai-instructions": "ze-arch-map-check",
	"ze-ai-sync":         "", // gitignored outputs -- excluded on purpose
	"ze-doc-index":       "ze-doc-check-stale",
	"ze-rules-render":    "ze-rules-render-check",
	"ze-rules-index":     "ze-rules-index-check",
	"ze-rules-condensed": "ze-rules-condensed-check",
	"ze-discovery-index": "ze-discovery-index-check",
	"ze-arch-map":        "ze-arch-map-check",
	"generate":           "", // expanded into its own generators above
	// Scripts run INSIDE those sub-targets' recipes. Walking only the target
	// names left these invisible: a reviewer showed a new script added to
	// ze-discovery-index's recipe would be accepted with no check at all.
	"scripts/dev/arch_map.py":        "ze-arch-map-check",
	"scripts/dev/code_to_docs.py":    "ze-doc-check-stale",
	"scripts/dev/rules_index.py":     "ze-rules-index-check",
	"scripts/dev/rules_points.py":    "ze-rules-render-check",
	"scripts/dev/rules_condensed.py": "ze-rules-condensed-check",
	"scripts/dev/package_map.py":     "ze-discovery-index-check",
	"scripts/dev/docs_to_code.py":    "ze-discovery-index-check",
	"scripts/dev/skill_sync.sh":      "", // gitignored outputs -- excluded on purpose
	"scripts/dev/testing_health.py":  "ze-test-health-check",
	"ze-test-health":                 "ze-test-health-check",
}

// recipeBody returns the lines of `target`'s recipe: everything after the rule
// head until the first line that starts at column 0.
//
// Line-based on purpose. A regex span like `(?ms)^generate:.*?\n\n` looks
// equivalent and is not: GNU make permits blank lines inside a recipe, so a
// stray blank line silently truncates the scan and an unguarded generator
// added below it is accepted. A reviewer proved exactly that.
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
		return head, body
	}
	t.Fatalf("target %q not found in the Makefile corpus. This test must not pass vacuously.", target)
	return "", nil
}

// optionalRecipeBody is recipeBody for a name that MIGHT not be a target.
//
// The deeper walk feeds it every field of a sub-target's rule head, and a
// prerequisite is legally a file (`ze-doc-index: bin/ze`), a variable
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
// actually uses -- including `$(GO) run`, which is house style at the ze-verify
// and ze-verify-changed recipes, and `uv run`, used by the ExaBGP suite. A
// HONEST LIMIT -- this is best-effort detection, NOT a guarantee. An earlier
// version of this comment claimed a generator in an unlisted form "is NOT
// silently ignored, because the exact-count assertions fail when the parsed set
// changes size". A reviewer measured that false twice over: (a) there is NO
// count assertion for generators found inside SUB-TARGET recipes -- the only
// counts are on `generate:`'s 4 and `ze-regen`'s 9 prerequisites; and (b) even
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
// VALIDATES: ze-regen-check-readonly carries exactly the documented
// prerequisite set, and every generator reachable from `make generate` or
// `make ze-regen` is guarded by one of them (or is an explicit, reasoned
// exclusion).
// PREVENTS: two distinct drifts. (a) A new generator added to `generate` or
// `ze-regen` with no read-only check, leaving its output's staleness unguarded
// everywhere -- the drift class this spec exists to kill, one level up from the
// stage list; two of the four codegen scripts (feature_tags, fuzz-targets) were
// previously guarded ONLY by the mutating ze-regen-check's `git diff`, which
// runs nowhere. (b) A prerequisite being quietly dropped from the target, which
// matters most for ze-arch-map-check: unlike the other doc checks it is NOT
// also run by ze-doc-test, so deleting it would leave ai/INSTRUCTIONS.md's
// architecture lists with no guard at all.
func TestRegenCheckReadonlyCoversGenerators(t *testing.T) {
	corpus := makefileCorpus(t)

	head, _ := recipeBody(t, corpus, "ze-regen-check-readonly")
	got := map[string]bool{}
	for f := range strings.FieldsSeq(head) {
		got[f] = true
	}
	for want, why := range regenCheckPrereqs {
		if !got[want] {
			t.Errorf("ze-regen-check-readonly is missing prerequisite %q (%s); that output's staleness would be unguarded under `make ze-verify`", want, why)
		}
	}
	for have := range got {
		if _, known := regenCheckPrereqs[have]; !known {
			t.Errorf("ze-regen-check-readonly has undocumented prerequisite %q; add it to regenCheckPrereqs with the generator and output it guards", have)
		}
	}

	// Every generator reachable from `make generate` or `make ze-regen`.
	//
	// Walk the sub-targets' RECIPES too, not just their names. ze-discovery-index
	// and ze-ai-instructions run generator scripts of their own, so stopping at
	// the name would let a fourth script added inside ze-discovery-index's recipe
	// go completely unguarded -- proven by a reviewer.
	var producers []string
	_, genBody := recipeBody(t, corpus, "generate")
	producers = append(producers, producerScripts(strings.Join(genBody, "\n"))...)
	if len(producers) != 4 {
		t.Fatalf("parsed %d generators from the `generate:` recipe (%v), expected 4; the recipe changed. Update generatorChecks to match. This test must not pass vacuously.", len(producers), producers)
	}

	regenHead, _ := recipeBody(t, corpus, "ze-regen")
	subTargets := strings.Fields(regenHead)
	// EXACT, not a floor: with `>= 5` a prerequisite could be DELETED from
	// ze-regen unnoticed, so `make ze-regen` would stop regenerating an output
	// that ze-regen-check-readonly (an un-parkable structural gate) still checks
	// -- a red whose documented remediation does not fix it.
	if len(subTargets) != 9 {
		t.Fatalf("`ze-regen` has %d prerequisites (%v), expected 9; if that is deliberate, update this count and generatorChecks together.", len(subTargets), subTargets)
	}
	producers = append(producers, subTargets...)
	for _, sub := range subTargets {
		if sub == "generate" {
			continue // already expanded above
		}
		_, subBody := recipeBody(t, corpus, sub)
		producers = append(producers, producerScripts(strings.Join(subBody, "\n"))...)
		// A sub-target may itself have prerequisites (ze-ai-instructions:
		// ze-arch-map); walk those recipes too, one level.
		subHead, _ := recipeBody(t, corpus, sub)
		for sub2 := range strings.FieldsSeq(subHead) {
			sub2Body := optionalRecipeBody(corpus, sub2)
			producers = append(producers, producerScripts(strings.Join(sub2Body, "\n"))...)
		}
	}

	for _, producer := range producers {
		target, known := generatorChecks[producer]
		if !known {
			t.Errorf("`make generate`/`ze-regen` runs %q, but generatorChecks does not say which read-only check guards its output. Wire one into ze-regen-check-readonly and record it, or record it as a reasoned exclusion -- otherwise its staleness is unguarded everywhere.", producer)
			continue
		}
		if target == "" {
			continue // documented exclusion
		}
		if !got[target] {
			t.Errorf("%q is guarded by %q, but that is not a prerequisite of ze-regen-check-readonly, so it never runs under `make ze-verify`", producer, target)
		}
	}
}

// TestMakeDryRunDetectsDashN guards the refusal that keeps `make -n ze-verify`
// from forging a verified state.
//
// VALIDATES: makeDryRun reads MAKEFLAGS the way GNU make writes it -- short
// flags concatenated in the first field with no leading dash.
// PREVENTS: the single worst failure this runner can have. ze-verify's recipe
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
		// leading space (captured: `make ze-verify ZE_VERIFY_LOG=tmp/x.log` ->
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
	for _, mode := range []string{"ze-verify", "ze-verify-changed"} {
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

// TestGovulncheckScheduledWorkflow pins the shape of the scheduled SCA workflow
// (spec-fixit-supply-chain-hardening AC-1, SCHEDULED default).
//
// VALIDATES: .github/workflows/govulncheck.yml is a SCHEDULED job (schedule/cron)
// that invokes `make ze-vulncheck`, never triggers on push/pull_request, and that
// govulncheck is deliberately NOT a stagesForMode entry -- so the inline
// `make ze-verify` / merge loop is never blocked by a vuln-DB network fetch or a
// transient advisory.
// PREVENTS: (a) the SCA scan drifting onto the fast merge gate; (b) the scheduled
// job losing its trigger and running nowhere; (c) a future edit wiring
// ze-vulncheck into stagesForMode, re-coupling the scan to the pre-commit loop.
func TestGovulncheckScheduledWorkflow(t *testing.T) {
	body := stripYAMLComments(readFile(t, repoRootFromScriptsStatus(), ".github/workflows/govulncheck.yml"))

	// Scheduled trigger present (also proves the file parsed to real content;
	// non-vacuous).
	for _, want := range []string{"schedule:", "cron"} {
		if !strings.Contains(body, want) {
			t.Errorf("govulncheck.yml must declare a %q trigger; body (comments stripped):\n%s", want, body)
		}
	}
	// Invokes the single-source-of-truth make target.
	if !strings.Contains(body, "make ze-vulncheck") {
		t.Errorf("govulncheck.yml must run `make ze-vulncheck`; body (comments stripped):\n%s", body)
	}
	// Never a push/pull_request trigger -- the dev loop stays unblocked.
	for _, forbidden := range []string{"push:", "pull_request:"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("govulncheck.yml must NOT trigger on %q: the SCA scan is scheduled-only and must not gate the merge loop", forbidden)
		}
	}
	// govulncheck is NOT a stagesForMode entry in EITHER mode, so neither
	// `make ze-verify` nor `make ze-verify-changed` runs it inline.
	for _, mode := range []string{"ze-verify", "ze-verify-changed"} {
		for _, st := range stagesForMode(mode, "make") {
			if strings.Contains(st.Name, "vulncheck") {
				t.Errorf("stagesForMode(%q) contains stage %q; govulncheck must stay a scheduled CI job, never an inline verify stage", mode, st.Name)
			}
		}
	}
}

// TestCodeQLBuildUsesShippedTags pins that CodeQL analyses the feature-gated
// surface, not a bare build (spec-fixit-supply-chain-hardening AC-2).
//
// VALIDATES: the manual Go build step in .github/workflows/codeql.yml compiles the
// SHIPPED -tags combos (ze_core/ze_distro/ze_appliance/ze_setup), so code behind
// //go:build ze_core / ze_appliance enters the CodeQL database, and is NOT a bare
// `go build ./...` (which compiles none of it).
// PREVENTS: reverting to a tagless build that leaves most of cmd/ze and the whole
// appliance -- the largest attack surface -- unanalysed while the job stays green.
func TestCodeQLBuildUsesShippedTags(t *testing.T) {
	body := stripYAMLComments(readFile(t, repoRootFromScriptsStatus(), ".github/workflows/codeql.yml"))

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
