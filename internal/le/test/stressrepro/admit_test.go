// VALIDATES: a stress run holds one job registry slot and every one of its N
// parallel children runs inside it, each as a distinct run (AC-45 of
// spec-le-subject-first-command-tree).
// PREVENTS: children 2..N attaching to child 1's verdict or queueing behind it,
// which silently collapses N requested repetitions into one.
package teststressrepro

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/job"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// admittingRunner stands in for the `le test <suite>` child: each invocation
// admits the same argv over the same tree, exactly as harnesstool.admitRun
// does, and records the ticket kind it was answered. A claimed child holds its
// slot briefly so a sibling overlaps it.
type admittingRunner struct {
	mu    sync.Mutex
	kinds []job.Kind
}

func (r *admittingRunner) Invoke(_ context.Context, spec invocation) processResult {
	admission, err := job.NewIn(spec.root)
	if err != nil {
		return processResult{code: 2, err: err}
	}
	ticket, err := admission.Admit("test-bgp", []string{"le", "test", "bgp", "-v", "--all"})
	if err != nil {
		return processResult{code: 2, err: err}
	}
	r.mu.Lock()
	r.kinds = append(r.kinds, ticket.Kind)
	r.mu.Unlock()
	if ticket.Kind == job.KindClaimed {
		time.Sleep(100 * time.Millisecond)
		ticket.Release(0)
	}
	return processResult{code: ticket.Code}
}

func (r *admittingRunner) buildRace(context.Context, string, string, string) processResult {
	return processResult{}
}

// TestStressIterationsEachRunInsideOneSlot drives admitRun with N parallel
// admitting children and a test process that holds no parent job. Every child
// MUST be answered KindInside: a claim or an attach means the child ran as its
// own job, which is the collapse this test exists to catch.
func TestStressIterationsEachRunInsideOneSlot(t *testing.T) {
	if !leroot.Admits(area) {
		t.Fatal("the stress run holds a slot, so it MUST register as admitted")
	}
	root := stressTree(t)
	saved := env.Get(job.ParentKey)
	if err := env.Set(job.ParentKey, ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = env.Set(job.ParentKey, saved) })

	const iterations = 4
	children := &admittingRunner{}
	deps, _ := testDependencies(children)
	opts := baseOptions()
	opts.Iterations, opts.Parallel = iterations, iterations

	report, code := admitRun(context.Background(), root, []string{"run", "suite", "bgp"}, opts, deps)
	if code != 1 || report.Completed != iterations || report.SetupError != "" {
		t.Fatalf("code %d, report %#v: want %d completed, not reproduced", code, report, iterations)
	}
	if len(children.kinds) != iterations {
		t.Fatalf("%d children admitted, want %d", len(children.kinds), iterations)
	}
	for index, kind := range children.kinds {
		if kind != job.KindInside {
			t.Errorf("child %d answered %s, want %s: it ran as its own job", index, kind, job.KindInside)
		}
	}
	if got := env.Get(job.ParentKey); got != "" {
		t.Errorf("parent %q left named after the run, want it restored", got)
	}
}
