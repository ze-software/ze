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
	server, _ := NewServer(&ServerConfig{}, reactor)

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
	server, _ := NewServer(&ServerConfig{}, reactor)

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

	// Create single pipe for bidirectional IPC.
	engineEnd, pluginEnd := net.Pipe()
	defer engineEnd.Close() //nolint:errcheck // test cleanup
	defer pluginEnd.Close() //nolint:errcheck // test cleanup

	// Engine side: MuxPluginConn for the Process.
	engineConn := rpc.NewConn(engineEnd, engineEnd)
	engineMux := rpc.NewMuxConn(engineConn)
	defer engineMux.Close() //nolint:errcheck // test cleanup

	proc := process.NewProcess(plugin.PluginConfig{
		Name:     "test-rpc",
		Internal: true,
		Encoder:  "json",
	})
	proc.SetConn(plugipc.NewMuxPluginConn(engineMux))
	proc.SetRunning(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reactor := &mockReactor{}
	server, _ := NewServer(&ServerConfig{}, reactor)
	server.ctx, server.cancel = context.WithCancel(ctx)
	server.coordinator = plugin.NewStartupCoordinator(1)

	// Plugin side: single MuxConn for bidirectional RPC.
	pluginConn := rpc.NewConn(pluginEnd, pluginEnd)
	pluginMux := rpc.NewMuxConn(pluginConn)
	defer pluginMux.Close() //nolint:errcheck // test cleanup

	// Run plugin protocol in goroutine (simulates SDK 5-stage startup via MuxConn)
	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)

		// Stage 1: Send declare-registration
		if _, err := pluginMux.CallRPC(ctx, "ze-plugin-engine:declare-registration", &rpc.DeclareRegistrationInput{
			Families:    []rpc.FamilyDecl{{Name: "ipv4/unicast", Mode: "both"}},
			WantsConfig: []string{"bgp"},
		}); err != nil {
			return
		}

		// Stage 2: Receive configure, respond OK
		select {
		case req := <-pluginMux.Requests():
			_ = pluginMux.SendOK(ctx, req.ID)
		case <-ctx.Done():
			return
		}

		// Stage 3: Send declare-capabilities
		if _, err := pluginMux.CallRPC(ctx, "ze-plugin-engine:declare-capabilities", &rpc.DeclareCapabilitiesInput{}); err != nil {
			return
		}

		// Stage 4: Receive share-registry, respond OK
		select {
		case req := <-pluginMux.Requests():
			_ = pluginMux.SendOK(ctx, req.ID)
		case <-ctx.Done():
			return
		}

		// Stage 5: Send ready
		if _, err := pluginMux.CallRPC(ctx, "ze-plugin-engine:ready", nil); err != nil {
			return
		}
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

	engineEnd, pluginEnd := net.Pipe()
	defer engineEnd.Close() //nolint:errcheck // test cleanup
	defer pluginEnd.Close() //nolint:errcheck // test cleanup

	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	defer engineMux.Close() //nolint:errcheck // test cleanup

	proc := process.NewProcess(plugin.PluginConfig{
		Name:     "test-dep-missing",
		Internal: true,
		Encoder:  "json",
	})
	proc.SetConn(plugipc.NewMuxPluginConn(engineMux))
	proc.SetRunning(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reactor := &mockReactor{}
	server, _ := NewServer(&ServerConfig{
		Plugins: []plugin.PluginConfig{
			{Name: "bgp-rs"},
		},
	}, reactor)
	server.ctx, server.cancel = context.WithCancel(ctx)
	server.coordinator = plugin.NewStartupCoordinator(1)

	pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
	defer pluginMux.Close() //nolint:errcheck // test cleanup

	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)
		if _, err := pluginMux.CallRPC(ctx, "ze-plugin-engine:declare-registration", &rpc.DeclareRegistrationInput{
			Dependencies: []string{"bgp-adj-rib-in"},
		}); err != nil {
			return // expected: server rejects
		}
	}()

	server.handleProcessStartupRPC(proc)
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

	engineEnd, pluginEnd := net.Pipe()
	defer engineEnd.Close() //nolint:errcheck // test cleanup
	defer pluginEnd.Close() //nolint:errcheck // test cleanup

	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	defer engineMux.Close() //nolint:errcheck // test cleanup

	proc := process.NewProcess(plugin.PluginConfig{
		Name:     "test-dep-ok",
		Internal: true,
		Encoder:  "json",
	})
	proc.SetConn(plugipc.NewMuxPluginConn(engineMux))
	proc.SetRunning(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reactor := &mockReactor{}
	server, _ := NewServer(&ServerConfig{
		Plugins: []plugin.PluginConfig{
			{Name: "test-dep-ok"},
			{Name: "bgp-adj-rib-in"},
		},
	}, reactor)
	server.ctx, server.cancel = context.WithCancel(ctx)
	server.coordinator = plugin.NewStartupCoordinator(1)

	pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
	defer pluginMux.Close() //nolint:errcheck // test cleanup

	pluginDone := make(chan struct{})
	go func() {
		defer close(pluginDone)

		if _, err := pluginMux.CallRPC(ctx, "ze-plugin-engine:declare-registration", &rpc.DeclareRegistrationInput{
			Dependencies: []string{"bgp-adj-rib-in"},
		}); err != nil {
			return
		}

		select {
		case req := <-pluginMux.Requests():
			_ = pluginMux.SendOK(ctx, req.ID) //nolint:errcheck // test
		case <-ctx.Done():
			return
		}

		if _, err := pluginMux.CallRPC(ctx, "ze-plugin-engine:declare-capabilities", &rpc.DeclareCapabilitiesInput{}); err != nil {
			return
		}

		select {
		case req := <-pluginMux.Requests():
			_ = pluginMux.SendOK(ctx, req.ID) //nolint:errcheck // test
		case <-ctx.Done():
			return
		}

		if _, err := pluginMux.CallRPC(ctx, "ze-plugin-engine:ready", nil); err != nil {
			return
		}
	}()

	server.handleProcessStartupRPC(proc)
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
			Handlers:  []string{"test", "test/sub"},
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
	assert.Equal(t, []string{"test", "test/sub"}, reg.PluginSchema.Handlers)
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
		// CallRPC returns RPC errors as Go errors directly.
		_, err := pluginConn.CallRPC(ctx, "ze-plugin-engine:decode-mp-reach", &rpc.DecodeMPReachInput{
			Hex: "0001",
		})
		done <- errResult{err}
	}()

	req, err := engineConn.ReadRequest(ctx)
	require.NoError(t, err)
	s.dispatchPluginRPC(proc, engineConn, req)

	malR := <-done
	require.Error(t, malR.err)
	assert.Contains(t, malR.err.Error(), "too short")
}

