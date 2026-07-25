package peer

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/chaos/engine"
)

// TestExecuteChaosSlowReadToggle verifies the on/off/on toggle cycle for
// ActionSlowRead, including default delay vs configured delay.
//
// VALIDATES: Toggle stores correct delay and restores to zero.
// PREVENTS: Stuck slow-read state or wrong delay after re-toggle.
func TestExecuteChaosSlowReadToggle(t *testing.T) {
	t.Run("default_delay", func(t *testing.T) {
		var readDelayNs atomic.Int64
		p := SimProfile{} // SlowRead == 0: use defaultSlowReadDelay (1s)
		cfg := SimulatorConfig{Quiet: true}
		action := engine.ChaosAction{Type: engine.ActionSlowRead}
		emit := func(Event) {}

		// Toggle ON: should store defaultSlowReadDelay (1s).
		result := executeChaos(context.Background(), action, nil, func() {}, p, cfg, emit, &readDelayNs)
		assert.False(t, result.Disconnected)
		assert.Equal(t, int64(defaultSlowReadDelay), readDelayNs.Load(), "first toggle should enable default delay")

		// Toggle OFF: should store 0.
		result = executeChaos(context.Background(), action, nil, func() {}, p, cfg, emit, &readDelayNs)
		assert.False(t, result.Disconnected)
		assert.Equal(t, int64(0), readDelayNs.Load(), "second toggle should disable delay")

		// Toggle ON again: should restore defaultSlowReadDelay.
		result = executeChaos(context.Background(), action, nil, func() {}, p, cfg, emit, &readDelayNs)
		assert.False(t, result.Disconnected)
		assert.Equal(t, int64(defaultSlowReadDelay), readDelayNs.Load(), "third toggle should re-enable default delay")
	})

	t.Run("configured_delay", func(t *testing.T) {
		var readDelayNs atomic.Int64
		p := SimProfile{SlowRead: 5 * time.Second} // CLI-configured delay
		cfg := SimulatorConfig{Quiet: true}
		action := engine.ChaosAction{Type: engine.ActionSlowRead}
		emit := func(Event) {}

		// Toggle ON: should use configured delay (5s), not default.
		result := executeChaos(context.Background(), action, nil, func() {}, p, cfg, emit, &readDelayNs)
		assert.False(t, result.Disconnected)
		assert.Equal(t, int64(5*time.Second), readDelayNs.Load(), "should use configured delay")

		// Toggle OFF.
		result = executeChaos(context.Background(), action, nil, func() {}, p, cfg, emit, &readDelayNs)
		assert.False(t, result.Disconnected)
		assert.Equal(t, int64(0), readDelayNs.Load())

		// Toggle ON again: should restore configured delay.
		result = executeChaos(context.Background(), action, nil, func() {}, p, cfg, emit, &readDelayNs)
		assert.False(t, result.Disconnected)
		assert.Equal(t, int64(5*time.Second), readDelayNs.Load(), "should restore configured delay")
	})

	t.Run("initially_slow_toggles_off_first", func(t *testing.T) {
		// Peer started with --slow-peers: readDelayNs is already non-zero.
		var readDelayNs atomic.Int64
		readDelayNs.Store(int64(2 * time.Second))
		p := SimProfile{SlowRead: 2 * time.Second}
		cfg := SimulatorConfig{Quiet: true}
		action := engine.ChaosAction{Type: engine.ActionSlowRead}
		emit := func(Event) {}

		// First toggle: already slow, should turn OFF.
		result := executeChaos(context.Background(), action, nil, func() {}, p, cfg, emit, &readDelayNs)
		assert.False(t, result.Disconnected)
		assert.Equal(t, int64(0), readDelayNs.Load(), "should toggle OFF when already slow")

		// Second toggle: should turn ON with configured delay.
		result = executeChaos(context.Background(), action, nil, func() {}, p, cfg, emit, &readDelayNs)
		assert.False(t, result.Disconnected)
		assert.Equal(t, int64(2*time.Second), readDelayNs.Load(), "should toggle ON with configured delay")
	})
}

// TestSimulatorDialerField verifies that executeReconnectStorm and
// executeConnectionCollision use SimulatorConfig.Dialer when set.
//
// VALIDATES: AC-4/AC-5: Dialer field used by storm/collision actions.
// PREVENTS: Hardcoded net.Dialer bypassing mock connections in-process.
func TestSimulatorDialerField(t *testing.T) {
	t.Run("reconnect_storm_uses_dialer", func(t *testing.T) {
		var dialCount int
		mockDialer := &countingDialer{onDial: func() (net.Conn, error) {
			dialCount++
			c1, c2 := net.Pipe()
			go func() { _ = c2.Close() }()
			return c1, nil
		}}

		p := SimProfile{ASN: 65000, RouterID: netip.MustParseAddr("10.0.0.2"), HoldTime: 90, Families: []string{"ipv4/unicast"}}
		emit := func(Event) {}

		executeReconnectStorm(context.Background(), "127.0.0.1:179", p, mockDialer, emit)
		assert.Greater(t, dialCount, 0, "dialer should be called during reconnect storm")
	})

	t.Run("connection_collision_uses_dialer", func(t *testing.T) {
		var dialCount int
		mockDialer := &countingDialer{onDial: func() (net.Conn, error) {
			dialCount++
			c1, c2 := net.Pipe()
			go func() { _ = c2.Close() }()
			return c1, nil
		}}

		p := SimProfile{ASN: 65000, RouterID: netip.MustParseAddr("10.0.0.2"), HoldTime: 90, Families: []string{"ipv4/unicast"}}
		emit := func(Event) {}

		executeConnectionCollision(context.Background(), "127.0.0.1:179", p, mockDialer, emit)
		assert.Equal(t, 1, dialCount, "dialer should be called once for collision")
	})

	t.Run("nil_dialer_uses_net_dialer", func(t *testing.T) {
		p := SimProfile{ASN: 65000, RouterID: netip.MustParseAddr("10.0.0.2"), HoldTime: 90, Families: []string{"ipv4/unicast"}}
		var events []Event
		emit := func(ev Event) { events = append(events, ev) }

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// With nil dialer and unreachable addr, storm should fail gracefully.
		executeReconnectStorm(ctx, "192.0.2.1:179", p, nil, emit)
		// No panic, no hang - function returned.
	})
}

type countingDialer struct {
	onDial func() (net.Conn, error)
}

func (d *countingDialer) DialContext(_ context.Context, _, _ string) (net.Conn, error) {
	return d.onDial()
}
