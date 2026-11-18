// Design: docs/architecture/l2tp/l2tp-9-observer.md -- admission and bounded login provenance
// Related: observer_cqm_lifecycle_test.go -- original contributor and escaped callback regressions

package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// TestObserverZeroBirthCannotChangeHistoryOrQuality checks all four typed
// callbacks against useful original history, samples and measured metrics.
// MUTATION: admitting a zero Birth changes a ring or its login quality.
func TestObserverZeroBirthCannotChangeHistoryOrQuality(t *testing.T) {
	checkObserverCQMInvalidDelivery(t, func(_ subscriberid.L2TPBirth,
		generation l2tpevents.ObservationGeneration,
	) (subscriberid.L2TPBirth, l2tpevents.ObservationGeneration) {
		return subscriberid.L2TPBirth{}, generation
	})
}

// TestObserverZeroGenerationCannotChangeHistoryOrQuality keeps the valid Birth
// and changes only generation, with a useful valid neighbor after each refusal.
// MUTATION: admitting a zero generation changes a ring or its login quality.
func TestObserverZeroGenerationCannotChangeHistoryOrQuality(t *testing.T) {
	checkObserverCQMInvalidDelivery(t, func(birth subscriberid.L2TPBirth,
		_ l2tpevents.ObservationGeneration,
	) (subscriberid.L2TPBirth, l2tpevents.ObservationGeneration) {
		return birth, l2tpevents.ObservationGeneration{}
	})
}

// TestObserverConsumedDownDoesNotRenewLoginInterest consumes Alice's Down,
// touches Bob, repeats Alice's Down and admits Carol into a two-login pool.
// MUTATION: promoting a consumed Down evicts useful Bob instead of ended Alice.
func TestObserverConsumedDownDoesNotRenewLoginInterest(t *testing.T) {
	bus, observer, generation, _ := newObserverCQMGuardTest(t, 4, 2)
	alice := newObserverCQMGuardBirth(t)
	bob := newObserverCQMGuardBirth(t)
	carol := newObserverCQMGuardBirth(t)
	emitObserverCQMGuardEstablished(t, bus, alice, generation, 201, "alice", 11*time.Millisecond)
	emitObserverCQMGuardDown(t, bus, alice, generation, 201, "alice")
	require.Equal(t, "down", observer.EchoState("alice").BucketState)
	require.Empty(t, observer.SessionSummaries())
	emitObserverCQMGuardEstablished(t, bus, bob, generation, 202, "bob", 13*time.Millisecond)
	beforeBob := observer.SessionEvents(202)
	require.Len(t, beforeBob, 3)
	require.Len(t, observer.LoginSummaries(), 2)

	emitObserverCQMGuardDown(t, bus, alice, generation, 201, "alice")
	emitObserverCQMGuardEstablished(t, bus, carol, generation, 203, "carol", 17*time.Millisecond)

	assert.Nil(t, observer.LoginSamples("alice"), "consumed Down cannot renew ended history")
	assert.NotNil(t, observer.LoginSamples("bob"), "the more recent login keeps its slot")
	assert.Equal(t, beforeBob, observer.SessionEvents(202), "Bob's permitted event history survives")
	require.NotNil(t, observer.EchoState("carol"))
	assert.Equal(t, 17*time.Millisecond, observer.EchoState("carol").LastRTT)
	assert.Equal(t, "established", observer.EchoState("carol").BucketState)
	assert.Len(t, observer.LoginSummaries(), 2)
}

// TestObserverContributorDownOrdersKeepSelectedPhase follows both departure
// orders. The phase belongs to the latest assigning Birth, not a live count.
// MUTATION: an older contributor's Down overwrites the selected phase.
func TestObserverContributorDownOrdersKeepSelectedPhase(t *testing.T) {
	for _, order := range []struct {
		name       string
		olderFirst bool
	}{
		{name: "older first", olderFirst: true},
		{name: "selected first"},
	} {
		t.Run(order.name, func(t *testing.T) {
			bus, observer, generation, _ := newObserverCQMGuardTest(t, 3, 2)
			older := newObserverCQMGuardBirth(t)
			selected := newObserverCQMGuardBirth(t)
			emitObserverCQMGuardEstablished(t, bus, older, generation, 301, "alice", 7*time.Millisecond)
			emitObserverCQMGuardEstablished(t, bus, selected, generation, 302, "alice", 13*time.Millisecond)
			require.Equal(t, "established", observer.EchoState("alice").BucketState)
			require.EqualValues(t, 2, observer.LoginSummaries()[0].EchoCount)

			firstBirth, firstSID := selected, uint16(302)
			lastBirth, lastSID := older, uint16(301)
			firstPhase := "down"
			if order.olderFirst {
				firstBirth, firstSID = older, 301
				lastBirth, lastSID = selected, 302
				firstPhase = "established"
			}
			lastHistory := observer.SessionEvents(lastSID)
			require.Len(t, lastHistory, 3)
			emitObserverCQMGuardDown(t, bus, firstBirth, generation, firstSID, "alice")
			assert.Equal(t, firstPhase, observer.EchoState("alice").BucketState)
			assert.Equal(t, lastHistory, observer.SessionEvents(lastSID))
			assert.Empty(t, observer.SessionEvents(firstSID))
			emitObserverCQMGuardDown(t, bus, lastBirth, generation, lastSID, "alice")
			assert.Empty(t, observer.SessionSummaries())
			assert.Equal(t, "down", observer.EchoState("alice").BucketState)
			assert.EqualValues(t, 2, observer.LoginSummaries()[0].EchoCount)
		})
	}
}

