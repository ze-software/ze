//go:build integration && linux

// Design: docs/architecture/iface/logical-name-resolution.md -- a failed binding keeps live state
package bfd

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/userns"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// configureBFDInterfaceSelector drives the real interface SDK callback so the
// shared resolver learns actual configured selectors, not an injected error.
// One names loopback; the other device is absent and interface apply defers it.
func configureBFDInterfaceSelector(t *testing.T) {
	t.Helper()
	reg := registry.Lookup("interface")
	require.NotNil(t, reg)
	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	done := make(chan struct{})
	go func() {
		reg.RunEngine(pluginEnd)
		close(done)
	}()
	t.Cleanup(func() {
		_ = mux.Close()
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("interface engine did not stop")
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	ack := func(method string) {
		t.Helper()
		select {
		case request := <-mux.Requests():
			require.NotNil(t, request)
			require.Equal(t, method, request.Method)
			require.NoError(t, mux.SendOK(ctx, request.ID))
		case <-ctx.Done():
			t.Fatalf("waiting for %s: %v", method, ctx.Err())
		}
	}
	ack("ze-plugin-engine:declare-registration")
	raw, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", rpc.ConfigureInput{Sections: []rpc.ConfigSection{{
		Root: "interface",
		Data: `{"interface":{"backend":"netlink","ethernet":{"bfd-real-lo":{"name":"bfd-real-lo","os-name":"lo"},"bfd-unbound":{"name":"bfd-unbound","os-name":"bfd-absent"}}}}`,
	}}})
	require.NoError(t, err)
	if len(raw) != 0 {
		var result rpc.ConfigApplyOutput
		require.NoError(t, json.Unmarshal(raw, &result))
		require.NotEqual(t, rpc.StatusError, result.Status, result.Error)
	}
	ack("ze-plugin-engine:declare-capabilities")
	_, err = mux.CallRPC(ctx, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	require.NoError(t, err)
	ack("ze-plugin-engine:ready")
}

// VALIDATES: a binding refusal is atomic for both configuration readers and
// existing session consumers. It must not publish rejected profiles/bindV6 or
// release an existing pinned handle before the unresolved candidate is refused.
func TestBFDUnresolvedSelectorKeepsActiveConfigurationAndHandles(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	state := newRuntimeState()
	t.Cleanup(state.stopAll)
	entry, err := parseSingleHopSession(wireListener.String(), map[string]any{
		"local": wireSender.String(), "interface": "bfd-real-lo",
	}, nil)
	require.NoError(t, err)
	active := &pluginConfig{
		enabled: true,
		profiles: map[string]profileConfig{"retained": {
			name: "retained", detectMult: 3, desiredMinTxUs: 100_000, requiredMinRxUs: 100_000,
		}},
		sessions: []sessionConfig{entry},
	}
	// Use the actual registered consumer graph, not a hand-written order:
	// BFD must observe interface's SDK publication before opening its socket.
	tiers, err := registry.TopologicalTiers([]string{"bfd", "interface"}, nil)
	require.NoError(t, err)
	for _, tier := range tiers {
		for _, name := range tier {
			switch name {
			case "interface":
				configureBFDInterfaceSelector(t)
			case "bfd":
				require.NoError(t, state.applyPinned(active))
			}
		}
	}
	var key api.Key
	var handle api.SessionHandle
	for key, handle = range state.pinned {
		break
	}
	require.NotNil(t, handle)
	candidateEntry := entry
	candidateEntry.iface = "bfd-unbound"
	candidate := &pluginConfig{
		enabled: true, bindV6: true,
		profiles: map[string]profileConfig{"rejected": {
			name: "rejected", detectMult: 5, desiredMinTxUs: 900_000, requiredMinRxUs: 900_000,
		}},
		sessions: []sessionConfig{candidateEntry},
	}
	err = state.applyPinned(candidate)
	require.ErrorContains(t, err, "bfd-unbound")
	require.Same(t, active, state.cfg, "a refused binding must not publish candidate settings")
	require.False(t, state.cfg.bindV6)
	require.Len(t, state.pinned, 1)
	require.Same(t, handle, state.pinned[key])

	service := &pluginService{state: state}
	profiles := service.Profiles()
	require.Len(t, profiles, 1)
	require.Equal(t, "retained", profiles[0].Name)
	require.Equal(t, uint32(100_000), profiles[0].DesiredMinTxUs)
	req := api.SessionRequest{
		Peer: wireListener, Local: wireSender, Interface: "lo", Mode: api.SingleHop, Profile: "retained",
	}
	joined, err := service.EnsureSession(req)
	require.NoError(t, err, "runtime clients must still be able to use the active profile")
	require.Equal(t, key, joined.Key())
	req.Profile = "rejected"
	_, err = service.EnsureSession(req)
	require.ErrorContains(t, err, "not defined")
	snapshot := service.Snapshot()
	require.Len(t, snapshot, 1)
	require.Equal(t, 2, snapshot[0].Refcount)
}
