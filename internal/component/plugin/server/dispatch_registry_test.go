package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestPluginRPCRegistryCoversAllPaths is the drift guard for the unified
// plugin->engine method registry (unify-rpc-dispatch, AC-4). Before the
// unification the same operation lived in up to three hand-maintained tables
// (the JSON switch, the Direct switch, the wireBridgeDispatch Set* list) keyed
// only by magic strings, and nothing forced them to agree. Now every operation
// is exactly one engineOp entry from which all three transports derive, so this
// test locks the method set and the typed-descriptor set: adding or dropping an
// op, or moving one on/off the typed fast path, must be a deliberate edit here.
func TestPluginRPCRegistryCoversAllPaths(t *testing.T) {
	t.Parallel()

	// Every plugin->engine runtime op. Each has a JSON socket path and an
	// in-process Direct path (both derive from engineOp.handle).
	wantMethods := []string{
		rpc.MethodUpdateRoute,
		rpc.MethodDispatchCommand,
		rpc.MethodDispatchCommandArgs,
		rpc.MethodSubscribeEvents,
		rpc.MethodUnsubscribeEvents,
		rpc.MethodEmitEvent,
		rpc.MethodForwardCached,
		rpc.MethodReleaseCached,
		rpc.MethodRouteInstall,
		rpc.MethodRouteRemove,
		rpc.MethodInjectWireRoute,
		rpc.MethodBatchValidate,
	}

	// The subset that also carries a typed DirectBridge fast-path slot. The
	// remainder (subscribe/unsubscribe, route-install/route-remove) intentionally
	// have no typed slot, exactly as before the unification.
	wantTyped := map[string]bool{
		rpc.MethodUpdateRoute:         true, // typed *selector.Selector variant
		rpc.MethodDispatchCommand:     true,
		rpc.MethodDispatchCommandArgs: true,
		rpc.MethodEmitEvent:           true,
		rpc.MethodForwardCached:       true,
		rpc.MethodReleaseCached:       true,
		rpc.MethodInjectWireRoute:     true,
		rpc.MethodBatchValidate:       true,
	}

	// The table advertises exactly the expected method set (no missing, no extra).
	gotMethods := make(map[string]bool, len(engineOps))
	for i := range engineOps {
		op := &engineOps[i]
		assert.False(t, gotMethods[op.method], "duplicate engineOp method: %s", op.method)
		gotMethods[op.method] = true

		// JSON + Direct both run entry.handle; it must exist for every op.
		assert.NotNil(t, op.handle, "engineOp %s has nil handle (JSON/Direct path missing)", op.method)

		// Typed-descriptor presence must match the declared fast-path set.
		if wantTyped[op.method] {
			assert.NotNil(t, op.typedWire, "engineOp %s expected a typed bridge descriptor", op.method)
		} else {
			assert.Nil(t, op.typedWire, "engineOp %s must NOT have a typed bridge descriptor", op.method)
		}
	}

	for _, m := range wantMethods {
		assert.True(t, gotMethods[m], "engineOp registry missing method: %s", m)
	}
	assert.Len(t, engineOps, len(wantMethods), "engineOp count drifted from expected op set")

	// lookupEngineOp resolves known methods and is fail-closed for unknown ones.
	for _, m := range wantMethods {
		assert.NotNil(t, lookupEngineOp(m), "lookupEngineOp(%q) should resolve", m)
	}
	assert.Nil(t, lookupEngineOp("ze-plugin-engine:does-not-exist"), "unknown method must not resolve")
	assert.Nil(t, lookupEngineOp(""), "empty method must not resolve")
}

// TestWireBridgeDispatchInstallsTypedSlots verifies that driving the typed
// descriptors over a bridge installs exactly the typed fast-path slots the
// registry declares (AC-3): the native slots are set by iterating the registry,
// not a hand-written list, and no JSON marshal is introduced on those paths.
func TestWireBridgeDispatchInstallsTypedSlots(t *testing.T) {
	t.Parallel()

	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	proc := process.NewProcess(plugin.PluginConfig{Name: "typed-slots"})
	b := rpc.NewDirectBridge()
	b.SetReady()

	for i := range engineOps {
		if engineOps[i].typedWire != nil {
			engineOps[i].typedWire(s, proc, b)
		}
	}

	assert.True(t, b.HasEmitEvent(), "emit-event typed slot not installed")
	assert.True(t, b.HasDispatchCommand(), "dispatch-command typed slot not installed")
	assert.True(t, b.HasDispatchCommandArgs(), "dispatch-command-args typed slot not installed")
	assert.True(t, b.HasUpdateRouteSel(), "update-route-sel typed slot not installed")
	assert.True(t, b.HasForwardCached(), "forward-cached typed slot not installed")
	assert.True(t, b.HasReleaseCached(), "release-cached typed slot not installed")
	assert.True(t, b.HasInjectWireRoute(), "inject-wire-route typed slot not installed")
	assert.True(t, b.HasBatchValidate(), "batch-validate typed slot not installed")
}

// TestEngineOpJSONAndDirectMatch asserts the JSON socket path and the in-process
// Direct path produce byte-identical results for a representative op, proving
// both derive from the one entry (AC-2). dispatch-command is used because it
// exercises the shared s.dispatchCommand core through both serve wrappers.
func TestEngineOpJSONAndDirectMatch(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	d.Register("parity test", func(_ *CommandContext, _ []string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"ok": true}}, nil
	}, "parity test")

	s := &Server{subscriptions: NewSubscriptionManager(), dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	proc := process.NewProcess(plugin.PluginConfig{Name: "parity"})

	op := lookupEngineOp(rpc.MethodDispatchCommand)
	require.NotNil(t, op)

	params := []byte(`{"command":"parity test"}`)
	result, err := op.handle(s, proc, params)
	require.NoError(t, err)

	direct, err := s.dispatchPluginRPCDirect(proc, rpc.MethodDispatchCommand, params)
	require.NoError(t, err)

	marshaled, err := directResultResponse(result)
	require.NoError(t, err)
	assert.JSONEq(t, string(marshaled), string(direct), "JSON and Direct results diverged")
}
