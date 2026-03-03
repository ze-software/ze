package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// helper: create a connected PluginConn pair (engine side + plugin side)
// using internal socket pairs (net.Pipe).
func newTestPluginConn(t *testing.T) (engineConn, pluginConn *PluginConn) {
	t.Helper()

	pairs, err := NewInternalSocketPairs()
	require.NoError(t, err)

	t.Cleanup(func() { pairs.Close() })

	// Engine side: reads from Engine.EngineSide (Socket A server),
	// writes to Callback.EngineSide (Socket B client)
	engineConn = NewPluginConn(pairs.Engine.EngineSide, pairs.Callback.EngineSide)

	// Plugin side: reads from Callback.PluginSide (Socket B server),
	// writes to Engine.PluginSide (Socket A client)
	pluginConn = NewPluginConn(pairs.Callback.PluginSide, pairs.Engine.PluginSide)

	return engineConn, pluginConn
}

// TestRPCDeclareRegistration verifies Stage 1: plugin declares registration via RPC.
//
// VALIDATES: Plugin sends declare-registration RPC, engine receives and parses it.
// PREVENTS: Registration data lost in transit or mis-parsed.
func TestRPCDeclareRegistration(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	regInput := &rpc.DeclareRegistrationInput{
		Families: []rpc.FamilyDecl{
			{Name: "ipv4/unicast", Mode: "both"},
			{Name: "ipv4/flow", Mode: "decode"},
		},
		Commands: []rpc.CommandDecl{
			{Name: "show-routes", Description: "Show routes"},
		},
		WantsConfig: []string{"bgp"},
		Schema: &rpc.SchemaDecl{
			Module:    "ze-test",
			Namespace: "urn:ze:test",
			Handlers:  []string{"test"},
		},
	}

	// Plugin sends declare-registration on Socket A (write side)
	done := make(chan error, 1)
	go func() {
		done <- pluginConn.SendDeclareRegistration(context.Background(), regInput)
	}()

	// Engine receives on Socket A (read side)
	req, err := engineConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)

	// Parse the params
	var received rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(req.Params, &received))
	assert.Equal(t, 2, len(received.Families))
	assert.Equal(t, "ipv4/unicast", received.Families[0].Name)
	assert.Equal(t, "both", received.Families[0].Mode)
	assert.Equal(t, 1, len(received.Commands))
	assert.Equal(t, "show-routes", received.Commands[0].Name)
	assert.Equal(t, []string{"bgp"}, received.WantsConfig)
	assert.Equal(t, "ze-test", received.Schema.Module)

	// Engine sends response
	require.NoError(t, engineConn.SendResult(context.Background(), req.ID, nil))

	// Plugin should complete without error
	require.NoError(t, <-done)
}

// TestRPCConfigure verifies Stage 2: engine delivers config to plugin via RPC.
//
// VALIDATES: Engine sends configure RPC on callback socket, plugin receives config sections.
// PREVENTS: Config delivery failing or sections being lost.
func TestRPCConfigure(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	sections := []rpc.ConfigSection{
		{Root: "bgp", Data: `{"router-id":"1.2.3.4"}`},
	}

	// Engine sends configure on Socket B (callback write side)
	done := make(chan error, 1)
	go func() {
		done <- engineConn.SendConfigure(context.Background(), sections)
	}()

	// Plugin receives on Socket B (callback read side)
	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:configure", req.Method)

	var input rpc.ConfigureInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, 1, len(input.Sections))
	assert.Equal(t, "bgp", input.Sections[0].Root)
	assert.Equal(t, `{"router-id":"1.2.3.4"}`, input.Sections[0].Data)

	// Plugin sends response
	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, nil))

	require.NoError(t, <-done)
}

// TestRPCDeclareCapabilities verifies Stage 3: plugin declares capabilities via RPC.
//
// VALIDATES: Plugin sends declare-capabilities RPC with capability list.
// PREVENTS: Capability codes or payloads being corrupted.
func TestRPCDeclareCapabilities(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	caps := &rpc.DeclareCapabilitiesInput{
		Capabilities: []rpc.CapabilityDecl{
			{Code: 64, Encoding: "hex", Payload: "40780078"},
			{Code: 2}, // route-refresh, no payload
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- pluginConn.SendDeclareCapabilities(context.Background(), caps)
	}()

	req, err := engineConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)

	var received rpc.DeclareCapabilitiesInput
	require.NoError(t, json.Unmarshal(req.Params, &received))
	assert.Equal(t, 2, len(received.Capabilities))
	assert.Equal(t, uint8(64), received.Capabilities[0].Code)
	assert.Equal(t, "40780078", received.Capabilities[0].Payload)
	assert.Equal(t, uint8(2), received.Capabilities[1].Code)

	require.NoError(t, engineConn.SendResult(context.Background(), req.ID, nil))
	require.NoError(t, <-done)
}

