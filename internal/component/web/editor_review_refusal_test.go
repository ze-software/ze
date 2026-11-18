// Design: docs/architecture/web-interface.md -- complete, private review refusals.
package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/core/opererr"
)

type reviewFailureEditor struct {
	contract.Editor
	err error
}

func (e *reviewFailureEditor) PendingReview(path []string) (string, error) {
	if e.err != nil {
		return "", e.err
	}
	return e.Editor.PendingReview(path)
}

// TestPendingReviewFailureKeepsItsOperatorContract exercises the manager's
// actual failure boundary: native refusals stay intact, unknown and joined
// causes retain their identity, and a healthy review remains useful.
func TestPendingReviewFailureKeepsItsOperatorContract(t *testing.T) {
	cause := errors.New("review storage read failed")
	closeFailure := errors.New("review storage release failed")
	native := opererr.Wrap(cause, "Pending review is unavailable.",
		"The saved configuration could not be inspected.", "ze show doctor")
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "plain cause", err: cause},
		{name: "native refusal", err: native},
		{name: "joined causes", err: errors.Join(native, closeFailure)},
		{name: "healthy review"},
	} {
		t.Run(test.name, func(t *testing.T) {
			mgr := newTestEditorManager(t)
			require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "192.0.2.10"))
			session, err := mgr.GetOrCreate("alice")
			require.NoError(t, err)
			const review = "- bgp router-id 1.2.3.4\n+ bgp router-id 192.0.2.10\n"
			session.editor = &reviewFailureEditor{Editor: session.editor, err: test.err}
			got, err := mgr.Diff("alice")
			if test.err == nil {
				require.NoError(t, err)
				assert.Equal(t, review, got)
				return
			}
			require.Error(t, err)
			assert.Empty(t, got, "a failed producer cannot publish a partial review")
			refusal, typed := err.(*opererr.Error)
			require.True(t, typed, "review failure has no typed problem, explanation and recovery: %v", err)
			assert.NotEmpty(t, refusal.Problem())
			assert.NotEmpty(t, refusal.Why())
			assert.NotEmpty(t, refusal.Fix())
			require.ErrorIs(t, err, cause)
			if test.err == native {
				assert.Same(t, native, refusal, "the native refusal already owns its recovery")
				assert.Equal(t, native.Error(), refusal.Error())
			}
			if errors.Is(test.err, closeFailure) {
				assert.ErrorIs(t, err, closeFailure)
				assert.Contains(t, refusal.Error(), closeFailure.Error())
			}
		})
	}
}

// TestPendingReviewFailureProtectsCachedCorruptIntent changes a real pending
// file after the authenticated session opened. Review must offer recovery
// without reflecting private parser fragments or changing either source.
func TestPendingReviewFailureProtectsCachedCorruptIntent(t *testing.T) {
	mgr := newTestEditorManager(t)
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "192.0.2.10"))
	_, err := mgr.GetOrCreate("alice")
	require.NoError(t, err)
	path := cli.ChangePath(mgr.configPath, "alice")
	valid, err := mgr.store.ReadFile(path)
	require.NoError(t, err)
	committed, err := mgr.committedConfig()
	require.NoError(t, err)
	corrupt := []byte("set unknown-path private-review-corruption-sentinel\n")
	require.NoError(t, mgr.store.WriteFile(path, corrupt, 0o600))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleConfigCommitWithAuthorizerAndAudit(mgr, renderer, nil, nil, nil)
	request := func() *http.Request {
		return httptest.NewRequestWithContext(withUsername(t.Context(), "alice"), http.MethodGet, "/config/commit/", http.NoBody)
	}
	refused := httptest.NewRecorder()
	handler.ServeHTTP(refused, request())
	assert.Equal(t, http.StatusInternalServerError, refused.Code)
	assert.Contains(t, refused.Body.String(), "/config/recover")
	assert.NotContains(t, refused.Body.String(), "private-review-corruption-sentinel")
	retained, err := mgr.store.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, corrupt, retained)
	stillCommitted, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, committed, stillCommitted)
	require.NoError(t, mgr.store.WriteFile(path, valid, 0o600))
	recovered := httptest.NewRecorder()
	handler.ServeHTTP(recovered, request())
	assert.Equal(t, http.StatusOK, recovered.Code)
	assert.Contains(t, recovered.Body.String(), "192.0.2.10")
}
