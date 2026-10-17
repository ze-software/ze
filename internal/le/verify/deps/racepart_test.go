// Design: docs/architecture/testing/verify-freshness-scope.md -- the race pass cut into pieces.
package verifydeps

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// racePieceDeps answers dependencies for a clean tree with no green baseline,
// whose changed population is therefore ./..., and whose `go list` of that
// population answers the given packages.
func racePieceDeps(t *testing.T, packages []string, executed *[][]string) (string, dependencies) {
	t.Helper()
	root := t.TempDir()
	verifiedBaseline(t, root)
	if err := os.Remove(filepath.Join(root, "tmp", "ze-verify.status")); err != nil {
		t.Fatal(err)
	}
	deps := fakeDependencies(root)
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		if plan.Command[0] == "git" {
			return "", childFrom(plan, 0)
		}
		if plan.Command[1] == "list" {
			return strings.Join(packages, "\n") + "\n", childFrom(plan, 0)
		}
		*executed = append(*executed, plan.Command)
		return "", childFrom(plan, 0)
	}
	return root, deps
}

// TestRacePiecesRunEveryPackageExactlyOnce cuts a whole-tree race pass into
// three pieces and collects what each piece executes. Thirty packages give
// every piece a share under the hash deal.
// VALIDATES: the pieces together race every listed package exactly once, the
// ./... pattern is expanded rather than handed whole to one piece, the list
// carries the race command's tag set, and only piece 1 runs the core check.
// PREVENTS: a cut whose pieces each rerun the whole tree (no faster), or drop
// a package (a pass over less than the change).
func TestRacePiecesRunEveryPackageExactlyOnce(t *testing.T) {
	packages := make([]string, 0, 30)
	for index := range 30 {
		packages = append(packages, "m/p"+strconv.Itoa(index))
	}
	raced := map[string]int{}
	coreRuns := 0
	for index := 1; index <= 3; index++ {
		var executed [][]string
		root, deps := racePieceDeps(t, packages, &executed)
		part := verifyengine.Part{Index: index, Of: 3}
		report, code := run(context.Background(), root, VerbUnitRaceChanged, part, deps)
		if code != 0 || report.Skipped {
			t.Fatalf("piece %d answered %d skipped=%v: %s", index, code, report.Skipped, report.Error)
		}
		if report.Part != part {
			t.Errorf("piece %d reported part %+v", index, report.Part)
		}
		listed := false
		for _, child := range report.Children {
			if child.Name == "race-package-population" {
				listed = true
				for _, flag := range []string{"-e", "-tags", "-race", "./..."} {
					if !slices.Contains(child.Command, flag) {
						t.Errorf("piece %d listed without %s: %v", index, flag, child.Command)
					}
				}
			}
		}
		if !listed {
			t.Errorf("piece %d raced without listing the population", index)
		}
		for _, command := range executed {
			if slices.Contains(command, "./cmd/ze/hub") {
				coreRuns++
				continue
			}
			if slices.Contains(command, "./...") {
				t.Errorf("piece %d raced the unexpanded pattern: %v", index, command)
			}
			for _, pkg := range packages {
				if slices.Contains(command, pkg) {
					raced[pkg]++
				}
			}
		}
		if index != 1 && len(executed) != 1 {
			t.Errorf("piece %d executed %d commands, want only its race share", index, len(executed))
		}
	}
	for _, pkg := range packages {
		if raced[pkg] != 1 {
			t.Errorf("package %s raced %d times across the pieces, want 1", pkg, raced[pkg])
		}
	}
	if coreRuns != 1 {
		t.Errorf("the core check ran %d times across the pieces, want 1", coreRuns)
	}
}

// TestAnEmptyRacePieceSaysItRanNothing cuts two packages into three pieces.
// VALIDATES: the piece the deal leaves empty executes nothing and reports a
// stated skip. PREVENTS: an empty piece reading as a pass over tests that ran.
func TestAnEmptyRacePieceSaysItRanNothing(t *testing.T) {
	var executed [][]string
	root, deps := racePieceDeps(t, []string{"m/a", "m/b"}, &executed)
	report, code := run(context.Background(), root, VerbUnitRaceChanged, verifyengine.Part{Index: 3, Of: 3}, deps)
	if code != 0 || !report.Skipped || report.Reason == "" {
		t.Fatalf("empty piece answered %d skipped=%v reason=%q", code, report.Skipped, report.Reason)
	}
	if len(executed) != 0 {
		t.Fatalf("empty piece executed %v", executed)
	}
}

