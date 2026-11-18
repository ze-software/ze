package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

func testObserverConfig() observerConfig {
	return observerConfig{
		MaxSessions:   10,
		EventRingSize: 4,
		MaxLogins:     3,
		BucketCount:   36,
	}
}

func TestEventRingAppendAndWrap(t *testing.T) {
	r := newEventRing(3)
	require.Nil(t, r.snapshot())

	for i := range 5 {
		r.append(ObserverEvent{SessionID: uint16(i)})
	}
	snap := r.snapshot()
	require.Len(t, snap, 3)
	require.Equal(t, uint16(2), snap[0].SessionID)
	require.Equal(t, uint16(4), snap[2].SessionID)
}

func TestEventRingReset(t *testing.T) {
	r := newEventRing(4)
	r.append(ObserverEvent{SessionID: 1})
	r.reset()
	require.Nil(t, r.snapshot())
}

func TestEventRingRoutesBySessionID(t *testing.T) {
	obs, generation := newObserverFixture(t, testObserverConfig())
	first := newObserverFixtureBirth(t)
	second := newObserverFixtureBirth(t)
	now := time.Now()

	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: first, Timestamp: now, Type: ObserverEventSessionUp, SessionID: 100}, ""))
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: second, Timestamp: now, Type: ObserverEventSessionUp, SessionID: 200}, ""))
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: first, Timestamp: now, Type: ObserverEventEchoRTT, SessionID: 100}, ""))

	snap100 := obs.SessionEvents(first)
	snap200 := obs.SessionEvents(second)
	require.Len(t, snap100, 2)
	require.Len(t, snap200, 1)
	require.Equal(t, ObserverEventSessionUp, snap100[0].Type)
	require.Equal(t, ObserverEventEchoRTT, snap100[1].Type)
}

func TestObserverPoolPreallocation(t *testing.T) {
	cfg := testObserverConfig()
	obs, generation := newObserverFixture(t, cfg)
	births := make([]subscriberid.L2TPBirth, cfg.MaxSessions+1)
	for i := range births {
		births[i] = newObserverFixtureBirth(t)
	}

	// All pool slots should be available
	for i := range cfg.MaxSessions {
		require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: births[i], Type: ObserverEventSessionUp, SessionID: uint16(i + 1)}, ""))
	}
	// One more should be silently dropped (pool exhausted)
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: births[cfg.MaxSessions], Type: ObserverEventSessionUp, SessionID: uint16(cfg.MaxSessions + 1)}, ""))
	require.Nil(t, obs.SessionEvents(births[cfg.MaxSessions]))
}

func TestObserverPoolReturnAndReuse(t *testing.T) {
	cfg := observerConfig{MaxSessions: 1, EventRingSize: 4, MaxLogins: 1, BucketCount: 10}
	obs, generation := newObserverFixture(t, cfg)
	first := newObserverFixtureBirth(t)
	second := newObserverFixtureBirth(t)

	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: first, Type: ObserverEventSessionUp, SessionID: 1}, ""))
	require.Len(t, obs.SessionEvents(first), 1)

	obs.mu.Lock()
	obs.releaseSessionLocked(first)
	obs.mu.Unlock()
	require.Nil(t, obs.SessionEvents(first))

	// Pool should have the buffer back
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: second, Type: ObserverEventSessionUp, SessionID: 2}, ""))
	require.Len(t, obs.SessionEvents(second), 1)
}

func TestObserverLRUEviction(t *testing.T) {
	cfg := observerConfig{MaxSessions: 10, EventRingSize: 4, MaxLogins: 2, BucketCount: 10}
	obs, generation := newObserverFixture(t, cfg)
	alice := newObserverFixtureBirth(t)
	bob := newObserverFixtureBirth(t)
	carol := newObserverFixtureBirth(t)
	now := time.Now()

	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: alice, Timestamp: now, Type: ObserverEventEchoRTT, SessionID: 1, RTT: 10 * time.Millisecond}, "alice"))
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: bob, Timestamp: now.Add(time.Second), Type: ObserverEventEchoRTT, SessionID: 2, RTT: 20 * time.Millisecond}, "bob"))

	// Both should exist
	require.NotNil(t, obs.LoginSamples("alice"))
	require.NotNil(t, obs.LoginSamples("bob"))

	// Adding a third login should evict alice (LRU -- bob was touched more recently)
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: carol, Timestamp: now.Add(2 * time.Second), Type: ObserverEventEchoRTT, SessionID: 3, RTT: 30 * time.Millisecond}, "carol"))
	require.Nil(t, obs.LoginSamples("alice"))
	require.NotNil(t, obs.LoginSamples("bob"))
	require.NotNil(t, obs.LoginSamples("carol"))
}

