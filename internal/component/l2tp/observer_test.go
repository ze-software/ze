package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testObserverConfig() ObserverConfig {
	return ObserverConfig{
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
	obs := NewObserver(testObserverConfig())
	now := time.Now()

	obs.RecordEvent(ObserverEvent{Timestamp: now, Type: ObserverEventSessionUp, SessionID: 100})
	obs.RecordEvent(ObserverEvent{Timestamp: now, Type: ObserverEventSessionUp, SessionID: 200})
	obs.RecordEvent(ObserverEvent{Timestamp: now, Type: ObserverEventEchoRTT, SessionID: 100})

	snap100 := obs.SessionEvents(100)
	snap200 := obs.SessionEvents(200)
	require.Len(t, snap100, 2)
	require.Len(t, snap200, 1)
	require.Equal(t, ObserverEventSessionUp, snap100[0].Type)
	require.Equal(t, ObserverEventEchoRTT, snap100[1].Type)
}

func TestObserverPoolPreallocation(t *testing.T) {
	cfg := testObserverConfig()
	obs := NewObserver(cfg)

	// All pool slots should be available
	for i := range cfg.MaxSessions {
		obs.RecordEvent(ObserverEvent{SessionID: uint16(i + 1)})
	}
	// One more should be silently dropped (pool exhausted)
	obs.RecordEvent(ObserverEvent{SessionID: uint16(cfg.MaxSessions + 1)})
	require.Nil(t, obs.SessionEvents(uint16(cfg.MaxSessions+1)))
}

func TestObserverPoolReturnAndReuse(t *testing.T) {
	cfg := ObserverConfig{MaxSessions: 1, EventRingSize: 4, MaxLogins: 1, BucketCount: 10}
	obs := NewObserver(cfg)

	obs.RecordEvent(ObserverEvent{SessionID: 1})
	require.Len(t, obs.SessionEvents(1), 1)

	obs.ReleaseSession(1)
	require.Nil(t, obs.SessionEvents(1))

	// Pool should have the buffer back
	obs.RecordEvent(ObserverEvent{SessionID: 2})
	require.Len(t, obs.SessionEvents(2), 1)
}

func TestObserverLRUEviction(t *testing.T) {
	cfg := ObserverConfig{MaxSessions: 10, EventRingSize: 4, MaxLogins: 2, BucketCount: 10}
	obs := NewObserver(cfg)
	now := time.Now()

	obs.RecordEcho("alice", now, 10*time.Millisecond)
	obs.RecordEcho("bob", now.Add(time.Second), 20*time.Millisecond)

	// Both should exist
	require.NotNil(t, obs.LoginSamples("alice"))
	require.NotNil(t, obs.LoginSamples("bob"))

	// Adding a third login should evict alice (LRU -- bob was touched more recently)
	obs.RecordEcho("carol", now.Add(2*time.Second), 30*time.Millisecond)
	require.Nil(t, obs.LoginSamples("alice"))
	require.NotNil(t, obs.LoginSamples("bob"))
	require.NotNil(t, obs.LoginSamples("carol"))
}

func TestLoginContinuity(t *testing.T) {
	cfg := testObserverConfig()
	obs := NewObserver(cfg)
	now := time.Now()

	// First session for "alice"
	obs.RecordEcho("alice", now, 10*time.Millisecond)
	obs.RecordEvent(ObserverEvent{SessionID: 100, Type: ObserverEventSessionUp})

	// Session ends, event ring released
	obs.ReleaseSession(100)
	require.Nil(t, obs.SessionEvents(100))

	// Same login reconnects with new SID
	obs.RecordEcho("alice", now.Add(time.Second), 20*time.Millisecond)
	obs.RecordEvent(ObserverEvent{SessionID: 200, Type: ObserverEventSessionUp})

	// Sample ring is continuous (login-keyed)
	require.NotNil(t, obs.LoginSamples("alice"))

	// Event ring is fresh (session-keyed)
	snap200 := obs.SessionEvents(200)
	require.Len(t, snap200, 1)
}

func TestCQMBucketBoundary(t *testing.T) {
	cfg := ObserverConfig{MaxSessions: 1, EventRingSize: 4, MaxLogins: 1, BucketCount: 10}
	obs := NewObserver(cfg)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Echoes within first bucket
	obs.RecordEcho("alice", start, 10*time.Millisecond)
	obs.RecordEcho("alice", start.Add(50*time.Second), 20*time.Millisecond)

	// No closed buckets yet (still within 100s)
	require.Empty(t, obs.LoginSamples("alice"))

	// Echo after bucket boundary triggers close
	obs.RecordEcho("alice", start.Add(101*time.Second), 30*time.Millisecond)

	snap := obs.LoginSamples("alice")
	require.Len(t, snap, 1)
	require.Equal(t, uint16(2), snap[0].EchoCount)
	require.Equal(t, 10*time.Millisecond, snap[0].MinRTT)
	require.Equal(t, 20*time.Millisecond, snap[0].MaxRTT)
}

func TestCQMBucketStateTag(t *testing.T) {
	cfg := ObserverConfig{MaxSessions: 1, EventRingSize: 4, MaxLogins: 1, BucketCount: 10}
	obs := NewObserver(cfg)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	obs.RecordEcho("alice", start, 10*time.Millisecond)
	obs.SetLoginState("alice", BucketStateEstablished)

	// Close bucket
	obs.RecordEcho("alice", start.Add(101*time.Second), 20*time.Millisecond)

	snap := obs.LoginSamples("alice")
	require.Len(t, snap, 1)
	// State was negotiating when bucket opened, then changed to established.
	// The last-set state wins because SetLoginState updates current.State.
	require.Equal(t, BucketStateEstablished, snap[0].State)
}

func TestObserverEventTypeString(t *testing.T) {
	require.Equal(t, "tunnel-up", ObserverEventTunnelUp.String())
	require.Equal(t, "session-down", ObserverEventSessionDown.String())
	require.Equal(t, "unknown", ObserverEventType(0).String())
}
