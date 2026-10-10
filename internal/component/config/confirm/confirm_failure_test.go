// Tests for the window when the store or the revert fails.
//
// VALIDATES: the pending record is persisted BEFORE a commit applies, so a
// failed save applies nothing and a crash after the apply still boots the
// rollback (AC-19); a failed deadline revert is retried with a bounded
// backoff, reported in Status and in every refusal, and never shows a
// negative countdown.
// PREVENTS: an applied commit no window can revert, and a window that stays
// open forever after its deadline with nothing telling anyone why.
package confirm

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errInjectedSave   = errors.New("injected save failure")
	errInjectedRevert = errors.New("injected revert failure")
)

// faultyDaemon is a fakeDaemon whose Save and Revert can be made to fail.
// Safe for concurrent use.
type faultyDaemon struct {
	*fakeDaemon
	mu            sync.Mutex
	failSave      bool
	revertFails   int // the next revertFails reverts fail
	revertAttempt int
}

func (d *faultyDaemon) Save(p Pending) error {
	d.mu.Lock()
	fail := d.failSave
	d.mu.Unlock()
	if fail {
		return errInjectedSave
	}
	return d.fakeDaemon.Save(p)
}

func (d *faultyDaemon) Revert(rollback []byte) error {
	d.mu.Lock()
	d.revertAttempt++
	fail := d.revertFails > 0
	if fail {
		d.revertFails--
	}
	d.mu.Unlock()
	if fail {
		return errInjectedRevert
	}
	return d.fakeDaemon.Revert(rollback)
}

func (d *faultyDaemon) set(failSave bool, revertFails int) {
	d.mu.Lock()
	d.failSave = failSave
	d.revertFails = revertFails
	d.mu.Unlock()
}

func (d *faultyDaemon) attempts() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.revertAttempt
}

// newFaultyWindow starts a window over d that retries a failed revert after
// 5ms, then 10ms, at most attemptsMax times.
func newFaultyWindow(t *testing.T, d *faultyDaemon, attemptsMax int) *Window {
	t.Helper()
	w := newWindow(d.Revert, d, revertRetry{first: 5 * time.Millisecond, max: 10 * time.Millisecond, attemptsMax: attemptsMax})
	t.Cleanup(w.Stop)
	return w
}

// TestConfirmWindowRecordsBeforeApply proves the record is on disk when the
// commit applies: a daemon that crashes inside Apply, or just after it,
// restarts with the record and reverts (AC-19).
func TestConfirmWindowRecordsBeforeApply(t *testing.T) {
	d := &faultyDaemon{fakeDaemon: newFakeDaemon("v1")}
	w := newFaultyWindow(t, d, 3)

	var atApply *Pending
	commit := d.commit("v2")
	apply := commit.Apply
	commit.Apply = func() error {
		atApply = d.saved()
		return apply()
	}
	require.NoError(t, w.Confirmed("alice", time.Minute, false, commit))
	require.NotNil(t, atApply, "the record must be persisted before the commit applies")
	assert.Equal(t, "v1", string(atApply.Rollback))

	// The crash: the daemon stops with the window open. The next start
	// reverts from the record it finds.
	w.Stop()
	reverted, err := RecoverOnStart(d.saved(), d.fakeDaemon.Revert, d)
	require.NoError(t, err)
	assert.True(t, reverted)
	assert.Equal(t, "v1", d.config())
}

// TestConfirmWindowSaveFailureAppliesNothing proves a record that cannot be
// saved refuses the commit before it applies, so no commit is ever left
// applied with no revert behind it.
func TestConfirmWindowSaveFailureAppliesNothing(t *testing.T) {
	d := &faultyDaemon{fakeDaemon: newFakeDaemon("v1")}
	w := newFaultyWindow(t, d, 3)
	d.set(true, 0)

	err := w.Confirmed("alice", time.Minute, false, d.commit("v2"))
	require.ErrorIs(t, err, errInjectedSave)
	assert.Equal(t, "v1", d.config(), "a commit whose record failed to save must not apply")
	_, open := w.Status()
	assert.False(t, open)

	// Nested: the owner's force commit inside an open window.
	d.set(false, 0)
	require.NoError(t, w.Confirmed("alice", time.Minute, false, d.commit("v2")))
	before, open := w.Status()
	require.True(t, open)
	d.set(true, 0)
	err = w.Confirmed("alice", time.Hour, true, d.commit("v3"))
	require.ErrorIs(t, err, errInjectedSave)
	assert.Equal(t, "v2", d.config(), "a nested commit whose record failed to save must not apply")
	after, open := w.Status()
	require.True(t, open)
	assert.Equal(t, before.Deadline, after.Deadline, "the countdown is unchanged")
}

