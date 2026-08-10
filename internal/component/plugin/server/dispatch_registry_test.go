package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
		rpc.MethodRelayStoredRoute,
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
		rpc.MethodRelayStoredRoute:    true, // adj-rib-in peer-up replay, in-process
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

	s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
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

// TestOpUpdateRouteInjectsInternalIdentity pins the fix for the route-propagation
// break (spec-fixit-authz-admin-fallthrough review finding 3). opUpdateRoute is an
// internal route-push RPC (RS/OSPF/IS-IS `update text`); before the fix it built a
// CommandContext with no Username, so on a box with authorization configured it hit
// the now-fail-closed "denied: empty identity" branch and route propagation broke.
// It must inject the reserved internal identity like the other dispatch paths.
//
// VALIDATES: opUpdateRoute dispatches under the reserved internal identity.
// PREVENTS: internal route push failing closed on an RBAC-configured box.
func TestOpUpdateRouteInjectsInternalIdentity(t *testing.T) {
	t.Parallel()

	d := NewDispatcher()
	var gotUsername string
	// A single-token key "peer" matches "peer <sel> route" with args [<sel>,route];
	// the handler records the identity the internal dispatch injected.
	d.Register("peer", func(ctx *CommandContext, _ []string) (*plugin.Response, error) {
		gotUsername = ctx.Username
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}, "peer")

	s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	proc := process.NewProcess(plugin.PluginConfig{Name: "routepush"})
	params := []byte(`{"command":"route","peer-selector":"p1"}`)
	_, err := s.opUpdateRoute(proc, params)
	require.NoError(t, err)
	assert.Equal(t, internalPluginIdentity("routepush"), gotUsername,
		"opUpdateRoute must inject the reserved internal identity, not an empty username")
}

// TestEngineOpInjectWireRouteJSONFallback exercises the inject-wire-route JSON
// codec fallback end-to-end (AC-6): dispatch the wire method through the Direct
// path, prove opInjectWireRoute unmarshals rpc.InjectWireRouteInput and forwards
// the round-tripped protocol/peer/body to the registered route injector, and that
// an unregistered injector fails closed.
//
// VALIDATES: AC-6 -- inject-wire-route has a working non-typed (JSON codec) path.
// PREVENTS: a silent regression in the InjectWireRouteInput round-trip, the
//
//	GetRouteInjector wiring, or the "no route injector registered" guard.
//
// Not parallel: mutates the process-global route injector (saved/restored).
func TestEngineOpInjectWireRouteJSONFallback(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	proc := process.NewProcess(plugin.PluginConfig{Name: "inject-json"})

	prev := rpc.GetRouteInjector()
	defer rpc.RegisterRouteInjector(prev)

	// Unregistered: fail closed with an explicit error, no panic.
	rpc.RegisterRouteInjector(nil)
	_, err := s.dispatchPluginRPCDirect(proc, rpc.MethodInjectWireRoute,
		json.RawMessage(`{"protocol":"bmp","peer-key":"p","update-body":"AQID"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no route injector registered")

	// Registered: the injector receives the round-tripped values; base64 "AQID"
	// decodes to 0x01 0x02 0x03.
	var gotProto, gotPeer string
	var gotBody []byte
	rpc.RegisterRouteInjector(func(protocol, peerKey string, updateBody []byte) error {
		gotProto, gotPeer, gotBody = protocol, peerKey, updateBody
		return nil
	})
	res, err := s.dispatchPluginRPCDirect(proc, rpc.MethodInjectWireRoute,
		json.RawMessage(`{"protocol":"bmp","peer-key":"peer-1","update-body":"AQID"}`))
	require.NoError(t, err)
	assert.Nil(t, res, "inject-wire-route returns no result payload")
	assert.Equal(t, "bmp", gotProto)
	assert.Equal(t, "peer-1", gotPeer)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, gotBody)
}

// TestEngineOpBatchValidateJSONFallback exercises the batch-validate JSON codec
// fallback end-to-end (AC-6): dispatch the wire method through the Direct path,
// prove opBatchValidate unmarshals rpc.BatchValidateInput, forwards the decisions
// to the registered batch validator, and marshals the *BatchValidateResult back;
// and that an unregistered validator fails closed.
//
// VALIDATES: AC-6 -- batch-validate has a working non-typed (JSON codec) path.
// PREVENTS: a silent regression in the BatchValidateInput/Result round-trip, the
//
//	GetBatchValidator wiring, or the "no batch validator registered" guard.
//
// Not parallel: mutates the process-global batch validator (saved/restored).
func TestEngineOpBatchValidateJSONFallback(t *testing.T) {
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	proc := process.NewProcess(plugin.PluginConfig{Name: "batch-json"})

	prev := rpc.GetBatchValidator()
	defer rpc.RegisterBatchValidator(prev)

	// Unregistered: fail closed with an explicit error.
	rpc.RegisterBatchValidator(nil)
	_, err := s.dispatchPluginRPCDirect(proc, rpc.MethodBatchValidate,
		json.RawMessage(`{"decisions":[]}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no batch validator registered")

	// Registered: the validator receives the round-tripped decision and its
	// result marshals back through directResultResponse. ValidationDecision has
	// no JSON tags, so keys are the Go field names.
	var got []rpc.ValidationDecision
	rpc.RegisterBatchValidator(func(decisions []rpc.ValidationDecision) (*rpc.BatchValidateResult, error) {
		got = decisions
		return &rpc.BatchValidateResult{Accepted: 2, Rejected: 1, Early: 0}, nil
	})
	res, err := s.dispatchPluginRPCDirect(proc, rpc.MethodBatchValidate,
		json.RawMessage(`{"decisions":[{"Accept":true,"PeerAddr":"10.0.0.1","Family":"ipv4/unicast","Prefix":"10.0.0.0/24","PathID":7,"ValState":1}]}`))
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.True(t, got[0].Accept)
	assert.Equal(t, "10.0.0.1", got[0].PeerAddr)
	assert.Equal(t, uint32(7), got[0].PathID)
	assert.Equal(t, uint8(1), got[0].ValState)

	var out rpc.BatchValidateResult
	require.NoError(t, json.Unmarshal(res, &out))
	assert.Equal(t, 2, out.Accepted)
	assert.Equal(t, 1, out.Rejected)
}
