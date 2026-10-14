// Design: docs/architecture/api/birdwatcher-compat.md -- Section 3.1, a command the engine refuses
// Related: server.go -- query, which reports a failed dispatch as an error
// Related: handler_api.go -- engineAnswer and the handlers that answer through it
// Related: server_test.go -- mockDispatch, the fixture that serves every command the daemon serves

package lg

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	{"/api/looking-glass/routes/bmp/peer1", "show bgp rib protocol bmp peer peer1", "routes", true},
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

// TestAPIBMPRoutesNameIsTheRIBKey proves a client can walk from protocols/bmp
// to routes/bmp/{name}: the name it discovers is the key it routes by.
//
// VALIDATES: protocols/bmp names each peer by the rib-peer key `show bmp peers`
// reports, and routes/bmp/{name} dispatches exactly
// `show bgp rib protocol bmp peer <name>`, for an IPv4 router, a bracketed
// IPv6 router and a Loc-RIB instance.
// PREVENTS: the name being rebuilt from the router and BGP ID, which the RIB
// never stores routes under, and the command spelling the retired
// `show bgp rib-protocol`.
// Method: a dispatcher that answers only the two commands the walk needs and
// records what it was asked.
func TestAPIBMPRoutesNameIsTheRIBKey(t *testing.T) {
	keys := []string{
		"192.0.2.8:12345:10.0.0.1",
		"[2001:db8::8]:4000:2001:db8::1",
		"[2001:db8::8]:4000:loc-rib:7:3221225985",
		"[fe80::8%eth0.100]:4000:fe80::1",
		"192.0.2.8:12345:instance:1:4294967298:10.0.0.1",
	}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			var asked []string
			dispatch := func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
				asked = append(asked, cmd)
				var out string
				switch cmd {
				case cmdShowBMPPeers:
					out = `{"peers":[{"router":"r","rib-peer":"` + key + `","peer-as":65001,"up":true}]}`
				case "show bgp rib protocol bmp peer " + key:
					out = fakeRouteRowsAnswer
				default:
					return nil, errUnknownCommand
				}
				return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
			}

			rec := apiRequest(t, dispatch, "/api/looking-glass/protocols/bmp")
			if rec.Code != http.StatusOK {
				t.Fatalf("protocols/bmp status = %d; body %s", rec.Code, rec.Body.String())
			}
			var listed struct {
				Protocols map[string]any `json:"protocols"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
				t.Fatalf("decode protocols: %v", err)
			}
			if len(listed.Protocols) != 1 {
				t.Fatalf("protocols = %v, want one", listed.Protocols)
			}
			var name string
			for n := range listed.Protocols {
				name = n
			}
			if name != key {
				t.Fatalf("protocol name = %q, want the RIB key %q", name, key)
			}

			rec = apiRequest(t, dispatch, "/api/looking-glass/routes/bmp/"+url.PathEscape(name))
			if rec.Code != http.StatusOK {
				t.Fatalf("routes/bmp status = %d; body %s; asked %q", rec.Code, rec.Body.String(), asked)
			}
			if got, want := asked[len(asked)-1], "show bgp rib protocol bmp peer "+key; got != want {
				t.Errorf("dispatched %q, want %q", got, want)
			}
			var routes struct {
				Count int `json:"routes_count"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &routes); err != nil {
				t.Fatalf("decode routes: %v", err)
			}
			if routes.Count != 1 {
				t.Errorf("routes_count = %d, want 1; body %s", routes.Count, rec.Body.String())
			}
		})
	}
}

// TestAPIBMPRoutesRefusesACraftedName proves a name carrying command words is
// refused before anything is dispatched.
//
// VALIDATES: routes/bmp/{name} answers 400 for a name with a space, a pipe or
// a quote, and dispatches nothing.
// PREVENTS: a public client appending words or a pipe to the RIB command.
func TestAPIBMPRoutesRefusesACraftedName(t *testing.T) {
	for _, name := range []string{"192.0.2.8:1:10.0.0.1 best", "[::1]:1:a|count", `[::1]:1:"a"`} {
		t.Run(name, func(t *testing.T) {
			dispatched := false
			dispatch := func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
				dispatched = true
				return nil, errUnknownCommand
			}
			rec := apiRequest(t, dispatch, "/api/looking-glass/routes/bmp/"+url.PathEscape(name))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d; body %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if dispatched {
				t.Error("a refused name reached the dispatcher")
			}
		})
	}
}

// TestAPIBMPProtocolsListsOnlyNamesRoutesAnswers proves protocols/bmp leaves
// out every row it cannot name with a key routes/bmp/{name} accepts, and says
// so in one warning.
//
// VALIDATES: a keyless row, a second row with a rib-peer already listed, and a
// rib-peer whose zone holds a character outside the safe set are each left out
// and counted; the warning names the three counts and what a client loses; the
// unsafe name is refused by routes/bmp/{name} with nothing dispatched.
// PREVENTS: protocols/bmp listing a name routes/bmp answers 400 for, and one
// peer overwriting another's row in silence.
func TestAPIBMPProtocolsListsOnlyNamesRoutesAnswers(t *testing.T) {
	const zoned = "[fe80::8%eth0]:4000:fe80::1"
	const unsafe = "[fe80::8%e;x]:4000:fe80::1"
	dispatched := []string{}
	dispatch := func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
		dispatched = append(dispatched, cmd)
		if cmd != cmdShowBMPPeers {
			return nil, errUnknownCommand
		}
		out := `{"peers":[` +
			`{"router":"r","rib-peer":"` + zoned + `","peer-as":65001,"up":true},` +
			`{"router":"r","rib-peer":"` + zoned + `","peer-as":65002,"up":true},` +
			`{"router":"r","rib-peer":"` + unsafe + `","peer-as":65003,"up":true},` +
			`{"router":"r","peer-as":65004,"up":true}]}`
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
	}

	var logged bytes.Buffer
	srv, err := NewLGServer(LGConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		Dispatch:    dispatch,
		Logger:      slog.New(slog.NewTextHandler(&logged, nil)),
	})
	if err != nil {
		t.Fatalf("new lg server: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/looking-glass/protocols/bmp", http.NoBody)
	srv.server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("protocols/bmp status = %d; body %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Protocols map[string]map[string]any `json:"protocols"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode protocols: %v", err)
	}
	if len(listed.Protocols) != 1 {
		t.Fatalf("protocols = %v, want only %q", listed.Protocols, zoned)
	}
	if _, ok := listed.Protocols[zoned]; !ok {
		t.Fatalf("protocols = %v, want %q", listed.Protocols, zoned)
	}

	line := logged.String()
	for _, want := range []string{"level=WARN", "routes/bmp", "show bmp peers", "no-rib-peer=1", "duplicate-rib-peer=1", "unsafe-rib-peer=1"} {
		if !strings.Contains(line, want) {
			t.Errorf("warning %q lacks %q", line, want)
		}
	}
	if n := strings.Count(line, "level=WARN"); n != 1 {
		t.Errorf("logged %d warnings, want one: %s", n, line)
	}

	dispatched = dispatched[:0]
	rec = apiRequest(t, dispatch, "/api/looking-glass/routes/bmp/"+url.PathEscape(unsafe))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("routes/bmp for the unsafe zone answered %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if len(dispatched) != 0 {
		t.Errorf("a refused name was dispatched: %q", dispatched)
	}
}
