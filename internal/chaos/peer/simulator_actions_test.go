package peer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/chaos/engine"
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
