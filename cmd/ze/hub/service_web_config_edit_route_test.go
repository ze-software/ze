//go:build ze_web

// Design: docs/architecture/web-interface.md -- URL Scheme and Web modes.
// Related: service_web.go -- production catch-all routing for /config/edit/.

package hub

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestWebConfigEditRoutePreservesUIModeAndCandidate exercises startWebServer's
// actual HTTPS route table for /config/edit/ URLs emitted by commit/delete/CLI
// redirects.
//
// VALIDATES: /config/edit/... routes through showHandler so Finder and Workbench
// modes, authentication, and per-user EditorManager candidate readback are
// preserved, and /config/edit (no trailing slash) resolves to root without an
// invalid-path redirect.
// PREVENTS: /config/edit/... falling through unconditionally to fragmentHandler
// (forcing Workbench users back into Finder) or being parsed as YANG path
// config/edit.
// MUTATION: omit /config/edit/ from web.IsViewRoutePath; Workbench requests
// render Finder (missing data-ui-mode="workbench") instead of Workbench.
func TestWebConfigEditRoutePreservesUIModeAndCandidate(t *testing.T) {
	resetAAABundleForTest(t)
	setAPIConfigDir(t, t.TempDir())
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile("config.conf", []byte("system { host committed-edit-host; }\n"), 0o600))

	users := []authz.UserConfig{{Name: "edit-operator", Hash: bcryptHash(t, "synthetic-edit-password")}}
	liveUsers := func() ([]authz.UserConfig, error) { return users, nil }
	server, broker, manager := startWebServer(store, "config.conf", []string{"127.0.0.1:0"}, false, "",
		nil, nil, nil, nil, nil, users, liveUsers, nil, nil, nil)
	require.NotNil(t, server)
	service := newWebService(server, broker, manager)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		require.NoError(t, service.Shutdown(ctx))
	})
	ready, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, server.WaitReady(ready))

	require.NoError(t, manager.SetValue("edit-operator", []string{"system"}, "host", "draft-edit-host"))

	client := &http.Client{
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // Per-test self-signed listener.
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	t.Cleanup(client.CloseIdleConnections)
	base := "https://" + server.Address()

	t.Run("unauthenticated is refused", func(t *testing.T) {
		req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/config/edit/system/", http.NoBody)
		require.NoError(t, reqErr)
		resp, respErr := client.Do(req)
		require.NoError(t, respErr)
		defer func() { require.NoError(t, resp.Body.Close()) }()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	modes := []struct {
		name       string
		cookie     string
		wantMarker string
		forbidMark string
	}{
		{
			name:       "finder",
			cookie:     "finder",
			wantMarker: `id="content-area"`,
			forbidMark: `data-ui-mode="workbench"`,
		},
		{
			name:       "workbench",
			cookie:     "workbench",
			wantMarker: `data-ui-mode="workbench"`,
			forbidMark: `id="content-area"`,
		},
	}

	for _, mode := range modes {
		for _, path := range []string{"/config/edit/system/", "/show/system/"} {
			t.Run(mode.name+" "+path, func(t *testing.T) {
				req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodGet, base+path, http.NoBody)
				require.NoError(t, reqErr)
				req.SetBasicAuth("edit-operator", "synthetic-edit-password")
				req.AddCookie(&http.Cookie{Name: "ze-ui-mode", Value: mode.cookie})
				resp, respErr := client.Do(req)
				require.NoError(t, respErr)
				defer func() { require.NoError(t, resp.Body.Close()) }()
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				require.NoError(t, readErr)
				html := string(body)
				require.Equal(t, http.StatusOK, resp.StatusCode, html)
				assert.Contains(t, html, mode.wantMarker)
				assert.NotContains(t, html, mode.forbidMark)
				assert.Contains(t, html, "draft-edit-host", "must render authenticated user's candidate")
				assert.NotContains(t, html, "committed-edit-host", "must not fall back to committed startup tree")
			})
		}
		for _, rootPath := range []string{"/config/edit/", "/config/edit"} {
			t.Run(mode.name+" root "+rootPath, func(t *testing.T) {
				req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodGet, base+rootPath, http.NoBody)
				require.NoError(t, reqErr)
				req.SetBasicAuth("edit-operator", "synthetic-edit-password")
				req.AddCookie(&http.Cookie{Name: "ze-ui-mode", Value: mode.cookie})
				resp, respErr := client.Do(req)
				require.NoError(t, respErr)
				defer func() { require.NoError(t, resp.Body.Close()) }()
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				require.NoError(t, readErr)
				html := string(body)
				require.Equal(t, http.StatusOK, resp.StatusCode, html)
				assert.Contains(t, html, mode.wantMarker)
				assert.NotContains(t, html, mode.forbidMark)
			})
		}
	}
}
