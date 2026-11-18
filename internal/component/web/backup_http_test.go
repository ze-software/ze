// Design: docs/architecture/web-interface.md -- isolated artifact HTTP boundaries.
package web

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/pkg/zefs"
)

func backupHTTPFixture(t *testing.T) (*backupWorkspaces, []byte) {
	t.Helper()
	owner, ok := HandleBackupWorkspace(RouteDeps{Dispatch: backupCommandDispatcher(t), Authorizer: fakeAuthorizer{allowEdit: true}}).(*backupWorkspaces)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	path := filepath.Join(t.TempDir(), "original.zefs")
	artifact, err := storage.CreateBlobPopulated(path, func(seed storage.Storage) error {
		if err := seed.WriteFile("router.conf", []byte(validWebConfig), 0o600); err != nil {
			return err
		}
		if err := seed.WriteFile("other.conf", []byte(validWebConfig), 0o600); err != nil {
			return err
		}
		if err := seed.WriteKey("file/draft/router.conf", []byte("preserved original draft")); err != nil {
			return err
		}
		if err := seed.WriteFile(cli.DraftPath("router.conf"), []byte("set bgp router-id 192.0.2.99\n"), 0o600); err != nil {
			return err
		}
		if err := seed.WriteFile(cli.ChangePath("router.conf", "alice"), []byte("set bgp router-id 198.51.100.80\n"), 0o600); err != nil {
			return err
		}
		if err := seed.WriteKey(zefs.KeyLocalAdminPassword.Pattern, []byte("credential-to-preserve")); err != nil {
			return err
		}
		return seed.WriteKey("unknown/binary", []byte{0, 255, 128, 0, 1})
	}, false, zefs.Spare(0))
	require.NoError(t, err)
	require.NoError(t, artifact.Close())
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return owner, data
}

func backupHTTPUpload(t *testing.T, owner *backupWorkspaces, content []byte, user, origin string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("backup", "original.zefs")
	require.NoError(t, err)
	_, err = file.Write(content)
	require.NoError(t, err)
	require.NoError(t, form.Close())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.com/backups/", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Origin", origin)
	request = request.WithContext(withUsername(request.Context(), user))
	response := httptest.NewRecorder()
	owner.ServeHTTP(response, request)
	return response
}

func backupHTTPRequest(t *testing.T, owner *backupWorkspaces, path, user string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	method := http.MethodGet
	var body io.Reader = http.NoBody
	if form != nil {
		method = http.MethodPost
		body = strings.NewReader(form.Encode())
	}
	request := httptest.NewRequestWithContext(t.Context(), method, "https://example.com"+path, body)
	request.Header.Set("Origin", "https://example.com")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = request.WithContext(withUsername(request.Context(), user))
	response := httptest.NewRecorder()
	owner.ServeHTTP(response, request)
	return response
}

func backupHTTPField(t *testing.T, content, name string) string {
	t.Helper()
	tokens := html.NewTokenizer(strings.NewReader(content))
	// The rendered page bounds this tokenizer walk by its byte length.
	for tokens.Next() != html.ErrorToken {
		token := tokens.Token()
		if token.Data != "input" {
			continue
		}
		attributes := make(map[string]string)
		for _, attribute := range token.Attr {
			attributes[attribute.Key] = attribute.Val
		}
		if attributes["name"] == name {
			return attributes["value"]
		}
	}
	t.Fatalf("backup form lacks field %q", name)
	return ""
}

