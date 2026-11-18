// Design: docs/architecture/web-interface.md -- native checks and salvage of browser-held bytes.
package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
)

func backupSourceUpload(t *testing.T, owner *backupWorkspaces, content []byte, user, origin string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("source", "damaged.zefs")
	require.NoError(t, err)
	_, err = file.Write(content)
	require.NoError(t, err)
	require.NoError(t, form.Close())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.com/backups/sources/", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Origin", origin)
	request = request.WithContext(withUsername(request.Context(), user))
	response := httptest.NewRecorder()
	owner.ServeHTTP(response, request)
	return response
}

func backupSourceHref(t *testing.T, body, id string) string {
	t.Helper()
	tokens := html.NewTokenizer(strings.NewReader(body))
	// The native rendered page bounds the complete token walk.
	for tokens.Next() != html.ErrorToken {
		token := tokens.Token()
		attributes := make(map[string]string, len(token.Attr))
		for _, attribute := range token.Attr {
			attributes[attribute.Key] = attribute.Val
		}
		if attributes["id"] == id {
			return attributes["href"]
		}
	}
	t.Fatalf("source page lacks link %q", id)
	return ""
}

// TestBackupHTTPRawSourceRepair follows the production HTTP adapter through a
// corrupt input, native failed check, useful salvage warning and valid download.
// Reading or repairing the source never consumes or changes its complete bytes.
func TestBackupHTTPRawSourceRepair(t *testing.T) {
	owner, original := backupHTTPFixture(t)
	corrupt := bytes.Clone(original)
	offset := bytes.Index(corrupt, []byte("credential-to-preserve"))
	require.GreaterOrEqual(t, offset, 0)
	corrupt[offset] ^= 0xff
	before := bytes.Clone(corrupt)
	upload := backupSourceUpload(t, owner, corrupt, "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, upload.Code, upload.Body.String())
	path := upload.Header().Get("Location")
	require.True(t, strings.HasPrefix(path, "/backups/sources/"), path)
	opened := backupHTTPRequest(t, owner, path, "alice", nil)
	require.Equal(t, http.StatusOK, opened.Code, opened.Body.String())
	require.Contains(t, opened.Body.String(), "backup-source-check")
	checked := backupHTTPRequest(t, owner, path, "alice", url.Values{"action": {"check"}})
	require.Equal(t, http.StatusBadRequest, checked.Code, checked.Body.String())
	require.Contains(t, strings.ToLower(checked.Body.String()), "crc")
	require.Contains(t, checked.Body.String(), "backup-source-report", "failed native checks retain their finding document")
	download := backupHTTPRequest(t, owner, path+"download", "alice", nil)
	require.Equal(t, http.StatusOK, download.Code, download.Body.String())
	require.Equal(t, before, download.Body.Bytes(), "a failed check and source download preserve the complete input")
	repaired := backupHTTPRequest(t, owner, path, "alice", url.Values{"action": {"repair"}})
	require.Equal(t, http.StatusOK, repaired.Code, repaired.Body.String())
	require.Contains(t, strings.ToLower(repaired.Body.String()), "warning")
	require.Contains(t, repaired.Body.String(), "skipped", "partial salvage keeps the native recovery counts")
	salvagePath := backupSourceHref(t, repaired.Body.String(), "backup-source-salvage")
	salvage := backupHTTPRequest(t, owner, salvagePath, "alice", nil)
	require.Equal(t, http.StatusOK, salvage.Code, salvage.Body.String())
	file := filepath.Join(t.TempDir(), "salvage.zefs")
	require.NoError(t, os.WriteFile(file, salvage.Body.Bytes(), 0o600))
	require.NoError(t, storage.CheckArtifact(file), "the browser downloads a complete native artifact")
	store, err := storage.OpenBlob(file, false)
	require.NoError(t, err)
	content, err := storage.ReadActiveConfig(store, "router.conf")
	require.NoError(t, err)
	require.Equal(t, validWebConfig, string(content))
	binary, err := store.ReadKey("unknown/binary")
	require.NoError(t, err)
	require.Equal(t, []byte{0, 255, 128, 0, 1}, binary)
	require.NoError(t, store.Close())
	kept := backupHTTPRequest(t, owner, path+"download", "alice", nil)
	require.Equal(t, http.StatusOK, kept.Code, kept.Body.String())
	require.Equal(t, before, kept.Body.Bytes())
	require.Equal(t, before, corrupt, "the browser adapter cannot edit its caller's original buffer")
	closed := backupHTTPRequest(t, owner, path, "alice", url.Values{"action": {"close"}})
	require.Equal(t, http.StatusSeeOther, closed.Code, closed.Body.String())
	missing := backupHTTPRequest(t, owner, path, "alice", nil)
	require.Equal(t, http.StatusNotFound, missing.Code)
}

// TestBackupHTTPRawSourceAccess keeps binary sources under the same private
// account and origin controls as editable artifacts, including after upload.
func TestBackupHTTPRawSourceAccess(t *testing.T) {
	for _, test := range []struct {
		name, user, origin string
		readOnly           bool
		status             int
	}{
		{name: "anonymous", origin: "https://example.com", status: http.StatusUnauthorized},
		{name: "read only", user: "alice", origin: "https://example.com", readOnly: true, status: http.StatusForbidden},
		{name: "wrong origin", user: "alice", origin: "https://other.example", status: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			owner, _ := backupHTTPFixture(t)
			if test.readOnly {
				owner.authorizer = fakeAuthorizer{allowEdit: false}
			}
			response := backupSourceUpload(t, owner, []byte{0, 255, 128}, test.user, test.origin)
			require.Equal(t, test.status, response.Code, response.Body.String())
		})
	}
	owner, _ := backupHTTPFixture(t)
	upload := backupSourceUpload(t, owner, []byte{0, 255, 128}, "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, upload.Code, upload.Body.String())
	path := upload.Header().Get("Location")
	for _, suffix := range []string{"", "download"} {
		response := backupHTTPRequest(t, owner, path+suffix, "bob", nil)
		require.Equal(t, http.StatusNotFound, response.Code, response.Body.String())
	}
}

// TestBackupHTTPRawSourceCanceledUpload proves an interrupted adapter aborts
// its native transfer, so no retained bytes remain available under that token.
func TestBackupHTTPRawSourceCanceledUpload(t *testing.T) {
	dispatch := backupCommandDispatcher(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	var transfer string
	owner := HandleBackupWorkspace(RouteDeps{Authorizer: fakeAuthorizer{allowEdit: true}, Dispatch: func(request context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
		response, err := dispatch(request, caller, line)
		if strings.HasPrefix(line, "request config artifact bytes begin ") {
			if response != nil {
				if data, ok := response.Data.(plugin.Map); ok {
					transfer, _ = data["transfer"].(string)
				}
			}
			cancel()
		}
		return response, err
	}})
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("source", "source.bin")
	require.NoError(t, err)
	_, err = io.WriteString(file, "private interrupted bytes")
	require.NoError(t, err)
	require.NoError(t, form.Close())
	request := httptest.NewRequestWithContext(ctx, http.MethodPost, "https://example.com/backups/sources/", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Origin", "https://example.com")
	request = request.WithContext(withUsername(request.Context(), "alice"))
	response := httptest.NewRecorder()
	owner.ServeHTTP(response, request)
	require.NotEqual(t, http.StatusSeeOther, response.Code)
	require.NotEmpty(t, transfer, "the interrupted request must have reached its native source admission")
	refused, err := dispatch(t.Context(), plugin.CallerIdentity{Username: "alice"}, "show config artifact transfer read transfer "+transfer+" offset 0")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, refused.Status)
}

// TestBackupHTTPRawSourceMultipartRefusal proves an extra part cannot publish
// the first uploaded source, even when that source would otherwise be valid.
func TestBackupHTTPRawSourceMultipartRefusal(t *testing.T) {
	dispatch := backupCommandDispatcher(t)
	var transfer string
	owner := HandleBackupWorkspace(RouteDeps{Authorizer: fakeAuthorizer{allowEdit: true}, Dispatch: func(ctx context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
		response, err := dispatch(ctx, caller, line)
		if strings.HasPrefix(line, "request config artifact bytes begin ") {
			if response != nil {
				if data, ok := response.Data.(plugin.Map); ok {
					transfer, _ = data["transfer"].(string)
				}
			}
		}
		return response, err
	}})
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("source", "one.bin")
	require.NoError(t, err)
	_, err = file.Write([]byte{0, 255, 128})
	require.NoError(t, err)
	require.NoError(t, form.WriteField("unexpected", "second part"))
	require.NoError(t, form.Close())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.com/backups/sources/", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Origin", "https://example.com")
	request = request.WithContext(withUsername(request.Context(), "alice"))
	response := httptest.NewRecorder()
	owner.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	require.NotEmpty(t, transfer, "the first source must have reached native admission")
	refused, err := dispatch(t.Context(), plugin.CallerIdentity{Username: "alice"}, "show config artifact transfer read transfer "+transfer+" offset 0")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, refused.Status)
}

