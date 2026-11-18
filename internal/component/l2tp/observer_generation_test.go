// Design: docs/architecture/l2tp/l2tp-9-observer.md -- original delivery scope and exact history

package l2tp

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

func cqmAPIGeneration(t *testing.T) l2tpevents.ObservationGeneration {
	t.Helper()
	generation, err := l2tpevents.NewObservationGeneration()
	require.NoError(t, err)
	return generation
}

func cqmAPIBirth(t *testing.T) subscriberid.L2TPBirth {
	t.Helper()
	birth, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	return birth
}

func cqmAPIObserver(t *testing.T, generation l2tpevents.ObservationGeneration, binding *l2tpMetrics, sessions, logins int) *Observer {
	t.Helper()
	observer, err := newObserver(observerConfig{
		Generation: generation, Metrics: binding, MaxSessions: sessions,
		EventRingSize: 16, MaxLogins: logins, BucketCount: 4, EchoInterval: time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(observer.Stop)
	return observer
}

func cqmAPIEvent(birth subscriberid.L2TPBirth, kind ObserverEventType, at time.Time, rtt time.Duration) ObserverEvent {
	return ObserverEvent{Birth: birth, Type: kind, Timestamp: at, TunnelID: 17, SessionID: 7, RTT: rtt}
}

func cqmAPIEstablished(t *testing.T, observer *Observer, generation l2tpevents.ObservationGeneration, birth subscriberid.L2TPBirth, login string, at time.Time) {
	t.Helper()
	require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth, ObserverEventSessionUp, at, 0), ""))
	require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth, ObserverEventSessionIPAssigned, at, 0), login))
	require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth, ObserverEventEchoRTT, at, 11*time.Millisecond), login))
}

// MUTATION: accepting absent construction scope creates an observer that cannot
// distinguish the original native root from a callback copied before restart.
func TestObserverConstructorRequiresOriginalGeneration(t *testing.T) {
	observer, err := newObserver(observerConfig{MaxSessions: 1, EventRingSize: 2})
	require.Error(t, err)
	require.Nil(t, observer)
	generation := cqmAPIGeneration(t)
	valid := cqmAPIObserver(t, generation, nil, 1, 1)
	birth := cqmAPIBirth(t)
	cqmAPIEstablished(t, valid, generation, birth, "alice", time.Unix(1000, 0))
	require.Len(t, valid.SessionEvents(birth), 3)
	require.Equal(t, "established", valid.EchoState("alice").BucketState)
}

// TestObserverConstructorRefusesInvalidEnabledPools checks bounded construction
// before ring allocation and preserves explicitly disabled zero-capacity pools.
// MUTATION: admitting a zero ring for an enabled pool panics on its first event.
func TestObserverConstructorRefusesInvalidEnabledPools(t *testing.T) {
	generation := cqmAPIGeneration(t)
	for _, config := range []observerConfig{
		{Generation: generation, MaxSessions: -1},
		{Generation: generation, MaxLogins: -1},
		{Generation: generation, EventRingSize: -1},
		{Generation: generation, BucketCount: -1},
		{Generation: generation, MaxSessions: 1},
		{Generation: generation, MaxLogins: 1},
	} {
		observer, err := newObserver(config)
		require.Error(t, err)
		require.Nil(t, observer)
	}
	disabled, err := newObserver(observerConfig{Generation: generation})
	require.NoError(t, err)
	t.Cleanup(disabled.Stop)
	cqmAPIEstablished(t, disabled, generation, cqmAPIBirth(t), "alice", time.Unix(1000, 0))
	require.Empty(t, disabled.SessionSummaries())
	require.Empty(t, disabled.LoginSummaries())
}

// MUTATION: removing the generation comparison admits a foreign payload even
// when it carries a valid Birth and identical readable wire coordinates.
func TestObserverForeignGenerationHasNoEffectOnUsefulOriginal(t *testing.T) {
	generation, foreign := cqmAPIGeneration(t), cqmAPIGeneration(t)
	observer := cqmAPIObserver(t, generation, nil, 2, 2)
	birth := cqmAPIBirth(t)
	at := time.Unix(1000, 0)
	cqmAPIEstablished(t, observer, generation, birth, "alice", at)
	beforeEvents, beforeLogins := observer.SessionEvents(birth), observer.LoginSummaries()
	beforeEcho := observer.EchoState("alice")
	for _, kind := range []ObserverEventType{ObserverEventSessionUp, ObserverEventSessionIPAssigned, ObserverEventEchoRTT, ObserverEventSessionDown} {
		require.NoError(t, observer.observeEvent(foreign, cqmAPIEvent(birth, kind, at.Add(BucketInterval), 97*time.Millisecond), "alice"))
	}
	require.Equal(t, beforeEvents, observer.SessionEvents(birth))
	require.Equal(t, beforeLogins, observer.LoginSummaries())
	require.Equal(t, beforeEcho, observer.EchoState("alice"))
	require.Empty(t, observer.LoginSamples("alice"))
	require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth, ObserverEventEchoRTT, at.Add(BucketInterval), 13*time.Millisecond), "alice"))
	require.Len(t, observer.LoginSamples("alice"), 1, "the original scope still finalizes a useful bucket")
}

