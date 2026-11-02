// Design: docs/architecture/api/process-protocol.md -- post-startup refusal
// Related: startup.go -- signalStartupComplete, WaitForStartupComplete

package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// errPostStartupRefusal stands for the bgp plugin's refusal of a config whose
// filter list names a required filter insertion.
var errPostStartupRefusal = errors.New("peer peer1: the import filter list names bgp-role:otc-ingress")

// TestPostStartupRefusalIsTheStartupError checks that a refusal from the
// reactor's post-startup callback reaches WaitForStartupComplete, which
// cmd/ze/hub exits on. Method: a Coordinator whose OnPostStartup callback
// refuses is the server's reactor; signalStartupComplete runs, and
// WaitForStartupComplete must answer that refusal at once. Dropping the
// callback's error (as the daemon once did, leaving it running with no peer
// started) makes it answer nil.
func TestPostStartupRefusalIsTheStartupError(t *testing.T) {
	coord := plugin.NewCoordinator(map[string]any{})
	called := false
	coord.OnPostStartup(func() error {
		called = true
		return errPostStartupRefusal
	})
	s, err := NewServer(&ServerConfig{}, coord)
	require.NoError(t, err)

	s.signalStartupComplete()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got := s.WaitForStartupComplete(ctx)
	assert.True(t, called, "the post-startup callback must run")
	require.ErrorIs(t, got, errPostStartupRefusal, "the callback's refusal must be the startup error")
}

// TestPostStartupRefusalKeepsEarlierStartupError checks that a plugin phase's
// failure stays the startup error when the post-startup callback also refuses:
// the phase failure is the cause, and the callback ran after it.
func TestPostStartupRefusalKeepsEarlierStartupError(t *testing.T) {
	coord := plugin.NewCoordinator(map[string]any{})
	coord.OnPostStartup(func() error { return errPostStartupRefusal })
	s, err := NewServer(&ServerConfig{}, coord)
	require.NoError(t, err)
	phaseErr := errors.New("config-path plugin startup failed")
	s.startupErr = phaseErr

	s.signalStartupComplete()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got := s.WaitForStartupComplete(ctx)
	require.ErrorIs(t, got, phaseErr)
	assert.NotErrorIs(t, got, errPostStartupRefusal)
}
