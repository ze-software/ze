package rpki

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/metrics"
)

func scrapeRPKIMetrics(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	require.Equal(t, 200, rec.Code)
	b, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	return string(b)
}

// TestBuildDecisionsRecordsASPAOutcomes verifies buildDecisions increments the
// per-result ze_rpki_aspa_outcomes_total counter for each active-ASPA route.
//
// VALIDATES: L84 phase 6 -- ASPA verification outcomes are metered (RPKI origin
// validation was already; ASPA was not).
// PREVENTS: ASPA valid/invalid/unknown volume being invisible on the dashboard.
func TestBuildDecisionsRecordsASPAOutcomes(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)
	t.Cleanup(func() { rpkiMetricsPtr.Store(nil) })

	rp := &rPKIPlugin{}
	batch := []validationRequest{
		{state: ValidationValid, aspaState: ASPAValid},
		{state: ValidationValid, aspaState: ASPAInvalid},
		{state: ValidationValid, aspaState: ASPAUnknown},
		{state: ValidationValid, aspaState: ASPAUnknown},
		{state: ValidationValid, aspaState: aspaStateNone}, // ASPA inactive: not counted
	}
	rp.buildDecisions(batch)

	out := scrapeRPKIMetrics(t, reg)
	assert.Contains(t, out, `ze_rpki_aspa_outcomes_total{result="valid"} 1`)
	assert.Contains(t, out, `ze_rpki_aspa_outcomes_total{result="invalid"} 1`)
	assert.Contains(t, out, `ze_rpki_aspa_outcomes_total{result="unknown"} 2`,
		"the aspaStateNone route must not inflate the unknown bucket")
}

// TestBuildDecisionsASPAOutcomesSkipsInactive verifies that when ASPA is inactive
// (aspaState == aspaStateNone) no aspa outcome series is created at all.
//
// VALIDATES: L84 -- ASPA metrics only count routes actually ASPA-verified.
// PREVENTS: a spurious ze_rpki_aspa_outcomes_total series when ASPA is disabled.
func TestBuildDecisionsASPAOutcomesSkipsInactive(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)
	t.Cleanup(func() { rpkiMetricsPtr.Store(nil) })

	rp := &rPKIPlugin{}
	batch := []validationRequest{
		{state: ValidationValid, aspaState: aspaStateNone},
		{state: ValidationNotFound, aspaState: aspaStateNone},
	}
	rp.buildDecisions(batch)

	out := scrapeRPKIMetrics(t, reg)
	assert.NotContains(t, out, "ze_rpki_aspa_outcomes_total{",
		"ASPA-inactive routes must not create an aspa outcome series")
}
