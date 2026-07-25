package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/audit"
)

// VALIDATES: AC-3 -- authenticated download streams the committed config as a
// text attachment and records an audit entry.
// PREVENTS: config download without attribution or content-type.
func TestConfigDownloadHandler(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	handler := HandleConfigDownload(mgr, recorder)

	req := httptest.NewRequest(http.MethodGet, "/config/download", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), ctxKeyUsername, "alice"))
	req.RemoteAddr = "192.0.2.5:1111"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "ze.conf")
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/plain")
	assert.Contains(t, rec.Body.String(), "router-id 1.2.3.4")

	entries := recorder.Query(audit.Filter{Action: audit.ActionConfigDownload})
	require.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].Actor)
	assert.Equal(t, audit.Web, entries[0].Surface)
}

// VALIDATES: AC-3 -- download without an authenticated session is rejected.
// PREVENTS: anonymous config exfiltration.
func TestConfigDownloadRequiresAuth(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	handler := HandleConfigDownload(mgr, nil)

	req := httptest.NewRequest(http.MethodGet, "/config/download", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// VALIDATES: AC-4 (spec-fixit-bcrypt-hash-credential) -- GET /config/download is
// gated by RequireEditAuthz: a read-only session gets 403, an edit-authorized
// session gets the RAW (unmasked) committed config. The raw stream proves AC-6:
// the download is byte-exact so a real password hash round-trips.
// PREVENTS: a read-only web user exfiltrating the raw config (and its bcrypt
// hash) via the download endpoint.
func TestConfigDownloadRouteGatedByEditAuthz(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	// Matches the production wiring: editWrap = auth + RequireEditAuthz.
	download := HandleConfigDownload(mgr, recorder)

	newReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/config/download", http.NoBody)
		req.RemoteAddr = "192.0.2.5:1111"
		return req.WithContext(withUsername(req.Context(), "bob"))
	}

	// Read-only user: RequireEditAuthz returns 403 before the handler runs.
	roGate := RequireEditAuthz(fakeAuthorizer{allowEdit: false}, download)
	roRec := httptest.NewRecorder()
	roGate.ServeHTTP(roRec, newReq())
	assert.Equal(t, http.StatusForbidden, roRec.Code, "read-only user must be denied the download")

	// Edit-authorized user: gets the raw committed config verbatim (unmasked).
	editGate := RequireEditAuthz(fakeAuthorizer{allowEdit: true}, download)
	editRec := httptest.NewRecorder()
	editGate.ServeHTTP(editRec, newReq())
	require.Equal(t, http.StatusOK, editRec.Code, "edit-authorized user must get the download")
	raw, readErr := mgr.CommittedConfig()
	require.NoError(t, readErr)
	assert.Equal(t, string(raw), editRec.Body.String(),
		"download must be the byte-exact raw committed config (unmasked round-trip)")
}

// VALIDATES: AC-4 -- a valid uploaded config is validated, written, and the
// reload hook fires; an audit entry is recorded.
// PREVENTS: silent upload that never reaches the daemon.
func TestConfigUploadValidApplies(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	hookCalled := false
	mgr.SetCommitHook(func() error { hookCalled = true; return nil })
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)

	validate := func(_, _ string) error { return nil }
	handler := HandleConfigUpload(mgr, validate, "cfg", adminWebAuthorizer(), recorder)

	newConfig := "bgp {\n\trouter-id 9.9.9.9\n\tsession { asn { local 65000; } }\n}\n"
	form := url.Values{"config": {newConfig}}
	req := postConfigRequest(t, "/config/upload", form, "alice")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.True(t, hookCalled, "reload hook must fire on a valid upload")

	committed, readErr := mgr.CommittedConfig()
	require.NoError(t, readErr)
	assert.Contains(t, string(committed), "9.9.9.9")

	entries := recorder.Query(audit.Filter{Action: audit.ActionConfigUpload})
	require.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].Actor)
}

// VALIDATES: AC-4 -- an invalid uploaded config is rejected with 400 and nothing
// is applied (reload hook not fired, committed config unchanged).
// PREVENTS: applying a config the validator rejected.
func TestConfigUploadValidatesRejects(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	hookCalled := false
	mgr.SetCommitHook(func() error { hookCalled = true; return nil })

	validate := func(_, _ string) error { return errors.New("unknown leaf foo") }
	handler := HandleConfigUpload(mgr, validate, "cfg", adminWebAuthorizer(), nil)

	form := url.Values{"config": {"garbage {"}}
	req := postConfigRequest(t, "/config/upload", form, "alice")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "unknown leaf foo")
	assert.False(t, hookCalled, "invalid upload must not fire the reload hook")

	committed, readErr := mgr.CommittedConfig()
	require.NoError(t, readErr)
	assert.Contains(t, string(committed), "1.2.3.4", "committed config must be unchanged")
	assert.NotContains(t, string(committed), "garbage")
}

// VALIDATES: AC-4 -- when the reload hook rejects an uploaded config, the prior
// committed content is restored (editor.go ApplyCommittedContent restore path)
// and the client receives 500.
// PREVENTS: the daemon being left with an on-disk config its reload rejected.
func TestConfigUploadHookFailureRestores(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	mgr.SetCommitHook(func() error { return errors.New("reload rejected") })

	handler := HandleConfigUpload(mgr, func(_, _ string) error { return nil }, "cfg", adminWebAuthorizer(), nil)

	form := url.Values{"config": {"bgp {\n\trouter-id 9.9.9.9\n\tsession { asn { local 65000; } }\n}\n"}}
	req := postConfigRequest(t, "/config/upload", form, "alice")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "applying config")

	committed, readErr := mgr.CommittedConfig()
	require.NoError(t, readErr)
	assert.Contains(t, string(committed), "1.2.3.4", "prior config must be restored after hook failure")
	assert.NotContains(t, string(committed), "9.9.9.9", "rejected config must not remain committed")
}

// VALIDATES: AC-1/AC-4 -- a read-only session cannot upload (403), config unchanged.
// PREVENTS: RBAC bypass through the upload endpoint.
func TestConfigUploadRBACDeny(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	hookCalled := false
	mgr.SetCommitHook(func() error { hookCalled = true; return nil })

	handler := HandleConfigUpload(mgr, func(_, _ string) error { return nil }, "cfg", readOnlyWebAuthorizer(), nil)

	form := url.Values{"config": {"bgp {\n\trouter-id 9.9.9.9\n}\n"}}
	req := postConfigRequest(t, "/config/upload", form, "alice")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, hookCalled, "denied upload must not fire the reload hook")
	committed, readErr := mgr.CommittedConfig()
	require.NoError(t, readErr)
	assert.Contains(t, string(committed), "1.2.3.4")
}
