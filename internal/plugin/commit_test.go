package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCommitContext creates a CommandContext with CommitManager for tests.
func testCommitContext() *CommandContext {
	return &CommandContext{
		Reactor:       &mockReactor{},
		CommitManager: NewCommitManager(),
	}
}

// TestCommitStart verifies commit start command.
//
// VALIDATES: Named commit is created with peer selector.
// PREVENTS: Transaction not starting.
func TestCommitStart(t *testing.T) {
	ctx := testCommitContext()

	resp, err := handleCommit(ctx, []string{"batch1", "start"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "batch1", data["commit"])

	// Verify commit exists
	tx, err := ctx.CommitManager.Get("batch1")
	require.NoError(t, err)
	assert.Equal(t, "batch1", tx.Name())
}

// TestCommitStartDuplicate verifies duplicate name rejection.
//
// VALIDATES: Error returned if commit already exists.
// PREVENTS: Overwriting active commits.
func TestCommitStartDuplicate(t *testing.T) {
	ctx := testCommitContext()

	_, err := handleCommit(ctx, []string{"batch1", "start"})
	require.NoError(t, err)

	resp, err := handleCommit(ctx, []string{"batch1", "start"})

	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
	errMsg, ok := resp.Data.(string)
	require.True(t, ok, "expected Data to be string")
	assert.Contains(t, errMsg, "exists")
}

// TestCommitEnd verifies commit end command.
//
// VALIDATES: End removes commit and returns stats.
// PREVENTS: Routes not being processed on commit.
func TestCommitEnd(t *testing.T) {
	ctx := testCommitContext()

	_, err := handleCommit(ctx, []string{"batch1", "start"})
	require.NoError(t, err)

	resp, err := handleCommit(ctx, []string{"batch1", "end"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "batch1", data["commit"])
	assert.Equal(t, "end", data["action"])

	// Verify commit removed
	_, err = ctx.CommitManager.Get("batch1")
	require.Error(t, err)
}

// TestCommitEOR verifies commit eor command.
//
// VALIDATES: EOR flag is set when using eor action.
// PREVENTS: EOR not being requested.
func TestCommitEOR(t *testing.T) {
	ctx := testCommitContext()

	_, err := handleCommit(ctx, []string{"batch1", "start"})
	require.NoError(t, err)

	resp, err := handleCommit(ctx, []string{"batch1", "eor"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "batch1", data["commit"])
	assert.Equal(t, "eor", data["action"])
}

// TestCommitEndNotFound verifies error for non-existent commit.
//
// VALIDATES: Error returned if commit doesn't exist.
// PREVENTS: Commit outside transaction context.
func TestCommitEndNotFound(t *testing.T) {
	ctx := testCommitContext()

	resp, err := handleCommit(ctx, []string{"nonexistent", "end"})

	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data.(string), "not found") //nolint:forcetypeassert // test code
}

// TestCommitRollback verifies rollback command.
//
// VALIDATES: Rollback removes commit and returns discard count.
// PREVENTS: Rollback not discarding routes.
func TestCommitRollback(t *testing.T) {
	ctx := testCommitContext()

	_, err := handleCommit(ctx, []string{"batch1", "start"})
	require.NoError(t, err)

	resp, err := handleCommit(ctx, []string{"batch1", "rollback"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "batch1", data["commit"])
	assert.Equal(t, 0, data["routes_discarded"]) // Empty commit

	// Verify commit removed
	_, err = ctx.CommitManager.Get("batch1")
	require.Error(t, err)
}

// TestCommitRollbackNotFound verifies error for non-existent commit.
//
// VALIDATES: Error returned if commit doesn't exist.
// PREVENTS: Rollback outside transaction context.
func TestCommitRollbackNotFound(t *testing.T) {
	ctx := testCommitContext()

	resp, err := handleCommit(ctx, []string{"nonexistent", "rollback"})

	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data.(string), "not found") //nolint:forcetypeassert // test code
}

// TestCommitShow verifies show command.
//
// VALIDATES: Show returns commit info.
// PREVENTS: Missing introspection.
func TestCommitShow(t *testing.T) {
	ctx := testCommitContext()

	_, err := handleCommit(ctx, []string{"batch1", "start"})
	require.NoError(t, err)

	resp, err := handleCommit(ctx, []string{"batch1", "show"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "batch1", data["commit"])
	assert.Equal(t, 0, data["queued"])
}

// TestCommitList verifies list command.
//
// VALIDATES: List returns all active commits.
// PREVENTS: Missing commits in list.
func TestCommitList(t *testing.T) {
	ctx := testCommitContext()

	_, _ = handleCommit(ctx, []string{"batch1", "start"})
	_, _ = handleCommit(ctx, []string{"batch2", "start"})

	resp, err := handleCommit(ctx, []string{"list"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 2, data["count"])
}

// TestCommitMissingArgs verifies error for missing arguments.
//
// VALIDATES: Error returned if no arguments.
// PREVENTS: Undefined behavior on empty input.
func TestCommitMissingArgs(t *testing.T) {
	ctx := testCommitContext()

	resp, err := handleCommit(ctx, nil)

	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data.(string), "usage") //nolint:forcetypeassert // test code
}

// TestCommitMissingAction verifies error for missing action.
//
// VALIDATES: Error returned if action missing.
// PREVENTS: Undefined behavior on incomplete input.
func TestCommitMissingAction(t *testing.T) {
	ctx := testCommitContext()

	resp, err := handleCommit(ctx, []string{"batch1"})

	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data.(string), "usage") //nolint:forcetypeassert // test code
}

// TestCommitUnknownAction verifies error for unknown action.
//
// VALIDATES: Error returned for invalid action.
// PREVENTS: Undefined behavior on bad action.
func TestCommitUnknownAction(t *testing.T) {
	ctx := testCommitContext()

	resp, err := handleCommit(ctx, []string{"batch1", "invalid"})

	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data.(string), "unknown") //nolint:forcetypeassert // test code
}

// TestCommitCommandRegistered verifies commit command is registered.
//
// VALIDATES: Commit command available.
// PREVENTS: Missing commit command.
func TestCommitCommandRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterCommitHandlers(d)

	c := d.Lookup("bgp commit")
	assert.NotNil(t, c, "bgp commit command must be registered")
}

