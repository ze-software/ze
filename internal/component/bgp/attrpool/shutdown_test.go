package attrpool

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestShutdownRejectsNewOperations verifies shutdown blocks new work.
//
// VALIDATES: Clean shutdown semantics.
//
// PREVENTS: Operations starting during shutdown, causing races or panics.
func TestShutdownRejectsNewOperations(t *testing.T) {
	p := New(1024)
	p.Shutdown()

	h, err := p.Intern([]byte("data"))
	require.ErrorIs(t, err, ErrPoolShutdown)
	require.Equal(t, InvalidHandle, h)
}

// TestShutdownIdempotent verifies multiple shutdown calls are safe.
//
// VALIDATES: Idempotent shutdown.
//
// PREVENTS: Double-free or panic on repeated shutdown calls.
func TestShutdownIdempotent(t *testing.T) {
	p := New(1024)

	require.NotPanics(t, func() {
		p.Shutdown()
		p.Shutdown()
		p.Shutdown()
	})
}

// TestShutdownExistingHandlesStillWork verifies existing data is accessible.
//
// VALIDATES: Graceful degradation - existing data remains accessible.
//
// PREVENTS: Data loss during shutdown.
func TestShutdownExistingHandlesStillWork(t *testing.T) {
	p := New(1024)

	h := mustIntern(t, p, []byte("existing-data"))

	p.Shutdown()

	// Existing handles should still work
	data, err := p.Get(h)
	require.NoError(t, err)
	require.Equal(t, []byte("existing-data"), data)
}

// TestShutdownReleasesStillWork verifies Release works after shutdown.
//
// VALIDATES: Cleanup operations work during shutdown.
//
// PREVENTS: Resource leaks from blocked cleanup.
func TestShutdownReleasesStillWork(t *testing.T) {
	p := New(1024)

	h := mustIntern(t, p, []byte("data"))

	p.Shutdown()

	// Release should still work
	require.NotPanics(t, func() {
		_ = p.Release(h)
	})
}

// TestIsShutdown verifies shutdown state query.
//
// VALIDATES: Shutdown state is queryable.
//
// PREVENTS: Callers can't check if pool is shutting down.
func TestIsShutdown(t *testing.T) {
	p := New(1024)

	require.False(t, p.IsShutdown())

	p.Shutdown()

	require.True(t, p.IsShutdown())
}

// TestShutdownMetricsStillWork verifies metrics are accessible after shutdown.
//
// VALIDATES: Observability during shutdown.
//
// PREVENTS: Metrics collection failing during shutdown.
func TestShutdownMetricsStillWork(t *testing.T) {
	p := New(1024)
	mustIntern(t, p, []byte("data"))

	p.Shutdown()

	m := p.Metrics()
	require.Equal(t, int32(1), m.LiveSlots)
}
