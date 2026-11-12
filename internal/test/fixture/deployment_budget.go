// Design: docs/functional-tests.md -- functional runner contention budgets.
// Related: internal/test/runner/parallel.go -- ChildParallelFactor.

package fixture

import (
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

// deploymentBudget applies the runner's contention factor once to an authored
// deployment wait. A single selected test keeps its original deadline.
func deploymentBudget(base time.Duration) time.Duration {
	return base * time.Duration(runner.ChildParallelFactor())
}
