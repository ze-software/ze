// Design: docs/architecture/web-interface.md -- upload refusals retain cleanup failures.
package web

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

// TestBackupHTTPSourceUploadRetainsCleanupFailures follows the registered
// source route into real transfer admission and abort. A small inner HTTP body
// limit produces the real MaxBytesError without allocating the full allowance.
func TestBackupHTTPSourceUploadRetainsCleanupFailures(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	cleanup := opererr.New("The private upload could not be removed.",
		"Its retained transfer still needs cleanup.", "Run ze show doctor, then reopen /backups/")
	for _, test := range []struct {
		name      string
		content   string
		file      bool
		limit     int64
		failAbort bool
		status    int
		diagnosis string
	}{
		{name: "malformed base64 with successful abort", content: "not base64!", status: http.StatusBadRequest, diagnosis: "illegal base64 data"},
		{name: "malformed base64 with failed abort", content: "not base64!", failAbort: true, status: http.StatusInternalServerError, diagnosis: "illegal base64 data"},
		{name: "body limit with successful abort", content: strings.Repeat("x", 2<<10), file: true, limit: 1024, status: http.StatusRequestEntityTooLarge, diagnosis: "upload allowance"},
		{name: "body limit with failed abort", content: strings.Repeat("x", 2<<10), file: true, limit: 1024, failAbort: true, status: http.StatusInternalServerError, diagnosis: "http: request body too large"},
		{name: "healthy binary source", content: "\x00\xff\x80", file: true, status: http.StatusSeeOther},
	} {
		t.Run(test.name, func(t *testing.T) {
			native := backupCommandDispatcher(t)
			var transfer string
			abortAttempts := 0
			dispatch := func(ctx context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
				if strings.HasPrefix(line, "request config artifact upload abort transfer ") {
					abortAttempts++
					if test.failAbort {
						response := plugin.OperatorErrorResponse(cleanup)
						response.Data = plugin.Map{"http-status": http.StatusInternalServerError}
						return response, nil
					}
				}
				response, err := native(ctx, caller, line)
				if strings.HasPrefix(line, "request config artifact bytes begin ") {
					if response != nil {
						if data, ok := response.Data.(plugin.Map); ok {
							transfer, _ = data["transfer"].(string)
						}
					}
				}
				return response, err
			}
			route, ok := lookupWebRoute(RegisteredWebRoutes(), backupWorkspacePath)
			require.True(t, ok)
			owner, ok := route.Build(RouteDeps{Dispatch: dispatch, Authorizer: fakeAuthorizer{allowEdit: true}}).(*backupWorkspaces)
			require.True(t, ok)
			t.Cleanup(func() { require.NoError(t, owner.Close()) })
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			var field io.Writer
			var err error
			if test.file {
				field, err = form.CreateFormFile("source", "source.bin")
			} else {
				field, err = form.CreateFormField("content")
			}
			require.NoError(t, err)
			_, err = io.WriteString(field, test.content)
			require.NoError(t, err)
			require.NoError(t, form.Close())
			request := httptest.NewRequestWithContext(withUsername(t.Context(), "alice"), http.MethodPost,
				"https://example.com/backups/sources/", &body)
			request.Header.Set("Content-Type", form.FormDataContentType())
			request.Header.Set("Origin", "https://example.com")
			out := httptest.NewRecorder()
			if test.limit > 0 {
				request.Body = http.MaxBytesReader(out, request.Body, test.limit)
			}
			owner.ServeHTTP(out, request)
			require.NotEmpty(t, transfer, "the input failure must occur after real native admission")
			assert.Equal(t, test.status, out.Code, out.Body.String())
			if test.status == http.StatusSeeOther {
				assert.Zero(t, abortAttempts)
				path := out.Header().Get("Location")
				require.True(t, strings.HasPrefix(path, "/backups/sources/"))
				download := backupHTTPRequest(t, owner, path+"download", "alice", nil)
				require.Equal(t, http.StatusOK, download.Code, download.Body.String())
				assert.Equal(t, []byte(test.content), download.Body.Bytes())
				return
			}
			assert.Equal(t, 1, abortAttempts)
			assert.Empty(t, out.Header().Get("Location"), "a refused source cannot become a successful redirect")
			assert.Contains(t, out.Body.String(), test.diagnosis)
			if test.failAbort {
				assert.Contains(t, out.Body.String(), cleanup.Error(), "the input category cannot hide independent cleanup evidence")
				assert.NotContains(t, out.Body.String(), "No complete source was retained", "failed cleanup cannot promise source removal")
			}
			// Consult the real owner after the injected abort. A failed abort
			// leaves an actual transfer to remove; a successful one is gone.
			answer, err := native(t.Context(), plugin.CallerIdentity{Username: "alice"},
				"request config artifact upload abort transfer "+backupWord(transfer))
			require.NoError(t, err)
			require.NotNil(t, answer)
			if test.failAbort {
				assert.Equal(t, plugin.StatusDone, answer.Status, answer.Error)
				return
			}
			assert.Equal(t, plugin.StatusError, answer.Status)
			data, ok := answer.Data.(plugin.Map)
			require.True(t, ok)
			assert.Equal(t, http.StatusNotFound, data["http-status"])
		})
	}
}