// TestARacePieceRefusesAnUnreadablePopulation makes the package list fail.
// VALIDATES: the failure code reaches the verdict. PREVENTS: a failed list
// read as an empty piece, which would skip and answer 0.
func TestARacePieceRefusesAnUnreadablePopulation(t *testing.T) {
	var executed [][]string
	root, deps := racePieceDeps(t, nil, &executed)
	deps.execute = func(_ context.Context, plan CommandPlan, _ io.Writer) (string, ChildReport) {
		if plan.Command[1] == "list" {
			return "", childFrom(plan, 5)
		}
		return "", childFrom(plan, 0)
	}
	report, code := run(context.Background(), root, VerbUnitRaceChanged, verifyengine.Part{Index: 1, Of: 2}, deps)
	if code != 5 || report.Error == "" || report.Skipped {
		t.Fatalf("unreadable population answered %d error=%q skipped=%v", code, report.Error, report.Skipped)
	}
	if !strings.Contains(report.Error, "rerun: ./le verify deps unit-race-changed part 1 of 2") {
		t.Errorf("the refusal names no rerun command: %q", report.Error)
	}
}

// TestAnEmptyRaceListNamesItsFix makes the package list print nothing.
// VALIDATES: the refusal says the pattern matched no directory and gives the
// rerun command. PREVENTS: an empty list read as an empty, passing piece.
func TestAnEmptyRaceListNamesItsFix(t *testing.T) {
	var executed [][]string
	root, deps := racePieceDeps(t, nil, &executed)
	report, code := run(context.Background(), root, VerbUnitRaceChanged, verifyengine.Part{Index: 2, Of: 3}, deps)
	if code != 1 || report.Skipped || len(executed) != 0 {
		t.Fatalf("empty list answered %d skipped=%v executed=%v", code, report.Skipped, executed)
	}
	for _, want := range []string{"names no package", "rerun: ./le verify deps unit-race-changed part 2 of 3"} {
		if !strings.Contains(report.Error, want) {
			t.Errorf("refusal %q lacks %q", report.Error, want)
		}
	}
}

// TestRacePiecesDealByPathNotPosition gives two pieces of one cut different
// lists, as when another session changes the tree between them.
// VALIDATES: every package both pieces saw is raced exactly once.
// PREVENTS: a positional deal shifting the shares, so a package present in
// both lists is raced by neither piece or by both.
func TestRacePiecesDealByPathNotPosition(t *testing.T) {
	before := make([]string, 0, 20)
	for index := range 20 {
		before = append(before, "m/p"+strconv.Itoa(index))
	}
	after := append([]string{"m/added-first"}, slices.Delete(slices.Clone(before), 3, 4)...)
	raced := map[string]int{}
	for index, packages := range [][]string{before, after} {
		var executed [][]string
		root, deps := racePieceDeps(t, packages, &executed)
		if _, code := run(context.Background(), root, VerbUnitRaceChanged, verifyengine.Part{Index: index + 1, Of: 2}, deps); code != 0 {
			t.Fatalf("piece %d answered %d", index+1, code)
		}
		for _, command := range executed {
			for _, pkg := range packages {
				if slices.Contains(command, pkg) {
					raced[pkg]++
				}
			}
		}
	}
	for _, pkg := range before {
		if !slices.Contains(after, pkg) {
			continue
		}
		if raced[pkg] != 1 {
			t.Errorf("package %s, in both lists, raced %d times, want 1", pkg, raced[pkg])
		}
	}
}

