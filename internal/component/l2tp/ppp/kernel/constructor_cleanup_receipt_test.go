// Design: docs/architecture/l2tp/subscriber-session-model.md -- constructor rollback physical receipt

package kernel

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKernelPPPConstructorClassifiesRollbackFailure drives the real constructor
// before stream wrapping. The primary refusal and each consumed descriptor's
// Close failure must remain distinguishable to the transport that owns cleanup.
func TestKernelPPPConstructorClassifiesRollbackFailure(t *testing.T) {
	primary := errors.New("interface lookup refused")
	cleanup := errors.New("original constructor raw close remains unknown")
	var closed []int
	ops := sessionTestOps(nil, &closed)
	cfg := sessionTestConfig()
	sessionTestUnit(cfg).describeErr = primary
	ops.closeFD = func(fd int) error { closed = append(closed, fd); return cleanup }
	frames, owner, err := sessionTestNewSession(cfg, ops)
	require.Nil(t, frames)
	require.Nil(t, owner)
	require.ErrorIs(t, err, primary)
	require.ErrorIs(t, err, cleanup)
	require.ErrorIs(t, SetupCleanupFailure(err), cleanup,
		"the transport must distinguish physical rollback uncertainty from setup refusal")
	require.Equal(t, []int{31, 32}, closed)
}

// TestKernelPPPConstructorConfirmedRollbackHasNoPhysicalFailure keeps a primary
// refusal from becoming a false retained cleanup failure when both original
// descriptors actually close successfully.
func TestKernelPPPConstructorConfirmedRollbackHasNoPhysicalFailure(t *testing.T) {
	primary := errors.New("interface lookup refused with confirmed cleanup")
	var closed []int
	ops := sessionTestOps(nil, &closed)
	cfg := sessionTestConfig()
	sessionTestUnit(cfg).describeErr = primary
	frames, owner, err := sessionTestNewSession(cfg, ops)
	require.Nil(t, frames)
	require.Nil(t, owner)
	require.ErrorIs(t, err, primary)
	require.NoError(t, SetupCleanupFailure(err))
	require.Equal(t, []int{31, 32}, closed)
}
