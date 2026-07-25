// Design: docs/architecture/chaos-web-dashboard.md -- orchestrator type and event processor tests

package orchestrator

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/validation"
)

func TestEstablishedStateSetAndSnapshot(t *testing.T) {
	es := NewEstablishedState(4)

	snap := es.Snapshot()
	require.Len(t, snap, 4)
	assert.Equal(t, []bool{false, false, false, false}, snap)

	es.Set(0, true)
	es.Set(2, true)

	snap = es.Snapshot()
	assert.Equal(t, []bool{true, false, true, false}, snap)

	snap[0] = false
	assert.Equal(t, []bool{true, false, true, false}, es.Snapshot())
}

func TestEstablishedStateSetFalse(t *testing.T) {
	es := NewEstablishedState(3)
	es.Set(1, true)
	assert.Equal(t, []bool{false, true, false}, es.Snapshot())

	es.Set(1, false)
	assert.Equal(t, []bool{false, false, false}, es.Snapshot())
}

func TestEstablishedStateConcurrent(t *testing.T) {
	es := NewEstablishedState(10)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for range 100 {
				es.Set(idx, true)
				_ = es.Snapshot()
				es.Set(idx, false)
			}
		}(i)
	}
	wg.Wait()

	snap := es.Snapshot()
	require.Len(t, snap, 10)
	for i, v := range snap {
		assert.False(t, v, "peer %d should be false after concurrent toggle", i)
	}
}

func TestChaosConfigZeroRate(t *testing.T) {
	cfg := ChaosConfig{
		Rate:     0,
		Interval: 10 * time.Second,
		Warmup:   5 * time.Second,
	}
	assert.Equal(t, 0.0, cfg.Rate)
}

func TestOrchestratorEventProcessing(t *testing.T) {
	m := validation.NewModel(3)
	tr := validation.NewTracker(3)
	conv := validation.NewConvergence(3, 5*time.Second)

	ep := &EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	now := time.Now()

	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 2, Time: now})

	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: prefix})

	recvTime := now.Add(50 * time.Millisecond)
	ep.Process(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: recvTime, Prefix: prefix})

	assert.True(t, m.Expected(1).Contains(prefix))
	assert.True(t, m.Expected(2).Contains(prefix))

	assert.True(t, tr.ActualRoutes(1).Contains(prefix))

	stats := conv.Stats()
	assert.Equal(t, 1, stats.Resolved)
	assert.Equal(t, 1, stats.Pending)
}

func TestOrchestratorWithdrawal(t *testing.T) {
	m := validation.NewModel(2)
	tr := validation.NewTracker(2)
	conv := validation.NewConvergence(2, 5*time.Second)

	ep := &EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	now := time.Now()

	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: prefix})
	ep.Process(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: now.Add(10 * time.Millisecond), Prefix: prefix})

	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now.Add(100 * time.Millisecond), Prefix: prefix})
	ep.Process(peer.Event{Type: peer.EventRouteWithdrawn, PeerIndex: 1, Time: now.Add(200 * time.Millisecond), Prefix: prefix})

	assert.False(t, tr.ActualRoutes(1).Contains(prefix))
}

func TestOrchestratorDisconnect(t *testing.T) {
	m := validation.NewModel(3)
	tr := validation.NewTracker(3)
	conv := validation.NewConvergence(3, 5*time.Second)

	ep := &EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	now := time.Now()

	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 2, Time: now})
	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: prefix})

	ep.Process(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: now.Add(time.Second)})

	assert.Equal(t, 0, m.Expected(1).Len())
	assert.Equal(t, 0, m.Expected(2).Len())

	assert.Equal(t, 0, tr.ActualRoutes(0).Len())
}

func TestOrchestratorCounters(t *testing.T) {
	m := validation.NewModel(2)
	tr := validation.NewTracker(2)
	conv := validation.NewConvergence(2, 5*time.Second)

	ep := &EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	now := time.Now()
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})

	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: netip.MustParsePrefix("10.0.0.0/24")})
	ep.Process(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now, Prefix: netip.MustParsePrefix("10.0.1.0/24")})
	ep.Process(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: now, Prefix: netip.MustParsePrefix("10.0.0.0/24")})

	assert.Equal(t, 2, ep.Announced)
	assert.Equal(t, 1, ep.Received)
}

func TestOrchestratorChaosCounters(t *testing.T) {
	m := validation.NewModel(2)
	tr := validation.NewTracker(2)
	conv := validation.NewConvergence(2, 5*time.Second)

	ep := &EventProcessor{
		Model:       m,
		Tracker:     tr,
		Convergence: conv,
	}

	now := time.Now()

	ep.Process(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, Time: now, ChaosAction: "tcp-disconnect"})
	ep.Process(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 1, Time: now, ChaosAction: "partial-withdraw"})
	ep.Process(peer.Event{Type: peer.EventReconnecting, PeerIndex: 0, Time: now})
	ep.Process(peer.Event{Type: peer.EventWithdrawalSent, PeerIndex: 1, Time: now, Count: 15})
	ep.Process(peer.Event{Type: peer.EventWithdrawalSent, PeerIndex: 0, Time: now, Count: 5})

	assert.Equal(t, 2, ep.ChaosEvents)
	assert.Equal(t, 1, ep.Reconnections)
	assert.Equal(t, 20, ep.Withdrawn, "withdrawn should sum Count fields")

	assert.Equal(t, 0, ep.Announced)
	assert.Equal(t, 0, ep.Received)
}