// TestRPCShareRegistry verifies Stage 4: engine shares command registry via RPC.
//
// VALIDATES: Engine sends share-registry RPC with command list.
// PREVENTS: Registry data being lost during Stage 4.
func TestRPCShareRegistry(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	commands := []rpc.RegistryCommand{
		{Name: "show-routes", Plugin: "flowspec", Encoding: "text"},
		{Name: "clear-rib", Plugin: "rib", Encoding: "text"},
	}

	done := make(chan error, 1)
	go func() {
		done <- engineConn.SendShareRegistry(context.Background(), commands)
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:share-registry", req.Method)

	var input rpc.ShareRegistryInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, 2, len(input.Commands))
	assert.Equal(t, "show-routes", input.Commands[0].Name)
	assert.Equal(t, "flowspec", input.Commands[0].Plugin)

	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, nil))
	require.NoError(t, <-done)
}

// TestRPCReady verifies Stage 5: plugin signals readiness via RPC.
//
// VALIDATES: Plugin sends ready RPC, engine acknowledges.
// PREVENTS: Ready signal not reaching engine.
func TestRPCReady(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan error, 1)
	go func() {
		done <- pluginConn.SendReady(context.Background())
	}()

	req, err := engineConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:ready", req.Method)

	require.NoError(t, engineConn.SendResult(context.Background(), req.ID, nil))
	require.NoError(t, <-done)
}