// TestBackupHTTPRawSourcePaste preserves binary and explicit empty input while
// refusing malformed base64 without retaining a partially decoded transfer.
func TestBackupHTTPRawSourcePaste(t *testing.T) {
	for _, content := range [][]byte{nil, {0, 255, 128, '\n', 1}} {
		owner, _ := backupHTTPFixture(t)
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		require.NoError(t, form.WriteField("content", base64.StdEncoding.EncodeToString(content)))
		require.NoError(t, form.Close())
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.com/backups/sources/", &body)
		request.Header.Set("Content-Type", form.FormDataContentType())
		request.Header.Set("Origin", "https://example.com")
		request = request.WithContext(withUsername(request.Context(), "alice"))
		response := httptest.NewRecorder()
		owner.ServeHTTP(response, request)
		require.Equal(t, http.StatusSeeOther, response.Code, response.Body.String())
		download := backupHTTPRequest(t, owner, response.Header().Get("Location")+"download", "alice", nil)
		require.Equal(t, http.StatusOK, download.Code, download.Body.String())
		require.Equal(t, content, download.Body.Bytes())
	}
	dispatch := backupCommandDispatcher(t)
	var transfer string
	owner := HandleBackupWorkspace(RouteDeps{Authorizer: fakeAuthorizer{allowEdit: true}, Dispatch: func(ctx context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
		response, err := dispatch(ctx, caller, line)
		if strings.HasPrefix(line, "request config artifact bytes begin ") {
			if response != nil {
				if data, ok := response.Data.(plugin.Map); ok {
					transfer, _ = data["transfer"].(string)
				}
			}
		}
		return response, err
	}})
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	require.NoError(t, form.WriteField("content", "YWJj*invalid"))
	require.NoError(t, form.Close())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.com/backups/sources/", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Origin", "https://example.com")
	request = request.WithContext(withUsername(request.Context(), "alice"))
	response := httptest.NewRecorder()
	owner.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	require.Contains(t, strings.ToLower(response.Body.String()), "base64")
	require.NotEmpty(t, transfer)
	refused, err := dispatch(t.Context(), plugin.CallerIdentity{Username: "alice"}, "show config artifact transfer read transfer "+transfer+" offset 0")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, refused.Status)
}

