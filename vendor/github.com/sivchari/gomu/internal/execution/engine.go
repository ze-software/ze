// Package execution provides mutation testing execution functionality.
package execution

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/sivchari/gomu/internal/mutation"
)

// Engine handles test execution using overlay-based mutation.
type Engine struct {
	overlay *OverlayMutator
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
	// Get the directory containing the original file for compilation
	compileDir := filepath.Dir(mutCtx.OriginalPath)

	// Build the entire package with overlay to properly resolve dependencies
	cmd := exec.Command("go", "build", "-overlay="+mutCtx.OverlayPath, ".")
	cmd.Dir = compileDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compilation error: %s", string(output))
	}

	return nil
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

	cmd := exec.CommandContext(ctx, "go", "test", "-overlay="+mutCtx.OverlayPath, ".")
	cmd.Dir = testDir

	output, err := cmd.CombinedOutput()

	// Analyze test results
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.Status = mutation.StatusTimedOut
		result.Error = "Test execution timed out"

		return result
	}

	result.Output = string(output)

	if err != nil {
		// Tests failed - check if it's because the mutant was killed
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() != 0 {
			result.Status = mutation.StatusKilled
		} else {
			result.Status = mutation.StatusError
			result.Error = err.Error()
		}
	} else {
		// Tests passed - mutant survived
		result.Status = mutation.StatusSurvived
	}

	return result
}
