// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- restart visibility.
// Related: register.go -- telemetry hook; connected_session.go -- counted entry.

package pppoeclient

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

// TestConnectedRenegotiationMetric scrapes the exporter after real network-phase
// restarts for two client names. The registry arrives after init registration.
func TestConnectedRenegotiationMetric(t *testing.T) {
	previousMetrics := clientMetricsPtr.Load()
	t.Cleanup(func() { clientMetricsPtr.Store(previousMetrics) })
	previous := registry.GetMetricsRegistry()
	t.Cleanup(func() { registry.SetMetricsRegistry(previous) })
	reg := metrics.NewPrometheusRegistry()
	registry.SetMetricsRegistry(reg)
	for _, name := range []string{"wan-a", "wan-b", "wan-a"} {
		synctest.Test(t, func(t *testing.T) {
			w := newConnectedExchange()
			s, initial := w.session(t)
			s.cfg.name = name
			stop, done := make(chan struct{}), make(chan struct{})
			defer close(stop)
			go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
			w.openLCP(t, 0, initial.magic)
			w.openIPCP(t, initial.ipcpIdentifier, 0)
		})
	}
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		w.modeFailure = ppp.NPModeDrop
		s, _ := w.session(t)
		s.cfg.name = "wan-failed"
		stop, done := make(chan struct{}), make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slog.New(slog.NewTextHandler(io.Discard, nil)))
		w.send(ppp.ProtoLCP, 1, 91, exchangePeerOptions(0))
		select {
		case <-done:
		default:
			t.Fatal("DROP failure did not end restart")
		}
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	reg.Handler().ServeHTTP(recorder, request)
	for _, want := range []string{
		`ze_pppoe_client_renegotiations_total{name="wan-a"} 2`,
		`ze_pppoe_client_renegotiations_total{name="wan-b"} 1`,
		`ze_pppoe_client_renegotiations_total{name="wan-failed"} 1`,
	} {
		if !strings.Contains(recorder.Body.String(), want+"\n") {
			t.Fatalf("exporter missing %s: %s", want, recorder.Body.String())
		}
	}
}
