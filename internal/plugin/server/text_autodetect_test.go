package server

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/plugin/process"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestTextAutoDetectHandshake verifies that the engine can auto-detect text vs JSON mode
// from the first byte and complete the handshake in the detected mode.
//
// VALIDATES: AC-8 (mode auto-detection from first byte).
// PREVENTS: PeekMode consuming the first byte and breaking subsequent reads.
func TestTextAutoDetectHandshake(t *testing.T) {
	t.Parallel()

	pairs, err := ipc.NewInternalSocketPairs()
	require.NoError(t, err)
	t.Cleanup(func() { pairs.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Plugin side sends text registration on Socket A.
	pluginA := rpc.NewTextConn(pairs.Engine.PluginSide, pairs.Engine.PluginSide)
	pluginB := rpc.NewTextConn(pairs.Callback.PluginSide, pairs.Callback.PluginSide)

	errCh := make(chan error, 1)
	go func() {
		// Stage 1: plugin sends registration
		text, fmtErr := rpc.FormatRegistrationText(rpc.DeclareRegistrationInput{
			Families: []rpc.FamilyDecl{{Name: "ipv4/unicast", Mode: "both"}},
		})
		if fmtErr != nil {
			errCh <- fmtErr
			return
		}
		if writeErr := pluginA.WriteMessage(ctx, text); writeErr != nil {
			errCh <- writeErr
			return
		}
		if _, readErr := pluginA.ReadLine(ctx); readErr != nil {
			errCh <- readErr
			return
		}

		// Stage 2: read configure from Socket B
		if _, readErr := pluginB.ReadMessage(ctx); readErr != nil {
			errCh <- readErr
			return
		}
		if writeErr := pluginB.WriteLine(ctx, "ok"); writeErr != nil {
			errCh <- writeErr
			return
		}

		// Stage 3: send capabilities
		capsText, fmtErr := rpc.FormatCapabilitiesText(rpc.DeclareCapabilitiesInput{})
		if fmtErr != nil {
			errCh <- fmtErr
			return
		}
		if writeErr := pluginA.WriteMessage(ctx, capsText); writeErr != nil {
			errCh <- writeErr
			return
		}
		if _, readErr := pluginA.ReadLine(ctx); readErr != nil {
			errCh <- readErr
			return
		}

		// Stage 4: read registry
		if _, readErr := pluginB.ReadMessage(ctx); readErr != nil {
			errCh <- readErr
			return
		}
		if writeErr := pluginB.WriteLine(ctx, "ok"); writeErr != nil {
			errCh <- writeErr
			return
		}

		// Stage 5: send ready
		readyText, fmtErr := rpc.FormatReadyText(rpc.ReadyInput{})
		if fmtErr != nil {
			errCh <- fmtErr
			return
		}
		if writeErr := pluginA.WriteMessage(ctx, readyText); writeErr != nil {
			errCh <- writeErr
			return
		}
		if _, readErr := pluginA.ReadLine(ctx); readErr != nil {
			errCh <- readErr
			return
		}

		errCh <- nil
	}()

	// Engine side: auto-detect mode, then complete text protocol.
	rawA := pairs.Engine.EngineSide
	mode, wrappedA, err := rpc.PeekMode(rawA)
	require.NoError(t, err)
	assert.Equal(t, rpc.ModeText, mode, "first byte should be 'r' from 'register'")

	// Create TextConns from the peeked conn.
	tcA := rpc.NewTextConn(wrappedA, wrappedA)
	tcB := rpc.NewTextConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide)

	// Run text protocol using the engine-side handler.
	_, err = completeTextProtocol(ctx, tcA, tcB)
	require.NoError(t, err)

	select {
	case pluginErr := <-errCh:
		require.NoError(t, pluginErr)
	case <-time.After(3 * time.Second):
		t.Fatal("plugin did not complete")
	}
}

// TestSubsystemAutoDetectText verifies that SubsystemHandler.completeProtocol
// auto-detects text mode from the first byte and completes the text handshake,
// extracting commands from the text registration.
//
// VALIDATES: AC-8 (auto-detect wiring through process.go initConns).
// PREVENTS: Text mode detection being implemented but not wired into subsystem path.
func TestSubsystemAutoDetectText(t *testing.T) {
	t.Parallel()

	// Create socket pairs (simulates what startInternal/startExternal stores).
	engineA, pluginA := net.Pipe()
	engineB, pluginB := net.Pipe()
	t.Cleanup(func() {
		if err := pluginA.Close(); err != nil {
			t.Log("cleanup pluginA:", err)
		}
		if err := pluginB.Close(); err != nil {
			t.Log("cleanup pluginB:", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a process with raw conns (simulates startInternal storing raw conns).
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-text-subsystem"})
	proc.SetRawConns(engineA, engineB)

	// Create subsystem handler.
	handler := &SubsystemHandler{
		config: SubsystemConfig{Name: "test-text"},
		proc:   proc,
	}

	// Plugin side: speak text protocol.
	errCh := make(chan error, 1)
	go func() {
		tcA := rpc.NewTextConn(pluginA, pluginA)
		tcB := rpc.NewTextConn(pluginB, pluginB)

		// Stage 1: send registration with commands
		regText, fmtErr := rpc.FormatRegistrationText(rpc.DeclareRegistrationInput{
			Commands: []rpc.CommandDecl{
				{Name: "test-cmd", Description: "a test command"},
				{Name: "test-other", Description: "another command"},
			},
		})
		if fmtErr != nil {
			errCh <- fmt.Errorf("format reg: %w", fmtErr)
			return
		}
		if writeErr := tcA.WriteMessage(ctx, regText); writeErr != nil {
			errCh <- fmt.Errorf("write reg: %w", writeErr)
			return
		}
		resp, readErr := tcA.ReadLine(ctx)
		if readErr != nil {
			errCh <- fmt.Errorf("read reg resp: %w", readErr)
			return
		}
		if resp != "ok" {
			errCh <- fmt.Errorf("stage 1: expected ok, got %q", resp)
			return
		}

		// Stage 2: read config
		if _, readErr := tcB.ReadMessage(ctx); readErr != nil {
			errCh <- fmt.Errorf("read config: %w", readErr)
			return
		}
		if writeErr := tcB.WriteLine(ctx, "ok"); writeErr != nil {
			errCh <- fmt.Errorf("write config resp: %w", writeErr)
			return
		}

		// Stage 3: send capabilities
		capsText, fmtErr := rpc.FormatCapabilitiesText(rpc.DeclareCapabilitiesInput{})
		if fmtErr != nil {
			errCh <- fmt.Errorf("format caps: %w", fmtErr)
			return
		}
		if writeErr := tcA.WriteMessage(ctx, capsText); writeErr != nil {
			errCh <- fmt.Errorf("write caps: %w", writeErr)
			return
		}
		resp, readErr = tcA.ReadLine(ctx)
		if readErr != nil {
			errCh <- fmt.Errorf("read caps resp: %w", readErr)
			return
		}
		if resp != "ok" {
			errCh <- fmt.Errorf("stage 3: expected ok, got %q", resp)
			return
		}

		// Stage 4: read registry
		if _, readErr := tcB.ReadMessage(ctx); readErr != nil {
			errCh <- fmt.Errorf("read registry: %w", readErr)
			return
		}
		if writeErr := tcB.WriteLine(ctx, "ok"); writeErr != nil {
			errCh <- fmt.Errorf("write registry resp: %w", writeErr)
			return
		}

		// Stage 5: send ready
		readyText, fmtErr := rpc.FormatReadyText(rpc.ReadyInput{})
		if fmtErr != nil {
			errCh <- fmt.Errorf("format ready: %w", fmtErr)
			return
		}
		if writeErr := tcA.WriteMessage(ctx, readyText); writeErr != nil {
			errCh <- fmt.Errorf("write ready: %w", writeErr)
			return
		}
		resp, readErr = tcA.ReadLine(ctx)
		if readErr != nil {
			errCh <- fmt.Errorf("read ready resp: %w", readErr)
			return
		}
		if resp != "ok" {
			errCh <- fmt.Errorf("stage 5: expected ok, got %q", resp)
			return
		}

		errCh <- nil
	}()

	// Engine side: completeProtocol should auto-detect text mode.
	err := handler.completeProtocol(ctx)
	require.NoError(t, err, "completeProtocol should succeed with text mode")

	// Wait for plugin side.
	select {
	case pluginErr := <-errCh:
		require.NoError(t, pluginErr, "plugin side error")
	case <-time.After(3 * time.Second):
		t.Fatal("plugin did not complete")
	}

	// Verify commands were extracted from text registration.
	commands := handler.Commands()
	assert.Equal(t, []string{"test-cmd", "test-other"}, commands)
}
