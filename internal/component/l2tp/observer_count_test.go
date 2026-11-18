// Design: docs/architecture/l2tp/l2tp-9-observer.md -- complete bucket counts
// Related: observer_metrics.go -- finalized bucket loss uses the same full count

package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestObserverBucketEchoCountCrossesUint16Boundary delivers the 65536th echo
// through admitted original observation and reads current and finalized quality.
// MUTATION: a uint16 count wraps, loses the average and resets the next minimum.
func TestObserverBucketEchoCountCrossesUint16Boundary(t *testing.T) {
	observer, generation := newObserverFixture(t, observerConfig{
		MaxSessions: 1, EventRingSize: 4, MaxLogins: 1, BucketCount: 2,
		EchoInterval: time.Millisecond,
	})
	birth := newObserverFixtureBirth(t)
	start := time.Unix(1000, 0)
	for index := range 65535 {
		err := observer.observeEvent(generation, cqmAPIEvent(birth, ObserverEventEchoRTT,
			start.Add(time.Duration(index)*time.Millisecond), 2*time.Millisecond), "alice")
		require.NoError(t, err)
	}
	summaries := observer.LoginSummaries()
	require.Len(t, summaries, 1)
	require.EqualValues(t, 65535, summaries[0].EchoCount, "useful pre-boundary control")
	require.Equal(t, 2*time.Millisecond, summaries[0].AvgRTT)
	require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth,
		ObserverEventEchoRTT, start.Add(65535*time.Millisecond), 4*time.Millisecond), "alice"))
	summaries = observer.LoginSummaries()
	require.Len(t, summaries, 1)
	assert.EqualValues(t, 65536, summaries[0].EchoCount)
	assert.Equal(t, (65535*2*time.Millisecond+4*time.Millisecond)/65536, summaries[0].AvgRTT)
	require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth,
		ObserverEventEchoRTT, start.Add(65536*time.Millisecond), 6*time.Millisecond), "alice"))
	require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth,
		ObserverEventEchoRTT, start.Add(BucketInterval), time.Millisecond), "alice"))
	buckets := observer.LoginSamples("alice")
	require.Len(t, buckets, 1)
	assert.EqualValues(t, 65537, buckets[0].EchoCount)
	assert.Equal(t, 2*time.Millisecond, buckets[0].MinRTT)
	assert.Equal(t, 6*time.Millisecond, buckets[0].MaxRTT)
	assert.Equal(t, (65535*2*time.Millisecond+10*time.Millisecond)/65537, buckets[0].AvgRTT())
}

// TestObserverPositiveEchoIntervalsKeepFullExpectedCount drives the existing
// accepted duration parser into the observer, live loss view and native metric.
// MUTATION: narrowing expected echoes changes short intervals and their loss.
func TestObserverPositiveEchoIntervalsKeepFullExpectedCount(t *testing.T) {
	for _, test := range []struct {
		name     string
		interval time.Duration
		loss     float64
		metric   string
	}{
		{"1s", time.Second, 0.99, "0.99"},
		{"1ms", time.Millisecond, 0.99999, "0.99999"},
		{"1ns", time.Nanosecond, 0.99999999999, "0.99999999999"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// The environment cache and original metric binding are process-wide;
			// this parser-to-observer path deliberately cannot run in parallel.
			useEnv(t, map[string]string{cqmEchoIntervalKey: test.name})
			root := NewSubsystem(&Parameters{CQMEnabled: true})
			interval := root.cqmEchoInterval()
			require.Equal(t, test.interval, interval, "the existing positive override remains supported")
			registry, binding := cqmAPIBinding(t)
			observer, generation := newObserverFixture(t, observerConfig{
				Metrics: binding, MaxSessions: 1, EventRingSize: 4, MaxLogins: 1,
				BucketCount: 2, EchoInterval: interval,
			})
			birth := newObserverFixtureBirth(t)
			start := time.Unix(1000, 0)
			require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth,
				ObserverEventEchoRTT, start, 2*time.Millisecond), "alice"))
			state := observer.EchoState("alice")
			require.NotNil(t, state)
			assert.Equal(t, test.interval, state.EchoInterval)
			if assert.NotNil(t, state.LossRatio, "positive expected count keeps measured loss") {
				assert.InDelta(t, test.loss, *state.LossRatio, 1e-15)
			}
			require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth,
				ObserverEventEchoRTT, start.Add(BucketInterval), 3*time.Millisecond), "alice"))
			assert.Contains(t, cqmAPIScrape(t, registry),
				"ze_l2tp_lcp_echo_loss_ratio{username=\"alice\"} "+test.metric+"\n")
		})
	}
}
