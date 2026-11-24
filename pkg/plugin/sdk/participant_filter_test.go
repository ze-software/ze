// Design: docs/architecture/api/process-protocol.md -- Stage 2 and live rebuild participant filter delivery in SDK
// Related: sdk.go -- Plugin.ParticipantFilter
// Related: sdk_callbacks.go -- Plugin.OnParticipantFilter and initCallbackDefaults
// Related: sdk_dispatch.go -- handleConfigure and handleParticipantFilter

package sdk

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestParticipantFilterStoredBeforeConfigureAndUpdatedOnRebuild verifies both
// Stage 2 delivery and live rebuild delivery over the wire protocol.
//
// VALIDATES: handleConfigure stores a deep clone of ParticipantFilter BEFORE
// invoking OnConfigure (so ParticipantFilter() inside OnConfigure returns the
// Stage 2 filter), invokes OnParticipantFilter on Stage 2 configure, and
// handles runtime ze-plugin-callback:participant-filter RPCs by updating
// ParticipantFilter() and invoking OnParticipantFilter with an isolated clone.
// PREVENTS: plugins observing a nil ParticipantFilter during Stage 2 setup,
// caller mutation corrupting SDK-held filter state, or live delivery-graph
// rebuilds failing to update running plugins.
func TestParticipantFilterStoredBeforeConfigureAndUpdatedOnRebuild(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	initialFilter := &rpc.BusParticipantFilter{
		ParticipantPath: "/endpoint/demo",
		EndpointName:    "demo",
		SendAll:         true,
		SendToAll:       true,
		SendRules: []rpc.BusFilterRule{{
			Message:   "bgp:update",
			Allowed:   true,
			TargetAll: true,
			Targets: []rpc.BusFilterTarget{{
				Path:    "/bgp/peer/p1",
				Address: "192.0.2.1",
			}},
		}},
		ReceiveAll:     true,
		ReceiveFromAll: true,
	}
	rebuiltFilter := &rpc.BusParticipantFilter{
		ParticipantPath: "/endpoint/demo",
		EndpointName:    "demo",
		SendAll:         true,
		SendRules: []rpc.BusFilterRule{{
			Message:   "bgp:update",
			Allowed:   true,
			TargetAll: false,
			Targets: []rpc.BusFilterTarget{
				{Path: "/bgp/peer/p1", Address: "192.0.2.1"},
				{Path: "/bgp/peer/p2", Address: "192.0.2.2"},
			},
		}},
		ReceiveAll:      true,
		ReceiveFromNone: true,
	}

	seenAtConfigure := make(chan *rpc.BusParticipantFilter, 1)
	callbackEvents := make(chan *rpc.BusParticipantFilter, 4)

	p.OnConfigure(func([]ConfigSection) error {
		got := p.ParticipantFilter()
		seenAtConfigure <- got
		// Mutate the returned copy to prove internal state is deep-cloned.
		if got != nil && len(got.SendRules) > 0 && len(got.SendRules[0].Targets) > 0 {
			got.SendRules[0].Targets[0].Address = "mutated"
		}
		return nil
	})
	p.OnParticipantFilter(func(f *rpc.BusParticipantFilter) {
		callbackEvents <- f
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, &Registration{})
	}()

	// Stage 1.
	req := readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, rpc.MethodDeclareRegistration, req.Method)
	require.NoError(t, engine.mux.SendResult(ctx, req.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}))

	// Stage 2: deliver initial ParticipantFilter alongside config sections.
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:configure", rpc.ConfigureInput{
		Sections:          []rpc.ConfigSection{{Root: "bgp", Data: `{}`}},
		ParticipantFilter: initialFilter,
	}))

	select {
	case got := <-seenAtConfigure:
		require.NotNil(t, got, "ParticipantFilter must be populated before OnConfigure runs")
		assert.Equal(t, "/endpoint/demo", got.ParticipantPath)
		assert.True(t, got.SendToAll)
		assert.True(t, got.ReceiveFromAll)
		require.Len(t, got.SendRules, 1)
		assert.True(t, got.SendRules[0].TargetAll)
		require.Len(t, got.SendRules[0].Targets, 1)
		// Original value was mutated inside OnConfigure after channel send, so
		// verify SDK-held state was NOT affected by that mutation.
		held := p.ParticipantFilter()
		require.NotNil(t, held)
		assert.Equal(t, "192.0.2.1", held.SendRules[0].Targets[0].Address)
	case <-time.After(2 * time.Second):
		t.Fatal("OnConfigure was not called")
	}

	select {
	case cbFilter := <-callbackEvents:
		require.NotNil(t, cbFilter)
		assert.Equal(t, "192.0.2.1", cbFilter.SendRules[0].Targets[0].Address)
	case <-time.After(2 * time.Second):
		t.Fatal("OnParticipantFilter was not called at Stage 2")
	}

	// Stage 3.
	req = readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Stage 4.
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{}))

	// Stage 5.
	req = readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Runtime live rebuild RPC.
	require.NoError(t, callAndExpectOK(ctx, engine.mux, rpc.MethodParticipantFilter, rpc.ParticipantFilterInput{
		Filter: rebuiltFilter,
	}))

	select {
	case cbFilter := <-callbackEvents:
		require.NotNil(t, cbFilter)
		assert.True(t, cbFilter.ReceiveFromNone)
		require.Len(t, cbFilter.SendRules, 1)
		require.Len(t, cbFilter.SendRules[0].Targets, 2)
		assert.Equal(t, "192.0.2.2", cbFilter.SendRules[0].Targets[1].Address)
	case <-time.After(2 * time.Second):
		t.Fatal("OnParticipantFilter was not called on runtime rebuild RPC")
	}

	heldAfterRebuild := p.ParticipantFilter()
	require.NotNil(t, heldAfterRebuild)
	assert.True(t, heldAfterRebuild.ReceiveFromNone)
	require.Len(t, heldAfterRebuild.SendRules[0].Targets, 2)
	assert.Equal(t, "192.0.2.2", heldAfterRebuild.SendRules[0].Targets[1].Address)

	// Clean shutdown.
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye", rpc.ByeInput{Reason: "test-complete"}))
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit after bye")
	}
}

