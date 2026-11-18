// Design: docs/architecture/l2tp/subscriber-session-model.md -- original disconnect audit
//go:build linux

package l2tp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// TestCapturedDisconnectAuditCannotMoveToReusedSID retains two real captures,
// retires one, then reuses the SID. The stale alias cannot write the new audit.
func TestCapturedDisconnectAuditCannotMoveToReusedSID(t *testing.T) {
	observer, generation := newObserverFixture(t, testObserverConfig())
	subsystem, original, admit, _ := sessionCaptureDispatchFixtureForObservation(t, observer, generation)
	captured, err := subsystem.CaptureSession(original)
	require.NoError(t, err)
	stale, err := subsystem.CaptureSession(original)
	require.NoError(t, err)
	require.NoError(t, captured.RecordDisconnect("alice", "first request", 6))
	require.NoError(t, captured.Teardown(t.Context(), l2tpevents.TerminateCauseAdminReset, "first request"))
	replacement := admit()
	require.NotEqual(t, original.Birth, replacement.Birth)
	require.ErrorIs(t, stale.RecordDisconnect("late", "wrong row", 7), ErrSessionNotFound)
	current, err := subsystem.CaptureSession(replacement)
	require.NoError(t, err)
	require.NoError(t, current.RecordDisconnect("bob", "new request", 8))
	history := observer.SessionEvents(original.Birth)
	require.Len(t, history, 1)
	require.Equal(t, "alice", history[0].Actor)
	require.Equal(t, "first request", history[0].Reason)
	require.Equal(t, uint32(6), history[0].Cause)
	require.Equal(t, uint16(100), history[0].TunnelID)
	replacementHistory := observer.SessionEvents(replacement.Birth)
	require.Len(t, replacementHistory, 1)
	require.Equal(t, "bob", replacementHistory[0].Actor)
}

// TestCapturedDisconnectAuditAllowsRetainedCleanupRetry injects one native
// deletion failure. A Birth-qualified retry skips a duplicate audit and still
// reaches the same physical cleanup owner after its logical row was removed.
func TestCapturedDisconnectAuditAllowsRetainedCleanupRetry(t *testing.T) {
	observer, generation := newObserverFixture(t, testObserverConfig())
	subsystem, original, _, observe := sessionCaptureDispatchFixtureForObservation(t, observer, generation)
	worker := subsystem.reactors[0].kernelWorker
	deleteOriginal := worker.ops.sessionDelete
	deleteFailure := errors.New("controlled original deletion refusal")
	deletions := 0
	worker.ops.sessionDelete = func(tid, sid uint16) error {
		deletions++
		if deletions == 1 {
			return deleteFailure
		}
		return deleteOriginal(tid, sid)
	}
	captured, err := subsystem.CaptureSession(original)
	require.NoError(t, err)
	require.NoError(t, captured.RecordDisconnect("alice", "maintenance", 6))
	require.ErrorIs(t, captured.Teardown(t.Context(), l2tpevents.TerminateCauseAdminReset, "maintenance"), deleteFailure)
	retry, err := subsystem.CaptureSession(original)
	require.NoError(t, err)
	require.NoError(t, retry.RecordDisconnect("alice", "maintenance", 6))
	require.NoError(t, retry.Teardown(t.Context(), l2tpevents.TerminateCauseAdminReset, "maintenance"))
	require.Len(t, observer.SessionEvents(original.Birth), 1)
	resources, _, closes := observe()
	require.Zero(t, resources)
	require.Equal(t, 3, closes)
}
