// Design: docs/architecture/api/process-protocol.md -- runtime replacement admission.
package server

import (
	"context"
	"encoding/json"
	"net"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestReplacementOutputWaitsForPeerDeclarations drives genuine respawns and
// probes the JSON and typed selected producers while reconciliation is pending.
// An unchanged successor must inherit the hold, then receive exactly its own
// post-startup callback after the transaction has reconciled its declaration.
func TestReplacementOutputWaitsForPeerDeclarations(t *testing.T) {
	for _, test := range []struct {
		name                   string
		deferred, duringFanout bool
	}{
		{name: "immediate"},
		{name: "deferred successor", deferred: true},
		{name: "initial fanout race", deferred: true, duringFanout: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), failureWait)
			defer cancel()
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			const pluginName = "runtime-peer-admission"
			var changed atomic.Bool
			postStartup := make(chan struct{}, 4)
			registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
				p := sdk.NewWithConn(pluginName, conn)
				p.OnAllPluginsReady(func() error {
					postStartup <- struct{}{}
					return nil
				})
				registration := &sdk.Registration{}
				if changed.Load() {
					registration.Claims = []string{"runtime-admission-owner"}
				}
				if err := p.Run(context.Background(), registration); err != nil {
					return 1
				}
				return 0
			})
			s, _ := newLifecycleStartupServer(t)
			rc := &selectedBoundaryReactor{mockReactor: &mockReactor{}}
			s.reactor = rc
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON}}))
			s.signalStartupComplete()
			select {
			case <-postStartup:
			case <-ctx.Done():
				t.Fatal("initial post-startup callback did not arrive")
			}
			pm := s.procManager.Load()
			input := selectedServerInput()
			encoded, err := json.Marshal(input)
			require.NoError(t, err)
			assertHeld := func() {
				proc := pm.GetProcess(pluginName)
				require.NotNil(t, proc)
				_, err := s.opSelectRoutes(t.Context(), proc, encoded)
				require.ErrorContains(t, err, "peer declarations")
				_, err = s.selectRoutes(t.Context(), proc, input)
				require.ErrorContains(t, err, "peer declarations")
				require.Zero(t, rc.calls, "no pending caller may reach final output")
				require.Empty(t, postStartup, "post-startup cannot publish before admission")
			}
			var rebuilds int
			registry.SetPeerDeclarationsChanged(func() error {
				rebuilds++
				assertHeld()
				return nil
			})
			t.Cleanup(func() { registry.SetPeerDeclarationsChanged(nil) })
			changed.Store(true)
			if test.deferred {
				require.True(t, s.txLock.tryAcquire())
			}
			require.NoError(t, s.restartPlugin(pluginName))
			if test.deferred {
				first := pm.GetProcess(pluginName)
				assertHeld()
				require.NoError(t, s.restartPlugin(pluginName))
				require.NotSame(t, first, pm.GetProcess(pluginName))
				assertHeld()
				if test.duringFanout {
					// Reproduce fan-out begun but startupDone not yet closed.
					s.startupDone = make(chan struct{})
					s.sendPostStartupToAll()
					assertHeld()
				}
				s.txLock.release()
			}
			require.Equal(t, 1, rebuilds)
			_, err = s.selectRoutes(t.Context(), pm.GetProcess(pluginName), input)
			require.NoError(t, err, "the reconciled current replacement remains useful")
			require.Equal(t, 1, rc.calls)
			select {
			case <-postStartup:
			case <-ctx.Done():
				t.Fatal("admitted replacement did not receive post-startup")
			}
			require.Empty(t, postStartup)
		})
	}
}

// TestPendingPeerDeclarationsFenceEveryOutputRail probes the actual bridge's
// JSON and typed registrations before their shared producers can touch a
// dispatcher/reactor. State listing still answers while output is held.
func TestPendingPeerDeclarationsFenceEveryOutputRail(t *testing.T) {
	s := &Server{ctx: t.Context()}
	proc := nativeOwnerProcess(t, s)
	proc.HoldPeerDeclarations()
	bridge := proc.Bridge()
	bridge.SetReady()
	_, err := proc.SelectedOwner()
	require.ErrorContains(t, err, "peer declarations", "source leases share this exact owner admission")
	for _, test := range []struct {
		name, method string
		input        any
		typed        func() error
	}{
		{name: "update", method: rpc.MethodUpdateRoute, input: rpc.UpdateRouteInput{}, typed: func() error {
			_, _, err := bridge.UpdateRouteSel(t.Context(), selector.ParseDefault("*"), "route", nil, 0)
			return err
		}},
		{name: "cached", method: rpc.MethodForwardCached, input: rpc.ForwardCachedInput{}, typed: func() error {
			return bridge.ForwardCached(t.Context(), rpc.ForwardCachedInput{IDs: []uint64{1}, Destinations: []string{"192.0.2.1"}})
		}},
		{name: "stored", method: rpc.MethodRelayStoredRoute, input: rpc.RelayStoredRouteInput{}, typed: func() error {
			return bridge.RelayStoredRoute(t.Context(), rpc.RelayStoredRouteInput{Destination: "192.0.2.1", Routes: []rpc.StoredRoute{{}}})
		}},
		{name: "command", method: rpc.MethodDispatchCommand, input: rpc.DispatchCommandInput{}, typed: func() error {
			_, err := bridge.DispatchCommand(t.Context(), "send bgp * route")
			return err
		}},
		{name: "command args", method: rpc.MethodDispatchCommandArgs, input: rpc.DispatchCommandArgsInput{}, typed: func() error {
			_, err := bridge.DispatchCommandArgs(t.Context(), "send bgp", []string{"route"}, "*")
			return err
		}},
		{name: "wire injection", method: rpc.MethodInjectWireRoute, input: rpc.InjectWireRouteInput{}, typed: func() error {
			return bridge.InjectWireRoute("bmp", "peer", []byte{1})
		}},
		{name: "selected", method: rpc.MethodSelectRoutes, input: selectedServerInput(), typed: func() error {
			_, err := bridge.SelectRoutes(t.Context(), selectedServerInput())
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.input)
			require.NoError(t, err)
			_, err = bridge.DispatchRPC(t.Context(), test.method, encoded)
			require.ErrorContains(t, err, "peer declarations")
			require.ErrorContains(t, test.typed(), "peer declarations")
		})
	}
	_, err = bridge.DispatchRPC(t.Context(), rpc.MethodStateList, json.RawMessage(`{"key":"meta/"}`))
	require.NoError(t, err, "setup state operations are not output authority")
	_, captured := proc.SelectedOwnerForCleanup()
	require.False(t, captured, "a refused source or selected admission creates no owner")
}
