// Design: docs/architecture/l2tp/l2tp-9-observer.md -- original sampling interval

package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestObserverEchoStateKeepsExactConfiguredInterval reads the accepted positive
// override through the actual parser, admitted Echo and public quality view.
// MUTATION: deriving interval from a truncated bucket count rounds or loses it.
func TestObserverEchoStateKeepsExactConfiguredInterval(t *testing.T) {
	for _, test := range []struct {
		name     string
		interval time.Duration
	}{
		{"1s", time.Second},
		{"3s", 3 * time.Second},
		{"101s", 101 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			// The existing environment cache is process-wide, so this parser
			// path MUST remain serial with other environment-changing tests.
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
			assert.Equal(t, test.interval, state.EchoInterval)
		})
	}
}