// TestRPCDeliverEvent verifies runtime event delivery via callback RPC.
//
// VALIDATES: Engine delivers BGP event to plugin via deliver-event RPC.
// PREVENTS: Events being lost or malformed during delivery.
func TestRPCDeliverEvent(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	eventJSON := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`

	done := make(chan error, 1)
	go func() {
		done <- engineConn.SendDeliverEvent(context.Background(), eventJSON)
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:deliver-event", req.Method)

	var input rpc.DeliverEventInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, eventJSON, input.Event)

	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, nil))
	require.NoError(t, <-done)
}

// TestRPCEncodeNLRI verifies encode-nlri callback RPC.
//
// VALIDATES: Engine sends encode request, plugin returns hex result.
// PREVENTS: Encode request/response corruption.
func TestRPCEncodeNLRI(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan struct {
		hex string
		err error
	}, 1)
	go func() {
		hex, err := engineConn.SendEncodeNLRI(context.Background(), "ipv4/flow", []string{"match", "source", "10.0.0.0/24"})
		done <- struct {
			hex string
			err error
		}{hex, err}
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:encode-nlri", req.Method)

	// Plugin responds with hex
	result := map[string]string{"hex": "180a0000"}
	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, result))

	r := <-done
	require.NoError(t, r.err)
	assert.Equal(t, "180a0000", r.hex)
}

// TestRPCDecodeNLRI verifies decode-nlri callback RPC.
//
// VALIDATES: Engine sends decode request, plugin returns JSON result.
// PREVENTS: Decode request/response corruption.
func TestRPCDecodeNLRI(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan struct {
		json string
		err  error
	}, 1)
	go func() {
		j, err := engineConn.SendDecodeNLRI(context.Background(), "ipv4/flow", "180a0000")
		done <- struct {
			json string
			err  error
		}{j, err}
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:decode-nlri", req.Method)

	result := map[string]string{"json": `[{"source":"10.0.0.0/24"}]`}
	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, result))

	r := <-done
	require.NoError(t, r.err)
	assert.Equal(t, `[{"source":"10.0.0.0/24"}]`, r.json)
}

// TestRPCBye verifies clean shutdown via bye RPC.
//
// VALIDATES: Engine sends bye, plugin receives and acknowledges.
// PREVENTS: Shutdown not being communicated to plugin.
func TestRPCBye(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan error, 1)
	go func() {
		done <- engineConn.SendBye(context.Background(), "shutdown")
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:bye", req.Method)

	var input rpc.ByeInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, "shutdown", input.Reason)

	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, nil))
	require.NoError(t, <-done)
}

// TestSlowPluginFatal verifies that slow event delivery causes a timeout error.
//
// VALIDATES: deliver-event with timeout returns error if plugin doesn't respond.
// PREVENTS: Engine hanging indefinitely on slow plugin.
func TestSlowPluginFatal(t *testing.T) {
	t.Parallel()

	engineConn, _ := newTestPluginConn(t)

	// Use a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Send event but plugin never reads — should timeout.
	// With net.Pipe, writes block when reader isn't reading,
	// so writeWithContext will return context.DeadlineExceeded.
	err := engineConn.SendDeliverEvent(ctx, `{"type":"bgp"}`)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestRPCRequestIDRoundTrip verifies that request IDs survive the round trip.
//
// VALIDATES: ID set on request is echoed in response.
// PREVENTS: ID correlation breaking.
func TestRPCRequestIDRoundTrip(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan error, 1)
	go func() {
		done <- pluginConn.SendReady(context.Background())
	}()

	req, err := engineConn.ReadRequest(context.Background())
	require.NoError(t, err)

	// Verify request has an ID
	assert.NotNil(t, req.ID)
	assert.True(t, len(req.ID) > 0)

	// Send response with same ID
	require.NoError(t, engineConn.SendResult(context.Background(), req.ID, nil))
	require.NoError(t, <-done)
}

// TestRPCErrorResponse verifies that error responses are propagated.
//
// VALIDATES: RPC error from engine is returned as Go error to plugin.
// PREVENTS: Errors being silently swallowed.
func TestRPCErrorResponse(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan error, 1)
	go func() {
		done <- pluginConn.SendReady(context.Background())
	}()

	req, err := engineConn.ReadRequest(context.Background())
	require.NoError(t, err)

	// Send error response
	require.NoError(t, engineConn.SendError(context.Background(), req.ID, "stage-timeout"))

	err = <-done
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stage-timeout")
}

// TestRPCFullStartupCycle verifies the complete 5-stage startup + bye via PluginConn.
//
// VALIDATES: All 5 stages execute in order using typed RPC methods.
// PREVENTS: Protocol violations when stages are chained together.
func TestRPCFullStartupCycle(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run plugin side in background (simulates a real plugin)
	errCh := make(chan error, 1)
	go func() {
		// Stage 1: plugin sends declare-registration
		if err := pluginConn.SendDeclareRegistration(ctx, &rpc.DeclareRegistrationInput{
			Families:    []rpc.FamilyDecl{{Name: "ipv4/unicast", Mode: "both"}},
			Commands:    []rpc.CommandDecl{{Name: "show-routes", Description: "Show routes"}},
			WantsConfig: []string{"bgp"},
		}); err != nil {
			errCh <- err
			return
		}

		// Stage 2: plugin receives configure
		req, err := pluginConn.ReadRequest(ctx)
		if err != nil {
			errCh <- err
			return
		}
		if err := pluginConn.SendResult(ctx, req.ID, nil); err != nil {
			errCh <- err
			return
		}

		// Stage 3: plugin sends declare-capabilities
		if err := pluginConn.SendDeclareCapabilities(ctx, &rpc.DeclareCapabilitiesInput{}); err != nil {
			errCh <- err
			return
		}

		// Stage 4: plugin receives share-registry
		req, err = pluginConn.ReadRequest(ctx)
		if err != nil {
			errCh <- err
			return
		}
		if err := pluginConn.SendResult(ctx, req.ID, nil); err != nil {
			errCh <- err
			return
		}

		// Stage 5: plugin sends ready
		if err := pluginConn.SendReady(ctx); err != nil {
			errCh <- err
			return
		}

		// Runtime: plugin receives bye
		req, err = pluginConn.ReadRequest(ctx)
		if err != nil {
			errCh <- err
			return
		}

		var bye rpc.ByeInput
		if err := json.Unmarshal(req.Params, &bye); err != nil {
			errCh <- err
			return
		}
		if err := pluginConn.SendResult(ctx, req.ID, nil); err != nil {
			errCh <- err
			return
		}

		errCh <- nil
	}()

	// Engine side: orchestrate the 5-stage protocol

	// Stage 1: engine reads declare-registration
	req, err := engineConn.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)

	var regInput rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(req.Params, &regInput))
	assert.Equal(t, 1, len(regInput.Families))
	assert.Equal(t, []string{"bgp"}, regInput.WantsConfig)
	require.NoError(t, engineConn.SendResult(ctx, req.ID, nil))

	// Stage 2: engine sends configure
	require.NoError(t, engineConn.SendConfigure(ctx, []rpc.ConfigSection{
		{Root: "bgp", Data: `{"router-id":"1.2.3.4"}`},
	}))

	// Stage 3: engine reads declare-capabilities
	req, err = engineConn.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
	require.NoError(t, engineConn.SendResult(ctx, req.ID, nil))

	// Stage 4: engine sends share-registry
	require.NoError(t, engineConn.SendShareRegistry(ctx, []rpc.RegistryCommand{
		{Name: "show-routes", Plugin: "test-plugin", Encoding: "text"},
	}))

	// Stage 5: engine reads ready
	req, err = engineConn.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engineConn.SendResult(ctx, req.ID, nil))

	// Shutdown: engine sends bye
	require.NoError(t, engineConn.SendBye(ctx, "test-complete"))

	// Plugin should exit cleanly
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit after bye")
	}
}

// TestEngineCallRPCIDVerification verifies that engine-side callRPC rejects
// responses with mismatched IDs.
//
// VALIDATES: Engine detects response ID mismatch (same safety as SDK).
// PREVENTS: Buggy plugin sending wrong response ID going undetected.
func TestEngineCallRPCIDVerification(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Engine sends an RPC via callRPC
	done := make(chan error, 1)
	go func() {
		done <- engineConn.SendReady(ctx)
	}()

	// Plugin reads request
	req, err := pluginConn.ReadRequest(ctx)
	require.NoError(t, err)

	// Plugin responds with WRONG ID (original ID + 999)
	wrongID := json.RawMessage(`999`)
	_ = req.ID // suppress unused
	require.NoError(t, pluginConn.SendResult(ctx, wrongID, nil))

	// Engine should detect the mismatch
	err = <-done
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id mismatch")
}

// TestEngineDecodeCapability verifies decode-capability RPC from engine to plugin.
//
// VALIDATES: Engine sends decode-capability, plugin returns decoded JSON.
// PREVENTS: decode-capability not being available on engine side.
func TestEngineDecodeCapability(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan struct {
		json string
		err  error
	}, 1)
	go func() {
		j, err := engineConn.SendDecodeCapability(context.Background(), 73, "07686f73746e616d65")
		done <- struct {
			json string
			err  error
		}{j, err}
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:decode-capability", req.Method)

	var input rpc.DecodeCapabilityInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, uint8(73), input.Code)
	assert.Equal(t, "07686f73746e616d65", input.Hex)

	result := map[string]string{"json": `{"hostname":"router1"}`}
	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, result))

	r := <-done
	require.NoError(t, r.err)
	assert.Equal(t, `{"hostname":"router1"}`, r.json)
}

// TestEngineExecuteCommand verifies execute-command RPC from engine to plugin.
//
// VALIDATES: Engine sends execute-command, plugin returns status + data.
// PREVENTS: execute-command not being available on engine side.
func TestEngineExecuteCommand(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan struct {
		output *rpc.ExecuteCommandOutput
		err    error
	}, 1)
	go func() {
		out, err := engineConn.SendExecuteCommand(context.Background(), "abc123", "show-routes", []string{"ipv4"}, "10.0.0.1")
		done <- struct {
			output *rpc.ExecuteCommandOutput
			err    error
		}{out, err}
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:execute-command", req.Method)

	var input rpc.ExecuteCommandInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, "abc123", input.Serial)
	assert.Equal(t, "show-routes", input.Command)
	assert.Equal(t, []string{"ipv4"}, input.Args)
	assert.Equal(t, "10.0.0.1", input.Peer)

	result := &rpc.ExecuteCommandOutput{Status: "done", Data: `{"routes":[]}`}
	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, result))

	r := <-done
	require.NoError(t, r.err)
	assert.Equal(t, "done", r.output.Status)
	assert.Equal(t, `{"routes":[]}`, r.output.Data)
}

// TestSendConfigVerifyOK verifies config-verify RPC with successful response.
//
// VALIDATES: Engine sends config-verify, plugin responds OK with status "ok".
// PREVENTS: Config verify RPC not being available on engine side.
func TestSendConfigVerifyOK(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan struct {
		output *rpc.ConfigVerifyOutput
		err    error
	}, 1)
	go func() {
		out, err := engineConn.SendConfigVerify(context.Background(), []rpc.ConfigSection{
			{Root: "bgp", Data: `{"router-id":"1.2.3.4"}`},
		})
		done <- struct {
			output *rpc.ConfigVerifyOutput
			err    error
		}{out, err}
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:config-verify", req.Method)

	var input rpc.ConfigVerifyInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, 1, len(input.Sections))
	assert.Equal(t, "bgp", input.Sections[0].Root)

	result := &rpc.ConfigVerifyOutput{Status: "ok"}
	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, result))

	r := <-done
	require.NoError(t, r.err)
	assert.Equal(t, "ok", r.output.Status)
	assert.Empty(t, r.output.Error)
}

// TestSendConfigVerifyError verifies config-verify RPC with rejection response.
//
// VALIDATES: Engine sends config-verify, plugin responds with status "error".
// PREVENTS: Config verify errors not being propagated to engine.
func TestSendConfigVerifyError(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan struct {
		output *rpc.ConfigVerifyOutput
		err    error
	}, 1)
	go func() {
		out, err := engineConn.SendConfigVerify(context.Background(), []rpc.ConfigSection{
			{Root: "bgp", Data: `{"invalid":true}`},
		})
		done <- struct {
			output *rpc.ConfigVerifyOutput
			err    error
		}{out, err}
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:config-verify", req.Method)

	result := &rpc.ConfigVerifyOutput{Status: "error", Error: "invalid config: missing router-id"}
	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, result))

	r := <-done
	require.NoError(t, r.err)
	assert.Equal(t, "error", r.output.Status)
	assert.Equal(t, "invalid config: missing router-id", r.output.Error)
}

// TestSendConfigApplyOK verifies config-apply RPC with successful response.
//
// VALIDATES: Engine sends config-apply with diff sections, plugin responds OK.
// PREVENTS: Config apply RPC not being available on engine side.
func TestSendConfigApplyOK(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan struct {
		output *rpc.ConfigApplyOutput
		err    error
	}, 1)
	go func() {
		out, err := engineConn.SendConfigApply(context.Background(), []rpc.ConfigDiffSection{
			{Root: "bgp", Added: `{"peer":{"p1":{}}}`, Changed: `{"router-id":"5.6.7.8"}`},
		})
		done <- struct {
			output *rpc.ConfigApplyOutput
			err    error
		}{out, err}
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:config-apply", req.Method)

	var input rpc.ConfigApplyInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, 1, len(input.Sections))
	assert.Equal(t, "bgp", input.Sections[0].Root)
	assert.Equal(t, `{"peer":{"p1":{}}}`, input.Sections[0].Added)
	assert.Equal(t, `{"router-id":"5.6.7.8"}`, input.Sections[0].Changed)

	result := &rpc.ConfigApplyOutput{Status: "ok"}
	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, result))

	r := <-done
	require.NoError(t, r.err)
	assert.Equal(t, "ok", r.output.Status)
}

// TestSendValidateOpenAccept verifies validate-open RPC when plugin accepts.
//
// VALIDATES: Engine sends validate-open, plugin responds with accept=true.
// PREVENTS: validate-open RPC not being available on engine side.
func TestSendValidateOpenAccept(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan struct {
		output *rpc.ValidateOpenOutput
		err    error
	}, 1)
	go func() {
		out, err := engineConn.SendValidateOpen(context.Background(), &rpc.ValidateOpenInput{
			Peer: "10.0.0.1",
			Local: rpc.ValidateOpenMessage{
				ASN:      65000,
				RouterID: "1.2.3.4",
				HoldTime: 180,
				Capabilities: []rpc.ValidateOpenCapability{
					{Code: 9, Hex: "03"}, // customer
				},
			},
			Remote: rpc.ValidateOpenMessage{
				ASN:      65001,
				RouterID: "5.6.7.8",
				HoldTime: 90,
				Capabilities: []rpc.ValidateOpenCapability{
					{Code: 9, Hex: "00"}, // provider
				},
			},
		})
		done <- struct {
			output *rpc.ValidateOpenOutput
			err    error
		}{out, err}
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:validate-open", req.Method)

	var input rpc.ValidateOpenInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Equal(t, "10.0.0.1", input.Peer)
	assert.Equal(t, uint32(65000), input.Local.ASN)
	assert.Equal(t, "1.2.3.4", input.Local.RouterID)
	assert.Equal(t, uint16(180), input.Local.HoldTime)
	require.Equal(t, 1, len(input.Local.Capabilities))
	assert.Equal(t, uint8(9), input.Local.Capabilities[0].Code)
	assert.Equal(t, "03", input.Local.Capabilities[0].Hex)
	assert.Equal(t, uint32(65001), input.Remote.ASN)
	assert.Equal(t, "00", input.Remote.Capabilities[0].Hex)

	result := &rpc.ValidateOpenOutput{Accept: true}
	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, result))

	r := <-done
	require.NoError(t, r.err)
	assert.True(t, r.output.Accept)
	assert.Equal(t, uint8(0), r.output.NotifyCode)
}

// TestSendValidateOpenReject verifies validate-open RPC when plugin rejects.
//
// VALIDATES: Engine sends validate-open, plugin responds with reject + NOTIFICATION codes.
// PREVENTS: Rejection reason/codes not propagating through RPC.
func TestSendValidateOpenReject(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	done := make(chan struct {
		output *rpc.ValidateOpenOutput
		err    error
	}, 1)
	go func() {
		out, err := engineConn.SendValidateOpen(context.Background(), &rpc.ValidateOpenInput{
			Peer: "10.0.0.2",
			Local: rpc.ValidateOpenMessage{
				ASN:          65000,
				RouterID:     "1.2.3.4",
				HoldTime:     180,
				Capabilities: []rpc.ValidateOpenCapability{{Code: 9, Hex: "03"}},
			},
			Remote: rpc.ValidateOpenMessage{
				ASN:          65002,
				RouterID:     "9.8.7.6",
				HoldTime:     90,
				Capabilities: []rpc.ValidateOpenCapability{{Code: 9, Hex: "03"}},
			},
		})
		done <- struct {
			output *rpc.ValidateOpenOutput
			err    error
		}{out, err}
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:validate-open", req.Method)

	// RFC 9234: Role Mismatch → NOTIFICATION 2/11.
	result := &rpc.ValidateOpenOutput{
		Accept:        false,
		NotifyCode:    2,
		NotifySubcode: 11,
		Reason:        "role mismatch: customer↔customer",
	}
	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, result))

	r := <-done
	require.NoError(t, r.err)
	assert.False(t, r.output.Accept)
	assert.Equal(t, uint8(2), r.output.NotifyCode)
	assert.Equal(t, uint8(11), r.output.NotifySubcode)
	assert.Equal(t, "role mismatch: customer↔customer", r.output.Reason)
}

// TestCapabilityCodeBoundary verifies boundary values for capability codes.
//
// VALIDATES: Capability code 0 and 255 are valid, no overflow.
// PREVENTS: Off-by-one errors in capability code handling.
// BOUNDARY: 0 (valid min), 255 (valid max uint8).
func TestCapabilityCodeBoundary(t *testing.T) {
	t.Parallel()

	// Test boundary values in a DeclareCapabilitiesInput
	caps := &rpc.DeclareCapabilitiesInput{
		Capabilities: []rpc.CapabilityDecl{
			{Code: 0, Encoding: "hex", Payload: ""},
			{Code: 255, Encoding: "hex", Payload: "FF"},
		},
	}

	data, err := json.Marshal(caps)
	require.NoError(t, err)

	var decoded rpc.DeclareCapabilitiesInput
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, uint8(0), decoded.Capabilities[0].Code)
	assert.Equal(t, uint8(255), decoded.Capabilities[1].Code)
}

// TestRPCDeliverBatch verifies batched event delivery via callback RPC.
//
// VALIDATES: AC-2 — N events delivered in one batch write, one ack.
// PREVENTS: Events lost or reordered during batch delivery.
func TestRPCDeliverBatch(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	events := []string{
		`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`,
		`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.2"}}}`,
		`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.3"}}}`,
	}

	done := make(chan error, 1)
	go func() {
		done <- engineConn.SendDeliverBatch(context.Background(), events)
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:deliver-batch", req.Method)

	var input struct {
		Events []json.RawMessage `json:"events"`
	}
	require.NoError(t, json.Unmarshal(req.Params, &input))
	require.Len(t, input.Events, 3)

	for i, raw := range input.Events {
		var eventStr string
		require.NoError(t, json.Unmarshal(raw, &eventStr), "event %d must be a JSON string", i)
		assert.JSONEq(t, events[i], eventStr)
	}

	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, nil))
	require.NoError(t, <-done)
}

