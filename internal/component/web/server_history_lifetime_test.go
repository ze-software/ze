// Design: docs/architecture/web-interface.md -- active POST streams join before cleanup.
package web

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/selfcert"
)

// TestWebServerNetworkDeadlineJoinsLivePostProducer follows an actual HTTPS
// stream through network grace expiry and requires its producer to finish.
func TestWebServerNetworkDeadlineJoinsLivePostProducer(t *testing.T) {
	cert, key, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)
	server, err := NewWebServer(WebConfig{ListenAddrs: []string{"127.0.0.1:0"}, CertPEM: cert, KeyPEM: key})
	require.NoError(t, err)
	finished := make(chan struct{})
	server.HandleFunc("POST /cli/stream", func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		if _, err := io.WriteString(w, "started\n"); err != nil {
			return
		}
		if err := http.NewResponseController(w).Flush(); err != nil {
			return
		}
		<-r.Context().Done()
	})
	served := make(chan error, 1)
	go func() { served <- server.ListenAndServe(context.Background()) }()
	t.Cleanup(func() {
		if err := server.Shutdown(context.Background()); err != nil {
			require.ErrorIs(t, err, context.Canceled)
		}
	})
	ready, cancelReady := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelReady()
	require.NoError(t, server.WaitReady(ready))
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // Per-test self-signed server.
	t.Cleanup(client.CloseIdleConnections)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://"+server.Address()+"/cli/stream", strings.NewReader("command=monitor"))
	require.NoError(t, err)
	response, err := client.Do(request)
	require.NoError(t, err)
	defer response.Body.Close() //nolint:errcheck // Shutdown force-closes this test stream.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, server.Shutdown(ctx), context.Canceled)
	select {
	case <-finished:
	default:
		t.Fatal("network shutdown returned before the actual POST producer finished")
	}
	require.NoError(t, <-served)
	late := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(late, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/cli/stream", http.NoBody))
	require.Equal(t, http.StatusServiceUnavailable, late.Code)
}
