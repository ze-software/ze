// Design: docs/architecture/l2tp/l2tp-9-observer.md -- original contributor and closed observer quality
// Related: observer.go -- real lifecycle subscriptions, bounded history and CQM buckets

package l2tp

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// TestObserverOlderContributorDownPreservesCurrentLogin ends the older of two
// established births sharing a login. Distinct SIDs isolate login selection
// from the separately tested equal-SID ring collision.
// MUTATION: an unconditional named Down replaces the newer contributor's phase.
func TestObserverOlderContributorDownPreservesCurrentLogin(t *testing.T) {
	bus, observer, generation := newObserverCQMTest(t, nil)
	first, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	second, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	emitObserverCQMEstablished(t, bus, first, generation, 41, 7*time.Millisecond)
	emitObserverCQMEstablished(t, bus, second, generation, 42, 13*time.Millisecond)
	require.Len(t, observer.SessionSummaries(), 2)
	require.Equal(t, "established", observer.EchoState("alice").BucketState)
	require.EqualValues(t, 2, observer.LoginSummaries()[0].EchoCount)

	_, err = l2tpevents.SessionDown.Emit(bus, &l2tpevents.SessionDownPayload{
		Birth: first, SubscriberID: first.String(), Generation: generation,
		TunnelID: 17, SessionID: 41, Username: "alice",
	})
	require.NoError(t, err)
	_, err = l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
		Birth: second, SubscriberID: second.String(), Generation: generation,
		TunnelID: 17, SessionID: 42, Username: "alice", RTT: 17 * time.Millisecond,
	})
	require.NoError(t, err)

	summaries := observer.SessionSummaries()
	require.Len(t, summaries, 1)
	require.EqualValues(t, 42, summaries[0].SessionID)
	history := observer.SessionEvents(second)
	require.Len(t, history, 4, "surviving Up, assignment and both echoes remain useful")
	require.Equal(t, ObserverEventSessionUp, history[0].Type)
	require.Equal(t, ObserverEventSessionIPAssigned, history[1].Type)
	require.Equal(t, 13*time.Millisecond, history[2].RTT)
	require.Equal(t, 17*time.Millisecond, history[3].RTT)
	require.EqualValues(t, 3, observer.LoginSummaries()[0].EchoCount)
	assert.Equal(t, "established", observer.EchoState("alice").BucketState,
		"older contributor Down cannot end the selected established contributor")
}

// TestObserverConsumedDownCannotDowngradeReconnectedLogin consumes the original
// Down before establishing a replacement birth, then repeats that original Down.
// The replacement's useful history and the continuing login phase must survive.
// MUTATION: touching the login for an already consumed Down overwrites its phase.
func TestObserverConsumedDownCannotDowngradeReconnectedLogin(t *testing.T) {
	bus, observer, generation := newObserverCQMTest(t, nil)
	first, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	second, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	emitObserverCQMEstablished(t, bus, first, generation, 51, 11*time.Millisecond)
	down := &l2tpevents.SessionDownPayload{
		Birth: first, SubscriberID: first.String(), Generation: generation,
		TunnelID: 17, SessionID: 51, Username: "alice",
	}
	_, err = l2tpevents.SessionDown.Emit(bus, down)
	require.NoError(t, err)
	require.Empty(t, observer.SessionSummaries())
	require.Equal(t, "down", observer.EchoState("alice").BucketState,
		"the original contributor's first Down remains effective")
	emitObserverCQMEstablished(t, bus, second, generation, 52, 13*time.Millisecond)
	beforeEvents := observer.SessionEvents(second)
	require.Len(t, beforeEvents, 3)
	beforeLogins := observer.LoginSummaries()
	require.Len(t, beforeLogins, 1)
	require.Equal(t, "established", beforeLogins[0].LastState)
	require.EqualValues(t, 2, beforeLogins[0].EchoCount)
	beforeEcho := observer.EchoState("alice")
	require.Equal(t, 12*time.Millisecond, beforeEcho.LastRTT)

	_, err = l2tpevents.SessionDown.Emit(bus, down)
	require.NoError(t, err)
	assert.Equal(t, beforeEvents, observer.SessionEvents(second), "replacement history survives")
	assert.Equal(t, beforeLogins, observer.LoginSummaries(),
		"a consumed Down cannot downgrade the replacement's phase or samples")
	assert.Equal(t, beforeEcho, observer.EchoState("alice"))
	summaries := observer.SessionSummaries()
	require.Len(t, summaries, 1)
	assert.EqualValues(t, 52, summaries[0].SessionID)
}

