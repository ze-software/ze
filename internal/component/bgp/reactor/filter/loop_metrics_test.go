package filter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/metrics"
)

func scrapeLoopMetrics(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	require.Equal(t, 200, rec.Code)
	b, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	return string(b)
}

// TestASPathLoopMetricIncrements verifies an AS_PATH loop reject increments the
// per-peer ze_bgp_as_path_loop_detected_total counter.
//
// VALIDATES: L84 phase 6 -- bgp_as_path_loop_detected_total emitted on the AS
// loop reject path (RFC 4271 Section 9).
// PREVENTS: the counter registered but never advanced, so AS-loop rejects are
// invisible on the dashboard.
func TestASPathLoopMetricIncrements(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)
	t.Cleanup(func() { loopMetricsPtr.Store(nil) })

	// Local ASN 65001 in AS_PATH on an iBGP peer -> AS loop -> reject + count.
	body := makeUpdateBody(buildASPathAttr([]uint32{65002, 65001, 65003}, false))
	assert.False(t, accept(ibgpPeer(), body), "AS loop must be rejected")

	out := scrapeLoopMetrics(t, reg)
	assert.Contains(t, out, `ze_bgp_as_path_loop_detected_total{peer="192.0.2.1"} 1`,
		"AS-path loop must increment the per-peer counter")
}

// TestASPathLoopMetricNotCountedOnCleanPath verifies an accepted route with no
// AS loop leaves the counter (series) absent.
//
// VALIDATES: L84 -- only genuine AS loops are counted.
// PREVENTS: false positives inflating the loop counter.
func TestASPathLoopMetricNotCountedOnCleanPath(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)
	t.Cleanup(func() { loopMetricsPtr.Store(nil) })

	body := makeUpdateBody(buildASPathAttr([]uint32{65002, 65003, 65004}, false))
	assert.True(t, accept(ebgpPeer(), body), "clean path must be accepted")

	out := scrapeLoopMetrics(t, reg)
	assert.NotContains(t, out, "ze_bgp_as_path_loop_detected_total{",
		"a clean AS_PATH must not create the loop series")
}

// TestASPathLoopMetricNotCountedForOriginatorLoop verifies an ORIGINATOR_ID loop
// reject does NOT increment the AS-path-loop counter (it is a different loop type).
//
// VALIDATES: L84 -- the counter is scoped to AS_PATH loops only.
// PREVENTS: conflating RFC 4456 reflector loops with RFC 4271 AS loops.
func TestASPathLoopMetricNotCountedForOriginatorLoop(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)
	t.Cleanup(func() { loopMetricsPtr.Store(nil) })

	src := ibgpPeer()
	body := makeUpdateBody(append(buildASPathAttr([]uint32{65002}, false), buildOriginatorIDAttr(src.RouterID)...))
	assert.False(t, accept(src, body), "ORIGINATOR_ID loop must be rejected")

	out := scrapeLoopMetrics(t, reg)
	assert.NotContains(t, out, "ze_bgp_as_path_loop_detected_total{",
		"ORIGINATOR_ID loop must not increment the AS-path-loop counter")
}

// TestASPathLoopMetricNilRegistryNoPanic verifies LoopIngress does not panic when
// no metrics registry has been wired (the common no-telemetry build).
//
// VALIDATES: L84 -- the metric increment is nil-guarded.
// PREVENTS: a nil-pointer panic in the ingress hot path on builds without metrics.
func TestASPathLoopMetricNilRegistryNoPanic(t *testing.T) {
	loopMetricsPtr.Store(nil)
	body := makeUpdateBody(buildASPathAttr([]uint32{65002, 65001, 65003}, false))
	assert.NotPanics(t, func() { _ = accept(ibgpPeer(), body) })
}
