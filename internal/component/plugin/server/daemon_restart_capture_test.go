// Design: docs/architecture/api/commands.md -- shared accepted restart completion.
package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCaptureRestartCompletionPreservesTheResponseBoundary proves capturing
// a ready restart does not run it, completion runs it once, and a later action
// replacement cannot retarget an already accepted response.
func TestCaptureRestartCompletionPreservesTheResponseBoundary(t *testing.T) {
	server, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	first, replacement := 0, 0
	server.SetRestartFunc(func() { first++ })
	complete, err := server.CaptureRestartCompletion(context.Background())
	require.NoError(t, err)
	require.Zero(t, first)
	server.SetRestartFunc(func() { replacement++ })
	complete()
	complete()
	require.Equal(t, 1, first)
	require.Zero(t, replacement)
}

// TestCaptureRestartCompletionRefusesCanceledOrStoppingAdmission proves the
// readiness policy refuses absent, canceled and terminal lifetimes, while a
// live installed action remains available before Stop enters.
func TestCaptureRestartCompletionRefusesCanceledOrStoppingAdmission(t *testing.T) {
	server, err := NewServer(&ServerConfig{}, nil)
	require.NoError(t, err)
	_, err = server.CaptureRestartCompletion(context.Background())
	require.Error(t, err)
	server.SetRestartFunc(func() { t.Error("refused restart ran") })
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = server.CaptureRestartCompletion(canceled)
	require.ErrorIs(t, err, context.Canceled)
	_, err = server.CaptureRestartCompletion(context.Background())
	require.NoError(t, err)
	server.Stop()
	_, err = server.CaptureRestartCompletion(context.Background())
	require.ErrorIs(t, err, errServerStopped)
	var absent *Server
	_, err = absent.CaptureRestartCompletion(context.Background())
	require.Error(t, err)
}
