// Design: docs/architecture/web-interface.md -- browser terminal streams.

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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebCLIStreamFlushesAndCancels proves the first producer answer reaches
// the client before the producer ends, and closing the request ends its worker.
// MUTATION: omit flushing in cliStreamWriter.Write; the first read never arrives.
func TestWebCLIStreamFlushesAndCancels(t *testing.T) {
	stopped := make(chan struct{})
	dispatch := plugin.CommandStreamDispatcher(func(ctx context.Context, caller plugin.CallerIdentity, input string, output io.Writer) (bool, error) {
		assert.Equal(t, "alice", caller.Username)
		assert.Equal(t, "monitor event", input)
		defer close(stopped)
		if _, err := io.WriteString(output, "first answer\n"); err != nil {
			return true, err
		}
		<-ctx.Done()
		return true, ctx.Err()
	})
	handler := HandleCLITerminalStream(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("a streaming command reached the one-shot terminal")
		w.WriteHeader(http.StatusInternalServerError)
	}), dispatch)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r.WithContext(context.WithValue(r.Context(), ctxKeyUsername, "alice")))
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, strings.NewReader(url.Values{
		"command": {"run monitor event"}, "mode": {terminalModeConfig},
	}.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")
	answer := make([]byte, len("first answer\n"))
	_, err = io.ReadFull(resp.Body, answer)
	require.NoError(t, err)
	assert.Equal(t, "first answer\n", string(answer))
	cancel()
	require.NoError(t, resp.Body.Close())
	select {
	case <-stopped:
	case <-t.Context().Done():
		t.Fatal("closing the browser request did not stop the stream")
	}
}

// TestWebCLIStreamPreservesFiniteCommands proves non-streaming commands keep
// their complete original request, including pasted content and identity.
func TestWebCLIStreamPreservesFiniteCommands(t *testing.T) {
	handler := HandleCLITerminalStream(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "testuser", GetUsernameFromRequest(r))
		assert.Equal(t, "set bgp router-id 192.0.2.5", r.FormValue("command"))
		assert.Equal(t, "pasted body", r.FormValue("content"))
		w.Header().Set("Content-Type", "application/json")
		_, err := io.WriteString(w, `{"output":"finite answer"}`)
		assert.NoError(t, err)
	}), func(context.Context, plugin.CallerIdentity, string, io.Writer) (bool, error) {
		t.Error("a config mutation reached the stream dispatcher")
		return false, nil
	})
	w := httptest.NewRecorder()
	handler(w, authedRequest(t, http.MethodPost, "/cli/stream", url.Values{
		"command": {"set bgp router-id 192.0.2.5"}, "content": {"pasted body"},
	}))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "finite answer")
}

// TestWebCLIStreamDenialHasNoOutput proves a refused stream has no successful
// response headers or producer bytes, and an unauthenticated request is refused.
func TestWebCLIStreamDenialHasNoOutput(t *testing.T) {
	handler := HandleCLITerminalStream(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a refused stream reached the finite-command handler")
	}), func(context.Context, plugin.CallerIdentity, string, io.Writer) (bool, error) {
		return true, plugin.ErrStreamUnauthorized
	})
	w := httptest.NewRecorder()
	handler(w, authedRequest(t, http.MethodPost, "/cli/stream", url.Values{"command": {"monitor event"}, "mode": {terminalModeOperational}}))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), plugin.UnauthorizedMessage)
	unauth := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/cli/stream", strings.NewReader("command=monitor+event"))
	unauth.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	handler(w, unauth)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestWebCLIStreamBoundAndFailureAreVisible checks the writer's finite budget
// and an explicit error after partial output instead of an empty success.
func TestWebCLIStreamBoundAndFailureAreVisible(t *testing.T) {
	w := httptest.NewRecorder()
	writer := newCLIStreamWriter(w, func() {})
	writer.remaining = 3
	n, err := io.WriteString(writer, "abcd")
	assert.Zero(t, n)
	assert.ErrorIs(t, err, errCLIStreamOutputLimit)
	assert.Empty(t, w.Body.String())
	handler := HandleCLITerminalStream(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), func(_ context.Context, _ plugin.CallerIdentity, _ string, output io.Writer) (bool, error) {
		if _, err := io.WriteString(output, "first\n"); err != nil {
			return true, err
		}
		return true, errors.New("producer unavailable; retry the command")
	})
	w = httptest.NewRecorder()
	handler(w, authedRequest(t, http.MethodPost, "/cli/stream", url.Values{"command": {"monitor event"}, "mode": {terminalModeOperational}}))
	assert.Contains(t, w.Body.String(), "first\n")
	assert.Contains(t, w.Body.String(), "error: The live command could not continue.")
	assert.Contains(t, w.Body.String(), "producer unavailable; retry the command")
	assert.Contains(t, w.Body.String(), "fix: Open /cli and run the live command again")
}