// TestBackupHTTPArtifactRoundTrip edits through native forms, downloads binary
// bytes and verifies semantic content and unrelated draft/binary preservation.
func TestBackupHTTPArtifactRoundTrip(t *testing.T) {
	owner, source := backupHTTPFixture(t)
	upload := backupHTTPUpload(t, owner, source, "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, upload.Code, upload.Body.String())
	path := upload.Header().Get("Location")
	opened := backupHTTPRequest(t, owner, path+"?config=router.conf", "alice", nil)
	require.Equal(t, http.StatusOK, opened.Code)
	edited := strings.Replace(validWebConfig, "1.2.3.4", "203.0.113.19", 1)
	form := url.Values{"config": {"router.conf"}, "base": {backupHTTPField(t, opened.Body.String(), "base")},
		"content": {edited}, "action": {"review"}, "review": {""}}
	reviewed := backupHTTPRequest(t, owner, path, "alice", form)
	require.Equal(t, http.StatusOK, reviewed.Code, reviewed.Body.String())
	assert.Contains(t, reviewed.Body.String(), "203.0.113.19")
	form.Set("review", backupHTTPField(t, reviewed.Body.String(), "review"))
	form.Set("action", "save")
	saved := backupHTTPRequest(t, owner, path, "alice", form)
	require.Equal(t, http.StatusOK, saved.Code, saved.Body.String())
	assert.Contains(t, saved.Body.String(), "saved into the backup")
	download := backupHTTPRequest(t, owner, path+"download", "alice", nil)
	require.Equal(t, http.StatusOK, download.Code)
	assert.Equal(t, "application/octet-stream", download.Header().Get("Content-Type"))
	output := filepath.Join(t.TempDir(), "download.zefs")
	require.NoError(t, os.WriteFile(output, download.Body.Bytes(), 0o600))
	restored, err := storage.ReadRestoreSource(output, "router.conf", "")
	require.NoError(t, err)
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	got, err := config.NewParser(schema).Parse(string(restored.Data))
	require.NoError(t, err)
	want, err := config.NewParser(schema).Parse(edited)
	require.NoError(t, err)
	assert.Equal(t, want.ToMap(), got.ToMap())
	artifact, err := storage.OpenBlob(output, false)
	require.NoError(t, err)
	draft, err := artifact.ReadKey("file/draft/router.conf")
	require.NoError(t, err)
	assert.Equal(t, []byte("preserved original draft"), draft)
	savedDraft, err := artifact.ReadFile(cli.DraftPath("router.conf"))
	require.NoError(t, err)
	assert.Equal(t, []byte("set bgp router-id 192.0.2.99\n"), savedDraft)
	changes, err := artifact.ReadFile(cli.ChangePath("router.conf", "alice"))
	require.NoError(t, err)
	assert.Equal(t, []byte("set bgp router-id 198.51.100.80\n"), changes)
	credential, err := artifact.ReadKey(zefs.KeyLocalAdminPassword.Pattern)
	require.NoError(t, err)
	assert.Equal(t, []byte("credential-to-preserve"), credential)
	binary, err := artifact.ReadKey("unknown/binary")
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 255, 128, 0, 1}, binary)
	other, err := storage.ReadActiveConfig(artifact, "other.conf")
	require.NoError(t, err)
	assert.Equal(t, []byte(validWebConfig), other)
	require.NoError(t, artifact.Close())
}

// TestBackupHTTPSaveKeepsValidationWarnings preserves the shared plaintext
// credential warning after the serializer also hashes it successfully.
func TestBackupHTTPSaveKeepsValidationWarnings(t *testing.T) {
	owner, source := backupHTTPFixture(t)
	upload := backupHTTPUpload(t, owner, source, "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, upload.Code)
	path := upload.Header().Get("Location")
	opened := backupHTTPRequest(t, owner, path+"?config=router.conf", "alice", nil)
	edited := validWebConfig + "\nsystem { authentication { user alice { password backup-warning-plaintext; plaintext-password short; } } }\n"
	form := url.Values{"config": {"router.conf"}, "base": {backupHTTPField(t, opened.Body.String(), "base")},
		"content": {edited}, "action": {"review"}, "review": {""}}
	reviewed := backupHTTPRequest(t, owner, path, "alice", form)
	require.Equal(t, http.StatusOK, reviewed.Code, reviewed.Body.String())
	require.Contains(t, reviewed.Body.String(), "bcrypt", "fixture must actually trigger the shared validator's warning")
	form.Set("review", backupHTTPField(t, reviewed.Body.String(), "review"))
	form.Set("action", "save")
	saved := backupHTTPRequest(t, owner, path, "alice", form)
	require.Equal(t, http.StatusOK, saved.Code, saved.Body.String())
	assert.Contains(t, saved.Body.String(), "saved into the backup")
	assert.Contains(t, saved.Body.String(), "bcrypt", "accepted publication must retain the validator's advisory")
	assert.Contains(t, saved.Body.String(), "weak password", "the serializer's separate advisory must also survive")
}

// TestBackupHTTPRefusals checks credential-bearing access before upload
// retention and proves corrupt or cross-origin input cannot create a workspace.
func TestBackupHTTPRefusals(t *testing.T) {
	for _, test := range []struct {
		name     string
		user     string
		origin   string
		corrupt  bool
		readOnly bool
		status   int
	}{
		{"anonymous", "", "https://example.com", false, false, http.StatusUnauthorized},
		{"read-only", "alice", "https://example.com", false, true, http.StatusForbidden},
		{"cross-origin", "alice", "https://other.example", false, false, http.StatusForbidden},
		{"corrupt", "alice", "https://example.com", true, false, http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			owner, source := backupHTTPFixture(t)
			if test.readOnly {
				owner.authorizer = fakeAuthorizer{allowEdit: false}
			}
			if test.corrupt {
				source = []byte("this is not a Ze backup")
			}
			response := backupHTTPUpload(t, owner, source, test.user, test.origin)
			assert.Equal(t, test.status, response.Code, response.Body.String())
			list := backupCommand(t, owner.dispatch, "alice", "show config artifact list")
			assert.Equal(t, "[]", backupJSON(t, list.Data))
		})
	}
}

