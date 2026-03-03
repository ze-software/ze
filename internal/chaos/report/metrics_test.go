package report

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/chaos/peer"
)

// TestMetricsEndpoint verifies /metrics returns valid Prometheus text format.
//
// VALIDATES: HTTP endpoint produces text/plain with Prometheus metric lines.
// PREVENTS: Metrics endpoint returning empty or malformed output.
func TestMetricsEndpoint(t *testing.T) {
	m := NewMetrics()
	defer func() { require.NoError(t, m.Close()) }()

	// Feed some events.
	m.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: time.Now()})
	m.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: time.Now()})

	// Serve /metrics via httptest.
	handler := m.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	text := string(body)

	// Must contain our custom metrics.
	assert.Contains(t, text, "ze_chaos_routes_announced_total")
	assert.Contains(t, text, "ze_chaos_peers_established")
}

// TestMetricsCounters verifies counters increment correctly per event.
//
// VALIDATES: Each route/chaos event increments the corresponding counter.
// PREVENTS: Counter not updating or wrong counter being incremented.
func TestMetricsCounters(t *testing.T) {
	m := NewMetrics()
	defer func() { require.NoError(t, m.Close()) }()

	// Send 3 route-sent, 2 route-received, 1 chaos.
	for range 3 {
		m.ProcessEvent(peer.Event{Type: peer.EventRouteSent, Time: time.Now()})
	}
	for range 2 {
		m.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, Time: time.Now()})
	}
	m.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, Time: time.Now(), ChaosAction: "tcp-disconnect"})

	text := scrapeMetrics(t, m)

	// Verify counters.
	assert.Contains(t, text, "ze_chaos_routes_announced_total 3")
	assert.Contains(t, text, "ze_chaos_routes_received_total 2")
	assert.Contains(t, text, "ze_chaos_chaos_events_total 1")
}

// TestMetricsGauges verifies peers established gauge updates on connect/disconnect.
//
// VALIDATES: Gauge increments on establish, decrements on disconnect.
// PREVENTS: Gauge only going up (never reflecting disconnects).
func TestMetricsGauges(t *testing.T) {
	m := NewMetrics()
	defer func() { require.NoError(t, m.Close()) }()

	// Establish 3 peers.
	for i := range 3 {
		m.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: i, Time: time.Now()})
	}

	text := scrapeMetrics(t, m)
	assert.Contains(t, text, "ze_chaos_peers_established 3")

	// Disconnect 1 peer.
	m.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 1, Time: time.Now()})

	text = scrapeMetrics(t, m)
	assert.Contains(t, text, "ze_chaos_peers_established 2")
}

// TestMetricsWithdrawals verifies withdrawal counter tracks count field.
//
// VALIDATES: EventWithdrawalSent adds ev.Count to the counter (not just 1).
// PREVENTS: Withdrawal counter counting events instead of routes.
func TestMetricsWithdrawals(t *testing.T) {
	m := NewMetrics()
	defer func() { require.NoError(t, m.Close()) }()

	m.ProcessEvent(peer.Event{Type: peer.EventWithdrawalSent, Time: time.Now(), Count: 50})
	m.ProcessEvent(peer.Event{Type: peer.EventWithdrawalSent, Time: time.Now(), Count: 30})

	text := scrapeMetrics(t, m)
	assert.Contains(t, text, "ze_chaos_routes_withdrawn_total 80")
}

// scrapeMetrics fetches the /metrics text from a Metrics instance.
func scrapeMetrics(t *testing.T, m *Metrics) string {
	t.Helper()
	handler := m.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer func() { require.NoError(t, resp.Body.Close()) }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Filter to only our metrics (skip comments and go_ metrics).
	var lines []string
	for line := range strings.SplitSeq(string(body), "\n") {
		if strings.HasPrefix(line, "ze_chaos_") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
