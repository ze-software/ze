// Design: docs/architecture/l2tp/subscriber-session-model.md -- guarded browser session actions

//go:build ze_l2tp

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// TestL2TPDisconnectPreservesBirthForSameSIDReplacement drives the real HTTP
// adapter against a command-owner double whose current birth changes while
// its readable SID stays 42. This proves transport of the browser precondition;
// the real captured reactor's atomic birth check needs its separate owner test.
func TestL2TPDisconnectPreservesBirthForSameSIDReplacement(t *testing.T) {
	original, err := subscriberid.ParseL2TPBirth("l2tp-00000000000000000000000000000001")
	require.NoError(t, err)
	replacement, err := subscriberid.ParseL2TPBirth("l2tp-00000000000000000000000000000002")
	require.NoError(t, err)
	current := original
	var cleared []subscriberid.L2TPBirth
	handlers := &l2TPHandlers{Dispatch: func(_ context.Context, _ plugin.CallerIdentity, input string) (*plugin.Response, error) {
		words, pipeAt, err := command.ScanExecution(input)
		require.NoError(t, err)
		require.Equal(t, -1, pipeAt)
		require.GreaterOrEqual(t, len(words), 6)
		require.Equal(t, []string{"clear", "l2tp", "session", "id", "42"}, words[:5])
		index := slices.Index(words, "expected-id")
		if index < 0 {
			return plugin.OperatorErrorResponse(opererr.New("the session reference is missing", "the readable session ID can be reused", "Open /l2tp")), nil
		}
		require.Less(t, index+1, len(words))
		expected, err := subscriberid.ParseL2TPBirth(words[index+1])
		require.NoError(t, err)
		if expected != current {
			return plugin.OperatorErrorResponse(opererr.New("the selected session has ended", "another session now holds its readable ID", "Open /l2tp")), nil
		}
		cleared = append(cleared, current)
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"status":"sent"}`)), nil
	}}

	response := disconnectBirthRequest(t, handlers, original.String())
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, []subscriberid.L2TPBirth{original}, cleared)

	current = replacement
	response = disconnectBirthRequest(t, handlers, original.String())
	require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	require.Equal(t, []subscriberid.L2TPBirth{original}, cleared, "the old page must not clear the same-SID replacement")

	response = disconnectBirthRequest(t, handlers, replacement.String())
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, []subscriberid.L2TPBirth{original, replacement}, cleared)
}

// TestL2TPDisconnectRefusesMalformedBirthBeforeDispatch proves that the HTTP
// boundary validates the complete canonical birth, including the absent and
// zero cases, before an operator command can affect a session.
func TestL2TPDisconnectRefusesMalformedBirthBeforeDispatch(t *testing.T) {
	for _, text := range []string{
		"", "l2tp-1-42", "l2tp-00000000000000000000000000000000",
		"l2tp-0123456789ABCDEF0123456789abcdef", "l2tp-00000000000000000000000000000001 extra",
	} {
		t.Run(text, func(t *testing.T) {
			calls := 0
			handlers := &l2TPHandlers{Dispatch: func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
				calls++
				return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"status":"sent"}`)), nil
			}}
			response := disconnectBirthRequest(t, handlers, text)
			require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			require.Zero(t, calls, "an invalid browser reference must not reach command dispatch")
		})
	}
}

func disconnectBirthRequest(t *testing.T, handlers *l2TPHandlers, birth string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"reason": {"maintenance"}, "expected-id": {birth}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/l2tp/42/disconnect?format=json", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handlers.handleL2TPDisconnect()(response, request)
	return response
}
