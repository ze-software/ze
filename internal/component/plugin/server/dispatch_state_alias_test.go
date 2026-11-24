// Design: docs/architecture/api/process-protocol.md -- plugin state grants keyed by compiled registration identity.
// Related: dispatch_state.go -- stateInput and opStateList, which authorize state RPCs per process.
// Related: dispatch_state_test.go -- stateRPCClientAs and stateRPCContext helpers.
// Related: dispatch_state_bound_test.go -- useStateTestStore helper.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	ddosFlowspecPlugin      = "ddos-flowspec"
	ddosFlowspecAlias       = "my-ddos"
	ddosFlowspecStateKey    = "meta/ddos-flowspec/announcement"
	ddosFlowspecStatePrefix = "meta/ddos-flowspec/"
	ddosFlowspecForeignRun  = "/usr/local/bin/fake-ddos"
)

// stateRPCClientWithConfig connects an SDK plugin backed by a Process built
// from cfg to a state dispatcher over the socket (direct false) or DirectBridge
// (direct true).
func stateRPCClientWithConfig(t *testing.T, direct bool, cfg plugin.PluginConfig) *sdk.Plugin {
	t.Helper()
	s := &Server{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	t.Cleanup(s.cancel)
	proc := process.NewProcess(cfg)
	client, engine := net.Pipe()
	conn := client
	if direct {
		bridge := rpc.NewDirectBridge()
		bridge.SetDeliverEvents(func([]string) error { return nil })
		bridge.SetDispatchRPC(func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
			return s.dispatchPluginRPCDirect(ctx, proc, method, params)
		})
		bridge.SetReady()
		conn = rpc.NewBridgedConn(client, bridge)
	}
	p := sdk.NewWithConn(cfg.Name, conn)
	done := make(chan struct{})
	if direct {
		close(done)
	} else {
		pc := plugipc.NewPluginConn(engine, engine)
		go func() {
			defer close(done)
			for {
				req, err := pc.ReadRequest(s.ctx)
				if err != nil {
					return
				}
				s.serveEngineOpJSON(proc, pc, req, lookupEngineOp(req.Method))
			}
		}()
	}
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		<-done
	})
	return p
}

