// Design: docs/architecture/testing/verify-freshness-scope.md -- native verifier dependency stages
// Overview: verifydeps.go -- plans, execution, and structured reports

package verifydeps

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
)

const testModulePath = "github.com/ze-software/ze"

// VALIDATES: the verifier-only command exposes its five native actions.
// PREVENTS: losing an action during verification routing changes.
func TestTheActionTableIsComplete(t *testing.T) {
	listing := Actions()
	got := make([]string, 0, len(listing.Actions))
	for _, action := range listing.Actions {
		got = append(got, action.Verb)
	}
	want := []string{
		VerbEvidenceVet,
		VerbVulnerability,
		VerbUnitCached,
		VerbUnitRaceChanged,
		VerbAlloc,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("verbs are %v, want %v", got, want)
	}
}

// VALIDATES: evidence vet covers every native Linux evidence package under the
// pinned non-cgo toolchain environment.
// PREVENTS: deleting one migrated evidence area from the vet population.
func TestEvidenceVetPlanPreservesItsExactPopulation(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"internal/le/evidence", "internal/le/deployment", "internal/le/qemu"} {
		mkdir(t, root, directory)
	}
	plan, _, code, err := planFor(context.Background(), root, VerbEvidenceVet, fakeDependencies(root))
	if err != nil || code != 0 {
		t.Fatalf("plan failed with code %d: %v", code, err)
	}
	want := []string{"go", "vet", "./internal/le/evidence/...", "./internal/le/deployment/...", "./internal/le/qemu/..."}
	if !slices.Equal(plan.Commands[0].Command, want) {
		t.Fatalf("command is %v, want %v", plan.Commands[0].Command, want)
	}
	assertOverride(t, plan.Commands[0], "GOOS=linux")
	assertOverride(t, plan.Commands[0], "CGO_ENABLED=0")
	assertNoOverride(t, plan.Commands[0], "GOARCH=")
	if slices.Contains(plan.Commands[0].Command, "-tags") {
		t.Fatal("evidence vet unexpectedly carries feature tags")
	}
}

// VALIDATES: vulnerability scanning resolves and directly executes an installed
// host tool over ./..., while only the scanner receives Linux/amd64.
// PREVENTS: restoring go run, cross-compiling the tool, or scanning the host
// package graph.
func TestVulnerabilityPlanUsesTheInstalledToolDirectly(t *testing.T) {
	root := t.TempDir()
	deps := fakeDependencies(root)
	deps.lookPath = func(name string) (string, error) {
		if name != "govulncheck" {
			t.Fatalf("looked up %q, want govulncheck", name)
		}
		return "/tools/govulncheck", nil
	}
	plan, _, code, err := planFor(context.Background(), root, VerbVulnerability, deps)
	if err != nil || code != 0 {
		t.Fatalf("plan failed with code %d: %v", code, err)
	}
	want := []string{"/tools/govulncheck", "./..."}
	if !slices.Equal(plan.Commands[0].Command, want) {
		t.Fatalf("command is %v, want %v", plan.Commands[0].Command, want)
	}
	assertOverride(t, plan.Commands[0], "GOOS=linux")
	assertOverride(t, plan.Commands[0], "GOARCH=amd64")
	assertOverride(t, plan.Commands[0], "GOTOOLCHAIN=go1.27.0")
	if slices.Contains(plan.Commands[0].Command, "run") {
		t.Fatal("vulnerability command routes through go run")
	}
}

// VALIDATES: a missing govulncheck fails before a scan with the shell's
// cannot-start code.
// PREVENTS: a missing SCA tool becoming a successful no-op.
func TestVulnerabilityPlanFailsClosedWhenTheToolIsMissing(t *testing.T) {
	root := t.TempDir()
	deps := fakeDependencies(root)
	deps.lookPath = func(string) (string, error) { return "", errors.New("not installed") }
	_, children, code, err := planFor(context.Background(), root, VerbVulnerability, deps)
	if err == nil || code != gaterun.CannotStart {
		t.Fatalf("missing tool answered code %d and error %v", code, err)
	}
	if len(children) != 0 {
		t.Fatalf("missing tool started children: %v", children)
	}
}

