// Design: docs/architecture/l2tp/l2tp-9-observer.md -- known and unavailable loss

package l2tp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestObserverEchoLossDistinguishesUnknownAndMeasuredZero compares real admitted
// observer projections without binding the test to the optional field's Go shape.
// MUTATION: an unavailable whole-bucket loss is returned as a measured zero.
func TestObserverEchoLossDistinguishesUnknownAndMeasuredZero(t *testing.T) {
	for _, test := range []struct {
		name      string
		interval  time.Duration
		lossKnown bool
		loss      float64
	}{
		{"1s", time.Second, true, 0.99},
		{"100s", 100 * time.Second, true, 0},
		{"101s", 101 * time.Second, false, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			// The accepted-override parser uses the existing process-wide cache;
			// this path MUST remain serial with environment-changing tests.
			useEnv(t, map[string]string{cqmEchoIntervalKey: test.name})
			root := NewSubsystem(&Parameters{CQMEnabled: true})
			interval := root.cqmEchoInterval()
			require.Equal(t, test.interval, interval)
			observer, generation := newObserverFixture(t, observerConfig{
				MaxSessions: 1, EventRingSize: 4, MaxLogins: 1, BucketCount: 2,
				EchoInterval: interval,
			})
			birth := newObserverFixtureBirth(t)
			require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth,
				ObserverEventEchoRTT, time.Unix(1000, 0), 2*time.Millisecond), "alice"))
			state := observer.EchoState("alice")
			require.NotNil(t, state)
			require.Equal(t, test.interval, state.EchoInterval)
			require.Equal(t, 2*time.Millisecond, state.LastRTT)
			summaries := observer.LoginSummaries()
			require.Len(t, summaries, 1)
			require.EqualValues(t, 1, summaries[0].EchoCount, "a useful original Echo was admitted")
			encoded, err := json.Marshal(state)
			require.NoError(t, err)
			var projection struct{ LossRatio *float64 }
			require.NoError(t, json.Unmarshal(encoded, &projection))
			if test.lossKnown {
				require.NotNil(t, projection.LossRatio, "a computed zero is still a measurement")
				assert.InDelta(t, test.loss, *projection.LossRatio, 1e-15)
			} else {
				assert.Nil(t, projection.LossRatio, "zero expected echoes leaves loss unavailable")
			}
		})
	}
}
