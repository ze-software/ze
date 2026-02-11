package plugin

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// mockPluginCommands defines commands the mock plugin declares and how it handles them.
type mockPluginCommands struct {
	decls   []rpc.CommandDecl
	handler func(command string) (status, data string)
}

// startTestHandler creates a SubsystemHandler with in-memory connections and
// completes the 5-stage protocol using a mock plugin goroutine.
// The mock plugin declares the given commands and handles execute-command RPCs.
func startTestHandler(t *testing.T, name string, mock *mockPluginCommands) *SubsystemHandler {
	t.Helper()

	pairs, err := NewInternalSocketPairs()
	require.NoError(t, err)
	t.Cleanup(func() { pairs.Close() })

	proc := NewProcess(PluginConfig{
		Name:     "subsystem-" + name,
		Internal: true,
	})
	proc.sockets = pairs
	proc.engineConnA = NewPluginConn(pairs.Engine.EngineSide, pairs.Engine.EngineSide)
	proc.engineConnB = NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide)
	proc.running.Store(true)

	handler := &SubsystemHandler{
		config: SubsystemConfig{Name: name},
		proc:   proc,
	}

	// Plugin side connections
	pluginConnA := NewPluginConn(pairs.Engine.PluginSide, pairs.Engine.PluginSide)
	pluginConnB := NewPluginConn(pairs.Callback.PluginSide, pairs.Callback.PluginSide)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	// Run mock plugin protocol in goroutine.
	// NOTE: Do NOT defer cancel() here — the engine-side completeProtocol uses
	// the same ctx. If the goroutine exits before completeProtocol finishes its
	// stage 5 SendResult, cancel() would kill the context mid-write, causing
	// "stage 5 respond: context canceled". The t.Cleanup below handles cancel.
	protocolDone := make(chan struct{})
	go func() {
		defer close(protocolDone)

		// Stage 1: Send declare-registration
		reg := &rpc.DeclareRegistrationInput{}
		if mock != nil {
			reg.Commands = mock.decls
		}
		if err := pluginConnA.SendDeclareRegistration(ctx, reg); err != nil {
			return
		}

		// Stage 2: Receive configure, respond OK
		req, err := pluginConnB.ReadRequest(ctx)
		if err != nil {
			return
		}
		if err := pluginConnB.SendResult(ctx, req.ID, nil); err != nil {
			return
		}

		// Stage 3: Send declare-capabilities
		if err := pluginConnA.SendDeclareCapabilities(ctx, &rpc.DeclareCapabilitiesInput{}); err != nil {
			return
		}

		// Stage 4: Receive share-registry, respond OK
		req, err = pluginConnB.ReadRequest(ctx)
		if err != nil {
			return
		}
		if err := pluginConnB.SendResult(ctx, req.ID, nil); err != nil {
			return
		}

		// Stage 5: Send ready
		if err := pluginConnA.SendReady(ctx); err != nil {
			return
		}

		// Command loop (if handler provided)
		if mock != nil && mock.handler != nil {
			for {
				req, err := pluginConnB.ReadRequest(ctx)
				if err != nil {
					return
				}

				var input rpc.ExecuteCommandInput
				if err := json.Unmarshal(req.Params, &input); err != nil {
					return
				}

				status, data := mock.handler(input.Command)
				out := &rpc.ExecuteCommandOutput{Status: status, Data: data}
				if err := pluginConnB.SendResult(ctx, req.ID, out); err != nil {
					return
				}
			}
		}
	}()

	// Complete engine-side protocol
	err = handler.completeProtocol(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		cancel()
		<-protocolDone
	})

	return handler
}

// TestSubsystemRPCProtocol verifies the 5-stage RPC protocol using in-memory connections.
//
// VALIDATES: completeProtocol correctly handles all 5 stages via YANG RPC.
// PREVENTS: Protocol regression when SDK or subsystem changes.
func TestSubsystemRPCProtocol(t *testing.T) {
	t.Parallel()

	mock := &mockPluginCommands{
		decls: []rpc.CommandDecl{
			{Name: "bgp cache list", Description: "List cache entries"},
			{Name: "bgp cache retain", Description: "Retain cache entry"},
			{Name: "bgp cache release", Description: "Release cache entry"},
		},
	}

	handler := startTestHandler(t, "cache", mock)

	// Verify commands were extracted from registration
	commands := handler.Commands()
	assert.Contains(t, commands, "bgp cache list")
	assert.Contains(t, commands, "bgp cache retain")
	assert.Contains(t, commands, "bgp cache release")
}

// TestSubsystemRPCCommand verifies command execution through the RPC protocol.
//
// VALIDATES: After completing 5-stage protocol, commands are routed and return responses.
// PREVENTS: Command routing failures after protocol completion.
func TestSubsystemRPCCommand(t *testing.T) {
	t.Parallel()

	mock := &mockPluginCommands{
		decls: []rpc.CommandDecl{
			{Name: "bgp session ping", Description: "Ping session"},
		},
		handler: func(command string) (string, string) {
			if command == "bgp session ping" {
				return "done", `{"pong":true}`
			}
			return "error", "unknown command: " + command
		},
	}

	handler := startTestHandler(t, "session", mock)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := handler.Handle(ctx, "bgp session ping")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(string)
	require.True(t, ok, "expected string data")
	assert.Contains(t, data, "pong")
}

