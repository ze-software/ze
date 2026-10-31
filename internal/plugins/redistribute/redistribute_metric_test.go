package redistribute

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// metricChain accepts every route and sets its metric, as a compiled modify
// instance's `set metric N` followed by final accept does.
type metricChain uint32

func (c metricChain) Evaluate(ribpolicy.Route) ribpolicy.Verdict {
	return ribpolicy.Verdict{Action: ribpolicy.ActionAccept, Sets: ribpolicy.Sets{Metric: uint32(c), HasMetric: true}}
}

// TestHandleBatchSetMetricReachesConsumer proves the metric an import chain
// sets reaches the consumer marked as set, which the BGP consumer turns into
// MULTI_EXIT_DISC, and that a rule whose chain sets none hands the consumer no
// set metric. Method: one source with a metric-setting rule into bgp, another
// with a plain rule, and the stub BGP consumer's recorded entries.
func TestHandleBatchSetMetricReachesConsumer(t *testing.T) {
	resetState(t)

	setID := redistevents.RegisterProtocol("metricsource")
	plainID := redistevents.RegisterProtocol("plainsource")
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "metricsource", Protocol: "metricsource"}))
	require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: "plainsource", Protocol: "plainsource"}))
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: "metricsource", Destination: "bgp", Final: "accept", Chain: metricChain(50)},
		{Source: "plainsource", Destination: "bgp"},
	}))

	consumer := registerBGPConsumer(t)
	handleBatch(context.Background(), skipIDs(bgpID), taggedAddBatch(setID, "10.0.0.0/8", 0))
	handleBatch(context.Background(), skipIDs(bgpID), taggedAddBatch(plainID, "10.1.0.0/16", 0))

	inj := consumer.snapshotInjected()
	require.Len(t, inj, 2)
	assert.True(t, inj[0].entry.HasMetric, "the chain's set metric is not marked as set")
	assert.Equal(t, uint32(50), inj[0].entry.Metric)
	assert.False(t, inj[1].entry.HasMetric, "a route no filter set a metric for is marked as set")
}
