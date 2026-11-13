// Design: docs/architecture/testing/qemu-integration.md -- selected native stress.

package testqemu

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"slices"
	"testing"
	"time"

	teststressrepro "github.com/ze-software/ze/internal/le/test/stressrepro"
)

func TestSelectedStressActionGrammar(t *testing.T) {
	args, err := actions.Parse("all", []string{"test", "test/draft/plugin/selected.ci", "stress", "80", "stress-parallel", "4"})
	if err != nil {
		t.Fatalf("registered selected-stress grammar refused: %v", err)
	}
	stress, err := selectedStressOf(args)
	if err != nil {
		t.Fatal(err)
	}
	if stress == nil {
		t.Fatal("registered stress invocation became ordinary execution")
	}
	if stress.iterations != 80 || stress.parallel != 4 {
		t.Fatalf("stress request = %+v, want iterations=80 parallel=4", stress)
	}
}

func TestSelectedStressRefusesInvalidRequests(t *testing.T) {
	for _, words := range [][]string{
		{"stress", "80"},
		{"test", "test/plugin/a.ci", "stress", "0"},
		{"test", "test/plugin/a.ci", "stress", "1001"},
		{"test", "test/plugin/a.ci", "stress", "18446744073709551616"},
		{"test", "test/plugin/a.ci", "stress-parallel", "2"},
		{"test", "test/plugin/a.ci", "stress", "2", "stress-parallel", "0"},
		{"test", "test/plugin/a.ci", "stress", "2", "stress-parallel", "5"},
		{"test", "test/plugin/../a.ci", "stress", "2"},
		{"test", "test/plugin/.ci", "stress", "2"},
	} {
		args, err := actions.Parse("all", words)
		if err != nil {
			continue
		}
		if _, err := selectedStressOf(args); err == nil {
			t.Errorf("accepted invalid stress request %q", words)
		}
	}
}

func TestSelectedStressDefaultAndOrdinarySelection(t *testing.T) {
	for _, words := range [][]string{nil, {"test", "test/plugin/a.ci"}} {
		args, err := actions.Parse("all", words)
		if err != nil {
			t.Fatal(err)
		}
		stress, err := selectedStressOf(args)
		if err != nil {
			t.Fatal(err)
		}
		if stress != nil {
			t.Fatalf("ordinary selection gained stress: %+v", stress)
		}
	}
	args, err := actions.Parse("all", []string{"test", "test/plugin/a.ci", "stress", "1"})
	if err != nil {
		t.Fatal(err)
	}
	stress, err := selectedStressOf(args)
	if err != nil {
		t.Fatal(err)
	}
	if stress == nil {
		t.Fatal("missing bounded stress request")
	}
	if stress.parallel != 1 {
		t.Fatalf("default workers = %d, want 1", stress.parallel)
	}
}