// TestBackupHTTPRawSourceRepairFailureKeepsSalvage distinguishes an unresolved
// native history failure from both a successful repair and a lost result.
func TestBackupHTTPRawSourceRepairFailureKeepsSalvage(t *testing.T) {
	owner, original := backupHTTPFixture(t)
	input := filepath.Join(t.TempDir(), "unresolved-history.zefs")
	require.NoError(t, os.WriteFile(input, original, 0o600))
	store, err := storage.OpenBlob(input, true)
	require.NoError(t, err)
	const pointer = "meta/config/missing.conf/active"
	const missingVersion = "20261117T120000.000000000Z"
	require.NoError(t, store.WriteKey(pointer, []byte(missingVersion)))
	require.NoError(t, store.Close())
	content, err := os.ReadFile(input)
	require.NoError(t, err)
	upload := backupSourceUpload(t, owner, content, "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, upload.Code, upload.Body.String())
	path := upload.Header().Get("Location")
	repaired := backupHTTPRequest(t, owner, path, "alice", url.Values{"action": {"repair"}})
	require.Equal(t, http.StatusBadRequest, repaired.Code, repaired.Body.String())
	require.Contains(t, repaired.Body.String(), "backup-source-error")
	require.NotContains(t, repaired.Body.String(), `id="backup-source-status"`, "an unresolved repair must not report successful completion")
	require.Contains(t, repaired.Body.String(), "dangling-pointer")
	require.Contains(t, repaired.Body.String(), pointer)
	salvagePath := backupSourceHref(t, repaired.Body.String(), "backup-source-salvage")
	salvage := backupHTTPRequest(t, owner, salvagePath, "alice", nil)
	require.Equal(t, http.StatusOK, salvage.Code, salvage.Body.String())
	output := filepath.Join(t.TempDir(), "salvage.zefs")
	require.NoError(t, os.WriteFile(output, salvage.Body.Bytes(), 0o600))
	require.NoError(t, storage.CheckArtifact(output), "unresolved history still leaves a complete downloadable artifact")
	recovered, err := storage.OpenBlob(output, false)
	require.NoError(t, err)
	keptPointer, err := recovered.ReadKey(pointer)
	require.NoError(t, err)
	require.Equal(t, missingVersion, string(keptPointer), "the native failure must not silently select a replacement history version")
	require.NoError(t, recovered.Close())
	kept := backupHTTPRequest(t, owner, path+"download", "alice", nil)
	require.Equal(t, http.StatusOK, kept.Code, kept.Body.String())
	require.Equal(t, content, kept.Body.Bytes(), "an unsuccessful repair retains the unchanged original source")
}
