package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// engineSide wraps a MuxConn for the engine's perspective on a single connection.
// Uses MuxConn for bidirectional RPC: reads plugin requests via Requests(),
// sends engine callbacks via CallRPC.
type engineSide struct {
	mux *rpc.MuxConn
}

// newTestPair creates a connected plugin SDK + engine side using a single net.Pipe.
// Returns the SDK plugin and the engine-side MuxConn helper.
func newTestPair(t *testing.T) (*Plugin, *engineSide) {
	t.Helper()

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() {
		pluginEnd.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
	})

	p := NewWithConn("test-plugin", pluginEnd)

	engineConn := rpc.NewConn(engineEnd, engineEnd)
	engineMux := rpc.NewMuxConn(engineConn)
	t.Cleanup(func() {
		engineMux.Close() //nolint:errcheck // test cleanup
	})

	return p, &engineSide{mux: engineMux}
}

// callAndExpectOK sends an RPC request and expects a successful response.
// CallRPC returns RPC errors as Go errors, so this just forwards the error.
func callAndExpectOK(ctx context.Context, mux *rpc.MuxConn, method string, params any) error {
	_, err := mux.CallRPC(ctx, method, params)
	return err
}

// readEngineRequest reads the next plugin-to-engine request from the MuxConn.
func readEngineRequest(t *testing.T, ctx context.Context, mux *rpc.MuxConn) *rpc.Request {
	t.Helper()
	select {
	case req := <-mux.Requests():
		return req
	case <-ctx.Done():
		t.Fatal("timed out waiting for plugin request")
		return nil
	}
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
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)

	var regInput Registration
	require.NoError(t, json.Unmarshal(req.Params, &regInput))
	assert.Equal(t, 1, len(regInput.Families))
	assert.Equal(t, "ipv4/unicast", regInput.Families[0].Name)
	assert.Equal(t, 1, len(regInput.Commands))
	assert.Equal(t, []string{"bgp"}, regInput.WantsConfig)

	// Respond
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// === Stage 2: Engine sends configure ===
	sections := []ConfigSection{
		{Root: "bgp", Data: `{"router-id":"1.2.3.4"}`},
	}
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{Sections: sections}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:configure", configInput))

	// Verify callback was called
	select {
	case got := <-configReceived:
		assert.Equal(t, 1, len(got))
		assert.Equal(t, "bgp", got[0].Root)
	case <-time.After(time.Second):
		t.Fatal("configure callback not called")
	}

	// === Stage 3: Engine reads declare-capabilities ===
	req = readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// === Stage 4: Engine sends share-registry ===
	commands := []RegistryCommand{
		{Name: "show-routes", Plugin: "test-plugin", Encoding: "text"},
	}
	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{Commands: commands}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:share-registry", registryInput))

	select {
	case got := <-registryReceived:
		assert.Equal(t, 1, len(got))
		assert.Equal(t, "show-routes", got[0].Name)
	case <-time.After(time.Second):
		t.Fatal("share-registry callback not called")
	}

	// === Stage 5: Engine reads ready ===
	req = readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// === Shutdown: Engine sends bye ===
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "test-complete"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:deliver-event", eventInput))

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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Stage 2: send configure
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:configure", configInput))

	// Stage 3: read and respond to declare-capabilities
	req = readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Stage 4: send share-registry
	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:share-registry", registryInput))

	// Stage 5: read and respond to ready
	req = readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))
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

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:encode-nlri", encInput)
	require.NoError(t, err)

	// CallRPC returns the result payload directly
	var result struct {
		Hex string `json:"hex"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, "180a0000", result.Hex)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:decode-nlri", decInput)
	require.NoError(t, err)

	// CallRPC returns the result payload directly
	var result struct {
		JSON string `json:"json"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, `["10.0.0.0/24"]`, result.JSON)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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
		{Code: 64, Encoding: CapEncodingHex, Payload: "0001"},
	}
	p.SetCapabilities(caps)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, reg)
	}()

	// Stage 1: read declare-registration
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Stage 2: send configure
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:configure", configInput))

	// Stage 3: read declare-capabilities — should contain our caps
	req = readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)

	var capsInput DeclareCapabilitiesInput
	require.NoError(t, json.Unmarshal(req.Params, &capsInput))
	require.Len(t, capsInput.Capabilities, 1)
	assert.Equal(t, uint8(64), capsInput.Capabilities[0].Code)
	assert.Equal(t, CapEncodingHex, capsInput.Capabilities[0].Encoding)
	assert.Equal(t, "0001", capsInput.Capabilities[0].Payload)

	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Stage 4: send share-registry
	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:share-registry", registryInput))

	// Stage 5: read ready
	req = readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:decode-capability", decCapInput)
	require.NoError(t, err)

	// CallRPC returns the result payload directly
	var result struct {
		JSON string `json:"json"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, `{"hostname":"router1"}`, result.JSON)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:execute-command", execInput)
	require.NoError(t, err)

	// CallRPC returns the result payload directly
	var result struct {
		Status string `json:"status"`
		Data   string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, "done", result.Status)
	assert.Equal(t, `{"routes":[]}`, result.Data)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:config-verify", verifyInput)
	require.NoError(t, err)

	// CallRPC returns the result payload directly
	var result struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:config-apply", applyInput)
	require.NoError(t, err)

	// CallRPC returns the result payload directly
	var result struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:config-verify", verifyInput)
	require.NoError(t, err, "rejection should be in result status, not RPC error")

	// CallRPC returns the result payload directly -- rejection is status "error" in the payload
	var result struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, "error", result.Status)
	assert.Equal(t, "invalid config: missing router-id", result.Error)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:config-apply", applyInput)
	require.NoError(t, err, "rejection should be in result status, not RPC error")

	// CallRPC returns the result payload directly -- rejection is status "error" in the payload
	var result struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, "error", result.Status)
	assert.Equal(t, "cannot apply: peer limit exceeded", result.Error)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:config-verify", verifyInput)
	require.NoError(t, err, "no-handler config-verify should succeed, not error")

	// CallRPC returns the result payload directly
	var result struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, "ok", result.Status)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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

	raw, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:config-apply", applyInput)
	require.NoError(t, err, "no-handler config-apply should succeed, not error")

	// CallRPC returns the result payload directly
	var result struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, "ok", result.Status)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:deliver-event", syncEvent))

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

	// Engine reads update-route request via MuxConn
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:update-route", req.Method)

	// Respond with result
	routeResult := struct {
		PeersAffected uint32 `json:"peers-affected"`
		RoutesSent    uint32 `json:"routes-sent"`
	}{PeersAffected: 3, RoutesSent: 1}
	require.NoError(t, engine.mux.SendResult(ctx, req.ID, routeResult))

	require.NoError(t, <-routeDone)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:deliver-event", syncEvent))

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

	// Engine reads dispatch-command request via MuxConn
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:dispatch-command", req.Method)

	// Respond with result
	dispatchResult := rpc.DispatchCommandOutput{
		Status: rpc.StatusDone,
		Data:   `{"last-index":42}`,
	}
	require.NoError(t, engine.mux.SendResult(ctx, req.ID, dispatchResult))

	require.NoError(t, <-dispatchDone)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
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

	// Start a goroutine that blocks on readCallback (no data will ever arrive).
	readDone := make(chan error, 1)
	go func() {
		_, err := p.readCallback(ctx)
		readDone <- err
	}()

	// Verify the goroutine is blocked (not completing before Close).
	require.Never(t, func() bool {
		select {
		case <-readDone:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, 10*time.Millisecond, "readCallback should be blocked")

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

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() {
		pluginEnd.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
	})

	p := NewWithConn("test-plugin", pluginEnd)
	engine := &engineSide{mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))}
	t.Cleanup(func() { engine.mux.Close() }) //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Simulate engine shutdown: close the connection.
	require.NoError(t, engine.mux.Close())

	select {
	case err := <-errCh:
		// Must be nil -- connection close is a clean shutdown, not an error.
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit after connection close")
	}
}

// TestSDKStage5ConnectionCloseCleanShutdown verifies that closing the connection
// during stage 5 (after reading the ready request but before responding) causes
// Run() to return nil (clean shutdown), not a "stage 5 (ready)" error.
//
// VALIDATES: Connection close at any startup stage is treated as clean shutdown.
// PREVENTS: "stage 5 (ready)" error logged when engine shuts down mid-handshake.
func TestSDKStage5ConnectionCloseCleanShutdown(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() {
		pluginEnd.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
	})

	p := NewWithConn("test-plugin", pluginEnd)
	engine := &engineSide{mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))}
	t.Cleanup(func() { engine.mux.Close() }) //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	// Stage 1: read and respond to declare-registration
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Stage 2: send configure
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:configure", configInput))

	// Stage 3: read and respond to declare-capabilities
	req = readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Stage 4: send share-registry
	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:share-registry", registryInput))

	// Stage 5: read the ready request but close BEFORE responding.
	req = readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:ready", req.Method)

	// Close the engine connection instead of sending OK.
	require.NoError(t, engine.mux.Close())

	select {
	case err := <-errCh:
		// Must be nil -- connection close during stage 5 is a clean shutdown, not an error.
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit after connection close during stage 5")
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
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)

	var regInput rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(req.Params, &regInput))
	assert.True(t, regInput.WantsValidateOpen, "WantsValidateOpen should be auto-set")
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Complete remaining startup stages (2-5)
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:configure", configInput))

	req = readEngineRequest(t, ctx, engine.mux)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:share-registry", registryInput))

	req = readEngineRequest(t, ctx, engine.mux)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

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
	result, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:validate-open", voInput)
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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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
	result, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:validate-open", voInput)
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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:deliver-event", syncEvent))

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

	// Engine reads decode-nlri request via MuxConn
	req := readEngineRequest(t, ctx, engine.mux)
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
	require.NoError(t, engine.mux.SendResult(ctx, req.ID, decodeResult))

	r := <-decodeDone
	require.NoError(t, r.err)
	assert.Equal(t, `[{"source":"10.0.0.0/24"}]`, r.json)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:deliver-event", syncEvent))

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

	// Engine reads encode-nlri request via MuxConn
	req := readEngineRequest(t, ctx, engine.mux)
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
	require.NoError(t, engine.mux.SendResult(ctx, req.ID, encodeResult))

	r := <-encodeDone
	require.NoError(t, r.err)
	assert.Equal(t, "0701180A0000", r.hex)

	// Shutdown
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:deliver-event", syncEvent))

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

	// Engine reads decode-mp-reach request via MuxConn
	req := readEngineRequest(t, ctx, engine.mux)
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
	require.NoError(t, engine.mux.SendResult(ctx, req.ID, decodeResult))

	mpR := <-decodeDone
	require.NoError(t, mpR.err)
	assert.Equal(t, "ipv4/unicast", mpR.output.Family)
	assert.Equal(t, "192.168.1.1", mpR.output.NextHop)
	assert.JSONEq(t, `["10.0.0.0/24"]`, string(mpR.output.NLRI))

	// Shutdown
	byeInput2 := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput2))

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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:deliver-event", syncEvent))

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

	// Engine reads decode-mp-unreach request via MuxConn
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:decode-mp-unreach", req.Method)

	var input rpc.DecodeMPUnreachInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, "00010118C0A800", input.Hex)
	assert.False(t, input.AddPath)

	decodeResult := rpc.DecodeMPUnreachOutput{
		Family: "ipv4/unicast",
		NLRI:   json.RawMessage(`["192.168.0.0/24"]`),
	}
	require.NoError(t, engine.mux.SendResult(ctx, req.ID, decodeResult))

	mpR := <-decodeDone
	require.NoError(t, mpR.err)
	assert.Equal(t, "ipv4/unicast", mpR.output.Family)
	assert.JSONEq(t, `["192.168.0.0/24"]`, string(mpR.output.NLRI))

	// Shutdown
	byeInput2 := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput2))

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
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:deliver-event", syncEvent))

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

	// Engine reads decode-update request via MuxConn
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:decode-update", req.Method)

	var input rpc.DecodeUpdateInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, "0000000B40010100400304C0A80101180A0000", input.Hex)
	assert.False(t, input.AddPath)

	// Respond with ze-bgp JSON
	decodeResult := struct {
		JSON string `json:"json"`
	}{JSON: `{"update":{"attr":{"origin":"igp"},"nlri":{"ipv4/unicast":[{"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24"]}]}}}`}
	require.NoError(t, engine.mux.SendResult(ctx, req.ID, decodeResult))

	upR := <-decodeDone
	require.NoError(t, upR.err)
	assert.Contains(t, upR.json, "update")
	assert.Contains(t, upR.json, "10.0.0.0/24")

	// Shutdown
	byeInput2 := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput2))

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

	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() {
		pluginEnd.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
	})

	// Wrap plugin-side connection with bridge reference.
	bridgedConn := rpc.NewBridgedConn(pluginEnd, bridge)
	p := NewWithConn("test-bridged", bridgedConn)

	engineConn := rpc.NewConn(engineEnd, engineEnd)
	engineMux := rpc.NewMuxConn(engineConn)
	t.Cleanup(func() { engineMux.Close() }) //nolint:errcheck // test cleanup

	return p, &engineSide{mux: engineMux}, bridge
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

	// Wait for bridge event loop to start. With bridge transport, the pipe is
	// closed after startup so we sync via bridge readiness instead.
	require.Eventually(t, func() bool { return bridge.Ready() }, 2*time.Second, 10*time.Millisecond,
		"bridge should be ready after startup")

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

	// Shutdown via bridge (pipe is closed).
	byeParams, _ := json.Marshal(struct {
		Reason string `json:"reason"`
	}{Reason: "done"})
	_, byeErr := bridge.SendCallback(ctx, "ze-plugin-callback:bye", byeParams)
	require.NoError(t, byeErr)

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
		return nil, fmt.Errorf("dispatch failed")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Wait for bridge event loop to start.
	require.Eventually(t, func() bool { return bridge.Ready() }, 2*time.Second, 10*time.Millisecond,
		"bridge should be ready after startup")

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

	// Shutdown via bridge (pipe is closed).
	byeParams, _ := json.Marshal(struct {
		Reason string `json:"reason"`
	}{Reason: "done"})
	_, byeErr := bridge.SendCallback(ctx, "ze-plugin-callback:bye", byeParams)
	require.NoError(t, byeErr)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKSingleConnStartup verifies that NewWithConn creates a plugin
// that completes the full 5-stage startup over a single bidirectional connection.
//
// VALIDATES: Single-conn mode works end-to-end with MuxConn.
// PREVENTS: Single-conn plugin failing during startup or event loop.
func TestSDKSingleConnStartup(t *testing.T) {
	t.Parallel()

	pluginEnd, engineEnd := net.Pipe()
	defer pluginEnd.Close() //nolint:errcheck // test cleanup
	defer engineEnd.Close() //nolint:errcheck // test cleanup

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p := NewWithConn("test-single-conn", pluginEnd)

	var receivedEvent string
	p.OnEvent(func(event string) error {
		receivedEvent = event
		return nil
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	// Engine side: single connection, use MuxConn for bidirectional.
	engineConn := rpc.NewConn(engineEnd, engineEnd)
	engineMux := rpc.NewMuxConn(engineConn)
	defer engineMux.Close() //nolint:errcheck // test cleanup

	// Stage 1: Read declare-registration from plugin.
	req := readMuxRequest(t, ctx, engineMux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engineMux.SendOK(ctx, req.ID))

	// Stage 2: Send configure to plugin.
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{}
	_, configErr := engineMux.CallRPC(ctx, "ze-plugin-callback:configure", configInput)
	require.NoError(t, configErr)

	// Stage 3: Read declare-capabilities from plugin.
	req = readMuxRequest(t, ctx, engineMux)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
	require.NoError(t, engineMux.SendOK(ctx, req.ID))

	// Stage 4: Send share-registry to plugin.
	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{}
	_, regErr := engineMux.CallRPC(ctx, "ze-plugin-callback:share-registry", registryInput)
	require.NoError(t, regErr)

	// Stage 5: Read ready from plugin.
	req = readMuxRequest(t, ctx, engineMux)
	assert.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engineMux.SendOK(ctx, req.ID))

	// Post-startup: send an event to the plugin.
	eventInput := struct {
		Event string `json:"event"`
	}{Event: `{"type":"bgp","peer":"1.2.3.4"}`}
	_, eventErr := engineMux.CallRPC(ctx, "ze-plugin-callback:deliver-event", eventInput)
	require.NoError(t, eventErr)
	assert.Equal(t, `{"type":"bgp","peer":"1.2.3.4"}`, receivedEvent)

	// Shutdown.
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "test-done"}
	_, byeErr := engineMux.CallRPC(ctx, "ze-plugin-callback:bye", byeInput)
	require.NoError(t, byeErr)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit after bye")
	}
}

// readMuxRequest reads the next inbound request from MuxConn.Requests().
func readMuxRequest(t *testing.T, ctx context.Context, mux *rpc.MuxConn) *rpc.Request {
	t.Helper()
	select {
	case req := <-mux.Requests():
		return req
	case <-ctx.Done():
		t.Fatal("timed out waiting for request from plugin")
		return nil
	}
}

// TestRegistrationWantsConfig verifies WantsConfig is included in Stage 1 registration.
//
// VALIDATES: AC-9 - WantsConfig declared in Stage 1.
// PREVENTS: Read-only config interest lost during registration.
func TestRegistrationWantsConfig(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	reg := Registration{
		WantsConfig: []string{"bgp", "interface"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, reg)
	}()

	// Stage 1: read declare-registration and verify WantsConfig
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)

	var regInput rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(req.Params, &regInput))
	assert.Equal(t, []string{"bgp", "interface"}, regInput.WantsConfig)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Complete remaining stages and shutdown
	completeStartupFromStage2(t, ctx, engine)
	shutdownPlugin(t, ctx, engine, errCh)
}

// TestRegistrationBudgets verifies VerifyBudget and ApplyBudget in Stage 1 registration.
//
// VALIDATES: AC-10 - Plugin estimates apply budget at registration.
// PREVENTS: Budget fields missing from wire protocol.
func TestRegistrationBudgets(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	reg := Registration{
		VerifyBudget: 5,
		ApplyBudget:  30,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, reg)
	}()

	// Stage 1: read declare-registration and verify budgets
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)

	var regInput rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(req.Params, &regInput))
	assert.Equal(t, 5, regInput.VerifyBudget)
	assert.Equal(t, 30, regInput.ApplyBudget)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Complete remaining stages and shutdown
	completeStartupFromStage2(t, ctx, engine)
	shutdownPlugin(t, ctx, engine, errCh)
}

// TestOnConfigRollbackCallback verifies SDK dispatches rollback to registered handler.
//
// VALIDATES: AC-5 - OnConfigRollback callback invoked during rollback.
// PREVENTS: Rollback events ignored by plugins that registered a handler.
func TestOnConfigRollbackCallback(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	rollbackCalled := make(chan string, 1)
	p.OnConfigRollback(func(txID string) error {
		rollbackCalled <- txID
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	completeStartup(t, ctx, engine)

	// Send config-rollback callback
	rollbackInput := struct {
		TransactionID string `json:"transaction-id"`
	}{TransactionID: "tx-001"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:config-rollback", rollbackInput))

	select {
	case txID := <-rollbackCalled:
		assert.Equal(t, "tx-001", txID)
	case <-time.After(time.Second):
		t.Fatal("rollback callback not called")
	}

	shutdownPlugin(t, ctx, engine, errCh)
}

// completeStartupFromStage2 completes stages 2-5 from the engine side.
func completeStartupFromStage2(t *testing.T, ctx context.Context, engine *engineSide) {
	t.Helper()

	// Stage 2: send configure
	configInput := struct {
		Sections []ConfigSection `json:"sections"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:configure", configInput))

	// Stage 3: read and respond to declare-capabilities
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Stage 4: send share-registry
	registryInput := struct {
		Commands []RegistryCommand `json:"commands"`
	}{}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:share-registry", registryInput))

	// Stage 5: read and respond to ready
	req = readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))
}