// TestObserverLaterEchoCannotAcquireLoginPhase lets another Birth echo against
// an existing established login without assigning it. Its Down cannot end it.
// MUTATION: selecting every echo's Birth or accepting every Down steals phase.
func TestObserverLaterEchoCannotAcquireLoginPhase(t *testing.T) {
	bus, observer, generation, _ := newObserverCQMGuardTest(t, 3, 2)
	assigned := newObserverCQMGuardBirth(t)
	echoing := newObserverCQMGuardBirth(t)
	emitObserverCQMGuardEstablished(t, bus, assigned, generation, 401, "alice", 11*time.Millisecond)
	_, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
		Birth: echoing, SubscriberID: echoing.String(), Generation: generation,
		TunnelID: 17, SessionID: 402,
	})
	require.NoError(t, err)
	_, err = l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
		Birth: echoing, SubscriberID: echoing.String(), Generation: generation,
		TunnelID: 17, SessionID: 402, Username: "alice", RTT: 13 * time.Millisecond,
	})
	require.NoError(t, err)
	require.Len(t, observer.SessionEvents(402), 2)
	require.EqualValues(t, 2, observer.LoginSummaries()[0].EchoCount)
	require.Equal(t, 12*time.Millisecond, observer.EchoState("alice").LastRTT)
	beforeAssigned := observer.SessionEvents(401)

	emitObserverCQMGuardDown(t, bus, echoing, generation, 402, "alice")
	assert.Equal(t, "established", observer.EchoState("alice").BucketState,
		"an echo on an existing login does not acquire its phase")
	assert.Equal(t, beforeAssigned, observer.SessionEvents(401))
	assert.EqualValues(t, 2, observer.LoginSummaries()[0].EchoCount)
	emitObserverCQMGuardDown(t, bus, assigned, generation, 401, "alice")
	assert.Equal(t, "down", observer.EchoState("alice").BucketState,
		"the actual assigning contributor's Down remains effective")
	assert.Empty(t, observer.SessionSummaries())
}

// TestObserverSameBirthAssignmentReopensConsumedPhase preserves a transport's
// second network lifetime: assignment reopens its phase and a new Down ends it.
// MUTATION: treating consumed Down as a permanent Birth tombstone drops renewal.
func TestObserverSameBirthAssignmentReopensConsumedPhase(t *testing.T) {
	bus, observer, generation, _ := newObserverCQMGuardTest(t, 2, 2)
	birth := newObserverCQMGuardBirth(t)
	emitObserverCQMGuardEstablished(t, bus, birth, generation, 501, "alice", 11*time.Millisecond)
	emitObserverCQMGuardDown(t, bus, birth, generation, 501, "alice")
	require.Empty(t, observer.SessionSummaries())
	require.Equal(t, "down", observer.EchoState("alice").BucketState)

	_, err := l2tpevents.SessionIPAssigned.Emit(bus, &l2tpevents.SessionIPAssignedPayload{
		Birth: birth, SubscriberID: birth.String(), Generation: generation,
		TunnelID: 17, SessionID: 501, Username: "alice",
	})
	require.NoError(t, err)
	_, err = l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
		Birth: birth, SubscriberID: birth.String(), Generation: generation,
		TunnelID: 17, SessionID: 501, Username: "alice", RTT: 17 * time.Millisecond,
	})
	require.NoError(t, err)
	require.Equal(t, "established", observer.EchoState("alice").BucketState)
	require.Equal(t, 14*time.Millisecond, observer.EchoState("alice").LastRTT)
	renewed := observer.SessionEvents(501)
	require.Len(t, renewed, 2)
	assert.Equal(t, ObserverEventSessionIPAssigned, renewed[0].Type)
	assert.Equal(t, 17*time.Millisecond, renewed[1].RTT)

	emitObserverCQMGuardDown(t, bus, birth, generation, 501, "alice")
	assert.Empty(t, observer.SessionSummaries())
	assert.Equal(t, "down", observer.EchoState("alice").BucketState)
	assert.EqualValues(t, 2, observer.LoginSummaries()[0].EchoCount)
}

