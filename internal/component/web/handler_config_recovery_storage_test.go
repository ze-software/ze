// Design: docs/architecture/web-interface.md -- recovery reports the observed storage outcome.
package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/audit"
)

// TestConfigRecoveryStorageOutcome injects the errors after an actual removal,
// proving that audit and cached state follow that action rather than its error.
func TestConfigRecoveryStorageOutcome(t *testing.T) {
	for _, failure := range []string{"release", "removed-error", "unknown-error"} {
		t.Run(failure, func(t *testing.T) {
			mgr := newTestEditorManager(t)
			require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.199"))
			us, err := mgr.GetOrCreate("testuser")
			require.NoError(t, err)
			path := cli.ChangePath(mgr.configPath, "testuser")
			require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
			mgr.store = &configRecoveryFailureStore{Storage: mgr.store, failure: failure}
			renderer, err := NewRenderer()
			require.NoError(t, err)
			recorder, err := audit.NewMemory(100)
			require.NoError(t, err)
			response := httptest.NewRecorder()
			HandleConfigRecovery(mgr, renderer, fakeAuthorizer{allowEdit: true}, recorder)(response,
				authedRequest(t, http.MethodPost, "/config/recover", url.Values{
					"confirm": {"discard"}, "fingerprint": {recoveryFingerprint(corruptRecoveryContent)},
				}))
			require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
			assert.False(t, mgr.store.Exists(path), "the injected fault follows a real removal")
			assert.NotContains(t, response.Body.String(), "Recovery kept the file")
			assert.NotContains(t, us.editor.WorkingContent(), "192.0.2.199", "the cached candidate must forget removed intent")
			assert.Zero(t, mgr.ChangeCount("testuser"))
			rows := recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard})
			require.Len(t, rows, 1)
			if failure == "unknown-error" {
				assert.Equal(t, audit.OutcomeError, rows[0].Outcome)
				assert.Contains(t, response.Body.String(), "could not confirm")
				return
			}
			assert.Equal(t, audit.OutcomeSuccess, rows[0].Outcome)
			assert.Contains(t, response.Body.String(), "was removed")
		})
	}
}

// TestConfigRecoveryUncertainRemovalDoesNotParseUnknownFile leaves the corrupt
// file present but makes removal readback fail. Recovery must retain uncertainty
// and fence the old candidate without parsing secret-bearing input again.
func TestConfigRecoveryUncertainRemovalDoesNotParseUnknownFile(t *testing.T) {
	mgr := newTestEditorManager(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.199"))
	us, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	path := cli.ChangePath(mgr.configPath, "testuser")
	const broken = "set unknown-private-recovery-sentinel value\n"
	require.NoError(t, mgr.store.WriteFile(path, []byte(broken), 0o600))
	mgr.store = &configRecoveryFailureStore{Storage: mgr.store, failure: "unknown-kept"}
	renderer, err := NewRenderer()
	require.NoError(t, err)
	response := httptest.NewRecorder()
	HandleConfigRecovery(mgr, renderer, fakeAuthorizer{allowEdit: true}, nil)(response,
		authedRequest(t, http.MethodPost, "/config/recover", url.Values{
			"confirm": {"discard"}, "fingerprint": {recoveryFingerprint(broken)},
		}))
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.Contains(t, response.Body.String(), "could not confirm")
	require.NotContains(t, response.Body.String(), "unknown-private-recovery-sentinel")
	require.NotContains(t, response.Body.String(), "was kept")
	require.Nil(t, us.editor.Tree())
	require.Error(t, us.editor.SetValue([]string{"bgp"}, "router-id", "192.0.2.200"))
	failureStore, ok := mgr.store.(*configRecoveryFailureStore)
	require.True(t, ok)
	require.Equal(t, 2, failureStore.guards, "uncertain removal must not resume the unknown file")
}

// TestConfigRecoveryReplacementParseFailureHidesContent injects a new malformed
// file after confirmed removal and before cached resumption, as another session
// can do. Its bytes stay private while the original removal remains audited.
func TestConfigRecoveryReplacementParseFailureHidesContent(t *testing.T) {
	mgr := newTestEditorManager(t)
	require.NoError(t, mgr.SetValue("testuser", []string{"bgp"}, "router-id", "192.0.2.199"))
	us, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	path := cli.ChangePath(mgr.configPath, "testuser")
	const replacement = "set unknown-private-replacement-sentinel value\n"
	us.editor = replacingRecoveryEditor{Editor: us.editor, store: mgr.store, path: path, replacement: replacement}
	require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	response := httptest.NewRecorder()
	HandleConfigRecovery(mgr, renderer, fakeAuthorizer{allowEdit: true}, recorder)(response,
		authedRequest(t, http.MethodPost, "/config/recover", url.Values{
			"confirm": {"discard"}, "fingerprint": {recoveryFingerprint(corruptRecoveryContent)},
		}))
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.Contains(t, response.Body.String(), "was discarded")
	require.NotContains(t, response.Body.String(), "unknown-private-replacement-sentinel")
	require.Nil(t, us.editor.Tree())
	data, err := mgr.store.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, replacement, string(data))
	rows := recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard})
	require.Len(t, rows, 1)
	require.Equal(t, audit.OutcomeSuccess, rows[0].Outcome)
}

type replacingRecoveryEditor struct {
	contract.Editor
	store       storage.Storage
	path        string
	replacement string
}

func (e replacingRecoveryEditor) ResumeSession() error {
	if err := e.store.WriteFile(e.path, []byte(e.replacement), 0o600); err != nil {
		return err
	}
	return e.Editor.ResumeSession()
}

var errConfigRecoveryStorage = errors.New("injected recovery storage failure")

type configRecoveryFailureStore struct {
	storage.Storage
	failure string
	guards  int
}

func (s *configRecoveryFailureStore) AcquireLock(path string) (storage.WriteGuard, error) {
	guard, err := s.Storage.AcquireLock(path)
	if err != nil {
		return nil, err
	}
	s.guards++
	// The POST first inspects, then removes. Resume receives a clean guard so
	// this test isolates the removal result from a separate refresh failure.
	if s.guards == 2 {
		return &configRecoveryFailureGuard{WriteGuard: guard, failure: s.failure}, nil
	}
	return guard, nil
}

type configRecoveryFailureGuard struct {
	storage.WriteGuard
	failure string
	removed bool
}

func (g *configRecoveryFailureGuard) Remove(path string) error {
	if g.failure == "unknown-kept" {
		g.removed = true // Marks the attempted removal for the readback fault.
		return errConfigRecoveryStorage
	}
	if err := g.WriteGuard.Remove(path); err != nil {
		return err
	}
	g.removed = true
	if g.failure == "release" {
		return nil
	}
	return errConfigRecoveryStorage
}

func (g *configRecoveryFailureGuard) ReadFile(path string) ([]byte, error) {
	if g.removed {
		if g.failure == "unknown-kept" {
			return nil, errConfigRecoveryStorage
		}
		if g.failure == "unknown-error" {
			return nil, errConfigRecoveryStorage
		}
	}
	return g.WriteGuard.ReadFile(path)
}

func (g *configRecoveryFailureGuard) Release() error {
	err := g.WriteGuard.Release()
	if g.failure == "release" {
		return errors.Join(err, errConfigRecoveryStorage)
	}
	return err
}