// TestObserverBusKeepsOverlappingOriginalScopesApart publishes equal readable
// labels and one login through two real native subscriptions on the same bus.
// MUTATION: accepting another native scope duplicates quality in both roots.
func TestObserverBusKeepsOverlappingOriginalScopesApart(t *testing.T) {
	bus := newCQMNativeBus(t)
	older := cqmAPIObserver(t, cqmAPIGeneration(t), nil, 2, 2)
	newer := cqmAPIObserver(t, cqmAPIGeneration(t), nil, 2, 2)
	for _, observer := range []*Observer{older, newer} {
		(&Subsystem{observer: observer}).wireObserverSubscriptions(bus)
	}
	oldBirth, newBirth := cqmAPIBirth(t), cqmAPIBirth(t)
	for _, original := range []struct {
		observer *Observer
		birth    subscriberid.L2TPBirth
	}{{older, oldBirth}, {newer, newBirth}} {
		_, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
			Birth: original.birth, SubscriberID: original.birth.String(), Generation: original.observer.generation,
			TunnelID: 17, SessionID: 7,
		})
		require.NoError(t, err)
		_, err = l2tpevents.SessionIPAssigned.Emit(bus, &l2tpevents.SessionIPAssignedPayload{
			Birth: original.birth, SubscriberID: original.birth.String(), Generation: original.observer.generation,
			TunnelID: 17, SessionID: 7, Username: "alice",
		})
		require.NoError(t, err)
		_, err = l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
			Birth: original.birth, SubscriberID: original.birth.String(), Generation: original.observer.generation,
			TunnelID: 17, SessionID: 7, Username: "alice", RTT: 11 * time.Millisecond,
		})
		require.NoError(t, err)
	}
	require.Len(t, older.SessionEvents(oldBirth), 3)
	require.Nil(t, older.SessionEvents(newBirth))
	require.Len(t, newer.SessionEvents(newBirth), 3)
	require.Nil(t, newer.SessionEvents(oldBirth))
	require.EqualValues(t, 1, older.LoginSummaries()[0].EchoCount)
	require.EqualValues(t, 1, newer.LoginSummaries()[0].EchoCount)
	older.RetainKnownSessions()
	beforeCurrent := newer.LoginSummaries()
	_, err := l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
		Birth: oldBirth, SubscriberID: oldBirth.String(), Generation: older.generation,
		TunnelID: 17, SessionID: 7, Username: "alice", RTT: 17 * time.Millisecond,
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, older.LoginSummaries()[0].EchoCount)
	require.Equal(t, beforeCurrent, newer.LoginSummaries())
	require.Len(t, older.SessionEvents(oldBirth), 4)
	require.Len(t, newer.SessionEvents(newBirth), 3)
}

// MUTATION: selecting history or releasing a ring by wire IDs returns or
// destroys another original whose TID and SID happen to be equal.
func TestObserverExactBirthHistoryAndJSONSurviveEqualCoordinates(t *testing.T) {
	generation := cqmAPIGeneration(t)
	observer := cqmAPIObserver(t, generation, nil, 2, 2)
	first, second, absent := cqmAPIBirth(t), cqmAPIBirth(t), cqmAPIBirth(t)
	at := time.Unix(1000, 0)
	cqmAPIEstablished(t, observer, generation, first, "alice", at)
	cqmAPIEstablished(t, observer, generation, second, "bob", at.Add(time.Second))
	for _, birth := range []subscriberid.L2TPBirth{first, second} {
		history := observer.SessionEvents(birth)
		require.Len(t, history, 3)
		for _, event := range history {
			require.Equal(t, birth, event.Birth)
			encoded, err := json.Marshal(event)
			require.NoError(t, err)
			require.Contains(t, string(encoded), `"subscriber-id":"`+birth.String()+`"`)
		}
	}
	require.Nil(t, observer.SessionEvents(absent))
	require.Len(t, observer.SessionSummaries(), 2, "unknown reads allocate no ring")
	for _, summary := range observer.SessionSummaries() {
		encoded, err := json.Marshal(summary)
		require.NoError(t, err)
		require.Contains(t, string(encoded), `"subscriber-id":"`+summary.Birth.String()+`"`)
		require.Contains(t, string(encoded), `"tunnel-id":17`)
	}
	before := observer.SessionEvents(second)
	observer.mu.Lock()
	observer.releaseSessionLocked(first)
	observer.mu.Unlock()
	require.Empty(t, observer.SessionEvents(first))
	require.Equal(t, before, observer.SessionEvents(second))
	encoded, err := json.Marshal(ObserverEvent{})
	require.NoError(t, err, "intentional empty formatting DTO omits the absent identity")
	require.NotContains(t, string(encoded), "subscriber-id")
}

