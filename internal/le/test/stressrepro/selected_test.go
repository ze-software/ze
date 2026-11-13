// Design: docs/functional-tests.md -- native canonical selection under stress.

package teststressrepro

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func nativeRequest(t *testing.T, root string) SelectedRequest {
	t.Helper()
	le := filepath.Join(root, "le")
	if err := os.WriteFile(le, []byte("prepared capability le"), 0o755); err != nil {
		t.Fatal(err)
	}
	return SelectedRequest{
		Suite: "plugin", Runner: []string{"bgp", "plugin", "--draft"}, Selector: "native-selected",
		Executable: le, Environment: []string{"ZE_BIN=" + filepath.Join(root, "bin", "ze"), "PATH=/prepared/bin:/usr/bin", "ZE_RUN_JOB=/prepared/parent", "ZE_TEST_NETNS=1"},
		Iterations: 4, Parallel: 2, Timeout: time.Second, Budget: time.Minute,
		Count: func(output string) (int, error) {
			if output != "selected test passed\n" {
				return 0, errors.New("selected test did not execute")
			}
			return 1, nil
		},
	}
}

func TestNativeSelectedAdapterKeepsCanonicalRunnerAndPreparedBinary(t *testing.T) {
	root := stressTree(t)
	request := nativeRequest(t, root)
	fake := &fakeRunner{results: []processResult{{output: "selected test passed\n"}, {output: "selected test passed\n"}, {output: "selected test passed\n"}, {output: "selected test passed\n"}}}
	deps, stopped := testDependencies(fake)
	report, code := runSelected(context.Background(), root, request, deps)
	if code != 1 || report.SetupError != "" || report.Reproduced || report.Completed != 4 || report.Executed != 4 {
		t.Fatalf("native selected report/code = %+v/%d", report, code)
	}
	if *stopped != 1 {
		t.Fatalf("burners joined %d times, want once", *stopped)
	}
	if fake.buildOutput != "" || len(fake.pluginBuilds) != 0 {
		t.Fatal("native adapter rebuilt a pinned prepared binary")
	}
	if len(fake.invocations) != 4 {
		t.Fatalf("native selection ran %d times", len(fake.invocations))
	}
	for _, invocation := range fake.invocations {
		argv, err := invocation.argv()
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(argv, []string{"test", "bgp", "plugin", "--draft", "-v", "native-selected"}) {
			t.Errorf("canonical child argv = %q", argv)
		}
		if invocation.testBin != request.Executable || invocation.zeBin != filepath.Join(root, "bin", "ze") {
			t.Errorf("prepared binaries were replaced: %+v", invocation)
		}
	}
	log, err := os.ReadFile(report.Log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), `argv=["test","bgp","plugin","--draft","-v","native-selected"]`) {
		t.Fatalf("capture lost exact child argv: %s", log)
	}
}

func TestNativeSelectedAdapterRejectsUnexecutedChild(t *testing.T) {
	for _, output := range []string{"", "pass 0/0 100.0% 0s skip 1\n", "no tests selected\n"} {
		root := stressTree(t)
		request := nativeRequest(t, root)
		fake := &fakeRunner{results: []processResult{{output: output}}}
		deps, _ := testDependencies(fake)
		report, code := runSelected(context.Background(), root, request, deps)
		if code != 2 || report.SetupError == "" || report.Executed != 0 {
			t.Fatalf("unexecuted child accepted: %+v/%d", report, code)
		}
	}
}

func TestNativeSelectedAdapterCancellationJoinsInvocations(t *testing.T) {
	root := stressTree(t)
	request := nativeRequest(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := &fakeRunner{waitForStop: true}
	deps, _ := testDependencies(fake)
	report, code := runSelected(ctx, root, request, deps)
	if code == 0 || !report.Interrupted || report.Executed != 0 || fake.count() != 0 {
		t.Fatalf("canceled native request = %+v/%d, children=%d", report, code, fake.count())
	}
}

// A rejected child must stop the sample after its existing batch drains. A
// third invocation is an explicit witness of a fresh batch, not a timing guess.
func TestNativeSelectedAdapterStopsAfterPopulationRefusal(t *testing.T) {
	root := stressTree(t)
	request := nativeRequest(t, root)
	refused := &populationRefusalRunner{third: make(chan struct{})}
	deps, stopped := testDependencies(refused)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan selectedRunResult, 1)
	go runPopulationRefusal(ctx, root, request, deps, done)
	select {
	case result := <-done:
		if result.code != 2 {
			t.Fatalf("population refusal exited %d: %+v", result.code, result.report)
		}
		if result.report.SetupError == "" {
			t.Fatal("population refusal returned no setup error")
		}
		if refused.calls.Load() != 2 {
			t.Fatalf("population refusal launched %d children, want only the initial batch", refused.calls.Load())
		}
		if *stopped != 1 {
			t.Fatalf("population refusal joined burners %d times, want once", *stopped)
		}
	case <-refused.third:
		cancel()
		<-done
		t.Fatal("population refusal launched a third invocation after canceling its first batch")
	case <-ctx.Done():
		<-done
		t.Fatal("population refusal did not terminate within its fallback deadline")
	}
}

type populationRefusalRunner struct {
	fakeRunner
	calls atomic.Int32
	third chan struct{}
}

func (r *populationRefusalRunner) Invoke(ctx context.Context, _ invocation) processResult {
	call := r.calls.Add(1)
	if call <= 2 {
		return processResult{output: "selected test did not execute\n"}
	}
	if call == 3 {
		close(r.third)
	}
	<-ctx.Done()
	return processResult{code: 1, err: ctx.Err()}
}

type selectedRunResult struct {
	report Report
	code   int
}

func runPopulationRefusal(ctx context.Context, root string, request SelectedRequest, deps runDependencies, done chan<- selectedRunResult) {
	report, code := runSelected(ctx, root, request, deps)
	done <- selectedRunResult{report: report, code: code}
}
