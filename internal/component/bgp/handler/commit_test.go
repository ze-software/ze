package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// TestHandlerCommitList verifies commit list returns active commits.
//
// VALIDATES: Commit list handler returns commits from CommitManager.
// PREVENTS: Commit list failing when no commits exist.
func TestHandlerCommitList(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleCommit(ctx, []string{"list"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, data["count"])
}

// TestHandlerCommitStartAndShow verifies commit start then show.
//
// VALIDATES: Start creates a named commit visible via show.
// PREVENTS: Lost commits after start.
func TestHandlerCommitStartAndShow(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	// Start a commit.
	resp, err := handleCommit(ctx, []string{"test-commit", "start"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	// Show the commit.
	resp, err = handleCommit(ctx, []string{"test-commit", "show"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test-commit", data["commit"])
	assert.Equal(t, 0, data["queued"])
}

// TestHandlerCommitStartAndEnd verifies commit start then end (no routes).
//
// VALIDATES: End flushes an empty commit without error.
// PREVENTS: Error on empty commit end.
func TestHandlerCommitStartAndEnd(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleCommit(ctx, []string{"test-commit", "start"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	resp, err = handleCommit(ctx, []string{"test-commit", "end"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "end", data["action"])
	assert.Equal(t, 0, data["queued"])
}

// TestHandlerCommitStartAndEOR verifies commit start then eor.
//
// VALIDATES: EOR action flushes with EOR flag.
// PREVENTS: EOR flag not set when using eor action.
func TestHandlerCommitStartAndEOR(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	_, err := handleCommit(ctx, []string{"test-commit", "start"})
	require.NoError(t, err)

	resp, err := handleCommit(ctx, []string{"test-commit", "eor"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "eor", data["action"])
}

// TestHandlerCommitRollback verifies commit start then rollback.
//
// VALIDATES: Rollback discards queued routes.
// PREVENTS: Rollback leaving stale state.
func TestHandlerCommitRollback(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	_, err := handleCommit(ctx, []string{"test-commit", "start"})
	require.NoError(t, err)

	resp, err := handleCommit(ctx, []string{"test-commit", "rollback"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test-commit", data["commit"])
}

// TestHandlerCommitUnknownAction verifies commit rejects unknown actions.
//
// VALIDATES: Only known commit actions are accepted.
// PREVENTS: Silently ignoring typos in action names.
func TestHandlerCommitUnknownAction(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleCommit(ctx, []string{"test-commit", "bogus"})
	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data, "unknown commit action")
}

// TestHandlerCommitMissingName verifies commit rejects single-arg (no action).
//
// VALIDATES: Commit requires both name and action.
// PREVENTS: Ambiguous command with only name.
func TestHandlerCommitMissingName(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleCommit(ctx, []string{"test-commit"})
	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
}

// TestHandlerCommitNilReactor verifies commit errors without reactor.
//
// VALIDATES: Commit handler returns error when reactor is nil.
// PREVENTS: Nil pointer dereference in commit operations.
func TestHandlerCommitNilReactor(t *testing.T) {
	ctx := newTestContext(nil)

	// "list" checks CommitManager (on Server), not reactor directly.
	// "start" checks reactor via RequireReactor.
	resp, err := handleCommit(ctx, []string{"test-commit", "start"})
	require.Error(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestHandlerCommitWithdrawRoute verifies withdraw route in a named commit.
//
// VALIDATES: Withdraw queues NLRI for IPv4 prefix.
// PREVENTS: Withdrawal not reaching the commit transaction.
func TestHandlerCommitWithdrawRoute(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	// Start a commit.
	_, err := handleCommit(ctx, []string{"test-commit", "start"})
	require.NoError(t, err)

	// Withdraw a route.
	resp, err := handleCommit(ctx, []string{"test-commit", "withdraw", "route", "10.0.0.0/24"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.0/24", data["prefix"])
	assert.Equal(t, 1, data["withdrawals"])
}

// TestHandlerCommitWithdrawIPv6 verifies withdraw route for IPv6 prefix.
//
// VALIDATES: Withdraw queues NLRI for IPv6 prefix.
// PREVENTS: IPv6 prefixes rejected by commit withdraw.
func TestHandlerCommitWithdrawIPv6(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	_, err := handleCommit(ctx, []string{"test-commit", "start"})
	require.NoError(t, err)

	resp, err := handleCommit(ctx, []string{"test-commit", "withdraw", "route", "2001:db8::/32"})
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2001:db8::/32", data["prefix"])
}

// TestHandlerCommitWithdrawInvalidPrefix verifies withdraw rejects invalid prefix.
//
// VALIDATES: Invalid prefix string produces error.
// PREVENTS: Panic on unparseable prefix.
func TestHandlerCommitWithdrawInvalidPrefix(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	_, err := handleCommit(ctx, []string{"test-commit", "start"})
	require.NoError(t, err)

	resp, err := handleCommit(ctx, []string{"test-commit", "withdraw", "route", "not-a-prefix"})
	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
}

// TestHandlerCommitWithdrawMissingPrefix verifies withdraw rejects missing prefix.
//
// VALIDATES: Withdraw route requires prefix argument.
// PREVENTS: Panic on missing args.
func TestHandlerCommitWithdrawMissingPrefix(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	_, err := handleCommit(ctx, []string{"test-commit", "start"})
	require.NoError(t, err)

	resp, err := handleCommit(ctx, []string{"test-commit", "withdraw", "route"})
	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
}

// TestHandlerCommitListAfterStart verifies list shows started commits.
//
// VALIDATES: Started commit appears in list.
// PREVENTS: Commit list not reflecting active commits.
func TestHandlerCommitListAfterStart(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	_, err := handleCommit(ctx, []string{"my-commit", "start"})
	require.NoError(t, err)

	resp, err := handleCommit(ctx, []string{"list"})
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, data["count"])
}

// TestHandlerCommitShowNotFound verifies show errors for unknown commit.
//
// VALIDATES: Show returns error for non-existent commit name.
// PREVENTS: Nil pointer when commit doesn't exist.
func TestHandlerCommitShowNotFound(t *testing.T) {
	ctx := newTestContext(&mockReactor{})

	resp, err := handleCommit(ctx, []string{"nonexistent", "show"})
	require.Error(t, err)
	assert.Equal(t, "error", resp.Status)
}