// TestBackupHTTPWrongOwnerAndStaleReview proves URL ownership and review-bound
// content, then retains a usable workspace after both refusals.
func TestBackupHTTPWrongOwnerAndStaleReview(t *testing.T) {
	owner, source := backupHTTPFixture(t)
	upload := backupHTTPUpload(t, owner, source, "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, upload.Code)
	path := upload.Header().Get("Location")
	assert.Equal(t, http.StatusNotFound, backupHTTPRequest(t, owner, path+"download", "bob", nil).Code)
	opened := backupHTTPRequest(t, owner, path+"?config=router.conf", "alice", nil)
	form := url.Values{"config": {"router.conf"}, "base": {backupHTTPField(t, opened.Body.String(), "base")},
		"content": {validWebConfig}, "action": {"review"}, "review": {""}}
	reviewed := backupHTTPRequest(t, owner, path, "alice", form)
	require.Equal(t, http.StatusOK, reviewed.Code)
	form.Set("review", backupHTTPField(t, reviewed.Body.String(), "review"))
	form.Set("action", "save")
	form.Set("content", strings.Replace(validWebConfig, "1.2.3.4", "203.0.113.20", 1))
	refused := backupHTTPRequest(t, owner, path, "alice", form)
	assert.Equal(t, http.StatusConflict, refused.Code)
	assert.Contains(t, refused.Body.String(), "203.0.113.20", "unreviewed text must remain available for correction")
	assert.Equal(t, http.StatusOK, backupHTTPRequest(t, owner, path+"download", "alice", nil).Code)
}

// TestBackupWorkspaceDirectNavigation follows the registered native workspace
// target while preserving the existing embedded-service portal route.
func TestBackupWorkspaceDirectNavigation(t *testing.T) {
	route, ok := lookupWebRoute(RegisteredWebRoutes(), backupWorkspacePath)
	require.True(t, ok)
	require.NotNil(t, route.Portal)
	assert.True(t, route.Portal.Direct)
	assert.Equal(t, backupWorkspacePath, route.Portal.NavigationURL())
	assert.Equal(t, "/portal/existing", (PortalService{Key: "existing", Path: "/existing/"}).NavigationURL(),
		"existing embedded services must retain their portal URL")
}

type backupOwnedHandler struct {
	entered  chan struct{}
	canceled chan struct{}
	release  chan struct{}
	finished atomic.Bool
	closed   atomic.Int32
	closeErr error
}

func (handler *backupOwnedHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	close(handler.entered)
	<-r.Context().Done()
	close(handler.canceled)
	<-handler.release
	handler.finished.Store(true)
}

func (handler *backupOwnedHandler) Close() error {
	if !handler.finished.Load() {
		return errors.New("owned route closed before its request finished")
	}
	handler.closed.Add(1)
	return handler.closeErr
}

// TestWebServerOwnedRouteClosesAfterRequests proves the generic route-owner
// lifecycle joins active requests, retains cleanup failures, and closes once.
func TestWebServerOwnedRouteClosesAfterRequests(t *testing.T) {
	cert, key, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)
	server, err := NewWebServer(WebConfig{ListenAddrs: []string{"127.0.0.1:0"}, CertPEM: cert, KeyPEM: key})
	require.NoError(t, err)
	failure := errors.New("owned route cleanup failed")
	handler := &backupOwnedHandler{entered: make(chan struct{}), canceled: make(chan struct{}),
		release: make(chan struct{}), closeErr: failure}
	server.OwnHandler(handler)
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		server.serveRequest(handler).ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/owned", http.NoBody))
	}()
	<-handler.entered
	stopped := make(chan error, 1)
	go func() { stopped <- server.Shutdown(context.Background()) }()
	<-handler.canceled
	assert.Zero(t, handler.closed.Load(), "cleanup must wait while its admitted request is still active")
	close(handler.release)
	<-requestDone
	require.ErrorIs(t, <-stopped, failure)
	require.ErrorIs(t, server.Shutdown(context.Background()), failure)
	assert.Equal(t, int32(1), handler.closed.Load())
}
