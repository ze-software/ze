// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact unused handoff observation

package ppp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUntransferredCloseReceiptRequiresCanceledOriginalOwner pairs refused
// uncanceled/wrong-owner observations with the original owner's actual Close.
// Recording never closes an owner or fabricates an admitted worker Control.
func TestUntransferredCloseReceiptRequiresCanceledOriginalOwner(t *testing.T) {
	start, _, owner := frameLeaseStart(t, 71)
	_, _, other := frameLeaseStart(t, 72)
	t.Cleanup(func() { require.NoError(t, other.Close()) })
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	attempt := start.Attempt
	require.ErrorIs(t, attempt.RecordUntransferredClose(owner, nil), ErrSessionNotCanceled)
	require.EqualValues(t, 0, owner.closes.Load())
	require.True(t, attempt.Cancel())
	require.ErrorIs(t, attempt.RecordUntransferredClose(other, nil), ErrSessionAttemptInvalid)
	require.EqualValues(t, 0, other.closes.Load())
	status, err := attempt.CleanupStatus()
	require.NoError(t, err)
	require.False(t, status.Finished)
	require.False(t, status.Released)
	require.NoError(t, owner.Close())
	require.NoError(t, attempt.RecordUntransferredClose(owner, nil))
	status, err = attempt.CleanupStatus()
	require.NoError(t, err)
	require.True(t, status.Finished)
	require.True(t, status.Released)
	require.False(t, status.Admitted)
	require.False(t, status.LeaseAcquired)
	require.NoError(t, status.Err)
	_, admitted := attempt.Control()
	require.False(t, admitted)
}

// TestUntransferredCloseReceiptCannotRewriteItsFirstOutcome records a real
// successful Close, then attempts to replace that observation. The first
// immutable result survives; recording must not execute a second Close.
func TestUntransferredCloseReceiptCannotRewriteItsFirstOutcome(t *testing.T) {
	start, _, owner := frameLeaseStart(t, 73)
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	attempt := start.Attempt
	require.True(t, attempt.Cancel())
	require.NoError(t, owner.Close())
	require.NoError(t, attempt.RecordUntransferredClose(owner, nil))
	require.ErrorIs(t, attempt.RecordUntransferredClose(owner, errors.New("late different outcome")), ErrSessionAttemptUsed)
	status, err := attempt.CleanupStatus()
	require.NoError(t, err)
	require.True(t, status.Finished)
	require.True(t, status.Released)
	require.NoError(t, status.Err)
	require.EqualValues(t, 1, owner.closes.Load())
}
