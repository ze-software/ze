package rest

import (
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
)

// execBody is the smallest request the execute route accepts.
const execBody = `{"command":"show bgp"}`

func authTestServer(t *testing.T, cfg RESTConfig) *RESTServer {
	t.Helper()
	engine := testEngine()
	openAPI, _ := api.OpenAPISchema(nil)
	if len(cfg.ListenAddrs) == 0 {
		cfg.ListenAddrs = []string{"127.0.0.1:0"}
	}
	srv, err := NewRESTServer(cfg, engine, nil, func() []byte { return openAPI })
	require.NoError(t, err)
	return srv
}

func executeWithToken(t *testing.T, srv *RESTServer, token string) int {
	t.Helper()
	headers := map[string]string{"Content-Type": "application/json"}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return doWithHeader(t, srv, "POST", "/api/v1/execute", execBody, headers).Status
}

func TestRESTAuthenticationProviderPublishesModesAtomically(t *testing.T) {
	var live atomic.Pointer[api.Authentication]
	publish := func(authentication api.Authentication) { live.Store(&authentication) }
	provider := func() api.Authentication { return *live.Load() }
	publish(testAuthentication("old", nil, nil)())
	srv := authTestServer(t, RESTConfig{Authentication: provider})

	assert.True(t, srv.Authenticated())
	assert.Equal(t, http.StatusOK, executeWithToken(t, srv, "old"))
	assert.Equal(t, http.StatusUnauthorized, executeWithToken(t, srv, "new"))

	publish(api.Authentication{Required: true})
	assert.True(t, srv.Authenticated(), "staging is gated for exposure checks")
	assert.Equal(t, http.StatusUnauthorized, executeWithToken(t, srv, "old"))
	assert.Equal(t, http.StatusUnauthorized, executeWithToken(t, srv, "new"))

	publish(testAuthentication("new", nil, nil)())
	assert.Equal(t, http.StatusUnauthorized, executeWithToken(t, srv, "old"))
	assert.Equal(t, http.StatusOK, executeWithToken(t, srv, "new"))

	publish(api.Authentication{})
	assert.False(t, srv.Authenticated())
	assert.Equal(t, http.StatusOK, executeWithToken(t, srv, ""))
}

func TestRESTRejectedCandidateNeverAuthenticates(t *testing.T) {
	var live atomic.Pointer[api.Authentication]
	accepted := testAuthentication("accepted", nil, nil)()
	live.Store(&accepted)
	srv := authTestServer(t, RESTConfig{Authentication: func() api.Authentication {
		return *live.Load()
	}})

	staging := api.Authentication{Required: true}
	live.Store(&staging)
	assert.Equal(t, http.StatusUnauthorized, executeWithToken(t, srv, "candidate"))

	live.Store(&accepted)
	assert.Equal(t, http.StatusUnauthorized, executeWithToken(t, srv, "candidate"))
	assert.Equal(t, http.StatusOK, executeWithToken(t, srv, "accepted"))
}
