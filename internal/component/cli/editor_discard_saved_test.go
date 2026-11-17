// Design: docs/architecture/config/change-file-structural-ops.md -- retryable saved discard.
// Related: editor_discard_saved.go -- saved ownership and materialization cleanup.

package cli

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestDiscardSavedDraftFailuresKeepChangeRecords injects a failed draft read,
// parse or write, then retries the same discard after repair. A refusal keeps
// durable change records and committed bytes intact until cleanup succeeds.
func TestDiscardSavedDraftFailuresKeepChangeRecords(t *testing.T) {
	for _, stage := range []string{"read", "parse", "write"} {
		t.Run(stage, func(t *testing.T) {
			editor, session, path := newLoadSessionEditor(t, validBGPConfig)
			require.NoError(t, editor.SetValue([]string{"bgp"}, "router-id", "10.0.0.99"))
			require.NoError(t, editor.SaveDraft())
			require.NoError(t, editor.SetValue([]string{"bgp", "group", "pending"}, "description", "unsaved"))
			store := editor.store
			draftPath := DraftPath(path)
			draft, err := store.ReadFile(draftPath)
			require.NoError(t, err)
			if stage == "parse" {
				require.NoError(t, store.WriteFile(draftPath, []byte("set unknown leaf invalid\n"), 0o600))
			}
			before, err := store.ReadFile(draftPath)
			require.NoError(t, err)
			changes, err := store.ReadFile(ChangePath(path, session.User))
			require.NoError(t, err)
			committed, err := store.ReadFile(path)
			require.NoError(t, err)
			failure := errors.New("injected saved draft " + stage + " failure")
			editor.store = &discardSavedFaultStore{Storage: store, path: draftPath, stage: stage, failure: failure}
			err = editor.DiscardSessionPath(nil)
			require.Error(t, err)
			if stage != "parse" {
				assert.ErrorIs(t, err, failure)
			}
			after, err := store.ReadFile(draftPath)
			require.NoError(t, err)
			assert.Equal(t, before, after)
			after, err = store.ReadFile(ChangePath(path, session.User))
			require.NoError(t, err)
			assert.Equal(t, changes, after)
			after, err = store.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, committed, after)
			editor.store = store
			if stage == "parse" {
				require.NoError(t, store.WriteFile(draftPath, draft, 0o600))
			}
			require.NoError(t, editor.DiscardSessionPath(nil))
			assert.Empty(t, editor.PendingChanges(session.ID))
		})
	}
}

// TestDiscardSavedDraftFailureHasCorrectiveAction reaches a storage refusal
// through the session discard entry point and checks its typed operator advice
// without losing the underlying cause used by callers and diagnostics.
func TestDiscardSavedDraftFailureHasCorrectiveAction(t *testing.T) {
	editor, _, path := newLoadSessionEditor(t, validBGPConfig)
	require.NoError(t, editor.SetValue([]string{"bgp"}, "router-id", "10.0.0.99"))
	require.NoError(t, editor.SaveDraft())
	store := editor.store
	failure := errors.New("saved draft is unavailable")
	editor.store = &discardSavedFaultStore{Storage: store, path: DraftPath(path), stage: "read", failure: failure}
	err := editor.DiscardSessionPath(nil)
	editor.store = store
	require.ErrorIs(t, err, failure)
	var advice *opererr.Error
	require.ErrorAs(t, err, &advice)
	assert.Contains(t, advice.Problem(), "saved configuration changes")
	assert.Contains(t, advice.Why(), "running configuration is unchanged")
	assert.Contains(t, advice.Why(), "pending")
	assert.Contains(t, advice.Why(), "retry")
	assert.Equal(t, "run show doctor", advice.Fix())
	assert.True(t, strings.HasSuffix(err.Error(), "\nfix: run show doctor"))
}

type discardSavedFaultStore struct {
	storage.Storage
	path    string
	stage   string
	failure error
}

func (s *discardSavedFaultStore) AcquireLock(path string) (storage.WriteGuard, error) {
	guard, err := s.Storage.AcquireLock(path)
	if err != nil {
		return nil, err
	}
	return &discardSavedFaultGuard{WriteGuard: guard, fault: s}, nil
}

type discardSavedFaultGuard struct {
	storage.WriteGuard
	fault *discardSavedFaultStore
}

func (g *discardSavedFaultGuard) ReadFile(path string) ([]byte, error) {
	if path == g.fault.path {
		if g.fault.stage == "read" {
			return nil, g.fault.failure
		}
	}
	return g.WriteGuard.ReadFile(path)
}

func (g *discardSavedFaultGuard) WriteFile(path string, data []byte, mode fs.FileMode) error {
	if path == g.fault.path {
		if g.fault.stage == "write" {
			return g.fault.failure
		}
	}
	return g.WriteGuard.WriteFile(path, data, mode)
}
