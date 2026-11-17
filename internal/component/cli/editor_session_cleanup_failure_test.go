// Design: docs/guide/config-editor.md -- warnings on an applied commit.
// Related: editor_session_cleanup_test.go -- unrelated ownership controls.

package cli

import (
	"errors"
	"io/fs"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestSessionCleanupFailureKeepsAppliedResult refuses the existing guarded
// draft removal only after the real publication write. Both commit paths must
// report that cleanup failed while preserving the applied config and the other
// session's intent; the fault does not replace the publication producer.
func TestSessionCleanupFailureKeepsAppliedResult(t *testing.T) {
	for _, route := range []string{"direct", "candidate"} {
		t.Run(route, func(t *testing.T) {
			fixture := newSessionCleanupFixture(t, "ssh")
			store := &sessionCleanupFailureStore{Storage: fixture.store,
				configPath: fixture.path, draftPath: DraftPath(fixture.path)}
			fixture.store = store
			fixture.current.store = store
			if route == "direct" {
				result, err := fixture.current.CommitSession()
				require.NoError(t, err, "the configuration was already published")
				require.NotNil(t, result)
				assert.Equal(t, 1, result.Applied)
				require.Len(t, result.Warnings, 1)
				assert.Contains(t, result.Warnings[0], errSessionCleanupRemove.Error())
				assert.Contains(t, result.Warnings[0], "commit applied")
			} else {
				result, content, err := fixture.current.CommitSessionCandidate(time.Now())
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, 1, result.Applied)
				require.NoError(t, storage.PromoteCandidate(store, fixture.path))
				err = fixture.current.MarkCommittedContent(content)
				require.ErrorIs(t, err, errSessionCleanupRemove)
				assert.Contains(t, err.Error(), "commit applied")
			}
			require.True(t, store.published, "the refusal must follow the real config write")
			require.Equal(t, 1, store.refused, "the original cleanup boundary must execute")
			committed, err := store.ReadFile(fixture.path)
			require.NoError(t, err)
			assert.Contains(t, string(committed), "web-committed")
			fixture.checkOther(t)
		})
	}
}

var errSessionCleanupRemove = errors.New("injected removal refusal after publication")

// This store is confined to one synchronous test: no goroutine borrows it.
type sessionCleanupFailureStore struct {
	storage.Storage
	configPath string
	draftPath  string
	published  bool
	refused    int
}

func (s *sessionCleanupFailureStore) AcquireLock(path string) (storage.WriteGuard, error) {
	guard, err := s.Storage.AcquireLock(path)
	if err != nil {
		return nil, err
	}
	return sessionCleanupFailureGuard{WriteGuard: guard, owner: s}, nil
}

type sessionCleanupFailureGuard struct {
	storage.WriteGuard
	owner *sessionCleanupFailureStore
}

func (g sessionCleanupFailureGuard) WriteFile(path string, data []byte, mode fs.FileMode) error {
	if err := g.WriteGuard.WriteFile(path, data, mode); err != nil {
		return err
	}
	if path == g.owner.configPath {
		g.owner.published = true
	}
	return nil
}

func (g sessionCleanupFailureGuard) Remove(path string) error {
	if g.owner.published {
		if path == g.owner.draftPath {
			g.owner.refused++
			return errSessionCleanupRemove
		}
	}
	return g.WriteGuard.Remove(path)
}
