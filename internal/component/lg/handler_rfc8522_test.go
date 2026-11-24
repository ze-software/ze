// Design: docs/architecture/web-interface.md -- RFC 8522 Looking Glass Command Set conformance tests
// Related: handler_rfc8522.go -- RFC 8522 HTTP handlers and JSend envelope
// Related: server_tls_test.go -- TLS certificate and live HTTPS server helpers

package lg

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	// Schema-only leaves declaring every daemon command handler_rfc8522.go dispatches.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer/yang"
	_ "github.com/ze-software/ze/internal/component/iface/yang"
	_ "github.com/ze-software/ze/internal/component/ping/yang"
	_ "github.com/ze-software/ze/internal/component/traceroute/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
)

const testRFC8522Hostname = "edge01.example.net"

var errRFC8522TestEngineFailed = errors.New("router backend failed")

// rfc8522MockDispatch serves every daemon command handler_rfc8522.go dispatches
// and records each command string received.
func rfc8522MockDispatch(recorded *[]string) CommandDispatcher {
	base := mockDispatch()
	return func(ctx context.Context, caller plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
		if recorded != nil {
			*recorded = append(*recorded, cmd)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		switch {
		case cmd == "show ping 192.0.2.1" || strings.HasPrefix(cmd, "show ping 192.0.2.1 "):
			out := `{"destination":"192.0.2.1","sent":5,"received":5,"loss-percent":0,` +
				`"min-rtt-ms":1.2,"avg-rtt-ms":1.5,"max-rtt-ms":2.0,` +
				`"replies":[{"seq":0,"status":"ok","rtt-ms":1.2},{"seq":1,"status":"ok","rtt-ms":1.5}]}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		case cmd == "show ping 2001:db8::35" || strings.HasPrefix(cmd, "show ping 2001:db8::35 "):
			out := `{"destination":"2001:db8::35","sent":5,"received":5,"loss-percent":0,` +
				`"min-rtt-ms":40,"avg-rtt-ms":41,"max-rtt-ms":44,` +
				`"replies":[{"seq":0,"status":"ok","rtt-ms":40}]}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		case cmd == "show ping example.com" || strings.HasPrefix(cmd, "show ping example.com "):
			out := `{"destination":"example.com","sent":4,"received":4,"loss-percent":0,` +
				`"min-rtt-ms":10.5,"avg-rtt-ms":11.0,"max-rtt-ms":12.5,` +
				`"replies":[{"seq":0,"status":"ok","rtt-ms":10.5}]}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		case cmd == "show ping 192.0.2.5" || strings.HasPrefix(cmd, "show ping 192.0.2.5 "):
			out := `{"destination":"192.0.2.5","sent":5,"received":0,"loss-percent":100,` +
				`"replies":[{"seq":0,"status":"timeout"},{"seq":1,"status":"timeout"}]}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		case cmd == "show traceroute 192.0.2.8":
			out := `{"hops":[{"ttl":1,"address":"198.51.100.1","rtt-ms":1.5},{"ttl":2,"address":"192.0.2.8","rtt-ms":3.2}]}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		case cmd == "show route 192.0.2.0/24":
			out := `{"routes":[{"dst":"192.0.2.0/24","gateway":"198.51.100.1","interface":"eth0","protocol":"bgp","metric":100}]}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		case cmd == "show route 2001:db8::/48":
			out := `{"routes":[{"dst":"2001:db8::/48","gateway":"fe80::1","interface":"eth0","protocol":"static","metric":1}]}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		case cmd == "show route 198.51.100.0/24":
			out := `{"routes":[]}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		case cmd == "show route lookup 192.0.2.1":
			out := `{"dst":"192.0.2.0/24","gateway":"198.51.100.1","interface":"eth0","protocol":"bgp","metric":100}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		case cmd == "show route lookup 198.51.100.9":
			out := `{"error":"no route to host"}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		case cmd == "show bgp peer 10.0.0.1 detail":
			out := `{"peers":{"10.0.0.1":{"remote-as":65001,"peer-type":"external","router-id":"10.0.0.1","state":"established","uptime":"1h0m0s"}}}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		case cmd == "show bgp peer 192.0.2.99 detail":
			out := `{"peers":{}}`
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		default:
			return base(ctx, caller, cmd)
		}
	}
}

// rfc8522RequestFrom sends a request from remoteAddr through the full LGServer
// handler chain with TLS populated (when useTLS is true).
func rfc8522RequestFrom(t *testing.T, srv *LGServer, method, target, remoteAddr string, useTLS bool) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody)
	req.Host = "lg.example.net"
	req.RemoteAddr = remoteAddr
	if useTLS {
		req.TLS = &tls.ConnectionState{Version: tls.VersionTLS13}
	}
	srv.server.Handler.ServeHTTP(rec, req)
	return rec
}

// rfc8522Request sends a request through the full LGServer handler chain with
// TLS populated (when useTLS is true) and returns the recorded response.
func rfc8522Request(t *testing.T, srv *LGServer, method, target string, useTLS bool) *httptest.ResponseRecorder {
	t.Helper()
	return rfc8522RequestFrom(t, srv, method, target, "198.51.100.10:54321", useTLS)
}

func newRFC8522TestServer(t *testing.T, dispatch CommandDispatcher, rateLimit int) *LGServer {
	t.Helper()
	srv, err := NewLGServer(LGConfig{
		ListenAddrs:            []ListenAddr{{Addr: "127.0.0.1:0"}},
		Dispatch:               dispatch,
		Hostname:               testRFC8522Hostname,
		RFC8522RateLimitPerMin: rateLimit,
	})
	if err != nil {
		t.Fatalf("NewLGServer: %v", err)
	}
	return srv
}

func decodeJSend(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != rfc8522FormatJSON {
		t.Fatalf("Content-Type = %q, want %q", ct, rfc8522FormatJSON)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v (raw=%s)", err, rec.Body.String())
	}
	return body
}

// assertJSendKeys enforces strict JSend top-level key rules (RFC 8522 Appendix A).
func assertJSendKeys(t *testing.T, body map[string]any, wantStatus string) {
	t.Helper()
	gotStatus, ok := body["status"].(string)
	if !ok {
		t.Fatalf("missing string status field: %v", body)
	}
	if gotStatus != wantStatus {
		t.Fatalf("status = %q, want %q (body=%v)", gotStatus, wantStatus, body)
	}
	switch wantStatus {
	case jsendSuccess, jsendFail:
		if len(body) != 2 {
			t.Fatalf("JSend %s response must contain only status and data at top level, got keys: %v", wantStatus, body)
		}
		if _, isObj := body["data"].(map[string]any); !isObj {
			t.Fatalf("JSend %s response missing data object: %v", wantStatus, body)
		}
	case jsendError:
		msg, hasMsg := body["message"].(string)
		if !hasMsg || msg == "" {
			t.Fatalf("JSend error response missing non-empty message: %v", body)
		}
		for k := range body {
			if k != "status" && k != "message" && k != "code" && k != "data" {
				t.Fatalf("unexpected top-level key %q in JSend error response", k)
			}
		}
	}
}

// rfc8522OutputText asserts data["output"] is a non-empty JSON string array
// (RFC 8522 Section 2.3.1 text/plain line array) and returns the joined text.
func rfc8522OutputText(t *testing.T, data map[string]any) string {
	t.Helper()
	raw, ok := data["output"].([]any)
	if !ok || len(raw) == 0 {
		t.Fatalf("data.output missing or empty line array: %v", data)
	}
	lines := make([]string, 0, len(raw))
	for _, elem := range raw {
		s, isStr := elem.(string)
		if !isStr {
			t.Fatalf("data.output contains non-string element %T: %v", elem, raw)
		}
		lines = append(lines, s)
	}
	return strings.Join(lines, "\n")
}

// assertRequiredDataFields validates performed_at (UTC ISO 8601 bounded to the
// [before, after] execution window) and runtime (>= 0).
func assertRequiredDataFields(t *testing.T, data map[string]any, before, after time.Time) {
	t.Helper()
	performedAt, ok := data["performed_at"].(string)
	if !ok || performedAt == "" {
		t.Fatalf("data.performed_at missing or empty: %v", data)
	}
	if !strings.HasSuffix(performedAt, "Z") {
		t.Fatalf("data.performed_at = %q, must be UTC ISO 8601 ending in Z", performedAt)
	}
	ts, err := time.Parse(time.RFC3339, performedAt)
	if err != nil {
		t.Fatalf("data.performed_at = %q is not valid RFC3339/ISO 8601: %v", performedAt, err)
	}
	low := before.UTC().Truncate(time.Second)
	high := after.UTC().Add(time.Second)
	if ts.Before(low) || ts.After(high) {
		t.Fatalf("data.performed_at = %s outside execution window [%s, %s]", ts.Format(time.RFC3339), low.Format(time.RFC3339), high.Format(time.RFC3339))
	}
	rt, ok := data["runtime"].(float64)
	if !ok {
		t.Fatalf("data.runtime missing or non-numeric: %v", data)
	}
	if rt < 0 {
		t.Fatalf("data.runtime = %v, must be >= 0", rt)
	}
}

// TestRFC8522WellKnownPrefixAndRejections proves the URI prefix requirement,
// closed command allowlist, and no direct client-to-router communication.
//
// VALIDATES: /.well-known/looking-glass/v1/... is served; non-well-known paths
// do not serve the RFC 8522 API; allowlisted commands reach CommandDispatcher
// while unknown commands, injection payloads, and non-GET methods are refused
// without invoking CommandDispatcher; Upgrade tunneling is rejected and no 3xx
// redirect or raw backend error leaks to the client.
// PREVENTS: namespace collision outside /.well-known/looking-glass, arbitrary
// command execution on the router, or direct client-router communication.
//
// RFC requirement: RFC8522-2-1 positive -- /.well-known/looking-glass/v1/routers serves the RFC 8522 API under the required well-known prefix.
// RFC requirement: RFC8522-2-1 negative -- /looking-glass/v1/routers without the /.well-known/looking-glass prefix is not served as an RFC 8522 endpoint.
// RFC requirement: RFC8522-2-2 positive -- an allowlisted command (/.well-known/looking-glass/v1/ping/192.0.2.1) reaches CommandDispatcher with the exact predefined router command.
// RFC requirement: RFC8522-2-2 negative -- unknown commands, POST requests, and injection payloads in host/route parameters are rejected with HTTP 400 JSend error without invoking CommandDispatcher.
// RFC requirement: RFC8522-2-3 positive -- router commands execute solely through CommandDispatcher with no 3xx Location redirect and redacted backend errors.
// RFC requirement: RFC8522-2-3 negative -- Upgrade header requests attempting a direct client-to-router tunnel are rejected with HTTP 400 JSend error without invoking CommandDispatcher.
func TestRFC8522WellKnownPrefixAndRejections(t *testing.T) {
	var calls []string
	srv := newRFC8522TestServer(t, rfc8522MockDispatch(&calls), 0)

	// Positive 2-1: well-known prefix answers JSend success.
	rec := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/routers", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("well-known prefix status = %d, want 200", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("unexpected Location redirect %q", loc)
	}
	body := decodeJSend(t, rec)
	assertJSendKeys(t, body, jsendSuccess)

	// Positive 2-2 & 2-3: allowlisted ping reaches CommandDispatcher with no redirect.
	calls = nil
	recPing := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/ping/192.0.2.1", true)
	if recPing.Code != http.StatusOK {
		t.Fatalf("allowlisted ping status = %d, want 200; body=%s", recPing.Code, recPing.Body.String())
	}
	if loc := recPing.Header().Get("Location"); loc != "" {
		t.Fatalf("ping returned unexpected Location redirect %q", loc)
	}
	if len(calls) != 1 || calls[0] != "show ping 192.0.2.1" {
		t.Fatalf("allowlisted ping dispatched %v, want [show ping 192.0.2.1]", calls)
	}

	// Positive 2-3: backend error is proxied as redacted 502, never exposing raw router error.
	failSrv := newRFC8522TestServer(t, func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return nil, errRFC8522TestEngineFailed
	}, 0)
	recErr := rfc8522Request(t, failSrv, http.MethodGet, "/.well-known/looking-glass/v1/ping/192.0.2.1", true)
	if recErr.Code != http.StatusBadGateway {
		t.Fatalf("backend error status = %d, want 502", recErr.Code)
	}
	errPayload := decodeJSend(t, recErr)
	if strings.Contains(errPayload["message"].(string), errRFC8522TestEngineFailed.Error()) {
		t.Fatalf("backend error leaked internal message: %q", errPayload["message"])
	}

	// Negative 2-1: path outside /.well-known/looking-glass does not serve RFC 8522.
	recOutside := rfc8522Request(t, srv, http.MethodGet, "/looking-glass/v1/routers", true)
	if recOutside.Code != http.StatusNotFound {
		t.Fatalf("non-well-known path status = %d, want 404", recOutside.Code)
	}

	// Negative 2-2: arbitrary commands, non-GET methods, and injection attempts never reach dispatcher.
	calls = nil
	rejected := []struct {
		name   string
		method string
		path   string
	}{
		{"unknown command", http.MethodGet, "/.well-known/looking-glass/v1/exec/uname"},
		{"arbitrary show", http.MethodGet, "/.well-known/looking-glass/v1/show/config"},
		{"bare prefix", http.MethodGet, "/.well-known/looking-glass"},
		{"post method", http.MethodPost, "/.well-known/looking-glass/v1/routers"},
		{"ping injection semicolon", http.MethodGet, "/.well-known/looking-glass/v1/ping/192.0.2.1;id"},
		{"ping injection pipe", http.MethodGet, "/.well-known/looking-glass/v1/ping/192.0.2.1|count"},
		{"route injection pipe", http.MethodGet, "/.well-known/looking-glass/v1/show/route/192.0.2.0/24|count"},
		{"route injection space", http.MethodGet, "/.well-known/looking-glass/v1/show/route/192.0.2.0%20limit%201"},
		{"unknown router", http.MethodGet, "/.well-known/looking-glass/v1/routers?router=other.example.net"},
		{"unknown routerindex", http.MethodGet, "/.well-known/looking-glass/v1/routers?routerindex=9"},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			r := rfc8522Request(t, srv, tc.method, tc.path, true)
			if r.Code != http.StatusBadRequest {
				t.Fatalf("%s status = %d, want 400; body=%s", tc.name, r.Code, r.Body.String())
			}
			errBody := decodeJSend(t, r)
			assertJSendKeys(t, errBody, jsendError)
		})
	}
	if len(calls) != 0 {
		t.Fatalf("rejected requests invoked dispatcher: %v", calls)
	}

	// Negative 2-3: Upgrade header is rejected so no direct tunnel can form.
	calls = nil
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/looking-glass/v1/ping/192.0.2.1", http.NoBody)
	req.TLS = &tls.ConnectionState{Version: tls.VersionTLS13}
	req.Header.Set("Upgrade", "websocket")
	upRec := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(upRec, req)
	if upRec.Code != http.StatusBadRequest {
		t.Fatalf("Upgrade request status = %d, want 400", upRec.Code)
	}
	assertJSendKeys(t, decodeJSend(t, upRec), jsendError)
	if len(calls) != 0 {
		t.Fatalf("Upgrade request invoked dispatcher: %v", calls)
	}
}

// TestRFC8522RandomParameterIgnored proves the server ignores the random query
// parameter value completely.
//
// VALIDATES: empty, alphanumeric, and special-character values of random
// (including case-insensitive RANDOM) return 200 success with the exact same
// router command dispatch and response payload data as the baseline request
// without random.
// PREVENTS: rejecting, altering behavior on, or forwarding the random parameter.
//
// RFC requirement: RFC8522-2.2-1 positive -- empty, alphanumeric, and case-insensitive random query parameters are ignored, dispatching the identical router command and producing identical response data to a baseline request without random.
func TestRFC8522RandomParameterIgnored(t *testing.T) {
	var calls []string
	srv := newRFC8522TestServer(t, rfc8522MockDispatch(&calls), 0)

	baseRec := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/ping/192.0.2.1", true)
	if baseRec.Code != http.StatusOK {
		t.Fatalf("baseline status = %d, want 200", baseRec.Code)
	}
	baseData := decodeJSend(t, baseRec)["data"].(map[string]any)

	targets := []string{
		"/.well-known/looking-glass/v1/ping/192.0.2.1?random=",
		"/.well-known/looking-glass/v1/ping/192.0.2.1?random=517A93B50",
		"/.well-known/looking-glass/v1/ping/192.0.2.1?RANDOM=ignored;value|test",
	}
	for _, target := range targets {
		rec := rfc8522Request(t, srv, http.MethodGet, target, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200; body=%s", target, rec.Code, rec.Body.String())
		}
		body := decodeJSend(t, rec)
		assertJSendKeys(t, body, jsendSuccess)
		gotData := body["data"].(map[string]any)
		for _, field := range []string{"router", "format", "output", "min", "avg", "max", "rate"} {
			if !reflect.DeepEqual(gotData[field], baseData[field]) {
				t.Fatalf("GET %s changed data.%s: got %v, want %v", target, field, gotData[field], baseData[field])
			}
		}
	}
	if len(calls) != len(targets)+1 {
		t.Fatalf("expected %d dispatch calls, got %d: %v", len(targets)+1, len(calls), calls)
	}
	for _, cmd := range calls {
		if cmd != "show ping 192.0.2.1" {
			t.Fatalf("dispatched command = %q, want %q (random leaked into command)", cmd, "show ping 192.0.2.1")
		}
	}
}

// TestRFC8522JSendEnvelopeAndTiming proves JSend compliance across success,
// fail, and error responses including mandatory performed_at, runtime,
// extra-data placement inside "data", HTTP status codes, and 401 auth errors.
//
// VALIDATES: every RFC 8522 response uses JSend with valid status ("success",
// "fail", or "error"), HTTP 200 on success/fail, UTC ISO 8601 performed_at
// matching execution time, real elapsed wallclock runtime, extra fields strictly
// inside data, and error status codes 400/401/500/502/504 with meaningful messages.
// PREVENTS: non-JSend responses leaking onto RFC 8522 routes, hardcoded timestamps
// or zero runtime, extra keys placed outside "data", or wrong HTTP status codes.
//
// RFC requirement: RFC8522-2.3-1 positive -- success, fail, client-error, router-error, timeout, and 401-unauthorized responses all emit strict JSend JSON with Content-Type application/json.
// RFC requirement: RFC8522-2.3-2 positive -- every RFC 8522 response carries a status field of success, fail, or error.
// RFC requirement: RFC8522-2.3.1-1 positive -- successful ping and show route responses set status to "success".
// RFC requirement: RFC8522-2.3.1-1 negative -- operational failure (0% ping, missing route) and error responses do not set status to "success".
// RFC requirement: RFC8522-2.3.1-2 positive -- successful responses carry a non-null data object with performed_at, runtime, router, format, and output.
// RFC requirement: RFC8522-2.3.1-3 positive -- data.performed_at is a UTC ISO 8601 timestamp falling within the operation's execution window.
// RFC requirement: RFC8522-2.3.1-4 positive -- data.runtime measures actual wallclock seconds used to execute the command (verified >= 0.01s on a 15ms command).
// RFC requirement: RFC8522-2.3.1-5 positive -- extra command fields (min, avg, max, rate, routers, id, name) are placed inside data and never at the top level.
// RFC requirement: RFC8522-2.3.1-6 positive -- successful and operational-fail responses return HTTP 200.
// RFC requirement: RFC8522-2.3.1-6 negative -- error responses (400 client error, 502 router error, 504 timeout) do not return HTTP 200.
// RFC requirement: RFC8522-2.3.2-1 positive -- 0% ping reachability and missing route return status "fail" with data carrying performed_at, runtime, output, and format.
// RFC requirement: RFC8522-2.3.2-1 negative -- reachable ping and found route return status "success", not "fail".
// RFC requirement: RFC8522-2.3.3-1 positive -- client errors, router failures, timeouts, and unauthorized calls return status "error" with meaningful cause-specific message text.
// RFC requirement: RFC8522-2.3.3-1 negative -- successful and operational-fail responses do not return status "error".
// RFC requirement: RFC8522-2.3.3-2 positive -- router failure and timeout error responses include optional data with valid performed_at and elapsed runtime.
// RFC requirement: RFC8522-2.3.3-3 positive -- client errors return HTTP 400, server JSON marshal failure returns HTTP 500, and target router dispatch failures return HTTP 502.
// RFC requirement: RFC8522-2.3.3-3 negative -- successful commands return HTTP 200 rather than 400, 500, or 502.
// RFC requirement: RFC8522-2.3.3-4 positive -- command execution timeout returns HTTP 504 with JSend error and timeout message.
// RFC requirement: RFC8522-2.3.3-4 negative -- non-timeout router dispatch failure returns HTTP 502 rather than HTTP 504.
func TestRFC8522JSendEnvelopeAndTiming(t *testing.T) {
	srv := newRFC8522TestServer(t, rfc8522MockDispatch(nil), 0)

	// 1. Success responses: ping (100% reachable), show route (found), routers, routers/0.
	beforeOK := time.Now()
	recOK := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/ping/192.0.2.1", true)
	afterOK := time.Now()
	if recOK.Code != http.StatusOK {
		t.Fatalf("success status = %d, want 200", recOK.Code)
	}
	bodyOK := decodeJSend(t, recOK)
	assertJSendKeys(t, bodyOK, jsendSuccess)
	dataOK := bodyOK["data"].(map[string]any)
	assertRequiredDataFields(t, dataOK, beforeOK, afterOK)
	if dataOK["router"] != testRFC8522Hostname {
		t.Fatalf("data.router = %v, want %q", dataOK["router"], testRFC8522Hostname)
	}
	if dataOK["format"] != rfc8522FormatText {
		t.Fatalf("data.format = %v, want %q", dataOK["format"], rfc8522FormatText)
	}
	if rfc8522OutputText(t, dataOK) == "" {
		t.Fatalf("success data.output empty: %v", dataOK)
	}
	for _, extraKey := range []string{"min", "avg", "max", "rate"} {
		if _, has := dataOK[extraKey]; !has {
			t.Fatalf("expected extra field %q inside data: %v", extraKey, dataOK)
		}
		if _, leaked := bodyOK[extraKey]; leaked {
			t.Fatalf("extra field %q leaked to top level", extraKey)
		}
	}

	// Success show route + routers + routers/0 extra-field placement inside data.
	recRouteOK := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/show/route/192.0.2.0/24", true)
	if recRouteOK.Code != http.StatusOK {
		t.Fatalf("show route success status = %d, want 200", recRouteOK.Code)
	}
	bodyRouteOK := decodeJSend(t, recRouteOK)
	assertJSendKeys(t, bodyRouteOK, jsendSuccess)
	if rfc8522OutputText(t, bodyRouteOK["data"].(map[string]any)) == "" {
		t.Fatalf("show route success missing data.output")
	}

	recRouters := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/routers", true)
	bodyRouters := decodeJSend(t, recRouters)
	assertJSendKeys(t, bodyRouters, jsendSuccess)
	if _, has := bodyRouters["data"].(map[string]any)["routers"]; !has {
		t.Fatalf("routers missing data.routers")
	}

	recDetails := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/routers/0", true)
	bodyDetails := decodeJSend(t, recDetails)
	assertJSendKeys(t, bodyDetails, jsendSuccess)
	detailsData := bodyDetails["data"].(map[string]any)
	if _, hasID := detailsData["id"]; !hasID {
		t.Fatalf("routers/0 missing data.id")
	}
	if _, hasName := detailsData["name"]; !hasName {
		t.Fatalf("routers/0 missing data.name")
	}

	// 2. Real elapsed wallclock runtime check (>= 10ms on a 15ms dispatch).
	baseMock := rfc8522MockDispatch(nil)
	slowSrv := newRFC8522TestServer(t, func(ctx context.Context, caller plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
		time.Sleep(15 * time.Millisecond)
		return baseMock(ctx, caller, cmd)
	}, 0)
	beforeSlow := time.Now()
	recSlow := rfc8522Request(t, slowSrv, http.MethodGet, "/.well-known/looking-glass/v1/ping/192.0.2.1", true)
	afterSlow := time.Now()
	if recSlow.Code != http.StatusOK {
		t.Fatalf("slow ping status = %d, want 200", recSlow.Code)
	}
	slowData := decodeJSend(t, recSlow)["data"].(map[string]any)
	assertRequiredDataFields(t, slowData, beforeSlow, afterSlow)
	if rt := slowData["runtime"].(float64); rt < 0.01 {
		t.Fatalf("slow ping data.runtime = %v, want >= 0.01s (elapsed wallclock)", rt)
	}

	// 3. Fail responses: 0% ping reachability AND missing route (HTTP 200, status "fail").
	beforeFail := time.Now()
	recFail := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/ping/192.0.2.5", true)
	afterFail := time.Now()
	if recFail.Code != http.StatusOK {
		t.Fatalf("fail ping status = %d, want 200", recFail.Code)
	}
	bodyFail := decodeJSend(t, recFail)
	assertJSendKeys(t, bodyFail, jsendFail)
	dataFail := bodyFail["data"].(map[string]any)
	assertRequiredDataFields(t, dataFail, beforeFail, afterFail)
	if dataFail["rate"] != float64(0) {
		t.Fatalf("fail data.rate = %v, want 0", dataFail["rate"])
	}
	if dataFail["format"] != rfc8522FormatText {
		t.Fatalf("fail data.format = %v, want %q", dataFail["format"], rfc8522FormatText)
	}
	if rfc8522OutputText(t, dataFail) == "" {
		t.Fatalf("fail ping data.output empty: %v", dataFail)
	}

	beforeMiss := time.Now()
	recMiss := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/show/route/198.51.100.0/24", true)
	afterMiss := time.Now()
	if recMiss.Code != http.StatusOK {
		t.Fatalf("missing route status = %d, want 200", recMiss.Code)
	}
	bodyMiss := decodeJSend(t, recMiss)
	assertJSendKeys(t, bodyMiss, jsendFail)
	dataMiss := bodyMiss["data"].(map[string]any)
	assertRequiredDataFields(t, dataMiss, beforeMiss, afterMiss)
	if dataMiss["format"] != rfc8522FormatText {
		t.Fatalf("missing route data.format = %v, want %q", dataMiss["format"], rfc8522FormatText)
	}
	if rfc8522OutputText(t, dataMiss) == "" {
		t.Fatalf("missing route data.output empty: %v", dataMiss)
	}

	// 4. Client error (HTTP 400) with meaningful cause message.
	recClient := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/ping/bad%20host!", true)
	if recClient.Code != http.StatusBadRequest {
		t.Fatalf("client error status = %d, want 400", recClient.Code)
	}
	bodyClient := decodeJSend(t, recClient)
	assertJSendKeys(t, bodyClient, jsendError)
	if msg := strings.ToLower(bodyClient["message"].(string)); !strings.Contains(msg, "host") {
		t.Fatalf("client error message = %q, want meaningful explanation naming host", bodyClient["message"])
	}

	// 5. Unauthorized error via auth.go middleware (HTTP 401 JSend error).
	authSrv, err := NewLGServer(LGConfig{
		ListenAddrs: []ListenAddr{{Addr: "127.0.0.1:0"}},
		Dispatch:    rfc8522MockDispatch(nil),
		Hostname:    testRFC8522Hostname,
		Token:       "rfc8522-secret-token",
	})
	if err != nil {
		t.Fatalf("NewLGServer with auth: %v", err)
	}
	recAuth := rfc8522Request(t, authSrv, http.MethodGet, "/.well-known/looking-glass/v1/routers", true)
	if recAuth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", recAuth.Code)
	}
	bodyAuth := decodeJSend(t, recAuth)
	assertJSendKeys(t, bodyAuth, jsendError)
	if !strings.Contains(strings.ToLower(bodyAuth["message"].(string)), "unauthorized") {
		t.Fatalf("401 message = %q, want unauthorized", bodyAuth["message"])
	}

	// 6. Server-side error (HTTP 500) on internal JSON encoding failure.
	recServerErr := httptest.NewRecorder()
	writeRFC8522JSON(recServerErr, http.StatusOK, map[string]any{"bad": make(chan int)})
	if recServerErr.Code != http.StatusInternalServerError {
		t.Fatalf("marshal error status = %d, want 500", recServerErr.Code)
	}
	bodyServerErr := decodeJSend(t, recServerErr)
	assertJSendKeys(t, bodyServerErr, jsendError)

	// 7. Router error (HTTP 502, not 504) + optional data timing object.
	failDispatch := func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return nil, errRFC8522TestEngineFailed
	}
	srvFail := newRFC8522TestServer(t, failDispatch, 0)
	beforeRouter := time.Now()
	recRouter := rfc8522Request(t, srvFail, http.MethodGet, "/.well-known/looking-glass/v1/ping/192.0.2.1", true)
	afterRouter := time.Now()
	if recRouter.Code != http.StatusBadGateway {
		t.Fatalf("router error status = %d, want 502", recRouter.Code)
	}
	bodyRouter := decodeJSend(t, recRouter)
	assertJSendKeys(t, bodyRouter, jsendError)
	if strings.Contains(bodyRouter["message"].(string), errRFC8522TestEngineFailed.Error()) {
		t.Fatalf("router error leaked raw backend error: %v", bodyRouter["message"])
	}
	if !strings.Contains(strings.ToLower(bodyRouter["message"].(string)), "engine error") {
		t.Fatalf("router error message = %q, want meaningful engine error explanation", bodyRouter["message"])
	}
	assertRequiredDataFields(t, bodyRouter["data"].(map[string]any), beforeRouter, afterRouter)

	// 8. Timeout error (HTTP 504) + optional data timing object with real elapsed runtime.
	timeoutDispatch := func(ctx context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	srvTimeout := newRFC8522TestServer(t, timeoutDispatch, 0)
	beforeTimeout := time.Now()
	recTimeout := rfc8522Request(t, srvTimeout, http.MethodGet, "/.well-known/looking-glass/v1/ping/192.0.2.1?runtime=0.02", true)
	afterTimeout := time.Now()
	if recTimeout.Code != http.StatusGatewayTimeout {
		t.Fatalf("timeout status = %d, want 504; body=%s", recTimeout.Code, recTimeout.Body.String())
	}
	bodyTimeout := decodeJSend(t, recTimeout)
	assertJSendKeys(t, bodyTimeout, jsendError)
	if !strings.Contains(strings.ToLower(bodyTimeout["message"].(string)), "timed out") {
		t.Fatalf("timeout message = %q, want timeout explanation", bodyTimeout["message"])
	}
	timeoutData := bodyTimeout["data"].(map[string]any)
	assertRequiredDataFields(t, timeoutData, beforeTimeout, afterTimeout)
	if rt := timeoutData["runtime"].(float64); rt < 0.015 {
		t.Fatalf("timeout data.runtime = %v, want >= 0.015s", rt)
	}
}

// TestRFC8522MandatoryCommands proves ping, show route, and all three
// organizational commands (routers, routers/{number}, cmd) plus optional BGP
// and traceroute commands, along with negative invalid-input cases.
//
// VALIDATES: ping (IPv4, IPv6 with protocol=2,1, DNS hostname, RTT stats and
// output, plus protocol-mismatch and invalid-host rejections), show route (CIDR
// & bare IP, found with route output & missing fail, plus invalid addr and
// protocol-mismatch rejections), routers, routers/0 (hostname and index, plus
// out-of-range/non-numeric/extra-path rejections), cmd (all entries carrying
// command, href, arguments, description, plus invalid subpath rejection), plus
// traceroute, show bgp, show bgp summary, and show bgp neighbors.
// PREVENTS: missing mandatory RFC 8522 functions, omitting required fields, or
// silently accepting invalid parameters.
//
// RFC requirement: RFC8522-3.1.1-1 positive -- GET /.well-known/looking-glass/v1/ping/{host} executes ping for IPv4, IPv6, and DNS hostname targets and returns JSend success with min, avg, max, rate, and output.
// RFC requirement: RFC8522-3.1.1-1 negative -- ping rejects protocol-family mismatch and invalid host characters with HTTP 400 JSend error.
// RFC requirement: RFC8522-3.2.1-1 positive -- GET /.well-known/looking-glass/v1/show/route/{addr} returns JSend success with route data.output for found IPv4/IPv6 CIDR and bare IP lookups, and JSend fail when absent.
// RFC requirement: RFC8522-3.2.1-1 negative -- show route rejects malformed addresses and protocol-family mismatches with HTTP 400 JSend error.
// RFC requirement: RFC8522-3.3-1 positive -- all three organizational commands (routers, routers/0, cmd with command/href/arguments/description entries) return HTTP 200 JSend success.
// RFC requirement: RFC8522-3.3-1 negative -- unknown organizational subpaths (/v1/routers/9, /v1/cmd/extra) are rejected with HTTP 400 JSend error.
// RFC requirement: RFC8522-3.3.2-1 positive -- GET /.well-known/looking-glass/v1/routers/0 returns the router hostname ("name") and router index ("id").
// RFC requirement: RFC8522-3.3.2-1 negative -- out-of-range (/v1/routers/1), non-numeric (/v1/routers/abc), and extra-segment (/v1/routers/0/extra) router-detail paths return HTTP 400 JSend error.
func TestRFC8522MandatoryCommands(t *testing.T) {
	srv := newRFC8522TestServer(t, rfc8522MockDispatch(nil), 0)

	// Ping positive: IPv4, IPv6 with protocol=2,1, and DNS hostname.
	pingCases := []struct {
		name    string
		target  string
		wantMin float64
		wantAvg float64
		wantMax float64
	}{
		{"ipv4", "/.well-known/looking-glass/v1/ping/192.0.2.1", 1.2, 1.5, 2.0},
		{"ipv6", "/.well-known/looking-glass/v1/ping/2001:DB8::35?protocol=2,1", 40, 41, 44},
		{"hostname", "/.well-known/looking-glass/v1/ping/example.com", 10.5, 11.0, 12.5},
	}
	for _, tc := range pingCases {
		before := time.Now()
		rec := rfc8522Request(t, srv, http.MethodGet, tc.target, true)
		after := time.Now()
		if rec.Code != http.StatusOK {
			t.Fatalf("ping %s status = %d, want 200; body=%s", tc.name, rec.Code, rec.Body.String())
		}
		body := decodeJSend(t, rec)
		assertJSendKeys(t, body, jsendSuccess)
		data := body["data"].(map[string]any)
		assertRequiredDataFields(t, data, before, after)
		if data["min"] != tc.wantMin || data["avg"] != tc.wantAvg || data["max"] != tc.wantMax {
			t.Fatalf("ping %s rtt = (%v,%v,%v), want (%v,%v,%v)", tc.name, data["min"], data["avg"], data["max"], tc.wantMin, tc.wantAvg, tc.wantMax)
		}
		if data["rate"] != float64(100) {
			t.Fatalf("ping %s rate = %v, want 100", tc.name, data["rate"])
		}
		if rfc8522OutputText(t, data) == "" {
			t.Fatalf("ping %s empty output: %v", tc.name, data)
		}
	}

	// Ping negative: protocol mismatch and invalid host refused with 400.
	for _, badPing := range []string{
		"/.well-known/looking-glass/v1/ping/192.0.2.1?protocol=2,1",
		"/.well-known/looking-glass/v1/ping/bad%20host!",
	} {
		rec := rfc8522Request(t, srv, http.MethodGet, badPing, true)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("GET %s status = %d, want 400", badPing, rec.Code)
		}
		assertJSendKeys(t, decodeJSend(t, rec), jsendError)
	}

	// show route positive: CIDR (IPv4 & IPv6) + bare IP lookup + missing route fail.
	beforeR4 := time.Now()
	recRoute4 := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/show/route/192.0.2.0/24", true)
	afterR4 := time.Now()
	if recRoute4.Code != http.StatusOK {
		t.Fatalf("show route v4 status = %d, want 200; body=%s", recRoute4.Code, recRoute4.Body.String())
	}
	bodyRoute4 := decodeJSend(t, recRoute4)
	assertJSendKeys(t, bodyRoute4, jsendSuccess)
	dataRoute4 := bodyRoute4["data"].(map[string]any)
	assertRequiredDataFields(t, dataRoute4, beforeR4, afterR4)
	if out := rfc8522OutputText(t, dataRoute4); !strings.Contains(out, "192.0.2.0/24") || !strings.Contains(out, "198.51.100.1") {
		t.Fatalf("show route v4 data.output = %q, want prefix and gateway", out)
	}

	recRoute6 := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/show/route/2001:DB8::/48?protocol=2,1", true)
	if recRoute6.Code != http.StatusOK {
		t.Fatalf("show route v6 status = %d, want 200; body=%s", recRoute6.Code, recRoute6.Body.String())
	}
	bodyRoute6 := decodeJSend(t, recRoute6)
	assertJSendKeys(t, bodyRoute6, jsendSuccess)
	if out := rfc8522OutputText(t, bodyRoute6["data"].(map[string]any)); !strings.Contains(out, "2001:db8::/48") {
		t.Fatalf("show route v6 data.output = %q, want 2001:db8::/48", out)
	}

	recLookup := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/show/route/192.0.2.1", true)
	if recLookup.Code != http.StatusOK {
		t.Fatalf("show route lookup status = %d, want 200; body=%s", recLookup.Code, recLookup.Body.String())
	}
	bodyLookup := decodeJSend(t, recLookup)
	assertJSendKeys(t, bodyLookup, jsendSuccess)
	if out := rfc8522OutputText(t, bodyLookup["data"].(map[string]any)); !strings.Contains(out, "192.0.2.0/24") {
		t.Fatalf("show route lookup data.output = %q, want 192.0.2.0/24", out)
	}

	recMissing := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/show/route/198.51.100.0/24", true)
	if recMissing.Code != http.StatusOK {
		t.Fatalf("missing route status = %d, want 200", recMissing.Code)
	}
	assertJSendKeys(t, decodeJSend(t, recMissing), jsendFail)

	recLookupMissing := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/show/route/198.51.100.9", true)
	if recLookupMissing.Code != http.StatusOK {
		t.Fatalf("missing route lookup status = %d, want 200", recLookupMissing.Code)
	}
	assertJSendKeys(t, decodeJSend(t, recLookupMissing), jsendFail)

	// show route negative: invalid address and protocol mismatch return HTTP 400.
	for _, badRoute := range []string{
		"/.well-known/looking-glass/v1/show/route/not-an-ip",
		"/.well-known/looking-glass/v1/show/route/192.0.2.0/24?protocol=2,1",
	} {
		rec := rfc8522Request(t, srv, http.MethodGet, badRoute, true)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("GET %s status = %d, want 400", badRoute, rec.Code)
		}
		assertJSendKeys(t, decodeJSend(t, rec), jsendError)
	}

	// Organizational positive: routers list.
	beforeRouters := time.Now()
	recRouters := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/routers", true)
	afterRouters := time.Now()
	if recRouters.Code != http.StatusOK {
		t.Fatalf("routers status = %d, want 200", recRouters.Code)
	}
	bodyRouters := decodeJSend(t, recRouters)
	assertJSendKeys(t, bodyRouters, jsendSuccess)
	routersData := bodyRouters["data"].(map[string]any)
	assertRequiredDataFields(t, routersData, beforeRouters, afterRouters)
	routersList, ok := routersData["routers"].([]any)
	if !ok || len(routersList) != 1 || routersList[0] != testRFC8522Hostname {
		t.Fatalf("routers = %v, want [%q]", routersData["routers"], testRFC8522Hostname)
	}

	// Organizational positive: router details (must include hostname and router index).
	beforeDet := time.Now()
	recDetails := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/routers/0", true)
	afterDet := time.Now()
	if recDetails.Code != http.StatusOK {
		t.Fatalf("routers/0 status = %d, want 200", recDetails.Code)
	}
	bodyDetails := decodeJSend(t, recDetails)
	assertJSendKeys(t, bodyDetails, jsendSuccess)
	detailsData := bodyDetails["data"].(map[string]any)
	assertRequiredDataFields(t, detailsData, beforeDet, afterDet)
	if detailsData["name"] != testRFC8522Hostname {
		t.Fatalf("routers/0 name = %v, want %q", detailsData["name"], testRFC8522Hostname)
	}
	if detailsData["id"] != float64(0) {
		t.Fatalf("routers/0 id = %v, want 0", detailsData["id"])
	}
	if detailsData["autonomous_system"] != float64(65000) {
		t.Fatalf("routers/0 autonomous_system = %v, want 65000", detailsData["autonomous_system"])
	}

	// Organizational negative: out-of-range, non-numeric, and extra router-detail segments.
	for _, badRouter := range []string{
		"/.well-known/looking-glass/v1/routers/1",
		"/.well-known/looking-glass/v1/routers/abc",
		"/.well-known/looking-glass/v1/routers/0/extra",
		"/.well-known/looking-glass/v1/cmd/extra",
	} {
		rec := rfc8522Request(t, srv, http.MethodGet, badRouter, true)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("GET %s status = %d, want 400", badRouter, rec.Code)
		}
		assertJSendKeys(t, decodeJSend(t, rec), jsendError)
	}

	// Organizational positive: cmd list with RFC 8522 Section 3.3.3 entry fields.
	beforeCmd := time.Now()
	recCmd := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/cmd", true)
	afterCmd := time.Now()
	if recCmd.Code != http.StatusOK {
		t.Fatalf("cmd status = %d, want 200", recCmd.Code)
	}
	bodyCmd := decodeJSend(t, recCmd)
	assertJSendKeys(t, bodyCmd, jsendSuccess)
	cmdData := bodyCmd["data"].(map[string]any)
	assertRequiredDataFields(t, cmdData, beforeCmd, afterCmd)
	cmdList, ok := cmdData["commands"].([]any)
	if !ok || len(cmdList) < 5 {
		t.Fatalf("cmd commands list too short: %v", cmdData["commands"])
	}
	foundPing := false
	foundShowRoute := false
	for _, entry := range cmdList {
		m, isMap := entry.(map[string]any)
		if !isMap {
			t.Fatalf("cmd entry is not an object: %v", entry)
		}
		for _, reqKey := range []string{"command", "href", "description"} {
			val, hasStr := m[reqKey].(string)
			if !hasStr || val == "" {
				t.Fatalf("cmd entry missing non-empty %q: %v", reqKey, m)
			}
		}
		if _, hasArgs := m["arguments"]; !hasArgs {
			t.Fatalf("cmd entry missing arguments field: %v", m)
		}
		if m["command"] == "ping" {
			foundPing = true
		}
		if m["command"] == "show route" {
			foundShowRoute = true
		}
	}
	if !foundPing || !foundShowRoute {
		t.Fatalf("cmd list missing mandatory ping (%v) or show route (%v)", foundPing, foundShowRoute)
	}

	// Optional commands: traceroute, show bgp, show bgp summary, show bgp neighbors, format=application/json.
	for _, target := range []string{
		"/.well-known/looking-glass/v1/traceroute/192.0.2.8",
		"/.well-known/looking-glass/v1/show/bgp/10.0.0.0/24",
		"/.well-known/looking-glass/v1/show/bgp/summary",
		"/.well-known/looking-glass/v1/show/bgp/neighbors/10.0.0.1",
		"/.well-known/looking-glass/v1/show/bgp/summary?format=application/yang,application/json",
	} {
		rec := rfc8522Request(t, srv, http.MethodGet, target, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200; body=%s", target, rec.Code, rec.Body.String())
		}
		assertJSendKeys(t, decodeJSend(t, rec), jsendSuccess)
	}

	// Missing BGP neighbor returns JSend fail (HTTP 200).
	recNoNbr := rfc8522Request(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/show/bgp/neighbors/192.0.2.99", true)
	if recNoNbr.Code != http.StatusOK {
		t.Fatalf("missing neighbor status = %d, want 200", recNoNbr.Code)
	}
	assertJSendKeys(t, decodeJSend(t, recNoNbr), jsendFail)
}

// TestRFC8522RequiresHTTPS proves over real TCP listeners that the RFC 8522 API
// is served over HTTPS and refused over plaintext HTTP.
//
// VALIDATES: a live TLS listener answers GET /.well-known/looking-glass/v1/routers
// with HTTP 200 JSend success, while a plaintext HTTP listener refuses the same
// path with HTTP 400 JSend error naming HTTPS.
// PREVENTS: serving RFC 8522 Looking Glass API calls in plaintext.
//
// RFC requirement: RFC8522-6-1 positive -- live HTTPS request to /.well-known/looking-glass/v1/routers succeeds with HTTP 200 and JSend success.
// RFC requirement: RFC8522-6-1 negative -- live plaintext HTTP request to /.well-known/looking-glass/v1/routers is rejected with HTTP 400 and JSend error requiring HTTPS.
func TestRFC8522RequiresHTTPS(t *testing.T) {
	// Positive: real TLS server answers RFC 8522 over HTTPS.
	certPEM, keyPEM := testChain(t, "rfc8522-lg")
	tlsSrv := startTLSTestServer(t, certPEM, keyPEM)
	tlsClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed test cert
		},
	}
	respHTTPS := doGet(t, tlsClient, "https://"+tlsSrv.Address()+"/.well-known/looking-glass/v1/routers")
	defer respHTTPS.Body.Close() //nolint:errcheck // test
	if respHTTPS.StatusCode != http.StatusOK {
		t.Fatalf("HTTPS RFC 8522 status = %d, want 200", respHTTPS.StatusCode)
	}
	var httpsBody map[string]any
	if err := json.NewDecoder(respHTTPS.Body).Decode(&httpsBody); err != nil {
		t.Fatalf("decode HTTPS body: %v", err)
	}
	assertJSendKeys(t, httpsBody, jsendSuccess)

	// Negative: plaintext HTTP server refuses RFC 8522.
	plainSrv, plainBase, plainClient := startTestServer(t)
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = plainSrv.Shutdown(shutCtx)
	}()
	respHTTP := doGet(t, plainClient, plainBase+"/.well-known/looking-glass/v1/routers")
	defer respHTTP.Body.Close() //nolint:errcheck // test
	if respHTTP.StatusCode != http.StatusBadRequest {
		t.Fatalf("plaintext HTTP RFC 8522 status = %d, want 400", respHTTP.StatusCode)
	}
	var httpBody map[string]any
	if err := json.NewDecoder(respHTTP.Body).Decode(&httpBody); err != nil {
		t.Fatalf("decode plaintext body: %v", err)
	}
	assertJSendKeys(t, httpBody, jsendError)
	if !strings.Contains(httpBody["message"].(string), "HTTPS") {
		t.Fatalf("plaintext refusal %q does not mention HTTPS", httpBody["message"])
	}
}

// TestRFC8522RateLimiting proves per-client-IP abuse mitigation on the RFC 8522 API.
//
// VALIDATES: requests within the per-minute limit succeed, the next request from
// the same client IP receives HTTP 429 with a JSend error envelope, and a distinct
// client IP remains unaffected by the first IP's throttle.
// PREVENTS: unbounded scripted flooding of looking-glass diagnostic endpoints or
// one abusive client starving other clients.
//
// RFC requirement: RFC8522-6.1-1 positive -- requests within a client IP's rate limit succeed with HTTP 200, and a distinct client IP succeeds even while another IP is throttled.
// RFC requirement: RFC8522-6.1-1 negative -- requests exceeding a client IP's per-minute rate limit are rejected with HTTP 429 and JSend error.
func TestRFC8522RateLimiting(t *testing.T) {
	srv := newRFC8522TestServer(t, rfc8522MockDispatch(nil), 2)

	for i := range 2 {
		rec := rfc8522RequestFrom(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/routers", "198.51.100.10:54321", true)
		if rec.Code != http.StatusOK {
			t.Fatalf("client A request %d status = %d, want 200", i+1, rec.Code)
		}
	}
	rec3 := rfc8522RequestFrom(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/routers", "198.51.100.10:54322", true)
	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("client A 3rd request status = %d, want 429", rec3.Code)
	}
	assertJSendKeys(t, decodeJSend(t, rec3), jsendError)

	// Per-IP isolation: a second client IP is not throttled by client A.
	recOther := rfc8522RequestFrom(t, srv, http.MethodGet, "/.well-known/looking-glass/v1/routers", "198.51.100.11:54321", true)
	if recOther.Code != http.StatusOK {
		t.Fatalf("client B 1st request status = %d, want 200 (per-IP isolation)", recOther.Code)
	}
	assertJSendKeys(t, decodeJSend(t, recOther), jsendSuccess)
}

// TestRFC8522CommandsAreDeclared proves every daemon command prefix
// handler_rfc8522.go dispatches is declared in Ze's YANG command tree.
//
// VALIDATES: show ping, show traceroute, show route, show route lookup, show bgp,
// and show bgp peer detail all exist in the compiled YANG command registry.
// PREVENTS: a renamed daemon command silently breaking RFC 8522 endpoints.
func TestRFC8522CommandsAreDeclared(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("yang loader: %v", err)
	}
	declared := make(map[string]bool)
	for _, paths := range configyang.WireMethodToPaths(loader) {
		for _, path := range paths {
			declared[path] = true
		}
	}
	want := []string{
		cmdShowPing,
		cmdShowTraceroute,
		cmdShowRoute,
		cmdShowRouteLookup,
		cmdBGPOverview,
		cmdShowBGPPeer + " detail",
	}
	for _, cmd := range want {
		if !declared[cmd] {
			t.Errorf("RFC 8522 handler dispatches %q, which the YANG command tree does not declare", cmd)
		}
	}
}
