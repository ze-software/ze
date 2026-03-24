package rib

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// ipv4Unicast is the family constant used for test route insertion.
var ipv4Unicast = nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

// scrapeRIBMetrics returns the Prometheus text exposition from the given registry.
func scrapeRIBMetrics(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	ts := httptest.NewServer(reg.Handler())
	defer ts.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL, http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // test-only URL
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// TestSetMetricsRegistry verifies gauge creation from a Prometheus registry.
//
// VALIDATES: SetMetricsRegistry creates global totals and per-peer GaugeVec gauges.
// PREVENTS: Gauges not being registered with Prometheus.
func TestSetMetricsRegistry(t *testing.T) {
	old := metricsPtr.Load()
	defer metricsPtr.Store(old)

	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)

	m := metricsPtr.Load()
	require.NotNil(t, m, "metricsPtr should be set")

	// Set global totals and verify they appear in scrape output
	m.routesIn.Set(42)
	m.routesOut.Set(7)

	// Set per-peer values
	m.routesInVec.With("10.0.0.1").Set(30)
	m.routesInVec.With("10.0.0.2").Set(12)
	m.routesOutVec.With("10.0.0.1").Set(5)
	m.routesOutVec.With("10.0.0.2").Set(2)

	body := scrapeRIBMetrics(t, reg)
	assert.Contains(t, body, "ze_rib_routes_in_total 42")
	assert.Contains(t, body, "ze_rib_routes_out_total 7")
	assert.Contains(t, body, `ze_rib_routes_in{peer="10.0.0.1"} 30`)
	assert.Contains(t, body, `ze_rib_routes_in{peer="10.0.0.2"} 12`)
	assert.Contains(t, body, `ze_rib_routes_out{peer="10.0.0.1"} 5`)
	assert.Contains(t, body, `ze_rib_routes_out{peer="10.0.0.2"} 2`)
}

// TestUpdateMetrics verifies route count gauges are populated from RIB state.
//
// VALIDATES: updateMetrics reads ribInPool and ribOut counts into per-peer and global gauges.
// PREVENTS: Prometheus output showing stale or zero route counts.
func TestUpdateMetrics(t *testing.T) {
	old := metricsPtr.Load()
	defer metricsPtr.Store(old)

	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)

	r := newTestRIBManager(t)

	// Populate ribInPool with a peer having routes
	peerRIB := storage.NewPeerRIB("10.0.0.1")
	// Insert some routes (use dummy family + wire bytes)
	peerRIB.Insert(ipv4Unicast, []byte{0x40, 0x01, 0x01, 0x00}, []byte{24, 10, 0, 0})
	peerRIB.Insert(ipv4Unicast, []byte{0x40, 0x01, 0x01, 0x00}, []byte{24, 10, 0, 1})
	r.ribInPool["10.0.0.1"] = peerRIB

	// Populate ribOut with a peer having routes
	r.ribOut["10.0.0.2"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
			"10.0.1.0/24": {Family: "ipv4/unicast", Prefix: "10.0.1.0/24"},
			"10.0.2.0/24": {Family: "ipv4/unicast", Prefix: "10.0.2.0/24"},
		},
	}

	r.updateMetrics()

	body := scrapeRIBMetrics(t, reg)
	// Global totals
	assert.Contains(t, body, "ze_rib_routes_in_total 2")
	assert.Contains(t, body, "ze_rib_routes_out_total 3")
	// Per-peer labels
	assert.Contains(t, body, `ze_rib_routes_in{peer="10.0.0.1"} 2`)
	assert.Contains(t, body, `ze_rib_routes_out{peer="10.0.0.2"} 3`)
}

// TestUpdateMetricsNilRegistry verifies updateMetrics is a no-op without metrics.
//
// VALIDATES: updateMetrics does not panic when no metrics registry is configured.
// PREVENTS: Nil pointer dereference when Prometheus is disabled.
func TestUpdateMetricsNilRegistry(t *testing.T) {
	old := metricsPtr.Load()
	defer metricsPtr.Store(old)

	metricsPtr.Store(nil)

	r := newTestRIBManager(t)
	r.updateMetrics() // Must not panic
}

// TestUpdateMetricsEmpty verifies gauges show zero for empty RIB.
//
// VALIDATES: Empty RIB produces zero-valued global gauges.
// PREVENTS: Gauges showing stale values from previous state.
func TestUpdateMetricsEmpty(t *testing.T) {
	old := metricsPtr.Load()
	defer metricsPtr.Store(old)

	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)

	r := newTestRIBManager(t)
	r.updateMetrics()

	body := scrapeRIBMetrics(t, reg)
	assert.Contains(t, body, "ze_rib_routes_in_total 0")
	assert.Contains(t, body, "ze_rib_routes_out_total 0")
}

