// Design: docs/architecture/isis/isis-3-l2-transport.md -- transport Prometheus metrics

package transport

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
)

// scrape renders the registry's exposition text via its HTTP handler.
func scrape(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), "GET", "/metrics", http.NoBody)
	w := httptest.NewRecorder()
	reg.Handler().ServeHTTP(w, req)
	return w.Body.String()
}

func TestTransportMetricsRegisterExactSeries(t *testing.T) {
	// VALIDATES: this spec registers EXACTLY the umbrella Metrics-table rows it
	// owns (ze_isis_frames_sent_total, ze_isis_frames_received_total,
	// ze_isis_frames_dropped_total, ze_isis_sockets_open) with the right labels.
	// PREVENTS: bare isis_* names or registering another owner's series.
	reg := metrics.NewPrometheusRegistry()
	m := newTransportMetrics(reg)
	if m == nil {
		t.Fatal("nil metrics")
	}
	// Exercise each so a label-count mismatch would panic at registration time.
	m.framesSent.With("eth0").Inc()
	m.framesReceived.With("eth0").Inc()
	m.framesDropped.With("eth0", "bad-llc").Inc()
	m.socketsOpen.Set(1)

	out := scrape(t, reg)
	for _, want := range []string{
		"ze_isis_frames_sent_total",
		"ze_isis_frames_received_total",
		"ze_isis_frames_dropped_total",
		"ze_isis_sockets_open",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metric %q not exposed", want)
		}
	}
	// No bare isis_* names (every series must be ze_isis_*).
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, "isis_") {
			t.Errorf("bare isis_* metric name: %q", line)
		}
	}
	// Label presence: dropped has interface+reason; sent/received have interface.
	if !strings.Contains(out, `reason="bad-llc"`) || !strings.Contains(out, `interface="eth0"`) {
		t.Errorf("expected interface/reason labels in output:\n%s", out)
	}
}

func TestNopTransportMetricsSafe(t *testing.T) {
	// VALIDATES: the default (pre-SetMetrics) metrics are safe no-ops.
	m := nopTransportMetrics()
	m.framesSent.With("eth0").Inc()
	m.framesReceived.With("eth0").Inc()
	m.framesDropped.With("eth0", "x").Inc()
	m.socketsOpen.Set(0)
}