// TestConfirmWindowApplyFailureClearsRecord proves a commit that fails to
// apply leaves no record, so a restart reverts nothing it never applied.
func TestConfirmWindowApplyFailureClearsRecord(t *testing.T) {
	d := &faultyDaemon{fakeDaemon: newFakeDaemon("v1")}
	w := newFaultyWindow(t, d, 3)
	errApply := errors.New("apply refused")

	commit := d.commit("v2")
	commit.Apply = func() error { return errApply }
	require.ErrorIs(t, w.Confirmed("alice", time.Minute, false, commit), errApply)
	assert.Nil(t, d.saved(), "no record survives a commit that did not apply")
	_, open := w.Status()
	assert.False(t, open)
}

// TestConfirmWindowRetriesFailedRevert proves a failed deadline revert is
// retried and shown: Status carries the failure while it retries, and the
// window closes as a timeout once a retry succeeds.
func TestConfirmWindowRetriesFailedRevert(t *testing.T) {
	d := &faultyDaemon{fakeDaemon: newFakeDaemon("v1")}
	w := newFaultyWindow(t, d, 5)
	d.set(false, 2)

	require.NoError(t, w.Confirmed("alice", 20*time.Millisecond, false, d.commit("v2")))
	assert.Equal(t, "v1", waitRevert(t, d.fakeDaemon))
	require.Eventually(t, func() bool {
		_, open := w.Status()
		return !open
	}, 5*time.Second, 5*time.Millisecond)
	assert.Equal(t, 3, d.attempts(), "two failures, then the retry that succeeded")
	timeouts, err := w.Timeouts()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), timeouts)
	assert.Nil(t, d.saved())
}

// TestConfirmWindowRevertRetriesSpent proves a revert that keeps failing
// stops retrying at the bound, keeps the window and its record, tells every
// user why with no negative countdown, and leaves the owner able to retry
// with `commit abort`.
func TestConfirmWindowRevertRetriesSpent(t *testing.T) {
	d := &faultyDaemon{fakeDaemon: newFakeDaemon("v1")}
	w := newFaultyWindow(t, d, 3)
	d.set(false, 1000)

	require.NoError(t, w.Confirmed("alice", 10*time.Millisecond, false, d.commit("v2")))
	require.Eventually(t, func() bool {
		status, open := w.Status()
		return open && status.RevertFailed != nil && status.Retry.IsZero()
	}, 5*time.Second, 5*time.Millisecond, "the retries end, and Status says so")
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 3, d.attempts(), "the retries are bounded")

	status, open := w.Status()
	require.True(t, open, "a window whose revert failed stays open")
	assert.Equal(t, time.Duration(0), status.Left(), "never a negative countdown")
	assert.NotNil(t, d.saved(), "the record stays, so a restart still reverts")
	for _, user := range []string{"alice", "bob"} {
		line := status.Line(user)
		assert.Contains(t, line, "revert", user)
		assert.Contains(t, line, errInjectedRevert.Error(), user)
		assert.NotContains(t, line, "-", user)
	}

	err := w.Now("bob", d.commit("v3"))
	var other *OtherUserError
	require.ErrorAs(t, err, &other)
	assert.Contains(t, err.Error(), errInjectedRevert.Error())
	assert.False(t, strings.Contains(err.Error(), " -"), "no negative seconds: %s", err)
	assert.Equal(t, "v2", d.config())

	d.set(false, 0)
	require.NoError(t, w.Abort("alice"), "the owner retries the revert")
	assert.Equal(t, "v1", d.config())
	_, open = w.Status()
	assert.False(t, open)
}
