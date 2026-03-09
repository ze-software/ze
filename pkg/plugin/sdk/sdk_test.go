package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// engineSide wraps two rpc.Conns for the engine's perspective:
//   - server: reads requests from Socket A (plugin→engine), responds on Socket A
//   - client: sends requests on Socket B (engine→plugin), reads responses from Socket B
type engineSide struct {
	server *rpc.Conn // Socket A: engine is server
	client *rpc.Conn // Socket B: engine is client
}

// newTestPair creates a connected plugin SDK + engine side using net.Pipe.
// Returns the SDK plugin and the engine-side helper.
func newTestPair(t *testing.T) (*Plugin, *engineSide) {
	t.Helper()

	// Socket A: Plugin → Engine
	aPlugin, aEngine := net.Pipe()
	// Socket B: Engine → Plugin
	bPlugin, bEngine := net.Pipe()

	t.Cleanup(func() {
		for _, c := range []net.Conn{aPlugin, aEngine, bPlugin, bEngine} {
			if err := c.Close(); err != nil {
				t.Logf("cleanup close: %v", err)
			}
		}
	})

	p := NewWithConn("test-plugin", aPlugin, bPlugin)

	engine := &engineSide{
		server: rpc.NewConn(aEngine, aEngine),
		client: rpc.NewConn(bEngine, bEngine),
	}

	return p, engine
}

// callAndExpectOK sends an RPC request and expects a successful response.
// This is a test-only helper that wraps CallRPC + CheckResponse.
func callAndExpectOK(ctx context.Context, c *rpc.Conn, method string, params any) error {
	raw, err := c.CallRPC(ctx, method, params)
	if err != nil {
		return err
	}
	return rpc.CheckResponse(raw)
}

// TestSDKStartup verifies the full 5-stage startup protocol via SDK.
//
// VALIDATES: SDK handles all 5 startup stages correctly.
// PREVENTS: Protocol violation during startup sequence.
func TestSDKStartup(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	// Configure SDK
	reg := Registration{
		Families: []FamilyDecl{
			{Name: "ipv4/unicast", Mode: "both"},
		},
		Commands: []CommandDecl{
			{Name: "show-routes", Description: "Show routes"},
		},
		WantsConfig: []string{"bgp"},
	}

	configReceived := make(chan []ConfigSection, 1)
	registryReceived := make(chan []RegistryCommand, 1)

	p.OnConfigure(func(sections []ConfigSection) error {
		configReceived <- sections
		return nil
	})
	p.OnShareRegistry(func(commands []RegistryCommand) {
		registryReceived <- commands
	})

	// Run plugin in background
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, reg)
	}()

	// === Stage 1: Engine reads declare-registration ===
	req, err := engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)

	var regInput Registration
	require.NoError(t, json.Unmarshal(req.Params, &regInput))
	assert.Equal(t, 1, len(regInput.Families))
	assert.Equal(t, "ipv4/unicast", regInput.Families[0].Name)
	assert.Equal(t, 1, len(regInput.Commands))
	assert.Equal(t, []string{"bgp"}, regInput.WantsConfig)

	// Respond
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// === Stage 2: Engine sends configure ===
	sections := []ConfigSection{
		{Root: "bgp", Data: `{"router-id":"1.2.3.4"}`},
	}
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{Sections: sections}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:configure", configInput))

	// Verify callback was called
	select {
	case got := <-configReceived:
		assert.Equal(t, 1, len(got))
		assert.Equal(t, "bgp", got[0].Root)
	case <-time.After(time.Second):
		t.Fatal("configure callback not called")
	}

	// === Stage 3: Engine reads declare-capabilities ===
	req, err = engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// === Stage 4: Engine sends share-registry ===
	commands := []RegistryCommand{
		{Name: "show-routes", Plugin: "test-plugin", Encoding: "text"},
	}
	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{Commands: commands}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:share-registry", registryInput))

	select {
	case got := <-registryReceived:
		assert.Equal(t, 1, len(got))
		assert.Equal(t, "show-routes", got[0].Name)
	case <-time.After(time.Second):
		t.Fatal("share-registry callback not called")
	}

	// === Stage 5: Engine reads ready ===
	req, err = engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// === Shutdown: Engine sends bye ===
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "test-complete"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	// Plugin should exit cleanly
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit after bye")
	}
}

