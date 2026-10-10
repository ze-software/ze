package cli

import (
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// discardNoticeFixture is a config on one store and bob, ready to force over
// alice's changes.
type discardNoticeFixture struct {
	store      storage.Storage
	configPath string
	bob        *Editor
}

func newDiscardNoticeFixture(t *testing.T) *discardNoticeFixture {
	t.Helper()
	configPath := writeTestConfig(t, validBGPConfig)
	f := &discardNoticeFixture{store: newTestTreeStore(t, configPath), configPath: configPath}
	f.bob = f.session(t, f.store, "bob", "ssh")
	return f
}

// session opens an editor for user on store.
func (f *discardNoticeFixture) session(t *testing.T, store storage.Storage, user, origin string) *Editor {
	t.Helper()
	ed, err := NewEditorWithStorage(store, f.configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // test cleanup
	ed.SetSession(NewEditSession(user, origin))
	return ed
}

// force has alice set router-id through holder, then bob force over it.
func (f *discardNoticeFixture) force(t *testing.T, holder *Editor, aliceID, bobID string) {
	t.Helper()
	require.NoError(t, holder.SetValue([]string{"bgp"}, "router-id", aliceID))
	require.NoError(t, f.bob.SetValue([]string{"bgp"}, "router-id", bobID))
	forced, err := f.bob.CommitSessionForce()
	require.NoError(t, err)
	require.Equal(t, 1, forced.Applied)
}

// ageShownLines rewrites every shown stamp in alice's notice log to an hour
// ago, as if the log had sat that long since a session showed it.
func (f *discardNoticeFixture) ageShownLines(t *testing.T) {
	t.Helper()
	path := DiscardNoticePath(f.configPath, "alice")
	data, err := f.store.ReadFile(path)
	require.NoError(t, err)
	old := strconv.FormatInt(time.Now().Add(-time.Hour).UnixNano(), 10)
	var out strings.Builder
	for line := range strings.Lines(string(data)) {
		fields := strings.SplitN(line, "\t", 4)
		require.Len(t, fields, 4, "notice line %q", line)
		if fields[1] != "0" {
			fields[1] = old
		}
		out.WriteString(strings.Join(fields, "\t"))
	}
	require.NoError(t, f.store.WriteFile(path, []byte(out.String()), 0o600))
}

// TestDiscardNoticeKeptUntilEverySessionShowsIt is review round 3 ISSUE 2 of
// spec-session-editor-file-mode-parity: a line shown by one session was pruned
// a minute later, so a session of the same user that had not shown it yet (a
// web session with no page open) was never told.
//
// GOAL: a notice stays until every session of the user that existed when it
// was written has shown it, however long that takes.
// METHOD: alice has an SSH and a web session; bob forces; the SSH session
// shows the notice; the log ages an hour; bob forces again, which rewrites
// the log; then the web session reads its notice.
//
// VALIDATES: the web session is told of both discards.
// PREVENTS: a time-based prune dropping a notice a session never showed.
func TestDiscardNoticeKeptUntilEverySessionShowsIt(t *testing.T) {
	f := newDiscardNoticeFixture(t)
	ssh := f.session(t, f.store, "alice", "ssh")
	web := f.session(t, f.store, "alice", "web")

	f.force(t, ssh, "10.0.0.1", "10.0.0.2")
	require.Contains(t, ssh.TakeDiscardNotice(), "discarded by bob")
	f.ageShownLines(t)
	f.force(t, ssh, "10.0.0.3", "10.0.0.4")

	notice, through := web.PendingDiscardNotice()
	assert.Equal(t, 2, strings.Count(notice, "was discarded by bob"), "the web session is told of both discards: %q", notice)
	assert.NotZero(t, through)
}

// TestDiscardNoticeForALaterSession is review round 3 ISSUE 2 of
// spec-session-editor-file-mode-parity: notices are no longer pruned by age,
// so a session opened later must not be told of a discard another session of
// the user already showed, and must be told of one no session showed.
//
// GOAL: a later session reports exactly the discards no session reported.
// METHOD: bob forces twice over alice; her SSH session shows the first notice
// and is gone before the second; then a new alice session reads its notice.
//
// VALIDATES: the new session reports the second discard only.
// PREVENTS: a stale notice reported again, or an unread one never reported.
func TestDiscardNoticeForALaterSession(t *testing.T) {
	f := newDiscardNoticeFixture(t)
	ssh := f.session(t, f.store, "alice", "ssh")
	f.force(t, ssh, "10.0.0.1", "10.0.0.2")
	require.Contains(t, ssh.TakeDiscardNotice(), "discarded by bob")
	f.force(t, ssh, "10.0.0.3", "10.0.0.4")

	later := f.session(t, f.store, "alice", "web")
	notice, _ := later.PendingDiscardNotice()
	assert.Equal(t, 1, strings.Count(notice, "was discarded by bob"), "only the unshown discard: %q", notice)
}

// lockFailingStore refuses AcquireLock while fail is set.
type lockFailingStore struct {
	storage.Storage
	fail *atomic.Bool
}

var errLockRefused = errors.New("lock refused by the test store")

func (s lockFailingStore) AcquireLock(path string) (storage.WriteGuard, error) {
	if s.fail.Load() {
		return nil, errLockRefused
	}
	return s.Storage.AcquireLock(path)
}

// TestDiscardNoticeAckThatFailsKeepsItPending is review round 3 NOTE 4 of
// spec-session-editor-file-mode-parity: AckDiscardNotice advanced the
// session's seen marker before taking the lock, so an ack whose lock failed
// neither rebuilt the view nor offered the notice again.
//
// GOAL: a failed ack leaves the notice pending for the session.
// METHOD: alice's session runs on a store whose lock is refused during her
// ack; the lock then works and she reads her notice again.
//
// VALIDATES: the ack returns the lock error and the notice is still pending.
// PREVENTS: a discarded value shown with no notice and no rebuilt view.
func TestDiscardNoticeAckThatFailsKeepsItPending(t *testing.T) {
	f := newDiscardNoticeFixture(t)
	fail := &atomic.Bool{}
	alice := f.session(t, lockFailingStore{Storage: f.store, fail: fail}, "alice", "ssh")
	f.force(t, alice, "10.0.0.1", "10.0.0.2")

	notice, through := alice.PendingDiscardNotice()
	require.Contains(t, notice, "discarded by bob")
	fail.Store(true)
	require.ErrorIs(t, alice.AckDiscardNotice(through), errLockRefused)
	fail.Store(false)

	again, _ := alice.PendingDiscardNotice()
	assert.Equal(t, notice, again, "the notice is offered again after a failed ack")
}