// TestTheRacePassIsNeverSwept names unit-race-changed beside another verb.
// VALIDATES: the line is refused with code 2 and nothing runs. PREVENTS: a
// sweep silently dropping the race pass, or running it with a verb read as
// its keywords.
func TestTheRacePassIsNeverSwept(t *testing.T) {
	ran := 0
	area := actionTable(func(context.Context, string, string, verifyengine.Part) (Report, int) {
		ran++
		return Report{}, 0
	})
	for _, line := range [][]string{{VerbUnitRaceChanged, VerbAlloc}, {VerbAlloc, VerbUnitRaceChanged}} {
		if _, code := area.AnswerOrSweep(line, leaction.RunEveryAction); code != 2 {
			t.Fatalf("a sweep %v naming the race pass answered %d, want 2", line, code)
		}
	}
	if ran != 0 {
		t.Fatalf("a refused sweep ran %d actions", ran)
	}
}

// TestOnlyTheRacePassAcceptsACut asks the cached pass for a piece.
// VALIDATES: a cut of a verb with no dealt population is refused with code 2.
// PREVENTS: a stage that judged a subset reporting under the whole stage name.
func TestOnlyTheRacePassAcceptsACut(t *testing.T) {
	var executed [][]string
	root, deps := racePieceDeps(t, nil, &executed)
	_, code := run(context.Background(), root, VerbUnitCached, verifyengine.Part{Index: 1, Of: 2}, deps)
	if code != 2 {
		t.Fatalf("a cut unit-cached answered %d, want 2", code)
	}
	if len(executed) != 0 {
		t.Fatalf("a refused cut executed %v", executed)
	}
}

// TestPartKeywordsArePairedAndCounted drives the keyword parser.
// VALIDATES: no keyword is the uncut population, both keywords name a piece,
// and one keyword alone, a zero, or an index past the count is refused.
func TestPartKeywordsArePairedAndCounted(t *testing.T) {
	cases := []struct {
		args leaction.Arguments
		want verifyengine.Part
		ok   bool
	}{
		{leaction.Arguments{}, verifyengine.Uncut(), true},
		{leaction.Arguments{"part": {"2"}, "of": {"3"}}, verifyengine.Part{Index: 2, Of: 3}, true},
		{leaction.Arguments{"part": {"2"}}, verifyengine.Part{}, false},
		{leaction.Arguments{"of": {"3"}}, verifyengine.Part{}, false},
		{leaction.Arguments{"part": {"0"}, "of": {"3"}}, verifyengine.Part{}, false},
		{leaction.Arguments{"part": {"4"}, "of": {"3"}}, verifyengine.Part{}, false},
		{leaction.Arguments{"part": {"x"}, "of": {"3"}}, verifyengine.Part{}, false},
	}
	for _, testCase := range cases {
		got, err := partFrom(testCase.args)
		if (err == nil) != testCase.ok {
			t.Errorf("partFrom(%v) error = %v, want ok=%v", testCase.args, err, testCase.ok)
			continue
		}
		if got != testCase.want {
			t.Errorf("partFrom(%v) = %+v, want %+v", testCase.args, got, testCase.want)
		}
	}
}

// TestTheCoreCheckRunsWhenPieceOneHoldsNoPackage lists only packages that
// hash away from piece 1, then runs all three pieces.
// VALIDATES: the bare ze_core check, planned only in piece 1, still runs
// exactly once. PREVENTS: an empty piece-1 share skipping the core check in
// every piece while the run reports green.
func TestTheCoreCheckRunsWhenPieceOneHoldsNoPackage(t *testing.T) {
	var packages []string
	for index := range 40 {
		pkg := "m/p" + strconv.Itoa(index)
		if racePieceOf(pkg, 3) != 1 {
			packages = append(packages, pkg)
		}
	}
	if len(packages) == 0 {
		t.Fatal("no candidate hashes away from piece 1")
	}
	coreRuns := 0
	for index := 1; index <= 3; index++ {
		var executed [][]string
		root, deps := racePieceDeps(t, packages, &executed)
		report, code := run(context.Background(), root, VerbUnitRaceChanged, verifyengine.Part{Index: index, Of: 3}, deps)
		if code != 0 {
			t.Fatalf("piece %d answered %d: %s", index, code, report.Error)
		}
		for _, command := range executed {
			if slices.Contains(command, "./cmd/ze/hub") {
				coreRuns++
			}
		}
		if index == 1 && report.Skipped {
			t.Errorf("piece 1 reported a skip although it owes the core check: %s", report.Reason)
		}
	}
	if coreRuns != 1 {
		t.Fatalf("the core check ran %d times across the pieces, want 1", coreRuns)
	}
}