// TestObserverNamedPreIPDownCreatesFailureObservation keeps a proxy failure
// useful without prior IP or Echo while preserving another established login.
// MUTATION: requiring an existing ring or login drops named pre-IP failures.
func TestObserverNamedPreIPDownCreatesFailureObservation(t *testing.T) {
	bus, observer, generation, _ := newObserverCQMGuardTest(t, 2, 2)
	neighbor := newObserverCQMGuardBirth(t)
	failed := newObserverCQMGuardBirth(t)
	emitObserverCQMGuardEstablished(t, bus, neighbor, generation, 601, "bob", 13*time.Millisecond)
	beforeNeighbor := observer.SessionEvents(601)
	beforeQuality := observer.EchoState("bob")
	require.Nil(t, observer.LoginSamples("proxy-failure"))

	emitObserverCQMGuardDown(t, bus, failed, generation, 602, "proxy-failure")

	assert.Equal(t, beforeNeighbor, observer.SessionEvents(601))
	assert.Equal(t, beforeQuality, observer.EchoState("bob"))
	assert.Empty(t, observer.SessionEvents(602))
	failure := observer.EchoState("proxy-failure")
	require.NotNil(t, failure, "a named pre-IP failure remains a useful login observation")
	assert.Equal(t, "down", failure.BucketState)
	assert.Zero(t, failure.LastRTT)
	assert.NotNil(t, observer.LoginSamples("proxy-failure"))
	assert.Len(t, observer.LoginSummaries(), 2)
	sessions := observer.SessionSummaries()
	require.Len(t, sessions, 1)
	assert.EqualValues(t, 601, sessions[0].SessionID)
}

// TestObserverFullEventPoolKeepsRinglessQualityAndDown fills the sole event
// ring, then establishes another login whose first Down must still be useful.
// MUTATION: making login quality conditional on a ring loses bounded-pool users.
func TestObserverFullEventPoolKeepsRinglessQualityAndDown(t *testing.T) {
	bus, observer, generation, _ := newObserverCQMGuardTest(t, 1, 2)
	owner := newObserverCQMGuardBirth(t)
	ringless := newObserverCQMGuardBirth(t)
	emitObserverCQMGuardEstablished(t, bus, owner, generation, 701, "alice", 11*time.Millisecond)
	beforeOwner := observer.SessionEvents(701)
	beforeOwnerQuality := observer.EchoState("alice")
	emitObserverCQMGuardEstablished(t, bus, ringless, generation, 702, "bob", 17*time.Millisecond)
	require.Empty(t, observer.SessionEvents(702), "the configured event pool is full")
	require.Equal(t, "established", observer.EchoState("bob").BucketState)
	require.Equal(t, 17*time.Millisecond, observer.EchoState("bob").LastRTT)
	require.Len(t, observer.LoginSummaries(), 2)

	emitObserverCQMGuardDown(t, bus, ringless, generation, 702, "bob")

	assert.Equal(t, "down", observer.EchoState("bob").BucketState,
		"the ringless selected contributor's first Down remains effective")
	assert.Equal(t, 17*time.Millisecond, observer.EchoState("bob").LastRTT)
	assert.Equal(t, beforeOwner, observer.SessionEvents(701))
	assert.Equal(t, beforeOwnerQuality, observer.EchoState("alice"))
	sessions := observer.SessionSummaries()
	require.Len(t, sessions, 1)
	assert.EqualValues(t, 701, sessions[0].SessionID)
}