// TestRPCDeliverBatchSingle verifies batch delivery with one event.
//
// VALIDATES: AC-1 — single event delivered as batch of 1.
// PREVENTS: Edge case where batch of 1 fails.
func TestRPCDeliverBatchSingle(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	events := []string{`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`}

	done := make(chan error, 1)
	go func() {
		done <- engineConn.SendDeliverBatch(context.Background(), events)
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:deliver-batch", req.Method)

	var input struct {
		Events []json.RawMessage `json:"events"`
	}
	require.NoError(t, json.Unmarshal(req.Params, &input))
	require.Len(t, input.Events, 1)

	var eventStr string
	require.NoError(t, json.Unmarshal(input.Events[0], &eventStr))
	assert.JSONEq(t, events[0], eventStr)

	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, nil))
	require.NoError(t, <-done)
}

// TestRPCDeliverBatchTextEvents verifies batch delivery of text-format events.
//
// VALIDATES: Text events (non-JSON) survive batch framing and produce valid JSON.
// PREVENTS: Text events producing invalid JSON in batch frame (startup race crash).
func TestRPCDeliverBatchTextEvents(t *testing.T) {
	t.Parallel()

	engineConn, pluginConn := newTestPluginConn(t)

	events := []string{
		"peer 10.0.0.1 received update 42 announce origin igp ipv4/unicast next-hop 10.0.0.1 nlri 192.168.1.0/24\n",
		"peer 10.0.0.2 state up\n",
	}

	done := make(chan error, 1)
	go func() {
		done <- engineConn.SendDeliverBatch(context.Background(), events)
	}()

	req, err := pluginConn.ReadRequest(context.Background())
	require.NoError(t, err, "text events must produce a valid JSON-RPC frame")
	assert.Equal(t, "ze-plugin-callback:deliver-batch", req.Method)

	// Parse batch — each event must be a valid JSON string that unwraps to the original text
	var input struct {
		Events []json.RawMessage `json:"events"`
	}
	require.NoError(t, json.Unmarshal(req.Params, &input))
	require.Len(t, input.Events, 2)

	for i, raw := range input.Events {
		var eventStr string
		require.NoError(t, json.Unmarshal(raw, &eventStr), "event %d must be a JSON string", i)
		assert.Equal(t, events[i], eventStr)
	}

	require.NoError(t, pluginConn.SendResult(context.Background(), req.ID, nil))
	require.NoError(t, <-done)
}

