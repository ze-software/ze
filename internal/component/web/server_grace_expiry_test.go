// Design: docs/architecture/web-interface.md -- network grace expiry and completed cleanup.
// Related: server_history_lifetime_test.go -- cancellation remains a network failure.
package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/selfcert"
)

// TestWebServerGraceExpiryCompletesJoinedCleanup holds an actual HTTP handler
// after cancellation. An expired network allowance must still join that handler
// and close its route exactly once, without mistaking completed forced closure
// for a network failure. A distinct owned-route failure remains a failure.
func TestWebServerGraceExpiryCompletesJoinedCleanup(t *testing.T) {
	routeFailure := errors.New("owned route cleanup failed after grace expiry")
	for _, test := range []struct {
		name       string
		routeError error
	}{
		{name: "complete cleanup"},
		{name: "route cleanup failure", routeError: routeFailure},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := newWebGraceServer(t)
			handler := &backupOwnedHandler{entered: make(chan struct{}), canceled: make(chan struct{}),
				release: make(chan struct{}), closeErr: test.routeError}
			server.Handle("GET /held", handler)
			server.OwnHandler(handler)
			hosted := httptest.NewUnstartedServer(server.server.Handler)
			hosted.Config = server.server
			hosted.Start()
			var release sync.Once
			releaseHandler := func() { release.Do(func() { close(handler.release) }) }
			t.Cleanup(func() {
				releaseHandler()
				server.requests.cancel()
				hosted.Close()
			})
			client := hosted.Client()
			clientDone := make(chan struct{})
			var clientError error
			go func() {
				defer close(clientDone)
				request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, hosted.URL+"/held", http.NoBody)
				if err != nil {
					clientError = err
					return
				}
				response, err := client.Do(request)
				clientError = err
				if response != nil {
					clientError = errors.Join(clientError, response.Body.Close())
				}
			}()
			t.Cleanup(func() {
				releaseHandler()
				server.requests.cancel()
				hosted.CloseClientConnections()
				<-clientDone
			})
			<-handler.entered
			// The deadline is already expired so no sleep or elapsed-time claim
			// decides the outcome. The live handler forces net/http to return
			// this owned grace expiration before its successful Close fallback.
			ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
			defer cancel()
			require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
			stopped := make(chan error, 1)
			go func() { stopped <- server.Shutdown(ctx) }()
			<-handler.canceled
			require.Zero(t, handler.closed.Load(), "owned-route cleanup must wait for the admitted handler")
			select {
			case err := <-stopped:
				t.Fatalf("shutdown returned before the admitted handler finished: %v", err)
			default:
			}
			releaseHandler()
			err := <-stopped
			if test.routeError == nil {
				require.NoError(t, err, "a completed close and join fulfill the expired network grace")
			} else {
				require.ErrorIs(t, err, test.routeError)
			}
			require.NoError(t, ShutdownNetworkError(err), "a successful forced close is not a transport failure")
			require.Equal(t, int32(1), handler.closed.Load())
			require.True(t, handler.finished.Load())
			<-clientDone
			require.Error(t, clientError, "the live HTTP connection must actually be closed")
			require.Equal(t, err, server.Shutdown(context.Background()), "repeated callers join the same outcome")
			require.Equal(t, int32(1), handler.closed.Load(), "repeated shutdown must not repeat route cleanup")
			late := httptest.NewRecorder()
			server.server.Handler.ServeHTTP(late, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/held", http.NoBody))
			require.Equal(t, http.StatusServiceUnavailable, late.Code)
		})
	}
}

type webGraceListenerFailure struct {
	net.Listener
	failure error
}

func (listener *webGraceListenerFailure) Close() error {
	if err := listener.Listener.Close(); err != nil {
		return errors.Join(listener.failure, err)
	}
	return listener.failure
}

// TestWebServerGraceExpiryKeepsTransportFailures injects listener-close causes
// while the caller context has not expired. A deadline-shaped transport error
// is still a failure, as is an unrelated close failure; neither is the owned
// grace expiration that permits a completed forced close.
func TestWebServerGraceExpiryKeepsTransportFailures(t *testing.T) {
	for _, test := range []struct {
		name    string
		failure error
	}{
		{name: "transport deadline before grace expiry", failure: context.DeadlineExceeded},
		{name: "transport close failure", failure: errors.New("listener close failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := newWebGraceServer(t)
			server.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			hosted := httptest.NewUnstartedServer(server.server.Handler)
			hosted.Config = server.server
			hosted.Listener = &webGraceListenerFailure{Listener: hosted.Listener, failure: test.failure}
			hosted.Start()
			t.Cleanup(hosted.Close)
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, hosted.URL+"/ready", http.NoBody)
			require.NoError(t, err)
			response, err := hosted.Client().Do(request)
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, response.StatusCode)
			require.NoError(t, response.Body.Close())
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			err = server.Shutdown(ctx)
			require.NoError(t, ctx.Err(), "the injected failure must precede any owned grace expiry")
			require.ErrorIs(t, err, test.failure)
			require.ErrorIs(t, ShutdownNetworkError(err), test.failure)
			require.ErrorIs(t, server.Shutdown(context.Background()), test.failure)
		})
	}
}

func newWebGraceServer(t *testing.T) *WebServer {
	t.Helper()
	cert, key, err := selfcert.GenerateWebCertWithAddr("")
	require.NoError(t, err)
	server, err := NewWebServer(WebConfig{ListenAddrs: []string{"127.0.0.1:0"}, CertPEM: cert, KeyPEM: key})
	require.NoError(t, err)
	return server
}