// TestUpdateMetricsMultiplePeers verifies counts aggregate across peers.
//
// VALIDATES: Route counts sum across all peers in globals, and per-peer labels are correct.
// PREVENTS: Only counting routes from one peer.
func TestUpdateMetricsMultiplePeers(t *testing.T) {
	old := metricsPtr.Load()
	defer metricsPtr.Store(old)

	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)

	r := newTestRIBManager(t)

	// Two peers in ribInPool
	peer1 := storage.NewPeerRIB("10.0.0.1")
	peer1.Insert(ipv4Unicast, []byte{0x40, 0x01, 0x01, 0x00}, []byte{24, 10, 0, 0})
	peer2 := storage.NewPeerRIB("10.0.0.2")
	peer2.Insert(ipv4Unicast, []byte{0x40, 0x01, 0x01, 0x00}, []byte{24, 10, 1, 0})
	peer2.Insert(ipv4Unicast, []byte{0x40, 0x01, 0x01, 0x00}, []byte{24, 10, 1, 1})
	r.ribInPool["10.0.0.1"] = peer1
	r.ribInPool["10.0.0.2"] = peer2

	// Two peers in ribOut
	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
		},
	}
	r.ribOut["10.0.0.2"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.1.0.0/24": {Family: "ipv4/unicast", Prefix: "10.1.0.0/24"},
			"10.1.1.0/24": {Family: "ipv4/unicast", Prefix: "10.1.1.0/24"},
		},
	}

	r.updateMetrics()

	body := scrapeRIBMetrics(t, reg)
	// Global totals: 1 + 2 = 3 routes in, 1 + 2 = 3 routes out
	assert.Contains(t, body, "ze_rib_routes_in_total 3")
	assert.Contains(t, body, "ze_rib_routes_out_total 3")
	// Per-peer labels
	assert.Contains(t, body, `ze_rib_routes_in{peer="10.0.0.1"} 1`)
	assert.Contains(t, body, `ze_rib_routes_in{peer="10.0.0.2"} 2`)
	assert.Contains(t, body, `ze_rib_routes_out{peer="10.0.0.1"} 1`)
	assert.Contains(t, body, `ze_rib_routes_out{peer="10.0.0.2"} 2`)
}

// TestUpdateMetricsStalePeerCleanup verifies that per-peer labels are deleted
// when a peer is removed from the RIB between metric cycles.
//
// VALIDATES: Stale Prometheus labels are cleaned up when peers disconnect.
// PREVENTS: Stale per-peer gauge series accumulating in long-running daemons.
func TestUpdateMetricsStalePeerCleanup(t *testing.T) {
	old := metricsPtr.Load()
	defer metricsPtr.Store(old)

	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)

	r := newTestRIBManager(t)

	// Cycle 1: two peers in ribInPool
	peer1 := storage.NewPeerRIB("10.0.0.1")
	peer1.Insert(ipv4Unicast, []byte{0x40, 0x01, 0x01, 0x00}, []byte{24, 10, 0, 0})
	peer2 := storage.NewPeerRIB("10.0.0.2")
	peer2.Insert(ipv4Unicast, []byte{0x40, 0x01, 0x01, 0x00}, []byte{24, 10, 1, 0})
	r.ribInPool["10.0.0.1"] = peer1
	r.ribInPool["10.0.0.2"] = peer2

	r.ribOut["10.0.0.1"] = map[string]map[string]*Route{
		"ipv4/unicast": {
			"10.0.0.0/24": {Family: "ipv4/unicast", Prefix: "10.0.0.0/24"},
		},
	}

	r.updateMetrics()

	body := scrapeRIBMetrics(t, reg)
	assert.Contains(t, body, `ze_rib_routes_in{peer="10.0.0.1"} 1`)
	assert.Contains(t, body, `ze_rib_routes_in{peer="10.0.0.2"} 1`)
	assert.Contains(t, body, `ze_rib_routes_out{peer="10.0.0.1"} 1`)

	// Cycle 2: remove peer 10.0.0.2 from ribInPool and 10.0.0.1 from ribOut
	delete(r.ribInPool, "10.0.0.2")
	delete(r.ribOut, "10.0.0.1")

	r.updateMetrics()

	body = scrapeRIBMetrics(t, reg)
	// Remaining peer still present
	assert.Contains(t, body, `ze_rib_routes_in{peer="10.0.0.1"} 1`)
	// Removed peers' labels must be gone from scrape output
	assert.NotContains(t, body, `peer="10.0.0.2"`)
	assert.NotContains(t, body, `ze_rib_routes_out{peer="10.0.0.1"}`)
}
