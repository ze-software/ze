//go:build ze_web

// Design: docs/architecture/web-interface.md -- authenticated browser monitor actions.

package hub

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestWebCLIViewActionRouteRefusesUnknownHandle reaches startWebServer's actual
// HTTPS route graph. An invented handle must reach the action refusal, never a
// missing route or a new monitor. The paired forged request must stop at origin.
// MUTATION: omit POST /cli/view/action registration; this answers 404, not 410.
func TestWebCLIViewActionRouteRefusesUnknownHandle(t *testing.T) {
	resetAAABundleForTest(t)
	setAPIConfigDir(t, t.TempDir())
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	users := []authz.UserConfig{{Name: "view-operator", Hash: bcryptHash(t, "synthetic-view-password")}}
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
	client := &http.Client{
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // Per-test self-signed listener.
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	t.Cleanup(client.CloseIdleConnections)
	base := "https://" + server.Address()
	for _, test := range []struct {
		name   string
		origin string
		status int
	}{
		{name: "unknown handle", origin: base, status: http.StatusGone},
		{name: "forged origin", origin: "https://untrusted.example", status: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, requestErr := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/cli/view/action",
				strings.NewReader(url.Values{"handle": {"unissued-handle"}, "action": {"stop"}}.Encode()))
			require.NoError(t, requestErr)
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Set("Origin", test.origin)
			request.SetBasicAuth("view-operator", "synthetic-view-password")
			response, responseErr := client.Do(request)
			require.NoError(t, responseErr)
			defer func() { require.NoError(t, response.Body.Close()) }()
			body, readErr := io.ReadAll(io.LimitReader(response.Body, 8192))
			require.NoError(t, readErr)
			assert.Equal(t, test.status, response.StatusCode, string(body))
			assert.NotContains(t, string(body), `"controls"`)
		})
	}
}