// TestObserverLoginEvictionForgetsConsumedProvenance reclaims an ended login,
// then accepts its named Down as a fresh observation without a global tombstone.
// MUTATION: retaining consumed-Birth history beyond login eviction drops it.
func TestObserverLoginEvictionForgetsConsumedProvenance(t *testing.T) {
	bus, observer, generation, _ := newObserverCQMGuardTest(t, 3, 1)
	alice := newObserverCQMGuardBirth(t)
	bob := newObserverCQMGuardBirth(t)
	emitObserverCQMGuardEstablished(t, bus, alice, generation, 801, "alice", 11*time.Millisecond)
	emitObserverCQMGuardDown(t, bus, alice, generation, 801, "alice")
	require.Equal(t, "down", observer.EchoState("alice").BucketState)
	emitObserverCQMGuardEstablished(t, bus, bob, generation, 802, "bob", 13*time.Millisecond)
	require.Nil(t, observer.LoginSamples("alice"))
	require.Equal(t, "established", observer.EchoState("bob").BucketState)
	beforeBob := observer.SessionEvents(802)

	emitObserverCQMGuardDown(t, bus, alice, generation, 801, "alice")

	assert.Equal(t, beforeBob, observer.SessionEvents(802), "event custody is independent of login eviction")
	assert.Nil(t, observer.LoginSamples("bob"))
	fresh := observer.EchoState("alice")
	require.NotNil(t, fresh, "eviction forgets old Down provenance")
	assert.Equal(t, "down", fresh.BucketState)
	assert.Zero(t, fresh.LastRTT, "recreated quality cannot inherit the evicted echo")
	logins := observer.LoginSummaries()
	require.Len(t, logins, 1)
	assert.Equal(t, "alice", logins[0].Login)
	assert.Zero(t, logins[0].EchoCount)
}

// checkObserverCQMInvalidDelivery gives each typed callback its own valid
// neighbor, completed sample and native measurement before changing one fact.
func checkObserverCQMInvalidDelivery(t *testing.T,
	invalidate func(subscriberid.L2TPBirth, l2tpevents.ObservationGeneration) (
		subscriberid.L2TPBirth, l2tpevents.ObservationGeneration),
) {
	t.Helper()
	for _, delivery := range []struct {
		name string
		emit func(*testBus, subscriberid.L2TPBirth, l2tpevents.ObservationGeneration, string) error
	}{
		{name: "up", emit: func(bus *testBus, birth subscriberid.L2TPBirth,
			generation l2tpevents.ObservationGeneration, id string,
		) error {
			_, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
				Birth: birth, SubscriberID: id, Generation: generation,
				TunnelID: 17, SessionID: 902,
			})
			return err
		}},
		{name: "assignment", emit: func(bus *testBus, birth subscriberid.L2TPBirth,
			generation l2tpevents.ObservationGeneration, id string,
		) error {
			_, err := l2tpevents.SessionIPAssigned.Emit(bus, &l2tpevents.SessionIPAssignedPayload{
				Birth: birth, SubscriberID: id, Generation: generation,
				TunnelID: 17, SessionID: 902, Username: "malformed",
			})
			return err
		}},
		{name: "echo", emit: func(bus *testBus, birth subscriberid.L2TPBirth,
			generation l2tpevents.ObservationGeneration, id string,
		) error {
			_, err := l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
				Birth: birth, SubscriberID: id, Generation: generation,
				TunnelID: 17, SessionID: 901, Username: "alice", RTT: 99 * time.Millisecond,
			})
			return err
		}},
		{name: "down", emit: func(bus *testBus, birth subscriberid.L2TPBirth,
			generation l2tpevents.ObservationGeneration, id string,
		) error {
			_, err := l2tpevents.SessionDown.Emit(bus, &l2tpevents.SessionDownPayload{
				Birth: birth, SubscriberID: id, Generation: generation,
				TunnelID: 17, SessionID: 901, Username: "alice",
			})
			return err
		}},
	} {
		t.Run(delivery.name, func(t *testing.T) {
			bus, observer, generation, registry := newObserverCQMGuardTest(t, 3, 3)
			neighbor := newObserverCQMGuardBirth(t)
			emitObserverCQMGuardEstablished(t, bus, neighbor, generation, 901, "alice", 11*time.Millisecond)
			observer.mu.Lock()
			observer.logins["alice"].current.Start = time.Now().Add(-BucketInterval)
			observer.mu.Unlock()
			_, err := l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
				Birth: neighbor, SubscriberID: neighbor.String(), Generation: generation,
				TunnelID: 17, SessionID: 901, Username: "alice", RTT: 13 * time.Millisecond,
			})
			require.NoError(t, err)
			beforeHistory := observer.SessionEvents(901)
			require.Len(t, beforeHistory, 4)
			beforeSessions := observer.SessionSummaries()
			require.Len(t, beforeSessions, 1)
			beforeSamples := observer.LoginSamples("alice")
			require.Len(t, beforeSamples, 1)
			require.Equal(t, 11*time.Millisecond, beforeSamples[0].AvgRTT())
			beforeLogins := observer.LoginSummaries()
			beforeEcho := observer.EchoState("alice")
			require.Equal(t, "established", beforeEcho.BucketState)
			require.Equal(t, 13*time.Millisecond, beforeEcho.LastRTT)
			beforeMetrics := observerCQMMetrics(registry)
			require.Contains(t, beforeMetrics, "ze_l2tp_lcp_echo_rtt_seconds_count{username=\"alice\"} 1\n")
			// A wrongly admitted echo now finalizes another useful bucket.
			observer.mu.Lock()
			observer.logins["alice"].current.Start = time.Now().Add(-BucketInterval)
			observer.mu.Unlock()
			birth, scope := invalidate(neighbor, generation)

			require.NoError(t, delivery.emit(bus, birth, scope, neighbor.String()))
			assert.Equal(t, beforeHistory, observer.SessionEvents(901))
			assert.Equal(t, beforeSessions, observer.SessionSummaries())
			assert.Equal(t, beforeSamples, observer.LoginSamples("alice"))
			assert.Equal(t, beforeLogins, observer.LoginSummaries())
			assert.Equal(t, beforeEcho, observer.EchoState("alice"))
			assert.Equal(t, beforeMetrics, observerCQMMetrics(registry))
			assert.Empty(t, observer.SessionEvents(902))
			assert.Nil(t, observer.LoginSamples("malformed"))

			_, err = l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
				Birth: neighbor, SubscriberID: neighbor.String(), Generation: generation,
				TunnelID: 17, SessionID: 901, Username: "alice", RTT: 17 * time.Millisecond,
			})
			require.NoError(t, err)
			after := observer.SessionEvents(901)
			require.NotEmpty(t, after, "the permitted neighbor still appends its own event")
			assert.Equal(t, ObserverEventEchoRTT, after[len(after)-1].Type)
			assert.Equal(t, 17*time.Millisecond, after[len(after)-1].RTT)
			assert.Equal(t, "established", observer.EchoState("alice").BucketState)
			assert.Equal(t, 17*time.Millisecond, observer.EchoState("alice").LastRTT)
		})
	}
}

