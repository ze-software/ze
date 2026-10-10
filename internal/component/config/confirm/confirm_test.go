// Tests for the daemon's confirmed-commit window, against a fake daemon.
//
// VALIDATES: spec-session-editor-file-mode-parity AC-14, 15, 17, 18, 19, 23,
// 24 at the worker: deadline revert, the user owns the window, refusals,
// nested force keeping the first rollback, and recovery at start.
// PREVENTS: a window that dies with its session, a second commit erased by a
// revert, and a daemon that boots on an unconfirmed config.
package confirm

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDaemon stands in for the hub: it holds the running config, applies a
// commit, reverts to a rollback, and records the pending record the window
// persists. Safe for concurrent use, because the window's worker and the test
// goroutine both reach it.
type fakeDaemon struct {
	mu       sync.Mutex
	running  string
	record   *Pending
	reverted chan string
}

func newFakeDaemon(running string) *fakeDaemon {
	return &fakeDaemon{running: running, reverted: make(chan string, 4)}
}

func (d *fakeDaemon) config() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running
}

func (d *fakeDaemon) commit(next string) Commit {
	return Commit{
		Snapshot: func() ([]byte, error) { return []byte(d.config()), nil },
		Apply: func() error {
			d.mu.Lock()
			defer d.mu.Unlock()
			d.running = next
			return nil
		},
	}
}

func (d *fakeDaemon) Revert(rollback []byte) error {
	d.mu.Lock()
	d.running = string(rollback)
	d.mu.Unlock()
	d.reverted <- string(rollback)
	return nil
}

func (d *fakeDaemon) Save(p Pending) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.record = &p
	return nil
}

func (d *fakeDaemon) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.record = nil
	return nil
}

func (d *fakeDaemon) saved() *Pending {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.record
}

func newTestWindow(t *testing.T, d *fakeDaemon) *Window {
	t.Helper()
	w := NewWindow(d.Revert, d)
	t.Cleanup(w.Stop)
	return w
}

func waitRevert(t *testing.T, d *fakeDaemon) string {
	t.Helper()
	select {
	case got := <-d.reverted:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("the window never reverted")
		return ""
	}
}

// TestConfirmWindowWorkerRevertsAtDeadline proves AC-14 and AC-15 at the
// worker: a confirmed commit applies at once, persists its record, and with no
// accept the worker restores the rollback at the deadline, whether or not the
// session that started it is still there (the worker holds no session).
func TestConfirmWindowWorkerRevertsAtDeadline(t *testing.T) {
	d := newFakeDaemon("v1")
	w := newTestWindow(t, d)

	require.NoError(t, w.Confirmed("alice", 50*time.Millisecond, false, d.commit("v2")))
	assert.Equal(t, "v2", d.config(), "the change applies at once")
	saved := d.saved()
	require.NotNil(t, saved, "the pending record is persisted")
	assert.Equal(t, "alice", saved.User)
	assert.Equal(t, "v1", string(saved.Rollback))

	assert.Equal(t, "v1", waitRevert(t, d))
	assert.Equal(t, "v1", d.config())
	assert.Nil(t, d.saved(), "the record is cleared after the revert")
	_, pending := w.Status()
	assert.False(t, pending)
}

// TestConfirmWindowOwnerIsTheUser proves AC-17 at the worker: the window
// belongs to the user name, so any session of alice accepts it, and the
// accept clears the record and stops the revert.
func TestConfirmWindowOwnerIsTheUser(t *testing.T) {
	d := newFakeDaemon("v1")
	w := newTestWindow(t, d)

	require.NoError(t, w.Confirmed("alice", 200*time.Millisecond, false, d.commit("v2")))
	status, pending := w.Status()
	require.True(t, pending)
	assert.Equal(t, "alice", status.User)
	assert.Positive(t, status.Left)

	require.NoError(t, w.Accept("alice"), "a new session of alice is the owner")
	assert.Nil(t, d.saved())
	select {
	case got := <-d.reverted:
		t.Fatalf("an accepted window reverted to %q", got)
	case <-time.After(400 * time.Millisecond):
	}
	assert.Equal(t, "v2", d.config())
}

// TestConfirmWindowRefusesOtherUsers proves AC-18 (b): another user's commit
// now, confirmed, accept and abort are refused naming the owner and the time
// left, and nothing changes.
func TestConfirmWindowRefusesOtherUsers(t *testing.T) {
	d := newFakeDaemon("v1")
	w := newTestWindow(t, d)
	require.NoError(t, w.Confirmed("alice", time.Minute, false, d.commit("v2")))

	for name, call := range map[string]func() error{
		"commit now":             func() error { return w.Now("bob", d.commit("v3")) },
		"commit confirmed":       func() error { return w.Confirmed("bob", time.Minute, false, d.commit("v3")) },
		"commit confirmed force": func() error { return w.Confirmed("bob", time.Minute, true, d.commit("v3")) },
		"commit accept":          func() error { return w.Accept("bob") },
		"commit abort":           func() error { return w.Abort("bob") },
	} {
		err := call()
		var other *OtherUserError
		require.ErrorAs(t, err, &other, name)
		assert.Equal(t, "alice", other.Owner, name)
		assert.Contains(t, err.Error(), "alice", name)
		assert.Contains(t, err.Error(), "seconds left", name)
	}
	assert.Equal(t, "v2", d.config(), "nothing another user ran changed the daemon")
	require.NotNil(t, d.saved())
	assert.Equal(t, "alice", d.saved().User)
}