// TestCommitConcurrent verifies multiple concurrent commits.
//
// VALIDATES: Multiple commits can be active simultaneously.
// PREVENTS: Commits interfering with each other.
func TestCommitConcurrent(t *testing.T) {
	ctx := testCommitContext()

	_, err := handleCommit(ctx, []string{"batch1", "start"})
	require.NoError(t, err)

	_, err = handleCommit(ctx, []string{"batch2", "start"})
	require.NoError(t, err)

	// Both should be accessible
	tx1, err := ctx.CommitManager.Get("batch1")
	require.NoError(t, err)
	assert.Equal(t, "batch1", tx1.Name())

	tx2, err := ctx.CommitManager.Get("batch2")
	require.NoError(t, err)
	assert.Equal(t, "batch2", tx2.Name())

	// End one, other still exists
	_, err = handleCommit(ctx, []string{"batch1", "end"})
	require.NoError(t, err)

	_, err = ctx.CommitManager.Get("batch1")
	require.Error(t, err)

	_, err = ctx.CommitManager.Get("batch2")
	require.NoError(t, err)
}

// TestCommitWithdrawRoute verifies queuing withdrawals to a commit.
//
// VALIDATES: Withdrawal is queued to transaction.
// PREVENTS: Withdrawals being lost.
func TestCommitWithdrawRoute(t *testing.T) {
	ctx := testCommitContext()

	_, err := handleCommit(ctx, []string{"batch1", "start"})
	require.NoError(t, err)

	resp, err := handleCommit(ctx, []string{"batch1", "withdraw", "route", "10.0.0.0/24"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	// Verify withdrawal queued
	tx, _ := ctx.CommitManager.Get("batch1")
	assert.Equal(t, 1, tx.WithdrawalCount())
}
