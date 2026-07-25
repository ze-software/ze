package gnmi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"github.com/ze-software/ze/internal/core/metrics"
)

func TestGNMIMetricsIncrement(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	m := initGNMIMetrics(reg)

	m.requestsTotal.With("Capabilities").Inc()
	m.requestsTotal.With("Get").Inc()
	m.requestsTotal.With("Get").Inc()
	m.requestsTotal.With("Set").Inc()
	m.requestsTotal.With("Subscribe").Inc()

	require.NotNil(t, m.requestsTotal)
}

func TestGNMIMetricsErrorLabels(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	m := initGNMIMetrics(reg)

	m.errorsTotal.With("Get", codes.NotFound.String()).Inc()
	m.errorsTotal.With("Set", codes.InvalidArgument.String()).Inc()
	m.errorsTotal.With("Subscribe", codes.Unavailable.String()).Inc()

	require.NotNil(t, m.errorsTotal)
}

func TestGNMISubscribeActiveGauge(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	m := initGNMIMetrics(reg)

	m.subscribeActive.Inc()
	m.subscribeActive.Inc()
	m.subscribeActive.Dec()

	require.NotNil(t, m.subscribeActive)
}

func TestServerRecordErrorNilMetrics(t *testing.T) {
	s := &Server{}
	assert.NotPanics(t, func() {
		s.recordError("Get", codes.NotFound)
	})
}