func TestLoginContinuity(t *testing.T) {
	cfg := testObserverConfig()
	obs, generation := newObserverFixture(t, cfg)
	first := newObserverFixtureBirth(t)
	second := newObserverFixtureBirth(t)
	now := time.Now()

	// First session for "alice"; Echo before Up also records its event.
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: first, SessionID: 100, Type: ObserverEventEchoRTT, Timestamp: now, RTT: 10 * time.Millisecond}, "alice"))
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: first, SessionID: 100, Type: ObserverEventSessionUp, Timestamp: now}, ""))

	// Session ends, event ring released.
	obs.mu.Lock()
	obs.releaseSessionLocked(first)
	obs.mu.Unlock()
	require.Nil(t, obs.SessionEvents(first))

	// The replacement's first Echo creates a fresh history before its Up.
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: second, SessionID: 200, Type: ObserverEventEchoRTT, Timestamp: now.Add(time.Second), RTT: 20 * time.Millisecond}, "alice"))
	snap200 := obs.SessionEvents(second)
	require.Len(t, snap200, 1)
	require.Equal(t, ObserverEventEchoRTT, snap200[0].Type)
	require.Equal(t, 20*time.Millisecond, snap200[0].RTT)
	require.Equal(t, second, snap200[0].Birth)
	require.Equal(t, uint16(200), snap200[0].SessionID)
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: second, SessionID: 200, Type: ObserverEventSessionUp, Timestamp: now.Add(time.Second)}, ""))

	// Login samples continue while both replacement events stay independent.
	require.NotNil(t, obs.LoginSamples("alice"))
	snap200 = obs.SessionEvents(second)
	require.Len(t, snap200, 2)
	require.Equal(t, ObserverEventEchoRTT, snap200[0].Type)
	require.Equal(t, ObserverEventSessionUp, snap200[1].Type)
	require.Equal(t, 20*time.Millisecond, snap200[0].RTT)
	require.Equal(t, second, snap200[0].Birth)
	require.Equal(t, uint16(200), snap200[0].SessionID)
	require.Equal(t, second, snap200[1].Birth)
	require.Equal(t, uint16(200), snap200[1].SessionID)
}

func TestCQMBucketBoundary(t *testing.T) {
	cfg := observerConfig{MaxSessions: 1, EventRingSize: 4, MaxLogins: 1, BucketCount: 10}
	obs, generation := newObserverFixture(t, cfg)
	alice := newObserverFixtureBirth(t)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Echoes within first bucket
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: alice, Timestamp: start, Type: ObserverEventEchoRTT, SessionID: 1, RTT: 10 * time.Millisecond}, "alice"))
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: alice, Timestamp: start.Add(50 * time.Second), Type: ObserverEventEchoRTT, SessionID: 1, RTT: 20 * time.Millisecond}, "alice"))

	// No closed buckets yet (still within 100s)
	require.Empty(t, obs.LoginSamples("alice"))

	// Echo after bucket boundary triggers close
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: alice, Timestamp: start.Add(101 * time.Second), Type: ObserverEventEchoRTT, SessionID: 1, RTT: 30 * time.Millisecond}, "alice"))

	snap := obs.LoginSamples("alice")
	require.Len(t, snap, 1)
	require.Equal(t, uint64(2), snap[0].EchoCount)
	require.Equal(t, 10*time.Millisecond, snap[0].MinRTT)
	require.Equal(t, 20*time.Millisecond, snap[0].MaxRTT)
}

func TestCQMBucketStateTag(t *testing.T) {
	cfg := observerConfig{MaxSessions: 1, EventRingSize: 4, MaxLogins: 1, BucketCount: 10}
	obs, generation := newObserverFixture(t, cfg)
	alice := newObserverFixtureBirth(t)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: alice, Timestamp: start, Type: ObserverEventEchoRTT, SessionID: 1, RTT: 10 * time.Millisecond}, "alice"))
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: alice, Timestamp: start, Type: ObserverEventSessionIPAssigned, SessionID: 1}, "alice"))

	// Close bucket
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: alice, Timestamp: start.Add(101 * time.Second), Type: ObserverEventEchoRTT, SessionID: 1, RTT: 20 * time.Millisecond}, "alice"))

	snap := obs.LoginSamples("alice")
	require.Len(t, snap, 1)
	// State was negotiating when bucket opened, then changed to established.
	// The admitted assignment updates the state of the current bucket.
	require.Equal(t, BucketStateEstablished, snap[0].State)
}

func TestObserverEventTypeString(t *testing.T) {
	require.Equal(t, "tunnel-up", ObserverEventTunnelUp.String())
	require.Equal(t, "session-down", ObserverEventSessionDown.String())
	require.Equal(t, "unknown", ObserverEventType(0).String())
	// Zero is the empty record; any other unnamed event type is an internal defect.
	defer func() {
		if got := recover(); got != "BUG: unknown L2TP observer event type" {
			t.Fatalf("unexpected panic for an unnamed observer event type: %v", got)
		}
	}()
	_ = ObserverEventType(99).String()
}

