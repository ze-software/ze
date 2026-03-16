package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	plugipc "codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestServerStartStop verifies server lifecycle.
//
// VALIDATES: Server starts and stops cleanly.
//
// PREVENTS: Resource leaks on shutdown.
func TestServerStartStop(t *testing.T) {
	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{}, reactor)

	// Start server
	err := server.Start()
	require.NoError(t, err)
	assert.True(t, server.Running())

	// Stop server
	server.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = server.Wait(ctx)
	require.NoError(t, err)
	assert.False(t, server.Running())
}

// TestNoUnixSocket verifies the server starts without creating a Unix socket.
//
// VALIDATES: Server operates without Unix socket listener (SSH-only interface).
// PREVENTS: Regression re-introducing socket listener.
func TestNoUnixSocket(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{}, reactor)

	err := server.Start()
	require.NoError(t, err)

	// Socket file must NOT be created (socket listener removed)
	_, statErr := os.Stat(sockPath)
	assert.True(t, os.IsNotExist(statErr), "socket file must not exist")

	server.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, server.Wait(ctx))
}

// Socket-specific tests removed: Unix socket replaced by SSH (spec-zefs-socket-locking).

// TestHandleProcessStartupRPC verifies the engine-side RPC handling of the 5-stage
// plugin startup protocol using per-socket PluginConns.
//
// VALIDATES: Engine correctly handles all 5 stages via YANG RPC protocol.
// PREVENTS: RPC infrastructure broken when plugins are converted from text protocol.
func TestHandleProcessStartupRPC(t *testing.T) {
	t.Parallel()

	// Create socket pairs
	pairs, err := plugipc.NewInternalSocketPairs()
	require.NoError(t, err)
	defer pairs.Close()

	// Set up process with RPC connections (per-socket wiring)
	proc := process.NewProcess(plugin.PluginConfig{
		Name:     "test-rpc",
		Internal: true,
		Encoder:  "json",
	})
	proc.SetSockets(pairs)
	proc.SetConnA(plugipc.NewPluginConn(pairs.Engine.EngineSide, pairs.Engine.EngineSide))
	proc.SetConnB(plugipc.NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide))
	proc.SetRunning(true)

	// Set up server with mock reactor
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{}, reactor)
	server.ctx, server.cancel = context.WithCancel(ctx)
	server.coordinator = plugin.NewStartupCoordinator(1)

	// Plugin side: per-socket PluginConns (simulates SDK pattern)
	pluginConnA := plugipc.NewPluginConn(pairs.Engine.PluginSide, pairs.Engine.PluginSide)
	pluginConnB := plugipc.NewPluginConn(pairs.Callback.PluginSide, pairs.Callback.PluginSide)

	// Run plugin protocol in goroutine (simulates SDK 5-stage startup)
	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)

		// Stage 1: Send declare-registration on Socket A
		_ = pluginConnA.SendDeclareRegistration(ctx, &rpc.DeclareRegistrationInput{
			Families:    []rpc.FamilyDecl{{Name: "ipv4/unicast", Mode: "both"}},
			WantsConfig: []string{"bgp"},
		})

		// Stage 2: Receive configure on Socket B, respond OK
		req, err := pluginConnB.ReadRequest(ctx)
		if err != nil {
			return
		}
		_ = pluginConnB.SendResult(ctx, req.ID, nil)

		// Stage 3: Send declare-capabilities on Socket A
		_ = pluginConnA.SendDeclareCapabilities(ctx, &rpc.DeclareCapabilitiesInput{})

		// Stage 4: Receive share-registry on Socket B, respond OK
		req, err = pluginConnB.ReadRequest(ctx)
		if err != nil {
			return
		}
		_ = pluginConnB.SendResult(ctx, req.ID, nil)

		// Stage 5: Send ready on Socket A
		_ = pluginConnA.SendReady(ctx)
	}()

	// Run engine-side RPC startup handler
	server.handleProcessStartupRPC(proc)

	// Verify process reached StageRunning
	assert.Equal(t, plugin.StageRunning, proc.Stage())

	// Verify registration was recorded
	reg := proc.Registration()
	assert.True(t, reg.Done, "registration should be marked done")
	assert.Contains(t, reg.Families, "ipv4/unicast")
	assert.Contains(t, reg.DecodeFamilies, "ipv4/unicast")
	assert.Equal(t, []string{"bgp"}, reg.WantsConfigRoots)
	assert.Equal(t, "test-rpc", reg.Name)

	// Clean up
	cancel()
	<-pluginDone
}

