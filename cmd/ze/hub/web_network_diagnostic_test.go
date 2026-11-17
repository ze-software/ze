//go:build ze_web

// Design: docs/architecture/doctor-and-health-checks.md -- network cleanup failures.
// Related: service_web.go -- reports cleanup before releasing borrowed history.
package hub

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	zeweb "github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/selfcert"
)

// TestWebNetworkCleanupFailureReportsDoctorEvent compares a clean unused
// service with an actual live request whose network grace is canceled.
func TestWebNetworkCleanupFailureReportsDoctorEvent(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	cert, key, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)
	newServer := func() *zeweb.WebServer {
		server, err := zeweb.NewWebServer(zeweb.WebConfig{ListenAddrs: []string{"127.0.0.1:0"}, CertPEM: cert, KeyPEM: key})
		require.NoError(t, err)
		return server
	}
	require.NoError(t, newWebService(newServer(), nil, nil).Shutdown(context.Background()))
	require.Empty(t, report.Errors(0), "successful cleanup must not raise a problem")
	server := newServer()
	finished := make(chan struct{})
	server.HandleFunc("GET /held", func(w http.ResponseWriter, r *http.Request) {
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
	service := newWebService(server, nil, nil)
	t.Cleanup(func() { require.ErrorIs(t, service.Shutdown(context.Background()), context.Canceled) })
	ready, cancelReady := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelReady()
	require.NoError(t, server.WaitReady(ready))
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // Per-test self-signed server.
	t.Cleanup(client.CloseIdleConnections)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+server.Address()+"/held", http.NoBody)
	require.NoError(t, err)
	response, err := client.Do(request)
	require.NoError(t, err)
	defer response.Body.Close() //nolint:errcheck // Network cleanup closes this held response.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, service.Shutdown(ctx), context.Canceled)
	<-finished
	require.NoError(t, <-served)
	require.ErrorIs(t, service.Shutdown(context.Background()), context.Canceled)
	events := report.Errors(0)
	require.Len(t, events, 1, "repeated joins must retain one event for the actual network failure")
	event := events[0]
	require.Equal(t, "web-server", event.Source)
	require.Equal(t, "web-network-cleanup", event.Subject)
	require.Equal(t, "doctor-web-network-cleanup", event.Code)
	require.Contains(t, event.Message, "network cleanup")
	require.Equal(t, "ze show log recent level err component web.server", event.Fix)
	require.Empty(t, event.Detail)
	diagnostic.RegisterBuiltinCodes()
	require.NotNil(t, diagnostic.Lookup(event.Code))
	require.Contains(t, diagnostic.EventCodes(), event.Code)
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name != "recent-events" {
			continue
		}
		require.Contains(t, check.Codes, event.Code)
		rows := check.Check(diagnostic.DoctorCheckContext{})
		require.Len(t, rows, 1)
		require.Equal(t, event.Code, rows[0].Code)
		require.Equal(t, event.Fix, rows[0].Fix)
		return
	}
	t.Fatal("the daemon recent-events doctor producer was not registered")
}