func TestObserverEventSnapshot(t *testing.T) {
	obs, generation := newObserverFixture(t, testObserverConfig())
	birth := newObserverFixtureBirth(t)
	now := time.Now()

	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: birth,
		Timestamp: now, Type: ObserverEventSessionUp,
		SessionID: 42, TunnelID: 1,
	}, ""))
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: birth,
		Timestamp: now.Add(time.Second), Type: ObserverEventEchoRTT,
		SessionID: 42, TunnelID: 1, RTT: 5 * time.Millisecond,
	}, ""))

	events := obs.SessionEvents(birth)
	require.Len(t, events, 2)
	require.Equal(t, ObserverEventSessionUp, events[0].Type)
	require.Equal(t, ObserverEventEchoRTT, events[1].Type)
	require.Equal(t, uint16(42), events[0].SessionID)
	require.Equal(t, 5*time.Millisecond, events[1].RTT)
}

func TestObserverSnapshotNonExistent(t *testing.T) {
	obs, _ := newObserverFixture(t, testObserverConfig())
	unknown := newObserverFixtureBirth(t)
	require.Nil(t, obs.SessionEvents(unknown))
}

func TestSessionSummaries(t *testing.T) {
	obs, generation := newObserverFixture(t, testObserverConfig())
	first := newObserverFixtureBirth(t)
	second := newObserverFixtureBirth(t)
	now := time.Now()

	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: first, Timestamp: now, Type: ObserverEventSessionUp, SessionID: 10}, ""))
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: second, Timestamp: now, Type: ObserverEventSessionUp, SessionID: 20}, ""))
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: first, Timestamp: now.Add(time.Second), Type: ObserverEventEchoRTT, SessionID: 10}, ""))

	summaries := obs.SessionSummaries()
	require.Len(t, summaries, 2)

	byID := map[uint16]SessionSummary{}
	for _, s := range summaries {
		byID[s.SessionID] = s
	}
	require.Equal(t, 2, byID[10].EventCount)
	require.Equal(t, "echo-rtt", byID[10].LastEventType)
	require.Equal(t, 1, byID[20].EventCount)
	require.Equal(t, "session-up", byID[20].LastEventType)
}

func TestLoginSummaries(t *testing.T) {
	cfg := testObserverConfig()
	cfg.EchoInterval = 10 * time.Second
	obs, generation := newObserverFixture(t, cfg)
	alice := newObserverFixtureBirth(t)
	bob := newObserverFixtureBirth(t)
	now := time.Now()

	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: alice, Timestamp: now, Type: ObserverEventEchoRTT, SessionID: 1, RTT: 10 * time.Millisecond}, "alice"))
	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: bob, Timestamp: now, Type: ObserverEventEchoRTT, SessionID: 2, RTT: 20 * time.Millisecond}, "bob"))

	summaries := obs.LoginSummaries()
	require.Len(t, summaries, 2)

	byLogin := map[string]LoginSummary{}
	for _, s := range summaries {
		byLogin[s.Login] = s
	}
	require.Equal(t, uint64(1), byLogin["alice"].EchoCount)
	require.Equal(t, uint64(1), byLogin["bob"].EchoCount)
}

func TestEchoState(t *testing.T) {
	cfg := testObserverConfig()
	cfg.EchoInterval = 10 * time.Second
	obs, generation := newObserverFixture(t, cfg)
	alice := newObserverFixtureBirth(t)
	now := time.Now()

	require.NoError(t, obs.observeEvent(generation, ObserverEvent{Birth: alice, Timestamp: now, Type: ObserverEventEchoRTT, SessionID: 1, RTT: 15 * time.Millisecond}, "alice"))

	state := obs.EchoState("alice")
	require.NotNil(t, state)
	require.Equal(t, "alice", state.Login)
	require.Equal(t, 15*time.Millisecond, state.LastRTT)
	require.Equal(t, 10*time.Second, state.EchoInterval)

	require.Nil(t, obs.EchoState("nonexistent"))
}

// newObserverFixture binds one explicit original observation generation before
// constructing the observer. The caller supplies every event's original Birth.
func newObserverFixture(t *testing.T, cfg observerConfig) (*Observer, l2tpevents.ObservationGeneration) {
	t.Helper()
	generation, err := l2tpevents.NewObservationGeneration()
	require.NoError(t, err)
	cfg.Generation = generation
	observer, err := newObserver(cfg)
	require.NoError(t, err)
	t.Cleanup(observer.Stop)
	return observer, generation
}

func newObserverFixtureBirth(t *testing.T) subscriberid.L2TPBirth {
	t.Helper()
	birth, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	return birth
}