func TestSelectedStressBudgetCoversEveryBatch(t *testing.T) {
	stress := &selectedStress{iterations: 80, parallel: 4}
	budget, err := stress.budget(900 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if budget <= 20*900*time.Second {
		t.Fatalf("budget %s does not cover 20 bounded batches and cleanup", budget)
	}
	for _, timeout := range []time.Duration{0, -1, time.Duration(math.MaxInt64)} {
		if budget, err := stress.budget(timeout); err == nil {
			t.Errorf("accepted invalid/overflowing timeout %s with budget %s", timeout, budget)
		}
	}
}

func TestSelectedStressVerdictRequiresExactSuccessfulPopulation(t *testing.T) {
	stress := &selectedStress{iterations: 80, parallel: 4}
	good := teststressrepro.Report{Iterations: 80, Completed: 80, Executed: 80}
	if err := selectedStressVerdict(stress, good, 1); err != nil {
		t.Fatalf("exact successful stress report refused: %v", err)
	}
	for _, change := range []func(*teststressrepro.Report){
		func(r *teststressrepro.Report) { r.Completed = 79 },
		func(r *teststressrepro.Report) { r.Completed = 81 },
		func(r *teststressrepro.Report) { r.Iterations = 79 },
		func(r *teststressrepro.Report) { r.Completed = 0 },
		func(r *teststressrepro.Report) { r.Executed = 0 },
		func(r *teststressrepro.Report) { r.Executed = 79 },
		func(r *teststressrepro.Report) { r.Interrupted = true },
		func(r *teststressrepro.Report) { r.Reproduced = true },
		func(r *teststressrepro.Report) { r.SetupError = "setup failed" },
		func(r *teststressrepro.Report) { r.Failed = 1 },
		func(r *teststressrepro.Report) { r.Exit = 1 },
	} {
		report := good
		change(&report)
		if err := selectedStressVerdict(stress, report, 1); err == nil {
			t.Errorf("accepted incomplete/failed report %+v", report)
		}
	}
	if err := selectedStressVerdict(stress, good, 2); err == nil {
		t.Fatal("accepted adapter setup exit despite successful-looking report")
	}
}

func TestSelectedStressCountRejectsEmptyAndSkippedChildren(t *testing.T) {
	for _, output := range []string{"", "no tests selected\n", "pass 0/0 100.0% 1s skipped 1\n", "pass 2/2 100.0% 1s\n", "fail 0/1 0.0% 1s\n"} {
		if count, err := selectedStressCount(output); err == nil {
			t.Errorf("accepted output %q with %d tests", output, count)
		}
	}
	if count, err := selectedStressCount("pass 1/1 100.0% 1s\n"); err != nil || count != 1 {
		t.Fatalf("one executed selected test = %d, %v", count, err)
	}
}

func TestSelectedStressReusesPreparedNamespace(t *testing.T) {
	run := vmFixture(t)
	run.Test = "test/draft/plugin/selected-stress.ci"
	run.Stress = &selectedStress{iterations: 80, parallel: 4}
	writeFile(t, run.Workspace, run.Test, "option=netns-link:name=zesrproof\ncmd=foreground:seq=1:exec=ze doctor\n")
	rec := &recorder{}
	run.Run = rec.run
	called := 0
	run.StressRun = func(_ context.Context, root string, request teststressrepro.SelectedRequest) (teststressrepro.Report, int) {
		called++
		if len(rec.calls) != 2 {
			t.Fatalf("stress began after %d commands, want namespace then conntrack only", len(rec.calls))
		}
		if root != run.Workspace {
			t.Errorf("adapter root = %q, want %q", root, run.Workspace)
		}
		if !slices.Equal(request.Runner, []string{"bgp", "plugin", "--draft"}) {
			t.Errorf("canonical runner = %q", request.Runner)
		}
		if request.Selector != "selected-stress" {
			t.Errorf("selector = %q", request.Selector)
		}
		if request.Executable != filepath.Join(netnsCapDir, leName) {
			t.Errorf("executable = %q, want prepared capability le", request.Executable)
		}
		suite, err := vmSuiteFor(run.Test)
		if err != nil {
			t.Fatal(err)
		}
		suite.Namespace = perTest
		if !slices.Equal(request.Environment, run.suiteEnvironment(suite, run.environment())) {
			t.Errorf("adapter did not receive exact prepared suite environment: %q", request.Environment)
		}
		if request.Iterations != 80 || request.Parallel != 4 || request.Budget <= 900*time.Second || request.Timeout != 900*time.Second {
			t.Errorf("unbounded or incomplete adapter request: %+v", request)
		}
		if request.Count == nil {
			t.Fatal("adapter has no runner-owned count admission")
		}
		return teststressrepro.Report{Iterations: 80, Completed: 80, Executed: 80}, 1
	}
	report, code := run.Execute()
	if code != 0 {
		t.Fatalf("prepared native stress exited %d: %+v", code, report)
	}
	if called != 1 {
		t.Fatalf("stress adapter called %d times, want once", called)
	}
	if len(rec.calls) != 2 {
		t.Fatalf("ordinary suite or duplicate preparation ran: %q", rec.calls)
	}
	if len(report.Phases) != 1 || report.Phases[0].Tests != 80 {
		t.Fatalf("actual stress population missing from phase report: %+v", report)
	}
}

func TestSelectedStressPreparationFailureNeverLaunches(t *testing.T) {
	run := vmFixture(t)
	run.Test = "test/draft/plugin/selected-stress.ci"
	run.Stress = &selectedStress{iterations: 80, parallel: 4}
	writeFile(t, run.Workspace, run.Test, "option=netns-link:name=zesrproof\ncmd=foreground:seq=1:exec=ze doctor\n")
	run.Look = func(string) (string, error) { return "", errors.New("missing namespace tool") }
	run.StressRun = func(context.Context, string, teststressrepro.SelectedRequest) (teststressrepro.Report, int) {
		t.Fatal("stress launched before namespace tool admission")
		return teststressrepro.Report{}, 2
	}
	if _, code := run.Execute(); code == 0 {
		t.Fatal("missing preparation tools accepted")
	}
}
