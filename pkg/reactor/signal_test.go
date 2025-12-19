package reactor

import (
	"context"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSignalHandlerNew verifies SignalHandler creation.
//
// VALIDATES: SignalHandler is created with correct initial state.
//
// PREVENTS: Handler auto-starting or with invalid state.
func TestSignalHandlerNew(t *testing.T) {
	handler := NewSignalHandler()

	require.NotNil(t, handler, "NewSignalHandler must return non-nil")
	require.False(t, handler.Running(), "handler should not be running initially")
}

// TestSignalHandlerStartStop verifies basic start/stop lifecycle.
//
// VALIDATES: SignalHandler can be started and stopped cleanly.
//
// PREVENTS: Resource leaks or goroutine leaks on stop.
func TestSignalHandlerStartStop(t *testing.T) {
	handler := NewSignalHandler()

	handler.Start()
	require.True(t, handler.Running(), "handler should be running after Start")

	handler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := handler.Wait(ctx)
	require.NoError(t, err)

	require.False(t, handler.Running(), "handler should not be running after Stop")
}

// TestSignalHandlerSIGTERM verifies SIGTERM handling.
//
// VALIDATES: SIGTERM triggers shutdown callback.
//
// PREVENTS: Missing shutdown signal handling.
func TestSignalHandlerSIGTERM(t *testing.T) {
	handler := NewSignalHandler()

	var called atomic.Bool
	handler.OnShutdown(func() {
		called.Store(true)
	})

	handler.Start()
	defer handler.Stop()

	// Send SIGTERM to self
	err := syscall.Kill(os.Getpid(), syscall.SIGTERM)
	require.NoError(t, err)

	// Wait for callback
	time.Sleep(50 * time.Millisecond)

	require.True(t, called.Load(), "shutdown callback should be called on SIGTERM")
}

// TestSignalHandlerSIGHUP verifies SIGHUP handling.
//
// VALIDATES: SIGHUP triggers reload callback.
//
// PREVENTS: Missing config reload signal handling.
func TestSignalHandlerSIGHUP(t *testing.T) {
	handler := NewSignalHandler()

	var called atomic.Bool
	handler.OnReload(func() {
		called.Store(true)
	})

	handler.Start()
	defer handler.Stop()

	// Send SIGHUP to self
	err := syscall.Kill(os.Getpid(), syscall.SIGHUP)
	require.NoError(t, err)

	// Wait for callback
	time.Sleep(50 * time.Millisecond)

	require.True(t, called.Load(), "reload callback should be called on SIGHUP")
}

// TestSignalHandlerSIGUSR1 verifies SIGUSR1 handling.
//
// VALIDATES: SIGUSR1 triggers status callback.
//
// PREVENTS: Missing status dump signal handling.
func TestSignalHandlerSIGUSR1(t *testing.T) {
	handler := NewSignalHandler()

	var called atomic.Bool
	handler.OnStatus(func() {
		called.Store(true)
	})

	handler.Start()
	defer handler.Stop()

	// Send SIGUSR1 to self
	err := syscall.Kill(os.Getpid(), syscall.SIGUSR1)
	require.NoError(t, err)

	// Wait for callback
	time.Sleep(50 * time.Millisecond)

	require.True(t, called.Load(), "status callback should be called on SIGUSR1")
}

// TestSignalHandlerContextCancellation verifies handler stops on context cancellation.
//
// VALIDATES: SignalHandler respects context cancellation.
//
// PREVENTS: Orphaned goroutines when parent context is cancelled.
func TestSignalHandlerContextCancellation(t *testing.T) {
	handler := NewSignalHandler()

	ctx, cancel := context.WithCancel(context.Background())

	handler.StartWithContext(ctx)
	require.True(t, handler.Running())

	// Cancel context
	cancel()

	// Should stop within reasonable time
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	err := handler.Wait(waitCtx)

	require.NoError(t, err, "handler should stop on context cancellation")
	require.False(t, handler.Running())
}

// TestSignalHandlerMultipleSignals verifies multiple signal types work together.
//
// VALIDATES: Different signals trigger different callbacks.
//
// PREVENTS: Signal handling interference.
func TestSignalHandlerMultipleSignals(t *testing.T) {
	handler := NewSignalHandler()

	var reloadCount, statusCount atomic.Int32

	handler.OnReload(func() {
		reloadCount.Add(1)
	})
	handler.OnStatus(func() {
		statusCount.Add(1)
	})

	handler.Start()
	defer handler.Stop()

	// Send multiple signals
	_ = syscall.Kill(os.Getpid(), syscall.SIGHUP)
	time.Sleep(20 * time.Millisecond)
	_ = syscall.Kill(os.Getpid(), syscall.SIGUSR1)
	time.Sleep(20 * time.Millisecond)
	_ = syscall.Kill(os.Getpid(), syscall.SIGHUP)
	time.Sleep(20 * time.Millisecond)

	require.Equal(t, int32(2), reloadCount.Load(), "reload should be called twice")
	require.Equal(t, int32(1), statusCount.Load(), "status should be called once")
}

// TestSignalHandlerNoCallback verifies signals are handled even without callbacks.
//
// VALIDATES: Missing callbacks don't cause panics or hangs.
//
// PREVENTS: Crashes when callbacks not configured.
func TestSignalHandlerNoCallback(t *testing.T) {
	handler := NewSignalHandler()

	handler.Start()
	defer handler.Stop()

	// Send signal without callback configured - should not panic
	err := syscall.Kill(os.Getpid(), syscall.SIGUSR1)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	// If we get here without panic, test passes
}