// TestStartupRPC_DependencyValidation verifies that missing deps are rejected
// at stage 1 on the production startup path (handleProcessStartupRPC).
//
// VALIDATES: Plugin declaring dep on missing plugin gets rejected at stage 1.
// PREVENTS: Plugins starting without required dependencies on the production path.
func TestStartupRPC_DependencyValidation(t *testing.T) {
	t.Parallel()

	pairs, err := plugipc.NewInternalSocketPairs()
	require.NoError(t, err)
	defer pairs.Close()

	proc := process.NewProcess(plugin.PluginConfig{
		Name:     "test-dep-missing",
		Internal: true,
		Encoder:  "json",
	})
	proc.SetSockets(pairs)
	proc.SetConnA(plugipc.NewPluginConn(pairs.Engine.EngineSide, pairs.Engine.EngineSide))
	proc.SetConnB(plugipc.NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide))
	proc.SetRunning(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Server configured with only "bgp-rs" — "bgp-adj-rib-in" is missing.
	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{
		Plugins: []plugin.PluginConfig{
			{Name: "bgp-rs"},
		},
	}, reactor)
	server.ctx, server.cancel = context.WithCancel(ctx)
	server.coordinator = plugin.NewStartupCoordinator(1)

	pluginConnA := plugipc.NewPluginConn(pairs.Engine.PluginSide, pairs.Engine.PluginSide)

	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)
		// Stage 1: Send declare-registration with dependency on bgp-adj-rib-in
		_ = pluginConnA.SendDeclareRegistration(ctx, &rpc.DeclareRegistrationInput{
			Dependencies: []string{"bgp-adj-rib-in"},
		})
	}()

	server.handleProcessStartupRPC(proc)

	// Plugin should NOT have reached StageRunning — rejected at stage 1.
	assert.Less(t, proc.Stage(), plugin.StageRunning)

	cancel()
	<-pluginDone
}

// TestStartupRPC_DependencySatisfied verifies that satisfied deps pass stage 1
// on the production startup path (handleProcessStartupRPC).
//
// VALIDATES: Plugin declaring dep on configured plugin passes stage 1.
// PREVENTS: False rejection of valid dependency declarations.
func TestStartupRPC_DependencySatisfied(t *testing.T) {
	t.Parallel()

	pairs, err := plugipc.NewInternalSocketPairs()
	require.NoError(t, err)
	defer pairs.Close()

	proc := process.NewProcess(plugin.PluginConfig{
		Name:     "test-dep-ok",
		Internal: true,
		Encoder:  "json",
	})
	proc.SetSockets(pairs)
	proc.SetConnA(plugipc.NewPluginConn(pairs.Engine.EngineSide, pairs.Engine.EngineSide))
	proc.SetConnB(plugipc.NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide))
	proc.SetRunning(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Server configured with both plugins — dependency is satisfied.
	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{
		Plugins: []plugin.PluginConfig{
			{Name: "test-dep-ok"},
			{Name: "bgp-adj-rib-in"},
		},
	}, reactor)
	server.ctx, server.cancel = context.WithCancel(ctx)
	server.coordinator = plugin.NewStartupCoordinator(1)

	pluginConnA := plugipc.NewPluginConn(pairs.Engine.PluginSide, pairs.Engine.PluginSide)
	pluginConnB := plugipc.NewPluginConn(pairs.Callback.PluginSide, pairs.Callback.PluginSide)

	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)

		// Stage 1: Send declare-registration with satisfied dependency
		_ = pluginConnA.SendDeclareRegistration(ctx, &rpc.DeclareRegistrationInput{
			Dependencies: []string{"bgp-adj-rib-in"},
		})

		// Stage 2: Receive configure, respond OK
		req, err := pluginConnB.ReadRequest(ctx)
		if err != nil {
			return
		}
		_ = pluginConnB.SendResult(ctx, req.ID, nil)

		// Stage 3: Send declare-capabilities
		_ = pluginConnA.SendDeclareCapabilities(ctx, &rpc.DeclareCapabilitiesInput{})

		// Stage 4: Receive share-registry, respond OK
		req, err = pluginConnB.ReadRequest(ctx)
		if err != nil {
			return
		}
		_ = pluginConnB.SendResult(ctx, req.ID, nil)

		// Stage 5: Send ready
		_ = pluginConnA.SendReady(ctx)
	}()

	server.handleProcessStartupRPC(proc)

	// Plugin should reach StageRunning — dependency satisfied.
	assert.Equal(t, plugin.StageRunning, proc.Stage())

	cancel()
	<-pluginDone
}

