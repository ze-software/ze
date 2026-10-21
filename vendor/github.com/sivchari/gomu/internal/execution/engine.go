// Package execution provides mutation testing execution functionality.
package execution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sivchari/gomu/internal/mutation"
)

// Engine handles test execution using overlay-based mutation.
type Engine struct {
	overlay  *OverlayMutator
	testArgs []string
}

// New creates a new execution engine.
func New() (*Engine, error) {
	overlay, err := NewOverlayMutator()
	if err != nil {
		return nil, fmt.Errorf("failed to create overlay mutator: %w", err)
	}

	return &Engine{
		overlay: overlay,
	}, nil
}

// SetTestArgs sets the flags forwarded to "go test" (and, for the flags that
// go build also understands, the compile check that precedes it). Call it
// before RunMutations or RunMutationsWithOptions.
func (e *Engine) SetTestArgs(args []string) {
	e.testArgs = args
}

// Close cleans up the execution engine.
func (e *Engine) Close() error {
	if e.overlay != nil {
		return e.overlay.Cleanup()
	}

	return nil
}

// RunMutations executes tests for all mutants in parallel.
func (e *Engine) RunMutations(mutants []mutation.Mutant) ([]mutation.Result, error) {
	return e.RunMutationsWithOptions(mutants, 4, 30)
}

// RunMutationsWithOptions executes tests for all mutants in parallel with custom options.
func (e *Engine) RunMutationsWithOptions(mutants []mutation.Mutant, workers, timeout int) ([]mutation.Result, error) {
	if len(mutants) == 0 {
		return nil, nil
	}

	results := make([]mutation.Result, len(mutants))
	resultsChan := make(chan indexedResult, len(mutants))

	var wg sync.WaitGroup

	semaphore := make(chan struct{}, workers)

	// Start workers - no file locks needed with overlay approach
	for i, mutant := range mutants {
		wg.Add(1)

		go func(index int, m mutation.Mutant) {
			defer wg.Done()

			semaphore <- struct{}{}

			defer func() { <-semaphore }()

			result := e.runSingleMutation(m, timeout)
			resultsChan <- indexedResult{index: index, result: result}
		}(i, mutant)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	for indexedRes := range resultsChan {
		results[indexedRes.index] = indexedRes.result
	}

	return results, nil
}

type indexedResult struct {
	index  int
	result mutation.Result
}

// runSingleMutation executes tests for a single mutant using overlay.
func (e *Engine) runSingleMutation(mutant mutation.Mutant, timeout int) mutation.Result {
	result := mutation.Result{
		Mutant: mutant,
		Status: mutation.StatusError,
	}

	// 1. Prepare mutation (create mutated file + overlay.json)
	mutCtx, err := e.overlay.PrepareMutation(mutant)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to prepare mutation: %v", err)

		return result
	}

	defer func() {
		if cleanupErr := e.overlay.CleanupMutation(mutCtx); cleanupErr != nil {
			fmt.Printf("Warning: failed to cleanup mutation: %v\n", cleanupErr)
		}
	}()

	// 2. Check if the mutated code compiles using overlay
	if err := e.checkCompilationWithOverlay(mutCtx); err != nil {
		result.Status = mutation.StatusNotViable
		result.Error = fmt.Sprintf("Compilation failed: %v", err)
		result.Output = err.Error()

		return result
	}

	// 3. Run tests using overlay
	return e.runTestWithOverlay(mutCtx, mutant, timeout)
}

// checkCompilationWithOverlay verifies that the mutated code compiles using overlay.
// No timeout is applied because compilation always terminates.
func (e *Engine) checkCompilationWithOverlay(mutCtx *MutationContext) error {
	compileDir := filepath.Dir(mutCtx.OriginalPath)

	// -o discards the binary so a package main does not write into the user's working tree.
	args := append([]string{"build", "-o", os.DevNull, "-overlay=" + mutCtx.OverlayPath}, splitBuildFlags(e.testArgs)...)
	args = append(args, ".")

	cmd := exec.CommandContext(context.Background(), "go", args...)
	cmd.Dir = compileDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compilation error: %s", string(output))
	}

	return nil
}

// isTestBinaryFailure reports whether go test output shows that the test
// binary failed to build or set up, so no test was executed.
func isTestBinaryFailure(output string) bool {
	return strings.Contains(output, "[build failed]") || strings.Contains(output, "[setup failed]")
}

// goTestArgs builds the "go test" argv for one mutant run.
//
// -vet=off keeps go test's vet pass from rejecting a mutant (for example an
// emptied printf format string) before any test runs, which would be
// misreported as killed. Unmutated code is vetted by the project's own CI.
func goTestArgs(overlayPath string, extra []string) []string {
	args := append([]string{"test", "-overlay=" + overlayPath, "-vet=off"}, extra...)

	return append(args, ".")
}

// runTestWithOverlay runs tests using the overlay configuration.
func (e *Engine) runTestWithOverlay(mutCtx *MutationContext, mutant mutation.Mutant, timeout int) mutation.Result {
	result := mutation.Result{
		Mutant: mutant,
		Status: mutation.StatusError,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// Get the directory containing the original file for running tests
	testDir := filepath.Dir(mutCtx.OriginalPath)

	cmd := exec.CommandContext(ctx, "go", goTestArgs(mutCtx.OverlayPath, e.testArgs)...)
	cmd.Dir = testDir

	output, err := cmd.CombinedOutput()

	// Analyze test results
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.Status = mutation.StatusTimedOut
		result.Error = "Test execution timed out"

		return result
	}

	result.Output = string(output)

	exitedNonZero := cmd.ProcessState != nil && cmd.ProcessState.ExitCode() != 0

	switch {
	case err == nil:
		result.Status = mutation.StatusSurvived
	case !exitedNonZero:
		result.Error = err.Error()
	case isTestBinaryFailure(result.Output):
		// A test package that never built or set up ran no test, so it
		// cannot have killed the mutant.
		result.Error = err.Error()
	default:
		result.Status = mutation.StatusKilled
	}

	return result
}
