// Design: docs/architecture/cli/error-surface.md -- browser operator refusals.

package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestEditorSessionRefusalPreservesOwnerRecovery proves a wrapped owner refusal
// keeps its own fix, while an untyped failure retains its cause and a browser
// recovery action instead of becoming an unexplained server error.
func TestEditorSessionRefusalPreservesOwnerRecovery(t *testing.T) {
	owner := opererr.New("This draft cannot be opened.", "Its owner must release it first.", "Open /cli and run disconnect alice")
	assert.Same(t, owner, editorSessionRefusal(fmt.Errorf("editor open: %w", owner)))
	cause := errors.New("candidate snapshot is unavailable")
	answer := editorSessionRefusal(cause)
	assert.ErrorIs(t, answer, cause)
	assert.Contains(t, answer.Error(), cause.Error())
	assert.Equal(t, "Reload /show/ to reopen your configuration", answer.Fix())
}

// TestWebCLIStreamPreservesOwnerRecovery checks both response phases, including
// a wrapped typed refusal after real output has already started.
func TestWebCLIStreamPreservesOwnerRecovery(t *testing.T) {
	owner := opererr.New("This monitor is blocked.", "The account cannot read these events.", "Sign in at /login with an account permitted to run this command")
	for _, started := range []bool{false, true} {
		t.Run(fmt.Sprint(started), func(t *testing.T) {
			handler := HandleCLITerminalStream(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("a handled stream reached the finite terminal")
			}), func(_ context.Context, _ plugin.CallerIdentity, _ string, output io.Writer) (bool, error) {
				if started {
					if _, err := io.WriteString(output, "first\n"); err != nil {
						return true, err
					}
				}
				return true, fmt.Errorf("dispatcher: %w", owner)
			})
			rec := httptest.NewRecorder()
			handler(rec, authedRequest(t, http.MethodPost, "/cli/stream", url.Values{"command": {"monitor event"}, "mode": {terminalModeOperational}}))
			if started {
				require.Equal(t, http.StatusOK, rec.Code)
				assert.Contains(t, rec.Body.String(), "first\n")
			} else {
				require.Equal(t, http.StatusBadRequest, rec.Code)
			}
			assert.Contains(t, rec.Body.String(), owner.Error())
			assert.Equal(t, 1, strings.Count(rec.Body.String(), "fix: "))
			assert.NotContains(t, rec.Body.String(), "run the live command again")
		})
	}
}