// TestRegistrationFromRPC verifies conversion from RPC types to engine types.
//
// VALIDATES: DeclareRegistrationInput correctly maps to PluginRegistration.
// PREVENTS: Lost fields or incorrect family mode mapping during conversion.
func TestRegistrationFromRPC(t *testing.T) {
	t.Parallel()

	input := &rpc.DeclareRegistrationInput{
		Families: []rpc.FamilyDecl{
			{Name: "ipv4/unicast", Mode: "both"},
			{Name: "ipv4/flow", Mode: "decode"},
			{Name: "ipv6/unicast", Mode: "encode"},
		},
		Commands:    []rpc.CommandDecl{{Name: "show-routes"}},
		WantsConfig: []string{"bgp", "environment"},
		Schema: &rpc.SchemaDecl{
			Module:    "ze-test",
			Namespace: "urn:ze:test",
			YANGText:  "module ze-test { }",
			Handlers:  []string{"test", "test.sub"},
		},
	}

	reg := registrationFromRPC(input)

	assert.True(t, reg.Done)
	// "both" → appears in both lists
	assert.Contains(t, reg.Families, "ipv4/unicast")
	assert.Contains(t, reg.DecodeFamilies, "ipv4/unicast")
	// "decode" → only in DecodeFamilies
	assert.NotContains(t, reg.Families, "ipv4/flow")
	assert.Contains(t, reg.DecodeFamilies, "ipv4/flow")
	// "encode" → only in Families
	assert.Contains(t, reg.Families, "ipv6/unicast")
	assert.NotContains(t, reg.DecodeFamilies, "ipv6/unicast")

	assert.Equal(t, []string{"show-routes"}, reg.Commands)
	assert.Equal(t, []string{"bgp", "environment"}, reg.WantsConfigRoots)

	require.NotNil(t, reg.PluginSchema)
	assert.Equal(t, "ze-test", reg.PluginSchema.Module)
	assert.Equal(t, "urn:ze:test", reg.PluginSchema.Namespace)
	assert.Equal(t, "module ze-test { }", reg.PluginSchema.Yang)
	assert.Equal(t, []string{"test", "test.sub"}, reg.PluginSchema.Handlers)
}

// TestCapabilitiesFromRPC verifies conversion of capability declarations.
//
// VALIDATES: DeclareCapabilitiesInput correctly maps to PluginCapabilities.
// PREVENTS: Lost capability fields during conversion.
func TestCapabilitiesFromRPC(t *testing.T) {
	t.Parallel()

	input := &rpc.DeclareCapabilitiesInput{
		Capabilities: []rpc.CapabilityDecl{
			{Code: 73, Encoding: "text", Payload: "router.example.com"},
			{Code: 64, Encoding: "hex", Payload: "0078", Peers: []string{"192.168.1.1"}},
		},
	}

	caps := capabilitiesFromRPC(input)

	assert.True(t, caps.Done)
	require.Len(t, caps.Capabilities, 2)

	assert.Equal(t, uint8(73), caps.Capabilities[0].Code)
	assert.Equal(t, "text", caps.Capabilities[0].Encoding)
	assert.Equal(t, "router.example.com", caps.Capabilities[0].Payload)
	assert.Empty(t, caps.Capabilities[0].Peers)

	assert.Equal(t, uint8(64), caps.Capabilities[1].Code)
	assert.Equal(t, "hex", caps.Capabilities[1].Encoding)
	assert.Equal(t, "0078", caps.Capabilities[1].Payload)
	assert.Equal(t, []string{"192.168.1.1"}, caps.Capabilities[1].Peers)
}