// VALIDATES: the cacheable pass derives the go-list population, excludes the
// module root, then runs shipped tags before bare-core compile-out checks.
// PREVENTS: testing ./... with the root tooling package or omitting the second
// absent-feature check.
func TestUnitCachedPlanPreservesPackageAndTagPopulations(t *testing.T) {
	root := t.TempDir()
	deps := fakeDependencies(root)
	deps.execute = scriptedExecutor(map[string]scriptedResult{
		"go list ./...": {output: testModulePath + "\n" + testModulePath + "/cmd/ze\n" + testModulePath + "/internal/core/x\n"},
	})
	plan, children, code, err := planFor(context.Background(), root, VerbUnitCached, deps)
	if err != nil || code != 0 {
		t.Fatalf("plan failed with code %d: %v", code, err)
	}
	wantPackages := []string{testModulePath + "/cmd/ze", testModulePath + "/internal/core/x"}
	if !slices.Equal(plan.Packages, wantPackages) {
		t.Fatalf("packages are %v, want %v", plan.Packages, wantPackages)
	}
	if len(children) != 1 || !slices.Equal(children[0].Command, []string{"go", "list", "./..."}) {
		t.Fatalf("discovery children are %+v", children)
	}
	if len(plan.Commands) != 2 {
		t.Fatalf("planned %d commands, want 2", len(plan.Commands))
	}
	assertContainsSequence(t, plan.Commands[0].Command, "-tags", "ze_core ze_bgp ze_web", wantPackages[0], wantPackages[1])
	assertContainsSequence(t, plan.Commands[1].Command, "-tags", "ze_core", "./cmd/ze/hub")
	assertOverride(t, plan.Commands[0], "CGO_ENABLED=0")
	assertOverride(t, plan.Commands[0], "GOMAXPROCS=4")
}

// VALIDATES: a child failure stops a multi-command stage and preserves the
// first external exit code.
// PREVENTS: a later compile-out check hiding the full-pass failure.
func TestUnitCachedStopsAtTheFirstFailureCode(t *testing.T) {
	root := t.TempDir()
	deps := fakeDependencies(root)
	calls := 0
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		calls++
		code := 0
		output := ""
		if plan.Name == "go-package-population" {
			output = testModulePath + "/internal/core/x\n"
		} else if strings.HasSuffix(plan.Name, ":full") {
			code = 23
		}
		return output, childFrom(plan, code)
	}
	report, code := run(context.Background(), root, VerbUnitCached, deps)
	if code != 23 || report.Code != 23 {
		t.Fatalf("stage answered %d / report %d, want 23", code, report.Code)
	}
	if calls != 2 {
		t.Fatalf("executed %d commands, want discovery plus first test only", calls)
	}
	if len(report.Children) != 2 {
		t.Fatalf("reported %d children, want 2", len(report.Children))
	}
}

// VALIDATES: changed-race selection includes mapped groups in native table order
// plus buildable unmapped packages, then enables both race and cgo.
// PREVENTS: silently dropping a changed package, changing group order, or
// confusing a race run with the non-cgo cached pass.
func TestRaceChangedDerivesTheExactPopulationAndEnvironment(t *testing.T) {
	root := t.TempDir()
	deps := fakeDependencies(root)
	unmapped := filepath.Join(root, "internal/le", "verifydeps")
	deps.execute = scriptedExecutor(map[string]scriptedResult{
		"git diff --name-only -- *.go":                                            {output: "internal/core/x.go\ninternal/le/verifydeps/new.go\n"},
		"git diff --cached --name-only -- *.go":                                   {output: "internal/component/bgp/x.go\n"},
		"git ls-files --others --exclude-standard -- *.go":                        {},
		"go list -e -f {{if not .Error}}{{.Dir}}{{end}} ./internal/le/verifydeps": {output: unmapped + "\n"},
	})
	plan, _, code, err := planFor(context.Background(), root, VerbUnitRaceChanged, deps)
	if err != nil || code != 0 {
		t.Fatalf("plan failed with code %d: %v", code, err)
	}
	want := []string{"./internal/component/bgp/...", "./internal/core/...", "./internal/le/verifydeps"}
	if !slices.Equal(plan.Packages, want) {
		t.Fatalf("packages are %v, want %v", plan.Packages, want)
	}
	if len(plan.Commands) != 2 {
		t.Fatalf("planned %d test commands, want 2", len(plan.Commands))
	}
	if !slices.Contains(plan.Commands[0].Command, "-race") {
		t.Fatalf("race command has no -race: %v", plan.Commands[0].Command)
	}
	assertOverride(t, plan.Commands[0], "CGO_ENABLED=1")
	assertOverride(t, plan.Commands[0], "GOMAXPROCS=4")
	assertContainsSequence(t, plan.Commands[0].Command, want...)
}