// TestObserverCopiedEchoAfterStopCannotChangeHistoryOrQuality pauses a real
// typed bus delivery after its handler has escaped the bus lock, closes the
// observer, then resumes that handler. Useful earlier history and every quality
// observable must remain unchanged, including finalized native metric series.
// MUTATION: Stop only unsubscribes, leaving copied callbacks able to mutate.
func TestObserverCopiedEchoAfterStopCannotChangeHistoryOrQuality(t *testing.T) {
	registry := metrics.NewPrometheusRegistry()
	previousMetrics := l2tpMetricsPtr.Load()
	binding := bindL2TPMetrics(registry)
	t.Cleanup(func() { l2tpMetricsPtr.Store(previousMetrics) })
	bus, observer, generation := newObserverCQMTest(t, binding)
	birth, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	emitObserverCQMEstablished(t, bus, birth, generation, 61, 11*time.Millisecond)

	// Lifecycle payloads have no clock field. Age only the already admitted
	// bucket under its existing mutex; the next real Echo closes that bucket.
	observer.mu.Lock()
	observer.logins["alice"].current.Start = time.Now().Add(-BucketInterval)
	observer.mu.Unlock()
	_, err = l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
		Birth: birth, SubscriberID: birth.String(), Generation: generation,
		TunnelID: 17, SessionID: 61, Username: "alice", RTT: 13 * time.Millisecond,
	})
	require.NoError(t, err)
	beforeEvents := observer.SessionEvents(birth)
	require.Len(t, beforeEvents, 4)
	require.Equal(t, 13*time.Millisecond, beforeEvents[3].RTT)
	beforeSamples := observer.LoginSamples("alice")
	require.Len(t, beforeSamples, 1)
	require.EqualValues(t, 1, beforeSamples[0].EchoCount)
	require.Equal(t, 11*time.Millisecond, beforeSamples[0].AvgRTT())
	beforeLogins := observer.LoginSummaries()
	require.Len(t, beforeLogins, 1)
	require.Equal(t, "established", beforeLogins[0].LastState)
	beforeEcho := observer.EchoState("alice")
	require.Equal(t, 13*time.Millisecond, beforeEcho.LastRTT)
	beforeMetrics := observerCQMMetrics(registry)
	require.Contains(t, beforeMetrics, "ze_l2tp_lcp_echo_rtt_seconds_count{username=\"alice\"} 1\n")
	require.Contains(t, beforeMetrics, "ze_l2tp_bucket_state{username=\"alice\"} 0\n")

	// A wrongly admitted late Echo would close another useful bucket, making
	// metric and sample mutations observable without sleeping for 100 seconds.
	observer.mu.Lock()
	observer.logins["alice"].current.Start = time.Now().Add(-BucketInterval)
	observer.mu.Unlock()
	captured := make(chan struct{})
	resume := make(chan struct{})
	release := sync.OnceFunc(func() { close(resume) })
	delivered := make(chan error, 1)
	key := l2tpevents.Namespace + "/" + l2tpevents.EchoRTTEvent
	bus.mu.Lock()
	handlers := append([]func(any){}, bus.handlers[key]...)
	bus.mu.Unlock()
	require.Len(t, handlers, 1)
	typedHandler := handlers[0]
	bus.mu.Lock()
	bus.handlers[key][0] = func(payload any) {
		close(captured)
		<-resume
		typedHandler(payload)
	}
	bus.mu.Unlock()
	t.Cleanup(func() {
		release()
		select {
		case <-delivered:
		case <-time.After(5 * time.Second):
			t.Error("copied Echo delivery did not finish after its gate was released")
		}
	})
	go func() {
		_, emitErr := l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
			Birth: birth, SubscriberID: birth.String(), Generation: generation,
			TunnelID: 17, SessionID: 61, Username: "alice", RTT: 17 * time.Millisecond,
		})
		delivered <- emitErr
		close(delivered)
	}()
	select {
	case <-captured:
	case <-time.After(5 * time.Second):
		t.Fatal("real typed Echo delivery did not reach its copied handler")
	}
	observer.Stop()
	// Stop may release its last metric leases. The escaped callback MUST NOT
	// mutate the metric state that remains after that legitimate cleanup.
	afterStopMetrics := observerCQMMetrics(registry)
	release()
	select {
	case emitErr := <-delivered:
		require.NoError(t, emitErr)
	case <-time.After(5 * time.Second):
		t.Fatal("copied Echo delivery did not return after Stop")
	}

	assert.Equal(t, beforeEvents, observer.SessionEvents(birth), "closed history is read-only")
	assert.Equal(t, beforeSamples, observer.LoginSamples("alice"), "closed sample history survives")
	assert.Equal(t, beforeLogins, observer.LoginSummaries(), "closed login summaries survive")
	assert.Equal(t, beforeEcho, observer.EchoState("alice"), "closed quality state survives")
	assert.Equal(t, afterStopMetrics, observerCQMMetrics(registry), "closed CQM metrics cannot change")
}

func newObserverCQMTest(t *testing.T, binding *l2tpMetrics) (*testBus, *Observer, l2tpevents.ObservationGeneration) {
	t.Helper()
	generation, err := l2tpevents.NewObservationGeneration()
	require.NoError(t, err)
	bus := newTestBus()
	cfg := testObserverConfig()
	cfg.EchoInterval = time.Second
	cfg.Generation = generation
	cfg.Metrics = binding
	observer, err := newObserver(cfg)
	require.NoError(t, err)
	subsystem := &Subsystem{observer: observer}
	subsystem.wireObserverSubscriptions(bus)
	t.Cleanup(observer.Stop)
	return bus, observer, generation
}

func emitObserverCQMEstablished(t *testing.T, bus *testBus, birth subscriberid.L2TPBirth,
	generation l2tpevents.ObservationGeneration, sid uint16, rtt time.Duration,
) {
	t.Helper()
	_, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
		Birth: birth, SubscriberID: birth.String(), Generation: generation,
		TunnelID: 17, SessionID: sid,
	})
	require.NoError(t, err)
	_, err = l2tpevents.SessionIPAssigned.Emit(bus, &l2tpevents.SessionIPAssignedPayload{
		Birth: birth, SubscriberID: birth.String(), Generation: generation,
		TunnelID: 17, SessionID: sid, Username: "alice",
	})
	require.NoError(t, err)
	_, err = l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
		Birth: birth, SubscriberID: birth.String(), Generation: generation,
		TunnelID: 17, SessionID: sid, Username: "alice", RTT: rtt,
	})
	require.NoError(t, err)
}

func observerCQMMetrics(registry *metrics.PrometheusRegistry) string {
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	return response.Body.String()
}
