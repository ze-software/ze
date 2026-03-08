package reactor

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: Session backpressure flow control (Pause/Resume/waitForResume).
// PREVENTS: Read loop deadlocks or races when backpressure is applied/released.

// newFlowSession creates a minimal Session with only flow-control fields initialized.
func newFlowSession() *Session {
	ps := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65001, 65002, 1)
	return NewSession(ps)
}

// TestSessionFlow_PauseResume verifies basic pause/resume cycle.
func TestSessionFlow_PauseResume(t *testing.T) {
	s := newFlowSession()

	assert.False(t, s.IsPaused())

	s.Pause()
	assert.True(t, s.IsPaused())

	s.Resume()
	assert.False(t, s.IsPaused())
}

// TestSessionFlow_PauseIdempotent verifies double Pause is a no-op.
func TestSessionFlow_PauseIdempotent(t *testing.T) {
	s := newFlowSession()

	s.Pause()
	assert.True(t, s.IsPaused())

	s.Pause() // second call should not panic or change state
	assert.True(t, s.IsPaused())

	s.Resume()
	assert.False(t, s.IsPaused())
}

// TestSessionFlow_ResumeIdempotent verifies Resume on non-paused is a no-op.
func TestSessionFlow_ResumeIdempotent(t *testing.T) {
	s := newFlowSession()

	s.Resume() // should not panic when not paused
	assert.False(t, s.IsPaused())
}

// TestSessionFlow_WaitForResume_NotPaused verifies immediate return when not paused.
func TestSessionFlow_WaitForResume_NotPaused(t *testing.T) {
	s := newFlowSession()

	err := s.waitForResume(context.Background())
	assert.NoError(t, err)
}

// TestSessionFlow_WaitForResume_Blocked verifies blocking until Resume is called.
func TestSessionFlow_WaitForResume_Blocked(t *testing.T) {
	s := newFlowSession()
	s.Pause()

	var wg sync.WaitGroup
	var waitErr error

	wg.Go(func() {
		waitErr = s.waitForResume(context.Background())
	})

	// Wait for the goroutine to observe pause state
	require.Eventually(t, func() bool { return s.IsPaused() }, 2*time.Second, 10*time.Millisecond, "session should be paused")

	s.Resume()
	wg.Wait()

	require.NoError(t, waitErr)
	assert.False(t, s.IsPaused())
}

// TestSessionFlow_WaitForResume_ContextCancel verifies cancel unblocks wait.
func TestSessionFlow_WaitForResume_ContextCancel(t *testing.T) {
	s := newFlowSession()
	s.Pause()

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	var waitErr error

	wg.Go(func() {
		waitErr = s.waitForResume(ctx)
	})

	// Give the goroutine time to enter select
	time.Sleep(10 * time.Millisecond)
	cancel()
	wg.Wait()

	require.Error(t, waitErr)
	assert.True(t, errors.Is(waitErr, context.Canceled))
}

// TestSessionFlow_WaitForResume_CloseReason verifies close reason returned after resume.
func TestSessionFlow_WaitForResume_CloseReason(t *testing.T) {
	s := newFlowSession()
	s.Pause()

	// Store a close reason before resume — simulates shutdown path
	shutdownErr := errors.New("session shutdown")
	s.closeReason.Store(&shutdownErr)

	var wg sync.WaitGroup
	var waitErr error

	wg.Go(func() {
		waitErr = s.waitForResume(context.Background())
	})

	time.Sleep(10 * time.Millisecond)
	s.Resume()
	wg.Wait()

	require.Error(t, waitErr)
	assert.Equal(t, "session shutdown", waitErr.Error())
}
