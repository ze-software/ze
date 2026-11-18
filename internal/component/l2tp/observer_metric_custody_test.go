// Design: docs/architecture/l2tp/l2tp-10-metrics.md -- exact original quality series custody

package l2tp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/metrics"
)

func cqmAPIBinding(t *testing.T) (*metrics.PrometheusRegistry, *l2tpMetrics) {
	t.Helper()
	previous := l2tpMetricsPtr.Load()
	t.Cleanup(func() { l2tpMetricsPtr.Store(previous) })
	registry := metrics.NewPrometheusRegistry()
	return registry, bindL2TPMetrics(registry)
}

func cqmAPIScrape(t *testing.T, registry *metrics.PrometheusRegistry) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	registry.Handler().ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), "GET", "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, recorder.Code)
	return recorder.Body.String()
}

// MUTATION: looking up the replaceable metric pointer at bucket close writes
// to the replacement registry and loses the original observer's measurement.
func TestObserverMetricsStayWithOriginalPhysicalBinding(t *testing.T) {
	originalRegistry, original := cqmAPIBinding(t)
	generation := cqmAPIGeneration(t)
	observer := cqmAPIObserver(t, generation, original, 2, 1)
	birth := cqmAPIBirth(t)
	at := time.Unix(1000, 0)
	cqmAPIEstablished(t, observer, generation, birth, "alice", at)
	require.NotContains(t, cqmAPIScrape(t, originalRegistry), `username="alice"`, "retention alone publishes no guessed sample")
	replacementRegistry, _ := cqmAPIBinding(t)
	require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth, ObserverEventEchoRTT, at.Add(BucketInterval), 13*time.Millisecond), "alice"))
	require.Contains(t, cqmAPIScrape(t, originalRegistry), "ze_l2tp_lcp_echo_rtt_seconds_count{username=\"alice\"} 1\n")
	require.NotContains(t, cqmAPIScrape(t, replacementRegistry), `username="alice"`)
	observer.Stop()
	require.NotContains(t, cqmAPIScrape(t, originalRegistry), `username="alice"`, "final original custody releases its actual physical series")
	require.NotContains(t, cqmAPIScrape(t, replacementRegistry), `username="alice"`)
	require.Len(t, observer.LoginSamples("alice"), 1, "series cleanup keeps readonly quality history")
}

// MUTATION: deleting labels when one observer evicts its login removes a
// still-retained physical series belonging to another observer's same login.
func TestObserverMetricEvictionReleasesOnlyItsOwnInterest(t *testing.T) {
	registry, binding := cqmAPIBinding(t)
	olderGeneration, newerGeneration := cqmAPIGeneration(t), cqmAPIGeneration(t)
	older := cqmAPIObserver(t, olderGeneration, binding, 2, 1)
	newer := cqmAPIObserver(t, newerGeneration, binding, 2, 1)
	olderBirth, newerBirth := cqmAPIBirth(t), cqmAPIBirth(t)
	at := time.Unix(1000, 0)
	cqmAPIEstablished(t, older, olderGeneration, olderBirth, "alice", at)
	cqmAPIEstablished(t, newer, newerGeneration, newerBirth, "alice", at)
	require.NoError(t, newer.observeEvent(newerGeneration, cqmAPIEvent(newerBirth, ObserverEventEchoRTT, at.Add(BucketInterval), 13*time.Millisecond), "alice"))
	measured := cqmAPIScrape(t, registry)
	require.Contains(t, measured, "ze_l2tp_lcp_echo_rtt_seconds_count{username=\"alice\"} 1\n")
	cqmAPIEstablished(t, older, olderGeneration, cqmAPIBirth(t), "bob", at)
	require.Nil(t, older.LoginSamples("alice"))
	require.Equal(t, measured, cqmAPIScrape(t, registry), "evicting an older slot cannot delete the current holder's series")
	older.Stop()
	require.Equal(t, measured, cqmAPIScrape(t, registry))
	newer.Stop()
	newer.Stop()
	require.NotContains(t, cqmAPIScrape(t, registry), `username="alice"`)
	require.Len(t, newer.LoginSamples("alice"), 1)
}

type cqmAPIRefusingGauge struct {
	metrics.GaugeVec
	retainer metrics.SeriesRetainer
	refused  string
	err      error
}

func (vector *cqmAPIRefusingGauge) RetainSeries(labels ...string) (metrics.SeriesLease, error) {
	if len(labels) == 1 {
		if labels[0] == vector.refused {
			return metrics.SeriesLease{}, vector.err
		}
	}
	return vector.retainer.RetainSeries(labels...)
}

type cqmAPIUnretainedGauge struct{ metrics.GaugeVec }

