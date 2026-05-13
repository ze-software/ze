package redistribute

import (
	"context"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/family"

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
