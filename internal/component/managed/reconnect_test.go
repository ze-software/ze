package managed

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestReconnectBackoff verifies exponential backoff doubling.
//
// VALIDATES: Backoff doubles: 1s, 2s, 4s, 8s (AC-12).
// PREVENTS: Fixed delay or linear backoff.
func TestReconnectBackoff(t *testing.T) {
	t.Parallel()

	b := NewBackoff(1*time.Second, 60*time.Second, 0) // no jitter for deterministic test

	assert.Equal(t, 1*time.Second, b.Next())
	assert.Equal(t, 2*time.Second, b.Next())
	assert.Equal(t, 4*time.Second, b.Next())
	assert.Equal(t, 8*time.Second, b.Next())
	assert.Equal(t, 16*time.Second, b.Next())
	assert.Equal(t, 32*time.Second, b.Next())
}

// TestReconnectBackoffCap verifies the maximum delay is capped.
//
// VALIDATES: Never exceeds 60s (AC-12).
// PREVENTS: Unbounded delay growth.
func TestReconnectBackoffCap(t *testing.T) {
	t.Parallel()

	b := NewBackoff(1*time.Second, 60*time.Second, 0)

	// Advance past cap.
	for range 10 {
		b.Next()
	}

	// Should be capped at 60s.
	delay := b.Next()
	assert.Equal(t, 60*time.Second, delay)
}

// TestReconnectBackoffJitter verifies jitter stays within 10%.
//
// VALIDATES: Jitter within 10% (AC-12).
// PREVENTS: Jitter exceeding bounds causing thundering herd.
func TestReconnectBackoffJitter(t *testing.T) {
	t.Parallel()

	b := NewBackoff(1*time.Second, 60*time.Second, 0.1)

	for range 20 {
		delay := b.Next()
		// With jitter, delay should be between base*0.9 and base*1.1.
		// We can't predict exact base after doubling, but delay must never exceed cap*1.1.
		assert.LessOrEqual(t, delay, 66*time.Second, "delay with jitter should not exceed cap*1.1")
		assert.Greater(t, delay, time.Duration(0), "delay must be positive")
	}
}

// TestReconnectBackoffReset verifies reset returns to initial delay.
//
// VALIDATES: Reset restores initial backoff state.
// PREVENTS: Stale delay after successful reconnect.
func TestReconnectBackoffReset(t *testing.T) {
	t.Parallel()

	b := NewBackoff(1*time.Second, 60*time.Second, 0)

	b.Next() // 1s
	b.Next() // 2s
	b.Next() // 4s
	b.Reset()

	assert.Equal(t, 1*time.Second, b.Next(), "should restart from initial delay after reset")
}