// TestRPCDeliverBatchTimeout verifies batch delivery respects context deadline.
//
// VALIDATES: AC-5 — batch write respects context deadline.
// PREVENTS: Engine hanging on slow plugin during batch delivery.
func TestRPCDeliverBatchTimeout(t *testing.T) {
	t.Parallel()

	engineConn, _ := newTestPluginConn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Plugin never reads — should timeout.
	// Under load, the timeout may surface as context.DeadlineExceeded (read phase)
	// or os.ErrDeadlineExceeded / i/o timeout (write phase, kernel deadline fires
	// before Go runtime context timer). Both are correct timeout behavior.
	err := engineConn.SendDeliverBatch(ctx, []string{`{"type":"bgp"}`})
	require.Error(t, err)
	isCtxTimeout := errors.Is(err, context.DeadlineExceeded)
	isIOTimeout := errors.Is(err, os.ErrDeadlineExceeded)
	assert.True(t, isCtxTimeout || isIOTimeout, "expected timeout error, got: %v", err)
}

// TestTextHandshakeRoundTrip verifies a full 5-stage text handshake over socket pairs.
// Engine side reads text stages 1,3,5 from Socket A; sends text stages 2,4 on Socket B.
// Plugin side formats text, sends, and reads responses.
//
// VALIDATES: AC-1 (text handshake completes all 5 stages), AC-3 (families round-trip),
//
//	AC-4 (config heredoc round-trip), AC-5 (capability injection from text),
//	AC-6 (registry sharing in text), AC-7 (event subscription in ready).
//
// PREVENTS: Protocol mismatch between format/parse functions and TextConn framing.
func TestTextHandshakeRoundTrip(t *testing.T) {
	t.Parallel()

	pairs, err := NewInternalSocketPairs()
	require.NoError(t, err)
	t.Cleanup(func() { pairs.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Engine side: per-socket TextConns.
	engineA := rpc.NewTextConn(pairs.Engine.EngineSide, pairs.Engine.EngineSide)
	engineB := rpc.NewTextConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide)

	// Plugin side: per-socket TextConns.
	pluginA := rpc.NewTextConn(pairs.Engine.PluginSide, pairs.Engine.PluginSide)
	pluginB := rpc.NewTextConn(pairs.Callback.PluginSide, pairs.Callback.PluginSide)

	// Test data for all stages.
	regInput := rpc.DeclareRegistrationInput{
		Families: []rpc.FamilyDecl{
			{Name: "ipv4/unicast", Mode: "both"},
			{Name: "ipv4/flow", Mode: "decode"},
		},
		Commands: []rpc.CommandDecl{
			{Name: "show-routes", Description: "Show routes", Args: []string{"family", "prefix"}, Completable: true},
		},
		WantsConfig: []string{"bgp"},
	}

	sections := []rpc.ConfigSection{
		{Root: "bgp", Data: `{"router-id":"1.2.3.4","asn":65000}`},
	}

	capsInput := rpc.DeclareCapabilitiesInput{
		Capabilities: []rpc.CapabilityDecl{
			{Code: 64, Encoding: "hex", Payload: "40780078"},
			{Code: 2},
		},
	}

	registryInput := rpc.ShareRegistryInput{
		Commands: []rpc.RegistryCommand{
			{Name: "show-routes", Plugin: "test-plugin", Encoding: "text"},
		},
	}

	readyInput := rpc.ReadyInput{
		Subscribe: &rpc.SubscribeEventsInput{
			Events:   []string{"update", "state"},
			Encoding: "text",
			Peers:    []string{"10.0.0.1"},
		},
	}

	// Plugin side goroutine — formats and sends text stages.
	errCh := make(chan error, 1)
	go func() {
		// Stage 1: plugin sends registration on Socket A
		text, fmtErr := rpc.FormatRegistrationText(regInput)
		if fmtErr != nil {
			errCh <- fmt.Errorf("format reg: %w", fmtErr)
			return
		}
		if writeErr := pluginA.WriteMessage(ctx, text); writeErr != nil {
			errCh <- fmt.Errorf("write reg: %w", writeErr)
			return
		}
		resp, readErr := pluginA.ReadLine(ctx)
		if readErr != nil {
			errCh <- fmt.Errorf("read reg resp: %w", readErr)
			return
		}
		if resp != "ok" {
			errCh <- fmt.Errorf("stage 1 resp: expected ok, got %q", resp)
			return
		}

		// Stage 2: plugin reads configure from Socket B
		configText, readErr := pluginB.ReadMessage(ctx)
		if readErr != nil {
			errCh <- fmt.Errorf("read config: %w", readErr)
			return
		}
		parsed, parseErr := rpc.ParseConfigureText(configText)
		if parseErr != nil {
			errCh <- fmt.Errorf("parse config: %w", parseErr)
			return
		}
		if len(parsed.Sections) != 1 || parsed.Sections[0].Root != "bgp" {
			errCh <- fmt.Errorf("unexpected config: %+v", parsed.Sections)
			return
		}
		if writeErr := pluginB.WriteLine(ctx, "ok"); writeErr != nil {
			errCh <- fmt.Errorf("write config resp: %w", writeErr)
			return
		}

		// Stage 3: plugin sends capabilities on Socket A
		capsText, fmtErr := rpc.FormatCapabilitiesText(capsInput)
		if fmtErr != nil {
			errCh <- fmt.Errorf("format caps: %w", fmtErr)
			return
		}
		if writeErr := pluginA.WriteMessage(ctx, capsText); writeErr != nil {
			errCh <- fmt.Errorf("write caps: %w", writeErr)
			return
		}
		resp, readErr = pluginA.ReadLine(ctx)
		if readErr != nil {
			errCh <- fmt.Errorf("read caps resp: %w", readErr)
			return
		}
		if resp != "ok" {
			errCh <- fmt.Errorf("stage 3 resp: expected ok, got %q", resp)
			return
		}

		// Stage 4: plugin reads registry from Socket B
		regText, readErr := pluginB.ReadMessage(ctx)
		if readErr != nil {
			errCh <- fmt.Errorf("read registry: %w", readErr)
			return
		}
		regParsed, parseErr := rpc.ParseRegistryText(regText)
		if parseErr != nil {
			errCh <- fmt.Errorf("parse registry: %w", parseErr)
			return
		}
		if len(regParsed.Commands) != 1 || regParsed.Commands[0].Name != "show-routes" {
			errCh <- fmt.Errorf("unexpected registry: %+v", regParsed.Commands)
			return
		}
		if writeErr := pluginB.WriteLine(ctx, "ok"); writeErr != nil {
			errCh <- fmt.Errorf("write registry resp: %w", writeErr)
			return
		}

		// Stage 5: plugin sends ready on Socket A
		readyText, fmtErr := rpc.FormatReadyText(readyInput)
		if fmtErr != nil {
			errCh <- fmt.Errorf("format ready: %w", fmtErr)
			return
		}
		if writeErr := pluginA.WriteMessage(ctx, readyText); writeErr != nil {
			errCh <- fmt.Errorf("write ready: %w", writeErr)
			return
		}
		resp, readErr = pluginA.ReadLine(ctx)
		if readErr != nil {
			errCh <- fmt.Errorf("read ready resp: %w", readErr)
			return
		}
		if resp != "ok" {
			errCh <- fmt.Errorf("stage 5 resp: expected ok, got %q", resp)
			return
		}

		errCh <- nil
	}()

	// Engine side: reads text stages from Socket A, sends on Socket B.

	// Stage 1: read registration from Socket A
	regText, err := engineA.ReadMessage(ctx)
	require.NoError(t, err, "engine read stage 1")
	receivedReg, err := rpc.ParseRegistrationText(regText)
	require.NoError(t, err, "engine parse stage 1")
	assert.Equal(t, 2, len(receivedReg.Families))
	assert.Equal(t, "ipv4/unicast", receivedReg.Families[0].Name)
	assert.Equal(t, "both", receivedReg.Families[0].Mode)
	assert.Equal(t, "ipv4/flow", receivedReg.Families[1].Name)
	assert.Equal(t, "decode", receivedReg.Families[1].Mode)
	assert.Equal(t, 1, len(receivedReg.Commands))
	assert.Equal(t, "show-routes", receivedReg.Commands[0].Name)
	assert.Equal(t, "Show routes", receivedReg.Commands[0].Description)
	assert.Equal(t, []string{"family", "prefix"}, receivedReg.Commands[0].Args)
	assert.True(t, receivedReg.Commands[0].Completable)
	assert.Equal(t, []string{"bgp"}, receivedReg.WantsConfig)
	require.NoError(t, engineA.WriteLine(ctx, "ok"))

	// Stage 2: send configure on Socket B
	configText, err := rpc.FormatConfigureText(rpc.ConfigureInput{Sections: sections})
	require.NoError(t, err, "engine format stage 2")
	require.NoError(t, engineB.WriteMessage(ctx, configText))
	resp, err := engineB.ReadLine(ctx)
	require.NoError(t, err, "engine read stage 2 resp")
	assert.Equal(t, "ok", resp)

	// Stage 3: read capabilities from Socket A
	capsText, err := engineA.ReadMessage(ctx)
	require.NoError(t, err, "engine read stage 3")
	receivedCaps, err := rpc.ParseCapabilitiesText(capsText)
	require.NoError(t, err, "engine parse stage 3")
	assert.Equal(t, 2, len(receivedCaps.Capabilities))
	assert.Equal(t, uint8(64), receivedCaps.Capabilities[0].Code)
	assert.Equal(t, "hex", receivedCaps.Capabilities[0].Encoding)
	assert.Equal(t, "40780078", receivedCaps.Capabilities[0].Payload)
	assert.Equal(t, uint8(2), receivedCaps.Capabilities[1].Code)
	require.NoError(t, engineA.WriteLine(ctx, "ok"))

	// Stage 4: send registry on Socket B
	regText2, err := rpc.FormatRegistryText(registryInput)
	require.NoError(t, err, "engine format stage 4")
	require.NoError(t, engineB.WriteMessage(ctx, regText2))
	resp, err = engineB.ReadLine(ctx)
	require.NoError(t, err, "engine read stage 4 resp")
	assert.Equal(t, "ok", resp)

	// Stage 5: read ready from Socket A
	readyText, err := engineA.ReadMessage(ctx)
	require.NoError(t, err, "engine read stage 5")
	receivedReady, err := rpc.ParseReadyText(readyText)
	require.NoError(t, err, "engine parse stage 5")
	require.NotNil(t, receivedReady.Subscribe)
	assert.Equal(t, []string{"update", "state"}, receivedReady.Subscribe.Events)
	assert.Equal(t, "text", receivedReady.Subscribe.Encoding)
	assert.Equal(t, []string{"10.0.0.1"}, receivedReady.Subscribe.Peers)
	require.NoError(t, engineA.WriteLine(ctx, "ok"))

	// Wait for plugin side
	select {
	case pluginErr := <-errCh:
		require.NoError(t, pluginErr, "plugin side error")
	case <-time.After(3 * time.Second):
		t.Fatal("plugin did not complete handshake")
	}
}
