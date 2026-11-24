// Design: docs/architecture/web-components.md -- actionable form refusals.

package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestGraphicalActionRequestErrorsExplainRecovery refuses before opening an
// editor and gives the browser operator the action that resumes the form.
func TestGraphicalActionRequestErrorsExplainRecovery(t *testing.T) {
	const secret = "private-form-sentinel"
	for _, tc := range []struct {
		name, method, body, fix string
		authenticated           bool
		status                  int
	}{
		{name: "sign in", method: http.MethodPost, authenticated: false, status: http.StatusUnauthorized,
			fix: "Open /login and sign in, then reopen Configuration actions"},
		{name: "button method", method: http.MethodGet, authenticated: true, status: http.StatusMethodNotAllowed,
			fix: "Open Configuration actions and press the action button"},
		{name: "unreadable form", method: http.MethodPost, body: "target=" + secret + "&action=%zz", authenticated: true, status: http.StatusBadRequest,
			fix: "Reload the YANG editor, reopen Configuration actions and submit the form again"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tc.method, "/config/action", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if tc.authenticated {
				req = req.WithContext(withUsername(req.Context(), "alice"))
			}
			rec := httptest.NewRecorder()
			HandleGraphicalAction(nil, nil, nil, nil, nil, nil)(rec, req)
			assert.Equal(t, tc.status, rec.Code)
			lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
			require.Len(t, lines, 3, rec.Body.String())
			assert.NotEmpty(t, lines[0])
			assert.NotEmpty(t, lines[1])
			assert.Equal(t, "fix: "+tc.fix, lines[2])
			assert.NotContains(t, rec.Body.String(), secret)
		})
	}
}

// TestGraphicalCollectionKeepsProducerCorrection preserves the shared schema's
// typed correction instead of replacing it with a generic browser instruction.
func TestGraphicalCollectionKeepsProducerCorrection(t *testing.T) {
	mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
	path := []string{"bgp", "peer", "upstream", "session", "community"}
	require.NoError(t, mgr.SetValue("testuser", path, "send", "standard"))
	form := url.Values{"leaf": {"send"}, "collection": {"true"}, "operation": {"save"}, "member": {"invalid"}}
	req := authedRequest(t, http.MethodPost, "/config/set/"+strings.Join(path, "/"), form)
	require.NoError(t, req.ParseForm())
	_, producerErr := graphicalValuesFromForm(req, path, "send", schema.LookupTokenPath(append(path, "send")))
	require.Error(t, producerErr)
	refusal, typed := errors.AsType[*opererr.Error](producerErr)
	require.True(t, typed)
	rec := httptest.NewRecorder()
	HandleConfigSetWithAuthorizer(mgr, schema, nil, nil)(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, refusal.Error()+"\n", rec.Body.String())
	assert.Equal(t, []string{"standard"}, walkTree(mgr.snapshotTree("testuser"), schema, path).GetSlice("send"))
}
