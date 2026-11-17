// Design: docs/architecture/web-interface.md -- authenticated durable recall.
package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestCLIHistoryHTTPProjectsSubmittedCommands follows the actual terminal POST
// and CLI GET, so a disconnected projection cannot pretend recall is durable.
func TestCLIHistoryHTTPProjectsSubmittedCommands(t *testing.T) {
	mgr := newTestEditorManager(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	terminalAsUser(t, mgr, "alice", nil, "set bgp router-id 192.0.2.77")
	w := httptest.NewRecorder()
	r := authedRequest(t, http.MethodGet, "/cli?history=1&mode=config", nil)
	r = r.WithContext(context.WithValue(r.Context(), ctxKeyUsername, "alice"))
	HandleCLIPageHTTP(renderer, false, mgr).ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var answer struct {
		Entries []string `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
	require.Equal(t, []string{"set bgp router-id 192.0.2.77"}, answer.Entries)
}

// TestCLIHistoryHTTPKeepsTrustedIdentityAndExactModes refuses unsupported mode
// spellings and ignores a query username while returning useful own-user recall.
func TestCLIHistoryHTTPKeepsTrustedIdentityAndExactModes(t *testing.T) {
	mgr := newTestEditorManager(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	require.NoError(t, mgr.RecordHistory("testuser", cli.ModeOperational, nil, "show own", "one"))
	require.NoError(t, mgr.RecordHistory("other", cli.ModeOperational, nil, "show other", "two"))
	handler := HandleCLIPageHTTP(renderer, false, mgr)
	w := httptest.NewRecorder()
	handler(w, authedRequest(t, http.MethodGet, "/cli?history=1&mode=operational&username=other", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"entries":["show own"]}`, w.Body.String())
	require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
	for _, mode := range []string{"", "Operational", "bogus"} {
		w = httptest.NewRecorder()
		handler(w, authedRequest(t, http.MethodGet, "/cli?history=1&mode="+mode, nil))
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
	w = httptest.NewRecorder()
	handler(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/cli?history=1&mode=config", http.NoBody))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestCLIHistoryStreamFallbackUsesOneFrozenSubmission inserts another accepted
// command between stream rejection and finite fallback. A second recorder would
// then append a third entry despite ordinary consecutive dedup.
func TestCLIHistoryStreamFallbackUsesOneFrozenSubmission(t *testing.T) {
	mgr := newTestEditorManager(t)
	lookups := 0
	registered := func(string) bool { lookups++; return true }
	terminal := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, mgr.schema, nil, nil, nil, nil, registered)
	stream := HandleCLITerminalStream(mgr, terminal, func(context.Context, plugin.CallerIdentity, string, io.Writer) (bool, error) {
		require.NoError(t, mgr.RecordHistory("testuser", cli.ModeOperational, nil, "show intervening", "between"))
		return false, nil
	}, registered)
	w := httptest.NewRecorder()
	stream(w, authedRequest(t, http.MethodPost, "/cli/stream", url.Values{"command": {"delete agent skill"}, "mode": {terminalModeOperational}, "submission": {"first"}}))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, lookups)
	entries, err := mgr.HistorySnapshot("testuser", cli.ModeOperational)
	require.NoError(t, err)
	require.Equal(t, []string{"delete agent skill", "show intervening"}, entries)
	entries, err = mgr.HistorySnapshot("testuser", cli.ModeConfig)
	require.NoError(t, err)
	require.Empty(t, entries)
}

// TestCLIHistoryHTTPRecordsBarAndContentContinuation follows both actual POST
// handlers and proves the uploaded body is absent from recall and persistence.
func TestCLIHistoryHTTPRecordsBarAndContentContinuation(t *testing.T) {
	mgr := newTestEditorManager(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	bar := HandleCLICommandWithAuthorizerAndAudit(mgr, mgr.schema, renderer, nil, nil)
	bar(httptest.NewRecorder(), authedRequest(t, http.MethodPost, "/cli", url.Values{"command": {"show"}, "submission": {"bar"}}))
	terminal := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, mgr.schema, nil, nil, nil, nil)
	load := url.Values{"command": {"load terminal relative merge"}, "mode": {terminalModeConfig}, "submission": {"load"}}
	terminal(httptest.NewRecorder(), authedRequest(t, http.MethodPost, "/cli/terminal", load))
	require.NoError(t, mgr.RecordHistory("testuser", cli.ModeConfig, nil, "compare", "intervening"))
	load.Set("content", "uploaded-content-must-not-be-history")
	terminal(httptest.NewRecorder(), authedRequest(t, http.MethodPost, "/cli/terminal", load))
	entries, err := mgr.HistorySnapshot("testuser", cli.ModeConfig)
	require.NoError(t, err)
	require.Equal(t, []string{"show", "load terminal relative merge", "compare"}, entries)
	require.NoError(t, mgr.Close())
	data, err := mgr.store.ReadFile(zefs.KeyHistory.Prefix() + "testuser/config")
	require.NoError(t, err)
	require.NotContains(t, string(data), "uploaded-content")
}

// TestCLIHistoryFailureWarnsWithoutBlockingExecution keeps a known history read
// failure while a useful operational result still reaches the terminal caller.
func TestCLIHistoryFailureWarnsWithoutBlockingExecution(t *testing.T) {
	mgr := newTestEditorManager(t)
	store := &browserHistoryStore{Storage: mgr.store}
	store.readFailure.Store(true)
	mgr.store = store
	terminal := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, mgr.schema, nil, func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"result": "useful diagnostic answer"}), nil
	}, nil, nil)
	w := httptest.NewRecorder()
	terminal(w, authedRequest(t, http.MethodPost, "/cli/terminal", url.Values{"command": {"show diagnostic"}, "mode": {terminalModeOperational}}))
	require.Equal(t, http.StatusOK, w.Code)
	var answer terminalResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
	require.Contains(t, answer.Output, "useful diagnostic answer")
	require.Contains(t, answer.HistoryWarning, "history read unavailable")
	require.Contains(t, w.Header().Get(historyWarningHeader), "history read unavailable")
	renderer, err := NewRenderer()
	require.NoError(t, err)
	w = httptest.NewRecorder()
	HandleCLIPageHTTP(renderer, false, mgr)(w, authedRequest(t, http.MethodGet, "/cli?history=1&mode=operational", nil))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestCLIHistoryAutomaticViewAndControlsDoNotAddRecall keeps automatic initial