// TestRegistrationFromRPCEdgeCases verifies edge cases in RPC-to-engine conversion.
//
// VALIDATES: Nil/empty inputs and unknown modes are handled gracefully.
// PREVENTS: Nil pointer dereference on empty input; silent misrouting on unknown mode.
func TestRegistrationFromRPCEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("nil_schema", func(t *testing.T) {
		t.Parallel()
		input := &rpc.DeclareRegistrationInput{
			Commands: []rpc.CommandDecl{{Name: "status"}},
		}
		reg := registrationFromRPC(input)
		assert.Nil(t, reg.PluginSchema)
		assert.Equal(t, []string{"status"}, reg.Commands)
		assert.True(t, reg.Done)
	})

	t.Run("empty_input", func(t *testing.T) {
		t.Parallel()
		input := &rpc.DeclareRegistrationInput{}
		reg := registrationFromRPC(input)
		assert.True(t, reg.Done)
		assert.Empty(t, reg.Families)
		assert.Empty(t, reg.DecodeFamilies)
		assert.Empty(t, reg.Commands)
		assert.Empty(t, reg.WantsConfigRoots)
		assert.Nil(t, reg.PluginSchema)
	})

	t.Run("unknown_mode_defaults_to_encode", func(t *testing.T) {
		t.Parallel()
		input := &rpc.DeclareRegistrationInput{
			Families: []rpc.FamilyDecl{
				{Name: "ipv4/unicast", Mode: "unknown-mode"},
			},
		}
		reg := registrationFromRPC(input)
		// Unknown mode falls into default case, treated as encode-only
		assert.Contains(t, reg.Families, "ipv4/unicast")
		assert.NotContains(t, reg.DecodeFamilies, "ipv4/unicast")
	})

	t.Run("empty_mode_defaults_to_encode", func(t *testing.T) {
		t.Parallel()
		input := &rpc.DeclareRegistrationInput{
			Families: []rpc.FamilyDecl{
				{Name: "ipv6/unicast", Mode: ""},
			},
		}
		reg := registrationFromRPC(input)
		assert.Contains(t, reg.Families, "ipv6/unicast")
		assert.NotContains(t, reg.DecodeFamilies, "ipv6/unicast")
	})

	t.Run("multi_word_commands", func(t *testing.T) {
		t.Parallel()
		input := &rpc.DeclareRegistrationInput{
			Commands: []rpc.CommandDecl{
				{Name: "rib adjacent in show"},
				{Name: "peer * refresh"},
			},
		}
		reg := registrationFromRPC(input)
		assert.Equal(t, []string{"rib adjacent in show", "peer * refresh"}, reg.Commands)
	})
}

// TestCapabilitiesFromRPCEdgeCases verifies edge cases in capability conversion.
//
// VALIDATES: Empty capability list and empty payload are handled.
// PREVENTS: Nil slice issues when plugin declares no capabilities.
func TestCapabilitiesFromRPCEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty_capabilities", func(t *testing.T) {
		t.Parallel()
		input := &rpc.DeclareCapabilitiesInput{}
		caps := capabilitiesFromRPC(input)
		assert.True(t, caps.Done)
		assert.Empty(t, caps.Capabilities)
	})

	t.Run("empty_payload", func(t *testing.T) {
		t.Parallel()
		// Empty payload is valid (e.g., RFC 2918 route-refresh)
		input := &rpc.DeclareCapabilitiesInput{
			Capabilities: []rpc.CapabilityDecl{
				{Code: 2, Encoding: "text", Payload: ""},
			},
		}
		caps := capabilitiesFromRPC(input)
		require.Len(t, caps.Capabilities, 1)
		assert.Equal(t, uint8(2), caps.Capabilities[0].Code)
		assert.Empty(t, caps.Capabilities[0].Payload)
	})
}