// TestProcessIdentityByRegistration verifies state RPC grants follow compiled
// registration identity rather than the operator's endpoint alias.
//
// VALIDATES: spec-explicit-plugin-and-attach-config AC-13 / S15 on both socket
// and DirectBridge transports:
//
//	(a) an internal plugin configured under alias "my-ddos" running registration
//	    "ddos-flowspec" is granted read, write, list, and remove access to
//	    "ddos-flowspec"'s state keys;
//	(b) a compiled plugin configured under alias "my-ddos" running registration
//	    "ddos-flowspec" with SeparateProcess: true keeps the same read, write,
//	    list, and remove grants;
//	(c) a foreign external plugin named "ddos-flowspec" (external with a run
//	    command) holds no grant to "ddos-flowspec"'s state keys: read, write,
//	    and remove are refused, and list returns no keys.
//
// PREVENTS: state RPC authorization using proc.Name() (the operator alias),
// which locks out aliased internal plugins and lets a foreign external plugin
// steal a compiled-in plugin's state keys by choosing the same endpoint name.
func TestProcessIdentityByRegistration(t *testing.T) {
	require.True(t, registry.Has(ddosFlowspecPlugin),
		"ddos-flowspec must be registered in this test binary (via all_import_test.go)")
	require.True(t, statestore.PluginKeyWritable(ddosFlowspecPlugin, ddosFlowspecStateKey),
		"ddos-flowspec must hold write grant on %s", ddosFlowspecStateKey)
	require.False(t, statestore.PluginKeyReadable(ddosFlowspecAlias, ddosFlowspecStateKey),
		"alias %s must hold no direct statestore grant of its own", ddosFlowspecAlias)

	payload := []byte(`{"dst":"198.51.100.1/32"}`)
	spoofPayload := []byte(`{"dst":"203.0.113.1/32"}`)

	for _, direct := range []bool{false, true} {
		transport := "socket"
		if direct {
			transport = "direct"
		}

		t.Run("aliased-internal/"+transport, func(t *testing.T) {
			useStateTestStore(t)
			ctx, cancel := stateRPCContext(t)
			defer cancel()

			cfg := plugin.PluginConfig{
				Name:     ddosFlowspecAlias,
				Run:      ddosFlowspecPlugin,
				Internal: true,
			}
			p := stateRPCClientWithConfig(t, direct, cfg)

			err := p.StatePut(ctx, ddosFlowspecStateKey, payload)
			require.NoError(t, err, "aliased internal %s (%s) must be allowed to write %s",
				ddosFlowspecAlias, ddosFlowspecPlugin, ddosFlowspecStateKey)

			got, found, err := p.StateGet(ctx, ddosFlowspecStateKey)
			require.NoError(t, err, "aliased internal %s (%s) must be allowed to read %s",
				ddosFlowspecAlias, ddosFlowspecPlugin, ddosFlowspecStateKey)
			require.True(t, found)
			require.True(t, bytes.Equal(got, payload), "read %q, want %q", got, payload)

			keys, err := p.StateList(ctx, ddosFlowspecStatePrefix)
			require.NoError(t, err, "aliased internal %s (%s) must be allowed to list %s",
				ddosFlowspecAlias, ddosFlowspecPlugin, ddosFlowspecStatePrefix)
			require.True(t, slices.Equal(keys, []string{ddosFlowspecStateKey}),
				"list = %v, want [%s]", keys, ddosFlowspecStateKey)

			err = p.StateRemove(ctx, ddosFlowspecStateKey)
			require.NoError(t, err, "aliased internal %s (%s) must be allowed to remove %s",
				ddosFlowspecAlias, ddosFlowspecPlugin, ddosFlowspecStateKey)
		})

		t.Run("aliased-separate/"+transport, func(t *testing.T) {
			useStateTestStore(t)
			ctx, cancel := stateRPCContext(t)
			defer cancel()

			cfg := plugin.PluginConfig{
				Name:            ddosFlowspecAlias,
				CompiledPlugin:  ddosFlowspecPlugin,
				SeparateProcess: true,
			}
			p := stateRPCClientWithConfig(t, direct, cfg)

			err := p.StatePut(ctx, ddosFlowspecStateKey, payload)
			require.NoError(t, err, "aliased separate-process %s (%s) must be allowed to write %s",
				ddosFlowspecAlias, ddosFlowspecPlugin, ddosFlowspecStateKey)

			got, found, err := p.StateGet(ctx, ddosFlowspecStateKey)
			require.NoError(t, err, "aliased separate-process %s (%s) must be allowed to read %s",
				ddosFlowspecAlias, ddosFlowspecPlugin, ddosFlowspecStateKey)
			require.True(t, found)
			require.True(t, bytes.Equal(got, payload), "read %q, want %q", got, payload)

			keys, err := p.StateList(ctx, ddosFlowspecStatePrefix)
			require.NoError(t, err, "aliased separate-process %s (%s) must be allowed to list %s",
				ddosFlowspecAlias, ddosFlowspecPlugin, ddosFlowspecStatePrefix)
			require.True(t, slices.Equal(keys, []string{ddosFlowspecStateKey}),
				"list = %v, want [%s]", keys, ddosFlowspecStateKey)

			err = p.StateRemove(ctx, ddosFlowspecStateKey)
			require.NoError(t, err, "aliased separate-process %s (%s) must be allowed to remove %s",
				ddosFlowspecAlias, ddosFlowspecPlugin, ddosFlowspecStateKey)
		})

		t.Run("foreign-external-same-name/"+transport, func(t *testing.T) {
			useStateTestStore(t)
			require.NoError(t, statestore.Write(ddosFlowspecStateKey, payload))

			ctx, cancel := stateRPCContext(t)
			defer cancel()

			cfg := plugin.PluginConfig{
				Name:     ddosFlowspecPlugin,
				Run:      ddosFlowspecForeignRun,
				Internal: false,
			}
			p := stateRPCClientWithConfig(t, direct, cfg)

			_, _, err := p.StateGet(ctx, ddosFlowspecStateKey)
			require.Error(t, err, "foreign external plugin named %s must be refused read of %s",
				ddosFlowspecPlugin, ddosFlowspecStateKey)

			err = p.StatePut(ctx, ddosFlowspecStateKey, spoofPayload)
			require.Error(t, err, "foreign external plugin named %s must be refused write of %s",
				ddosFlowspecPlugin, ddosFlowspecStateKey)

			err = p.StateRemove(ctx, ddosFlowspecStateKey)
			require.Error(t, err, "foreign external plugin named %s must be refused remove of %s",
				ddosFlowspecPlugin, ddosFlowspecStateKey)

			keys, err := p.StateList(ctx, ddosFlowspecStatePrefix)
			if err == nil {
				require.Empty(t, keys, "foreign external plugin named %s must not list %s keys",
					ddosFlowspecPlugin, ddosFlowspecStatePrefix)
			}

			stored, found, err := statestore.Read(ddosFlowspecStateKey)
			require.NoError(t, err)
			require.True(t, found)
			require.True(t, bytes.Equal(stored, payload),
				"stored state changed under refused foreign mutations: got %q, want %q", stored, payload)
		})
	}
}
