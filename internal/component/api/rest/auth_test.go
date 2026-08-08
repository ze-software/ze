package rest

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/api"
)

// execBody is the smallest request the execute route accepts.
const execBody = `{"command":"show bgp summary"}`

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

// VALIDATES: AC-1 -- turning authentication ON in a reloaded config makes the
// RUNNING REST server demand it, with no restart and no rebind.
// PREVENTS: a reload that reports success while the listener keeps serving
// every request unauthenticated, which is what the daemon did while
// authentication was fixed at construction.
func TestRESTUpdateAuthTurnsAuthenticationOn(t *testing.T) {
	srv := authTestServer(t, RESTConfig{})

	// Built without credentials: every request is served.
	assert.False(t, srv.Authenticated())
	assert.Equal(t, http.StatusOK, executeWithToken(t, srv, ""))

	restore, err := srv.UpdateAuth("secret", nil)
	require.NoError(t, err)
	require.NotNil(t, restore)

	assert.True(t, srv.Authenticated())
	assert.Equal(t, http.StatusUnauthorized, executeWithToken(t, srv, ""), "unauthenticated request must be refused after the reload")
	assert.Equal(t, http.StatusUnauthorized, executeWithToken(t, srv, "wrong"))
	assert.Equal(t, http.StatusOK, executeWithToken(t, srv, "secret"))
}

// VALIDATES: AC-1 (other direction) -- turning authentication OFF in a reloaded
// config also takes effect on the running server.
// PREVENTS: an asymmetric implementation that only ever adds credentials, so
// the guard's view of the server drifts from what it serves.
func TestRESTUpdateAuthTurnsAuthenticationOff(t *testing.T) {
	srv := authTestServer(t, RESTConfig{Token: "secret"})

	assert.True(t, srv.Authenticated())
	assert.Equal(t, http.StatusUnauthorized, executeWithToken(t, srv, ""))

	restore, err := srv.UpdateAuth("", nil)
	require.NoError(t, err)
	require.NotNil(t, restore)

	assert.False(t, srv.Authenticated())
	assert.Equal(t, http.StatusOK, executeWithToken(t, srv, ""), "the reloaded config no longer asks for credentials")
}

// VALIDATES: AC-2 -- the restore function UpdateAuth returns puts the previous
// credentials back, so a reload that fails after the rebuild leaves the server
// exactly as authenticated as it was.
// PREVENTS: a half-applied reload dropping authentication permanently.
func TestRESTUpdateAuthRestoreRevertsCredentials(t *testing.T) {
	srv := authTestServer(t, RESTConfig{Token: "original"})

	restore, err := srv.UpdateAuth("", nil)
	require.NoError(t, err)
	assert.False(t, srv.Authenticated())
	assert.Equal(t, http.StatusOK, executeWithToken(t, srv, ""))

	restore()

	assert.True(t, srv.Authenticated())
	assert.Equal(t, http.StatusUnauthorized, executeWithToken(t, srv, ""))
	assert.Equal(t, http.StatusOK, executeWithToken(t, srv, "original"))
}

// VALIDATES: a per-user authenticator installed by a reload gates requests, and
// takes precedence over a token exactly as it does at construction.
// PREVENTS: UpdateAuth writing only the token field, leaving per-user
// credentials from the reloaded config unenforced.
func TestRESTUpdateAuthInstallsPerUserAuthenticator(t *testing.T) {
	srv := authTestServer(t, RESTConfig{})

	_, err := srv.UpdateAuth("ignored", func(header string) (string, bool) {
		return "alice", header == "Bearer alice:pw"
	})
	require.NoError(t, err)

	assert.True(t, srv.Authenticated())
	assert.Equal(t, http.StatusUnauthorized, executeWithToken(t, srv, "ignored"), "the token must not be checked while an authenticator is set")
	assert.Equal(t, http.StatusOK, executeWithToken(t, srv, "alice:pw"))
}

// VALIDATES: UpdateAuth fails closed on a server that has been shut down.
// PREVENTS: a reload reporting that it rebuilt authentication on a server that
// is no longer serving, so the migrator believes a state nothing holds.
func TestRESTUpdateAuthRefusedAfterShutdown(t *testing.T) {
	srv := authTestServer(t, RESTConfig{Token: "secret"})
	require.NoError(t, srv.Shutdown(t.Context()))

	restore, err := srv.UpdateAuth("", nil)
	require.Error(t, err)
	assert.Nil(t, restore)
	assert.True(t, srv.Authenticated(), "a refused update must leave the credentials untouched")
}
