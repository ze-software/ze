// Design: docs/architecture/testing/verify-freshness-scope.md -- race coverage execution.
package verifydeps

import (
	"context"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// TestRaceChangedExecutesTheWholeTreeAFullRunPublished drives the stage's
// execution seam, not just its plan, for a clean tree under the `./...` change
// set a full verify run publishes.
func TestRaceChangedExecutesTheWholeTreeAFullRunPublished(t *testing.T) {
	root := t.TempDir()
	anchoredCheckout(t, root)
	publishWholeTree(t, root)
	deps := fakeDependencies(root)
	var executed [][]string
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		if plan.Command[0] == "git" {
			return "", childFrom(plan, 0)
		}
		executed = append(executed, plan.Command)
		if !slices.Contains(plan.Command, "-race") {
			t.Fatalf("whole-tree selection executed a non-race command: %v", plan.Command)
		}
		if !slices.Contains(plan.Environment, "CGO_ENABLED=1") {
			t.Fatal("race execution has no cgo")
		}
		return "", childFrom(plan, 0)
	}
	report, code := run(context.Background(), root, VerbUnitRaceChanged, verifyengine.Uncut(), deps)
	if code != 0 || report.Skipped {
		t.Fatalf("whole tree answered code=%d skipped=%v: %+v", code, report.Skipped, report)
	}
	if len(executed) != 2 {
		t.Fatalf("executed %v, want full race and compile-out populations", executed)
	}
	if !slices.Contains(executed[0], "./...") {
		t.Fatalf("whole-tree race population is %v", executed[0])
	}
	if !slices.Contains(executed[1], "./cmd/ze/hub") {
		t.Fatalf("compile-out population is %v", executed[1])
	}
}

// TestRaceChangedRefusesWithNoAnchor proves a checkout whose branch has no
// upstream and no origin/HEAD races nothing: the stage fails naming the fixing
// command, and never falls back to the whole tree.
func TestRaceChangedRefusesWithNoAnchor(t *testing.T) {
	root := t.TempDir()
	anchoredCheckout(t, root)
	deps := fakeDependencies(root)
	var executed [][]string
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		if plan.Command[0] != "git" {
			executed = append(executed, plan.Command)
		}
		// The checkout is on a branch: with HEAD detached the refusal names
		// git remote set-head instead (repochanged.noUpstreamError).
		if strings.Join(plan.Command, " ") == anchorBranchQuery {
			return "main\n", childFrom(plan, 0)
		}
		return "", childFrom(plan, 0)
	}
	report, code := run(context.Background(), root, VerbUnitRaceChanged, verifyengine.Uncut(), deps)
	if code == 0 || report.Skipped {
		t.Fatalf("no anchor answered code=%d skipped=%v, want a failure", code, report.Skipped)
	}
	if !strings.Contains(report.Error, "git branch --set-upstream-to=origin/") {
		t.Fatalf("refusal %q names no fixing command", report.Error)
	}
	if len(executed) != 0 {
		t.Fatalf("no anchor executed %v, want nothing", executed)
	}
}

// TestRaceChangedExecutesGoCommittedBeforeClosure keeps all working-tree queries
// empty and supplies the earlier Go edit only through the baseline-to-HEAD diff.
func TestRaceChangedExecutesGoCommittedBeforeClosure(t *testing.T) {
	root := t.TempDir()
	anchoredCheckout(t, root)
	deps := fakeDependencies(root)
	var executed [][]string
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		if out, ok := anchorAnswer(plan); ok {
			return out, childFrom(plan, 0)
		}
		switch strings.Join(plan.Command, " ") {
		case "git diff --name-only anchor HEAD -- *.go":
			return "internal/component/bgp/wire/update.go\n", childFrom(plan, 0)
		}
		if slices.Contains(plan.Command, "-race") {
			executed = append(executed, plan.Command)
		}
		return "", childFrom(plan, 0)
	}
	report, code := run(context.Background(), root, VerbUnitRaceChanged, verifyengine.Uncut(), deps)
	if code != 0 || report.Skipped || len(executed) != 2 {
		t.Fatalf("closure answered code=%d, report=%+v, executions=%v", code, report, executed)
	}
	if !slices.Contains(executed[0], "./internal/component/bgp/...") {
		t.Fatalf("earlier Go edit was not raced: %v", executed[0])
	}
	if slices.Contains(executed[0], "./...") {
		t.Fatalf("verified baseline unnecessarily widened: %v", executed[0])
	}
}

