// Design: docs/architecture/testing/verify-freshness-scope.md -- race coverage execution.
package verifydeps

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestRaceChangedExecutesTheWholeTreeWithoutGreenEvidence drives the stage's
// execution seam, not just its plan, for a clean tree with no green baseline.
func TestRaceChangedExecutesTheWholeTreeWithoutGreenEvidence(t *testing.T) {
	for _, certificate := range []string{"", "exit=1\ngit_sha=verified\n", "exit=0\n"} {
		t.Run(certificate, func(t *testing.T) {
			root := t.TempDir()
			verifiedBaseline(t, root)
			status := filepath.Join(root, "tmp", "ze-verify.status")
			if certificate == "" {
				if err := os.Remove(status); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(status, []byte(certificate), 0o600); err != nil {
				t.Fatal(err)
			}
			deps := fakeDependencies(root)
			var executed [][]string
			deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
				if plan.Command[0] == "git" {
					return "", childFrom(plan, 0)
				}
				executed = append(executed, plan.Command)
				if !slices.Contains(plan.Command, "-race") {
					t.Fatalf("cold selection executed a non-race command: %v", plan.Command)
				}
				if !slices.Contains(plan.Environment, "CGO_ENABLED=1") {
					t.Fatal("race execution has no cgo")
				}
				return "", childFrom(plan, 0)
			}
			report, code := run(context.Background(), root, VerbUnitRaceChanged, deps)
			if code != 0 || report.Skipped {
				t.Fatalf("cold tree answered code=%d skipped=%v: %+v", code, report.Skipped, report)
			}
			if len(executed) != 2 {
				t.Fatalf("executed %v, want full race and compile-out populations", executed)
			}
			if !slices.Contains(executed[0], "./...") {
				t.Fatalf("cold race population is %v", executed[0])
			}
			if !slices.Contains(executed[1], "./cmd/ze/hub") {
				t.Fatalf("compile-out population is %v", executed[1])
			}
		})
	}
}

// TestRaceChangedExecutesGoCommittedBeforeClosure keeps all working-tree queries
// empty and supplies the earlier Go edit only through the baseline-to-HEAD diff.
func TestRaceChangedExecutesGoCommittedBeforeClosure(t *testing.T) {
	root := t.TempDir()
	verifiedBaseline(t, root)
	deps := fakeDependencies(root)
	var executed [][]string
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		switch strings.Join(plan.Command, " ") {
		case "git cat-file -t verified":
			return "commit\n", childFrom(plan, 0)
		case "git diff --name-only verified HEAD -- *.go":
			return "internal/component/bgp/wire/update.go\n", childFrom(plan, 0)
		}
		if slices.Contains(plan.Command, "-race") {
			executed = append(executed, plan.Command)
		}
		return "", childFrom(plan, 0)
	}
	report, code := run(context.Background(), root, VerbUnitRaceChanged, deps)
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
	verifiedBaseline(t, root)
	if err := os.Remove(filepath.Join(root, "tmp", "ze-verify.status")); err != nil {
		t.Fatal(err)
	}
	deps := fakeDependencies(root)
	calls := 0
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
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
	report, code := run(context.Background(), root, VerbUnitRaceChanged, deps)
	if code != 23 || report.Code != 23 || calls != 2 {
		t.Fatalf("failure answered code=%d report=%+v calls=%d", code, report, calls)
	}
}

// TestRaceChangedRefusesPartiallyUnresolvedPopulation supplies one valid group
// beside a directory go list dropped and asserts that no test certifies it.
func TestRaceChangedRefusesPartiallyUnresolvedPopulation(t *testing.T) {
	root := t.TempDir()
	verifiedBaseline(t, root)
	deps := fakeDependencies(root)
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		switch strings.Join(plan.Command, " ") {
		case "git cat-file -t verified":
			return "commit\n", childFrom(plan, 0)
		case "git diff --name-only -- *.go":
			return "internal/core/env/env.go\ninternal/le/gone/gone.go\n", childFrom(plan, 0)
		}
		if slices.Contains(plan.Command, "-race") {
			t.Fatalf("unresolved population reached execution: %v", plan.Command)
		}
		return "", childFrom(plan, 0)
	}
	report, code := run(context.Background(), root, VerbUnitRaceChanged, deps)
	if code == 0 || report.Skipped || !strings.Contains(report.Error, "./internal/le/gone") {
		t.Fatalf("partial population answered code=%d report=%+v", code, report)
	}
}

// TestRaceChangedRefusesAnUnreadableCommittedRange makes only the historical
// query fail, so successful empty working-tree queries cannot certify a skip.
func TestRaceChangedRefusesAnUnreadableCommittedRange(t *testing.T) {
	root := t.TempDir()
	verifiedBaseline(t, root)
	deps := fakeDependencies(root)
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		switch strings.Join(plan.Command, " ") {
		case "git cat-file -t verified":
			return "commit\n", childFrom(plan, 0)
		case "git diff --name-only verified HEAD -- *.go":
			return "", childFrom(plan, 7)
		}
		if slices.Contains(plan.Command, "-race") {
			t.Fatalf("unreadable history reached execution: %v", plan.Command)
		}
		return "", childFrom(plan, 0)
	}
	report, code := run(context.Background(), root, VerbUnitRaceChanged, deps)
	if code != 7 || report.Skipped || report.Error == "" {
		t.Fatalf("unreadable history answered code=%d report=%+v", code, report)
	}
}