// show and Enter-consumed control commands out beside a useful auto-entered edit.
func TestCLIHistoryAutomaticViewAndControlsDoNotAddRecall(t *testing.T) {
	mgr := newTestEditorManager(t)
	terminal := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, mgr.schema, nil, nil, nil, nil)
	for _, form := range []url.Values{
		{"command": {"show"}, "history": {"0"}},
		{"command": {"configure"}, "mode": {terminalModeOperational}},
		{"command": {"exit"}},
		{"command": {"run"}},
		{"command": {"set bgp router-id 192.0.2.78"}, "mode": {terminalModeOperational}},
	} {
		terminal(httptest.NewRecorder(), authedRequest(t, http.MethodPost, "/cli/terminal", form))
	}
	entries, err := mgr.HistorySnapshot("testuser", cli.ModeConfig)
	require.NoError(t, err)
	require.Equal(t, []string{"set bgp router-id 192.0.2.78"}, entries)
	w := httptest.NewRecorder()
	terminal(w, authedRequest(t, http.MethodPost, "/cli/terminal", url.Values{"command": {"show"}, "mode": {"bogus"}}))
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestWebServerShutdownCancelsAndJoinsActivePost runs a handler that can only
// finish when its HTTP context ends, then requires shutdown to join that end.
func TestWebServerShutdownCancelsAndJoinsActivePost(t *testing.T) {
	cert, key, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)
	srv, err := NewWebServer(WebConfig{ListenAddrs: []string{"127.0.0.1:0"}, CertPEM: cert, KeyPEM: key})
	require.NoError(t, err)
	entered := make(chan struct{})
	finished := make(chan struct{})
	srv.HandleFunc("POST /cli/stream", func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(finished)
	})
	requestCtx, cancelRequest := context.WithCancel(t.Context())
	t.Cleanup(cancelRequest)
	go srv.server.Handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(requestCtx, http.MethodPost, "/cli/stream", nil))
	<-entered
	shutdownCtx, cancelShutdown := context.WithCancel(t.Context())
	cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("shutdown returned while an admitted POST stream still owned its context")
	}
}