// TestObserverUnsupportedMetricCustodyRefusesBeforeEviction uses an actual
// vector through an interface without native custody and keeps a useful login.
// MUTATION: silently falling back to unleased metrics evicts admitted history.
func TestObserverUnsupportedMetricCustodyRefusesBeforeEviction(t *testing.T) {
	_, binding := cqmAPIBinding(t)
	generation := cqmAPIGeneration(t)
	observer := cqmAPIObserver(t, generation, binding, 2, 1)
	at := time.Unix(1000, 0)
	cqmAPIEstablished(t, observer, generation, cqmAPIBirth(t), "alice", at)
	before := observer.LoginSummaries()
	original := binding.lcpEchoLossRatio
	binding.lcpEchoLossRatio = cqmAPIUnretainedGauge{GaugeVec: original}
	err := observer.observeEvent(generation, cqmAPIEvent(cqmAPIBirth(t), ObserverEventSessionIPAssigned, at, 0), "bob")
	require.ErrorIs(t, err, errCQMSeriesUnsupported)
	require.Equal(t, before, observer.LoginSummaries())
	require.Nil(t, observer.LoginSamples("bob"))
	binding.lcpEchoLossRatio = original
	cqmAPIEstablished(t, observer, generation, cqmAPIBirth(t), "bob", at)
	require.Equal(t, "established", observer.EchoState("bob").BucketState)
}

// MUTATION: acquiring after LRU eviction destroys useful history on refusal;
// retaining only some vectors leaks custody and blocks later ordinary deletion.
func TestObserverMetricPartialRefusalKeepsHistoryAndReleasesAcquisitions(t *testing.T) {
	for _, failedVector := range []string{"loss", "state"} {
		t.Run(failedVector, func(t *testing.T) {
			registry, binding := cqmAPIBinding(t)
			generation := cqmAPIGeneration(t)
			observer := cqmAPIObserver(t, generation, binding, 3, 1)
			birth := cqmAPIBirth(t)
			at := time.Unix(1000, 0)
			cqmAPIEstablished(t, observer, generation, birth, "alice", at)
			require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth, ObserverEventEchoRTT, at.Add(BucketInterval), 13*time.Millisecond), "alice"))
			beforeSamples, beforeLogins := observer.LoginSamples("alice"), observer.LoginSummaries()
			refusal := errors.New("controlled physical vector retention refusal")
			originalLoss, originalState := binding.lcpEchoLossRatio, binding.bucketState
			failed := originalLoss
			if failedVector == "state" {
				failed = originalState
			}
			wrapper := &cqmAPIRefusingGauge{GaugeVec: failed, retainer: failed.(metrics.SeriesRetainer), refused: "bob", err: refusal}
			if failedVector == "loss" {
				binding.lcpEchoLossRatio = wrapper
			} else {
				binding.bucketState = wrapper
			}
			other := cqmAPIBirth(t)
			err := observer.observeEvent(generation, cqmAPIEvent(other, ObserverEventSessionIPAssigned, at, 0), "bob")
			require.ErrorIs(t, err, refusal)
			require.Equal(t, beforeSamples, observer.LoginSamples("alice"))
			require.Equal(t, beforeLogins, observer.LoginSummaries())
			require.Nil(t, observer.LoginSamples("bob"))
			// Publish through the actual native vectors only after refusal. A leaked
			// earlier lease would make their ordinary checked deletion return false.
			binding.lcpEchoRTTSecs.With("bob").Observe(0.017)
			require.True(t, binding.lcpEchoRTTSecs.Delete("bob"), "partial RTT custody was released")
			if failedVector == "state" {
				originalLoss.With("bob").Set(0.5)
				require.True(t, originalLoss.Delete("bob"), "partial loss custody was released")
			}
			require.NotContains(t, cqmAPIScrape(t, registry), `username="bob"`)
			binding.lcpEchoLossRatio, binding.bucketState = originalLoss, originalState
			cqmAPIEstablished(t, observer, generation, other, "bob", at)
			require.NotNil(t, observer.EchoState("bob"), "a useful retry can acquire the complete original vector set")
			require.Nil(t, observer.LoginSamples("alice"), "only successful admission may evict the old bounded login")
		})
	}
}

// MUTATION: bypassing native label admission admits an invalid label or evicts
// useful quality before Prometheus can refuse the malformed login.
func TestObserverMetricInvalidLabelCannotEvictUsefulLogin(t *testing.T) {
	_, binding := cqmAPIBinding(t)
	generation := cqmAPIGeneration(t)
	observer := cqmAPIObserver(t, generation, binding, 3, 1)
	at := time.Unix(1000, 0)
	cqmAPIEstablished(t, observer, generation, cqmAPIBirth(t), "alice", at)
	before := observer.LoginSummaries()
	invalid := string([]byte{0xff})
	err := observer.observeEvent(generation, cqmAPIEvent(cqmAPIBirth(t), ObserverEventSessionIPAssigned, at, 0), invalid)
	require.Error(t, err)
	require.Equal(t, before, observer.LoginSummaries())
	require.Nil(t, observer.LoginSamples(invalid))
	cqmAPIEstablished(t, observer, generation, cqmAPIBirth(t), "Zoë", at)
	require.Equal(t, "established", observer.EchoState("Zoë").BucketState)
}