func newObserverCQMGuardTest(t *testing.T, maxSessions, maxLogins int) (
	*testBus, *Observer, l2tpevents.ObservationGeneration, *metrics.PrometheusRegistry,
) {
	t.Helper()
	registry := metrics.NewPrometheusRegistry()
	previousMetrics := l2tpMetricsPtr.Load()
	bindL2TPMetrics(registry)
	t.Cleanup(func() { l2tpMetricsPtr.Store(previousMetrics) })
	generation, err := l2tpevents.NewObservationGeneration()
	require.NoError(t, err)
	bus := newTestBus()
	cfg := testObserverConfig()
	cfg.MaxSessions = maxSessions
	cfg.MaxLogins = maxLogins
	cfg.EchoInterval = time.Second
	observer := newObserver(cfg)
	subsystem := &Subsystem{observer: observer}
	subsystem.wireObserverSubscriptions(bus)
	t.Cleanup(observer.Stop)
	return bus, observer, generation, registry
}

func newObserverCQMGuardBirth(t *testing.T) subscriberid.L2TPBirth {
	t.Helper()
	birth, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	return birth
}

func emitObserverCQMGuardEstablished(t *testing.T, bus *testBus, birth subscriberid.L2TPBirth,
	generation l2tpevents.ObservationGeneration, sid uint16, login string, rtt time.Duration,
) {
	t.Helper()
	_, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
		Birth: birth, SubscriberID: birth.String(), Generation: generation,
		TunnelID: 17, SessionID: sid,
	})
	require.NoError(t, err)
	_, err = l2tpevents.SessionIPAssigned.Emit(bus, &l2tpevents.SessionIPAssignedPayload{
		Birth: birth, SubscriberID: birth.String(), Generation: generation,
		TunnelID: 17, SessionID: sid, Username: login,
	})
	require.NoError(t, err)
	_, err = l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
		Birth: birth, SubscriberID: birth.String(), Generation: generation,
		TunnelID: 17, SessionID: sid, Username: login, RTT: rtt,
	})
	require.NoError(t, err)
}

func emitObserverCQMGuardDown(t *testing.T, bus *testBus, birth subscriberid.L2TPBirth,
	generation l2tpevents.ObservationGeneration, sid uint16, login string,
) {
	t.Helper()
	_, err := l2tpevents.SessionDown.Emit(bus, &l2tpevents.SessionDownPayload{
		Birth: birth, SubscriberID: birth.String(), Generation: generation,
		TunnelID: 17, SessionID: sid, Username: login,
	})
	require.NoError(t, err)
}