// TestParticipantFilterDefaultHandlerOverBridge verifies that an internal
// plugin over DirectBridge accepts live participant-filter updates even without
// registering an OnParticipantFilter callback, and stores the updated filter.
//
// VALIDATES: initCallbackDefaults registers rpc.MethodParticipantFilter so
// plugins that do not install OnParticipantFilter answer ok and keep
// ParticipantFilter() current over DirectBridge.
// PREVENTS: live delivery-graph rebuilds failing with "unknown method" on
// internal plugins that rely on polling ParticipantFilter().
func TestParticipantFilterDefaultHandlerOverBridge(t *testing.T) {
	t.Parallel()

	engineEnd, pluginEnd := net.Pipe()
	bridge := rpc.NewDirectBridge()
	p := NewWithConn("bridged-filter-test", rpc.NewBridgedConn(pluginEnd, bridge))
	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	t.Cleanup(func() {
		engineMux.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
		pluginEnd.Close() //nolint:errcheck // test cleanup
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, &Registration{})
	}()

	req := readEngineRequest(t, ctx, engineMux)
	require.Equal(t, rpc.MethodDeclareRegistration, req.Method)
	require.NoError(t, engineMux.SendResult(ctx, req.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}))
	require.NoError(t, callAndExpectOK(ctx, engineMux, "ze-plugin-callback:configure", rpc.ConfigureInput{}))
	req = readEngineRequest(t, ctx, engineMux)
	require.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
	require.NoError(t, engineMux.SendOK(ctx, req.ID))
	require.NoError(t, callAndExpectOK(ctx, engineMux, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{}))
	req = readEngineRequest(t, ctx, engineMux)
	require.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engineMux.SendOK(ctx, req.ID))

	rawParams, err := json.Marshal(rpc.ParticipantFilterInput{
		Filter: &rpc.BusParticipantFilter{
			ParticipantPath: "/endpoint/bridged-filter-test",
			EndpointName:    "bridged-filter-test",
			SendToNone:      true,
			ReceiveFromAll:  true,
		},
	})
	require.NoError(t, err)

	_, err = bridge.SendCallback(ctx, rpc.MethodParticipantFilter, rawParams)
	require.NoError(t, err)

	got := p.ParticipantFilter()
	require.NotNil(t, got)
	assert.True(t, got.SendToNone)
	assert.True(t, got.ReceiveFromAll)

	byeParams, err := json.Marshal(rpc.ByeInput{Reason: "done"})
	require.NoError(t, err)
	_, err = bridge.SendCallback(ctx, "ze-plugin-callback:bye", byeParams)
	require.NoError(t, err)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("bridged plugin did not exit after bye")
	}
}
