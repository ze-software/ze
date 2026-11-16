// Design: docs/architecture/web-interface.md -- bounded browser stream refusals.

package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestWebCLIStreamRefusalPreservesVisibleRecoveryBudget drives long failures
// after a real first chunk. Details yield space to the complete recovery line,
// and a fix that cannot fit gets an explicit browser recovery action.
func TestWebCLIStreamRefusalPreservesVisibleRecoveryBudget(t *testing.T) {
	for _, tc := range []struct {
		name       string
		failure    error
		wantDetail string
		wantFix    string
	}{
		{
			name:       "long-multibyte-cause",
			failure:    errors.New("producer unavailable: " + strings.Repeat("é界", maxCommandLength)),
			wantDetail: "producer unavailable:",
			wantFix:    "Open /cli and run the live command again",
		},
		{
			name:       "oversize-owner-fix",
			failure:    opererr.New("The live command failed.", "Its requested recovery cannot fit on the page.", strings.Repeat("é界", maxCommandLength)),
			wantDetail: "recovery instructions exceed the display limit",
			wantFix:    "Open /cli and submit a shorter live command",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := HandleCLITerminalStream(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("a handled stream reached the finite terminal")
			}), func(_ context.Context, _ plugin.CallerIdentity, _ string, output io.Writer) (bool, error) {
				if _, err := io.WriteString(output, "first\n"); err != nil {
					return true, err
				}
				return true, tc.failure
			})
			rec := httptest.NewRecorder()
			handler(rec, authedRequest(t, http.MethodPost, "/cli/stream", url.Values{"command": {"monitor event"}, "mode": {terminalModeOperational}}))
			require.Equal(t, http.StatusOK, rec.Code)
			body := rec.Body.String()
			assert.True(t, strings.HasPrefix(body, "first\n"))
			assert.Contains(t, body, tc.wantDetail)
			assert.True(t, utf8.ValidString(body), "the byte limit split a UTF-8 character")
			assert.LessOrEqual(t, len(body), len("first\n")+maxCommandLength+len("\nerror: \n"))
			assert.True(t, strings.HasSuffix(body, "\nfix: "+tc.wantFix+"\n"), "the complete recovery must be last")
			assert.Equal(t, 1, strings.Count(body, "fix: "))
		})
	}
}
