// Design: docs/architecture/web-interface.md -- native backup refusals and cleanup evidence.
package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

// TestBackupCommandFailureKeepsNativeAndCleanupEvidence preserves a complete
// native wire refusal exactly while keeping a separately joined cleanup error.
func TestBackupCommandFailureKeepsNativeAndCleanupEvidence(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	native := opererr.New("The backup source is unavailable.",
		"The workspace and live configuration are unchanged.", "Open /backups/ and upload the source again")
	response := plugin.OperatorErrorResponse(native)
	response.Data = plugin.Map{"http-status": http.StatusGone}
	_, failure := decodeBackupAnswer(response)
	require.Error(t, failure)
	cleanup := errors.New("the backup transfer could not be removed")
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "native wire refusal", err: failure},
		{name: "single joined refusal", err: errors.Join(failure)},
		{name: "refusal and cleanup", err: errors.Join(failure, cleanup)},
	} {
		t.Run(test.name, func(t *testing.T) {
			out := httptest.NewRecorder()
			backupCommandFailure(out, test.err)
			if errors.Is(test.err, cleanup) {
				assert.Equal(t, http.StatusInternalServerError, out.Code)
				assert.Contains(t, out.Body.String(), native.Error())
				assert.Contains(t, out.Body.String(), cleanup.Error())
				return
			}
			assert.Equal(t, http.StatusGone, out.Code)
			assert.Equal(t, native.Error()+"\n", out.Body.String(), "the native correction must remain last and unchanged")
		})
	}
}

// TestBackupCommandFailureExplainsAuthorization keeps the canonical denial
// while adding the recovery that a plain dispatcher sentinel does not carry.
func TestBackupCommandFailureExplainsAuthorization(t *testing.T) {
	owner, _ := backupHTTPFixture(t)
	owner.dispatch = func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		return nil, pluginserver.ErrUnauthorized
	}
	request := httptest.NewRequestWithContext(withUsername(t.Context(), "alice"), http.MethodGet, "/backups/", http.NoBody)
	_, err := owner.response(t.Context(), request, "show config artifact list")
	require.Error(t, err)
	out := httptest.NewRecorder()
	backupCommandFailure(out, err)
	assert.Equal(t, http.StatusForbidden, out.Code)
	assert.Contains(t, out.Body.String(), plugin.UnauthorizedMessage)
	assert.Contains(t, out.Body.String(), "fix: ")
	assert.Contains(t, out.Body.String(), "/login")
}

// TestBackupCommandFailureExplainsIncompleteWireText refuses malformed error
// text without turning it into a panic or dropping the original diagnosis.
func TestBackupCommandFailureExplainsIncompleteWireText(t *testing.T) {
	for _, text := range []string{"", "the backup handler returned an incomplete diagnosis"} {
		t.Run(text, func(t *testing.T) {
			_, err := decodeBackupAnswer(&plugin.Response{Status: plugin.StatusError, Error: text})
			require.Error(t, err)
			out := httptest.NewRecorder()
			backupCommandFailure(out, err)
			assert.Equal(t, http.StatusBadRequest, out.Code)
			assert.Contains(t, out.Body.String(), "fix: ")
			if text != "" {
				assert.Contains(t, out.Body.String(), text)
			}
		})
	}
}