// TestSDKEventDelivery verifies event delivery via SDK callbacks.
//
// VALIDATES: Events delivered via callback are forwarded to handler.
// PREVENTS: Events lost during runtime.
func TestSDKEventDelivery(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	eventReceived := make(chan string, 1)
	p.OnEvent(func(event string) error {
		eventReceived <- event
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	// Complete startup (stages 1-5)
	completeStartup(t, ctx, engine)

	// === Runtime: deliver event ===
	eventJSON := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`
	eventInput := struct {
		Event string `json:"event"`
	}{Event: eventJSON}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:deliver-event", eventInput))

	select {
	case got := <-eventReceived:
		assert.Equal(t, eventJSON, got)
	case <-time.After(time.Second):
		t.Fatal("event callback not called")
	}

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKByeCallback verifies bye callback is invoked.
//
// VALIDATES: Bye reason is forwarded to handler.
// PREVENTS: Shutdown reason lost.
func TestSDKByeCallback(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	byeReason := make(chan string, 1)
	p.OnBye(func(reason string) {
		byeReason <- reason
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send bye
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "shutdown"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case reason := <-byeReason:
		assert.Equal(t, "shutdown", reason)
	case <-time.After(time.Second):
		t.Fatal("bye callback not called")
	}

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// completeStartup runs through all 5 startup stages from the engine side.
func completeStartup(t *testing.T, ctx context.Context, engine *engineSide) {
	t.Helper()

	// Stage 1: read and respond to declare-registration
	req, err := engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// Stage 2: send configure
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:configure", configInput))

	// Stage 3: read and respond to declare-capabilities
	req, err = engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// Stage 4: send share-registry
	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:share-registry", registryInput))

	// Stage 5: read and respond to ready
	req, err = engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engine.server.SendOK(ctx, req.ID))
}

// TestSDKEncodeNLRI verifies encode-nlri dispatch in the event loop.
//
// VALIDATES: Engine can request NLRI encoding and receive hex result.
// PREVENTS: encode-nlri rejected as unknown method during runtime.
func TestSDKEncodeNLRI(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	p.OnEncodeNLRI(func(family string, args []string) (string, error) {
		return "180a0000", nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send encode-nlri request
	encInput := struct {
		Family string   `json:"family"`
		Args   []string `json:"args,omitempty"`
	}{Family: "ipv4/unicast", Args: []string{"10.0.0.0/24"}}

	raw, err := engine.client.CallRPC(ctx, "ze-plugin-callback:encode-nlri", encInput)
	require.NoError(t, err)

	// Parse response — should contain hex result
	var probe struct {
		Error  string          `json:"error,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
	}
	require.NoError(t, json.Unmarshal(raw, &probe))
	assert.Empty(t, probe.Error)

	var result struct {
		Hex string `json:"hex"`
	}
	require.NoError(t, json.Unmarshal(probe.Result, &result))
	assert.Equal(t, "180a0000", result.Hex)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKDecodeNLRI verifies decode-nlri dispatch in the event loop.
//
// VALIDATES: Engine can request NLRI decoding and receive JSON result.
// PREVENTS: decode-nlri rejected as unknown method during runtime.
func TestSDKDecodeNLRI(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	p.OnDecodeNLRI(func(family string, hex string) (string, error) {
		return `["10.0.0.0/24"]`, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send decode-nlri request
	decInput := struct {
		Family string `json:"family"`
		Hex    string `json:"hex"`
	}{Family: "ipv4/unicast", Hex: "180a0000"}

	raw, err := engine.client.CallRPC(ctx, "ze-plugin-callback:decode-nlri", decInput)
	require.NoError(t, err)

	// Parse response — should contain json result
	var probe struct {
		Error  string          `json:"error,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
	}
	require.NoError(t, json.Unmarshal(raw, &probe))
	assert.Empty(t, probe.Error)

	var result struct {
		JSON string `json:"json"`
	}
	require.NoError(t, json.Unmarshal(probe.Result, &result))
	assert.Equal(t, `["10.0.0.0/24"]`, result.JSON)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKCapabilities verifies that capabilities declared via Registration are sent in Stage 3.
//
// VALIDATES: Capabilities from Registration are sent in declare-capabilities.
// PREVENTS: Stage 3 always sending empty capabilities.
func TestSDKCapabilities(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	reg := Registration{
		Families: []FamilyDecl{
			{Name: "ipv4/unicast", Mode: "both"},
		},
	}

	caps := []CapabilityDecl{
		{Code: 64, Encoding: "hex", Payload: "0001"},
	}
	p.SetCapabilities(caps)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, reg)
	}()

	// Stage 1: read declare-registration
	req, err := engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// Stage 2: send configure
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:configure", configInput))

	// Stage 3: read declare-capabilities — should contain our caps
	req, err = engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)

	var capsInput DeclareCapabilitiesInput
	require.NoError(t, json.Unmarshal(req.Params, &capsInput))
	require.Len(t, capsInput.Capabilities, 1)
	assert.Equal(t, uint8(64), capsInput.Capabilities[0].Code)
	assert.Equal(t, "hex", capsInput.Capabilities[0].Encoding)
	assert.Equal(t, "0001", capsInput.Capabilities[0].Payload)

	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// Stage 4: send share-registry
	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:share-registry", registryInput))

	// Stage 5: read ready
	req, err = engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKDecodeCapability verifies decode-capability dispatch in the event loop.
//
// VALIDATES: Engine can request capability decoding and receive JSON result.
// PREVENTS: decode-capability rejected as unknown method during runtime.
func TestSDKDecodeCapability(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	p.OnDecodeCapability(func(code uint8, hex string) (string, error) {
		return `{"hostname":"router1"}`, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send decode-capability request
	decCapInput := struct {
		Code uint8  `json:"code"`
		Hex  string `json:"hex"`
	}{Code: 73, Hex: "07686f73746e616d65"}

	raw, err := engine.client.CallRPC(ctx, "ze-plugin-callback:decode-capability", decCapInput)
	require.NoError(t, err)

	var probe struct {
		Error  string          `json:"error,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
	}
	require.NoError(t, json.Unmarshal(raw, &probe))
	assert.Empty(t, probe.Error)

	var result struct {
		JSON string `json:"json"`
	}
	require.NoError(t, json.Unmarshal(probe.Result, &result))
	assert.Equal(t, `{"hostname":"router1"}`, result.JSON)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKExecuteCommand verifies execute-command dispatch in the event loop.
//
// VALIDATES: Engine can request command execution and receive status + data.
// PREVENTS: execute-command rejected as unknown method during runtime.
func TestSDKExecuteCommand(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
		return "done", `{"routes":[]}`, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send execute-command request
	execInput := struct {
		Serial  string   `json:"serial"`
		Command string   `json:"command"`
		Args    []string `json:"args,omitempty"`
		Peer    string   `json:"peer,omitempty"`
	}{Serial: "abc123", Command: "show-routes", Args: []string{"ipv4"}, Peer: "10.0.0.1"}

	raw, err := engine.client.CallRPC(ctx, "ze-plugin-callback:execute-command", execInput)
	require.NoError(t, err)

	var probe struct {
		Error  string          `json:"error,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
	}
	require.NoError(t, json.Unmarshal(raw, &probe))
	assert.Empty(t, probe.Error)

	var result struct {
		Status string `json:"status"`
		Data   string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(probe.Result, &result))
	assert.Equal(t, "done", result.Status)
	assert.Equal(t, `{"routes":[]}`, result.Data)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKDispatchConfigVerify verifies config-verify dispatch in the event loop.
//
// VALIDATES: Engine sends config-verify, SDK dispatches to registered handler.
// PREVENTS: config-verify rejected as unknown method during runtime.
func TestSDKDispatchConfigVerify(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	verifyReceived := make(chan []ConfigSection, 1)
	p.OnConfigVerify(func(sections []ConfigSection) error {
		verifyReceived <- sections
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send config-verify request
	verifyInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{Sections: []ConfigSection{{Root: "bgp", Data: `{"router-id":"1.2.3.4"}`}}}

	raw, err := engine.client.CallRPC(ctx, "ze-plugin-callback:config-verify", verifyInput)
	require.NoError(t, err)

	// Parse response — should be OK with status
	var probe struct {
		Error  string          `json:"error,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
	}
	require.NoError(t, json.Unmarshal(raw, &probe))
	assert.Empty(t, probe.Error)

	var result struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(probe.Result, &result))
	assert.Equal(t, "ok", result.Status)

	// Verify callback was invoked
	select {
	case got := <-verifyReceived:
		require.Len(t, got, 1)
		assert.Equal(t, "bgp", got[0].Root)
	case <-time.After(time.Second):
		t.Fatal("config-verify callback not called")
	}

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKDispatchConfigApply verifies config-apply dispatch in the event loop.
//
// VALIDATES: Engine sends config-apply, SDK dispatches to registered handler.
// PREVENTS: config-apply rejected as unknown method during runtime.
func TestSDKDispatchConfigApply(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	applyReceived := make(chan []ConfigDiffSection, 1)
	p.OnConfigApply(func(sections []ConfigDiffSection) error {
		applyReceived <- sections
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send config-apply request
	applyInput := struct {
		Sections []ConfigDiffSection `json:"sections"`
	}{Sections: []ConfigDiffSection{{Root: "bgp", Added: `{"peer":{"p1":{}}}`, Changed: `{"router-id":"5.6.7.8"}`}}}

	raw, err := engine.client.CallRPC(ctx, "ze-plugin-callback:config-apply", applyInput)
	require.NoError(t, err)

	var probe struct {
		Error  string          `json:"error,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
	}
	require.NoError(t, json.Unmarshal(raw, &probe))
	assert.Empty(t, probe.Error)

	var result struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(probe.Result, &result))
	assert.Equal(t, "ok", result.Status)

	// Verify callback was invoked
	select {
	case got := <-applyReceived:
		require.Len(t, got, 1)
		assert.Equal(t, "bgp", got[0].Root)
		assert.Equal(t, `{"peer":{"p1":{}}}`, got[0].Added)
	case <-time.After(time.Second):
		t.Fatal("config-apply callback not called")
	}

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKConfigVerifyReject verifies handler rejection produces status "error" response.
//
// VALIDATES: Handler returning error → SDK sends {status:"error", error:"..."}.
// PREVENTS: Validation rejections being silently swallowed or misformatted.
func TestSDKConfigVerifyReject(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	p.OnConfigVerify(func(sections []ConfigSection) error {
		return fmt.Errorf("invalid config: missing router-id")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send config-verify — handler will reject
	verifyInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{Sections: []ConfigSection{{Root: "bgp", Data: `{"invalid":true}`}}}

	raw, err := engine.client.CallRPC(ctx, "ze-plugin-callback:config-verify", verifyInput)
	require.NoError(t, err)

	// Response should be a result (not RPC error) with status "error"
	var probe struct {
		Error  string          `json:"error,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
	}
	require.NoError(t, json.Unmarshal(raw, &probe))
	assert.Empty(t, probe.Error, "rejection should be in result, not RPC error")

	var result struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(probe.Result, &result))
	assert.Equal(t, "error", result.Status)
	assert.Equal(t, "invalid config: missing router-id", result.Error)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKConfigApplyReject verifies handler rejection produces status "error" response.
//
// VALIDATES: Handler returning error → SDK sends {status:"error", error:"..."}.
// PREVENTS: Apply rejections being silently swallowed or misformatted.
func TestSDKConfigApplyReject(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	p.OnConfigApply(func(sections []ConfigDiffSection) error {
		return fmt.Errorf("cannot apply: peer limit exceeded")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send config-apply — handler will reject
	applyInput := struct {
		Sections []ConfigDiffSection `json:"sections"`
	}{Sections: []ConfigDiffSection{{Root: "bgp", Added: `{"peer":{"p99":{}}}`}}}

	raw, err := engine.client.CallRPC(ctx, "ze-plugin-callback:config-apply", applyInput)
	require.NoError(t, err)

	var probe struct {
		Error  string          `json:"error,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
	}
	require.NoError(t, json.Unmarshal(raw, &probe))
	assert.Empty(t, probe.Error, "rejection should be in result, not RPC error")

	var result struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(probe.Result, &result))
	assert.Equal(t, "error", result.Status)
	assert.Equal(t, "cannot apply: peer limit exceeded", result.Error)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKConfigVerifyNoHandler verifies graceful no-op when no config-verify handler is set.
//
// VALIDATES: config-verify with no handler returns OK (not error).
// PREVENTS: Plugins without config handling crashing on config-verify.
func TestSDKConfigVerifyNoHandler(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)
	// No OnConfigVerify registered

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send config-verify — no handler should still get OK
	verifyInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{Sections: []ConfigSection{{Root: "bgp", Data: `{}`}}}

	raw, err := engine.client.CallRPC(ctx, "ze-plugin-callback:config-verify", verifyInput)
	require.NoError(t, err)

	var probe struct {
		Error  string          `json:"error,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
	}
	require.NoError(t, json.Unmarshal(raw, &probe))
	assert.Empty(t, probe.Error, "no-handler config-verify should succeed, not error")

	var result struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(probe.Result, &result))
	assert.Equal(t, "ok", result.Status)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKConfigApplyNoHandler verifies graceful no-op when no config-apply handler is set.
//
// VALIDATES: config-apply with no handler returns OK (not error).
// PREVENTS: Plugins without config handling crashing on config-apply.
func TestSDKConfigApplyNoHandler(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)
	// No OnConfigApply registered

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send config-apply — no handler should still get OK
	applyInput := struct {
		Sections []ConfigDiffSection `json:"sections"`
	}{Sections: []ConfigDiffSection{{Root: "bgp", Added: `{}`}}}

	raw, err := engine.client.CallRPC(ctx, "ze-plugin-callback:config-apply", applyInput)
	require.NoError(t, err)

	var probe struct {
		Error  string          `json:"error,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
	}
	require.NoError(t, json.Unmarshal(raw, &probe))
	assert.Empty(t, probe.Error, "no-handler config-apply should succeed, not error")

	var result struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(probe.Result, &result))
	assert.Equal(t, "ok", result.Status)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKUpdateRoute verifies the plugin can call update-route on the engine.
//
// VALIDATES: SDK UpdateRoute sends RPC to engine and parses response.
// PREVENTS: Plugin unable to inject routes at runtime.
func TestSDKUpdateRoute(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Synchronize: send a no-op event to confirm the event loop is running.
	// This ensures the Stage 5 ready callRPC goroutine on engineConn has
	// completed before we call UpdateRoute on the same connection.
	syncEvent := struct {
		Event string `json:"event"`
	}{Event: "{}"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:deliver-event", syncEvent))

	// Plugin calls UpdateRoute in background (during event loop, we need
	// to handle the engine's response while also keeping the event loop alive)
	routeDone := make(chan error, 1)
	go func() {
		affected, sent, err := p.UpdateRoute(ctx, "*", "announce route 10.0.0.0/24 next-hop 1.1.1.1")
		if err != nil {
			routeDone <- err
			return
		}
		if affected != 3 || sent != 1 {
			routeDone <- fmt.Errorf("unexpected: affected=%d sent=%d", affected, sent)
			return
		}
		routeDone <- nil
	}()

	// Engine reads update-route request on Socket A
	req, err := engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:update-route", req.Method)

	// Respond with result
	routeResult := struct {
		PeersAffected uint32 `json:"peers-affected"`
		RoutesSent    uint32 `json:"routes-sent"`
	}{PeersAffected: 3, RoutesSent: 1}
	require.NoError(t, engine.server.SendResult(ctx, req.ID, routeResult))

	require.NoError(t, <-routeDone)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKDispatchCommand verifies the plugin can call dispatch-command on the engine
// and receive the full {status, data} response.
//
// VALIDATES: SDK DispatchCommand sends RPC to engine and parses response.
// PREVENTS: Plugin unable to dispatch commands to other plugins at runtime.
func TestSDKDispatchCommand(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Synchronize: confirm event loop is running.
	syncEvent := struct {
		Event string `json:"event"`
	}{Event: "{}"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:deliver-event", syncEvent))

	// Plugin calls DispatchCommand in background
	dispatchDone := make(chan error, 1)
	go func() {
		status, data, err := p.DispatchCommand(ctx, "bgp rib list")
		if err != nil {
			dispatchDone <- err
			return
		}
		if status != "done" {
			dispatchDone <- fmt.Errorf("unexpected status: %s", status)
			return
		}
		if data != `{"last-index":42}` {
			dispatchDone <- fmt.Errorf("unexpected data: %s", data)
			return
		}
		dispatchDone <- nil
	}()

	// Engine reads dispatch-command request on Socket A
	req, err := engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:dispatch-command", req.Method)

	// Respond with result
	dispatchResult := rpc.DispatchCommandOutput{
		Status: rpc.StatusDone,
		Data:   `{"last-index":42}`,
	}
	require.NoError(t, engine.server.SendResult(ctx, req.ID, dispatchResult))

	require.NoError(t, <-dispatchDone)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestNewFromFDs verifies plugin creation from file descriptors.
//
// VALIDATES: NewFromFDs creates a working plugin from socketpair FDs.
// PREVENTS: External plugin SDK constructor not working.
func TestNewFromFDs(t *testing.T) {
	t.Parallel()

	// Create real socketpairs
	pairs, err := newExternalTestPairs(t)
	require.NoError(t, err)

	engineFD := pairs.engineFD
	callbackFD := pairs.callbackFD

	p, err := NewFromFDs("test-plugin", engineFD, callbackFD)
	require.NoError(t, err)
	t.Cleanup(func() {
		if closeErr := p.Close(); closeErr != nil {
			t.Logf("close plugin: %v", closeErr)
		}
	})

	// Verify we can communicate — send a frame from engine side, read from plugin
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Engine writes a request on callback socket (Socket B)
	go func() {
		if writeErr := pairs.callbackEngine.WriteFrame(map[string]any{
			"method": "ze-plugin-callback:bye",
			"params": map[string]string{"reason": "test"},
			"id":     1,
		}); writeErr != nil {
			t.Logf("WriteFrame: %v", writeErr)
		}
	}()

	// Plugin reads it
	req, err := p.callbackConn.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:bye", req.Method)
}

// TestSDKCloseUnblocksRead verifies that Close() unblocks goroutines waiting on Read().
//
// VALIDATES: Close() causes blocked ReadRequest to return an error promptly.
// PREVENTS: Goroutine leaks when plugin is shut down via Close().
func TestSDKCloseUnblocksRead(t *testing.T) {
	t.Parallel()

	p, _ := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a goroutine that blocks on ReadRequest (no data will ever arrive).
	readDone := make(chan error, 1)
	go func() {
		_, err := p.callbackConn.ReadRequest(ctx)
		readDone <- err
	}()

	// Give the goroutine time to block on Read().
	time.Sleep(50 * time.Millisecond)

	// Close should unblock the read.
	require.NoError(t, p.Close())

	select {
	case err := <-readDone:
		// We expect an error (io.EOF or "use of closed network connection").
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Close() did not unblock ReadRequest")
	}
}

// TestSDKConnectionCloseCleanShutdown verifies that closing the callback connection
// causes Run() to return nil (clean shutdown), not an error.
//
// VALIDATES: Connection close during event loop is treated as clean shutdown.
// PREVENTS: Scary "plugin failed" ERROR logs when engine shuts down normally.
func TestSDKConnectionCloseCleanShutdown(t *testing.T) {
	t.Parallel()

	// Use raw net.Pipe so we can close the engine side independently.
	aPlugin, aEngine := net.Pipe()
	bPlugin, bEngine := net.Pipe()
	t.Cleanup(func() {
		for _, c := range []net.Conn{aPlugin, aEngine, bPlugin, bEngine} {
			if err := c.Close(); err != nil {
				t.Logf("cleanup close: %v", err)
			}
		}
	})

	p := NewWithConn("test-plugin", aPlugin, bPlugin)
	engine := &engineSide{
		server: rpc.NewConn(aEngine, aEngine),
		client: rpc.NewConn(bEngine, bEngine),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Simulate engine shutdown: close the callback connection (Socket B engine side).
	// This is what Process.Stop() does for internal plugins.
	require.NoError(t, bEngine.Close())

	select {
	case err := <-errCh:
		// Must be nil — connection close is a clean shutdown, not an error.
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit after connection close")
	}
}

// externalTestPairs holds real socketpair FDs for testing NewFromFDs.
type externalTestPairs struct {
	engineFD       int       // Plugin's end of Socket A (FD for plugin→engine)
	callbackFD     int       // Plugin's end of Socket B (FD for engine→plugin)
	callbackEngine *rpc.Conn // Engine's end of Socket B (for writing requests)
}

// newExternalTestPairs creates real Unix socketpairs for testing external plugin FD passing.
func newExternalTestPairs(t *testing.T) (*externalTestPairs, error) {
	t.Helper()

	// Socket A: Plugin→Engine
	aFDs, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, fmt.Errorf("socketpair A: %w", err)
	}

	// Socket B: Engine→Plugin
	bFDs, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		closeFDs(t, aFDs[0], aFDs[1])
		return nil, fmt.Errorf("socketpair B: %w", err)
	}

	// Engine's end of Socket B → wrap as net.Conn for writing
	bEngineFile := os.NewFile(uintptr(bFDs[0]), "callback-engine")
	bEngineConn, err := net.FileConn(bEngineFile)
	if closeErr := bEngineFile.Close(); closeErr != nil {
		t.Logf("close bEngineFile: %v", closeErr)
	}
	if err != nil {
		closeFDs(t, aFDs[0], aFDs[1], bFDs[1])
		return nil, fmt.Errorf("fileconn B engine: %w", err)
	}

	t.Cleanup(func() {
		closeFDs(t, aFDs[0])
		if closeErr := bEngineConn.Close(); closeErr != nil {
			t.Logf("close bEngineConn: %v", closeErr)
		}
	})

	return &externalTestPairs{
		engineFD:       aFDs[1],
		callbackFD:     bFDs[1],
		callbackEngine: rpc.NewConn(bEngineConn, bEngineConn),
	}, nil
}

// closeFDs closes file descriptors, logging any errors.
func closeFDs(t *testing.T, fds ...int) {
	t.Helper()
	for _, fd := range fds {
		if err := syscall.Close(fd); err != nil {
			t.Logf("close fd %d: %v", fd, err)
		}
	}
}

// TestSDKDispatchValidateOpen verifies validate-open dispatch in the event loop.
//
// VALIDATES: Engine sends validate-open, SDK dispatches to registered handler, returns accept.
// PREVENTS: validate-open rejected as unknown method during runtime.
func TestSDKDispatchValidateOpen(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	type validateOpenCall struct {
		input rpc.ValidateOpenInput
	}
	received := make(chan validateOpenCall, 1)

	p.OnValidateOpen(func(input *rpc.ValidateOpenInput) *rpc.ValidateOpenOutput {
		received <- validateOpenCall{input: *input}
		return &rpc.ValidateOpenOutput{Accept: true}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	// Verify WantsValidateOpen is auto-set in Stage 1
	req, err := engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)

	var regInput rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(req.Params, &regInput))
	assert.True(t, regInput.WantsValidateOpen, "WantsValidateOpen should be auto-set")
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// Complete remaining startup stages (2-5)
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:configure", configInput))

	req, err = engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:share-registry", registryInput))

	req, err = engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// Send validate-open request
	voInput := rpc.ValidateOpenInput{
		Peer: "10.0.0.1",
		Local: rpc.ValidateOpenMessage{
			ASN:      65000,
			RouterID: "1.2.3.4",
			HoldTime: 180,
			Capabilities: []rpc.ValidateOpenCapability{
				{Code: 9, Hex: "03"},
			},
		},
		Remote: rpc.ValidateOpenMessage{
			ASN:      65001,
			RouterID: "5.6.7.8",
			HoldTime: 90,
			Capabilities: []rpc.ValidateOpenCapability{
				{Code: 9, Hex: "00"},
			},
		},
	}
	raw, err := engine.client.CallRPC(ctx, "ze-plugin-callback:validate-open", voInput)
	require.NoError(t, err)

	// Parse response
	result, err := rpc.ParseResponse(raw)
	require.NoError(t, err)

	var output rpc.ValidateOpenOutput
	require.NoError(t, json.Unmarshal(result, &output))
	assert.True(t, output.Accept)

	// Verify callback was invoked
	select {
	case got := <-received:
		assert.Equal(t, "10.0.0.1", got.input.Peer)
		assert.Equal(t, uint32(65000), got.input.Local.ASN)
		assert.Equal(t, uint8(9), got.input.Local.Capabilities[0].Code)
	case <-time.After(time.Second):
		t.Fatal("validate-open callback not called")
	}

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKValidateOpenReject verifies validate-open reject response.
//
// VALIDATES: SDK returns reject with NOTIFICATION codes when callback rejects.
// PREVENTS: Rejection codes not propagating through SDK layer.
func TestSDKValidateOpenReject(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	p.OnValidateOpen(func(input *rpc.ValidateOpenInput) *rpc.ValidateOpenOutput {
		return &rpc.ValidateOpenOutput{
			Accept:        false,
			NotifyCode:    2,
			NotifySubcode: 11,
			Reason:        "role mismatch",
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send validate-open request
	voInput := rpc.ValidateOpenInput{
		Peer: "10.0.0.2",
		Local: rpc.ValidateOpenMessage{
			ASN: 65000, RouterID: "1.2.3.4", HoldTime: 180,
			Capabilities: []rpc.ValidateOpenCapability{{Code: 9, Hex: "03"}},
		},
		Remote: rpc.ValidateOpenMessage{
			ASN: 65002, RouterID: "9.8.7.6", HoldTime: 90,
			Capabilities: []rpc.ValidateOpenCapability{{Code: 9, Hex: "03"}},
		},
	}
	raw, err := engine.client.CallRPC(ctx, "ze-plugin-callback:validate-open", voInput)
	require.NoError(t, err)

	result, err := rpc.ParseResponse(raw)
	require.NoError(t, err)

	var output rpc.ValidateOpenOutput
	require.NoError(t, json.Unmarshal(result, &output))
	assert.False(t, output.Accept)
	assert.Equal(t, uint8(2), output.NotifyCode)
	assert.Equal(t, uint8(11), output.NotifySubcode)
	assert.Equal(t, "role mismatch", output.Reason)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKDecodeNLRIEngineCall verifies plugin→engine decode-nlri RPC.
//
// VALIDATES: Plugin calls DecodeNLRI() and receives JSON result from engine.
// PREVENTS: Plugin unable to request NLRI decoding from the engine.
func TestSDKDecodeNLRIEngineCall(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Synchronize: send a no-op event to confirm the event loop is running.
	syncEvent := struct {
		Event string `json:"event"`
	}{Event: "{}"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:deliver-event", syncEvent))

	// Plugin calls DecodeNLRI in background
	decodeDone := make(chan struct {
		json string
		err  error
	}, 1)
	go func() {
		j, err := p.DecodeNLRI(ctx, "ipv4/flow", "0701180A0000")
		decodeDone <- struct {
			json string
			err  error
		}{j, err}
	}()

	// Engine reads decode-nlri request on Socket A
	req, err := engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:decode-nlri", req.Method)

	// Verify params
	var input rpc.DecodeNLRIInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, "ipv4/flow", input.Family)
	assert.Equal(t, "0701180A0000", input.Hex)

	// Respond with JSON result
	decodeResult := struct {
		JSON string `json:"json"`
	}{JSON: `[{"source":"10.0.0.0/24"}]`}
	require.NoError(t, engine.server.SendResult(ctx, req.ID, decodeResult))

	r := <-decodeDone
	require.NoError(t, r.err)
	assert.Equal(t, `[{"source":"10.0.0.0/24"}]`, r.json)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKEncodeNLRIEngineCall verifies plugin→engine encode-nlri RPC.
//
// VALIDATES: Plugin calls EncodeNLRI() and receives hex result from engine.
// PREVENTS: Plugin unable to request NLRI encoding from the engine.
func TestSDKEncodeNLRIEngineCall(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Synchronize: send a no-op event to confirm the event loop is running.
	syncEvent := struct {
		Event string `json:"event"`
	}{Event: "{}"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:deliver-event", syncEvent))

	// Plugin calls EncodeNLRI in background
	encodeDone := make(chan struct {
		hex string
		err error
	}, 1)
	go func() {
		h, err := p.EncodeNLRI(ctx, "ipv4/flow", []string{"match", "source", "10.0.0.0/24"})
		encodeDone <- struct {
			hex string
			err error
		}{h, err}
	}()

	// Engine reads encode-nlri request on Socket A
	req, err := engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:encode-nlri", req.Method)

	// Verify params
	var input rpc.EncodeNLRIInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, "ipv4/flow", input.Family)
	assert.Equal(t, []string{"match", "source", "10.0.0.0/24"}, input.Args)

	// Respond with hex result
	encodeResult := struct {
		Hex string `json:"hex"`
	}{Hex: "0701180A0000"}
	require.NoError(t, engine.server.SendResult(ctx, req.ID, encodeResult))

	r := <-encodeDone
	require.NoError(t, r.err)
	assert.Equal(t, "0701180A0000", r.hex)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKDecodeMPReachEngineCall verifies plugin→engine decode-mp-reach RPC.
//
// VALIDATES: Plugin calls DecodeMPReach() and receives structured result from engine.
// PREVENTS: Plugin unable to decode MP_REACH_NLRI via engine.
func TestSDKDecodeMPReachEngineCall(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Synchronize: send a no-op event to confirm the event loop is running.
	syncEvent := struct {
		Event string `json:"event"`
	}{Event: "{}"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:deliver-event", syncEvent))

	// Plugin calls DecodeMPReach in background
	type mpReachResult struct {
		output *rpc.DecodeMPReachOutput
		err    error
	}
	decodeDone := make(chan mpReachResult, 1)
	go func() {
		out, err := p.DecodeMPReach(ctx, "00010104C0A8010100180A0000", false)
		decodeDone <- mpReachResult{out, err}
	}()

	// Engine reads decode-mp-reach request on Socket A
	req, err := engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:decode-mp-reach", req.Method)

	// Verify params
	var input rpc.DecodeMPReachInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, "00010104C0A8010100180A0000", input.Hex)
	assert.False(t, input.AddPath)

	// Respond with structured result
	decodeResult := rpc.DecodeMPReachOutput{
		Family:  "ipv4/unicast",
		NextHop: "192.168.1.1",
		NLRI:    json.RawMessage(`["10.0.0.0/24"]`),
	}
	require.NoError(t, engine.server.SendResult(ctx, req.ID, decodeResult))

	mpR := <-decodeDone
	require.NoError(t, mpR.err)
	assert.Equal(t, "ipv4/unicast", mpR.output.Family)
	assert.Equal(t, "192.168.1.1", mpR.output.NextHop)
	assert.JSONEq(t, `["10.0.0.0/24"]`, string(mpR.output.NLRI))

	// Shutdown
	byeInput2 := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput2))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKDecodeMPUnreachEngineCall verifies plugin→engine decode-mp-unreach RPC.
//
// VALIDATES: Plugin calls DecodeMPUnreach() and receives structured result from engine.
// PREVENTS: Plugin unable to decode MP_UNREACH_NLRI via engine.
func TestSDKDecodeMPUnreachEngineCall(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	syncEvent := struct {
		Event string `json:"event"`
	}{Event: "{}"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:deliver-event", syncEvent))

	// Plugin calls DecodeMPUnreach in background
	type mpUnreachResult struct {
		output *rpc.DecodeMPUnreachOutput
		err    error
	}
	decodeDone := make(chan mpUnreachResult, 1)
	go func() {
		out, err := p.DecodeMPUnreach(ctx, "00010118C0A800", false)
		decodeDone <- mpUnreachResult{out, err}
	}()

	// Engine reads decode-mp-unreach request on Socket A
	req, err := engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:decode-mp-unreach", req.Method)

	var input rpc.DecodeMPUnreachInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, "00010118C0A800", input.Hex)
	assert.False(t, input.AddPath)

	decodeResult := rpc.DecodeMPUnreachOutput{
		Family: "ipv4/unicast",
		NLRI:   json.RawMessage(`["192.168.0.0/24"]`),
	}
	require.NoError(t, engine.server.SendResult(ctx, req.ID, decodeResult))

	mpR := <-decodeDone
	require.NoError(t, mpR.err)
	assert.Equal(t, "ipv4/unicast", mpR.output.Family)
	assert.JSONEq(t, `["192.168.0.0/24"]`, string(mpR.output.NLRI))

	// Shutdown
	byeInput2 := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput2))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKDecodeUpdateEngineCall verifies plugin→engine decode-update RPC.
//
// VALIDATES: Plugin calls DecodeUpdate() and receives ze-bgp JSON from engine.
// PREVENTS: Plugin unable to decode full UPDATE message via engine.
func TestSDKDecodeUpdateEngineCall(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	syncEvent := struct {
		Event string `json:"event"`
	}{Event: "{}"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:deliver-event", syncEvent))

	// Plugin calls DecodeUpdate in background
	type updateResult struct {
		json string
		err  error
	}
	decodeDone := make(chan updateResult, 1)
	go func() {
		j, err := p.DecodeUpdate(ctx, "0000000B40010100400304C0A80101180A0000", false)
		decodeDone <- updateResult{j, err}
	}()

	// Engine reads decode-update request on Socket A
	req, err := engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:decode-update", req.Method)

	var input rpc.DecodeUpdateInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, "0000000B40010100400304C0A80101180A0000", input.Hex)
	assert.False(t, input.AddPath)

	// Respond with ze-bgp JSON
	decodeResult := struct {
		JSON string `json:"json"`
	}{JSON: `{"update":{"attr":{"origin":"igp"},"nlri":{"ipv4/unicast":[{"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24"]}]}}}`}
	require.NoError(t, engine.server.SendResult(ctx, req.ID, decodeResult))

	upR := <-decodeDone
	require.NoError(t, upR.err)
	assert.Contains(t, upR.json, "update")
	assert.Contains(t, upR.json, "10.0.0.0/24")

	// Shutdown
	byeInput2 := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput2))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// newBridgedTestPair creates a connected plugin SDK + engine side using net.Pipe,
// with the plugin side wrapped in BridgedConn for direct transport testing.
func newBridgedTestPair(t *testing.T) (*Plugin, *engineSide, *rpc.DirectBridge) {
	t.Helper()

	bridge := rpc.NewDirectBridge()

	// Socket A: Plugin → Engine
	aPlugin, aEngine := net.Pipe()
	// Socket B: Engine → Plugin
	bPlugin, bEngine := net.Pipe()

	t.Cleanup(func() {
		for _, c := range []net.Conn{aPlugin, aEngine, bPlugin, bEngine} {
			if err := c.Close(); err != nil {
				t.Logf("cleanup close: %v", err)
			}
		}
	})

	// Wrap plugin-side connections with bridge
	aPluginBridged := rpc.NewBridgedConn(aPlugin, bridge)
	bPluginBridged := rpc.NewBridgedConn(bPlugin, bridge)

	p := NewWithConn("test-bridged", aPluginBridged, bPluginBridged)

	engine := &engineSide{
		server: rpc.NewConn(aEngine, aEngine),
		client: rpc.NewConn(bEngine, bEngine),
	}

	return p, engine, bridge
}

// TestCallEngineRawDirect verifies callEngineRaw dispatches through bridge.
//
// VALIDATES: AC-2 — Engine dispatcher called directly without JSON marshal or net.Pipe I/O.
// PREVENTS: Plugin→engine RPCs going through socket when bridge is available.
func TestCallEngineRawDirect(t *testing.T) {
	t.Parallel()

	p, engine, bridge := newBridgedTestPair(t)

	// Register engine-side RPC handler on bridge.
	// In production, the engine sets this after startup completes.
	var dispatchCalled bool
	bridge.SetDispatchRPC(func(method string, params json.RawMessage) (json.RawMessage, error) {
		dispatchCalled = true
		assert.Equal(t, "ze-plugin-engine:update-route", method)
		// Return a valid UpdateRouteOutput response wrapped in result envelope
		return json.RawMessage(`{"result":{"peers-affected":2,"routes-sent":3}}`), nil
	})
	// Note: bridge.SetReady() is called by SDK's Run() after startup.
	// DispatchRPC must be registered before that.

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Synchronize: send a no-op event to confirm the event loop is running.
	syncEvent := struct {
		Event string `json:"event"`
	}{Event: "{}"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:deliver-event", syncEvent))

	// Call UpdateRoute — should go through bridge, not socket
	routeDone := make(chan error, 1)
	go func() {
		_, _, err := p.UpdateRoute(ctx, "*", "update text origin set igp")
		routeDone <- err
	}()

	select {
	case err := <-routeDone:
		require.NoError(t, err)
		assert.True(t, dispatchCalled, "bridge.DispatchRPC should have been called")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for UpdateRoute")
	}

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestCallEngineRawDirectError verifies error propagation from bridge dispatch.
//
// VALIDATES: AC-6 — Error propagated to SDK caller correctly.
// PREVENTS: Errors from direct RPC dispatch being lost.
func TestCallEngineRawDirectError(t *testing.T) {
	t.Parallel()

	p, engine, bridge := newBridgedTestPair(t)

	bridge.SetDispatchRPC(func(method string, params json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"error":"dispatch failed"}`), nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Synchronize
	syncEvent := struct {
		Event string `json:"event"`
	}{Event: "{}"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:deliver-event", syncEvent))

	// Call UpdateRoute — bridge returns error response
	routeDone := make(chan error, 1)
	go func() {
		_, _, err := p.UpdateRoute(ctx, "*", "update text origin set igp")
		routeDone <- err
	}()

	select {
	case err := <-routeDone:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dispatch failed")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for UpdateRoute")
	}

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// newExternalTestPair creates a connected plugin SDK + engine side using real Unix sockets.
// Required for tests that need SCM_RIGHTS fd passing (connection handoff).
// Returns the SDK plugin, engine helper, and the raw engine-side Socket B for fd sending.
func newExternalTestPair(t *testing.T) (*Plugin, *engineSide, net.Conn) {
	t.Helper()

	// Socket A: Plugin → Engine
	fdsA, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	aPlugin := fileConn(t, fdsA[0], "socketA-plugin")
	aEngine := fileConn(t, fdsA[1], "socketA-engine")

	// Socket B: Engine → Plugin
	fdsB, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	bPlugin := fileConn(t, fdsB[0], "socketB-plugin")
	bEngine := fileConn(t, fdsB[1], "socketB-engine")

	t.Cleanup(func() {
		for _, c := range []net.Conn{aPlugin, aEngine, bPlugin, bEngine} {
			if closeErr := c.Close(); closeErr != nil {
				t.Logf("cleanup close: %v", closeErr)
			}
		}
	})

	p := NewWithConn("test-plugin", aPlugin, bPlugin)

	engine := &engineSide{
		server: rpc.NewConn(aEngine, aEngine),
		client: rpc.NewConn(bEngine, bEngine),
	}

	return p, engine, bEngine
}

// fileConn converts a raw fd to a net.Conn via os.NewFile + net.FileConn.
func fileConn(t *testing.T, fd int, name string) net.Conn {
	t.Helper()
	f := os.NewFile(uintptr(fd), name)
	require.NotNil(t, f)
	conn, err := net.FileConn(f)
	require.NoError(t, f.Close())
	require.NoError(t, err)
	return conn
}

// TestSDKAutoReceiveListeners verifies that Run() auto-receives listen socket fds
// between Stage 1 and Stage 2 when connection-handlers are declared.
//
// VALIDATES: SDK auto-receives listener fds from engine via SCM_RIGHTS during startup.
// PREVENTS: Plugin using Run() with connection-handlers gets stuck or misses fds.
func TestSDKAutoReceiveListeners(t *testing.T) {
	t.Parallel()

	p, engine, rawEngineB := newExternalTestPair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find a free port.
	var lc net.ListenConfig
	tmpLn, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	tcpAddr, ok := tmpLn.Addr().(*net.TCPAddr)
	require.True(t, ok)
	port := tcpAddr.Port
	require.NoError(t, tmpLn.Close())

	// Registration with connection-handler.
	reg := Registration{
		ConnectionHandlers: []ConnectionHandlerDecl{
			{Type: "listen", Port: port, Address: "127.0.0.1"},
		},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, reg)
	}()

	// === Stage 1: Engine reads declare-registration ===
	req, err := engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// Engine creates listen socket and sends fd to plugin via SCM_RIGHTS.
	ln, err := lc.Listen(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	tcpLn, ok := ln.(*net.TCPListener)
	require.True(t, ok)
	lnFile, err := tcpLn.File()
	require.NoError(t, err)
	ln.Close() //nolint:errcheck,gosec // fd ownership transferred to lnFile

	require.NoError(t, ipc.SendFD(rawEngineB, lnFile))
	lnFile.Close() //nolint:errcheck,gosec // engine closes its copy after send

	// === Stage 2: Engine sends configure ===
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:configure", configInput))

	// === Stage 3: Engine reads declare-capabilities ===
	req, err = engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// === Stage 4: Engine sends share-registry ===
	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:share-registry", registryInput))

	// === Stage 5: Engine reads ready ===
	req, err = engine.server.ReadRequest(ctx)
	require.NoError(t, err)
	require.NoError(t, engine.server.SendOK(ctx, req.ID))

	// Verify plugin received the listener via auto-receive.
	require.Len(t, p.Listeners(), 1)
	receivedLn := p.Listeners()[0]
	defer receivedLn.Close() //nolint:errcheck,gosec // test cleanup

	// Verify the received listener can accept connections.
	acceptDone := make(chan error, 1)
	go func() {
		conn, acceptErr := receivedLn.Accept()
		if acceptErr != nil {
			acceptDone <- acceptErr
			return
		}
		conn.Close() //nolint:errcheck,gosec // test cleanup
		acceptDone <- nil
	}()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	var dialer net.Dialer
	client, dialErr := dialer.DialContext(ctx, "tcp", addr)
	require.NoError(t, dialErr)
	client.Close() //nolint:errcheck,gosec // test cleanup

	require.NoError(t, <-acceptDone, "received listener must accept connections")

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.client, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit after bye")
	}
}
