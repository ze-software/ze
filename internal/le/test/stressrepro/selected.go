// Design: docs/functional-tests.md -- bounded selected native stress.
// Related: run.go -- the shared stress orchestrator.

package teststressrepro

import (
	"context"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
)

// SelectedRequest is raw input from a native runner which has already resolved
// its canonical runner words and prepared its binary environment. Count reads
// the owning runner's summary and refuses anything except one executed test.
type SelectedRequest struct {
	Suite       string
	Runner      []string
	Selector    string
	Executable  string
	Environment []string
	Iterations  int
	Parallel    int
	Timeout     time.Duration
	Budget      time.Duration
	Count       func(string) (int, error)
}

// RunSelected enters the shared stress orchestrator with a native selection.
func RunSelected(ctx context.Context, root string, request SelectedRequest) (Report, int) {
	return runSelected(ctx, root, request, realDependencies())
}

// selectedRun owns a validated copy of the native request. Its zero/nil value
// is uninitialized; only prepareSelected constructs it for the orchestrator.
type selectedRun struct {
	request SelectedRequest
	zeBin   string
}

func runSelected(ctx context.Context, root string, request SelectedRequest, deps runDependencies) (Report, int) {
	base := Report{Suite: request.Suite, Test: request.Selector, Iterations: request.Iterations, Parallel: request.Parallel}
	selected, err := prepareSelected(request)
	if err != nil {
		return setupFailure(base, err)
	}
	if ctx.Err() != nil {
		base.Interrupted = true
		return base, 1
	}
	deps.selected = selected
	return run(ctx, root, Options{Suite: request.Suite, Test: request.Selector,
		Iterations: request.Iterations, Parallel: request.Parallel, AnyFailure: true}, deps)
}

func prepareSelected(request SelectedRequest) (*selectedRun, error) {
	if _, found := testfunctional.SuiteNamed(request.Suite); !found {
		return nil, unknownSuiteError(request.Suite)
	}
	if len(request.Runner) == 0 {
		return nil, selectedRequestError("native selection has no canonical runner words")
	}
	if slices.Contains(request.Runner, testfunctional.AllTests) {
		return nil, selectedRequestError("native selection still requests the whole suite")
	}
	if request.Selector == "" {
		return nil, selectedRequestError("native selection has no stable test name")
	}
	if request.Iterations < 1 {
		return nil, selectedRequestError("native selection requires positive iterations")
	}
	if request.Iterations > 1000 {
		return nil, selectedRequestError("native selection exceeds 1000 iterations")
	}
	if request.Parallel < 1 {
		return nil, selectedRequestError("native selection requires positive workers")
	}
	if request.Parallel > 4 {
		return nil, selectedRequestError("native selection exceeds four workers")
	}
	if request.Timeout <= 0 {
		return nil, selectedRequestError("native selection requires a positive invocation timeout")
	}
	if request.Budget < request.Timeout {
		return nil, selectedRequestError("native selection budget cannot cover its invocation timeout")
	}
	if request.Count == nil {
		return nil, selectedRequestError("native selection has no executed-test counter")
	}
	zeBin := env.LookupIn(request.Environment, "ze.bin")
	if zeBin == "" {
		return nil, selectedRequestError("native selection environment does not pin its prepared ZE_BIN")
	}
	if err := ensureBinaries(zeBin, request.Executable); err != nil {
		return nil, err
	}
	request.Runner = slices.Clone(request.Runner)
	request.Environment = slices.Clone(request.Environment)
	return &selectedRun{request: request, zeBin: zeBin}, nil
}

func selectedRequestError(problem string) error {
	return opererr.New(problem, "Native stress cannot start without a bounded prepared selection",
		"./le test qemu all test test/<dir>/<name>.ci stress 80 stress-parallel 1")
}
