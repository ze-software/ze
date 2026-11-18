// Design: docs/architecture/web-workbench-pages.md -- guarded L2TP form submission

//go:build ze_l2tp

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestL2TPDisconnectRefusesAnUnguardedForm drives the real HTTP action. A
// canonical guarded form remains useful; the same action without its original
// reference must be refused before the command dispatcher can act.
// It uses only current APIs and remains byte-identical after the identity fix.
func TestL2TPDisconnectRefusesAnUnguardedForm(t *testing.T) {
	calls := 0
	handlers := &l2TPHandlers{Dispatch: func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		calls++
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"status":"sent"}`)), nil
	}}
	for _, guarded := range []bool{true, false} {
		form := url.Values{"reason": {"maintenance"}}
		if guarded {
			form.Set("expected-id", "l2tp-00000000000000000000000000000001")
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/l2tp/42/disconnect?format=json", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handlers.handleL2TPDisconnect()(response, request)
		if guarded {
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			require.Equal(t, 1, calls)
			continue
		}
		require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
		require.Equal(t, 1, calls, "the unguarded form must not dispatch")
	}
}
