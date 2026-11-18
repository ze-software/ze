// Design: docs/architecture/api/process-protocol.md -- explicit joined and unknown failures.
package rpc

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCachedWriteFailureNeedsExactPositiveCompletion(t *testing.T) {
	const deadline int64 = 7001
	for _, cause := range []error{errors.New("original target refused"), context.Canceled, context.DeadlineExceeded} {
		output := CompletedCachedWrite(deadline, cause)
		err := output.Validate(deadline)
		require.Error(t, err)
		_, joined := errors.AsType[*CachedWriteJoinedError](err)
		require.True(t, joined)
		if errors.Is(cause, context.Canceled) {
			require.ErrorIs(t, err, context.Canceled)
		}
		if errors.Is(cause, context.DeadlineExceeded) {
			require.ErrorIs(t, err, context.DeadlineExceeded)
		}
		for _, alter := range []func(*CachedWriteOutput){func(o *CachedWriteOutput) { o.Completed = false }, func(o *CachedWriteOutput) { o.DeadlineMS++ }, func(o *CachedWriteOutput) { o.FailureCause = 99 }} {
			broken := *output
			alter(&broken)
			err := broken.Validate(deadline)
			require.Error(t, err)
			_, joined := errors.AsType[*CachedWriteJoinedError](err)
			require.False(t, joined, "malformed completion is unknown, never joined")
		}
	}
	var missing *CachedWriteOutput
	err := missing.Validate(deadline)
	_, joined := errors.AsType[*CachedWriteJoinedError](err)
	require.False(t, joined)
}