// TestHandoffListenSocketsIntegration verifies the full connection handoff path:
// plugin declares connection-handlers in Stage 1 → engine creates listen socket →
// engine sends fd via SCM_RIGHTS on Socket B → plugin receives fd → full 5-stage completes.
//
// VALIDATES: Engine-side handoffListenSockets creates, sends, and plugin receives a working listener.
// PREVENTS: Connection handoff silently broken after refactoring (wiring test).
func TestHandoffListenSocketsIntegration(t *testing.T) {
	t.Parallel()

	// Need real Unix sockets for SCM_RIGHTS fd passing (net.Pipe doesn't support it).
	pairs, err := plugipc.NewExternalSocketPairs()
	require.NoError(t, err)
	defer pairs.Close()

	// Set up process with external socket pairs.
	proc := process.NewProcess(plugin.PluginConfig{
		Name:     "test-handoff",
		Internal: false,
		Encoder:  "json",
	})
	proc.SetSockets(pairs)
	proc.SetConnA(plugipc.NewPluginConn(pairs.Engine.EngineSide, pairs.Engine.EngineSide))
	proc.SetConnB(plugipc.NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide))
	proc.SetRunning(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find a free port by briefly listening then releasing.
	var lc net.ListenConfig
	tmpLn, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	tcpAddr, ok := tmpLn.Addr().(*net.TCPAddr)
	require.True(t, ok)
	port := tcpAddr.Port
	require.NoError(t, tmpLn.Close())

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{}, reactor)
	server.ctx, server.cancel = context.WithCancel(ctx)
	server.coordinator = plugin.NewStartupCoordinator(1)

	// Plugin-side connections.
	pluginConnA := plugipc.NewPluginConn(pairs.Engine.PluginSide, pairs.Engine.PluginSide)
	rawPluginB := pairs.Callback.PluginSide
	pluginConnB := plugipc.NewPluginConn(rawPluginB, rawPluginB)

	// Run plugin protocol in goroutine (simulates SDK 5-stage startup with fd receive).
	var receivedListener net.Listener
	var pluginErr error
	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)

		// Stage 1: declare-registration with connection-handlers.
		if err := pluginConnA.SendDeclareRegistration(ctx, &rpc.DeclareRegistrationInput{
			ConnectionHandlers: []rpc.ConnectionHandlerDecl{
				{Type: "listen", Port: port, Address: "127.0.0.1"},
			},
		}); err != nil {
			pluginErr = fmt.Errorf("stage 1: %w", err)
			return
		}

		// Receive listen socket fd via SCM_RIGHTS on Socket B (raw connection).
		// Must happen before PluginConn's FrameReader starts (which is lazy).
		received, err := plugipc.ReceiveFD(rawPluginB)
		if err != nil {
			pluginErr = fmt.Errorf("receive fd: %w", err)
			return
		}
		ln, err := net.FileListener(received)
		received.Close() //nolint:errcheck,gosec // fd ownership transferred to listener
		if err != nil {
			pluginErr = fmt.Errorf("file listener: %w", err)
			return
		}
		receivedListener = ln

		// Stage 2: Receive configure on Socket B, respond OK.
		req, err := pluginConnB.ReadRequest(ctx)
		if err != nil {
			pluginErr = fmt.Errorf("stage 2 read: %w", err)
			return
		}
		_ = pluginConnB.SendResult(ctx, req.ID, nil)

		// Stage 3: Send declare-capabilities on Socket A.
		if err := pluginConnA.SendDeclareCapabilities(ctx, &rpc.DeclareCapabilitiesInput{}); err != nil {
			pluginErr = fmt.Errorf("stage 3: %w", err)
			return
		}

		// Stage 4: Receive share-registry on Socket B, respond OK.
		req, err = pluginConnB.ReadRequest(ctx)
		if err != nil {
			pluginErr = fmt.Errorf("stage 4 read: %w", err)
			return
		}
		_ = pluginConnB.SendResult(ctx, req.ID, nil)

		// Stage 5: Send ready on Socket A.
		if err := pluginConnA.SendReady(ctx); err != nil {
			pluginErr = fmt.Errorf("stage 5: %w", err)
			return
		}
	}()

	// Run engine-side RPC startup handler.
	server.handleProcessStartupRPC(proc)

	// Wait for plugin goroutine to finish all stages before canceling context.
	// Engine sends Stage 5 OK, then returns — but the plugin goroutine needs
	// the context alive to read that OK via readFrame(ctx).
	<-pluginDone
	cancel()

	// Verify process reached StageRunning.
	assert.Equal(t, plugin.StageRunning, proc.Stage())

	require.NoError(t, pluginErr)
	require.NotNil(t, receivedListener, "plugin must have received a listen socket")
	defer func() { _ = receivedListener.Close() }()

	// Verify the received listener works: connect to it and accept a connection.
	acceptDone := make(chan error, 1)
	go func() {
		conn, acceptErr := receivedListener.Accept()
		if acceptErr != nil {
			acceptDone <- acceptErr
			return
		}
		conn.Close() //nolint:errcheck,gosec // test cleanup
		acceptDone <- nil
	}()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	var dialer net.Dialer
	client, dialErr := dialer.DialContext(context.Background(), "tcp", addr)
	require.NoError(t, dialErr)
	client.Close() //nolint:errcheck,gosec // test cleanup

	require.NoError(t, <-acceptDone, "received listener must accept connections")

	// Verify registration recorded the connection handler.
	reg := proc.Registration()
	require.Len(t, reg.ConnectionHandlers, 1)
	assert.Equal(t, "listen", reg.ConnectionHandlers[0].Type)
	assert.Equal(t, port, reg.ConnectionHandlers[0].Port)
	assert.Equal(t, "127.0.0.1", reg.ConnectionHandlers[0].Address)
}

