//go:build ze_web

// Design: docs/architecture/cli/command-completion.md -- actual authenticated plugin hints.

package hub

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zeweb "github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type completionPolicy struct {
	t       *testing.T
	denyAll bool
}

func (p *completionPolicy) Authorize(username, remote, input string, _ bool) bool {
	assert.Equal(p.t, "203.0.113.9:4567", remote)
	if p.denyAll {
		return false
	}
	if strings.HasPrefix(input, "system command complete") {
		return username != "rpc-denied"
	}
	assert.True(p.t, strings.HasPrefix(input, "show parity sample"), input)
	return username == "operator"
}

type completionAuthenticator struct{ policy plugin.Authorizer }

func (a completionAuthenticator) Authenticate(request aaa.AuthRequest) (aaa.AuthResult, error) {
	return aaa.AuthResult{Authenticated: request.Password == "synthetic", Authorizer: a.policy}, nil
}

// TestWebPluginCompletionUsesAuthenticatedCaller exercises Basic authentication,
// the real request factory, real dispatcher and real plugin mux. It checks live
// registration/retirement, both authorization layers, quote-preserving argument
// tails and caller cancellation without invoking the operational target.
func TestWebPluginCompletionUsesAuthenticatedCaller(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	server.Dispatcher().SetAuthorizer(&completionPolicy{t: t, denyAll: true})
	engineEnd, pluginEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	workerDone := make(chan struct{})
	workerErrors := make(chan error, 1)
	inputs := make(chan rpc.ExecuteCommandInput, 8)
	t.Cleanup(func() {
		_ = mux.Close()       //nolint:errcheck // test-owned connection teardown
		_ = pluginEnd.Close() //nolint:errcheck // test-owned connection teardown
		<-workerDone
		select {
		case workerErr := <-workerErrors:
			require.NoError(t, workerErr)
		default:
		}
	})
	proc := process.NewProcess(plugin.PluginConfig{Name: "completion-parity"})
	proc.SetConn(ipc.NewMuxPluginConn(mux))
	proc.SetRunning(true)
	go func() {
		defer close(workerDone)
		conn := rpc.NewConn(pluginEnd, pluginEnd)
		for range 8 { // This test makes at most eight completion requests.
			req, readErr := conn.ReadRequest(t.Context())
			if readErr != nil {
				return
			} // Cleanup closes the lifetime reader.
			var input rpc.ExecuteCommandInput
			if decodeErr := json.Unmarshal(req.Params, &input); decodeErr != nil {
				workerErrors <- decodeErr
				return
			}
			if writeErr := rpc.WriteDocumentAnswer(conn.AnswerWriter(t.Context()), req.ID, rpc.AnswerTail{}, []byte(`{"completions":[{"value":"private-value","short-help":"Live plugin value"},{"value":"two words|one"}]}`)); writeErr != nil {
				workerErrors <- writeErr
				return
			}
			inputs <- input
		}
	}()
	policy := &completionPolicy{t: t}
	source := webCompletionSource(&command.Node{}, server.Dispatcher().Registry().VisibleCommandEntries, server.WithArgCompletion, serverDispatcher(server, "web"))
	handler := zeweb.HandleCLICompleteWithSource(nil, source, serverOperationalCommand(server), nil, nil)
	store := zeweb.NewSessionStore(func() ([]authz.UserConfig, error) { return nil, nil })
	authenticated := zeweb.AuthMiddlewareWithAudit(store, completionAuthenticator{policy}, func(http.ResponseWriter, *http.Request) {}, handler, nil)
	request := func(ctx context.Context, username, input string, explain bool) *httptest.ResponseRecorder {
		query := url.Values{"input": {input}, "mode": {"operational"}}
		if explain {
			query.Set("explain", "true")
		}
		r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/cli/complete?"+query.Encode(), http.NoBody)
		r.RemoteAddr = "203.0.113.9:4567"
		r.SetBasicAuth(username, "synthetic")
		w := httptest.NewRecorder()
		authenticated.ServeHTTP(w, r)
		return w
	}
	assert.JSONEq(t, `[]`, request(t.Context(), "operator", "show parity sample ", false).Body.String())
	registrations := server.Dispatcher().Registry().Register(proc, []pluginserver.CommandDef{{Name: "show parity sample", Completable: true, Description: "Read the selected sample."}})
	require.True(t, registrations[0].OK, registrations[0].Error)
	allowed := request(t.Context(), "operator", "show parity sample pr", false)
	require.Equal(t, http.StatusOK, allowed.Code)
	assert.JSONEq(t, `[{"text":"private-value","description":"Live plugin value","type":"value"}]`, allowed.Body.String())
	first := <-inputs
	assert.Equal(t, "show parity sample", first.Command)
	assert.Empty(t, first.Args)
	assert.Equal(t, "pr", first.Peer)
	quoted := request(t.Context(), "operator", `show parity sample "two words|one" `, false)
	require.Equal(t, http.StatusOK, quoted.Code)
	assert.Contains(t, quoted.Body.String(), "private-value")
	second := <-inputs
	assert.Equal(t, []string{"two words|one"}, second.Args)
	assert.Empty(t, second.Peer)
	for _, username := range []string{"reader", "rpc-denied"} {
		denied := request(t.Context(), username, "show parity sample ", false)
		assert.Equal(t, http.StatusForbidden, denied.Code, username)
		assert.NotContains(t, denied.Body.String(), "private-value")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	canceled := request(ctx, "operator", "show parity sample ", false)
	assert.Equal(t, http.StatusRequestTimeout, canceled.Code)
	assert.NotContains(t, canceled.Body.String(), "private-value")
	assert.JSONEq(t, `{"explanation":"Read the selected sample.","found":true}`, request(t.Context(), "operator", `show parity sample "two words|one"`, true).Body.String())
	server.Dispatcher().Registry().Unregister(proc, []string{"show parity sample"})
	assert.JSONEq(t, `[]`, request(t.Context(), "operator", "show parity sample ", false).Body.String())
	select {
	case extra := <-inputs:
		t.Fatalf("denied, canceled or retired completion reached plugin: %+v", extra)
	default:
	}
}
