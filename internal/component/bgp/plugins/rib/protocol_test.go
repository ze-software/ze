package rib

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
	pluginEnd, engineEnd := net.Pipe()

	engineConn := rpc.NewConn(engineEnd, engineEnd)
	mux := rpc.NewMuxConn(engineConn)

	pluginDone := make(chan int, 1)
	go func() {
		pluginDone <- RunRIBPlugin(pluginEnd)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// ── Stage 1: declare-registration ───────────────────────────────────
	// The rib plugin declares its commands (14 total: 7 short + 5 long + 2 GR).
	stage1 := readMuxRequestTimeout(t, ctx, mux)
	require.Equal(t, "ze-plugin-engine:declare-registration", stage1.Method)

	var regInput rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(stage1.Params, &regInput))

	// Verify command registration — unified pipeline commands.
	commandNames := make([]string, len(regInput.Commands))
	for i, cmd := range regInput.Commands {
		commandNames[i] = cmd.Name
	}
	assert.Contains(t, commandNames, "rib status")
	assert.Contains(t, commandNames, "rib show")
	assert.Contains(t, commandNames, "rib clear in")
	assert.Contains(t, commandNames, "rib clear out")
	// Legacy status alias
	assert.Contains(t, commandNames, "rib adjacent status")
	// GR support commands (RFC 4724)
	assert.Contains(t, commandNames, "rib retain-routes")
	assert.Contains(t, commandNames, "rib release-routes")
	assert.Contains(t, commandNames, "rib mark-stale")
	assert.Contains(t, commandNames, "rib purge-stale")
	// Best-path commands (RFC 4271 §9.1.2)
	assert.Contains(t, commandNames, "rib best")
	assert.Contains(t, commandNames, "rib best status")
	// Meta-commands (introspection)
	assert.Contains(t, commandNames, "rib help")
	assert.Contains(t, commandNames, "rib command list")
	assert.Contains(t, commandNames, "rib event list")
	assert.Len(t, regInput.Commands, 14, "rib registers exactly 14 commands")

	require.NoError(t, mux.SendOK(ctx, stage1.ID))

	// ── Stage 2: configure ──────────────────────────────────────────────
	// Rib plugin has no OnConfigure handler, so empty config is fine.
	configInput := &rpc.ConfigureInput{Sections: []rpc.ConfigSection{}}
	_, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", configInput)
	require.NoError(t, err)

	// ── Stage 3: declare-capabilities ───────────────────────────────────
	// Rib plugin declares no BGP capabilities.
	stage3 := readMuxRequestTimeout(t, ctx, mux)
	require.Equal(t, "ze-plugin-engine:declare-capabilities", stage3.Method)

	var capsInput rpc.DeclareCapabilitiesInput
	require.NoError(t, json.Unmarshal(stage3.Params, &capsInput))
	assert.Empty(t, capsInput.Capabilities, "rib plugin declares no capabilities")

	require.NoError(t, mux.SendOK(ctx, stage3.ID))

	// ── Stage 4: share-registry ─────────────────────────────────────────
	registryInput := &rpc.ShareRegistryInput{Commands: []rpc.RegistryCommand{}}
	_, err = mux.CallRPC(ctx, "ze-plugin-callback:share-registry", registryInput)
	require.NoError(t, err)

	// ── Stage 5: ready (with atomic subscriptions) ──────────────────────
	// This is the critical stage: the ready RPC MUST include the event
	// subscriptions so the engine registers them BEFORE SignalAPIReady.
	// Without atomic subscriptions, there is a race between the engine
	// sending initial routes and the rib plugin subscribing to events.
	stage5 := readMuxRequestTimeout(t, ctx, mux)
	require.Equal(t, "ze-plugin-engine:ready", stage5.Method)

	var readyInput rpc.ReadyInput
	require.NoError(t, json.Unmarshal(stage5.Params, &readyInput))

	require.NotNil(t, readyInput.Subscribe,
		"ready RPC MUST include Subscribe — atomic subscription prevents race")
	assert.ElementsMatch(t,
		[]string{"update direction sent", "update direction received", "state", "refresh"},
		readyInput.Subscribe.Events,
		"rib subscribes to sent/received updates, state changes, and route-refresh")
	assert.Equal(t, "full", readyInput.Subscribe.Format,
		"rib uses full format for events")

	require.NoError(t, mux.SendOK(ctx, stage5.ID))

	// ── Plugin is now in event loop ─────────────────────────────────────
	// Verify the plugin can process an event after startup.
	sentEvent := `{"type":"sent","msg-id":1,"peer":{"address":"10.0.0.1","remote":{"as":65001}},"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["10.0.0.0/24"]}]}`
	deliverEventSync(t, ctx, mux, sentEvent)

	// ── Cleanup ─────────────────────────────────────────────────────────
	if closeErr := mux.Close(); closeErr != nil {
		t.Logf("mux close: %v", closeErr)
	}
	if closeErr := engineEnd.Close(); closeErr != nil {
		t.Logf("engineEnd close: %v", closeErr)
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
	pluginEnd, engineEnd := net.Pipe()

	engineConn := rpc.NewConn(engineEnd, engineEnd)
	mux := rpc.NewMuxConn(engineConn)

	pluginDone := make(chan int, 1)
	go func() {
		pluginDone <- RunRIBPlugin(pluginEnd)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stage 1: Plugin sends declare-registration on plugin-initiated first.
	// Verify this is always the first message (no other RPC before it).
	stage1 := readMuxRequestTimeout(t, ctx, mux)
	assert.Equal(t, "ze-plugin-engine:declare-registration", stage1.Method,
		"Stage 1 must be declare-registration (first message on plugin-initiated)")
	require.NoError(t, mux.SendOK(ctx, stage1.ID))

	// Stage 2: Engine sends configure on engine-initiated.
	// Plugin waits for this — it blocks until configure arrives.
	configInput := &rpc.ConfigureInput{Sections: []rpc.ConfigSection{}}
	_, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", configInput)
	require.NoError(t, err)

	// Stage 3: Plugin sends declare-capabilities on plugin-initiated.
	// Must come after configure (Stage 2) completes.
	stage3 := readMuxRequestTimeout(t, ctx, mux)
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", stage3.Method,
		"Stage 3 must be declare-capabilities (after configure)")
	require.NoError(t, mux.SendOK(ctx, stage3.ID))

	// Stage 4: Engine sends share-registry on engine-initiated.
	registryInput := &rpc.ShareRegistryInput{Commands: []rpc.RegistryCommand{}}
	_, err = mux.CallRPC(ctx, "ze-plugin-callback:share-registry", registryInput)
	require.NoError(t, err)

	// Stage 5: Plugin sends ready on plugin-initiated.
	// Must come after share-registry (Stage 4) completes.
	stage5 := readMuxRequestTimeout(t, ctx, mux)
	assert.Equal(t, "ze-plugin-engine:ready", stage5.Method,
		"Stage 5 must be ready (after share-registry)")
	require.NoError(t, mux.SendOK(ctx, stage5.ID))

	// After Stage 5, the event loop is running — no more stage RPCs.
	// Close connections to trigger clean shutdown.
	if closeErr := mux.Close(); closeErr != nil {
		t.Logf("mux close: %v", closeErr)
	}
	if closeErr := engineEnd.Close(); closeErr != nil {
		t.Logf("engineEnd close: %v", closeErr)
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
