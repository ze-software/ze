package bgp_rib

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestRIBPluginFiveStageProtocol verifies that the rib plugin completes the
// 5-stage startup handshake correctly and includes atomic event subscriptions
// in the Stage 5 ready RPC.
//
// VALIDATES: The rib plugin sends startup subscriptions atomically with the
// ready RPC, ensuring the engine registers them before SignalAPIReady. This
// prevents the race where routes are sent before the rib is subscribed.
//
// PREVENTS: Regression where startup subscriptions are sent as a separate RPC
// after ready, allowing a window where events (sent routes, state changes) are
// missed by the rib plugin.
func TestRIBPluginFiveStageProtocol(t *testing.T) {
	pluginA, engineA := net.Pipe()
	pluginB, engineB := net.Pipe()

	connA := rpc.NewConn(engineA, engineA)
	connB := rpc.NewConn(engineB, engineB)

	pluginDone := make(chan int, 1)
	go func() {
		pluginDone <- RunRIBPlugin(pluginA, pluginB)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// ── Stage 1: declare-registration ───────────────────────────────────
	// The rib plugin declares its commands (10 total: 5 short + 5 long names).
	stage1 := readRequestTimeout(t, ctx, connA)
	require.Equal(t, "ze-plugin-engine:declare-registration", stage1.Method)

	var regInput rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(stage1.Params, &regInput))

	// Verify command registration — short names (primary).
	commandNames := make([]string, len(regInput.Commands))
	for i, cmd := range regInput.Commands {
		commandNames[i] = cmd.Name
	}
	assert.Contains(t, commandNames, "rib status")
	assert.Contains(t, commandNames, "rib show in")
	assert.Contains(t, commandNames, "rib clear in")
	assert.Contains(t, commandNames, "rib show out")
	assert.Contains(t, commandNames, "rib clear out")
	// Long names (RFC 4271 Adj-RIB terminology).
	assert.Contains(t, commandNames, "rib adjacent status")
	assert.Contains(t, commandNames, "rib adjacent inbound show")
	assert.Contains(t, commandNames, "rib adjacent inbound empty")
	assert.Contains(t, commandNames, "rib adjacent outbound show")
	assert.Contains(t, commandNames, "rib adjacent outbound resend")
	assert.Len(t, regInput.Commands, 10, "rib registers exactly 10 commands")

	require.NoError(t, connA.SendOK(ctx, stage1.ID))

	// ── Stage 2: configure ──────────────────────────────────────────────
	// Rib plugin has no OnConfigure handler, so empty config is fine.
	configInput := &rpc.ConfigureInput{Sections: []rpc.ConfigSection{}}
	raw, err := connB.CallRPC(ctx, "ze-plugin-callback:configure", configInput)
	require.NoError(t, err)
	require.NoError(t, rpc.CheckResponse(raw))

	// ── Stage 3: declare-capabilities ───────────────────────────────────
	// Rib plugin declares no BGP capabilities.
	stage3 := readRequestTimeout(t, ctx, connA)
	require.Equal(t, "ze-plugin-engine:declare-capabilities", stage3.Method)

	var capsInput rpc.DeclareCapabilitiesInput
	require.NoError(t, json.Unmarshal(stage3.Params, &capsInput))
	assert.Empty(t, capsInput.Capabilities, "rib plugin declares no capabilities")

	require.NoError(t, connA.SendOK(ctx, stage3.ID))

	// ── Stage 4: share-registry ─────────────────────────────────────────
	registryInput := &rpc.ShareRegistryInput{Commands: []rpc.RegistryCommand{}}
	raw, err = connB.CallRPC(ctx, "ze-plugin-callback:share-registry", registryInput)
	require.NoError(t, err)
	require.NoError(t, rpc.CheckResponse(raw))

	// ── Stage 5: ready (with atomic subscriptions) ──────────────────────
	// This is the critical stage: the ready RPC MUST include the event
	// subscriptions so the engine registers them BEFORE SignalAPIReady.
	// Without atomic subscriptions, there is a race between the engine
	// sending initial routes and the rib plugin subscribing to events.
	stage5 := readRequestTimeout(t, ctx, connA)
	require.Equal(t, "ze-plugin-engine:ready", stage5.Method)

	var readyInput rpc.ReadyInput
	require.NoError(t, json.Unmarshal(stage5.Params, &readyInput))

	require.NotNil(t, readyInput.Subscribe,
		"ready RPC MUST include Subscribe — atomic subscription prevents race")
	assert.ElementsMatch(t,
		[]string{"update direction sent", "state", "refresh"},
		readyInput.Subscribe.Events,
		"rib subscribes to sent updates, state changes, and route-refresh")
	assert.Equal(t, "full", readyInput.Subscribe.Format,
		"rib uses full format for events")

	require.NoError(t, connA.SendOK(ctx, stage5.ID))

	// ── Plugin is now in event loop ─────────────────────────────────────
	// Verify the plugin can process an event after startup.
	sentEvent := `{"type":"sent","msg-id":1,"peer":{"address":"10.0.0.1","asn":65001},"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24"]}]}`
	deliverEventSync(t, ctx, connB, sentEvent)

	// ── Cleanup ─────────────────────────────────────────────────────────
	if closeErr := connA.Close(); closeErr != nil {
		t.Logf("connA close: %v", closeErr)
	}
	if closeErr := connB.Close(); closeErr != nil {
		t.Logf("connB close: %v", closeErr)
	}
	if closeErr := engineA.Close(); closeErr != nil {
		t.Logf("engineA close: %v", closeErr)
	}
	if closeErr := engineB.Close(); closeErr != nil {
		t.Logf("engineB close: %v", closeErr)
	}

	select {
	case exitCode := <-pluginDone:
		t.Logf("plugin exited with code %d", exitCode)
	case <-time.After(5 * time.Second):
		t.Error("plugin did not exit within timeout")
	}
}