// TestConfirmWindowCommitNowRefused proves AC-18 (a): the owner's commit now
// and a plain nested commit confirmed are refused with ErrPending, and the
// window and its deadline are unchanged.
func TestConfirmWindowCommitNowRefused(t *testing.T) {
	d := newFakeDaemon("v1")
	w := newTestWindow(t, d)
	require.NoError(t, w.Confirmed("alice", time.Minute, false, d.commit("v2")))
	before := d.saved().Deadline

	require.ErrorIs(t, w.Now("alice", d.commit("v3")), ErrPending)
	require.ErrorIs(t, w.Confirmed("alice", 30*time.Second, false, d.commit("v3")), ErrPending)
	assert.Equal(t, "v2", d.config())
	assert.Equal(t, before, d.saved().Deadline, "the deadline is unchanged")
	assert.Contains(t, ErrPending.Error(), "commit accept")
	assert.Contains(t, ErrPending.Error(), "commit abort")
	assert.Contains(t, ErrPending.Error(), "commit confirmed <seconds> force")
}

// TestConfirmWindowNestedRevertsToFirst proves AC-23: a nested confirmed
// commit with force applies and restarts the countdown, and the revert
// restores the state before the FIRST unconfirmed commit; abort does the same
// at once; a nested commit that fails keeps the first window's deadline.
func TestConfirmWindowNestedRevertsToFirst(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		d := newFakeDaemon("v1")
		w := newTestWindow(t, d)
		require.NoError(t, w.Confirmed("alice", time.Minute, false, d.commit("v2")))
		require.NoError(t, w.Confirmed("alice", 50*time.Millisecond, true, d.commit("v3")))
		assert.Equal(t, "v3", d.config(), "the nested commit applies")
		assert.Equal(t, "v1", string(d.saved().Rollback), "the rollback stays the first one")
		assert.Equal(t, "v1", waitRevert(t, d), "the countdown restarted at the nested seconds")
	})
	t.Run("abort", func(t *testing.T) {
		d := newFakeDaemon("v1")
		w := newTestWindow(t, d)
		require.NoError(t, w.Confirmed("alice", time.Minute, false, d.commit("v2")))
		require.NoError(t, w.Confirmed("alice", time.Minute, true, d.commit("v3")))
		require.NoError(t, w.Abort("alice"))
		assert.Equal(t, "v1", waitRevert(t, d))
		assert.Nil(t, d.saved())
		_, pending := w.Status()
		assert.False(t, pending)
	})
	t.Run("failing nested commit", func(t *testing.T) {
		d := newFakeDaemon("v1")
		w := newTestWindow(t, d)
		require.NoError(t, w.Confirmed("alice", time.Minute, false, d.commit("v2")))
		before := d.saved().Deadline
		refused := errors.New("validation failed")
		failing := Commit{
			Snapshot: func() ([]byte, error) { return []byte(d.config()), nil },
			Apply:    func() error { return refused },
		}
		require.ErrorIs(t, w.Confirmed("alice", time.Second, true, failing), refused)
		assert.Equal(t, before, d.saved().Deadline, "the first deadline stands")
		assert.Equal(t, "v2", d.config())
	})
}

// TestConfirmWindowAcceptKeepsCandidateEdits proves AC-24 at the worker:
// accept ends the window and applies nothing, so edits the owner made and did
// not commit stay out of the daemon.
func TestConfirmWindowAcceptKeepsCandidateEdits(t *testing.T) {
	d := newFakeDaemon("v1")
	w := newTestWindow(t, d)
	require.NoError(t, w.Confirmed("alice", time.Minute, false, d.commit("v2")))
	require.NoError(t, w.Accept("alice"))
	assert.Equal(t, "v2", d.config(), "accept keeps the applied config and applies nothing else")
	require.NoError(t, w.Now("alice", d.commit("v3")), "commit now runs again once the window is closed")
	assert.Equal(t, "v3", d.config())
}

// TestConfirmWindowAcceptAbortOutsideWindow proves accept and abort with no
// window open are refused with ErrNoWindow, and change nothing.
func TestConfirmWindowAcceptAbortOutsideWindow(t *testing.T) {
	d := newFakeDaemon("v1")
	w := newTestWindow(t, d)
	require.ErrorIs(t, w.Accept("alice"), ErrNoWindow)
	require.ErrorIs(t, w.Abort("alice"), ErrNoWindow)
	assert.Equal(t, "v1", d.config())
}

// TestConfirmWindowRevertsOnStart proves AC-19: a record left by a daemon that
// stopped during a window is reverted at start and cleared; no record reverts
// nothing.
func TestConfirmWindowRevertsOnStart(t *testing.T) {
	d := newFakeDaemon("v2")
	reverted, err := RecoverOnStart(&Pending{User: "alice", Deadline: time.Now().Add(time.Minute), Rollback: []byte("v1")}, d.Revert, d)
	require.NoError(t, err)
	assert.True(t, reverted)
	assert.Equal(t, "v1", waitRevert(t, d))
	assert.Nil(t, d.saved())

	reverted, err = RecoverOnStart(nil, d.Revert, d)
	require.NoError(t, err)
	assert.False(t, reverted)
}

// TestConfirmWindowStopped proves a call after Stop answers ErrStopped
// instead of blocking on a worker that is gone.
func TestConfirmWindowStopped(t *testing.T) {
	d := newFakeDaemon("v1")
	w := NewWindow(d.Revert, d)
	w.Stop()
	require.ErrorIs(t, w.Now("alice", d.commit("v2")), ErrStopped)
	w.Stop()
}
