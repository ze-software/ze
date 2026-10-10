package cli

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// noticeFailingStore is a store whose locked writes of a discard notice fail,
// so a forced commit lands and its discard of the overridden change does not.
type noticeFailingStore struct {
	storage.Storage
}

func (s noticeFailingStore) AcquireLock(path string) (storage.WriteGuard, error) {
	guard, err := s.Storage.AcquireLock(path)
	if err != nil {
		return nil, err
	}
	return noticeFailingGuard{WriteGuard: guard}, nil
}

type noticeFailingGuard struct {
	storage.WriteGuard
}

var errNoticeWrite = errors.New("notice write refused by the test store")

func (g noticeFailingGuard) WriteFile(path string, data []byte, mode fs.FileMode) error {
	if strings.Contains(path, ".discarded.") {
		return errNoticeWrite
	}
	return g.WriteGuard.WriteFile(path, data, mode)
}

// TestForcedCommitReportsAFailedDiscard is review round 2 ISSUE 3 of
// spec-session-editor-file-mode-parity: a forced commit that lands while its
// discard of the overridden change fails must say so, and must not say the
// commit failed.
//
// GOAL: the forcing user's answer is truthful about what applied and what
// did not, on the direct path and on the candidate path.
// METHOD: alice holds a LIVE change, bob forces over it through a store whose
// discard-notice write fails. Direct: CommitSessionForce. Candidate:
// CommitSessionCandidateForce, then MarkCommittedContent as a caller does once
// the daemon took the candidate.
//
// VALIDATES: the direct commit returns no error, the applied count, a warning
// naming the failed discard, and bob's view at the committed config; the
// candidate path's MarkCommittedContent returns the failed discard.
// PREVENTS: a plain "success" while alice's overridden change stays pending
// (logged only), and a "commit failed" after the config file was written.
func TestForcedCommitReportsAFailedDiscard(t *testing.T) {
	setup := func(t *testing.T) (*Editor, storage.Storage, string) {
		t.Helper()
		configPath := writeTestConfig(t, validBGPConfig)
		store := noticeFailingStore{Storage: newTestTreeStore(t, configPath)}

		alice, err := NewEditorWithStorage(store, configPath)
		require.NoError(t, err)
		t.Cleanup(func() { alice.Close() }) //nolint:errcheck,gosec // test cleanup
		alice.SetSession(NewEditSession("alice", "ssh"))
		require.NoError(t, alice.SetValue([]string{"bgp"}, "router-id", "10.0.0.1"))

		bob, err := NewEditorWithStorage(store, configPath)
		require.NoError(t, err)
		t.Cleanup(func() { bob.Close() }) //nolint:errcheck,gosec // test cleanup
		bob.SetSession(NewEditSession("bob", "ssh"))
		require.NoError(t, bob.SetValue([]string{"bgp"}, "router-id", "10.0.0.2"))
		return bob, store, configPath
	}

	t.Run("direct", func(t *testing.T) {
		bob, store, configPath := setup(t)
		forced, err := bob.CommitSessionForce()
		require.NoError(t, err, "the config file was written: the commit applied")
		require.NotNil(t, forced)
		assert.Equal(t, 1, forced.Applied)
		require.Len(t, forced.Warnings, 1)
		assert.Contains(t, forced.Warnings[0], errNoticeWrite.Error())
		assert.Contains(t, forced.Warnings[0], "not discarded")

		committed, err := store.ReadFile(configPath)
		require.NoError(t, err)
		assert.Contains(t, string(committed), "router-id 10.0.0.2")
		assert.Contains(t, bob.OriginalContent(), "router-id 10.0.0.2", "bob's view moved to the committed config")
	})

	t.Run("candidate", func(t *testing.T) {
		bob, _, _ := setup(t)
		forced, content, err := bob.CommitSessionCandidateForce(time.Now())
		require.NoError(t, err)
		require.Equal(t, 1, forced.Applied)
		err = bob.MarkCommittedContent(content)
		require.ErrorIs(t, err, errNoticeWrite)
		assert.Contains(t, err.Error(), "not discarded")
	})
}