// TestSubsystemShutdown verifies graceful shutdown closes connections.
//
// VALIDATES: Stop() closes connections without panic.
// PREVENTS: Resource leaks after subsystem shutdown.
func TestSubsystemShutdown(t *testing.T) {
	t.Parallel()

	handler := startTestHandler(t, "cache", nil)

	assert.True(t, handler.Running())

	// Stop closes connections and cancels context.
	// For in-memory connections, running is cleared by the process
	// wait goroutine (not present in test setup), so we just verify
	// Stop() completes without panic and connections are closed.
	handler.Stop()

	// After Stop, Handle should fail (connections closed)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err := handler.Handle(ctx, "anything")
	assert.Error(t, err)
}

// TestSubsystemHandler verifies the SubsystemHandler wrapper.
//
// VALIDATES: SubsystemHandler completes protocol and routes commands.
// PREVENTS: Handler failing to complete protocol or route commands.
func TestSubsystemHandler(t *testing.T) {
	t.Parallel()

	mock := &mockPluginCommands{
		decls: []rpc.CommandDecl{
			{Name: "bgp session ping", Description: "Ping session"},
			{Name: "bgp session bye", Description: "Session goodbye"},
		},
		handler: func(command string) (string, string) {
			switch command {
			case "bgp session ping":
				return "done", `{"pong":true}`
			case "bgp session bye":
				return "done", `{"status":"goodbye"}`
			default:
				return "error", "unknown command: " + command
			}
		},
	}

	handler := startTestHandler(t, "session", mock)

	// Verify commands were declared
	commands := handler.Commands()
	assert.Contains(t, commands, "bgp session ping")
	assert.Contains(t, commands, "bgp session bye")

	// Send a command
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := handler.Handle(ctx, "bgp session ping")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
}

// TestSubsystemManager verifies the SubsystemManager with pre-started handlers.
//
// VALIDATES: Manager tracks handlers and routes commands by prefix.
// PREVENTS: Command routing failures in manager.
func TestSubsystemManager(t *testing.T) {
	t.Parallel()

	cacheMock := &mockPluginCommands{
		decls: []rpc.CommandDecl{
			{Name: "bgp cache list", Description: "List cache entries"},
		},
		handler: func(command string) (string, string) {
			if command == "bgp cache list" {
				return "done", "[]"
			}
			return "error", "unknown"
		},
	}

	sessionMock := &mockPluginCommands{
		decls: []rpc.CommandDecl{
			{Name: "bgp session ping", Description: "Ping session"},
		},
		handler: func(command string) (string, string) {
			if command == "bgp session ping" {
				return "done", `{"pong":true}`
			}
			return "error", "unknown"
		},
	}

	cacheHandler := startTestHandler(t, "cache", cacheMock)
	sessionHandler := startTestHandler(t, "session", sessionMock)

	manager := NewSubsystemManager()
	manager.mu.Lock()
	manager.handlers["cache"] = cacheHandler
	manager.handlers["session"] = sessionHandler
	manager.mu.Unlock()

	// Verify both accessible
	assert.NotNil(t, manager.Get("cache"))
	assert.NotNil(t, manager.Get("session"))

	// Find handler for command
	h := manager.FindHandler("bgp cache list")
	require.NotNil(t, h)
	assert.Equal(t, "cache", h.Name())

	h = manager.FindHandler("bgp session ping")
	require.NotNil(t, h)
	assert.Equal(t, "session", h.Name())

	// All commands
	allCmds := manager.AllCommands()
	assert.Contains(t, allCmds, "bgp cache list")
	assert.Contains(t, allCmds, "bgp session ping")
}

// TestDispatcherSubsystemIntegration verifies Dispatcher routes to subsystems.
//
// VALIDATES: Dispatcher routes commands to subsystem handlers.
// PREVENTS: Commands not being routed to subsystems.
func TestDispatcherSubsystemIntegration(t *testing.T) {
	t.Parallel()

	mock := &mockPluginCommands{
		decls: []rpc.CommandDecl{
			{Name: "bgp session ping", Description: "Ping session"},
		},
		handler: func(command string) (string, string) {
			if command == "bgp session ping" {
				return "done", `{"pong":true}`
			}
			return "error", "unknown"
		},
	}

	sessionHandler := startTestHandler(t, "session", mock)

	d := NewDispatcher()
	manager := NewSubsystemManager()
	manager.mu.Lock()
	manager.handlers["session"] = sessionHandler
	manager.mu.Unlock()
	d.SetSubsystems(manager)

	// Dispatch command to subsystem
	resp, err := d.Dispatch(nil, "bgp session ping")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	if data, ok := resp.Data.(string); ok {
		assert.Contains(t, data, "pong")
	}
}