// VALIDATES: every changed-scope query is required, and its exact failure code
// reaches the stage verdict.
// PREVENTS: a failed git query being read as no changed Go files.
func TestRaceChangedFailsClosedOnPopulationFailure(t *testing.T) {
	root := t.TempDir()
	deps := fakeDependencies(root)
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		return "", childFrom(plan, 7)
	}
	report, code := run(context.Background(), root, VerbUnitRaceChanged, deps)
	if code != 7 || report.Error == "" {
		t.Fatalf("stage answered code %d and error %q", code, report.Error)
	}
	if len(report.Children) != 1 {
		t.Fatalf("reported %d children, want the first failed query", len(report.Children))
	}
}

// VALIDATES: a genuinely empty changed population runs no race command and
// reports the three successful git queries.
// PREVENTS: treating fail-closed discovery as permission to skip, or running
// broad race tests when nothing changed.
func TestRaceChangedEmptyPopulationRunsNoTests(t *testing.T) {
	root := t.TempDir()
	deps := fakeDependencies(root)
	deps.execute = scriptedExecutor(map[string]scriptedResult{
		"git diff --name-only -- *.go":                     {},
		"git diff --cached --name-only -- *.go":            {},
		"git ls-files --others --exclude-standard -- *.go": {},
	})
	report, code := run(context.Background(), root, VerbUnitRaceChanged, deps)
	if code != 0 || report.Code != 0 {
		t.Fatalf("empty population answered %d / %d", code, report.Code)
	}
	if len(report.Children) != 3 {
		t.Fatalf("reported %d children, want three git queries", len(report.Children))
	}
}

// VALIDATES: allocation planning preserves the exact package boundary,
// count-based benchtime, log path, benchmark flags, and non-cgo environment.
// PREVENTS: benchmarking plugin/server, inheriting time-based benchtime, or
// turning an allocation measurement into a race build.
func TestAllocPlanPreservesBenchmarkContract(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, "internal/component/bgp/reactor")
	mkdir(t, root, "internal/component/plugin")
	deps := fakeDependencies(root)
	deps.getenv = func(key string) string {
		if key == "ALLOC_GATE_BENCHTIME" {
			return "450x"
		}
		return ""
	}
	plan, _, code, err := planFor(context.Background(), root, VerbAlloc, deps)
	if err != nil || code != 0 {
		t.Fatalf("plan failed with code %d: %v", code, err)
	}
	if plan.Benchtime != "450x" {
		t.Fatalf("benchtime is %q, want 450x", plan.Benchtime)
	}
	wantLog := filepath.Join(root, "tmp", "verify", "alloc-gate-bench.txt")
	if plan.BenchmarkLog != wantLog {
		t.Fatalf("log is %q, want %q", plan.BenchmarkLog, wantLog)
	}
	assertContainsSequence(t, plan.Commands[0].Command,
		"-run", "^$", "-bench", ".", "-benchmem", "-benchtime=450x",
		"./internal/component/bgp/reactor/...", "./internal/component/plugin")
	assertOverride(t, plan.Commands[0], "CGO_ENABLED=0")
}

// VALIDATES: allocation parsing strips the processor suffix, uses the worst
// repeated sample, accepts the ceiling boundary, and rejects both ceiling+1 and
// absent registered benchmarks.
// PREVENTS: a partial or repeated benchmark output passing the ceiling gate.
func TestAllocationVerdictsEnforceBoundaryWorstAndMissing(t *testing.T) {
	output := "BenchmarkBoundary-4 300 5 allocs/op\n" +
		"BenchmarkWorst-4 300 1 allocs/op\n" +
		"BenchmarkWorst-4 300 7 allocs/op\n"
	verdicts, err := allocationVerdicts(output, map[string]int{
		"BenchmarkBoundary": 5,
		"BenchmarkWorst":    6,
		"BenchmarkMissing":  0,
	})
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]AllocationVerdict, len(verdicts))
	for _, verdict := range verdicts {
		byName[verdict.Name] = verdict
	}
	if !byName["BenchmarkBoundary"].Passed {
		t.Errorf("ceiling boundary failed: %+v", byName["BenchmarkBoundary"])
	}
	if byName["BenchmarkWorst"].Passed || byName["BenchmarkWorst"].AllocsPerOp != 7 {
		t.Errorf("worst sample did not fail: %+v", byName["BenchmarkWorst"])
	}
	if !byName["BenchmarkMissing"].Missing || byName["BenchmarkMissing"].Passed {
		t.Errorf("missing benchmark did not fail closed: %+v", byName["BenchmarkMissing"])
	}
}