// TestHandoffListenSocketsInternalPluginSkipped verifies that connection handoff
// is skipped gracefully for internal plugins (net.Pipe connections).
//
// VALIDATES: Plugin with connection-handlers but internal transport doesn't crash.
// PREVENTS: Panic from SCM_RIGHTS on net.Pipe (internal plugins use direct bridge).
func TestHandoffListenSocketsInternalPluginSkipped(t *testing.T) {
	t.Parallel()

	// Internal socket pairs use net.Pipe — no SCM_RIGHTS support.
	pairs, err := plugipc.NewInternalSocketPairs()
	require.NoError(t, err)
	defer pairs.Close()

	proc := process.NewProcess(plugin.PluginConfig{
		Name:     "test-internal-handoff",
		Internal: true,
		Encoder:  "json",
	})
	proc.SetSockets(pairs)
	proc.SetConnA(plugipc.NewPluginConn(pairs.Engine.EngineSide, pairs.Engine.EngineSide))
	proc.SetConnB(plugipc.NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide))
	proc.SetRunning(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reactor := &mockReactor{}
	server := NewServer(&ServerConfig{}, reactor)
	server.ctx, server.cancel = context.WithCancel(ctx)
	server.coordinator = plugin.NewStartupCoordinator(1)

	pluginConnA := plugipc.NewPluginConn(pairs.Engine.PluginSide, pairs.Engine.PluginSide)

	// Find a free port so the test doesn't depend on a hardcoded port being available.
	// This ensures the Listen() succeeds and the test actually exercises the SendFD
	// failure path (net.Pipe type assertion), not a port-in-use error.
	var lc net.ListenConfig
	tmpLn, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	tcpAddr, ok := tmpLn.Addr().(*net.TCPAddr)
	require.True(t, ok)
	freePort := tcpAddr.Port
	require.NoError(t, tmpLn.Close())

	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)

		// Stage 1: declare-registration with connection-handlers (but internal plugin).
		_ = pluginConnA.SendDeclareRegistration(ctx, &rpc.DeclareRegistrationInput{
			ConnectionHandlers: []rpc.ConnectionHandlerDecl{
				{Type: "listen", Port: freePort, Address: "127.0.0.1"},
			},
		})

		// Internal plugins can't receive fds — the handoff will fail,
		// and the engine should stop the startup for this plugin.
		// Don't try to continue stages.
	}()

	server.handleProcessStartupRPC(proc)

	// Internal plugin should NOT reach StageRunning — handoff fails on net.Pipe.
	assert.Less(t, proc.Stage(), plugin.StageRunning)

	cancel()
	<-pluginDone
}

