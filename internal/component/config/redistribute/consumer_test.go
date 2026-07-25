package redistribute

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubConsumer struct {
	name      string
	injected  []RouteEntry
	withdrawn []string
}

func (s *stubConsumer) Name() string { return s.name }

func (s *stubConsumer) InjectRoute(_ context.Context, _ family.Family, entry RouteEntry) {
	s.injected = append(s.injected, entry)
}

func (s *stubConsumer) WithdrawRoute(_ context.Context, _ family.Family, prefix string) {
	s.withdrawn = append(s.withdrawn, prefix)
}

// TestRegisterConsumer verifies consumer registration and lookup.
//
// VALIDATES: RegisterConsumer stores consumer, LookupConsumer retrieves it.
// PREVENTS: Registered consumers lost or returned with wrong name.
func TestRegisterConsumer(t *testing.T) {
	c := &stubConsumer{name: "test-register"}
	require.NoError(t, RegisterConsumer(c))

	got, ok := LookupConsumer("test-register")
	require.True(t, ok)
	assert.Equal(t, "test-register", got.Name())
}

// TestLookupConsumer verifies lookup returns false for missing names.
//
// VALIDATES: LookupConsumer returns false for unregistered name.
// PREVENTS: Panic or incorrect match on unknown consumer.
func TestLookupConsumer(t *testing.T) {
	_, ok := LookupConsumer("nonexistent-consumer")
	assert.False(t, ok)
}

// TestConsumerNames verifies sorted name list.
//
// VALIDATES: ConsumerNames returns all registered names sorted.
// PREVENTS: Missing or unsorted names.
func TestConsumerNames(t *testing.T) {
	require.NoError(t, RegisterConsumer(&stubConsumer{name: "test-z-consumer"}))
	require.NoError(t, RegisterConsumer(&stubConsumer{name: "test-a-consumer"}))

	names := ConsumerNames()
	aIdx, zIdx := -1, -1
	for i, n := range names {
		if n == "test-a-consumer" {
			aIdx = i
		}
		if n == "test-z-consumer" {
			zIdx = i
		}
	}
	require.NotEqual(t, -1, aIdx, "test-a-consumer not found")
	require.NotEqual(t, -1, zIdx, "test-z-consumer not found")
	assert.True(t, aIdx < zIdx, "names not sorted: a at %d, z at %d", aIdx, zIdx)
}

// TestRegisterConsumerConflict verifies that re-registering the same name returns an error.
//
// VALIDATES: RegisterConsumer rejects duplicate names.
// PREVENTS: Silent overwrite of existing consumer.
func TestRegisterConsumerConflict(t *testing.T) {
	require.NoError(t, RegisterConsumer(&stubConsumer{name: "test-conflict"}))
	err := RegisterConsumer(&stubConsumer{name: "test-conflict"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrConsumerConflict)
}

// TestReregisterConsumerRewires is the isis-11 regression: re-registering the
// same consumer name must REWIRE to the new consumer instance rather than fail.
// On an SDK reconnect OnStarted re-fires with a fresh engine; a plain
// RegisterConsumer would return ErrConsumerConflict and redistribution into the
// destination protocol would silently stop for the new instance.
//
// VALIDATES: ReregisterConsumer replaces the existing consumer under the same
// name (no error) and LookupConsumer then returns the NEW instance.
// PREVENTS: Redistribution silently dropping after an engine recreate because
// the second registration failed and the stale consumer stayed wired.
func TestReregisterConsumerRewires(t *testing.T) {
	first := &stubConsumer{name: "test-rewire"}
	second := &stubConsumer{name: "test-rewire"}

	// First registration: nothing to replace.
	require.False(t, ReregisterConsumer(first))
	got, ok := LookupConsumer("test-rewire")
	require.True(t, ok)
	require.Same(t, first, got)

	// Re-register the same name with a new instance: must rewire, not fail.
	require.True(t, ReregisterConsumer(second), "expected replaced=true on rewire")
	got, ok = LookupConsumer("test-rewire")
	require.True(t, ok)
	require.Same(t, second, got, "lookup must return the NEW consumer after rewire")

	// Routes delivered after the rewire reach the new instance, not the stale one.
	got.InjectRoute(context.Background(), family.IPv4Unicast, RouteEntry{Prefix: "10.0.0.0/24"})
	assert.Len(t, second.injected, 1, "new consumer should receive the route")
	assert.Empty(t, first.injected, "stale consumer must not receive routes after rewire")
}
