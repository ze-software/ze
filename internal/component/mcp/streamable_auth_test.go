package mcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/loginservice"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/audit"
)

func postMCPToolCall(t *testing.T, handler http.Handler, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	body, params := buildMCPBody(t, 1, methodToolsCall, capsNone, `{"name":"ze_execute","arguments":{"command":"show version"}}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, Endpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", mediaTypeJSON)
	for k, v := range mcpHeaders(methodToolsCall, params) {
		req.Header.Set(k, v)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// VALIDATES: AC-20 -- MCP Streamable HTTP enforces LoginServiceGate under "mcp"
// for named bearer-list and oauth principals on every POST (returning the exact
// HTTP 401 Bearer error="invalid_token" response, recording audit.ActionAuthFail,
// and running zero tools on refusal, then admitting immediately when the gate
// flips to allow on reload), while anonymous none and single-bearer modes skip
// LoginServiceGate.
// PREVENTS: named MCP principals bypassing login-service restrictions, or
// anonymous MCP modes being broken by the login-service gate.
func TestMCPStreamableLoginServiceGate(t *testing.T) {
	assert.Equal(t, "mcp", loginServiceMCP)
	assert.True(t, loginservice.Registered(loginServiceMCP), "mcp must be registered in loginservice at init")
	assert.True(t, slices.Contains(loginservice.Names(), "mcp"), "loginservice.Names must include mcp")

	errDenied := errors.New("login service denied")

	t.Run("bearer-list named principal gated per POST with live reload", func(t *testing.T) {
		var allow atomic.Bool
		var dispatchCalls atomic.Int32
		var gotUser, gotService string

		recorder, err := audit.NewMemory(100)
		require.NoError(t, err)

		srv, err := NewStreamable(StreamableConfig{
			AuthMode: AuthBearerList,
			BearerList: []BearerListEntry{
				{Name: "alice", Token: "alice-secret-token"},
			},
			AuditRecorder: recorder,
			Dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
				dispatchCalls.Add(1)
				return plugin.NewResponse(plugin.StatusDone, plugin.Map{"result": "ok"}), nil
			},
			LoginServiceGate: func(username, service string) error {
				gotUser = username
				gotService = service
				if !allow.Load() {
					return errDenied
				}
				return nil
			},
		})
		require.NoError(t, err)
		defer srv.Close()

		// Capture wrong-token response to verify AC-2/AC-20 indistinguishability.
		wrongRec := postMCPToolCall(t, srv, "wrong-token")
		require.Equal(t, http.StatusUnauthorized, wrongRec.Code)

		// Named principal refused by LoginServiceGate.
		refusedRec := postMCPToolCall(t, srv, "alice-secret-token")
		assert.Equal(t, http.StatusUnauthorized, refusedRec.Code)
		assert.Equal(t, wrongRec.Header().Get("WWW-Authenticate"), refusedRec.Header().Get("WWW-Authenticate"),
			"AC-20: login-service refusal must emit the exact same WWW-Authenticate challenge as a bad token")
		assert.Contains(t, refusedRec.Header().Get("WWW-Authenticate"), `Bearer `)
		assert.Contains(t, refusedRec.Header().Get("WWW-Authenticate"), `error="invalid_token"`)
		assert.Equal(t, wrongRec.Body.String(), refusedRec.Body.String(),
			"AC-2: login-service refusal body must match a bad-token response")
		assert.Equal(t, "alice", gotUser)
		assert.Equal(t, loginServiceMCP, gotService)
		assert.Equal(t, int32(0), dispatchCalls.Load(), "AC-20: refused POST must execute zero tools")

		entries := recorder.Query(audit.Filter{Action: audit.ActionAuthFail})
		require.Len(t, entries, 2, "wrong token and login-service refusal must each record one audit.ActionAuthFail")
		assert.Equal(t, audit.MCP, entries[1].Surface)
		assert.Equal(t, audit.OutcomeDenied, entries[1].Outcome)

		// Live reload allows alice on mcp without restarting Streamable.
		allow.Store(true)
		admittedRec := postMCPToolCall(t, srv, "alice-secret-token")
		assert.Equal(t, http.StatusOK, admittedRec.Code)
		assert.Equal(t, int32(1), dispatchCalls.Load(), "admitted POST must execute the tool")
	})

	t.Run("oauth named principal gated per POST with live reload", func(t *testing.T) {
		as := newTestAS(t)
		var allow atomic.Bool
		var dispatchCalls atomic.Int32
		var gotUser, gotService string

		recorder, err := audit.NewMemory(100)
		require.NoError(t, err)

		srv, err := NewStreamable(StreamableConfig{
			AuthMode: AuthOAuth,
			OAuth: OAuthConfig{
				AuthorizationServer: as.Issuer(),
				Audience:            "https://mcp.example/",
			},
			AuditRecorder: recorder,
			Dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
				dispatchCalls.Add(1)
				return plugin.NewResponse(plugin.StatusDone, plugin.Map{"result": "ok"}), nil
			},
			LoginServiceGate: func(username, service string) error {
				gotUser = username
				gotService = service
				if !allow.Load() {
					return errDenied
				}
				return nil
			},
		})
		require.NoError(t, err)
		defer srv.Close()

		token := as.MintToken(t, map[string]any{"sub": "bob"})

		refusedRec := postMCPToolCall(t, srv, token)
		assert.Equal(t, http.StatusUnauthorized, refusedRec.Code)
		challenge := refusedRec.Header().Get("WWW-Authenticate")
		assert.Contains(t, challenge, `Bearer `)
		assert.Contains(t, challenge, `error="invalid_token"`)
		assert.Contains(t, challenge, `resource_metadata=`)
		assert.Equal(t, "bob", gotUser)
		assert.Equal(t, loginServiceMCP, gotService)
		assert.Equal(t, int32(0), dispatchCalls.Load(), "AC-20: refused OAuth POST must execute zero tools")

		entries := recorder.Query(audit.Filter{Action: audit.ActionAuthFail})
		require.Len(t, entries, 1, "AC-20: OAuth login-service refusal must record one audit.ActionAuthFail")
		assert.Equal(t, audit.MCP, entries[0].Surface)
		assert.Equal(t, audit.OutcomeDenied, entries[0].Outcome)

		// Live reload allows bob on mcp.
		allow.Store(true)
		admittedRec := postMCPToolCall(t, srv, token)
		assert.Equal(t, http.StatusOK, admittedRec.Code)
		assert.Equal(t, int32(1), dispatchCalls.Load(), "admitted OAuth POST must execute the tool")
	})

	t.Run("anonymous auth-mode none skips login-service gate", func(t *testing.T) {
		var gateCalls atomic.Int32
		var dispatchCalls atomic.Int32

		srv, err := NewStreamable(StreamableConfig{
			AuthMode: AuthNone,
			Dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
				dispatchCalls.Add(1)
				return plugin.NewResponse(plugin.StatusDone, plugin.Map{"result": "ok"}), nil
			},
			LoginServiceGate: func(_, _ string) error {
				gateCalls.Add(1)
				return errDenied
			},
		})
		require.NoError(t, err)
		defer srv.Close()

		rec := postMCPToolCall(t, srv, "")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, int32(0), gateCalls.Load(), "AC-20: auth-mode none must never call LoginServiceGate")
		assert.Equal(t, int32(1), dispatchCalls.Load())
	})

	t.Run("anonymous single shared bearer skips login-service gate", func(t *testing.T) {
		var gateCalls atomic.Int32
		var dispatchCalls atomic.Int32

		srv, err := NewStreamable(StreamableConfig{
			AuthMode: AuthBearer,
			Token:    "shared-bearer-token",
			Dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
				dispatchCalls.Add(1)
				return plugin.NewResponse(plugin.StatusDone, plugin.Map{"result": "ok"}), nil
			},
			LoginServiceGate: func(_, _ string) error {
				gateCalls.Add(1)
				return errDenied
			},
		})
		require.NoError(t, err)
		defer srv.Close()

		rec := postMCPToolCall(t, srv, "shared-bearer-token")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, int32(0), gateCalls.Load(), "AC-20: single shared bearer mode must never call LoginServiceGate")
		assert.Equal(t, int32(1), dispatchCalls.Load())
	})
}