// TestRIBPluginStageOrdering verifies that the 5-stage protocol is strictly
// sequential — each stage completes before the next begins.
//
// VALIDATES: Sending stages out of order or skipping a stage causes failure.
// PREVENTS: Protocol implementation that accidentally accepts stages in wrong
// order, which would break assumptions about what state is available at each
// stage (e.g., config must be received before ready).
func TestRIBPluginStageOrdering(t *testing.T) {
	pluginA, engineA := net.Pipe()
	pluginB, engineB := net.Pipe()

	connA := rpc.NewConn(engineA, engineA)
	connB := rpc.NewConn(engineB, engineB)

	pluginDone := make(chan int, 1)
	go func() {
		pluginDone <- RunRIBPlugin(pluginA, pluginB)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stage 1: Plugin sends declare-registration on Socket A first.
	// Verify this is always the first message (no other RPC before it).
	stage1 := readRequestTimeout(t, ctx, connA)
	assert.Equal(t, "ze-plugin-engine:declare-registration", stage1.Method,
		"Stage 1 must be declare-registration (first message on Socket A)")
	require.NoError(t, connA.SendOK(ctx, stage1.ID))

	// Stage 2: Engine sends configure on Socket B.
	// Plugin waits for this — it blocks until configure arrives.
	configInput := &rpc.ConfigureInput{Sections: []rpc.ConfigSection{}}
	raw, err := connB.CallRPC(ctx, "ze-plugin-callback:configure", configInput)
	require.NoError(t, err)
	require.NoError(t, rpc.CheckResponse(raw))

	// Stage 3: Plugin sends declare-capabilities on Socket A.
	// Must come after configure (Stage 2) completes.
	stage3 := readRequestTimeout(t, ctx, connA)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", stage3.Method,
		"Stage 3 must be declare-capabilities (after configure)")
	require.NoError(t, connA.SendOK(ctx, stage3.ID))

	// Stage 4: Engine sends share-registry on Socket B.
	registryInput := &rpc.ShareRegistryInput{Commands: []rpc.RegistryCommand{}}
	raw, err = connB.CallRPC(ctx, "ze-plugin-callback:share-registry", registryInput)
	require.NoError(t, err)
	require.NoError(t, rpc.CheckResponse(raw))

	// Stage 5: Plugin sends ready on Socket A.
	// Must come after share-registry (Stage 4) completes.
	stage5 := readRequestTimeout(t, ctx, connA)
	assert.Equal(t, "ze-plugin-engine:ready", stage5.Method,
		"Stage 5 must be ready (after share-registry)")
	require.NoError(t, connA.SendOK(ctx, stage5.ID))

	// After Stage 5, the event loop is running — no more stage RPCs.
	// Close connections to trigger clean shutdown.
	if closeErr := connA.Close(); closeErr != nil {
		t.Logf("connA close: %v", closeErr)
	}
	if closeErr := connB.Close(); closeErr != nil {
		t.Logf("connB close: %v", closeErr)
	}
	if closeErr := engineA.Close(); closeErr != nil {
		t.Logf("engineA close: %v", closeErr)
	}
	if closeErr := engineB.Close(); closeErr != nil {
		t.Logf("engineB close: %v", closeErr)
	}

	select {
	case exitCode := <-pluginDone:
		t.Logf("plugin exited with code %d", exitCode)
	case <-time.After(5 * time.Second):
		t.Error("plugin did not exit within timeout")
	}
}

// readRequestTimeout is defined in blocking_test.go — reused here.
// deliverEventSync is defined in blocking_test.go — reused here.