// shutdownPlugin sends bye and waits for clean exit.
func shutdownPlugin(t *testing.T, ctx context.Context, engine *engineSide, errCh <-chan error) {
	t.Helper()
	byeInput := struct {
		Reason string `json:"reason"`
	}{Reason: "done"}
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", byeInput))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit")
	}
}

// TestSDKOnAllPluginsReadyFires verifies the OnAllPluginsReady handler runs
// when the engine sends the post-startup callback via the event loop.
//
// VALIDATES: AC-1 -- OnAllPluginsReady handler fires after post-startup
//
//	AC-6 -- external pipe plugins dispatch the callback via the
//	normal event loop, so the handler runs AFTER OnStarted returns.
//
// PREVENTS: bgp-rpki-style races where an OnStarted dispatch hits a
//
//	dispatcher registry that does not yet contain cross-phase
//	plugin commands.
func TestSDKOnAllPluginsReadyFires(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	onStartedFired := make(chan struct{}, 1)
	onAllReadyFired := make(chan struct{}, 1)

	p.OnStarted(func(context.Context) error {
		onStartedFired <- struct{}{}
		return nil
	})
	p.OnAllPluginsReady(func() error {
		onAllReadyFired <- struct{}{}
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	// Stage 1: declare-registration
	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Complete stages 2-5.
	completeStartupFromStage2(t, ctx, engine)

	// OnStarted must fire synchronously after stage 5 and BEFORE the event
	// loop starts processing callbacks. Drain it first.
	select {
	case <-onStartedFired:
	case <-time.After(2 * time.Second):
		t.Fatal("OnStarted did not fire after stage 5")
	}

	// OnAllPluginsReady must NOT have fired yet -- post-startup hasn't been
	// sent by the engine. Sanity check with a short poll.
	select {
	case <-onAllReadyFired:
		t.Fatal("OnAllPluginsReady fired before engine sent post-startup")
	case <-time.After(50 * time.Millisecond):
	}

	// Engine sends post-startup callback via the event loop path.
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:post-startup", nil))

	// Handler should fire now.
	select {
	case <-onAllReadyFired:
	case <-time.After(2 * time.Second):
		t.Fatal("OnAllPluginsReady did not fire after post-startup callback")
	}

	shutdownPlugin(t, ctx, engine, errCh)
}

// TestSDKOnAllPluginsReadyPropagatesError verifies that an error returned from
// the OnAllPluginsReady handler surfaces as an RPC error response to the
// engine, without crashing the plugin. The plugin continues running and
// receives subsequent callbacks (like bye) normally.
//
// VALIDATES: AC-3 -- errors from the handler do not kill the plugin.
// PREVENTS: regression where a DispatchCommand failure tears down bgp-rpki.
func TestSDKOnAllPluginsReadyPropagatesError(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	p.OnAllPluginsReady(func() error {
		return fmt.Errorf("synthetic handler failure")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	completeStartupFromStage2(t, ctx, engine)

	// Post-startup arrives; the handler returns an error; the engine sees
	// an RPC error response, not a transport failure.
	_, err := engine.mux.CallRPC(ctx, "ze-plugin-callback:post-startup", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "synthetic handler failure")

	// Plugin is still alive: bye works.
	shutdownPlugin(t, ctx, engine, errCh)
}

// TestSDKOnAllPluginsReadyNoHandlerIsNoop verifies that plugins that never
// register a handler accept the post-startup callback silently. This lets the
// engine fan-out the callback to every plugin without first asking which ones
// care.
//
// VALIDATES: default initCallbackDefaults entry for callbackPostStartup.
// PREVENTS: engine SendPostStartup returning "unknown method" for plugins
//
//	that do not need cross-plugin coordination.
func TestSDKOnAllPluginsReadyNoHandlerIsNoop(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)
	// Intentionally do NOT register OnAllPluginsReady.

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	req := readEngineRequest(t, ctx, engine.mux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	completeStartupFromStage2(t, ctx, engine)

	// Post-startup MUST succeed against a plugin with no registered handler.
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:post-startup", nil))

	shutdownPlugin(t, ctx, engine, errCh)
}
