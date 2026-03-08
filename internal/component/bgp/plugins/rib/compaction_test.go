package rib

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attrpool"
)

func mustIntern(t *testing.T, p *attrpool.Pool, data []byte) attrpool.Handle {
	t.Helper()
	h, err := p.Intern(data)
	require.NoError(t, err)
	return h
}

// TestCompactionSchedulerStartsOnPluginStartup verifies that runCompaction
// starts the scheduler and it performs compaction when pools have dead entries.
//
// VALIDATES: AC-3 — Scheduler starts in OnStarted callback.
//
// PREVENTS: Scheduler never running, leaving dead bytes unreclaimed.
func TestCompactionSchedulerStartsOnPluginStartup(t *testing.T) {
	p, err := attrpool.NewWithIdx(2, 1024)
	require.NoError(t, err)

	// Create dead entries: intern then release
	var handles []attrpool.Handle
	for i := range 20 {
		h := mustIntern(t, p, []byte{byte(i), byte(i + 1), byte(i + 2)})
		handles = append(handles, h)
	}
	// Release first 15 (75% dead ratio — well above 25% threshold)
	for _, h := range handles[:15] {
		require.NoError(t, p.Release(h))
	}

	m := p.Metrics()
	require.Greater(t, m.DeadSlots, int32(0), "must have dead slots before compaction")
	initialDead := m.DeadSlots

	go runCompaction(t.Context(), []*attrpool.Pool{p})

	// Wait for scheduler to compact (check interval is 50ms default)
	assert.Eventually(t, func() bool {
		return p.Metrics().DeadSlots < initialDead
	}, 2*time.Second, 10*time.Millisecond, "scheduler must compact dead entries")
}

// TestCompactionSchedulerStopsOnShutdown verifies that canceling the context
// causes the scheduler goroutine to exit cleanly.
//
// VALIDATES: AC-4 — Scheduler exits on context cancel (no goroutine leak).
//
// PREVENTS: Goroutine leak when plugin shuts down.
func TestCompactionSchedulerStopsOnShutdown(t *testing.T) {
	p, err := attrpool.NewWithIdx(2, 1024)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Go(func() {
		runCompaction(ctx, []*attrpool.Pool{p})
	})

	// Cancel context — scheduler should exit
	cancel()

	// Wait for goroutine to exit with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success — goroutine exited
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler goroutine did not exit after context cancel")
	}
}

// TestSchedulerCompactsAfterChurn verifies that after route churn
// (intern + release cycles), the scheduler reclaims dead buffer space.
//
// VALIDATES: AC-5 — Dead ratio drops, buffer bytes reclaimed after churn.
//
// PREVENTS: Buffer memory leak under sustained route churn.
func TestSchedulerCompactsAfterChurn(t *testing.T) {
	p, err := attrpool.NewWithIdx(2, 4096)
	require.NoError(t, err)

	// Simulate route churn: intern all first, then release most.
	// Must batch interns before releases to prevent free-list slot reuse,
	// which would keep dead slot count too low for the scheduler threshold.
	var handles []attrpool.Handle
	for i := range 100 {
		data := []byte{byte(i >> 8), byte(i), 0xAA, 0xBB}
		handles = append(handles, mustIntern(t, p, data))
	}
	// Release 75% — well above 25% dead ratio threshold
	var liveHandles []attrpool.Handle
	for i, h := range handles {
		if i%4 == 0 {
			liveHandles = append(liveHandles, h)
		} else {
			require.NoError(t, p.Release(h))
		}
	}

	m := p.Metrics()
	require.Greater(t, m.DeadSlots, int32(0), "must have dead slots from churn")
	require.Greater(t, m.DeadBytes, int64(0), "must have dead bytes from churn")
	initialDeadBytes := m.DeadBytes

	go runCompaction(t.Context(), []*attrpool.Pool{p})

	// Wait for compaction to reclaim dead bytes
	assert.Eventually(t, func() bool {
		return p.Metrics().DeadBytes < initialDeadBytes
	}, 2*time.Second, 10*time.Millisecond, "dead bytes must decrease after compaction")

	// Live data must still be accessible
	for _, h := range liveHandles {
		data, err := p.Get(h)
		assert.NoError(t, err, "live handle must still be accessible after compaction")
		assert.NotEmpty(t, data, "live data must not be empty")
	}
}