// VALIDATES: the allocation stage streams and logs the benchmark bytes before
// applying the native ceiling verdict.
// PREVENTS: parsing a different output than the operator saw or silently losing
// the permanent benchmark log.
func TestAllocRunLogsAndEnforcesTheSameOutput(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, "internal/component/bgp/reactor")
	mkdir(t, root, "internal/component/plugin")
	deps := fakeDependencies(root)
	deps.ceilings = func() map[string]int { return map[string]int{"BenchmarkHot": 0} }
	const benchmark = "BenchmarkHot-4 300 1 allocs/op\n"
	deps.execute = func(_ context.Context, plan CommandPlan, writer io.Writer) (string, ChildReport) {
		if writer != nil {
			if _, err := io.WriteString(writer, benchmark); err != nil {
				t.Fatalf("write benchmark output: %v", err)
			}
		}
		return benchmark, childFrom(plan, 0)
	}
	report, code := run(context.Background(), root, VerbAlloc, deps)
	if code != 1 || len(report.Allocations) != 1 || report.Allocations[0].Passed {
		t.Fatalf("allocation regression report is %+v, code %d", report, code)
	}
	logged, err := os.ReadFile(filepath.Join(root, "tmp", "verify", "alloc-gate-bench.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(logged) != benchmark {
		t.Fatalf("logged %q, want %q", logged, benchmark)
	}
}

// VALIDATES: action dispatch reaches each commissioned verb without deriving a
// Make target name.
// PREVENTS: a registered root whose action table cannot invoke its runners.
func TestEveryActionDispatches(t *testing.T) {
	seen := make(map[string]bool)
	area := actionTable(func(_ context.Context, _ string, verb string) (Report, int) {
		seen[verb] = true
		return Report{Verb: verb}, 0
	})
	for _, verb := range []string{VerbEvidenceVet, VerbVulnerability, VerbUnitCached, VerbUnitRaceChanged, VerbAlloc} {
		if _, code := area.Answer([]string{verb}); code != 0 {
			t.Fatalf("%s answered %d", verb, code)
		}
		if !seen[verb] {
			t.Errorf("%s did not reach its runner", verb)
		}
	}
}

type scriptedResult struct {
	output string
	code   int
}

func scriptedExecutor(results map[string]scriptedResult) commandExecutor {
	return func(_ context.Context, plan CommandPlan, writer io.Writer) (string, ChildReport) {
		key := strings.Join(plan.Command, " ")
		result, ok := results[key]
		if !ok {
			return "", childFrom(plan, 0)
		}
		if writer != nil && result.output != "" {
			_, _ = io.WriteString(writer, result.output)
		}
		return result.output, childFrom(plan, result.code)
	}
}

func fakeDependencies(root string) dependencies {
	chain := gotoolchain.Toolchain{
		Root:        root,
		Features:    []string{"ze_bgp", "ze_web"},
		GoToolchain: "go1.27.0",
		Procs:       4,
		Timeout:     "20m",
	}
	return dependencies{
		toolchain: func(string) (gotoolchain.Toolchain, error) { return chain, nil },
		module:    func(string) (string, error) { return testModulePath, nil },
		execute:   scriptedExecutor(nil),
		lookPath:  func(string) (string, error) { return "/tools/govulncheck", nil },
		getenv:    func(string) string { return "" },
		ceilings:  func() map[string]int { return map[string]int{} },
	}
}

func childFrom(plan CommandPlan, code int) ChildReport {
	return ChildReport{
		Name:      plan.Name,
		Command:   slices.Clone(plan.Command),
		Overrides: slices.Clone(plan.Overrides),
		Code:      code,
	}
}

func assertOverride(t *testing.T, plan CommandPlan, want string) {
	t.Helper()
	if !slices.Contains(plan.Overrides, want) {
		t.Errorf("overrides are %v, want %q", plan.Overrides, want)
	}
}

func assertNoOverride(t *testing.T, plan CommandPlan, prefix string) {
	t.Helper()
	for _, got := range plan.Overrides {
		if strings.HasPrefix(got, prefix) {
			t.Errorf("override %q unexpectedly has prefix %q", got, prefix)
		}
	}
}

func assertContainsSequence(t *testing.T, values []string, sequence ...string) {
	t.Helper()
	position := 0
	for _, value := range values {
		if position < len(sequence) && value == sequence[position] {
			position++
		}
	}
	if position != len(sequence) {
		t.Errorf("%v does not contain ordered sequence %v", values, sequence)
	}
}

func mkdir(t *testing.T, root, relative string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(relative)), 0o755); err != nil {
		t.Fatal(err)
	}
}