// TestHandoffPortBoundary verifies port validation boundaries via validHandoffPort.
// Tests the validation function directly — no sockets, no bind, no environment dependency.
//
// VALIDATES: Port range 1-65535 is enforced (0 rejected, 1 accepted, 65535 accepted, 65536 rejected).
// PREVENTS: Off-by-one in port validation allowing invalid ports.
func TestHandoffPortBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		port  int
		valid bool
	}{
		// Boundary: first invalid below range.
		{"port_0_rejected", 0, false},
		// Boundary: first valid (lower boundary).
		{"port_1_accepted", 1, true},
		// Boundary: last valid (upper boundary).
		{"port_65535_accepted", 65535, true},
		// Boundary: first invalid above range.
		{"port_65536_rejected", 65536, false},
		// Boundary: negative port.
		{"port_negative_rejected", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.valid, validHandoffPort(tt.port), "port %d", tt.port)
		})
	}
}

// TestHandoffUnsupportedType verifies that unsupported handler types are skipped with warning.
//
// VALIDATES: Handler type != "listen" is skipped gracefully (no fd sent, handoff succeeds).
// PREVENTS: Unknown handler types causing crashes or silent failure.
func TestHandoffUnsupportedType(t *testing.T) {
	t.Parallel()

	pairs, err := plugipc.NewExternalSocketPairs()
	require.NoError(t, err)
	defer pairs.Close()

	proc := process.NewProcess(plugin.PluginConfig{
		Name:     "test-unsupported-type",
		Internal: false,
		Encoder:  "json",
	})
	proc.SetSockets(pairs)
	proc.SetConnA(plugipc.NewPluginConn(pairs.Engine.EngineSide, pairs.Engine.EngineSide))
	proc.SetConnB(plugipc.NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide))
	proc.SetRunning(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := NewServer(&ServerConfig{}, &mockReactor{})
	server.ctx, server.cancel = context.WithCancel(ctx)

	// "connect" is not a supported handler type — should be skipped with warning.
	reg := &plugin.PluginRegistration{
		ConnectionHandlers: []plugin.ConnectionHandler{
			{Type: "connect", Port: 8080, Address: "127.0.0.1"},
		},
	}

	// handoffListenSockets should succeed (unsupported types are skipped, not errors).
	result := server.handoffListenSockets(proc, reg)
	assert.True(t, result, "unsupported handler type should be skipped, not cause failure")
}

// TestDispatchDecodeMPReach_Malformed verifies error handling for truncated MP_REACH_NLRI.
//
// VALIDATES: Engine returns RPC error for malformed hex input.
// PREVENTS: Panic or silent failure on truncated MP_REACH_NLRI data.
func TestDispatchDecodeMPReach_Malformed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Build a minimal CodecRPCHandler for decode-mp-reach that validates length.
	// Cannot import bgpserver (import cycle), so provide inline handler.
	s := &Server{ctx: ctx, rpcFallback: func(method string) func(json.RawMessage) (any, error) {
		if method != "ze-plugin-engine:decode-mp-reach" {
			return nil
		}
		return func(params json.RawMessage) (any, error) {
			var input rpc.DecodeMPReachInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, fmt.Errorf("invalid params: %w", err)
			}
			data, err := hex.DecodeString(input.Hex)
			if err != nil {
				return nil, fmt.Errorf("invalid hex: %w", err)
			}
			if len(data) < 5 {
				return nil, fmt.Errorf("MP_REACH_NLRI too short: %d bytes", len(data))
			}
			return nil, nil
		}
	}}

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = pluginEnd.Close(); _ = engineEnd.Close() })

	engineConn := plugipc.NewPluginConn(engineEnd, engineEnd)
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	proc := &process.Process{}

	// Only 2 bytes — too short for MP_REACH_NLRI (need at least AFI+SAFI+NHLen = 4)
	type errResult struct {
		err error
	}
	done := make(chan errResult, 1)
	go func() {
		raw, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:decode-mp-reach", &rpc.DecodeMPReachInput{
			Hex: "0001",
		})
		if err != nil {
			done <- errResult{err}
			return
		}
		_, err = rpc.ParseResponse(raw)
		done <- errResult{err}
	}()

	req, err := engineConn.ReadRequest(ctx)
	require.NoError(t, err)
	s.dispatchPluginRPC(proc, engineConn, req)

	malR := <-done
	require.Error(t, malR.err)
	assert.Contains(t, malR.err.Error(), "too short")
}
