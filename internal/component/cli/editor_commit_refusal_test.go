package cli

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// errDraftMedium is the read failure failingReadStore injects.
var errDraftMedium = errors.New("draft medium failure")

// failingReadStore wraps a storage and fails every locked read of one name
// with errDraftMedium once armed, so a test can reach the branch a commit
// takes when the draft exists but cannot be read.
type failingReadStore struct {
	storage.Storage
	name  string
	armed bool
}

func (s *failingReadStore) AcquireLock(path string) (storage.WriteGuard, error) {
	guard, err := s.Storage.AcquireLock(path)
	if err != nil {
		return nil, err
	}
	return &failingReadGuard{WriteGuard: guard, store: s}, nil
}

type failingReadGuard struct {
	storage.WriteGuard
	store *failingReadStore
}

func (g *failingReadGuard) ReadFile(name string) ([]byte, error) {
	if g.store.armed && name == g.store.name {
		return nil, errDraftMedium
	}
	return g.WriteGuard.ReadFile(name)
}

// commitPaths names the two session commit entry points; both read the draft
// and the change file, so every refusal below holds for each of them.
var commitPaths = []struct {
	name   string
	commit func(ed *Editor) (*CommitResult, error)
}{
	{"CommitSession", func(ed *Editor) (*CommitResult, error) { return ed.CommitSession() }},
	{"CommitSessionCandidate", func(ed *Editor) (*CommitResult, error) {
		result, _, err := ed.CommitSessionCandidate(time.Now())
		return result, err
	}},
}

// VALIDATES: a commit whose draft exists but cannot be read fails, and the
// error names the read failure.
// PREVENTS: the old answer, CommitResult{Applied: 0} with a nil error, which
// the editors print as a successful commit that applied nothing.
func TestCommitRefusesUnreadableDraft(t *testing.T) {
	for _, path := range commitPaths {
		t.Run(path.name, func(t *testing.T) {
			configPath := writeTestConfig(t, validBGPConfig)
			store := &failingReadStore{Storage: newTestTreeStore(t, configPath), name: DraftPath(configPath)}
			ed, err := NewEditorWithStorage(store, configPath)
			require.NoError(t, err)
			t.Cleanup(func() { _ = ed.Close() })
			ed.SetSession(NewEditSession("thomas", "ssh"))
			require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))

			store.armed = true
			result, err := path.commit(ed)
			require.Error(t, err, "an unreadable draft fails the commit, got result %+v", result)
			assert.ErrorIs(t, err, errDraftMedium, "the error carries the read failure")
		})
	}
}

// VALIDATES: a commit over a change file that does not parse fails, names the
// change file, and leaves the file in place for the operator to inspect.
// PREVENTS: readChangeFile's old warn-and-discard, which treated the pending
// changes as absent, so the commit answered success with nothing applied and
// the next edit overwrote the file.
func TestCommitRefusesCorruptChangeFile(t *testing.T) {
	for _, path := range commitPaths {
		t.Run(path.name, func(t *testing.T) {
			configPath := writeTestConfig(t, validBGPConfig)
			store := newTestTreeStore(t, configPath)
			ed, err := NewEditorWithStorage(store, configPath)
			require.NoError(t, err)
			t.Cleanup(func() { _ = ed.Close() })
			session := NewEditSession("thomas", "ssh")
			ed.SetSession(session)
			require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))

			changePath := ChangePath(configPath, session.User)
			corrupt := []byte("set no-such-container leaf x\n")
			require.NoError(t, store.WriteFile(changePath, corrupt, 0o600))

			result, err := path.commit(ed)
			require.Error(t, err, "a corrupt change file fails the commit, got result %+v", result)
			assert.Contains(t, err.Error(), changePath, "the error names the change file")

			kept, err := store.ReadFile(changePath)
			require.NoError(t, err, "the change file is kept")
			assert.Equal(t, corrupt, kept, "the change file is left as it was")
		})
	}
}

// VALIDATES: an edit over a change file that does not parse fails and leaves
// the file as it was.
// PREVENTS: the write-through rewriting the change file from an empty tree,
// which dropped every pending change it held.
func TestEditRefusesCorruptChangeFile(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	store := newTestTreeStore(t, configPath)
	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })
	session := NewEditSession("thomas", "ssh")
	ed.SetSession(session)

	changePath := ChangePath(configPath, session.User)
	corrupt := []byte("set no-such-container leaf x\n")
	require.NoError(t, store.WriteFile(changePath, corrupt, 0o600))

	err = ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9")
	require.Error(t, err, "an edit over a corrupt change file fails")
	assert.Contains(t, err.Error(), changePath, "the error names the change file")

	kept, err := store.ReadFile(changePath)
	require.NoError(t, err)
	assert.Equal(t, corrupt, kept, "the change file is left as it was")
}