// TestHasConfiguredPluginRunCommand verifies that hasConfiguredPlugin matches
// external plugins by Run command when config name differs from registry name.
//
// VALIDATES: Config "adj-rib-in" with Run "ze plugin bgp-adj-rib-in" matches
//
//	registry name "bgp-adj-rib-in".
//
// PREVENTS: Auto-loader launching duplicate plugin instances when config uses
//
//	short names (e.g., "adj-rib-in") and registry uses full names ("bgp-adj-rib-in").
func TestHasConfiguredPluginRunCommand(t *testing.T) {
	s := &Server{
		config: &ServerConfig{
			Plugins: []plugin.PluginConfig{
				{Name: "adj-rib-in", Run: "ze plugin bgp-adj-rib-in", Encoder: "json"},
				{Name: "rpki-decorator", Run: "ze plugin bgp-rpki-decorator", Encoder: "json"},
				{Name: "my-custom-plugin", Encoder: "json", Internal: true},
			},
		},
	}

	// Exact name match
	assert.True(t, s.hasConfiguredPlugin("adj-rib-in"), "exact config name should match")
	assert.True(t, s.hasConfiguredPlugin("my-custom-plugin"), "exact internal name should match")

	// Registry name match via Run command
	assert.True(t, s.hasConfiguredPlugin("bgp-adj-rib-in"), "registry name in Run command should match")
	assert.True(t, s.hasConfiguredPlugin("bgp-rpki-decorator"), "registry name in Run command should match")

	// No match
	assert.False(t, s.hasConfiguredPlugin("bgp-nonexistent"), "non-existent plugin should not match")
	assert.False(t, s.hasConfiguredPlugin(""), "empty name should not match")
}