// MUTATION: permitting a new lifecycle Birth through a retained seal consumes
// bounded history; refusing all ringless Down loses the selected login phase.
func TestObserverRetainedSealKeepsKnownHistoryAndRinglessSelectedDown(t *testing.T) {
	generation := cqmAPIGeneration(t)
	observer := cqmAPIObserver(t, generation, nil, 1, 2)
	known, selected, foreign := cqmAPIBirth(t), cqmAPIBirth(t), cqmAPIBirth(t)
	at := time.Unix(1000, 0)
	cqmAPIEstablished(t, observer, generation, known, "blocker", at)
	cqmAPIEstablished(t, observer, generation, selected, "alice", at)
	require.Nil(t, observer.SessionEvents(selected), "quality may be useful after the event pool is full")
	observer.RetainKnownSessions()
	before := observer.SessionSummaries()
	for _, kind := range []ObserverEventType{ObserverEventSessionUp, ObserverEventSessionIPAssigned, ObserverEventEchoRTT, ObserverEventSessionDown} {
		require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(foreign, kind, at, time.Second), "mallory"))
	}
	require.Equal(t, before, observer.SessionSummaries())
	require.Nil(t, observer.EchoState("mallory"))
	require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(known, ObserverEventEchoRTT, at, 17*time.Millisecond), "blocker"))
	require.Len(t, observer.SessionEvents(known), 4)
	require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(selected, ObserverEventSessionDown, at, 0), "alice"))
	require.Equal(t, "down", observer.EchoState("alice").BucketState)
	require.Nil(t, observer.SessionEvents(selected), "the ringless exception cannot allocate")
	beforeLogins := observer.LoginSummaries()
	require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(selected, ObserverEventSessionDown, at, 0), "alice"))
	require.Equal(t, beforeLogins, observer.LoginSummaries())
}

// cqmAPIBus gates the actual server subscription after dispatch copied it.
// The wrapper owns only its four subscriptions, not a server or native worker.
type cqmAPIBus struct {
	*pluginserver.Server
	mu       sync.Mutex
	active   int
	echoGate func()
}

func newCQMNativeBus(t *testing.T) *cqmAPIBus {
	t.Helper()
	server, err := pluginserver.NewServer(nil, nil)
	require.NoError(t, err)
	return &cqmAPIBus{Server: server}
}

func (bus *cqmAPIBus) Subscribe(namespace, event string, handler func(any)) func() {
	bus.mu.Lock()
	bus.active++
	bus.mu.Unlock()
	unsub := bus.Server.Subscribe(namespace, event, func(payload any) {
		bus.mu.Lock()
		gate := bus.echoGate
		bus.mu.Unlock()
		if event == l2tpevents.EchoRTTEvent {
			if gate != nil {
				gate()
			}
		}
		handler(payload)
	})
	return sync.OnceFunc(func() {
		unsub()
		bus.mu.Lock()
		bus.active--
		bus.mu.Unlock()
	})
}

// MUTATION: an unsubscribe without closed admission lets an already copied
// real server callback alter readonly history, samples and login quality.
func TestObserverStopClosesActualServerCopiedCallback(t *testing.T) {
	generation := cqmAPIGeneration(t)
	observer := cqmAPIObserver(t, generation, nil, 2, 2)
	bus := newCQMNativeBus(t)
	(&Subsystem{observer: observer}).wireObserverSubscriptions(bus)
	birth := cqmAPIBirth(t)
	cqmAPIEstablished(t, observer, generation, birth, "alice", time.Now())
	beforeEvents, beforeLogins := observer.SessionEvents(birth), observer.LoginSummaries()
	beforeSamples, beforeEcho := observer.LoginSamples("alice"), observer.EchoState("alice")
	entered, resume, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(resume) })
	bus.mu.Lock()
	bus.echoGate = func() { close(entered); <-resume }
	bus.mu.Unlock()
	deliveryErrors := make(chan error, 1)
	t.Cleanup(func() {
		release()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("copied callback did not join")
		}
	})
	go func() {
		defer close(done)
		_, err := l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
			Birth: birth, SubscriberID: birth.String(), Generation: generation,
			TunnelID: 17, SessionID: 7, Username: "alice", RTT: 97 * time.Millisecond,
		})
		deliveryErrors <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("actual server callback was not copied")
	}
	observer.Stop()
	observer.Stop()
	bus.mu.Lock()
	active := bus.active
	bus.mu.Unlock()
	require.Zero(t, active, "all four original subscriptions are released exactly once")
	release()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("copied callback did not return")
	}
	require.NoError(t, <-deliveryErrors)
	require.NoError(t, observer.observeEvent(generation, cqmAPIEvent(birth, ObserverEventSessionDown, time.Now(), 0), "alice"))
	require.Equal(t, beforeEvents, observer.SessionEvents(birth))
	require.Equal(t, beforeLogins, observer.LoginSummaries())
	require.Equal(t, beforeSamples, observer.LoginSamples("alice"))
	require.Equal(t, beforeEcho, observer.EchoState("alice"))
}