// TestRaceChangedRunsCompileOutAfterFailure proves a red full race population
// cannot hide the compile-out judgment or have its failure replaced by it.
func TestRaceChangedRunsCompileOutAfterFailure(t *testing.T) {
	root := t.TempDir()
	anchoredCheckout(t, root)
	publishWholeTree(t, root)
	deps := fakeDependencies(root)
	calls := 0
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		if out, ok := anchorAnswer(plan); ok {
			return out, childFrom(plan, 0)
		}
		if plan.Command[0] == "git" {
			return "", childFrom(plan, 0)
		}
		calls++
		code := 23
		if strings.HasSuffix(plan.Name, ":core") {
			code = 19
		}
		return "", childFrom(plan, code)
	}
	report, code := run(context.Background(), root, VerbUnitRaceChanged, verifyengine.Uncut(), deps)
	if code != 23 || report.Code != 23 || calls != 2 {
		t.Fatalf("failure answered code=%d report=%+v calls=%d", code, report, calls)
	}
}

// TestRaceChangedRefusesPartiallyUnresolvedPopulation supplies one valid group
// beside a directory go list dropped and asserts that no test certifies it.
func TestRaceChangedRefusesPartiallyUnresolvedPopulation(t *testing.T) {
	root := t.TempDir()
	anchoredCheckout(t, root)
	deps := fakeDependencies(root)
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		if out, ok := anchorAnswer(plan); ok {
			return out, childFrom(plan, 0)
		}
		switch strings.Join(plan.Command, " ") {
		case "git diff --name-only -- *.go":
			return "internal/core/env/env.go\ninternal/le/gone/gone.go\n", childFrom(plan, 0)
		}
		if slices.Contains(plan.Command, "-race") {
			t.Fatalf("unresolved population reached execution: %v", plan.Command)
		}
		return "", childFrom(plan, 0)
	}
	report, code := run(context.Background(), root, VerbUnitRaceChanged, verifyengine.Uncut(), deps)
	if code == 0 || report.Skipped || !strings.Contains(report.Error, "./internal/le/gone") {
		t.Fatalf("partial population answered code=%d report=%+v", code, report)
	}
}

// TestRaceChangedRefusesAnUnreadableCommittedRange makes only the historical
// query fail, so successful empty working-tree queries cannot certify a skip.
func TestRaceChangedRefusesAnUnreadableCommittedRange(t *testing.T) {
	root := t.TempDir()
	anchoredCheckout(t, root)
	deps := fakeDependencies(root)
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		if out, ok := anchorAnswer(plan); ok {
			return out, childFrom(plan, 0)
		}
		switch strings.Join(plan.Command, " ") {
		case "git diff --name-only anchor HEAD -- *.go":
			return "", childFrom(plan, 7)
		}
		if slices.Contains(plan.Command, "-race") {
			t.Fatalf("unreadable history reached execution: %v", plan.Command)
		}
		return "", childFrom(plan, 0)
	}
	report, code := run(context.Background(), root, VerbUnitRaceChanged, verifyengine.Uncut(), deps)
	if code != 7 || report.Skipped || report.Error == "" {
		t.Fatalf("unreadable history answered code=%d report=%+v", code, report)
	}
}

// publishScope publishes packages as the run's package answer for root.
func publishScope(t *testing.T, root string, packages ...string) {
	t.Helper()
	path := filepath.Join(root, "tmp", "scope-packages.txt")
	if err := repochanged.WriteScopePackages(path, root, packages); err != nil {
		t.Fatal(err)
	}
	t.Setenv(repochanged.ScopeFileKey, path)
	env.ResetCache()
}

// TestUnitChangedTestsAndVetsThePublishedScope proves the scoped unit pass
// tests and vets exactly the packages the run published, then compiles the
// bare core, and that an empty scope runs no command at all: an empty package
// list handed to go test would test the current directory (AC-13).
func TestUnitChangedTestsAndVetsThePublishedScope(t *testing.T) {
	root := t.TempDir()
	anchoredCheckout(t, root)
	publishScope(t, root, "./internal/le/verify")
	deps := fakeDependencies(root)
	var executed [][]string
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		executed = append(executed, plan.Command)
		return "", childFrom(plan, 0)
	}
	report, code := run(context.Background(), root, VerbUnitChanged, verifyengine.Uncut(), deps)
	if code != 0 {
		t.Fatalf("unit-changed exited %d: %#v", code, report)
	}
	if len(executed) != 3 {
		t.Fatalf("executed %v, want test, vet and the core compile-out", executed)
	}
	for index, verb := range []string{"test", "vet"} {
		if !slices.Contains(executed[index], verb) || !slices.Contains(executed[index], "./internal/le/verify") ||
			slices.Contains(executed[index], "./...") {
			t.Errorf("command %d is %v, want go %s over the published package only", index, executed[index], verb)
		}
	}
	if !slices.Contains(executed[2], "./cmd/ze/hub") {
		t.Errorf("compile-out command is %v", executed[2])
	}

	publishScope(t, root)
	executed = nil
	if _, code := run(context.Background(), root, VerbUnitChanged, verifyengine.Uncut(), deps); code != 0 {
		t.Fatalf("unit-changed over an empty scope exited %d", code)
	}
	if len(executed) != 0 {
		t.Errorf("an empty scope executed %v, want nothing", executed)
	}
}
