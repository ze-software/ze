//go:build ze_web

// Design: docs/architecture/web-interface.md -- Per-User Editor recovery.
// Related: web_editor_recovery_test.go -- authenticated recovery fixture.

package hub

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	zeconfigcmd "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	zeweb "github.com/ze-software/ze/internal/component/web"
)

// TestWebEditorRecoveryStorageFailureKeepsCustody proves a failed open never
// publishes an editor, and that both a retry and a fresh manager recover the
// intent after a refusal before either write or after final publication.
// The pinned session deliberately gives both attempts the same RFC3339 ID.
func TestWebEditorRecoveryStorageFailureKeepsCustody(t *testing.T) {
	for _, boundary := range []string{"draft", "change", "release"} {
		for _, restart := range []bool{false, true} {
			name := boundary + "/retry"
			if restart {
				name = boundary + "/new-manager"
			}
			t.Run(name, func(t *testing.T) {
				fixture := newWebEditorRecoveryFixture(t)
				require.NoError(t, fixture.alice.SaveDraft())
				fault := &webRecoveryFaultStore{Storage: fixture.store, boundary: boundary}
				fixture.store = fault
				manager := fixture.managerWithPinnedSession(t)
				response := fixture.get(t, manager, "finder", "/show/system/")
				require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
				assert.Contains(t, response.Body.String(), "synthetic recovery publication failure")
				require.True(t, fault.failed, "the requested recovery boundary must have been reached")
				assert.False(t, manager.ReadTree("alice", func(*zeconfig.Tree) {}), "failed publication must not cache an editor")
				committed, err := fixture.store.ReadFile("config.conf")
				require.NoError(t, err)
				assert.Equal(t, []byte(webEditorRecoveryBase), committed)
				fixture.checkOwners(t)
				if restart {
					manager = fixture.managerWithPinnedSession(t)
				}
				fixture.checkCandidate(t, manager, "finder")
				fixture.checkCommit(t, manager)
			})
		}
	}
}

type webRecoveryFaultStore struct {
	storage.Storage
	boundary string
	failed   bool
}

func (s *webRecoveryFaultStore) AcquireLock(path string) (storage.WriteGuard, error) {
	guard, err := s.Storage.AcquireLock(path)
	if err != nil {
		return nil, err
	}
	return &webRecoveryFaultGuard{WriteGuard: guard, store: s}, nil
}

type webRecoveryFaultGuard struct {
	storage.WriteGuard
	store *webRecoveryFaultStore
}

func (g *webRecoveryFaultGuard) WriteFile(path string, content []byte, mode fs.FileMode) error {
	if !g.store.failed {
		target := cli.DraftPath("config.conf")
		if g.store.boundary == "change" {
			target = cli.ChangePath("config.conf", "alice")
		}
		if g.store.boundary != "release" {
			if path == target {
				g.store.failed = true
				return errors.New("synthetic recovery publication failure")
			}
		}
	}
	return g.WriteGuard.WriteFile(path, content, mode)
}

func (g *webRecoveryFaultGuard) Release() error {
	err := g.WriteGuard.Release()
	if g.store.boundary == "release" {
		if !g.store.failed {
			g.store.failed = true
			return errors.Join(err, errors.New("synthetic recovery publication failure"))
		}
	}
	return err
}

// managerWithPinnedSession controls identity at the adapter boundary, without
// changing NewEditSession's clock or sleeping until another second arrives.
func (f *webEditorRecoveryFixture) managerWithPinnedSession(t *testing.T) *zeweb.EditorManager {
	t.Helper()
	factory := func(store any, path string) (contract.Editor, error) {
		editor, err := newEditorFactory(zeconfigcmd.ValidateContent)(store, path)
		if err != nil {
			return nil, err
		}
		adapter, ok := editor.(*editorAdapter)
		if !ok {
			return nil, fmt.Errorf("test editor factory returned %T, expected *editorAdapter", editor)
		}
		return &webRecoveryPinnedEditor{adapter}, nil
	}
	manager := zeweb.NewEditorManager(f.store, "config.conf", f.schema, factory, newEditSessionFactory())
	manager.SetCommitHook(func() error { return storage.PromoteCandidate(f.store, "config.conf") })
	t.Cleanup(func() { require.NoError(t, manager.Close()) })
	return manager
}

type webRecoveryPinnedEditor struct {
	*editorAdapter
}

func (e *webRecoveryPinnedEditor) SetSession(session contract.EditSession) {
	started := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	e.ed.SetSession(&cli.EditSession{
		User: session.User, Origin: session.Origin, StartTime: started,
		ID: session.User + "@" + session.Origin + "%" + started.Format(time.RFC3339),
	})
}
