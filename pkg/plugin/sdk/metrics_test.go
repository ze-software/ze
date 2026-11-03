// Design: docs/plugin-development/metrics.md -- metrics of a forked plugin
// Related: metrics.go -- the publisher under test
package sdk

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// publishRecorder answers each metrics-publish with the next queued answer
// ("{}" once the queue is empty) and hands the input to the test.
type publishRecorder struct {
	inputs  chan rpc.MetricsPublishInput
	answers []string
}

func (r *publishRecorder) call(_ context.Context, method string, params any) (json.RawMessage, error) {
	in, ok := params.(*rpc.MetricsPublishInput)
	if !ok || method != rpc.MethodMetricsPublish {
		return nil, assert.AnError
	}
	r.inputs <- *in
	answer := "{}"
	if len(r.answers) > 0 {
		answer, r.answers = r.answers[0], r.answers[1:]
	}
	return json.RawMessage(answer), nil
}

func (r *publishRecorder) next(t *testing.T) rpc.MetricsPublishInput {
	t.Helper()
	select {
	case in := <-r.inputs:
		return in
	case <-time.After(10 * time.Second):
		t.Fatal("no metrics publish arrived")
	}
	return rpc.MetricsPublishInput{}
}

// VALIDATES: the plugin decides when the engine learns of updates. A
// declaration is published at once; increments accumulate and travel only at
// the plugin's tick, as one summed update; a publish the engine answers
// unavailable keeps its increments for the next tick.
// METHOD: the worker runs on a ticker the test drives, against a recorder
// standing for the engine.
// PREVENTS: a publish per increment (the first update would carry 5, not 7),
// and increments lost while the engine has no registry.
func TestMetricsPublishFollowsThePluginsCadence(t *testing.T) {
	m := NewMetricsPublisher()
	tick := make(chan time.Time)
	m.newTicker = func(time.Duration) (<-chan time.Time, func()) { return tick, func() {} }
	counter := m.Counter("ze_sdktest_events_total", "Events.")

	rec := &publishRecorder{inputs: make(chan rpc.MetricsPublishInput, 8), answers: []string{"{}", `{"unavailable":true}`}}
	ctx := t.Context()
	quit, done := make(chan struct{}), make(chan struct{})
	go m.runWorker(ctx, quit, done, rec.call)
	defer func() { close(quit); <-done }()

	first := rec.next(t)
	require.Len(t, first.Declare, 1, "the declaration goes at once")
	assert.Equal(t, "ze_sdktest_events_total", first.Declare[0].Name)
	assert.Empty(t, first.Update, "updates wait for the plugin's tick")

	counter.Add(5)
	counter.Add(2)
	tick <- time.Now()
	second := rec.next(t)
	assert.Empty(t, second.Declare, "an applied declaration is not sent again")
	require.Len(t, second.Update, 1)
	assert.InDelta(t, 7, second.Update[0].Value, 0, "the increments since the last publish, summed")

	m.Gauge("ze_sdktest_depth", "Depth.")
	declared := rec.next(t)
	require.Len(t, declared.Declare, 1, "a new name wakes a publish without a tick")
	assert.Empty(t, declared.Update)

	// The engine answered the second publish unavailable: its 7 stays pending.
	counter.Inc()
	tick <- time.Now()
	third := rec.next(t)
	var events float64
	for _, u := range third.Update {
		if u.Name == "ze_sdktest_events_total" {
			events = u.Value
		}
	}
	assert.InDelta(t, 8, events, 0, "the unapplied 7 travels again with the new 1")
}

// VALIDATES: when the engine removes a forked plugin (bye), the updates made
// since the last tick are published before the removal is acknowledged.
// PREVENTS: the last interval of a removed plugin's counters vanishing.
func TestMetricsFlushBeforeByeIsAcknowledged(t *testing.T) {
	m := NewMetricsPublisher()
	m.Counter("ze_sdktest_bye_total", "Byes.").Add(3)
	client, engine := net.Pipe()
	p := NewWithConn("sdktest", ConnWithMetrics(client, m))
	pc := plugipc.NewPluginConn(engine, engine)
	t.Cleanup(func() {
		_ = p.Close()
		_ = engine.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	loopDone := make(chan error, 1)
	go func() { loopDone <- p.eventLoop(ctx) }()
	byeDone := make(chan error, 1)
	go func() {
		_, err := pc.CallRPC(ctx, callbackBye, nil)
		byeDone <- err
	}()

	req, err := pc.ReadRequest(ctx)
	require.NoError(t, err)
	require.Equal(t, rpc.MethodMetricsPublish, req.Method, "the publish comes before the bye answer")
	var in rpc.MetricsPublishInput
	require.NoError(t, json.Unmarshal(req.Params, &in))
	require.Len(t, in.Update, 1)
	assert.InDelta(t, 3, in.Update[0].Value, 0)
	select {
	case err := <-byeDone:
		t.Fatalf("bye answered before the metrics publish was: %v", err)
	default:
	}
	require.NoError(t, pc.SendResult(ctx, req.ID, rpc.MetricsPublishOutput{}))
	require.NoError(t, <-byeDone)
	require.NoError(t, <-loopDone)
}
