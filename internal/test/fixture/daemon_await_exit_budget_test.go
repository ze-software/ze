// Design: docs/architecture/testing/runner-architecture.md -- inherited fixture wait budgets.
// Related: daemon_await_exit_fixture.go -- the actual process-exit barrier.

package fixture

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
)

// VALIDATES: the actual exit barrier derives its inner wait from the runner's
// published budget, while refusing a process that remains alive.
// PREVENTS: a fixed helper deadline overriding the case's authored budget.
// MUTATION: replace WaitAttempts with 600 and the parent expires first.
func TestDaemonAwaitExitHonorsRunnerBudget(t *testing.T) {
	t.Cleanup(env.ResetCache)
	t.Setenv(runner.TestBudgetEnv, "400ms")
	env.ResetCache()
	target := startAwaitExitTarget(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	err := daemonAwaitExit(ctx, nil)
	require.Error(t, err, "a running process released the barrier")
	assert.Contains(t, err.Error(), "never exited")
	state, stateErr := daemonProcessState(t.Context(), target.Process.Pid)
	require.NoError(t, stateErr)
	require.NotEmpty(t, state, "the barrier's target disappeared")
	assert.False(t, strings.HasPrefix(state, "Z"), "the barrier's target exited")
	assert.NoError(t, ctx.Err(), "the helper ignored the published budget and waited for its parent to expire")
}

// VALIDATES: an earlier caller deadline still stops the actual exit barrier,
// even when the runner publishes a longer wait budget.
// PREVENTS: a derived helper budget detaching from its caller's cancellation.
func TestDaemonAwaitExitHonorsEarlierCallerDeadline(t *testing.T) {
	t.Cleanup(env.ResetCache)
	t.Setenv(runner.TestBudgetEnv, "400ms")
	env.ResetCache()
	target := startAwaitExitTarget(t)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	err := daemonAwaitExit(ctx, nil)
	require.Error(t, err, "a running process released the barrier")
	assert.Contains(t, err.Error(), "never exited")
	assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
	state, stateErr := daemonProcessState(t.Context(), target.Process.Pid)
	require.NoError(t, stateErr)
	require.NotEmpty(t, state, "the barrier's target disappeared")
	assert.False(t, strings.HasPrefix(state, "Z"), "the barrier's target exited")
}
