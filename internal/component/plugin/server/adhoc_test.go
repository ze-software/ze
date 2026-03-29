package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// TestAdHocProcessHandshake verifies that HandleAdHocPluginSession completes
// the 5-stage handshake over a net.Pipe (simulating an SSH channel).
//
// VALIDATES: AC-9 -- ad-hoc plugin session completes 5-stage handshake.
// PREVENTS: Regression where coordinator != nil blocks ad-hoc sessions at barriers.
func TestAdHocProcessHandshake(t *testing.T) {
	t.Parallel()

	pluginSide, engineSide := net.Pipe()

	// Minimal server with required fields for handshake.
	s := &Server{
		subscriptions: NewSubscriptionManager(),
		dispatcher:    NewDispatcher(),
		registry:      plugin.NewPluginRegistry(),
		capInjector:   plugin.NewCapabilityInjector(),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	// Engine-side: run ad-hoc session handler in goroutine.
	handshakeDone := make(chan error, 1)
	go func() {
		handshakeDone <- s.HandleAdHocPluginSession(engineSide, engineSide)
	}()

	// Plugin-side: use SDK to drive the 5-stage handshake.
	p := sdk.NewWithConn("test-adhoc", pluginSide)
	defer p.Close() //nolint:errcheck // test cleanup

	runDone := make(chan error, 1)
	go func() {
		runDone <- p.Run(context.Background(), sdk.Registration{})
	}()

	// Give the handshake time to complete, then close plugin side.
	time.Sleep(200 * time.Millisecond)

	// Close triggers shutdown of both sides.
	require.NoError(t, p.Close())

	select {
	case err := <-runDone:
		// SDK Run returns nil or connection-closed error -- both OK.
		_ = err
	case <-time.After(5 * time.Second):
		t.Fatal("SDK Run did not return in time")
	}

	select {
	case err := <-handshakeDone:
		assert.NoError(t, err, "engine-side handshake should complete without error")
	case <-time.After(5 * time.Second):
		t.Fatal("HandleAdHocPluginSession did not return in time")
	}
}

// TestAdHocProcessRuntime verifies that after the handshake, runtime commands
// are dispatched correctly through the ad-hoc session.
//
// VALIDATES: AC-11 -- dispatch-command works after handshake.
// PREVENTS: Regression where ad-hoc sessions don't enter the runtime command loop.
func TestAdHocProcessRuntime(t *testing.T) {
	t.Parallel()

	pluginSide, engineSide := net.Pipe()

	// Server with dispatch-command support.
	s := &Server{
		subscriptions: NewSubscriptionManager(),
		dispatcher:    NewDispatcher(),
		registry:      plugin.NewPluginRegistry(),
		capInjector:   plugin.NewCapabilityInjector(),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	// Engine-side handler.
	handshakeDone := make(chan error, 1)
	go func() {
		handshakeDone <- s.HandleAdHocPluginSession(engineSide, engineSide)
	}()

	// Plugin-side: SDK with OnStarted callback that sends a dispatch-command.
	p := sdk.NewWithConn("test-runtime", pluginSide)
	defer p.Close() //nolint:errcheck // test cleanup

	var dispatchStatus string
	var dispatchErr error

	p.OnStarted(func(ctx context.Context) error {
		// Send dispatch-command through the runtime dispatch path.
		// This will likely return "unknown command" since no commands are
		// registered, but proving the RPC round-trips is the test.
		dispatchStatus, _, dispatchErr = p.DispatchCommand(ctx, "test echo hello")
		// Close to trigger shutdown after the test.
		p.Close() //nolint:errcheck // test cleanup
		return nil
	})

	_ = p.Run(context.Background(), sdk.Registration{})

	// The dispatch-command may return an error (unknown command) but the
	// RPC itself should complete -- proving the runtime loop is running.
	if dispatchErr != nil {
		// Error from unknown command is expected and proves the RPC worked.
		assert.Contains(t, dispatchErr.Error(), "command", "error should be about command dispatch")
	} else {
		assert.NotEmpty(t, dispatchStatus, "dispatch should return a status")
	}
}

// TestNewWithIO verifies that sdk.NewWithIO creates a functional plugin
// that can complete the 5-stage handshake.
//
// VALIDATES: NewWithIO constructor works for non-net.Conn transports.
// PREVENTS: Regression where NewWithIO creates broken MuxConn.
func TestNewWithIO(t *testing.T) {
	t.Parallel()

	pluginSide, engineSide := net.Pipe()

	// Use NewWithIO on the plugin side (simulating SSH channel).
	p := sdk.NewWithIO("test-io", pluginSide, pluginSide)
	defer p.Close() //nolint:errcheck // test cleanup

	// Minimal server for handshake.
	s := &Server{
		subscriptions: NewSubscriptionManager(),
		dispatcher:    NewDispatcher(),
		registry:      plugin.NewPluginRegistry(),
		capInjector:   plugin.NewCapabilityInjector(),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	// Engine-side: ad-hoc handler.
	handshakeDone := make(chan error, 1)
	go func() {
		handshakeDone <- s.HandleAdHocPluginSession(engineSide, engineSide)
	}()

	// Plugin-side: run handshake.
	runDone := make(chan error, 1)
	go func() {
		runDone <- p.Run(context.Background(), sdk.Registration{})
	}()

	time.Sleep(200 * time.Millisecond)
	require.NoError(t, p.Close())

	select {
	case err := <-runDone:
		_ = err // connection-closed expected
	case <-time.After(5 * time.Second):
		t.Fatal("SDK Run did not return in time")
	}

	select {
	case err := <-handshakeDone:
		assert.NoError(t, err, "NewWithIO plugin should complete handshake")
	case <-time.After(5 * time.Second):
		t.Fatal("HandleAdHocPluginSession did not return in time")
	}
}
