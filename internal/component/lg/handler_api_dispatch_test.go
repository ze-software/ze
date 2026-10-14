// Design: docs/architecture/api/birdwatcher-compat.md -- Section 3.1, a command the engine refuses
// Related: server.go -- query, which reports a failed dispatch as an error
// Related: handler_api.go -- engineAnswer and the handlers that answer through it
// Related: server_test.go -- mockDispatch, the fixture that serves every command the daemon serves

package lg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// apiRequest drives one GET through the server's own handler chain, the entry
// point an operator's client reaches.
func apiRequest(t *testing.T, dispatch CommandDispatcher, target string) *httptest.ResponseRecorder {
	t.Helper()

	srv, err := NewLGServer(LGConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		Dispatch:    dispatch,
	})
	if err != nil {
		t.Fatalf("new lg server: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	srv.server.Handler.ServeHTTP(rec, req)

	return rec
}

// dispatchEndpoints are the API endpoints that answer from one dispatched
// command, with the command each one sends and the key its payload sits under.
// peerScoped marks the endpoints that answer through peerRoutesAnswer, which
// reads an error envelope as 404 rather than 502.
var dispatchEndpoints = []struct {
	target     string
	command    string
	key        string
	peerScoped bool
}{
	{"/api/looking-glass/protocols/bgp", cmdBGPOverview, "protocols", false},
	{"/api/looking-glass/protocols/short", cmdBGPOverview, "protocols", false},
	{"/api/looking-glass/protocols/bmp", cmdShowBMPPeers, "protocols", false},
	{"/api/looking-glass/routes/prefix?prefix=10.0.0.0/24", "show bgp rib received prefix 10.0.0.0/24", "routes", false},
	{"/api/looking-glass/routes/protocol/peer1", "show bgp rib received peer peer1", "routes", true},
	{"/api/looking-glass/routes/peer/192.0.2.1", "show bgp rib received peer 192.0.2.1", "routes", true},
	{"/api/looking-glass/routes/export/peer1", "show bgp rib sent peer peer1", "routes", true},
	{"/api/looking-glass/routes/bmp/peer1", "show bgp rib-protocol bmp peer1", "routes", true},
}

// errorBody decodes a refusal and returns its message, failing the test when
// the body is not the JSON error object the contract names.
func errorBody(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v; body %s", err, rec.Body.String())
	}
	message, ok := body["error"].(string)
	if !ok {
		t.Fatalf("body carries no error string: %s", rec.Body.String())
	}
	return message
}

// TestAPIDispatchFailureIsRefused proves that a dispatch the engine fails is
// answered as a failure, never as data.
//
// VALIDATES: each endpoint answers 502 with a JSON error naming the command it
// sent and "engine error", and never the dispatcher's own error, which can
// carry a host detail (birdwatcher-compat.md Section 3.1).
// PREVENTS: query rendering the failure as {"error": ...}, which parsed, so
// the handlers transformed the refusal into an empty protocols object or an
// empty routes array and answered 200 over it
// (plan/journal/zero-value-as-valid-answer.md, 2026-09-20).
func TestAPIDispatchFailureIsRefused(t *testing.T) {
	failing := func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return nil, errTestEngineDown
	}

	for _, endpoint := range dispatchEndpoints {
		t.Run(endpoint.target, func(t *testing.T) {
			rec := apiRequest(t, failing, endpoint.target)

			if rec.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusBadGateway, rec.Body.String())
			}
			message := errorBody(t, rec)
			if want := endpoint.command + ": " + failureEngineError; message != want {
				t.Errorf("error = %q, want %q", message, want)
			}
			if strings.Contains(message, errTestEngineDown.Error()) {
				t.Errorf("error %q carries the dispatcher's own error", message)
			}
		})
	}
}

// TestAPIProtocolsRefusalEnvelopeIsRefused proves that an answer which is the
// engine's own error envelope is refused, the second failure shape a plugin
// produces (bgp-rib answers a command it cannot parse with {"error": ...}).
//
// VALIDATES: protocols/bgp, protocols/short, protocols/bmp and routes/prefix
// answer 502 naming the command and "engine error" when the answer is an
// error envelope.
// PREVENTS: the protocols transforms rendering the envelope as an empty set.
func TestAPIProtocolsRefusalEnvelopeIsRefused(t *testing.T) {
	for _, endpoint := range dispatchEndpoints {
		if endpoint.peerScoped {
			continue
		}
		t.Run(endpoint.target, func(t *testing.T) {
			rec := apiRequest(t, envelopeDispatch, endpoint.target)

			if rec.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusBadGateway, rec.Body.String())
			}
			if want, message := endpoint.command+": "+failureEngineError, errorBody(t, rec); message != want {
				t.Errorf("error = %q, want %q", message, want)
			}
		})
	}
}

// envelopeDispatch answers every command with the error envelope bgp-rib
// answers a command it cannot parse.
func envelopeDispatch(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
	return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"error":"refused here"}`)), nil
}

// TestAPIPeerRoutesEnvelopeIsNotFound proves the peer-scoped route endpoints
// keep the two failures apart: the engine's envelope is 404 carrying its
// refusal, while TestAPIDispatchFailureIsRefused holds a failed dispatch at
// 502.
//
// VALIDATES: routes/protocol, routes/peer, routes/export and routes/bmp answer
// 404 with the envelope's own message.
// PREVENTS: peerRoutesAnswer swapping its branches, which would answer a dead
// engine as an absent peer and an absent peer as a dead engine.
func TestAPIPeerRoutesEnvelopeIsNotFound(t *testing.T) {
	for _, endpoint := range dispatchEndpoints {
		if !endpoint.peerScoped {
			continue
		}
		t.Run(endpoint.target, func(t *testing.T) {
			rec := apiRequest(t, envelopeDispatch, endpoint.target)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusNotFound, rec.Body.String())
			}
			if message := errorBody(t, rec); message != "refused here" {
				t.Errorf("error = %q, want the engine's refusal %q", message, "refused here")
			}
		})
	}
}

// TestAPIDispatchSuccessAnswers proves the refusal above is not a refusal of
// everything: a served command still answers 200 with its payload.
//
// VALIDATES: each endpoint answers 200 and carries its payload key over
// mockDispatch, which serves every command those endpoints send.
// PREVENTS: a fix that refuses by default.
func TestAPIDispatchSuccessAnswers(t *testing.T) {
	for _, endpoint := range dispatchEndpoints {
		t.Run(endpoint.target, func(t *testing.T) {
			rec := apiRequest(t, mockDispatch(), endpoint.target)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v; body %s", err, rec.Body.String())
			}
			if _, ok := body[endpoint.key]; !ok {
				t.Errorf("body carries no %q: %s", endpoint.key, rec.Body.String())
			}
			if _, refused := body["error"]; refused {
				t.Errorf("body carries an error: %s", rec.Body.String())
			}
		})
	}
}
